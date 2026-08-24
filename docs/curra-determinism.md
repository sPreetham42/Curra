# CURRA Determinism & Replay Contract

This document specifies the exact determinism guarantees, replay specifications, and persistence requirements for the CURRA scheduling engine.

## 1. Determinism Core Principles

CURRA is designed to be **100% deterministic**. Given identical problem inputs, constraint rules, solver options, and random seeds, CURRA will produce identical timetable assignments, diagnostic metrics, and score breakdowns across execution runs.

---

## 2. Deterministic Inputs Required for Replay

To reproduce any solve result identically, the Application layer MUST store and persist the following state:

1. **Problem Definition (`problem.Problem`)**:
   - Tenant ID, Term ID.
   - Complete domain catalogs (`Departments`, `Programs`, `Classes`, `StudentGroups`, `Subjects`, `CourseOfferings`, `SessionRequirements`, `Faculty`, `Rooms`, `RoomFeatures`, `TimeSlots`).
   - Availability lists (`FacultyAvailabilities`, `RoomAvailabilities`).
   - Preference lists (`FacultyPreferences`).
   - Locked assignments (`LockedAssignments`).
   - Grid bounds (`PeriodsPerDay`).

2. **Constraint Rules (`[]constraints.ConstraintInstance`)**:
   - Exact list of user-configured constraint instances.
   - SHA-256 rule set hash (`RuleSetHash`) produced during `constraints.Compile`.

3. **CSP Solver Options (`problem.SolveOptions`)**:
   - `MaxNodes` (integer search node limit).
   - `ViolationLimit` (diagnostic violation reporting cap).
   - `SearchMode` (`SearchModeHeuristic`, `SearchModeHeuristicLCV`, `SearchModeBasic`).

4. **Tabu Local Search Options (`localsearch.TabuSearchOptions`)**:
   - **`Seed` (int64 random seed)**: *Critical for pseudo-random neighborhood generation*.
   - `MaxIterations` (maximum Tabu search iterations).
   - `MaxDuration` (time duration limit).
   - `NoImprovementLimit` (stagnation iteration threshold).
   - `TabuTenure` (memory tabu tenure).
   - `MaxCandidates` (candidate neighborhood size per iteration).

5. **Soft Objective Configuration (`scorer.ObjectiveConfig`)**:
   - Component weights (e.g. `StudentGapPenalty` weight).

6. **Engine Build Identifier**:
   - CURRA engine version or git commit hash.

---

## 3. Solver Internal Determinism Mechanisms

### 3.1 CSP Search Phase Determinism
* **Sorted Iterations**: Map iteration in Go is randomized by design. To guarantee determinism, CSP search pre-sorts all catalog IDs (departments, faculty, rooms, time slots, course offerings, requirements) into deterministic lexicographical slices prior to decision variable ordering and value domain selection.
* **Deterministic Heuristics**: Both MRV (Minimum Remaining Values), Degree Heuristic, and LCV (Least Constraining Value) ties are broken using strict lexicographical comparison (`decisionLess`).

### 3.2 Tabu Local Search Phase Determinism
* **Seeded PRNG**: Tabu search local optimization initializes a dedicated Go PRNG instance:
  ```go
  rng := rand.New(rand.NewSource(opts.Seed))
  ```
* **Deterministic Neighborhood Candidates**: All candidate move generations use `rng`. Identical seeds yield identical neighborhood exploration paths across runs on the same Go version.

---

## 4. Replay Audit & Persistence Schema

To provide full auditability and replay guarantees, the Application database schema should store solve executions as immutable records:

```json
{
  "solveId": "solve-2026-fall-v1",
  "engineVersion": "v1.4.2-frozen",
  "seed": 42,
  "ruleSetHash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "solveOptions": {
    "maxNodes": 100000,
    "searchMode": "HEURISTIC_LCV"
  },
  "tabuOptions": {
    "seed": 42,
    "maxIterations": 1000,
    "noImprovementLimit": 100,
    "tabuTenure": 10,
    "maxCandidates": 100
  },
  "score": {
    "hardViolations": 0,
    "softPenalty": 12,
    "breakdown": {
      "studentGapPenalty": 12
    }
  },
  "diagnostics": {
    "status": "SOLVED",
    "nodesExplored": 420,
    "candidates": 1350,
    "backtracks": 12
  }
}
```

---

## 5. Non-Deterministic Anti-Patterns to Avoid

1. **Passing Seed = 0**: In `localsearch`, if `Seed == 0` is passed, it falls back to a default seed of `42`. Applications MUST explicitly store and pass the exact seed used.
2. **System Clock / Wall-Clock Duration Timeouts**: Relying strictly on `opts.MaxDuration` for stopping criteria can lead to minor variation in Tabu iterations if CPU load fluctuates between runs. For bit-exact replay, use `MaxIterations` or `NoImprovementLimit` without duration caps.
3. **Environment-Dependent Go Compiler Versions**: Minor changes in Go standard library random generator implementations across major Go versions could affect PRNG streams. Pin the Go binary version in deployment.
