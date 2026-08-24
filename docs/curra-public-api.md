# CURRA Public API Inventory

This document provides a precise inventory of all packages, types, constructors, and methods in CURRA, classifying each symbol's suitability for application exposure.

## Symbol Classifications

- **PUBLIC APPLICATION CONTRACT**: Safe, stable, and intended for direct application usage via adapter.
- **INTERNAL ENGINE API**: Engine orchestration type; should be accessed strictly via `CurraAdapter`.
- **UNSAFE TO EXPOSE**: Internal solver state or mutable indexing structure that must NEVER be accessed by application code.
- **NEEDS ADAPTER WRAPPER**: Public symbol requiring an additive wrapper in a root package to be importable outside the Go module.

---

## Package Inventory

### 1. `internal/scheduler/engine`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `Request` | Struct | Defines solve request payload (`Problem`, `Constraints`, `SolveOptions`, `TabuOptions`, `ObjectiveConfig`, `DisableOptimize`) | **PUBLIC APPLICATION CONTRACT** | Requires adapter wrapper for external module import. |
| `Response` | Struct | Defines solve result payload (`Solution`, `Diagnostics`, `Score`) | **PUBLIC APPLICATION CONTRACT** | Requires adapter wrapper for external module import. |
| `Solve` | Function | Canonical orchestrator pipeline for validation, CSP search, Tabu search, and authoritative verification | **INTERNAL ENGINE API** | Primary entrypoint wrapped by `CurraAdapter.Solve`. |

---

### 2. `internal/scheduler/problem`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `Problem` | Struct | Complete self-contained academic scheduling problem instance | **PUBLIC APPLICATION CONTRACT** | Map-based catalog. Immutable once prepared. |
| `Solution` | Struct | Complete timetable solution (`Assignments []Assignment`, `Index`, `Score`) | **PUBLIC APPLICATION CONTRACT** | `Index` field (`SolutionIndex`) is unexported from JSON (`json:"-"`). |
| `Assignment` | Struct | Single scheduled session instance (`ID`, `CourseOfferingID`, `StudentGroupID`, `FacultyID`, `RoomID`, `TimeSlotID`, `SessionRequirementID`, `Instance`) | **PUBLIC APPLICATION CONTRACT** | Clean JSON DTO. |
| `AssignmentID` | Type | Unique string identifier for assignment (`RequirementID#Instance`) | **PUBLIC APPLICATION CONTRACT** | String type alias. |
| `Placement` | Struct | Room and time slot pair for assignment positioning | **PUBLIC APPLICATION CONTRACT** | Used in Move/Swap operations. |
| `Move` | Struct | Move operation specification (`AssignmentID`, `From`, `To`) | **PUBLIC APPLICATION CONTRACT** | Used for manual editing. |
| `SolveOptions` | Struct | CSP solver configuration (`MaxNodes`, `ViolationLimit`, `SearchMode`) | **PUBLIC APPLICATION CONTRACT** | Value struct with `Normalize()` default filler. |
| `SearchMode` | Type | Enum (`SearchModeBasic`, `SearchModeHeuristic`, `SearchModeHeuristicLCV`) | **PUBLIC APPLICATION CONTRACT** | Enum for CSP heuristics. |
| `Validate` | Function | Pure structural validation of problem definitions | **PUBLIC APPLICATION CONTRACT** | Returns `[]diagnostics.Violation`. |
| `PreSolve` | Function | Fast pre-solve feasibility analysis | **PUBLIC APPLICATION CONTRACT** | Detects zero domains, faculty overloads, room feature bottlenecks. |
| `(p *Problem) Prepare` | Method | Populates derived availability and slot indexes | **PUBLIC APPLICATION CONTRACT** | Must be called before search/verification if indexes not prebuilt. |
| `(s Solution) Clone` | Method | Deep copy of solution and internal indexes | **PUBLIC APPLICATION CONTRACT** | Safe for cloning state before manual edits. |
| `(s *Solution) ApplyMove` | Method | In-place mutation of solution & index for move | **NEEDS ADAPTER WRAPPER** | Mutates solution in place without hard constraint validation. |
| `(s *Solution) ApplySwap` | Method | In-place mutation of solution & index for swap | **NEEDS ADAPTER WRAPPER** | Mutates solution in place without hard constraint validation. |
| `SolutionIndex` | Struct | Fast O(1) lookup index for faculty, room, and group slot occupancy | **UNSAFE TO EXPOSE** | Internal solver search optimization. Never access directly. |

---

### 3. `internal/scheduler/verifier`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `VerifySolution` | Function | Authoritative pure read-only verification pass | **PUBLIC APPLICATION CONTRACT** | Checks requirement completeness, counts, FKs, grid duration, locked items, hard constraints, score consistency. |
| `VerifyOptions` | Struct | Verification settings (`Compiled`, `ObjectiveConfig`) | **PUBLIC APPLICATION CONTRACT** | Optional parameters for verification. |
| `VerificationReport` | Struct | Verification report (`Valid`, `Status`, `Violations`, `Message`) | **PUBLIC APPLICATION CONTRACT** | Trustworthy verification output for UI/Audit. |
| `ErrInvalidResult` | Error | Sentinel error for result structural integrity failure | **PUBLIC APPLICATION CONTRACT** | Returned when solution output is malformed. |
| `ErrHardConstraintViolation` | Error | Sentinel error for hard constraint failure | **PUBLIC APPLICATION CONTRACT** | Returned when candidate solution violates hard constraints. |

---

### 4. `internal/scheduler/diagnostics`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `SolveStatus` | Type | Status enum (`SOLVED`, `INFEASIBLE`, `INVALID_PROBLEM`, `INVALID_RESULT`, `CANCELLED`, `DEADLINE_EXCEEDED`, `NODE_LIMIT`) | **PUBLIC APPLICATION CONTRACT** | Primary status indicator. |
| `Severity` | Type | Violation severity enum (`HARD`, `SOFT`, `INFO`) | **PUBLIC APPLICATION CONTRACT** | Used in diagnostic reporting. |
| `Violation` | Struct | Violation explanation (`ConstraintName`, `Severity`, `Message`, `AssignmentID`, `RelatedIDs`, `Metadata`) | **PUBLIC APPLICATION CONTRACT** | Structured, human-readable & UI-ready diagnostic. |
| `Diagnostics` | Struct | Search execution summary (`Status`, `NodesExplored`, `Candidates`, `Backtracks`, `Violations`, `Message`) | **PUBLIC APPLICATION CONTRACT** | Execution metrics payload. |

---

### 5. `internal/scheduler/scorer`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `Score` | Struct | Feasibility & soft penalty summary (`HardViolations`, `SoftPenalty`, `Breakdown`) | **PUBLIC APPLICATION CONTRACT** | Score result DTO. |
| `ScoreBreakdown` | Struct | Detailed penalty breakdown (`StudentGapPenalty`, `GroupGaps`, `Details`, `Components`) | **PUBLIC APPLICATION CONTRACT** | Detailed breakdown for UI visualization. |
| `ObjectiveConfig` | Struct | Soft objective component configurations and weights | **PUBLIC APPLICATION CONTRACT** | User/Admin configurable objective weights. |
| `ObjectiveComponent` | Struct | Objective ID and weight pair | **PUBLIC APPLICATION CONTRACT** | Used in ObjectiveConfig. |
| `GroupDayGap` | Struct | Per-group, per-day gap detail (`StudentGroupID`, `Day`, `Gaps`, `FirstPeriod`, `LastPeriod`) | **PUBLIC APPLICATION CONTRACT** | Explains gap locations to UI. |

---

### 6. `internal/scheduler/constraints`

| Symbol | Kind | Purpose | Application Suitability | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `Compile` | Function | Compiles uncompiled constraint instances into executable rules | **PUBLIC APPLICATION CONTRACT** | Generates `RuleSetHash` and `CompiledConstraintSet`. |
| `ConstraintInstance` | Struct | Declarative rule definition (`ID`, `TemplateID`, `Scope`, `Params`, `Kind`, `Weight`) | **PUBLIC APPLICATION CONTRACT** | Input structure for user-configurable rules. |
| `CompiledConstraintSet` | Struct | Compiled constraint rules and SHA-256 rule set hash | **NEEDS ADAPTER WRAPPER** | Contains rule set hash; internal compiled defs should remain encapsulated. |
| `CompileError` | Struct | Syntax/catalog validation error detail | **PUBLIC APPLICATION CONTRACT** | Structured error for invalid constraint instances. |

---

### 7. Internal Solver Packages (UNSAFE / INTERNAL)

| Package | Purpose | Application Suitability |
| :--- | :--- | :--- |
| `internal/scheduler/solver/backtracking` | CSP backtracking solver search engine | **UNSAFE TO EXPOSE** (Internal to Solver) |
| `internal/scheduler/solver/localsearch` | Tabu search local optimization engine | **UNSAFE TO EXPOSE** (Internal to Solver) |
| `internal/scheduler/model` | Low-level domain entities | **INTERNAL ENGINE API** (Imported via problem package) |
