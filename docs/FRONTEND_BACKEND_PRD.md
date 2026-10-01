# CURRA: Frontend-Backend Integration PRD

## Executive Summary
CURRA is a timetable scheduling platform. It allows educational institutions to define academic master data (institutions, departments, programs, classes, faculty, rooms, courses), compile these inputs into scheduling problems, and trigger asynchronous solver jobs to generate timetables.

The backend currently supports:
- CRUD operations for all master data entities.
- Snapshot generation that freezes master data into a solver-ready "Problem".
- Asynchronous solve runs that execute a constraint-solving algorithm in a background worker.
- Versioning of solver results. Runs transition to Schedule Versions.
- Manual editing of draft schedules via "move" and "swap" operations that validate against the backend constraints.
- Independent verification of versions against original snapshots.
- A strict state machine for Schedule Versions (DRAFT → REVIEW → PUBLISHED → ARCHIVED).

The frontend must present these capabilities intuitively: guiding users through master data setup, managing the asynchronous solve lifecycle via polling, rendering a visually coherent interactive timetable for manual edits, and correctly communicating system states (e.g., verifying, draft, published).

## User Workflows

### 1. Master Data Management
1. User navigates to configuration screens (Rooms, Faculty, Classes, etc.).
2. Frontend fetches lists (`GET /institutions/{instId}/*`).
3. Frontend provides UI to create/update entities (`POST /institutions/{instId}/*`).
4. **State Transitions**: Immediate synchronous CRUD.

### 2. Create and Solve Timetable
1. User starts a new solve for a Timetable (`POST /timetables/{id}/snapshots` to freeze data).
2. Frontend creates a run (`POST /timetables/{id}/runs` with `snapshotId`).
3. Backend returns a `QUEUED` run.
4. Frontend polls run status (`GET /runs/{id}`).
5. Run enters `RUNNING`, then eventually `SOLVED` (or terminal failure).
6. Once `SOLVED`, user can review diagnostics and violations.
7. User converts `SOLVED` run into a Draft Version (`POST /timetables/{id}/versions`).

### 3. Timetable Editing (Draft Mode)
1. User selects a Draft Version (`GET /versions/{id}`).
2. Frontend renders the schedule grid.
3. User drags an assignment to a new slot/room.
4. Frontend sends `POST /versions/{id}/assignments/move` (with optimistic concurrency `If-Match`).
5. Backend validates the move synchronously. If valid, persists atomic change and returns updated Score.
6. Frontend updates UI with new score and assignment positions.

### 4. Publish / Archive Workflow
1. User reviews Draft and decides to submit for review (`POST /versions/{id}/submit-review`).
2. Version transitions to `REVIEW`. Edit operations (move/swap) are now forbidden.
3. Approver reviews and either sends back (`POST /versions/{id}/send-back`) or publishes (`POST /versions/{id}/publish`).
4. Publishing automatically executes a final verification check in the backend.
5. If published, the version transitions to `PUBLISHED` and becomes the authoritative timetable. Old published versions are automatically archived.

## API Contract

| Method | Endpoint | Purpose | Request | Response | Errors | UI usage |
| ------ | -------- | ------- | ------- | -------- | ------ | -------- |
| GET | `/auth/me` | Current user profile | Auth header | `User` object | 401 | Init session |
| GET | `/institutions/{instId}/*` | List master data | Path param | Array of objects | 401, 403, 404 | Data grids |
| POST | `/timetables` | Create timetable | Timetable info | `Timetable` object | 400 | Start new |
| POST | `/timetables/{id}/snapshots` | Freeze problem | None | `ProblemSnapshot` | 400 | Pre-solve |
| POST | `/timetables/{id}/runs` | Start solve | `snapshotId`, configs | `ScheduleRun` (QUEUED)| 400, 409 | Start job |
| GET | `/runs/{id}` | Poll solve status | None | `ScheduleRun` | 404 | Progress UI |
| POST | `/runs/{id}/cancel` | Cancel solve | None | `ScheduleRun` | 409 | Stop run |
| POST | `/timetables/{id}/versions` | Create draft | `snapshotId`, `sourceRunId` | `ScheduleVersion` | 400 | Save result |
| GET | `/versions/{id}` | Get version data | None | Version + Assignments | 404 | Render grid |
| POST | `/versions/{id}/assignments/move` | Move assignment | `AssignmentID`, `From`, `To` | Updated Score | 400, 409, 422| Drag & drop |
| POST | `/versions/{id}/assignments/swap` | Swap assignments| Two assignments/placements| Updated Score | 400, 409, 422| Swap action |
| POST | `/verify` | Independent verify| `snapshotId`, `versionId` | `VerificationReport` | 400 | Verify UI |
| POST | `/versions/{id}/submit-review` | Submit for review| `If-Match` header | `ScheduleVersion` | 409, 422 | Approvals |
| POST | `/versions/{id}/publish` | Publish version | `If-Match` header | `ScheduleVersion` | 409, 422 | Finalize |

*All modification endpoints on versions require `If-Match` header for optimistic locking.*

## Data Models

### Authoritative Backend Models
- **Institution / Department / Class / Room / Faculty**: Master data entities. Must not be mutated by frontend locally without API confirmation.
- **TimeSlot**: Contains `day`, `period`, `label`. The actual mapping of a period to clock time (e.g., 09:00 - 10:00) is implicitly derived by sequence or explicit labels.
- **ScheduleRun**: Contains solver state, violations, and diagnostics.
- **ScheduleVersion**: Derived from a Run, contains a snapshot of assignments. Has a `Score` containing `hardViolations` and `softPenalty`.
- **ScheduleAssignment**: Links a `CourseOfferingId`, `SessionRequirementId`, `StudentGroupId`, `FacultyId`, `RoomId`, and `TimeSlotId`.

### Client-derived Fields
- **Timetable Conflicts**: The frontend must map `violations` (from `Score`) back to specific `ScheduleAssignment`s to visually highlight conflicts (e.g., red borders on colliding blocks).
- **Time Grid Layout**: The UI must derive column (Days) and row (Periods) structures from the `TimeSlot` collection.

## State Machines

### Schedule Run (Solve Jobs)
```text
QUEUED → RUNNING → SOLVED
   ↓        ↓          ↓
 CANCELLED / FAILED / INFEASIBLE / DEADLINE_EXCEEDED
```

| State | Meaning | Allowed actions | Frontend behavior |
| ----- | ------- | --------------- | ----------------- |
| `QUEUED` | Job waiting in worker queue | Cancel | Show spinner/queue position. Poll. |
| `RUNNING` | Solver is actively working | Cancel | Show progress/timer. Poll. |
| `SOLVED` | Job completed successfully | Create Version | Show success, diagnostics, and "Save" button. |
| `FAILED` | Internal error | Retry | Show error message. |
| `INFEASIBLE`| No solution possible | Retry with relaxed constraints | Show diagnostic failure reason. |

### Schedule Version
```text
DRAFT ↔ REVIEW → PUBLISHED → ARCHIVED
  ↓
ARCHIVED
```

| State | Meaning | Allowed actions | Forbidden actions |
| ----- | ------- | --------------- | ----------------- |
| `DRAFT` | Editable work in progress | Move, Swap, Pin, SubmitReview, Archive | Publish, SendBack |
| `REVIEW` | Pending approval | Publish, SendBack | Move, Swap, Pin, SubmitReview, Archive |
| `PUBLISHED` | Active authoritative timetable | Archive | Move, Swap, Pin, Publish, SendBack |
| `ARCHIVED` | Historical record | None | All mutations |

## Asynchronous Solve Job Behavior
The backend performs asynchronous solves in a background worker, and communicates state exclusively via **Database Polling**.
- **No SSE or WebSockets are exposed.**
- The frontend must issue a `POST /timetables/{id}/runs` which returns a job ID and status `QUEUED`.
- The frontend must begin polling `GET /runs/{id}`.
- Recommended polling interval: **2-3 seconds**.
- **Terminal States**: Polling should stop immediately when `status` is `SOLVED`, `FAILED`, `INFEASIBLE`, `INVALID_PROBLEM`, `INVALID_RESULT`, `CANCELLED`, `DEADLINE_EXCEEDED`, or `NODE_LIMIT`.
- **Browser Refresh**: If the user refreshes, the frontend should fetch recent runs (`GET /timetables/{id}/runs`), see any in `RUNNING`/`QUEUED`, and resume polling.

## Verification Model
Verification is independent of solver execution. It confirms that a manual set of assignments is valid against the constraints of a snapshot.
- The `move` and `swap` endpoints perform synchronous validation, meaning the backend guarantees versions don't enter an invalid state during normal editing.
- `POST /verify` exists to independently verify a version (`versionId`) against a snapshot (`snapshotId`).
- A solution is invalid if `valid: false` or `hardViolations > 0`.
- The frontend should show a "Verify" button or state. It should mark verification as "stale" or simply rely on the synchronous score updates from moves.

## Timetable Editing
- **Move/Swap**: `POST /versions/{id}/assignments/move` and `swap`.
- **Target**: DRAFT versions only.
- **Concurrency**: Must pass `If-Match: "{versionNumber}"`. If a 409 Conflict occurs, the frontend must reload the version data.
- **Side effects**: A successful move returns the new constraint validation response. The backend atomically applies this change. The frontend must update the visual score and re-render the assignments.

## Publish / Archive / Versioning
- Users cannot edit `REVIEW`, `PUBLISHED`, or `ARCHIVED` versions. The UI must render these as read-only grids.
- Only one version should logically be "Current Published", which can be found via `currentPublishedVersionId` on the `Timetable` object.
- The backend automatically handles transitioning the old published version to `ARCHIVED` when a new version is `PUBLISHED` (via `PublishTx`).

## Solution / Timetable Display Contract
The frontend needs to render a 2D matrix (Days vs Time Slots).
- **Dimensions**: Rows = `TimeSlot` periods. Columns = `TimeSlot` days.
- **Cards**: Each cell block is an `Assignment`.
- **Views**: The UI must support pivoting the grid. A user needs to see the timetable *by Room*, *by Faculty*, or *by Student Group*.
- **Conflicts**: If `Violations` exist for an assignment, the UI must visually flag it (e.g., overlapping assignments for the same faculty).

## Error & Validation Contract
- **System Errors (5xx)**: Show generic "Something went wrong" toast.
- **Validation Errors (400, 422)**: Extract the `details` array. Map field errors to form inputs.
- **Concurrency Errors (409)**: Show "Data has changed, please refresh".
- **Domain Errors (Infeasible)**: The problem constraints are mathematically impossible. The user must relax constraints (e.g., add more rooms, loosen faculty unavailability).

## Loading / Empty / Error / Partial States
| State | Representation |
| ----- | -------------- |
| Timetable Workspace (Empty) | Call to action: "Define your master data before starting." |
| Solve in Progress | Non-blocking bottom sheet or overlay showing elapsed time. |
| Timetable Grid (Loading) | Skeleton loader matching the grid dimensions. |
| Version (Read Only) | Remove drag-and-drop handles, hide edit actions. Banner indicating state (e.g., "Pending Review"). |

## Authentication / Authorization
- Authentication is handled via JWT (`Bearer` token) obtained from `/auth/google`.
- The API contract specifies `BearerAuth` for almost all routes.
- The frontend must include `Authorization: Bearer <token>`.
- The `InstitutionId` is derived from context/user profile or explicitly passed.

## Realtime / Refresh Strategy
- **Master Data**: Fetch once on load. Manual refresh not usually necessary unless multi-user edits are frequent.
- **Solve Jobs**: Polling (2-3s interval).
- **Timetable Versions**: Refetch entirely on `409 Conflict`. Otherwise, apply the optimistic update from the `Move/Swap` response to avoid full refetching.

## Frontend Types
### API Types
- `ScheduleRun`: (id, status, violations, diagnostics, score)
- `ScheduleVersion`: (id, status, name, score)
- `ScheduleAssignment`: (id, versionId, roomId, timeSlotId, etc.)

### UI State Types
- `GridPivot`: enum (`ROOM`, `FACULTY`, `STUDENT_GROUP`).
- `DragItem`: state representing an assignment currently being dragged.
- `ValidationOverlay`: state mapping backend violations to UI elements.

## API Client Requirements
- **JSON Serialization**
- **Idempotency Keys**: Critical for `POST /timetables/{id}/runs` and `POST /timetables/{id}/versions` to prevent duplicate solves/versions on retry. The frontend must generate a UUID and pass it in the `Idempotency-Key` header for these mutations.
- **Optimistic Locking**: Client must track the `version` integer of `ScheduleVersion` and send it as `If-Match: <version>` for edit operations.
- **Polling Utility**: Built-in support for polling with backoff/abort signals.

## Backend Gaps / Frontend Blockers
| Gap | Why frontend needs it | Current backend behavior | Minimal backend change required | Priority |
| --- | --------------------- | ------------------------ | ------------------------------- | -------- |
| Dry Run for Moves | To allow real-time hover feedback while dragging, before dropping. | Exists in service layer (`dryRun` boolean) but not exposed in the OpenAPI schema or handlers. | Expose `?dryRun=true` on move/swap endpoints to return the validation result without committing. | P2 |
| Server Sent Events | Polling is inefficient for long-running solves. | Polling only. | Implement SSE at `/runs/{id}/events`. | P3 |
| Paginated Lists | Master data might grow large. | Most GET endpoints return unpaginated arrays. | Add `page` and `limit` to master data. | P3 |

## Frontend-Backend Contract Table

| Frontend Capability | Backend Endpoint(s) | Required Data | Mutation | Async? | Error Cases | Backend Gap? |
| ------------------- | ------------------- | ------------- | -------- | ------ | ----------- | ------------ |
| Start Solve | `POST /timetables/{id}/runs` | `snapshotId`, configs | Yes | Yes | 400 | No |
| Poll Job | `GET /runs/{id}` | - | No | No | 404 | No (Polling used) |
| Save Draft | `POST /timetables/{id}/versions` | `snapshotId`, `sourceRunId`| Yes | No | 409 | No |
| Edit Assignment | `POST /versions/{id}/assignments/move` | `If-Match`, Placements | Yes | No | 400, 409 | Missing dryRun |
| Review Submit | `POST /versions/{id}/submit-review` | `If-Match` | Yes | No | 409 (Lock) | No |
| Publish | `POST /versions/{id}/publish` | `If-Match` | Yes | No | 409, 422 | No |

## Screen-Level Requirements
### Dashboard / Timetable List
- **Purpose**: Show existing timetables and active runs.
- **API**: `GET /timetables`, `GET /timetables/{id}/runs`.
- **Navigation**: Click timetable to enter Workspace.

### Timetable Workspace (Solver View)
- **Purpose**: Prepare data, trigger solve, monitor progress.
- **API**: `POST /timetables/{id}/snapshots`, `POST /runs`, `GET /runs/{id}`.
- **Dependencies**: Must wait for `SOLVED` to proceed.

### Interactive Grid (Draft View)
- **Purpose**: Render the schedule, drag and drop to resolve soft conflicts.
- **API**: `GET /versions/{id}`, `POST /move`, `POST /swap`.
- **State**: Requires `GridPivot` state. Requires complex collision detection on the client (or relying entirely on backend response).

## Frontend Agent Handoff
1. The frontend **MUST** support optimistic concurrency via the `If-Match` header when editing versions.
2. The frontend **MUST** use polling (not SSE) to track background solver jobs (`GET /runs/{id}`).
3. The frontend **MUST** supply `Idempotency-Key` headers for job creation and version creation to prevent duplicate side-effects.
4. The frontend **MUST NOT** allow edit operations on versions unless their status is explicitly `DRAFT`.
5. The frontend **MAY** derive conflict visualization purely by mapping the backend's `Violations` payload to UI cell boundaries.
6. The `dryRun` capability for live-drag feedback is currently a backend gap. Standard drag-and-drop should commit on drop, reverting visually if a 4xx error occurs.
7. Verification is handled natively during move/swap operations. The backend guarantees that an atomic move won't leave the timetable in an invalid DB state regarding hard constraints.
