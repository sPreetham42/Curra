# CURRA Backend v1 — Final P0/P1 Remediation Report

---

## 1. P0-1 — Fix Subsequent Publish

### Problem
The database enforces a partial unique index:
```sql
CREATE UNIQUE INDEX idx_versions_one_published
ON schedule_versions(timetable_id)
WHERE status = 'PUBLISHED';
```
Previously, `PublishTx` attempted to transition a target version directly to `PUBLISHED` without archiving the existing `PUBLISHED` version for that timetable. Publishing a second version on any timetable caused an immediate PostgreSQL `unique_violation` constraint error.

### Remediated Implementation
In [`application/internal/database/repositories/schedule_version_repo.go`](file:///c:/Users/Preetham%20S/timetable-platform/application/internal/database/repositories/schedule_version_repo.go#L163), `PublishTx` executes inside a single explicit PostgreSQL transaction:

```sql
BEGIN;

  -- 1. Identify and transition any existing PUBLISHED version for this timetable to ARCHIVED
  UPDATE schedule_versions
  SET status = 'ARCHIVED', updated_at = now()
  WHERE timetable_id = $1 AND status = 'PUBLISHED' AND id != $2;

  -- 2. CAS transition the target version from REVIEW to PUBLISHED
  UPDATE schedule_versions
  SET status = 'PUBLISHED', version = version + 1, updated_at = now()
  WHERE id = $1 AND version = $2;
  -- If RowsAffected == 0 -> ROLLBACK and return ErrOptimisticLock (HTTP 409 Conflict)

  -- 3. Update the timetable's current published version pointer
  UPDATE timetables
  SET current_published_version_id = $1, updated_at = now()
  WHERE id = $2;

  -- 4. Insert transactional audit log
  INSERT INTO audit_events (id, institution_id, user_id, action, resource_type, resource_id, details, created_at)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

COMMIT;
```

### Concurrency and Invariant Guarantees
- **Atomic Swap of Published Version**: Because the archiving of the old version and activation of the new version happen within the same transaction, at no point can two versions exist with `status = 'PUBLISHED'`. The partial unique index `idx_versions_one_published` is strictly respected.
- **Rollback Safety**: If the CAS check fails on the target version or any error occurs, the entire transaction is rolled back, preserving the old version as `PUBLISHED` and the timetable pointer untouched.

### Test Evidence
- [`TestPublish_SubsequentPublishArchivesOldVersion`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L609): Version 1 (`PUBLISHED`) and Version 2 (`REVIEW`) $\rightarrow$ Publish Version 2 $\rightarrow$ Version 1 becomes `ARCHIVED`, Version 2 becomes `PUBLISHED`, timetable pointer updated to Version 2.
- [`TestPublish_RollbackPreservesOldPublished`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L670): Injected CAS failure $\rightarrow$ Version 1 remains `PUBLISHED`, Version 2 remains `REVIEW`, timetable pointer unchanged.
- [`TestConcurrency_PublishRace`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L721): Concurrent publication attempts result in exactly 1 winner and 1 HTTP 409 conflict.

---

## 2. P0-2 — Fix Idempotency Lock Ownership

### Problem
Previously, `Acquire` attempted to determine whether the calling process acquired the lock by checking whether `locked_at` was within 5 seconds of `time.Now()`. Because a fresh lock created by Caller A has a timestamp $< 5$ seconds old, a concurrent Caller B arriving 50ms later also evaluated that condition to `true`, resulting in concurrent duplication.

### Remediated Lock-Token Ownership Architecture
1. **Schema Update**: Added `lock_token UUID` column to `idempotency_keys` in [`migrations.go`](file:///c:/Users/Preetham%20S/timetable-platform/application/internal/database/migrations.go#L358) and [`domain/scheduling.go`](file:///c:/Users/Preetham%20S/timetable-platform/application/internal/domain/scheduling.go#L201).
2. **Atomic Upsert Query**: In [`idempotency_repo.go`](file:///c:/Users/Preetham%20S/timetable-platform/application/internal/database/repositories/idempotency_repo.go#L42):
   Each acquisition generates a cryptographically random `myLockToken := uuid.New()`.
   ```sql
   INSERT INTO idempotency_keys
     (id, institution_id, idempotency_key, status, resource_type, lock_token, locked_at, created_at, updated_at)
   VALUES ($1, $2, $3, 'IN_PROGRESS', $4, $5, now(), now(), now())
   ON CONFLICT (institution_id, idempotency_key) DO UPDATE
     SET lock_token = CASE
           WHEN idempotency_keys.status = 'IN_PROGRESS' AND idempotency_keys.locked_at < now() - INTERVAL '1 minute'
           THEN $5
           ELSE idempotency_keys.lock_token
         END,
         locked_at = CASE
           WHEN idempotency_keys.status = 'IN_PROGRESS' AND idempotency_keys.locked_at < now() - INTERVAL '1 minute'
           THEN now()
           ELSE idempotency_keys.locked_at
         END,
         updated_at = now()
   RETURNING id, institution_id, idempotency_key, status, resource_type,
             resource_id, response_code, response_body, lock_token, locked_at, created_at, updated_at;
   ```
3. **Exact Token Matching**:
   ```go
   if ik.Status == domain.IdempotencyStatusCompleted {
       return &ik, true, nil
   }
   if ik.LockToken != nil && *ik.LockToken == myLockToken {
       return &ik, false, nil // THIS caller acquired ownership
   }
   return nil, false, ErrIdempotencyConflict // Active lock held by another token
   ```
4. **Token-Guarded Completion & Release**:
   - `Complete`: Updates to `COMPLETED` checking `lock_token = $lockToken`.
   - `Release`: Deletes `IN_PROGRESS` row checking `lock_token = $lockToken`.

### Crash Recovery
If a process crashes while holding an `IN_PROGRESS` key, after 1 minute (`now() - INTERVAL '1 minute'`) subsequent callers atomically reclaim the lock by installing their own new `lock_token`. The key is never permanently poisoned.

### Test Evidence
- [`TestIdempotency_ActiveLockOwnershipRejected`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L804): Caller A acquires lockToken A; Caller B arriving while A is active is rejected with `ErrIdempotencyConflict`.
- [`TestIdempotency_StaleLockReclamation`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L824): Stale lock older than 1 minute is reclaimed with a new distinct token B.
- [`TestIdempotency_CompletedReturnsStoredResult`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L858): Completed operation returns stored response payload with `isCompleted = true`.
- [`TestConcurrency_IdempotencyRace`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L765): 5 concurrent requests resolve to exactly 1 `schedule_run` in the database.

---

## 3. P1-1 — Persist Dynamic Worker Lease

### Problem
The dynamic worker lease was computed in memory but never written to PostgreSQL. `ClaimQueued` continued writing the hardcoded default 5-minute lease to `schedule_runs.lease_expires_at`.

### Remediated Implementation
In [`application/internal/database/repositories/schedule_run_repo.go`](file:///c:/Users/Preetham%20S/timetable-platform/application/internal/database/repositories/schedule_run_repo.go#L111), `ClaimQueued` dynamically calculates and persists `lease_expires_at` directly in PostgreSQL during the atomic row update:

```sql
UPDATE schedule_runs
SET status = 'RUNNING',
    worker_id = $1,
    lease_expires_at = now() + (
      COALESCE(
        NULLIF((solver_config->>'timeoutSeconds')::int, 0),
        NULLIF((solver_config->>'maxDurationSeconds')::int, 0),
        GREATEST((solver_config->>'maxNodes')::int / 1000, 300),
        $2
      ) + 120 || ' seconds'
    )::INTERVAL,
    started_at = COALESCE(started_at, now()),
    heartbeat_at = now(),
    version = version + 1,
    updated_at = now()
WHERE id = (
  SELECT id
  FROM schedule_runs
  WHERE status = 'QUEUED'
  ORDER BY created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, timetable_id, institution_id, snapshot_id, status, solver_config,
          objective_config, seed, rule_set_hash, curra_version, curra_commit,
          worker_id, lease_expires_at, retry_count, version, created_by;
```

### Safety Invariant
For all claimed runs:
$$\text{lease\_expires\_at} = \text{now}() + \text{solver\_timeout} + 120\text{s (safety margin)}$$
- `timeoutSeconds: 600` (10 min) $\rightarrow$ lease is 12 minutes (720s).
- `maxDurationSeconds: 1800` (30 min) $\rightarrow$ lease is 32 minutes (1920s).
- Fallback / empty config $\rightarrow$ lease is 7 minutes (420s).

### Test Evidence
- [`TestWorker_DynamicLeasePersistence`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L893): Claiming a run with `timeoutSeconds = 600` produces `lease_expires_at > 700s` from now.
- [`TestWorker_LeaseSafetyMargin`](file:///c:/Users/Preetham%20S/timetable-platform/application/concurrency_test.go#L924): Verified dynamic lease duration across short (60s), long (1800s), maxDuration (900s), and fallback configs.

---

## 4. Regression Verification Summary

```text
======================================================================
1. go test -v ./... (application/)
======================================================================
=== RUN   TestCurraImportBoundary                          -> PASS (0.01s)
=== RUN   TestPublish_SubsequentPublishArchivesOldVersion  -> PASS (0.00s)
=== RUN   TestPublish_RollbackPreservesOldPublished        -> PASS (0.00s)
=== RUN   TestConcurrency_PublishRace                      -> PASS (0.00s)
=== RUN   TestConcurrency_IdempotencyRace                  -> PASS (0.05s)
=== RUN   TestIdempotency_ActiveLockOwnershipRejected      -> PASS (0.00s)
=== RUN   TestIdempotency_StaleLockReclamation             -> PASS (0.00s)
=== RUN   TestIdempotency_CompletedReturnsStoredResult     -> PASS (0.00s)
=== RUN   TestWorker_DynamicLeasePersistence               -> PASS (0.00s)
=== RUN   TestWorker_LeaseSafetyMargin                     -> PASS (0.00s)
=== RUN   TestConcurrency_MoveRace                         -> PASS (0.00s)
=== RUN   TestConcurrency_SwapRace                         -> PASS (0.00s)
=== RUN   TestConcurrency_StaleWorkerRejection             -> PASS (0.00s)
=== RUN   TestTransaction_FailureRollsBackEverything       -> PASS (0.00s)
=== RUN   TestHealthEndpoint                               -> PASS (0.00s)
=== RUN   TestPreconditionRequired_MissingIfMatch          -> PASS (0.00s)
=== RUN   TestConflict_VersionMismatch                     -> PASS (0.00s)
=== RUN   TestTenantIsolation_CrossTenantReturns404        -> PASS (0.00s)
=== RUN   TestAdapter_Solve_SimpleProblem                  -> PASS (0.03s)
=== RUN   TestAdapter_Verify                               -> PASS (0.00s)
=== RUN   TestAdapter_CompileConstraints                   -> PASS (0.00s)
=== RUN   TestOptimisticLocking_VersionConflict            -> PASS (0.00s)
=== RUN   TestVersionLifecycle_StateMachine                -> PASS (0.00s)
=== RUN   TestVersionLifecycle_StateMachineInvalidTransitions -> PASS (0.00s)
=== RUN   TestRBAC_VersionPermissions                      -> PASS (0.00s)
=== RUN   TestIdempotency_AtomicDeduplication              -> PASS (0.00s)
=== RUN   TestRunCancellation_RaceSafe                     -> PASS (0.00s)
=== RUN   TestWorker_ClaimAndExecute                       -> PASS (0.50s)
=== RUN   TestWorker_StaleWorkerRejected                   -> PASS (0.00s)
=== RUN   TestWorker_RetryCeilingReachesFailed             -> PASS (0.00s)
PASS: 100% of tests passed

======================================================================
2. go test ./... (root)
======================================================================
PASS: All scheduler tests, constraint tests, and solver tests pass.

======================================================================
3. go vet ./... (root and application/)
======================================================================
PASS: 0 issues reported.

======================================================================
4. git diff internal/scheduler/
======================================================================
VERIFIED: 0 modifications to frozen core solver.
```

---

## 5. Remaining Issues

None. All P0 and P1 audit findings have been resolved and verified with dedicated test suites.
