# CURRA TIMETABLE PLATFORM — ARCHITECTURAL DOSSIER FOR EXTERNAL AUDIT

Generated: 2026-08-30
Repository: `sPreetham42/Curra` (module `github.com/sPreetham42/timetable-platform`)
Commit / Working-Tree State: Clean core build v1.0.0; modified soft objective scoring suite (Room Change Minimization implemented with full differential parity and verifier decoupling).
Primary Languages: Go 1.26.6 (Engine & Application Service), PostgreSQL 16+ SQL (Database Schema).
Purpose: Standalone, comprehensive architectural handoff document for external AI architect (Claude).

---

# 1. PROJECT IDENTITY

CURRA is an enterprise-grade automated academic timetabling platform. It solves complex NP-hard academic scheduling problems by assigning course sessions to recurring time slots and physical rooms across multi-week terms while honoring hard feasibility constraints (such as faculty, room, and student group availability and conflict rules) and optimizing soft preferences (such as student gap minimization, faculty slot preferences, and room change minimization).

The system consists of two major sub-systems:
1. **Core Scheduling Engine (`internal/scheduler`)**: A pure, stateless, zero-IO Go package providing constraint compilation, CSP backtracking search with forward checking and variable heuristics (MRV, Degree, LCV), Tabu Search local optimization with incremental delta evaluation, and an authoritative pure-function verifier.
2. **Application Service (`application/`)**: A multi-tenant REST API server (Go `net/http` / `gorilla/mux`) and asynchronous worker daemon backed by PostgreSQL 16+. It handles identity/RBAC, academic catalog persistence, problem snapshot creation, job queuing, optimistic concurrency version control (Draft/Review/Published state machine), manual move/swap validation, and audit logging.

- **Go Version**: 1.26.6
- **Database**: PostgreSQL 16+ (pgx/v5 driver with `jackc/puddle` connection pooling)
- **External Dependencies**: Zero runtime dependencies for core scheduler. Application dependencies: `github.com/gorilla/mux` v1.8.1, `github.com/jackc/pgx/v5` v5.6.0, `github.com/google/uuid` v1.6.0.
- **Frontend**: None present in codebase (`OBSERVED`).

---

# 2. REPOSITORY TREE

```text
timetable-platform/
├── api/                                # API specifications & fixtures
│   ├── fixtures/                       # Synthetic problem benchmark generators
│   └── openapi.yaml                    # OpenAPI 3.0 specification
├── application/                        # Web application service & worker daemon
│   ├── cmd/
│   │   ├── server/                     # API server entrypoint (main.go)
│   │   └── worker/                     # Asynchronous solver worker entrypoint (main.go)
│   ├── internal/
│   │   ├── api/                        # HTTP routing, middleware, handlers
│   │   │   ├── handlers/               # HTTP request handlers & DTO mapping
│   │   │   ├── middleware/             # Auth & logging middleware
│   │   │   └── router.go               # net/http ServeMux routing table
│   │   ├── curra/                      # CURRA Engine Adapter (Strict boundary wrapper)
│   │   │   ├── adapter.go              # Adapter implementation
│   │   │   └── types.go                # Application-facing DTOs
│   │   ├── database/                   # Database connection & SQL repositories
│   │   │   └── repositories/           # Postgres repository implementations
│   │   ├── domain/                     # Application domain entities
│   │   ├── services/                   # Application business logic services
│   │   └── worker/                     # Background job polling & solver execution
│   ├── boundary_test.go                # Import boundary verification tests
│   ├── concurrency_test.go             # Optimistic locking & concurrent edit tests
│   └── go.mod                          # Application module file
├── cmd/
│   └── solver/                         # CLI entrypoint for standalone solver execution
├── contracts/                          # Architecture contract specifications
│   ├── api-contract.md                 # REST API specification
│   └── curra-adapter.md                # Adapter boundary contract
├── database/
│   └── schema.md                       # PostgreSQL DDL schema & table documentation
├── docs/                               # Additional documentation & specifications
├── internal/                           # Core CURRA scheduling engine (pure Go)
│   └── scheduler/
│       ├── constraints/                # Hard constraint compilation & evaluation
│       ├── diagnostics/                # Solve status, violations, and performance metrics
│       ├── engine/                     # Pipeline orchestrator (Solve pipeline)
│       ├── model/                      # Core domain entities & TimeSlot keying
│       ├── problem/                    # Problem definition, Solution index, Moves
│       ├── scorer/                     # Soft objective penalty calculators
│       ├── solver/
│       │   ├── backtracking/           # CSP solver with forward checking & heuristics
│       │   └── localsearch/            # Tabu Search with incremental score evaluator
│       ├── testutil/                   # Problem generation utilities for testing
│       └── verifier/                   # Authoritative pure-function verifier
├── tests/                              # Integration, red-team fuzz, & benchmark tests
├── go.mod                              # Root engine module file
└── README.md                           # System overview documentation
```

### Directory Purposes
- `internal/scheduler`: Frozen, self-contained core engine. Imports no external packages and performs zero disk or network I/O.
- `application`: Multi-tenant web platform and background worker daemon. Depends on core engine via `internal/curra/adapter.go`.
- `contracts`: Defines immutable architectural boundaries between application and engine.
- `api/openapi.yaml`: Formal API contract specification for client consumption.
- `database/schema.md`: PostgreSQL schema specification enforcing multi-tenancy via `institution_id` FKs and optimistic concurrency via `version` counters.

---

# 3. ARCHITECTURE

### Layered Architecture Diagram

```text
                       [ External HTTP Clients / Frontend ]
                                        │
                                        ▼ (HTTP / REST)
                       [ Application HTTP Router ]
                        (application/internal/api)
                                        │
                                        ▼
                      [ Business Logic Services ]
                    (application/internal/services)
                                        │
                    ┌───────────────────┴───────────────────┐
                    ▼                                       ▼
       [ Database Repositories ]               [ CURRA Engine Adapter ]
   (application/internal/database)             (application/internal/curra)
                    │                                       │
                    ▼                                       ▼
          [ PostgreSQL 16+ ]                    [ Core Engine Pipeline ]
                                                   (internal/scheduler)
                                                            │
                                            ┌───────────────┼───────────────┐
                                            ▼               ▼               ▼
                                          [CSP]          [Tabu]        [Verifier]
```

### Architectural Invariants
1. **Import Boundary**: Application packages (`api`, `services`, `repositories`) MUST NOT import `internal/scheduler` packages directly. All engine interactions pass through `curra.Adapter` (`application/internal/curra/adapter.go`), enforced by automated boundary tests (`application/boundary_test.go`).
2. **Core Determinism**: The solver engine is pure and deterministic. Given identical `Problem`, `Seed`, `ObjectiveConfig`, and `ConstraintInstances`, solver execution produces identical assignments, score breakdowns, and diagnostics.
3. **Authoritative Verifier Independence**: The verifier (`internal/scheduler/verifier`) independently calculates soft scores directly from raw solution assignments and problem definitions. It NEVER calls production scorer implementations.
4. **Optimistic Locking**: All database entity updates and schedule version mutations enforce strict version increment checks (`WHERE id = $1 AND version = $2`).

---

# 4. COMPLETE DOMAIN MODEL

### 1. `model.TimeSlot`
- **File**: `internal/scheduler/model/timeslot.go`
- **Fields**: `ID TimeSlotID`, `Day time.Weekday`, `Period int`, `Label string`
- **Key**: `SlotKey{Day, Period}` maps recurring weekly slots.
- **Used by**: All grid assignment indexing and consecutive slot expansion.

### 2. `model.StudentGroup` & `model.Class`
- **File**: `internal/scheduler/model/entities.go`
- **Fields (`Class`)**: `ID ClassID`, `ProgramID ProgramID`, `Name string`, `WholeGroupID StudentGroupID`, `StudentGroupIDs []StudentGroupID`
- **Fields (`StudentGroup`)**: `ID StudentGroupID`, `ClassID ClassID`, `Name string`, `Size int`
- **Semantics**: `WholeGroupID` represents the entire class cohort. `StudentGroupIDs` contains subgroups (e.g. lab sections). `Problem.BuildStudentGroupOverlaps()` computes overlap matrices so whole-group and subgroup sessions do not clash.

### 3. `model.CourseOffering` & `model.SessionRequirement`
- **File**: `internal/scheduler/model/entities.go`
- **Fields (`CourseOffering`)**: `ID CourseOfferingID`, `TermID TermID`, `ClassID ClassID`, `SubjectID SubjectID`, `StudentGroupID StudentGroupID`, `FacultyID FacultyID`, `RequiredRoomFeatureIDs []RoomFeatureID`, `SessionRequirementIDs []SessionRequirementID`
- **Fields (`SessionRequirement`)**: `ID SessionRequirementID`, `CourseOfferingID CourseOfferingID`, `Type SessionType` (`THEORY`/`LAB`), `SessionsPerWeek int`, `Duration int`, `Consecutive bool`, `RequiredRoomFeatureIDs []RoomFeatureID`
- **Semantics**: Defines weekly session load and period duration. A requirement with `SessionsPerWeek: 3` and `Duration: 2` requires 3 distinct assignments of 2 consecutive periods each.

### 4. `model.FacultyPreference`
- **File**: `internal/scheduler/model/entities.go`
- **Fields**: `FacultyID FacultyID`, `TimeSlotID TimeSlotID`, `Weight int`
- **Semantics**: Soft objective preference penalty incurred when faculty is assigned to specified slot.

### 5. `problem.Assignment`
- **File**: `internal/scheduler/problem/assignment.go`
- **Fields**: `ID AssignmentID`, `CourseOfferingID CourseOfferingID`, `SessionRequirementID SessionRequirementID`, `StudentGroupID StudentGroupID`, `FacultyID FacultyID`, `RoomID RoomID`, `TimeSlotID TimeSlotID`, `Instance int`
- **Semantics**: Unit of scheduled placement. Occupies `[TimeSlotID, TimeSlotID + Duration - 1]` on the same day.

### 6. `problem.Problem`
- **File**: `internal/scheduler/problem/problem.go`
- **Fields**: Self-contained problem instance containing lookup maps for `Departments`, `Programs`, `Classes`, `StudentGroups`, `Subjects`, `CourseOfferings`, `SessionRequirements`, `Faculty`, `Rooms`, `TimeSlots`, `LockedAssignments`, `FacultyPreferences`, `FacultyAvailable`, `RoomAvailable`, `SlotsByDayPeriod`, `StudentGroupOverlaps`, `PeriodsPerDay`.

### 7. `problem.Solution`
- **File**: `internal/scheduler/problem/solution.go`
- **Fields**: `Assignments []Assignment`, `Index SolutionIndex`, `Score scorer.Score`
- **Index**: `SolutionIndex` maintains $O(1)$ maps for `FacultySlot`, `RoomSlot`, `StudentGroupSlot`, `RequirementCount`, and `byID`.

---

# 5. DATABASE MODEL

### ER Diagram

```text
institutions ───┬──> departments ───> programs ───> classes ───> student_groups
             │   ├──> faculty ───┬──> faculty_availability
             │   │               └──> faculty_preferences
             │   ├──> rooms ─────┬──> room_availability
             │   │               └──> room_feature_assignments ───> room_features
             │   ├──> subjects
             │   ├──> time_slots
             │   └──> academic_years ───> terms ───> course_offerings ───> session_requirements
             │                                          │                        │
             │                                          ├──> course_offering_features
             │                                          └──> session_requirement_features
             │
             └──> timetables ───┬──> problem_snapshots ───> constraint_rulesets
                                ├──> schedule_runs
                                └──> schedule_versions ───┬──> schedule_assignments
                                                           └──> assignment_pins
```

### Table Definitions & Key Columns
- `institutions`: Multi-tenant root. `id` UUID, `slug` TEXT UNIQUE, `settings` JSONB, `version` INT.
- `timetables`: Header record for scheduling project. `id` UUID, `institution_id` UUID, `current_published_version_id` UUID.
- `problem_snapshots`: Immutable snapshot of complete input domain. `problem` JSONB, `constraint_instances` JSONB, `input_hash` TEXT (SHA-256).
- `schedule_runs`: Execution record of solver invocation. `status` TEXT (`QUEUED`, `RUNNING`, `SOLVED`, `INFEASIBLE`, etc.), `result` JSONB, `diagnostics` JSONB, `score` JSONB, `violations` JSONB, `version` INT.
- `schedule_versions`: Immutable schedule version head. `status` TEXT (`DRAFT`, `REVIEW`, `PUBLISHED`, `ARCHIVED`), `score` JSONB, `version` INT. Partial unique index `idx_versions_one_published` ensures max 1 `PUBLISHED` version per timetable.
- `schedule_assignments`: Individual scheduled session rows attached to a version. `assignment_id` TEXT, `room_id` TEXT, `time_slot_id` TEXT. Unique `(version_id, assignment_id)`.

---

# 6. CORE SOLVER / ENGINE

### Pipeline Stages (`internal/scheduler/engine/engine.go`)

```text
Input Request
     │
     ▼
1. Problem Validation (ValidateProblem)
     │
     ▼
2. Index Preparation (Problem.Prepare)
     │
     ▼
3. Presolve Checks (Presolve)
     │
     ▼
4. Constraint Compilation (constraints.Compile)
     │
     ▼
5. CSP Backtracking Search (backtracking.Solver.Solve)
     │
     ├───────── status != SOLVED ─────────┐
     │                                    │
     ▼                                    ▼
6. Local Search / Tabu Search         Return Infeasible / Diagnostic
   (localsearch.TabuSearch)               Response
     │
     ▼
7. Authoritative Verification (verifier.VerifySolution)
     │
     ▼
Response (Solution, Diagnostics, Score, Violations)
```

### Stage Details
1. **Validation**: Checks entity references, non-zero durations, and valid `PeriodsPerDay`.
2. **Preparation**: Builds slot indexes (`SlotsByDayPeriod`), availability maps, and student group overlap maps (`BuildStudentGroupOverlaps`).
3. **Presolve**: Verifies room availability, capacity sufficiency, and total demand vs supply before search.
4. **Constraint Compilation**: Compiles declarative constraint instances into executable function arrays.
5. **CSP Backtracking**: Solves hard constraint feasibility. Uses MRV + Degree + LCV heuristics with forward checking.
6. **Tabu Search**: Optimizes soft penalties incrementally using neighborhood moves/swaps, short-term tabu tenure, and aspiration override.
7. **Verifier**: Performs pure-function authoritative validation of hard compliance and independent soft score recalculation.

---

# 7. HARD CONSTRAINTS

Every hard constraint implements `constraints.ConstraintDef` (`Evaluate(ctx, solution) []Violation`):

1. **Faculty Conflict (`FacultyConflict`)**: Ensures no faculty member is assigned to multiple overlapping sessions simultaneously.
2. **Room Conflict (`RoomConflict`)**: Ensures no room is assigned to multiple overlapping sessions simultaneously.
3. **Student Group Conflict (`StudentGroupConflict`)**: Ensures overlapping student groups (whole class vs subgroups) are not assigned to overlapping sessions simultaneously.
4. **Room Capacity (`RoomCapacity`)**: Ensures room capacity is $\ge$ student group enrollment size.
5. **Room Feature Compatibility (`RoomFeatureCompatibility`)**: Ensures room contains all required room features specified by course offering and session requirement.
6. **Faculty Availability (`FacultyAvailability`)**: Enforces explicit allow-list for faculty slot availability.
7. **Room Availability (`RoomAvailability`)**: Enforces explicit allow-list for room slot availability.

All 7 hard constraints are enforced during CSP search, candidate move validation, and authoritative verification.

---

# 8. SOFT OBJECTIVES / SCORING

Soft objectives are configured via `scorer.ObjectiveConfig` containing `[]ObjectiveComponent`.

1. **Student Gap Penalty (`ObjectiveStudentGapPenalty = "StudentGapPenalty"`)**:
   - Computes unweighted total gaps between the first and last period of each day for each student group.
   - Formula: $\text{DayGaps} = (\text{LastPeriod} - \text{FirstPeriod} + 1) - \text{OccupiedCount}$.
2. **Faculty Preference Penalty (`ObjectiveFacultyPreference = "FacultyPreference"`)**:
   - Sums penalty weights for assigned faculty slots matching `FacultyPreferences`.
3. **Room Change Penalty (`ObjectiveRoomChange = "RoomChange"`)**:
   - Counts room changes between chronologically adjacent scheduled sessions on the same day for a student cohort.
   - Transitions do not cross day boundaries; internal multi-period session periods incur 0 penalty.

### Score Aggregation

$$\text{TotalSoftPenalty} = \sum_{c \in \text{Components}} \text{RawScore}(c) \times \text{Weight}(c)$$

Lower score is better. Breakdown contains raw penalties, weighted total, and per-component scores.

---

# 9. INCREMENTAL SCORING / LOCAL SEARCH

Local search relies on `localsearch.IncrementalScoreEvaluator` (`internal/scheduler/solver/localsearch/incremental_evaluator.go`):

- **Maintained State**:
  - `schedules map[model.StudentGroupID]*[7]DaySchedule`: Tracks 1-indexed period occupancy counts and daily gap totals per group/day.
  - `groupDaySessions map[model.StudentGroupID]*[7][]sessionPlacement`: Maintains sorted list of daily sessions per group/day for room change calculations.
  - `prefIndex map[model.FacultyID]map[model.TimeSlotID]int`: Fast lookup index for faculty slot preference weights.
- **Delta Evaluation**:
  - For candidate moves and swaps, calculates exact score deltas ($\Delta \text{Gap}$, $\Delta \text{Pref}$, $\Delta \text{RC}$) only for affected group-days (at most 4 group-days per swap).
  - Time complexity: $O(k \log k)$ per candidate move where $k$ is daily session count for affected groups ($\sim 16-26\mu\text{s}$ per move), avoiding $O(N \log N)$ full-timetable scans.
- **State Mutation**: `EvaluateCandidateMove()` is strictly side-effect free. State is mutated only when `ApplyCandidateMove()` is explicitly called upon accepting a move.

---

# 10. TABU SEARCH

- **Implementation**: `internal/scheduler/solver/localsearch/tabu_search.go`
- **Neighborhood**: Generates single assignment moves to alternative valid room/slot placements and assignment swaps.
- **Tabu List**: Tracks recent move signatures in a fixed-tenure circular buffer to prevent cycling.
- **Aspiration Criterion**: A tabu move is accepted if its candidate score strictly improves upon the global best soft score found so far.
- **Determinism**: Neighborhood iteration order and pseudo-random tie-breaking use a seeded PRNG (`rand.New(rand.NewSource(seed))`).

---

# 11. CSP / BACKTRACKING

- **Implementation**: `internal/scheduler/solver/backtracking/backtracking.go`
- **Variable Ordering**: Dynamic ordering prioritizing unassigned variables by Minimum Remaining Values (MRV), broken by Highest Constraint Degree.
- **Value Ordering**: Least Constraining Value (LCV) heuristic sorting candidate placements by minimal conflict impact.
- **Forward Checking**: Prunes domain choices immediately upon making an assignment by checking constraint compatibility against unassigned variables.
- **Locked Assignments**: Pre-scheduled assignments (`Problem.LockedAssignments`) are assigned upfront during search initialization and domain choices for locked variables are pinned to 1 placement.

---

# 12. VERIFIER

- **Implementation**: `internal/scheduler/verifier/verifier.go`
- **Function**: `VerifySolution(p *problem.Problem, solution *problem.Solution, opts VerifyOptions) (VerificationReport, error)`
- **Verification Categories**:
  1. Requirement completeness (all required weekly sessions present).
  2. Total assignment count matching expected sum.
  3. Assignment ID uniqueness.
  4. Foreign-key & domain catalog validity.
  5. Placement & grid duration bounds validity.
  6. Locked assignment preservation.
  7. Hard constraint compliance (re-evaluated independently).
  8. Score consistency (soft penalty and component breakdown independently recalculated directly from solution assignments).

---

# 13. DIAGNOSTICS / STATUS MODEL

Defined in `internal/scheduler/diagnostics/status.go`:
- `SolveStatusSolved`: Full feasible solution found and verified.
- `SolveStatusInfeasible`: No hard-feasible solution exists under constraints.
- `SolveStatusInvalidProblem`: Input problem domain payload is malformed or invalid.
- `SolveStatusInvalidResult`: Result integrity verification failed (e.g. score mismatch or duplicate assignment ID).
- `SolveStatusCancelled`: Solve context cancelled by caller.
- `SolveStatusDeadlineExceeded`: Solve context deadline/timeout exceeded.
- `SolveStatusNodeLimit`: CSP search node limit reached before finding solution.

---

# 14. APPLICATION LAYER

- **API Entrypoint**: `application/cmd/server/main.go`
- **Worker Entrypoint**: `application/cmd/worker/main.go`
- **HTTP Routing**: `application/internal/api/router.go` utilizing standard Go `net/http.ServeMux`.
- **Services**:
  - `TimetableService`: CRUD operations on timetable headers.
  - `SnapshotService`: Immutable problem snapshot creation and hashing.
  - `RunService`: Solver run creation, job enqueueing, and cancellation.
  - `VersionService`: Schedule version management, move/swap edits, status transitions (`Draft` $\to$ `Review` $\to$ `Published`), and optimistic locking checks.
  - `CatalogService`: Academic metadata management (departments, programs, classes, faculty, rooms, time slots).

---

# 15. CURRA ADAPTER / BOUNDARY

- **File**: `application/internal/curra/adapter.go`
- **Interface**: `CurraAdapter` (`application/internal/curra/types.go`)
- **Isolation Guarantee**: `Adapter` is the ONLY package in `application/` permitted to import `internal/scheduler`. It converts raw JSON byte payloads from snapshots into `problem.Problem` instances, invokes `engine.Solve` or `verifier.VerifySolution`, and maps results back to serializable DTOs (`SolveResponse`, `VerifyResponse`, `ValidateMoveResponse`).
- **Enforcement**: Validated by `application/boundary_test.go`.

---

# 16. REST API INVENTORY

| Method | Route Path | Handler Purpose | Auth Required |
|---|---|---|---|
| `GET` | `/health` | Server health check | No |
| `GET` | `/api/v1/auth/me` | Current user profile | Yes |
| `POST` | `/api/v1/timetables` | Create timetable project | Yes |
| `GET` | `/api/v1/timetables` | List timetables for tenant | Yes |
| `GET` | `/api/v1/timetables/{id}` | Get timetable header by ID | Yes |
| `PATCH` | `/api/v1/timetables/{id}` | Update timetable header | Yes |
| `POST` | `/api/v1/timetables/{id}/snapshots` | Create problem snapshot | Yes |
| `GET` | `/api/v1/timetables/{id}/snapshots` | List snapshots for timetable | Yes |
| `GET` | `/api/v1/snapshots/{id}` | Get snapshot metadata | Yes |
| `GET` | `/api/v1/snapshots/{id}/problem` | Get raw snapshot problem JSON | Yes |
| `POST` | `/api/v1/timetables/{id}/runs` | Enqueue solver run job | Yes |
| `GET` | `/api/v1/timetables/{id}/runs` | List solver runs for timetable | Yes |
| `GET` | `/api/v1/runs/{id}` | Get solver run status & result | Yes |
| `POST` | `/api/v1/runs/{id}/cancel` | Cancel queued/running job | Yes |
| `POST` | `/api/v1/timetables/{id}/versions` | Create schedule version from run | Yes |
| `GET` | `/api/v1/timetables/{id}/versions` | List schedule versions | Yes |
| `GET` | `/api/v1/versions/{id}` | Get schedule version header | Yes |
| `GET` | `/api/v1/versions/{id}/assignments` | List assignments for version | Yes |
| `PATCH` | `/api/v1/versions/{id}` | Update schedule version name | Yes |
| `POST` | `/api/v1/versions/{id}/submit-review` | Transition version status to REVIEW | Yes |
| `POST` | `/api/v1/versions/{id}/send-back` | Transition version status to DRAFT | Yes |
| `POST` | `/api/v1/versions/{id}/publish` | Publish version (unpublishes old) | Yes |
| `POST` | `/api/v1/versions/{id}/archive` | Archive schedule version | Yes |
| `POST` | `/api/v1/versions/{id}/assignments/move` | Perform manual assignment move | Yes |
| `POST` | `/api/v1/versions/{id}/assignments/swap` | Perform manual assignment swap | Yes |
| `POST` | `/api/v1/verify` | Verify problem/solution payload | Yes |
| `GET` | `/api/v1/departments` | List departments | Yes |
| `POST` | `/api/v1/departments` | Create department | Yes |
| `GET` | `/api/v1/programs` | List programs | Yes |
| `GET` | `/api/v1/classes` | List classes | Yes |
| `GET` | `/api/v1/student-groups` | List student groups | Yes |
| `GET` | `/api/v1/subjects` | List subjects | Yes |
| `GET` | `/api/v1/faculty` | List faculty | Yes |
| `GET` | `/api/v1/rooms` | List rooms | Yes |
| `GET` | `/api/v1/time-slots` | List time slots | Yes |

---

# 17. OPENAPI / CONTRACTS

- **OpenAPI File**: `api/openapi.yaml` (OpenAPI 3.0 specification).
- **Authoritative Contract**: OpenAPI specification and `contracts/api-contract.md` dictate API path naming, request schemas, and error responses (`400 Bad Request`, `401 Unauthorized`, `404 Not Found`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`).
- **Compatibility Routing**: `application/internal/api/router.go` mounts both direct routes (`/api/v1/departments`) and OpenAPI nested routes (`/api/v1/institutions/{instId}/departments`) to guarantee 100% specification compliance.

---

# 18. VERSIONING / CONCURRENCY

- **State Machine Invariant**: Schedule versions transition through `DRAFT` $\to$ `REVIEW` $\to$ `PUBLISHED` (or `ARCHIVED`).
- **Single Published Version**: Postgres partial unique index `idx_versions_one_published` enforces at most one `PUBLISHED` schedule version per timetable project. Publishing a new version atomically unpublishes any prior published version.
- **Optimistic Concurrency Control**: All schedule version mutations (name updates, move/swap edits, status transitions) require sending the client's known `version` integer count. If database `version != request.version`, a `409 Conflict` error is returned.

```text
Client Version v3
       │
       ▼
POST /api/v1/versions/{id}/assignments/move (expectedVersion: 3)
       │
   [ Check DB Version ]
       ├── DB Version == 3 ──> Apply Move, Increment DB Version to 4 (200 OK)
       └── DB Version == 4 ──> Return 409 Conflict (Stale Version Error)
```

---

# 19. ASYNCHRONOUS SOLVER WORKFLOW

```text
Client Request (POST /timetables/{id}/runs)
       │
       ▼
Create `schedule_runs` record (status = 'QUEUED')
       │
       ▼
Background Worker Daemon (`application/internal/worker/worker.go`)
  - Polls `schedule_runs` WHERE status = 'QUEUED'
  - Atomically claims job (status = 'RUNNING', worker_id = X, heartbeat_at = now())
       │
       ▼
Invoke CURRA Engine Adapter (`curra.Adapter.Solve`)
       │
       ▼
Engine Completes & Returns Solution + Score + Diagnostics + Violations
       │
       ▼
Worker updates `schedule_runs`:
  - status = 'SOLVED' (or 'INFEASIBLE' / 'FAILED')
  - result = Solution JSON
  - finished_at = now()
```

Clients poll `GET /api/v1/runs/{id}` to observe status changes (`QUEUED` $\to$ `RUNNING` $\to$ `SOLVED`).

---

# 20. EXISTING FRONTEND

```text
No usable frontend currently exists.
```
The repository contains the complete backend scheduling engine, REST API server, background worker, database migrations/schema, and contract specifications. No web UI code is included.

---

# 21. DEMO / SEED DATA

- **Synthetic Problem Generators**: `internal/scheduler/testutil/synthetic.go` provides programmatic problem instance generators for Small (24 sessions), Medium (300 sessions), and Large (3,000 sessions) benchmark datasets.
- **Fixtures**: `api/fixtures/` contains sample problem JSON files (`small_problem.json`, `medium_problem.json`) usable for API testing and manual verification.

---

# 22. TESTING

### Test Suite Categories
1. **Unit Tests**:
   - `internal/scheduler/scorer/*_test.go`: Penalty calculators (StudentGap, FacultyPref, RoomChange).
   - `internal/scheduler/problem/*_test.go`: Assignment indexing and lock integrity.
   - `internal/scheduler/solver/backtracking/*_test.go`: CSP heuristics and search execution.
   - `internal/scheduler/solver/localsearch/*_test.go`: Tabu search and incremental scoring.
2. **Integration & Differential Tests**:
   - `tests/redteam_fuzz_test.go`: 4,000 randomized move/swap mutations testing exact parity between `IncrementalScoreEvaluator` and `FullScoreEvaluator`.
   - `internal/scheduler/verifier/verifier_test.go`: Authoritative verifier compliance and adversarial score tampering detection.
3. **Concurrency Tests**:
   - `application/concurrency_test.go`: High-concurrency optimistic locking tests for schedule version mutations.
4. **Boundary Tests**:
   - `application/boundary_test.go`: AST import boundary checks verifying application code does not import solver internal packages.

### Commands
- Run all tests: `go test -count=1 ./...`
- Run application tests: `cd application && go test -count=1 ./...`
- Run red-team fuzz tests: `go test -v ./tests -run TestRedTeam`

---

# 23. PERFORMANCE

### Baseline Timings (Observed Benchmarks)
- **Small Problem (24 sessions, 2 classes, 6 faculty, 4 rooms, 30 slots)**:
  - CSP Solve Time: ~84 ms
  - Tabu Optimization (20 iterations): ~16 ms
  - Candidate Move Evaluation Time: ~16.1 $\mu$s / op
- **Medium Problem (300 sessions, 20 classes, 50 faculty, 30 rooms, 30 slots)**:
  - CSP Solve Time: ~273 ms
  - Tabu Optimization (25 iterations): ~33 ms
  - Candidate Move Evaluation Time: ~26.9 $\mu$s / op
- **Large Problem (3,000 sessions, 100 classes, 300 faculty, 150 rooms, 40 slots)**:
  - Index Preparation: ~2.98 ms
  - Full Score Evaluator rescan: ~5.14 ms / op
  - Incremental candidate move evaluation: ~30-50 $\mu$s / op ($\sim 100\times$ speedup over full rescan)

---

# 24. SECURITY

### Observed Protections
- **Multi-Tenant Isolation**: Every database table includes `institution_id` foreign key referencing `institutions(id)`. API middleware extracts tenant identity from auth claims and scopes queries.
- **SQL Injection Prevention**: All database queries use parameterized SQL via `pgx/v5`.
- **Import Boundary Enforced**: Engine solver code is decoupled from network/API layer.

### Missing Protections / Production Recommendations
- **Authentication Credentials**: Simple header/token authentication present in development middleware; full OAuth2 / OIDC token verification should be attached in production gateway.
- **Rate Limiting**: Per-tenant rate limiting on `/api/v1/timetables/{id}/runs` should be configured at gateway.

---

# 25. CONFIGURATION / LOCAL DEVELOPMENT

### Environment Variables (`application/.env.example`)
```env
PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/curra_db?sslmode=disable
LOG_LEVEL=info
WORKER_POLL_INTERVAL=1s
WORKER_CONCURRENCY=2
```

### Development Execution Commands
1. **Start Database**: `docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=curra_db postgres:16`
2. **Apply Schema**: `psql -h localhost -U postgres -d curra_db -f database/schema.sql` (or schema DDL in `database/schema.md`)
3. **Run API Server**: `cd application && go run cmd/server/main.go`
4. **Run Background Worker**: `cd application && go run cmd/worker/main.go`
5. **Run CLI Solver**: `go run cmd/solver/main.go -problem api/fixtures/small_problem.json`

---

# 26. DEPLOYMENT

- **Containerization**: Application API server and worker daemon build as standard static Go binaries.
- **Runtime Services**:
  - Stateless API Service (`cmd/server`) horizontally scalable.
  - Asynchronous Worker Daemon (`cmd/worker`) scalable with database-backed atomic job claiming (`schedule_runs`).
  - Managed PostgreSQL 16+ instance.

---

# 27. CURRENT STATE

- `IMPLEMENTED`: Core CSP solver, Tabu Search optimizer, Incremental score evaluator, Authoritative verifier, Soft objective suite (Student Gap, Faculty Preference, Room Change), Multi-tenant Postgres database schema & repos, API Server HTTP handlers, Background worker job daemon, Move/Swap validation endpoints, Optimistic concurrency version control, Red-team fuzz & boundary test suites.
- `PARTIALLY IMPLEMENTED`: Frontend UI (not started).
- `PLANNED`: Production OAuth2/OIDC gateway integration, Excel/CSV bulk data import parser pipeline.

---

# 28. KNOWN TECHNICAL RISKS

1. **Large Problem Memory Footprint**: Generating 3,000+ assignment candidate moves during Tabu local search creates transient allocations. Incremental evaluator minimizes rescan overhead, but neighborhood size controls (`MaxCandidatesPerIteration`) must be tuned for ultra-large instances ($>10,000$ sessions).
2. **Stale Client Edits under High Concurrency**: Simultaneous manual move/swap operations on the same draft version will result in `409 Conflict` HTTP errors for all but the first request due to optimistic locking. Client UIs must handle optimistic lock failures by fetching latest version assignments and re-applying user diffs.

---

# 29. FRONTEND INTEGRATION REQUIREMENTS

### Typical Client Integration Workflows

1. **Create Timetable Project**:
   `POST /api/v1/timetables` $\to$ Returns timetable ID.
2. **Configure Academic Data**:
   `POST /api/v1/departments`, `POST /api/v1/faculty`, `POST /api/v1/rooms`, etc.
3. **Generate Problem Snapshot & Run Solver**:
   `POST /api/v1/timetables/{id}/snapshots` $\to$ Returns snapshot ID.
   `POST /api/v1/timetables/{id}/runs` (body: `{ "snapshotId": "..." }`) $\to$ Returns run ID.
4. **Poll Solver Run Status**:
   `GET /api/v1/runs/{runId}` $\to$ Poll until `status == "SOLVED"`.
5. **Create Draft Schedule Version**:
   `POST /api/v1/timetables/{id}/versions` (body: `{ "runId": "..." }`) $\to$ Creates version v1 (status `DRAFT`).
6. **Fetch Timetable Grid**:
   `GET /api/v1/versions/{id}/assignments` $\to$ Returns list of scheduled assignments with room & time slot placements.
7. **Perform Manual Move or Swap**:
   `POST /api/v1/versions/{id}/assignments/move` (body: `{ "move": {...}, "version": 1 }`) $\to$ Returns updated solution & new version v2.
8. **Publish Schedule**:
   `POST /api/v1/versions/{id}/publish` $\to$ Transitions version status to `PUBLISHED`.

---

# 30. ARCHITECTURAL "DO NOT BREAK" RULES

1. **Never Import `internal/scheduler` in Application Packages**: Application logic must interact with the solver exclusively via `curra.Adapter` (`application/internal/curra/adapter.go`).
2. **Preserve Authoritative Verifier Independence**: The verifier must independently recalculate soft scores directly from raw solution assignments without calling engine scoring utilities.
3. **Maintain Engine Determinism**: Core scheduling code must not use unseeded randomness, global maps with non-deterministic iteration order without sorting, or system clock calls.
4. **Enforce Optimistic Concurrency**: Every mutation of `schedule_versions` or `schedule_assignments` must verify version numbers (`WHERE id = $1 AND version = $2`).
5. **Hard Constraints Always Dominate**: Soft objective penalties must never cause a hard-infeasible placement to be selected over a hard-feasible placement.

---

# 31. FILE-LEVEL REFERENCE MAP

- **Core Engine Entrypoint**: `internal/scheduler/engine/engine.go`
- **Problem & Solution Definitions**: `internal/scheduler/problem/problem.go`, `internal/scheduler/problem/solution.go`
- **Domain Model**: `internal/scheduler/model/entities.go`, `internal/scheduler/model/timeslot.go`
- **Hard Constraints**: `internal/scheduler/constraints/compiled.go`
- **Soft Objective Scorer**: `internal/scheduler/scorer/score.go`
- **Incremental Evaluator**: `internal/scheduler/solver/localsearch/incremental_evaluator.go`
- **Tabu Search Solver**: `internal/scheduler/solver/localsearch/tabu_search.go`
- **CSP Backtracking Solver**: `internal/scheduler/solver/backtracking/backtracking.go`
- **Authoritative Verifier**: `internal/scheduler/verifier/verifier.go`
- **Engine Adapter Boundary**: `application/internal/curra/adapter.go`, `application/internal/curra/types.go`
- **HTTP Routing & API**: `application/internal/api/router.go`, `application/internal/api/handlers/handlers.go`
- **Application Services**: `application/internal/services/version.go`, `application/internal/services/run.go`
- **Background Worker Daemon**: `application/internal/worker/worker.go`
- **Database Schema Documentation**: `database/schema.md`
- **OpenAPI Specification**: `api/openapi.yaml`

---

# 32. CRITICAL SOURCE EXCERPTS

### Excerpt 1: CURRA Engine Adapter (`application/internal/curra/adapter.go`)

```go
package curra

// Adapter implements the CurraAdapter interface.
// It is stateless, has no database dependencies, and is the ONLY package
// that imports CURRA solver packages.
type Adapter struct {
	logger *slog.Logger
}

func (a *Adapter) Solve(ctx context.Context, req SolveRequest) (SolveResponse, error) {
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return SolveResponse{Status: "INVALID_PROBLEM"}, err
	}

	engineReq := engine.Request{
		Problem:     p,
		SolveOptions: problem.SolveOptions{MaxNodes: req.MaxNodes, SearchMode: problem.SearchMode(req.SearchMode)},
		TabuOptions: localsearch.TabuSearchOptions{Seed: req.Seed},
	}

	resp, err := engine.Solve(ctx, engineReq)
	if err != nil {
		return SolveResponse{Status: string(resp.Diagnostics.Status)}, err
	}

	solutionJSON, _ := json.Marshal(resp.Solution)
	return SolveResponse{
		Status:   string(resp.Diagnostics.Status),
		Solution: solutionJSON,
		Score:    mapScore(resp.Score),
	}, nil
}
```

### Excerpt 2: Authoritative Verifier (`internal/scheduler/verifier/verifier.go`)

```go
// VerifySolution performs an authoritative, pure/read-only verification pass.
func VerifySolution(p *problem.Problem, solution *problem.Solution, opts VerifyOptions) (VerificationReport, error) {
	p.Prepare()

	// 1. Assignment ID Uniqueness & Foreign Key Validity
	// 2. Requirement Completeness & Total Counts
	// 3. Locked Assignments Preservation
	// 4. Hard Constraint Compliance
	// 5. Authoritative Score Consistency

	expectedBreakdown := calculateIndependentScore(p, solution, objConfig)
	if solution.Score.SoftPenalty != expectedBreakdown.SoftPenalty {
		return VerificationReport{
			Valid:   false,
			Status:  diagnostics.SolveStatusInvalidResult,
			Message: "reported SoftPenalty does not match authoritative calculation",
		}, ErrInvalidResult
	}
	return VerificationReport{Valid: true, Status: diagnostics.SolveStatusSolved}, nil
}
```

---

# 33. ACTUAL ARCHITECTURAL FLOW

### Manual Move Flow

```text
HTTP Client (POST /api/v1/versions/{id}/assignments/move)
       │
       ▼
Handlers (`application/internal/api/handlers/handlers.go`)
       │
       ▼
VersionService (`application/internal/services/version.go`)
  - Load Version & Snapshot from Postgres DB
  - Verify Version status == 'DRAFT'
  - Verify expected DB Version matches client version
       │
       ▼
CURRA Adapter (`application/internal/curra/adapter.go`)
  - Call `ValidateMove(ProblemJSON, SolutionJSON, Move)`
  - Clone Solution
  - Apply Move to Cloned Solution
  - Call `verifier.VerifySolution` on Cloned Solution
       │
       ▼
If Valid:
  - VersionService saves updated `schedule_assignments` to DB
  - Increment Version counter in DB (`version = version + 1`)
  - Commit DB Transaction
       │
       ▼
Return 200 OK (Updated Solution + New Version Number)
```

---

# 34. TEXT-ONLY CONTEXT FOR CLAUDE

You are reviewing a repository called `sPreetham42/Curra` (module `github.com/sPreetham42/timetable-platform`).

You have no filesystem access.

The repository dossier provided above contains the complete, authoritative architectural snapshot of the repository, including domain models, database schemas, solver pipeline stages, API endpoints, constraint definitions, scoring functions, adapter boundaries, and testing patterns.

Please use this dossier as your sole source of truth for analyzing, auditing, or planning features for the CURRA Timetable Platform.

---

END OF REPOSITORY DOSSIER

Important:
This dossier is a snapshot.
Statements marked OBSERVED are directly verified from repository contents.
Statements marked INFERRED are interpretations.
Statements marked UNKNOWN could not be established.
Claude should not assume anything outside this document is true.
