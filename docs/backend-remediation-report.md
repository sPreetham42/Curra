# CURRA Backend v1 — Post-Audit Remediation Report

---

## 1. P0-1 — Atomic Move / Swap / Publish / Worker Persistence

### Problem Identified in Audit
Previously, assignment replacement, version optimistic locking updates (CAS), and audit event writes were executed across separate database operations. If the version CAS check failed or the server crashed mid-way, assignments were already committed in `schedule_assignments`, resulting in state corruption.

### Remediated Transaction Design

#### Move and Swap (`ApplyAssignmentUpdateTx`)
All write operations execute inside a single explicit PostgreSQL transaction:
```sql
BEGIN;
  -- 1. Replace existing assignments for version
  DELETE FROM schedule_assignments WHERE version_id = $1;
  -- 2. Insert new assignment batch
  INSERT INTO schedule_assignments (...) VALUES (...);
  -- 3. Optimistic concurrency CAS version check and score bump
  UPDATE schedule_versions
  SET score = $1, version = version + 1, updated_at = now()
  WHERE id = $2 AND version = $3;
  -- If RowsAffected == 0 -> ROLLBACK and return ErrOptimisticLock (HTTP 409 Conflict)
  -- 4. Transactional audit log
  INSERT INTO audit_events (...) VALUES (...);
COMMIT;
```

#### Publishing (`PublishTx`)
```sql
BEGIN;
  -- 1. CAS status update to PUBLISHED
  UPDATE schedule_versions
  SET status = 'PUBLISHED', version = version + 1, updated_at = now()
  WHERE id = $1 AND version = $2;
  -- If RowsAffected == 0 -> ROLLBACK and return ErrOptimisticLock (HTTP 409 Conflict)
  -- 2. Update timetable current published pointer
  UPDATE timetables SET current_published_version_id = $1, updated_at = now() WHERE id = $2;
  -- 3. Audit log
  INSERT INTO audit_events (...) VALUES (...);
COMMIT;
```

#### Worker Terminal Persistence (`CommitTerminalResultTx`)
The CURRA solve itself executes strictly **outside** the database transaction without holding any database connections. After solver completion:
```sql
BEGIN;
  -- 1. Stale worker check and terminal status update
  UPDATE schedule_runs
  SET status = $3, result = $4, score = $5, diagnostics = $6, violations = $7,
      duration_ms = $8, curra_version = $9, curra_commit = $10, rule_set_hash = $11,
      finished_at = now(), updated_at = now(), version = version + 1
  WHERE id = $1 AND status = 'RUNNING' AND worker_id = $2;
  -- If RowsAffected == 0 -> ROLLBACK and return ErrStaleWorker
  -- 2. If status == 'SOLVED', create draft version & batch insert assignments
  INSERT INTO schedule_versions (...) VALUES (...);
  INSERT INTO schedule_assignments (...) VALUES (...);
  -- 3. Audit log
  INSERT INTO audit_events (...) VALUES (...);
COMMIT;
```

### Proof of Rollback
Tested via `TestTransaction_FailureRollsBackEverything` in `application/concurrency_test.go`:
- Injected CAS failure during `Move`.
- Verified that assignment replacements, version metadata updates, and audit event logs were completely reverted to their pre-transaction state.

---

## 2. P0-2 — Atomic Idempotency

### Problem Identified in Audit
The prior design performed `SELECT` followed by resource creation and subsequent key insertion. Under concurrent identical requests, two distinct `schedule_runs` or `schedule_versions` were created.

### Remediated Database-Backed Design
- Schema enhanced in `migrations.go` with `status`, `locked_at`, and `updated_at`:
  ```sql
  CREATE TABLE IF NOT EXISTS idempotency_keys (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
      idempotency_key TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'IN_PROGRESS'
          CHECK (status IN ('IN_PROGRESS', 'COMPLETED', 'FAILED')),
      resource_type TEXT NOT NULL,
      resource_id UUID,
      response_code INT,
      response_body JSONB,
      locked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      UNIQUE (institution_id, idempotency_key)
  );
  ```
- **Atomic Acquisition (`IdempotencyRepo.Acquire`)**:
  Performs `INSERT ... ON CONFLICT (institution_id, idempotency_key) DO UPDATE SET locked_at = CASE ...`
  - If `status == 'COMPLETED'`: returns the cached response immediately.
  - If newly acquired: caller proceeds to create the resource and calls `Complete(...)`.
  - If active lock held by another in-flight caller: caller polls with exponential backoff until the primary request completes and returns the identical resource.
  - Crash recovery: stale locks older than 1 minute are automatically reclaimed without permanently poisoning the key.
  - Failure recovery: if resource creation fails, `Release(...)` immediately frees the key for clean client retries.

### Concurrency Test Results
Tested via `TestConcurrency_IdempotencyRace` in `application/concurrency_test.go`:
- Dispatched 5 concurrent requests with identical `(institution_id, idempotency_key)`.
- Outcome: exactly 1 `schedule_run` row created in the database, all 5 goroutines resolved to the identical run ID.

---

## 3. P1-1 — Dynamic Worker Lease

### Problem Identified in Audit
Worker lease was hardcoded to `5 * time.Minute`, risking premature expiry for large solver runs.

### Remediated Calculation
Implemented `calculateLeaseDuration(solverConfigJSON)` in `application/internal/worker/worker.go`:
```go
lease = solver maximum execution window + fixed safety margin
```
- Inspects `timeoutSeconds`, `maxDurationSeconds`, or calculates an estimate based on `maxNodes`.
- Adds a default safety margin of `2 * time.Minute`.
- Fallback: `5 * time.Minute + 2 * time.Minute = 7 * time.Minute`.

---

## 4. P1-2 — Retry Counter & Expired Run Recovery

### Problem Identified in Audit
Direct claiming of expired `RUNNING` rows in `ClaimQueued` could bypass the retry ceiling.

### Remediated Semantics
1. `ClaimQueued` strictly claims rows where `status = 'QUEUED'`.
2. Expired `RUNNING` rows (`status = 'RUNNING' AND lease_expires_at < now()`) are reclaimed strictly through `RecoverExpired(ctx, maxRetries)`:
   - For `retry_count < maxRetries`:
     `UPDATE schedule_runs SET status = 'QUEUED', retry_count = retry_count + 1, worker_id = NULL, lease_expires_at = NULL ...`
   - For `retry_count >= maxRetries`:
     `UPDATE schedule_runs SET status = 'FAILED', finished_at = now(), diagnostics = '{"message":"lease expired and exceeded maximum retry limit"}' ...`
3. Verified via `TestWorker_RetryCeilingReachesFailed`:
   `RUNNING` $\rightarrow$ lease expires $\rightarrow$ `retry_count=1, QUEUED` $\rightarrow$ `RUNNING` $\rightarrow$ lease expires $\rightarrow$ `retry_count=2, QUEUED` $\rightarrow$ `RUNNING` $\rightarrow$ lease expires $\rightarrow$ terminal `FAILED`.

---

## 5. P1-3 — Version State Machine Correction

### Problem Identified in Audit
`VersionService.Archive` permitted `REVIEW -> ARCHIVED` directly, violating `docs/state-machines.md` rule requiring `REVIEW` to transition back to `DRAFT` before archiving.

### Remediated Implementation
In `application/internal/services/version.go`:
```go
if ver.Status == domain.VersionStatusReview {
    return domain.ScheduleVersion{}, fmt.Errorf("%w: cannot archive version in REVIEW status (must send back to DRAFT first)", ErrInvalidState)
}
if ver.Status != domain.VersionStatusDraft && ver.Status != domain.VersionStatusPublished {
    return domain.ScheduleVersion{}, fmt.Errorf("%w: can only archive DRAFT or PUBLISHED versions", ErrInvalidState)
}
```
Tested via `TestVersionLifecycle_StateMachineInvalidTransitions`.

---

## 6. P1-4 — OpenAPI / Router Endpoint Reconciliation

### Reconciled Route Table

| HTTP Method | OpenAPI Route | Router Implementation | Status |
|---|---|---|---|
| `POST` | `/api/v1/versions/{id}/submit-review` | Mounted (`/submit-review` + `/review` alias) | Aligned |
| `GET` | `/api/v1/snapshots/{id}/problem` | Mounted `GetProblemJSON` handler | Aligned |
| `GET` | `/api/v1/versions/{id}/assignments` | Mounted `ListAssignments` handler | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/departments` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/programs` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/classes` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/student-groups` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/subjects` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/faculty` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/rooms` | Mounted with context tenant validation | Aligned |
| `GET` / `POST` | `/api/v1/institutions/{instId}/time-slots` | Mounted with context tenant validation | Aligned |

---

## 7. P2-1 — RBAC Authorization Checks

### Remediated Authorization Matrix
Added explicit `RequireRole` checks in `VersionService`:
- `SubmitReview`: `RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler)`
- `Archive`: `RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler)`

Tested via `TestRBAC_VersionPermissions`:
- Unauthenticated (empty ctx): rejected.
- Wrong tenant: returns HTTP 404 (via `RequireTenantMatch`).
- Authenticated `VIEWER`: returns HTTP 403 `ErrForbidden`.
- Authenticated `SCHEDULER`: succeeds.
- Authenticated `INSTITUTION_ADMIN`: succeeds.

---

## 8. Concurrency & Integration Tests Summary

| Test Case | Scenario | Result |
|---|---|---|
| `TestConcurrency_MoveRace` | 2 concurrent writers moving on Version 7 | Exactly 1 committed, 1 got 409 Conflict, no losing assignments persisted. |
| `TestConcurrency_SwapRace` | 2 concurrent writers swapping on Version 1 | Exactly 1 committed, 1 got 409 Conflict. |
| `TestConcurrency_PublishRace` | 2 concurrent admin publish calls on Version 3 | Exactly 1 succeeded, 1 got 409 Conflict, timetable pointer set. |
| `TestConcurrency_IdempotencyRace` | 5 concurrent requests with identical key | Exactly 1 run created, all 5 returned the identical run ID. |
| `TestConcurrency_StaleWorkerRejection` | Stale Worker A attempts commit after lease transfer | Worker A rejected with `ErrStaleWorker`, Worker B result preserved. |
| `TestTransaction_FailureRollsBackEverything` | Forced CAS failure between assignment and version update | Full atomic rollback (assignments, version, audit event unmutated). |
| `TestWorker_RetryCeilingReachesFailed` | Worker lease expiration cycle until ceiling | Incremented retry count, transitioned to `FAILED` at ceiling. |

---

## 9. Verification Commands & Results

1. **Unit & Concurrency Tests**:
   - `go test -v ./...` in `application/`: **PASS (100%)**
   - `go test ./...` at root: **PASS (100%)**
2. **Go Vet**:
   - `go vet ./...` in `application/`: **PASS (0 issues)**
   - `go vet ./...` at root: **PASS (0 issues)**
3. **AST Import Boundary**:
   - `go test -v -run TestCurraImportBoundary ./application/...`: **PASS (0 violations)**
4. **CURRA Engine Diff**:
   - `git diff internal/scheduler/`: **Verified 0 changes introduced by remediation**.
5. **Static Analysis Environment Note**:
   - `golangci-lint` binary is not installed in the local Windows execution environment; AST-based AST verification in `application/boundary_test.go` and `go vet` were used for automated boundary and syntax verification.

---

## 10. Remaining Issues

1. **Live PostgreSQL Infrastructure**: Tests were executed against in-memory transactional database simulating PostgreSQL row-level locks and CAS operations; end-to-end multi-process verification against a live multi-node PostgreSQL instance requires a deployed PostgreSQL environment.
