# CURRA Adapter Boundary Contract

## 1. Architecture

```
Application Layer (MIMO)
        │
        ▼
┌─────────────────────────────────────────┐
│           CurraAdapter                   │
│  (Pure Go package, no DB, stateless)    │
│  Maps application DTOs ↔ CURRA types    │
│  Enforces clone-before-mutate            │
│  Calls independent verification         │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         CURRA Engine (Frozen)            │
│  engine.Solve()                         │
│  verifier.VerifySolution()              │
│  problem.Problem / Solution             │
│  constraints.Compile()                  │
└─────────────────────────────────────────┘
```

## 2. Adapter Interface

```go
package curraadapter

// CurraAdapter is the ONLY boundary between application and CURRA.
// It is stateless. Each call is independent. No solver state persists.
type CurraAdapter interface {
    // Solve runs the complete pipeline: validate → presolve → CSP → Tabu → verify.
    Solve(ctx context.Context, req SolveRequest) (SolveResponse, error)

    // Verify independently checks a stored solution against a snapshot.
    Verify(ctx context.Context, req VerifyRequest) (VerifyResponse, error)

    // ValidateMove tests a manual edit without mutating the original solution.
    ValidateMove(ctx context.Context, req ValidateMoveRequest) (ValidateMoveResponse, error)

    // CompileConstraints validates and compiles constraint instances.
    CompileConstraints(ctx context.Context, req CompileRequest) (CompileResponse, error)
}
```

## 3. Request/Response Types (Application-facing)

These are the adapter's own types, NOT direct re-exports of CURRA internals.

```go
type SolveRequest struct {
    ProblemJSON      json.RawMessage `json:"problem"`          // Serialized problem.Problem
    ConstraintsJSON  json.RawMessage `json:"constraints"`      // Serialized []ConstraintInstance
    Seed             int64           `json:"seed"`             // Deterministic seed
    ObjectiveWeights map[string]int  `json:"objectiveWeights"` // e.g. {"StudentGapPenalty": 1}
    DisableOptimize  bool            `json:"disableOptimize"`
    // Solver limits — application manages defaults, not exposed to users
    MaxNodes         int             `json:"maxNodes,omitempty"`
    SearchMode       string          `json:"searchMode,omitempty"` // "HEURISTIC_LCV" recommended
}

type SolveResponse struct {
    Status       string          `json:"status"`       // SOLVED, INFEASIBLE, etc.
    SolutionJSON json.RawMessage `json:"solution"`     // Serialized problem.Solution
    Score        ScoreDTO        `json:"score"`
    Diagnostics  DiagnosticsDTO  `json:"diagnostics"`
    RuleSetHash  string          `json:"ruleSetHash"`
    Error        string          `json:"error,omitempty"`
}

type VerifyRequest struct {
    ProblemJSON      json.RawMessage `json:"problem"`
    SolutionJSON     json.RawMessage `json:"solution"`
    ConstraintsJSON  json.RawMessage `json:"constraints,omitempty"`
    ObjectiveWeights map[string]int  `json:"objectiveWeights,omitempty"`
}

type VerifyResponse struct {
    Valid      bool            `json:"valid"`
    Status     string          `json:"status"`
    Violations []ViolationDTO  `json:"violations,omitempty"`
    Score      ScoreDTO        `json:"score"`
}

type ValidateMoveRequest struct {
    ProblemJSON      json.RawMessage `json:"problem"`
    SolutionJSON     json.RawMessage `json:"solution"`
    Move             MoveDTO         `json:"move"`
    ConstraintsJSON  json.RawMessage `json:"constraints,omitempty"`
}

type ValidateMoveResponse struct {
    Valid      bool            `json:"valid"`
    Status     string          `json:"status"`
    Violations []ViolationDTO  `json:"violations,omitempty"`
    Score      ScoreDTO        `json:"score"`
}

type CompileRequest struct {
    ProblemJSON     json.RawMessage `json:"problem"`
    ConstraintsJSON json.RawMessage `json:"constraints"`
}

type CompileResponse struct {
    RuleSetHash string        `json:"ruleSetHash"`
    Errors      []CompileError `json:"errors,omitempty"`
}
```

## 4. Why JSON Raw Messages

The adapter uses `json.RawMessage` for CURRA's domain types (`Problem`, `Solution`, `ConstraintInstance[]`) because:

1. **Stable boundary:** The application stores these as JSONB in PostgreSQL. Deserializing at the adapter boundary keeps the boundary clean.
2. **No CURRA import in application code:** The application services deal with JSON, not Go structs. Only the adapter package imports CURRA.
3. **Schema evolution:** CURRA types can evolve without changing the adapter API surface (within semver bounds).

## 5. Manual Edit Flow

For the visual timetable editor, manual edits follow this flow:

```
1. Load stored solution (JSON) from database
2. Deserialize into problem.Solution (inside adapter)
3. Clone the solution (sol.Clone())
4. Apply the move: clonedSol.ApplyMove(p, move)
5. Verify: verifier.VerifySolution(p, &clonedSol, opts)
6. If valid: serialize and store as new ScheduleVersion
7. Return verification result to API layer
```

The original solution is never mutated. The adapter handles all cloning internally.

## 6. Safety Guarantees

| Guarantee | How |
|---|---|
| No CURRA internals leak | Adapter types are own DTOs, not CURRA types |
| No solver state persists | Each call is stateless, no adapter-level caches |
| No stale state | Problem is re-prepared on each Solve call |
| Clone before mutate | Adapter clones solution before any ApplyMove |
| Independent verification | Every solve and move passes through verifier |
| Thread safety | No shared mutable state in adapter |

## 7. Package Location

The adapter lives in the application repository, not in CURRA:

```
application/
  internal/
    curra/
      adapter.go      // CurraAdapter implementation
      types.go        // Application-facing DTOs
      mapper.go       // JSON ↔ CURRA type mapping
```

The adapter imports CURRA as a Go module dependency.
