# CURRA Project Context & Architecture Dossier

**Authoritative Technical Reference & Source of Truth**  
**Repository:** `github.com/sPreetham42/timetable-platform`  
**Execution Environment:** Go 1.22+ (Windows / Linux / macOS)  
**Status:** Core Solver Engine Frozen (v1.0.0) | Application Contract Frozen (v1) | Structural Refactor Complete

---

## 1. Project Identity

- **Project Name:** CURRA (formerly referred to as Cura in legacy comments)
- **What CURRA Is:** A production-grade, deterministic academic timetable scheduling engine and platform written in standard Go.
- **Primary Purpose:** Resolves multi-variable academic timetable scheduling problems by satisfying strict institutional hard constraints (room capacity, equipment, faculty availability, student group conflicts, consecutive period requirements) and minimizing soft operational penalties (idle gaps for student cohorts between classes).
- **Implementation Language(s):** Go (100% standard library for the core solver; zero runtime external dependencies). PostgreSQL 16+ for persistence. OpenAPI 3.1 for REST contracts.
- **Repository / Module Name:** `github.com/sPreetham42/timetable-platform`
- **Current Project Status:** Core scheduling engine (`internal/scheduler/...`) is fully implemented, verified against ITC 2007 benchmarks, hardened against invariant violations, and frozen. Downstream application contracts (`application/internal/curra`, `api/openapi.yaml`, `database/schema.md`) are frozen at v1.
- **What Is Implemented:**
  - Complete domain modeling for universities (Departments, Programs, Classes, Subgroups, Offerings, Faculty, Rooms, Features, TimeSlots).
  - Fast pure pre-solve analysis (`PreSolve`) and structural validation (`Validate`).
  - Declarative constraint compilation engine (`constraints.Compile`) with canonical SHA-256 rule hashing.
  - Complete backtracking CSP solver with MRV, Degree, LCV, and Forward Checking heuristics.
  - Tabu search meta-heuristic local search optimizer with delta scoring, move validation, and aspiration criteria.
  - 14-point pure read-only authoritative solution verifier (`verifier.VerifySolution`).
  - Centralized test utilities and package-local unit test suite co-location.
  - Multi-tenant PostgreSQL database schema and REST API contract specification.
  - Stateless downstream application adapter (`CurraAdapter`).
- **What Is Explicitly NOT Implemented:**
  - Date-range or academic calendar holiday awareness (atomic scheduling unit is strictly a weekly recurring bell-schedule grid).
  - Individual student electives tracking (students are modeled as atomic cohorts / `StudentGroup` sets).
  - Multi-objective Pareto frontier solvers (single weighted scalar penalty breakdown).
  - Dynamic distributed cluster solving (in-memory single-process execution).

---

## 2. Current Repository Structure

```
timetable-platform/
├── cmd/
│   └── solver/
│       └── main.go                                     # Production CLI executable entrypoint
├── internal/
│   └── scheduler/
│       ├── model/                                      # Pure domain entity identifiers and data types
│       │   ├── model.go                                # Entity structs (Room, Faculty, Offering, etc.)
│       │   ├── ids.go                                  # Strongly typed identifier types
│       │   └── availability.go                         # Faculty & Room availability bitset models
│       ├── diagnostics/                                # Diagnostics, SolveStatus, and Violations
│       │   └── diagnostics.go                          # SolveStatus, Severity, Violation structs
│       ├── problem/                                    # Problem graph, SolutionIndex, Move/Swap lifecycle
│       │   ├── problem.go                              # Problem graph & Prepare() indexes
│       │   ├── solution.go                             # Solution & SolutionIndex definitions
│       │   ├── move.go                                 # Move, Swap, and in-place apply/undo operations
│       │   ├── presolver.go                            # Fast pre-solve bottleneck detection
│       │   ├── validation.go                           # Pure structural catalog validation
│       │   ├── options.go                              # SolveOptions & SearchMode enums
│       │   ├── invariants_test.go                      # [Package-local] Solution & index invariant tests
│       │   └── locked_assignments_test.go              # [Package-local] Pinned assignment tests
│       ├── constraints/                                # Declarative constraint framework
│       │   ├── framework.go                            # Compile(), CompiledConstraintSet, SearchCtx
│       │   ├── hard_rules.go                           # Built-in constraint rule templates
│       │   ├── legacy_hard.go                          # Backward-compatible hard constraint evaluators
│       │   ├── membership.go                           # Student group hierarchy membership index
│       │   ├── faculty_availability.go                 # Faculty availability constraint
│       │   ├── room_availability.go                    # Room availability constraint
│       │   ├── constraints_test.go                     # [Package-local] Rule compilation & evaluation tests
│       │   └── membership_test.go                      # [Package-local] Membership index tests
│       ├── scorer/                                     # Soft objective evaluations & gap penalties
│       │   ├── scorer.go                               # Student gap penalty calculations & ObjectiveConfig
│       │   ├── scorer_test.go                          # [Package-local] Gap penalty scoring tests
│       │   └── weighted_scoring_test.go                # [Package-local] Soft objective weight tests
│       ├── solver/
│       │   ├── backtracking/                           # CSP search engine
│       │   │   ├── backtracking.go                     # Recursive backtracking with Forward Checking
│       │   │   ├── heuristics.go                       # MRV, Degree, LCV ordering algorithms
│       │   │   ├── backtracking_test.go                # [Package-local] CSP unit tests
│       │   └── lcv_test.go                             # [Package-local] LCV heuristic isolation tests
│       │   └── localsearch/                            # Tabu Search optimizer & delta scoring
│       │       ├── tabu_search.go                      # Meta-heuristic Tabu Search loop
│       │       ├── move_validator.go                   # Scoped candidate move legality checking
│       │       ├── candidate_generator.go              # Randomized neighborhood candidate generation
│       │       ├── incremental_scorer.go               # Delta score evaluator
│       │       ├── localsearch_test.go                 # [Package-local] Move validator tests
│       │       ├── tabu_search_test.go                 # [Package-local] Tabu search optimizer tests
│       │       ├── incremental_scorer_test.go          # [Package-local] Incremental score parity tests
│       │       └── tabu_search_bench_test.go           # [Package-local] Tabu search benchmarks
│       ├── verifier/                                   # 14-point authoritative verification
│       │   ├── verifier.go                             # Authoritative pure read-only verifier
│       │   └── verifier_test.go                        # [Package-local] Verification failure mode tests
│       ├── engine/                                     # Top-level solver pipeline orchestrator
│       │   ├── engine.go                               # Solve() orchestrator
│       │   ├── engine_test.go                          # [Package-local] Engine integration tests
│       │   └── engine_hardening_test.go                # [Package-local] Engine lifecycle & safety tests
│       └── testutil/                                   # Centralized synthetic fixtures & test assertions
│           └── fixtures.go                             # Problem generators (Small, Medium, Large) & asserts
├── tests/                                              # System integration, property fuzzers, & benchmarks
│   ├── benchmark_itc2007_test.go                       # ITC 2007 standard curriculum benchmark suite
│   ├── csp_heuristic_investigation_test.go             # CSP heuristic ablation harness
│   ├── hardening_invariants_test.go                    # Cross-cutting invariant stress suite
│   ├── performance_baseline_test.go                    # Engine throughput and memory baselines
│   ├── randomized_invariant_test.go                    # Property-based randomized fuzzer across seeds
│   └── stress_pathological_test.go                     # Pathological and overconstrained CSP stress tests
├── application/                                        # Downstream production API service module
│   ├── cmd/                                            # Application server entrypoint
│   ├── internal/
│   │   ├── api/                                        # REST handlers & middleware
│   │   ├── curra/                                      # CurraAdapter (sole CURRA consumer)
│   │   ├── database/                                   # PostgreSQL queries & repositories
│   │   ├── domain/                                     # Application domain entities
│   │   ├── services/                                   # Business logic & timetable services
│   │   └── worker/                                     # Background async solver worker
│   └── scripts/
│       └── check-curra-imports.sh                      # CI check enforcing CURRA import boundaries
├── contracts/                                          # Architecture and integration contracts
│   ├── api-contract.md                                 # REST API resource and endpoint definitions
│   └── curra-adapter.md                                # CurraAdapter Go boundary contract
├── docs/                                               # System architecture documentation
│   ├── CURA_ARCHITECTURE_CURRENT_STATE.md             # In-depth solver architecture specification
│   ├── curra-public-api.md                             # Public API inventory and suitability classification
│   ├── curra-determinism.md                            # Determinism guarantees & replay specifications
│   ├── curra-application-mapping.md                    # Database-to-solver domain mapping guide
│   ├── domain-model.md                                 # Application entity taxonomy
│   ├── reproducibility.md                              # Snapshot, reproduce, verify, and audit contracts
│   ├── state-machines.md                               # Lifecycle state machines (Runs, Versions, Batches)
│   ├── permissions.md                                  # RBAC permissions matrix
│   └── agent-boundaries.md                             # Multi-agent and CI boundary constraints
├── database/
│   └── schema.md                                       # PostgreSQL 16+ DDL schema definitions
├── api/
│   ├── openapi.yaml                                    # OpenAPI 3.1.0 specification
│   └── fixtures/                                       # JSON payload mock fixtures
├── .gitignore
├── LICENSE
├── README.md
└── go.mod
```

---

## 3. CURRA Architecture

```
[ Frontend (Manus / React) ]
            │ HTTP / OpenAPI 3.1 JSON
            ▼
[ Backend API (MIMO / Go Application) ]
            │
            ▼
[ Worker / Service Layer ] ──> Stores Snapshot in PostgreSQL (JSONB)
            │
            ▼
┌────────────────────────────────────────────────────────┐
│                   CurraAdapter                         │
│  (application/internal/curra - Pure Go, Stateless)     │
│  - Maps Application DTOs ↔ CURRA Domain Structs       │
│  - Enforces Clone-Before-Mutate                       │
│  - Invokes Authoritative Verification                  │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│                   CURRA Engine                         │
│            (internal/scheduler/engine)                 │
│                                                        │
│  1. Problem.Validate() (Structural catalog check)      │
│  2. Problem.Prepare()  (Index derived availability)   │
│  3. Problem.PreSolve() (Fast bottleneck check)         │
│  4. constraints.Compile() (SHA-256 RuleSetHash)        │
│                                                        │
│  5. CSP Backtracking Solver (solver/backtracking)      │
│     - Seed LockedAssignments                           │
│     - MRV + Degree Variable Ordering                   │
│     - LCV Value Ordering & Forward Checking Pruning    │
│     - Yields Initial Feasible Solution (0 Hard Viols)  │
│                                                        │
│  6. Tabu Search Optimizer (solver/localsearch)         │
│     - Delta Scoring via IncrementalScoreEvaluator      │
│     - Move/Swap Neighborhood Exploration               │
│     - Scoped Feasibility Invariant Preservation        │
│     - Minimizes Student Gap Soft Penalties             │
│                                                        │
│  7. Authoritative Verifier (verifier)                  │
│     - Pure read-only 14-point audit pass               │
│     - Proves requirement completeness & 0 violations   │
└────────────────────────────────────────────────────────┘
```

---

## 4. Domain Model

Located in `internal/scheduler/model/` and `internal/scheduler/problem/`:

### Core Identifiers (`ids.go`)
Strongly typed `string` aliases: `TenantID`, `DepartmentID`, `ProgramID`, `ClassID`, `StudentGroupID`, `SubjectID`, `CourseOfferingID`, `SessionRequirementID`, `FacultyID`, `RoomID`, `RoomFeatureID`, `TimeSlotID`, `TermID`, `AssignmentID`.

### Entities (`model.go`, `availability.go`)
1. **Department:** `ID DepartmentID`, `TenantID TenantID`, `Name string`
2. **Program:** `ID ProgramID`, `DepartmentID DepartmentID`, `Name string`
3. **Class:** `ID ClassID`, `ProgramID ProgramID`, `Name string`, `WholeGroupID StudentGroupID`, `StudentGroupIDs []StudentGroupID`
4. **StudentGroup:** `ID StudentGroupID`, `ClassID ClassID`, `Name string`, `Size int`
5. **Subject:** `ID SubjectID`, `DepartmentID DepartmentID`, `Code string`, `Name string`
6. **CourseOffering:** `ID CourseOfferingID`, `SubjectID SubjectID`, `FacultyID FacultyID`, `ClassID ClassID`, `StudentGroupID StudentGroupID`, `RequiredRoomFeatureIDs []RoomFeatureID`
7. **SessionRequirement:** `ID SessionRequirementID`, `CourseOfferingID CourseOfferingID`, `Type string` (`THEORY`/`LAB`), `SessionsPerWeek int`, `Duration int`, `Consecutive bool`, `RequiredRoomFeatureIDs []RoomFeatureID`
8. **Faculty:** `ID FacultyID`, `TenantID TenantID`, `Name string`
9. **FacultyAvailability:** `FacultyID FacultyID`, `TimeSlotID TimeSlotID` (Allow-list entry)
10. **FacultyPreference:** `FacultyID FacultyID`, `TimeSlotID TimeSlotID`, `Weight int`
11. **Room:** `ID RoomID`, `TenantID TenantID`, `Name string`, `Capacity int`, `FeatureIDs []RoomFeatureID`
12. **RoomFeature:** `ID RoomFeatureID`, `Name string`
13. **RoomAvailability:** `RoomID RoomID`, `TimeSlotID TimeSlotID` (Allow-list entry)
14. **Term:** `ID TermID`, `TenantID TenantID`, `Name string`
15. **TimeSlot:** `ID TimeSlotID`, `Day string` (`Monday`..`Sunday`), `Period int` (1-indexed), `Label string`

### Problem & Solution Structures (`problem.go`, `solution.go`, `move.go`)
16. **Problem:** Map-based catalog containing all departments, programs, classes, groups, subjects, offerings, requirements, faculty, rooms, features, time slots, availabilities, preferences, locked assignments, and `PeriodsPerDay int`.
17. **Assignment:** `ID AssignmentID` (`RequirementID#Instance`), `CourseOfferingID`, `StudentGroupID`, `FacultyID`, `RoomID`, `TimeSlotID`, `SessionRequirementID`, `Instance int`.
18. **Solution:** `Assignments []Assignment`, `Score scorer.Score`, `Index *SolutionIndex` (`json:"-"`).
19. **Placement:** `RoomID RoomID`, `TimeSlotID TimeSlotID`.
20. **Move:** `AssignmentID AssignmentID`, `From Placement`, `To Placement`.
21. **Swap:** `Assignment1ID AssignmentID`, `Assignment2ID AssignmentID`, `Placement1 Placement`, `Placement2 Placement`.

---

## 5. Backend ↔ Frontend Contract

Derived from `contracts/api-contract.md` and `api/openapi.yaml`. Base prefix: `/api/v1`.

### Endpoints Matrix

| Method & Path | Purpose | Auth Required | Request Schema | Response Schema | Status Codes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `POST /auth/google` | Exchange OAuth code for JWT | None | `{ code: string }` | `{ accessToken, refreshToken, user }` | 200, 400, 401 |
| `GET /auth/me` | Current user profile | Bearer | Empty | `{ data: User }` | 200, 401 |
| `GET /institutions/{instId}` | Get institution details | Bearer | Empty | `{ data: Institution }` | 200, 401, 404 |
| `GET /institutions/{instId}/departments` | List departments | Bearer | Empty | `{ data: []Department }` | 200, 401 |
| `POST /institutions/{instId}/departments` | Create department | Admin | `{ name: string }` | `{ data: Department }` | 201, 400, 403 |
| `GET /institutions/{instId}/programs` | List programs | Bearer | Empty | `{ data: []Program }` | 200, 401 |
| `GET /institutions/{instId}/classes` | List classes | Bearer | Empty | `{ data: []Class }` | 200, 401 |
| `GET /institutions/{instId}/student-groups` | List student groups | Bearer | Empty | `{ data: []StudentGroup }` | 200, 401 |
| `GET /institutions/{instId}/subjects` | List subjects | Bearer | Empty | `{ data: []Subject }` | 200, 401 |
| `GET /institutions/{instId}/faculty` | List faculty | Bearer | Empty | `{ data: []Faculty }` | 200, 401 |
| `GET /institutions/{instId}/rooms` | List rooms | Bearer | Empty | `{ data: []Room }` | 200, 401 |
| `GET /institutions/{instId}/room-features` | List room features | Bearer | Empty | `{ data: []RoomFeature }` | 200, 401 |
| `GET /institutions/{instId}/time-slots` | List time slots | Bearer | Empty | `{ data: []TimeSlot }` | 200, 401 |
| `GET /institutions/{instId}/course-offerings`| List offerings | Bearer | Empty | `{ data: []CourseOffering }`| 200, 401 |
| `GET /institutions/{instId}/session-requirements`| List requirements | Bearer | Empty | `{ data: []SessionRequirement }`| 200, 401 |
| `GET /timetables` | List timetables | Bearer | Query: `?institutionId` | `{ data: []Timetable }` | 200, 401 |
| `POST /timetables` | Create timetable | Scheduler | `{ name: string }` | `{ data: Timetable }` | 201, 400 |
| `GET /timetables/{id}` | Read timetable | Bearer | Empty | `{ data: Timetable }` | 200, 404 |
| `POST /timetables/{id}/snapshots` | Create snapshot from live data | Scheduler | Empty | `{ data: ProblemSnapshot }` | 201, 404 |
| `GET /snapshots/{id}` | Read snapshot | Bearer | Empty | `{ data: ProblemSnapshot }` | 200, 404 |
| `GET /snapshots/{id}/problem` | Download canonical problem JSON | Bearer | Empty | `application/json` (`problem.Problem`) | 200, 404 |
| `POST /timetables/{id}/runs` | Create solver run | Scheduler | `{ snapshotId: UUID, solverConfig?: object }` | `{ data: ScheduleRun }` | 201, 400, 404 |
| `GET /runs/{id}` | Read run status/result | Bearer | Empty | `{ data: ScheduleRun }` | 200, 404 |
| `POST /runs/{id}/cancel` | Cancel running solver | Scheduler | Empty | `{ data: ScheduleRun }` | 200, 400, 404 |
| `GET /runs/{id}/verify` | Re-verify run result | Bearer | Empty | `{ data: VerificationReport }` | 200, 404 |
| `POST /timetables/{id}/versions` | Create draft version | Scheduler | `{ sourceRunId?: UUID, name: string }` | `{ data: ScheduleVersion }` | 201, 400 |
| `GET /versions/{id}` | Read version + assignments | Bearer | Empty | `{ data: ScheduleVersion }` | 200, 404 |
| `POST /versions/{id}/assignments/move` | Move assignment (validated) | Scheduler | `{ assignmentId, to: { roomId, timeSlotId } }` | `{ data: ValidateMoveResponse }` | 200, 400, 409 |
| `POST /versions/{id}/assignments/swap` | Swap 2 assignments (validated)| Scheduler | `{ assignment1Id, assignment2Id }` | `{ data: ValidateMoveResponse }` | 200, 400, 409 |
| `POST /versions/{id}/publish` | Publish reviewed version | Admin | Empty | `{ data: ScheduleVersion }` | 200, 400, 403 |
| `POST /verify` | Verify solution vs snapshot | Bearer | `{ snapshotId: UUID, versionId: UUID }` | `{ data: VerificationReport }` | 200, 400 |

---

## 6. API Contract Schemas

### Standard Envelope Schemas (`api/openapi.yaml`)

```yaml
Error:
  type: object
  required: [error]
  properties:
    error:
      type: object
      required: [code, message]
      properties:
        code:
          type: string
          enum: [VALIDATION_ERROR, UNAUTHORIZED, FORBIDDEN, NOT_FOUND, CONFLICT, UNPROCESSABLE, RATE_LIMITED, INTERNAL_ERROR]
        message:
          type: string
        details:
          type: array
          items:
            type: object
            properties:
              field: { type: string }
              message: { type: string }

Score:
  type: object
  properties:
    hardViolations: { type: integer }
    softPenalty: { type: integer }
    breakdown:
      type: object
      properties:
        studentGapPenalty: { type: integer }
        groupGaps:
          type: object
          additionalProperties: { type: integer }

Diagnostics:
  type: object
  properties:
    status: { type: string }
    nodesExplored: { type: integer }
    candidates: { type: integer }
    backtracks: { type: integer }
    message: { type: string }

Violation:
  type: object
  properties:
    constraintName: { type: string }
    severity:
      type: string
      enum: [HARD, SOFT, INFO]
    message: { type: string }
    assignmentId: { type: string }
    relatedIds:
      type: object
      additionalProperties: { type: string }
    metadata:
      type: object
      additionalProperties: { type: string }

VerificationReport:
  type: object
  properties:
    valid: { type: boolean }
    status: { type: string }
    violations:
      type: array
      items: { $ref: "#/components/schemas/Violation" }
    message: { type: string }
```

---

## 7. Solver Pipeline

Located in `internal/scheduler/engine/engine.go`:

```
Request (Problem, Constraints, SolveOptions, TabuOptions, ObjectiveConfig)
  │
  ├─ 1. Problem.Validate() ──> If violations: return INVALID_PROBLEM
  ├─ 2. Problem.Prepare()  ──> Index slots and faculty/room availability maps
  ├─ 3. Problem.PreSolve() ──> Detects 0-domain requirements, room bottleneck: return INFEASIBLE
  ├─ 4. constraints.Compile() ──> Compiles ConstraintInstances, computes SHA-256 RuleSetHash
  │
  ├─ 5. CSP Feasibility Phase (`backtracking.Solve`)
  │      - Seeds LockedAssignments
  │      - Explores search tree with MRV + Degree + LCV + Forward Checking
  │      - Respects context.Deadline and MaxNodes limit
  │      - Returns feasible baseline solution (0 hard violations)
  │
  ├─ 6. Tabu Optimization Phase (`localsearch.TabuSearcher.Search`)
  │      - Skipped if req.DisableOptimize == true
  │      - Delta scoring on StudentGapPenalty
  │      - Strict MoveValidator checks (hard violations rejected)
  │      - Respects context.Done() and MaxIterations / NoImprovementLimit
  │      - Returns optimized best solution
  │
  └─ 7. Authoritative Verification (`verifier.VerifySolution`)
         - Independent pure read-only check
         - Recomputes exact soft penalty breakdown
         - Emits final Response
```

---

## 8. Hard Constraints

Every hard constraint in CURRA is strictly enforced during CSP search, Tabu move validation, and authoritative verification:

1. **Faculty Conflict (`FacultyConflict`):** A faculty member cannot be scheduled to teach more than one session in the same time slot.
2. **Room Conflict (`RoomConflict`):** A room cannot host more than one session in the same time slot.
3. **Student Group Conflict (`StudentGroupConflict`):** A student group (or any overlapping subgroup/whole group) cannot attend more than one session in the same time slot.
4. **Room Capacity (`RoomCapacity`):** The scheduled room's capacity must be greater than or equal to the student group size.
5. **Room Features (`RoomFeatures`):** The room must satisfy all required room feature tags requested by the course offering and session requirement.
6. **Faculty Availability (`FacultyAvailability`):** Sessions can only be scheduled in time slots explicitly present in the faculty member's allow-list.
7. **Room Availability (`RoomAvailability`):** Sessions can only be scheduled in time slots explicitly present in the room's allow-list.
8. **Consecutive Period Boundaries (`ConsecutivePeriods`):** Multi-period sessions (e.g. Duration = 2) must be scheduled in consecutive periods on the same day without crossing daily period boundaries (`Period + duration - 1 <= PeriodsPerDay`).
9. **Daily Subject Limit (`DailySubjectLimit`):** A student group cannot have more than $N$ sessions (default 1) of the same subject on the same day.
10. **Locked Assignments (`LockedAssignments`):** Pinned assignments must be placed at their exact specified room and time slot, and cannot be relocated by search or optimization.

---

## 9. Soft Constraints / Scoring

Located in `internal/scheduler/scorer/scorer.go`:

### Implemented Objective: Student Gap Penalty (`StudentGapPenalty`)
- **Objective ID:** `StudentGapPenalty` (default weight = 1).
- **Formula:** For each student group on each day:
  $$\text{Gap} = (\text{LastPeriod} - \text{FirstPeriod} + 1) - \text{TotalOccupiedPeriods}$$
  $$\text{TotalSoftPenalty} = \sum_{\text{group}, \text{day}} \text{Gap} \times \text{Weight}$$
- **Incremental Scoring:** `localsearch.IncrementalScoreEvaluator` maintains per-group, per-day period bitsets and calculates $\Delta\text{Score}$ in $O(1)$ without rescanning all assignments.

### NOT Implemented Soft Constraints
- Faculty teaching preference slot penalties (`model.FacultyPreference` exists in domain catalogs, but soft scoring evaluation is currently NOT implemented/active in the scorer).
- Room change minimization for classes.
- Balanced daily distribution of faculty workload.

---

## 10. CSP Solver

Located in `internal/scheduler/solver/backtracking/`:

- **Variable Selection:**
  - **MRV (Minimum Remaining Values):** Selects the unassigned session requirement with the smallest remaining candidate placement domain.
  - **Degree Heuristic (Tie-Breaker):** Selects the variable involved in the largest number of constraints with other unassigned variables (overlapping faculty, student groups, and room feature competition).
  - **Lexicographical Tie-Breaker:** Breaks ties deterministically via `decisionLess`.
- **Value Ordering:**
  - **LCV (Least Constraining Value):** Orders valid placements by minimizing the number of eliminations caused in neighbor domains.
- **Inference & Domain Pruning:**
  - **Forward Checking (FC):** Prunes neighbor domains immediately upon tentative assignment.
- **State Management & Invariants:**
  - In-place mutation of `problem.Solution` with `SolutionIndex` update and exact rollback on backtrack.
  - Returns `diagnostics.SolveStatusSolved`, `diagnostics.SolveStatusInfeasible`, `diagnostics.SolveStatusTimeout` (`DEADLINE_EXCEEDED`), or `diagnostics.SolveStatusNodeLimit`.

---

## 11. Tabu / Local Search Solver

Located in `internal/scheduler/solver/localsearch/`:

- **Initialization:** Receives complete feasible solution from CSP solver (0 hard violations).
- **Neighborhood Operators:**
  1. `SingleMove`: Reassigns a single assignment to an unoccupied (Room, TimeSlot).
  2. `TwoWaySwap`: Swaps the placements of two assignments.
- **Move Validation:** `MoveValidator` checks candidate moves against all hard constraints. Any move introducing hard violations is rejected ($O(1)$ indexed checks).
- **Tabu List & Tenure:** Circular memory buffer tracks reverse move signatures (`AssignmentID -> OldPlacement`) for $T$ iterations (`TabuTenure`, default 10).
- **Aspiration Criterion:** A tabu move is accepted if it strictly improves upon the global best soft score found so far.
- **Termination Criteria:**
  - `MaxIterations` reached (default 1000).
  - `NoImprovementLimit` reached (default 100).
  - `context.DeadlineExceeded` / `context.Canceled`.
  - `MaxDuration` elapsed.

---

## 12. Validation & Verification

### Distinct Validation Layers
1. **Input Structural Validation (`problem.Validate`):** Pure static check ensuring foreign keys exist, capacities $> 0$, durations $> 0$, and grid boundaries are valid. Returns `INVALID_PROBLEM`.
2. **Pre-Solve Feasibility Analysis (`problem.PreSolve`):** Fast bottleneck check identifying offerings with 0 matching rooms, overloaded faculty hours, or overbooked rooms before starting CSP. Returns `INFEASIBLE`.
3. **Search-Time Checking:** Incremental constraint pruning in CSP and `MoveValidator` in Tabu Search.
4. **Authoritative Solution Verification (`verifier.VerifySolution`):** Pure read-only 14-point audit:
   - Requirement completeness (every requirement scheduled for exact required sessions).
   - Total assignment count matches sum of requirement instances.
   - Assignment ID uniqueness (`RequirementID#Instance`).
   - TimeSlot and Room catalog existence.
   - Grid duration fit (`Period + duration - 1 <= PeriodsPerDay`).
   - Locked assignment exact preservation.
   - Faculty conflict freedom.
   - Room conflict freedom.
   - Student group conflict freedom.
   - Room capacity satisfaction.
   - Room feature requirement satisfaction.
   - Faculty availability allow-list satisfaction.
   - Room availability allow-list satisfaction.
   - Score integrity (recomputed soft penalty matches reported score).

### Solve Status Enums (`diagnostics.SolveStatus`)
- `SOLVED`: Feasible and verified solution found.
- `INFEASIBLE`: Problem is provably impossible to solve without constraint violations.
- `INVALID_PROBLEM`: Problem catalog or constraint configuration failed structural validation.
- `INVALID_RESULT`: Engine or solver produced a corrupted or unverified output.
- `CANCELLED`: Operation cancelled via context cancellation.
- `DEADLINE_EXCEEDED`: Search exceeded allocated time deadline.
- `NODE_LIMIT`: CSP search exceeded `MaxNodes` budget.

---

## 13. Invariants & Guarantees

### Explicitly Enforced Invariants
- **Feasibility Guarantee:** `engine.Solve` NEVER returns status `SOLVED` if there is a single hard constraint violation.
- **Locked Assignment Guarantee:** Pinned assignments in `p.LockedAssignments` are seeded first in CSP and cannot be moved by Tabu Search.
- **Authoritative Verification Gate:** No solution is emitted from `engine.Solve` without passing `verifier.VerifySolution`.

### Tested Invariants
- **Undo Invariant:** `ApplyMove` followed by `UndoMove` (and `ApplySwap` followed by `UndoSwap`) restores byte-exact `SolutionIndex` state.
- **Delta Scoring Parity:** `IncrementalScoreEvaluator` delta scores exactly match `scorer.CalculateScore` on all randomized moves and swaps.
- **Replay Invariant:** Executing `Solve()` twice on identical problem data with identical seeds produces byte-identical assignment outputs.

---

## 14. Determinism

CURRA guarantees **100% deterministic replay**:
- **Elimination of Go Map Randomness:** All map iteration in CSP heuristics and problem preparation iterates over pre-sorted lexicographical slices of keys.
- **Seeded PRNG:** Tabu Search initializes a dedicated `rand.New(rand.NewSource(opts.Seed))`.
- **Tie-Breaking:** Variable and value selection ties are resolved using deterministic string comparators (`decisionLess`).

---

## 15. Error Model

- **Sentinel Errors:**
  - `backtracking.ErrNoSolution`: Infeasible problem instance.
  - `backtracking.ErrTimeout`: CSP deadline exceeded.
  - `backtracking.ErrNodeLimit`: CSP node limit exhausted.
  - `verifier.ErrInvalidResult`: Structural corruption in solution.
  - `verifier.ErrHardConstraintViolation`: Verification failed hard rules.
- **HTTP Status Code Mapping in Application Layer:**
  - `INVALID_PROBLEM` $\rightarrow$ `HTTP 400 Bad Request`
  - `INFEASIBLE` $\rightarrow$ `HTTP 422 Unprocessable Entity`
  - `DEADLINE_EXCEEDED` $\rightarrow$ `HTTP 200 OK` (with status `DEADLINE_EXCEEDED` in body)
  - `CANCELLED` $\rightarrow$ `HTTP 200 OK` (with status `CANCELLED` in body)
  - `CONFLICT` $\rightarrow$ `HTTP 409 Conflict` (optimistic lock mismatch)

---

## 16. Backend Integration

Located in `application/internal/curra/`:
- **Boundary Adapter (`CurraAdapter`):** Stateless Go adapter wrapping `engine.Solve`, `verifier.VerifySolution`, `constraints.Compile`.
- **DTO Isolation:** Application stores problem snapshots and solution results as `json.RawMessage` / JSONB. No direct dependency on internal solver state.
- **Clone-Before-Mutate:** The adapter performs `sol.Clone()` before applying manual user moves, preventing mutable corruption of cached solutions.

---

## 17. Frontend Integration

Defined by `contracts/api-contract.md` and `api/openapi.yaml`:
- **UI Tooling:** Visual timetable grid with drag-and-drop manual editing.
- **Manual Edit Workflow:** UI sends `POST /versions/:id/assignments/move`. Adapter clones solution, tests move via `verifier.VerifySolution`, and returns verified response or violation list.
- **State Machine UI:** Enforces version workflow: `DRAFT` $\rightarrow$ `REVIEW` $\rightarrow$ `PUBLISHED` (with rollback to `DRAFT` via `send-back`).

---

## 18. Database / Persistence

PostgreSQL 16+ DDL defined in `database/schema.md`:
- **Multi-Tenant Isolation:** Every table includes `institution_id UUID NOT NULL REFERENCES institutions(id)`.
- **Optimistic Locking:** All mutable entity tables have `version INT NOT NULL DEFAULT 1`.
- **Core Tables:**
  - `institutions`, `users`, `user_roles`
  - `departments`, `programs`, `classes`, `student_groups`, `subjects`, `faculty`, `rooms`, `room_features`, `time_slots`
  - `course_offerings`, `session_requirements`, `faculty_availabilities`, `room_availabilities`, `faculty_preferences`
  - `timetables`, `problem_snapshots` (`problem JSONB`), `schedule_runs`, `schedule_versions`, `schedule_assignments`, `assignment_pins`, `import_batches`, `audit_events`.

---

## 19. Tests

### Test Organization
1. **Package-Local Unit Tests (`internal/scheduler/...`):**
   - `constraints/constraints_test.go`, `constraints/membership_test.go`
   - `scorer/scorer_test.go`, `scorer/weighted_scoring_test.go`
   - `problem/invariants_test.go`, `problem/locked_assignments_test.go`
   - `solver/backtracking/backtracking_test.go`, `solver/backtracking/lcv_test.go`
   - `solver/localsearch/localsearch_test.go`, `solver/localsearch/tabu_search_test.go`, `solver/localsearch/incremental_scorer_test.go`, `solver/localsearch/tabu_search_bench_test.go`
   - `verifier/verifier_test.go`
   - `engine/engine_test.go`, `engine/engine_hardening_test.go`
2. **System Integration Suites (`tests/`):**
   - `tests/benchmark_itc2007_test.go` (ITC 2007 CB-CTT benchmark instances)
   - `tests/csp_heuristic_investigation_test.go` (CSP heuristic ablation harness)
   - `tests/performance_baseline_test.go` (Throughput & memory scaling measurements)
   - `tests/hardening_invariants_test.go` (Transactional invariant & rollback tests)
   - `tests/randomized_invariant_test.go` (Property-based randomized fuzzer across seeds)
   - `tests/stress_pathological_test.go` (Overconstrained and tight-availability stress tests)

---

## 20. Performance Baselines

Extracted from `tests/performance_baseline_test.go` and `tests/benchmark_itc2007_test.go`:

| Problem Instance | Sessions | Faculty | Rooms | Slots | CSP Time | Tabu Time (25 iters) | Full Score Eval / Op |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Small** | 24 | 6 | 4 | 30 | ~50 ms | ~12 ms | 12.7 µs |
| **Medium** | 300 | 50 | 30 | 30 | ~270 ms | ~35 ms | 222.6 µs |
| **Large** | 3000 | 300 | 150 | 40 | N/A | N/A | 2.31 ms |

---

## 21. Current Hardening Status

- **RESOLVED:**
  - Incremental scoring bitset sync parity across moves and swaps.
  - CSP and Tabu transactional rollback on failed search branches.
  - Elimination of map iteration non-determinism.
  - Package-local test co-location and centralized test fixtures.
  - Context cancellation timing in Tabu search.
- **OPEN / ONGOING:**
  - `FacultyPreference` soft objective implementation.
- **KNOWN / ACCEPTED:**
  - `-race` flag execution requires 64-bit MinGW toolchain on Windows host environments.
- **OUT OF SCOPE:**
  - Dynamic multi-room lab splits across non-adjacent periods.

---

## 22. Recent Structural Reorganization

In the latest architectural refactoring:
- All 16 unit test files previously placed in the root `tests/` directory were migrated into their respective Go packages under `internal/scheduler/...`.
- Shared test generators and problem instances were consolidated into [internal/scheduler/testutil/fixtures.go](file:///c:/Users/Preetham%20S/timetable-platform/internal/scheduler/testutil/fixtures.go).
- Root `tests/` was preserved strictly for cross-cutting system integration suites, property fuzzers, and benchmarks.
- Production solver behavior was verified to have **zero algorithmic, behavioral, or semantic changes**.

---

## 23. Known Important Decisions

1. **Two-Stage CSP $\rightarrow$ Tabu Architecture:** Separate feasibility satisfaction (CSP) from soft objective minimization (Tabu Search) to guarantee conflict-free schedules.
2. **Authoritative Verifier Gate:** Pure read-only verification pass runs outside solver search loops to prevent solver bugs from emitting invalid timetables.
3. **Stateless Adapter Boundary:** Downstream application code interacts with CURRA strictly through JSON DTOs and pure adapter functions without shared mutable memory.

---

## 24. Open Problems / TODO

- **Important:** Implement soft preference scoring for `model.FacultyPreference` in `internal/scheduler/scorer/scorer.go`.
- **Nice-to-Have:** Room change minimization soft penalty component.
- **Deferred:** Date-range holiday awareness and calendar break scheduling.

---

## 25. What NOT to Change

1. **Solver Determinism & PRNG Contracts:** Do not replace seeded PRNGs with unseeded instances or iterate Go maps directly.
2. **Authoritative Verification Rules:** Never weaken or bypass `verifier.VerifySolution`.
3. **Hard Constraint Feasibility Guarantee:** Never allow Tabu search or manual edits to commit a state with `HardViolations > 0`.
4. **Adapter Boundary Isolation:** Do not import `internal/scheduler/...` packages outside `application/internal/curra/`.

---

## 26. Quick Reference

```text
Repository:         github.com/sPreetham42/timetable-platform
Go Version:         1.22+
Core Architecture:  Two-Stage (CSP Backtracking Feasibility -> Tabu Search Local Optimization)

Core Packages:
  - Orchestrator:   internal/scheduler/engine
  - CSP Solver:     internal/scheduler/solver/backtracking
  - Local Search:   internal/scheduler/solver/localsearch
  - Verifier:       internal/scheduler/verifier
  - Constraints:    internal/scheduler/constraints
  - Scorer:         internal/scheduler/scorer
  - Domain Model:   internal/scheduler/model
  - Problem State:  internal/scheduler/problem
  - Test Fixtures:  internal/scheduler/testutil

Backend Adapter:    application/internal/curra
Database Schema:    PostgreSQL 16+ (database/schema.md)
REST API Contract:  OpenAPI 3.1.0 (api/openapi.yaml)

Solver Statuses:    SOLVED | INFEASIBLE | INVALID_PROBLEM | INVALID_RESULT | CANCELLED | DEADLINE_EXCEEDED | NODE_LIMIT
Current Status:     Solver Engine Frozen (v1.0.0) | Application Contract Frozen (v1) | All Tests Passing (100%)
```
