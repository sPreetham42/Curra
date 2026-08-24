# State Machines

## 1. ScheduleRun State Machine

A ScheduleRun represents a single execution of the CURRA solver against a ProblemSnapshot.

### States

| State | Meaning |
|---|---|
| `QUEUED` | Run is waiting for an available worker. |
| `RUNNING` | Solver is actively executing. |
| `SOLVED` | Feasible, verified timetable found. |
| `INFEASIBLE` | Problem is provably unsolvable. |
| `INVALID_PROBLEM` | Snapshot failed structural validation or pre-solve checks. |
| `INVALID_RESULT` | Solution failed independent verification (engine bug). |
| `CANCELLED` | User or system cancelled before completion. |
| `DEADLINE_EXCEEDED` | Solver time limit reached without completing. |
| `NODE_LIMIT` | CSP node limit reached without finding a solution. |
| `FAILED` | Unexpected error (panic, OOM, infrastructure failure). |

### Transitions

```
QUEUED ──────→ RUNNING ──────→ SOLVED
                 │               INFEASIBLE
                 │               INVALID_PROBLEM
                 │               INVALID_RESULT
                 │               CANCELLED
                 │               DEADLINE_EXCEEDED
                 │               NODE_LIMIT
                 │               FAILED
                 │
                 └──→ (on worker crash) FAILED
                 
QUEUED ──→ CANCELLED (user cancels before start)
```

### Rules

- **Terminal states:** SOLVED, INFEASIBLE, INVALID_PROBLEM, INVALID_RESULT, CANCELLED, DEADLINE_EXCEEDED, NODE_LIMIT, FAILED. No transitions out.
- **QUEUED → RUNNING:** Only one worker claims a run. Use PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED` or `UPDATE ... WHERE status = 'QUEUED'`.
- **RUNNING → terminal:** Only the owning worker can transition.
- **Retry:** A FAILED or CANCELLED run may be retried by creating a new ScheduleRun with the same snapshot_id. Never modify the existing run.
- **Heartbeat:** While RUNNING, the worker updates `heartbeat_at` every 30 seconds. If heartbeat is stale (>120s), a reaper can transition to FAILED.
- **Idempotency:** Creating a run with the same snapshot_id and solver_config within 5 seconds returns the existing QUEUED/RUNNING run.
- **Duplicate prevention:** If a SOLVED run already exists for this snapshot with identical config, return the existing result instead of creating a new run.
- **Crash recovery:** If the worker process dies while RUNNING, the heartbeat stale check transitions the run to FAILED. A new run can be created.

### Solver Status Mapping

| CURRA Status | ScheduleRun Status | Notes |
|---|---|---|
| SOLVED | SOLVED | After independent verification passes |
| INFEASIBLE | INFEASIBLE | Pre-solve or CSP proved no solution |
| INVALID_PROBLEM | INVALID_PROBLEM | Snapshot validation failed |
| INVALID_RESULT | INVALID_RESULT | Verification failed after solver reported success |
| CANCELLED | CANCELLED | Context cancelled by user |
| DEADLINE_EXCEEDED | DEADLINE_EXCEEDED | Time limit hit |
| NODE_LIMIT | NODE_LIMIT | Node limit hit |

---

## 2. ScheduleVersion State Machine

A ScheduleVersion is an immutable timetable result that users can review, edit, and publish.

### States

| State | Meaning |
|---|---|
| `DRAFT` | Editable. Can be modified, validated, or deleted. |
| `REVIEW` | Submitted for review. Read-only to the author, editable by reviewers. |
| `PUBLISHED` | Active, live timetable. Read-only to everyone. |
| `ARCHIVED` | Historical record. Read-only, hidden from active views. |

### Transitions

```
DRAFT ──→ REVIEW ──→ PUBLISHED ──→ ARCHIVED
  │          │
  │          └──→ DRAFT (sent back for revisions)
  │
  └──→ ARCHIVED (abandoned draft)

PUBLISHED ──→ DRAFT (creates a new draft version, not in-place edit)
```

### Rules

- **Valid transitions:**
  - DRAFT → REVIEW: Author or admin submits for review.
  - REVIEW → PUBLISH: Reviewer or admin publishes.
  - REVIEW → DRAFT: Reviewer sends back with feedback.
  - DRAFT → ARCHIVED: Author or admin abandons.
  - PUBLISHED → ARCHIVED: Admin archives an old version.
  - PUBLISHED → DRAFT: Creates a NEW version (copy-on-write). The published version stays PUBLISHED.

- **Invalid transitions:**
  - REVIEW → ARCHIVED: Must go through DRAFT first.
  - ARCHIVED → anything: Archived versions are frozen.
  - Direct creation at PUBLISHED: A version must go through DRAFT.

- **Published content is immutable:** Once PUBLISHED, the assignments and score are frozen. No field on a PUBLISHED version can be modified. Any edit creates a new DRAFT.

- **Copy-on-write editing:** Editing a PUBLISHED version:
  1. Create a new DRAFT version with the same assignments (deep copy).
  2. Apply edits to the new DRAFT.
  3. The original PUBLISHED version remains untouched.

- **Current published pointer:** Each Timetable has at most one PUBLISHED version, tracked by `current_published_version_id`. When a new version is published:
  1. If a previous version was PUBLISHED, transition it to ARCHIVED.
  2. Set the new version to PUBLISHED.
  3. Update the pointer.

- **Optimistic concurrency:** Every state transition requires checking `version = expected_version`. If version mismatch, return HTTP 409.

- **Rollback:** To "roll back" a published timetable, publish a previous ARCHIVED version (transition ARCHIVED → PUBLISHED). This creates a new DRAFT from the archived version, then publishes it.

---

## 3. ImportBatch State Machine

### States

| State | Meaning |
|---|---|
| `PENDING` | Upload received, not yet processed. |
| `PARSING` | File is being parsed and validated. |
| `STAGED` | Rows parsed and stored in staging table. |
| `VALIDATING` | Business validation running against CURRA rules. |
| `READY` | All rows valid. Ready for user to preview and commit. |
| `COMMITTED` | User confirmed. Data merged into live academic records. |
| `FAILED` | Parsing or validation error. |
| `CANCELLED` | User cancelled the import. |

### Transitions

```
PENDING → PARSING → STAGED → VALIDATING → READY → COMMITTED
                │              │
                └──→ FAILED    └──→ FAILED
                
CANCELLED can be reached from: PENDING, PARSING, STAGED, VALIDATING, READY
```

### Rules

- No retry — create a new import batch.
- Staging data is temporary and cleaned up after COMMITTED or FAILED.
