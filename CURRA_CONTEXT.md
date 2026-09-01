# CURRA Engineering Context & System Specification

> **Target Audience:** Claude Code implementation and hardening agents.  
> **Repository Authority:** Current Go source code, test suites, contracts, and schema as of **2026-09-01**.  
> **Specification Stance:** Grounded strictly in repository evidence. Distinguishes *Current Implementation*, *Explicit Intent*, *Inferred Design Decisions*, *Known Issues*, and *Open Questions*.

---

## 1. System Identity

### Current Implementation vs. Stated Identity
* **Stated Intent:** *"CURRA is an industry-grade timetable scheduling engine written in Go using a two-stage process: (1) CSP Backtracking for hard constraints, (2) Tabu Search for soft objectives."*
* **Current Implementation:** Verified. The core scheduling engine in `internal/scheduler` implements a deterministic two-stage pipeline:
  1. **Stage 1 (CSP):** Systematic backtracking with Minimum Remaining Values (MRV), Degree heuristic, Least Constraining Value (LCV) ordering, and Forward Checking (domain pruning) enforcing all hard constraints.
  2. **Stage 2 (Local Search):** Tabu Search neighborhood exploration performing single and swap moves that optimize soft objectives while strictly rejecting any move violating hard constraints.
  3. **Stage 3 (Independent Verification):** Pure read-only verification pass (`internal/scheduler/verifier`) proving structural integrity, foreign-key validity, locked assignment preservation, hard constraint compliance, and score consistency before returning `SOLVED`.
* **Platform Scope:** The repository contains both the pure Go solver core (`internal/scheduler/`, root `go.mod`) and a surrounding multi-tenant platform backend (`application/`, separate `go.mod`, PostgreSQL, REST API) with a React frontend (`web/`).

---

## 2. Repository Organization & Module Boundaries

```
Curra/
├── go.mod                        # Root module: github.com/sPreetham42/timetable-platform (Go 1.22+, zero deps)
├── cmd/solver/main.go            # Standalone solver CLI
├── internal/scheduler/           # CORE SOLVER ENGINE (Frozen boundary)
│   ├── model/                    # Domain IDs, Entities, TimeSlots
│   ├── problem/                  # Problem, Solution, SolutionIndex, Validation, Presolve, Moves
│   ├── constraints/              # Legacy & compiled constraint framework
│   ├── scorer/                   # Multi-objective scoring (Student gaps, Faculty pref, Room change)
│   ├── solver/
│   │   ├── backtracking/         # CSP solver (MRV, Degree, LCV, Forward Checking)
│   │   └── localsearch/          # Tabu Search optimizer (Single/Swap moves, Tabu list, Aspiration)
│   ├── verifier/                 # Authoritative independent solution verifier
│   ├── engine/                   # Canonical pipeline orchestrator (Solve)
│   ├── diagnostics/              # SolveStatus, Severity, structured Violations
│   └── testutil/                 # Shared fixtures & test problem generators
├── tests/                        # Integration, stress, fuzz, baseline, benchmark suites
├── contracts/                    # Frozen architectural boundaries (curra-adapter.md, api-contract.md)
├── docs/                         # State machines, agent boundaries
├── api/                          # OpenAPI 3.0 specification & mocking fixtures
├── database/                     # PostgreSQL schema specification & dev seed script
├── application/                  # PLATFORM BACKEND (Separate Go module)
│   ├── go.mod                    # Depends on root module via replace (../) + pgx, mux, uuid
│   ├── cmd/server/ & cmd/worker/ # HTTP API server and background solver worker daemons
│   ├── internal/api/             # Handlers, router, JWT middleware
│   ├── internal/curra/           # CURRA ADAPTER (SOLE permitted importer of internal/scheduler)
│   ├── internal/database/        # Schema migrations (20+ tables) & pgx repositories
│   ├── internal/domain/          # Application-level entities & DTOs
│   ├── internal/services/        # Business workflows (run, snapshot, timetable, version, move)
│   └── boundary_test.go          # Mechanical AST test forbidding curra imports outside adapter
└── web/                          # FRONTEND (React, TypeScript, Vite, Tailwind CSS)
```

### Critical Dependency Rules
1. **Zero External Dependencies in Solver Core:** Root `go.mod` has no third-party libraries. All solver algorithms, data structures, and tests rely strictly on the Go standard library.
2. **Strict Application Boundary:** Only `application/internal/curra` is permitted to import `internal/scheduler/...`. This rule is mechanically enforced by `application/boundary_test.go` via AST parsing.
3. **Immutability across Boundaries:** The adapter deserializes input JSON into `problem.Problem`, calls `problem.Solution.Clone()` before applying mutations, and validates every manual move through `verifier.VerifySolution`.

---

## 3. System Architecture & Lifecycle

### Architectural Diagram

```
                        CURRA ENGINE EXECUTION PIPELINE
                                (engine.Solve)
                                      │
    ┌─────────────────────────────────┴─────────────────────────────────┐
    │                                                                   │
    ▼                                                                   ▼
[problem.Validate]                                             [constraints.Compile]
Validates catalog integrity,                                   Validates & sorts rules,
hierarchy, bounds, references                                  produces SHA-256 RuleSetHash
    │                                                                   │
    └─────────────────────────────────┬─────────────────────────────────┘
                                      ▼
                             [problem.Prepare]
                   Builds derived lookup indexes:
                   - SlotsByDayPeriod (O(1) slot math)
                   - FacultyAvailable / RoomAvailable bitsets
                   - StudentGroupOverlaps (Hierarchy graph)
                                      │
                                      ▼
                             [problem.PreSolve]
                   Fast feasibility bottleneck analysis:
                   - Zero-domain check for requirements
                   - Faculty slot demand vs. availability
                   - Specialized room feature supply vs. demand
                                      │
                                      ▼
                        [STAGE 1: CSP Backtracking]
                   Variable Ordering: MRV + Degree heuristic
                   Value Ordering:    LCV (Least Constraining Value)
                   Pruning:           Forward Checking (domain pruning)
                   State:             SolutionIndex (O(1) conflict detection)
                                      │
                             (Feasible Timetable)
                                      ▼
                         [STAGE 2: Tabu Local Search]
                   Candidate Generator: Unlocked Single Moves & Swaps
                   Move Validator:     Hard constraints (never breached)
                   Delta Scorer:       IncrementalScoreEvaluator
                   Memory:             TabuList (tenure + reverse signature)
                   Aspiration:         Global best score override
                                      │
                             (Optimized Timetable)
                                      ▼
                      [STAGE 3: Authoritative Verifier]
                   (verifier.VerifySolution - Pure Read-Only)
                   - Requirement completeness (no missing/excess sessions)
                   - ID uniqueness & foreign-key existence
                   - Locked assignment preservation
                   - Re-evaluates ALL hard constraints from raw assignments
                   - Authoritative independent soft score recalculation
                                      │
                                      ▼
                           [engine.Response Output]
                   Status: SOLVED | INFEASIBLE | INVALID_PROBLEM |
                           INVALID_RESULT | CANCELLED | DEADLINE_EXCEEDED |
                           NODE_LIMIT
```

---

## 4. Domain Model

Located in `internal/scheduler/model/`:

| Entity | ID Type | Key Fields | Invariants & Edge Cases |
| :--- | :--- | :--- | :--- |
| **Department** | `DepartmentID` | `TenantID`, `Name` | Root grouping for academic programs within a tenant. |
| **Program** | `ProgramID` | `DepartmentID`, `Name` | References a valid department. |
| **Class** | `ClassID` | `ProgramID`, `WholeGroupID`, `StudentGroupIDs` | `WholeGroupID` must belong to `ClassID` and be present in `StudentGroupIDs`. Subgroups must also point to `ClassID`. |
| **StudentGroup** | `StudentGroupID` | `ClassID`, `Name`, `Size` | `Size >= 0`. Used for capacity checks against room capacity. Overlaps with whole class group and identical ID. |
| **Subject** | `SubjectID` | `Code`, `Name` | Academic discipline identifier. Taught via `CourseOffering`. |
| **CourseOffering** | `CourseOfferingID` | `TermID`, `ClassID`, `SubjectID`, `StudentGroupID`, `FacultyID`, `RequiredRoomFeatureIDs`, `SessionRequirementIDs` | Schedulable course unit. Must reference active term, valid faculty, class, and at least one session requirement. |
| **SessionRequirement** | `SessionRequirementID` | `CourseOfferingID`, `Type` (`THEORY`/`LAB`), `SessionsPerWeek`, `Duration`, `Consecutive`, `RequiredRoomFeatureIDs` | `SessionsPerWeek > 0`, `Duration > 0`. `Duration <= PeriodsPerDay`. Each session consumes `Duration` consecutive slots starting at a grid period. |
| **Faculty** | `FacultyID` | `Name` | Instructor assigned to course offerings. Cannot be double-booked across overlapping time slots. |
| **FacultyAvailability** | — | `FacultyID`, `TimeSlotID` | Allow-list model. If availability entries exist, faculty can only teach during listed slots. |
| **FacultyPreference** | — | `FacultyID`, `TimeSlotID`, `Weight` | Soft preference penalty. Scheduling faculty during this slot incurs `Weight` penalty. |
| **Room** | `RoomID` | `Name`, `Capacity`, `FeatureIDs` | `Capacity >= 0`. Must satisfy student group size. Cannot be double-booked. |
| **RoomFeature** | `RoomFeatureID` | `Name` | Specialized feature (e.g. `PROJECTOR`, `LAB_EQUIPMENT`). |
| **RoomAvailability** | — | `RoomID`, `TimeSlotID` | Allow-list model. Room is usable only when available. |
| **TimeSlot** | `TimeSlotID` | `Day` (`time.Weekday`), `Period` (`int`), `Label` | Recurring weekly slot. `Period >= 1`. `Key() -> SlotKey{Day, Period}` must be unique across the problem. |
| **Term** | `TermID` | `TenantID`, `Name` | Academic term boundary. All course offerings must reference the active term. |

### Assignment & Solution Types (`internal/scheduler/problem/`)
* **`AssignmentID`**: Formatted deterministically as `<SessionRequirementID>#<Instance>` (e.g., `req-cs101-theory#0`).
* **`Assignment`**: Tuple of `(ID, CourseOfferingID, StudentGroupID, FacultyID, RoomID, TimeSlotID, SessionRequirementID, Instance)`.
* **`Placement`**: Pair of `(RoomID, TimeSlotID)`.
* **`Move`**: Mutation descriptor: `(AssignmentID, From Placement, To Placement)`.
* **`Solution`**: Container of `[]Assignment`, `SolutionIndex`, and `scorer.Score`.
* **`SolutionIndex`**: In-memory O(1) conflict lookup structures:
  * `FacultySlot: map[resourceSlotKey]AssignmentID`
  * `RoomSlot: map[resourceSlotKey]AssignmentID`
  * `StudentGroupSlot: map[resourceSlotKey]AssignmentID`
  * `RequirementCount: map[SessionRequirementID]int`
  * `byID: map[AssignmentID]Assignment`

---

## 5. Validation Subsystem

**Authoritative File:** `internal/scheduler/problem/validation.go -> Validate(p Problem) []diagnostics.Violation`

### Validation Execution Contract
* **Purity:** Pure structural check run before `Prepare()` or solver execution.
* **Determinism:** Map iteration order is explicitly neutralized by collecting and sorting all entity IDs prior to checking.
* **Scope of Invariants Checked:**
  1. Non-empty TenantID and `PeriodsPerDay > 0`.
  2. Entity ID non-empty and catalog foreign-key integrity (Programs -> Departments, Classes -> Programs, Groups -> Classes).
  3. Class hierarchy consistency: `WholeGroupID` belongs to class, is listed in `StudentGroupIDs`, and no duplicate group IDs exist.
  4. Course offering relationships: term matches `p.Term.ID`, class and group match, faculty exists, session requirements belong to offering.
  5. Session requirements: `SessionsPerWeek > 0`, `0 < Duration <= PeriodsPerDay`, valid session type (`THEORY`/`LAB`), required room features exist.
  6. Room integrity: `Capacity >= 0`, room feature references valid.
  7. TimeSlot grid integrity: `Period > 0`, no duplicate `(Day, Period)` pairs.
  8. Availability & preference foreign-key validity against faculty, rooms, and timeslots.
  9. Non-empty catalog check: `len(Rooms) > 0`, `len(TimeSlots) > 0`, `Term.ID != ""`.
  10. Locked assignment validity: matches catalog, offering faculty, offering group, duration fits grid, room capacity >= group size, room provides required features, faculty and room are available, total locked per requirement `<= SessionsPerWeek`, and no two locked assignments conflict.

### Pre-Solve Feasibility Analysis
**Authoritative File:** `internal/scheduler/problem/presolver.go -> PreSolve(p *Problem) []diagnostics.Violation`
* Runs immediately after validation and `p.Prepare()`.
* Detects provably unsolvable problems without tree search:
  1. `checkZeroDomain`: Proves whether every session requirement has at least `SessionsPerWeek` legal placements considering capacity, features, faculty availability, and room availability.
  2. `checkFacultyOverload`: Proves whether total sessions demanded of a faculty member exceed their total available slot count.
  3. `checkRoomFeatureBottleneck`: Verifies that total demand for specialized room features does not exceed existing room supply.

---

## 6. Compilation & Preprocessing Subsystem

### Purpose & Architecture
Preprocessing transforms raw problem input into high-performance index structures and validates declarative rulesets.

### Derived Problem Indexes (`problem.Problem.Prepare()`)
1. **`SlotsByDayPeriod` (`map[model.SlotKey]model.TimeSlotID`)**: Allows O(1) multi-period session expansion (`OccupiedSlotIDs(startSlot, duration)`).
2. **`FacultyAvailable` / `RoomAvailable` (`map[ID]map[TimeSlotID]struct{}`)**: Pre-populates allow-lists from raw slices for instant availability verification.
3. **`StudentGroupOverlaps` (`map[GroupID]map[GroupID]struct{}`)**: Precomputes student cohort intersection graph:
   * A whole class group overlaps with itself and all subgroups in that class.
   * Disjoint subgroups within the same class do not overlap with each other unless explicitly linked.

### Constraint Compilation (`constraints.Compile`)
**Authoritative File:** `internal/scheduler/constraints/framework.go -> Compile(p, instances)`
* Canonicalizes declarative `ConstraintInstance` records:
  * Sorts instances by `(ID, TemplateID)`.
  * Computes deterministic SHA-256 `RuleSetHash`.
  * Validates parameter types and cross-references against problem entities (when `p != nil`).
  * Instantiates compiled `ConstraintDef` implementations into `CompiledConstraintSet{Hard, Soft, RuleSetHash}`.
* **Compilation Safety Guard:** Currently, `inst.Kind == ConstraintKindSoft` returns a compile error (`"soft constraints are not supported by the current scoring engine"`). All soft objectives are managed through the scoring subsystem.

---

## 7. Constraint Architecture

### Dual Constraint Subsystem
CURRA contains two coexisting constraint paradigms:
1. **Legacy Interface (`constraints.Constraint` / `constraints.ScopedValidator`):**
   * Interface: `Check(p, sol, a) []diagnostics.Violation` and `CheckAtSlot(...)`.
   * Standard 7 built-ins: `FacultyConflict`, `RoomConflict`, `StudentGroupConflict`, `RoomCapacity`, `FacultyAvailability`, `RoomAvailability`, `RoomFeatureCompatibility`.
2. **Compiled Framework Interface (`constraints.ConstraintDef`):**
   * Interface:
     ```go
     type ConstraintDef interface {
         ID() string
         Kind() ConstraintKind
         IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool
         ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation
         Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation
     }
     ```
   * Supports configurable rules (e.g., `SubjectMaxPerDayConstraint`) and wraps the built-in templates.
   * `SearchCtx` provides access to the problem and hierarchical `MembershipIndex`.

### Hard vs. Soft Execution
* **Hard Constraints:** Enforced strictly across all stages:
  * CSP backtracking rejects inconsistent assignments (`isAssignmentConsistent`).
  * Tabu search rejects any candidate move where `MoveValidator.IsLegal` fails.
  * Verifier re-evaluates all hard constraints on the full solution.
* **Soft Constraints:** Evaluated as optimization objectives in `internal/scheduler/scorer`:
  1. `StudentGapPenalty` (Weight default: 1): Penalizes idle periods between sessions on the same day for student groups.
  2. `FacultyPreference` (Weight default: 1): Penalizes scheduling faculty in slots marked with preference weight.
  3. `RoomChange` (Weight default: 1): Penalizes student groups switching rooms across consecutive sessions on the same day.

---

## 8. CSP Backtracking Solver

**Authoritative File:** `internal/scheduler/solver/backtracking/backtracking.go`

### Algorithmic Pipeline
1. **Seeding:** Seeds all `p.LockedAssignments` into `problem.Solution` via `solution.AddAssignment(&p, locked)`.
2. **Decisions Formulation:** Formulates unassigned session decisions sorted deterministically by:
   `Duration DESC -> CourseOfferingID ASC -> SessionRequirementID ASC -> Instance ASC`
3. **Variable Selection (MRV + Degree):**
   * **MRV (Minimum Remaining Values):** Selects unassigned variable with smallest domain size (`len(domains[i])`).
   * **Degree Heuristic (Tie-Breaker):** Selects variable with highest count of unassigned conflicting neighbors (sharing faculty, overlapping student groups, or competing for identical room features via `buildRoomConflictMap`).
   * **Deterministic Fallback:** Breaks remaining ties using `decisionLess`.
4. **Value Ordering (LCV):**
   * When `SearchModeHeuristicLCV` is enabled, candidate placements are sorted by `countEliminations`: values that rule out the fewest candidate choices in remaining unassigned variables are tried first.
5. **Forward Checking & Pruning:**
   * After each assignment, `pruneDomains` removes inconsistent placements from all remaining unassigned decision variables.
   * If any remaining variable has an empty domain (`len(filtered) == 0`), the branch fails immediately without deeper recursion.
6. **State Restoration:** In-place backtracking using `solution.RemoveLastAssignment(p)`.
7. **Search Limits & Context Checks:**
   * `diag.NodesExplored >= options.MaxNodes` aborts search with `ErrNodeLimit` (`NODE_LIMIT`).
   * `ctx.Err()` aborts search with `context.Canceled` (`CANCELLED`) or `context.DeadlineExceeded` (`DEADLINE_EXCEEDED`).

---

## 9. Tabu Search Optimizer

**Authoritative File:** `internal/scheduler/solver/localsearch/tabu_search.go`

### Algorithmic Pipeline
1. **Pre-condition Feasibility Check:** Initial solution is validated against all hard constraints. If violations exist, returns `ErrInitialSolutionInfeasible` immediately.
2. **Neighborhood Generation (`neighborhood.go`):**
   * Generates up to `MaxCandidates` random moves on unlocked assignments:
     * **Single Move (50% probability):** Relocates an assignment to an alternative room/slot.
     * **Swap Move (50% probability):** Swaps placements of two unlocked assignments.
   * All selections use deterministically seeded PRNG (`opts.Seed`).
3. **Move Validation (`validator.go`):**
   * Evaluates legality of single moves and swaps without full cloning using transactional apply-check-undo operations on `SolutionIndex`.
   * Evaluates both legacy built-in validators and compiled `ConstraintDef` rules.
4. **Objective Scoring & Delta Evaluation (`incremental_evaluator.go`):**
   * Computes soft penalty delta using `IncrementalScoreEvaluator` for student gaps, faculty preferences, and room changes.
5. **Tabu Memory & Aspiration Criteria (`tabu_list.go`):**
   * Moves matching active tabu signatures (`assignmentId:fromRoom,fromSlot->toRoom,toSlot`) within `TabuTenure` iterations are rejected.
   * **Aspiration Override:** If a tabu move achieves a soft score strictly better than the global best score (`res.Score.SoftPenalty < bestScore.SoftPenalty`), tabu status is bypassed.
   * Accepted moves record their `ReverseSignature()` into the tabu list.
6. **Termination Criteria:**
   * `NoImprovementLimit` reached (default: 100 iterations without improving global best).
   * `MaxIterations` reached (default: 1,000).
   * `MaxDuration` elapsed.
   * `ctx.Done()` triggered (returns best solution found so far).
7. **Post-optimization Feasibility Assurance:** Final solution is independently re-verified against hard constraints via `finalizeSolution`.

---

## 10. Authoritative Solution Verifier

**Authoritative File:** `internal/scheduler/verifier/verifier.go -> VerifySolution(p, solution, opts)`

### Verification Contracts
The verifier is a pure, independent read-only oracle that re-evaluates the solution from raw data without trusting solver-reported flags:

| Check Category | Verification Details | Failure Status |
| :--- | :--- | :--- |
| **Completeness** | `actual == SessionsPerWeek` for every session requirement. Total assignments == sum of all requirement sessions. | `INVALID_RESULT` |
| **Uniqueness** | Every `AssignmentID` is non-empty and unique. | `INVALID_RESULT` |
| **Catalog Integrity** | All assigned offerings, subjects, classes, student groups, faculty, rooms, and slots exist in problem catalog. | `INVALID_RESULT` |
| **Hierarchy Match** | Offering faculty and group match assignment faculty and group. | `INVALID_RESULT` |
| **Placement Bounds** | Start slot + duration does not exceed grid periods per day. `0 <= Instance < SessionsPerWeek`. | `INVALID_RESULT` |
| **Lock Preservation** | Every locked assignment in `p.LockedAssignments` is present with identical room, slot, faculty, and group. | `INVALID_RESULT` |
| **Hard Constraints** | Evaluates all 7 built-in constraints or compiled rules across all assignments. | `INFEASIBLE` |
| **Score Consistency** | Independently calculates StudentGapPenalty, FacultyPreferencePenalty, and RoomChangePenalty; asserts exact equality with solution score. | `INVALID_RESULT` |

---

## 11. Solver Statuses & Engine Lifecycle

### Status Definitions (`diagnostics.SolveStatus`)

| Status | Exact Condition When Returned |
| :--- | :--- |
| `SOLVED` | A complete timetable was found, satisfied all hard constraints, and passed authoritative independent verification. |
| `INFEASIBLE` | The problem is provably unsolvable (proven by PreSolve zero-domain/overload analysis or CSP exhaustive tree exhaustion). |
| `INVALID_PROBLEM` | The problem failed structural validation (`problem.Validate`), contained compile errors (`constraints.Compile`), or had invalid objective weights. |
| `INVALID_RESULT` | A solution produced by the solver failed independent verification (missing sessions, duplicate IDs, lock corruption, or score mismatch). Indicates an engine bug. |
| `CANCELLED` | The operation was cancelled via `context.Context` (`context.Canceled`). |
| `DEADLINE_EXCEEDED`| The execution exceeded its allotted context deadline (`context.DeadlineExceeded` or `opts.MaxDuration`). |
| `NODE_LIMIT` | CSP tree search exceeded `options.MaxNodes` before finding a feasible timetable. |

---

## 12. Determinism Model

CURRA guarantees **bit-level reproducibility**: identical problem input, solver options, constraint set, and random seed will always produce identical output across executions and platforms.

### Determinism Foundations in Code
1. **Zero Uncontrolled Concurrency:** Solver search (CSP and Tabu) runs synchronously on a single goroutine per `Solve()` call.
2. **Deterministic Map Iteration:**
   * All maps in `validation.go`, `backtracking.go`, `neighborhood.go`, and `verifier.go` are sorted into slices by key before traversal.
   * Helper sorting functions: `sortedRoomIDs`, `sortedTimeSlotIDs`, `sortedOfferings`, `sortedDepartmentIDs`, etc.
3. **Explicit Decision Ordering:** Decisions are sorted with stable comparator `decisionLess` (`Duration DESC -> OfferingID ASC -> RequirementID ASC -> Instance ASC`).
4. **Deterministic RuleSet Hashing:** `constraints.Compile` canonicalizes constraint instances into sorted JSON before computing SHA-256 hash.
5. **Seeded Pseudo-Random Number Generation:** Tabu search initializes a local `rand.New(rand.NewSource(seed))` instance passed explicitly through generator functions.

---

## 13. Concurrency & Context Handling

* **`context.Context` Integration:**
  * Checked at every search node in CSP backtracking (`searchBasic`, `searchHeuristic`).
  * Checked at every iteration in Tabu Search.
  * Properly distinguishes between `context.Canceled` and `context.DeadlineExceeded`.
* **State Isolation & Thread Safety:**
  * `problem.Problem` contains mutable index maps populated during `p.Prepare()`. A `Problem` struct must not be mutated concurrently across goroutines without cloning.
  * `problem.Solution` is deep-copied via `Clone()` during branch exploration, adapter requests, and Tabu best-state caching.
  * Solver instances (`backtracking.Solver`, `localsearch.TabuSearcher`) are stateless workers and safe for concurrent invocation across independent problems.
* **Platform Worker Concurrency:**
  * Background solve workers claim queued runs in PostgreSQL using `SELECT ... FOR UPDATE SKIP LOCKED`.
  * Running workers publish heartbeats every 30 seconds (`heartbeat_at`) with stale lease detection (>120s).

---

## 14. Testing Architecture & Verification Baseline

### Test Suite Organization
* **Unit Tests:** Package-level correctness tests in `internal/scheduler/...` (`constraints_test`, `engine_test`, `invariants_test`, `tabu_search_test`, `verifier_test`, etc.).
* **Hardening & Invariant Tests (`tests/hardening_invariants_test.go`):** Proves index consistency, transactional swap rollback, RuleSetHash stability, and verifier independence.
* **Pathological & Stress Tests (`tests/stress_pathological_test.go`):** Proves correct `INFEASIBLE` and `NODE_LIMIT` termination under pigeonhole bottlenecks and contradictory constraints.
* **Fuzz & Redteam Tests (`tests/redteam_fuzz_test.go`):** Mutates inputs to ensure the engine never panics and rejects corrupt inputs cleanly with `INVALID_PROBLEM`.
* **Heuristic Exploration (`tests/csp_heuristic_investigation_test.go`):** Benchmarks MRV, Degree, LCV, and Forward Checking efficiencies.
* **Application Boundary Tests (`application/boundary_test.go`):** Mechanical AST scan enforcing architectural import barriers.

### Baseline Verification Status (Executed 2026-09-01)
* **Root Module Test Suite (`go test -count=1 -short ./...`):**
  * `internal/scheduler/constraints`: **PASS** (1.42s)
  * `internal/scheduler/engine`: **PASS** (1.88s)
  * `internal/scheduler/problem`: **PASS** (1.66s)
  * `internal/scheduler/scorer`: **PASS** (0.82s)
  * `internal/scheduler/solver/backtracking`: **PASS** (4.91s)
  * `internal/scheduler/solver/localsearch`: **PASS** (1.20s)
  * `internal/scheduler/verifier`: **PASS** (1.45s)
  * `tests`: **PASS** (2.15s)
  * **Overall Root Status:** `ok` — 100% tests passing, zero failures.
* **Application Module Test Suite (`cd application && go test -count=1 ./...`):**
  * `application`: **PASS** (1.00s)
  * `application/internal/api/handlers`: **PASS** (1.08s)
  * `application/internal/curra`: **PASS** (0.85s)
  * `application/internal/services`: **PASS** (0.87s)
  * `application/internal/worker`: **PASS** (1.36s)
  * **Overall Application Status:** `ok` — 100% tests passing, zero failures.

---

## 15. Known Issues / Engineering Debt

| ID | Severity | Location | Description & Evidence | Status |
| :--- | :--- | :--- | :--- | :--- |
| **DEBT-01** | Medium | `internal/scheduler/constraints/framework.go:154` | **Soft constraints disabled in constraint compilation:** `Compile` explicitly rejects `ConstraintKindSoft` with compile error *"soft constraints are not supported by the current scoring engine"*. Soft objectives can only be configured via `scorer.ObjectiveConfig`. | Known architectural gap pending soft scoring bridge migration. |
| **DEBT-02** | Low | `internal/scheduler/constraints/` | **Dual constraint representation:** Coexistence of legacy `Constraint`/`ScopedValidator` interfaces alongside compiled `ConstraintDef`. Some built-in rules exist in both formats. | Functional but redundant; built-in rules should eventually unify under `ConstraintDef`. |
| **DEBT-03** | Low | `internal/scheduler/problem/validation.go:14` | **Validation takes Problem by value but calls pointer mutation:** `Validate(p Problem)` calls `p.Prepare()`, which mutates index maps on the copy. Callers must separately call `Prepare()` on their canonical pointer. | Handled correctly in `engine.Solve`, but caller must remain vigilant. |
| **DEBT-04** | Low | `application/internal/curra/adapter.go:20` | **Version constants hardcoded in adapter:** `CurrAVersion = "1.0.0"` and `CurrACommit = "f8cc385"` are static string constants in the adapter rather than dynamically injected via build tags. | Static tracking; requires manual update on version increments. |

---

## 16. Rules for Future Claude Code Work

1. **Preserve Two-Stage Architecture:** Never blur feasibility and optimization. Feasibility belongs strictly to CSP Backtracking; soft optimization belongs strictly to Tabu Search.
2. **Hard Constraints are Inviolable:** Optimization must never introduce or tolerate a hard constraint violation. Every optimized timetable must pass independent verification.
3. **Preserve Determinism:** Never iterate over raw Go maps during search, validation, or scoring. Always collect and sort keys. Always use seeded PRNG instances.
4. **Respect Context Cancellation:** Every long-running loop in solver algorithms must monitor `ctx.Done()`. Never suppress cancellation or deadline errors.
5. **No Solver Dependencies:** Do not add third-party dependencies to the root `go.mod`. The core engine must remain standard-library-only.
6. **Protect Architectural Boundaries:** Never import `internal/scheduler` from application packages other than `application/internal/curra`. Verify with `application/boundary_test.go`.
7. **Maintain Independent Verifier Purity:** The verifier in `internal/scheduler/verifier` must remain an independent check. Do not couple the verifier to internal solver shortcuts or shared mutable state.
8. **Test Coverage Mandatory:** Any modification to solver logic, constraints, or validation must include unit tests and pass all invariant regression suites in `tests/`.

---

## 17. Phase-Based Development Sequence

Future implementation and hardening should proceed in strict sequence:
1. **Phase 1: Architecture, Domain, Validation & Compilation**  
   *Domain integrity, structural validation rules, derived index maintenance, RuleSetHash stability.*
2. **Phase 2: Hard Constraints & Solution Verification**  
   *Unification of constraint definitions, verifier completeness, independent oracle correctness.*
3. **Phase 3: CSP Solver Hardening**  
   *MRV/Degree/LCV optimization, forward checking efficiency, node limit and timeout predictability.*
4. **Phase 4: Tabu Search & Soft Objective Optimization**  
   *Neighborhood move generation, incremental score evaluation, multi-objective weighting bridge.*
5. **Phase 5: Determinism, Concurrency & Cancellation**  
   *Thread-safety verification, map sorting audit, context propagation, worker lease management.*
6. **Phase 6: Benchmarking, Stress Testing & Production Readiness**  
   *Pathological test workloads, fuzzing, memory allocation optimization, CLI polishing.*

---

## 18. Parallel Development Boundaries

| Boundary Status | Target Areas | Justification & Dependencies |
| :--- | :--- | :--- |
| **SAFE TO PARALLELIZE** | `web/` (Frontend UI) | Isolated from backend Go code. Interacts solely through OpenAPI specification (`api/openapi.yaml`) and mock fixtures (`api/fixtures/`). |
| **SAFE TO PARALLELIZE** | `application/internal/database/` (Repositories/Migrations) | Self-contained PostgreSQL operations conforming to `database/schema.md`. No solver dependencies. |
| **SAFE TO PARALLELIZE** | `tests/` (Independent benchmarks / Pathological tests) | Read-only test harnesses interacting exclusively via public engine APIs (`engine.Solve`). |
| **POSSIBLY SAFE** | `internal/scheduler/scorer/` vs. `internal/scheduler/solver/backtracking/` | Scorer and Backtracking have decoupled interfaces, but both interact with `problem.Problem` and `model.TimeSlot`. |
| **NOT SAFE YET** | `internal/scheduler/problem/` vs. `internal/scheduler/constraints/` | Highly coupled. Changes to `problem.SolutionIndex` or `problem.Move` directly impact constraint evaluation and verification. |
| **NOT SAFE YET** | `internal/scheduler/engine/` vs. `application/internal/curra/` | The adapter directly mirrors the engine input/output types. Changes to `engine.Request` require synchronous updates to `curra.Adapter`. |
