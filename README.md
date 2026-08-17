# Cura

**A deterministic academic timetabling engine written in Go.**

Cura solves the university course-scheduling problem in two stages: a CSP backtracking solver guarantees a hard-constraint-feasible timetable, then a Tabu Search optimizer improves it against soft objectives — without ever compromising feasibility.

<p>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://github.com/sPreetham42/Curra/blob/master/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/Tests-Passing-brightgreen?style=flat-square" alt="Tests">
</p>

---

## Why Cura

Most scheduling tools blur feasibility and optimization together. Cura keeps them strictly separate:

1. **Feasibility first** — a backtracking CSP solver finds a timetable that satisfies every hard constraint (faculty conflicts, room capacity, availability, etc.), or proves none exists.
2. **Optimization second** — a Tabu Search local optimizer takes that feasible timetable and improves soft-objective quality (e.g. minimizing student idle gaps), guaranteeing it never reintroduces a hard violation.

## Architecture

```
                        CURA PLATFORM ENGINE
 ┌────────────────┐   ┌─────────────────────┐   ┌───────────────────┐
 │  Domain Model   │   │ Configurable Rules  │   │  Hard Constraints  │
 │ Programs, Rooms,│   │ ConstraintInstance, │   │ Faculty / Room /   │
 │ Groups, Slots   │   │ JSON Schema, Hash   │   │ Capacity / etc.    │
 └────────┬────────┘   └──────────┬──────────┘   └─────────┬─────────┘
          └───────────────────────┼──────────────────────────┘
                                   ▼
                     Problem Formulation & Validation
                                   │
                                   ▼
                  STAGE 1 · CSP Backtracking Solver
              MRV + Degree + LCV + Forward Checking
                                   │  (feasible seed timetable)
                                   ▼
                  STAGE 2 · Tabu Search Optimizer
        Neighborhood generation → validate → score → aspiration
                                   │
                                   ▼
                    Optimal, Feasible Timetable
             Score breakdown + full solve diagnostics
```

## Key Features

- **Guaranteed feasibility** — every `SOLVED` timetable is verified against all hard constraints before being reported.
- **Two-stage solving pipeline** — CSP backtracking (MRV, Degree, LCV, Forward Checking) for feasibility, Tabu Search for optimization.
- **Assignment locking** — pin specific sessions to a room/slot without interfering with search.
- **Configurable constraint engine** — declarative rule templates, compile-time validation, deterministic SHA-256 `RuleSetHash`.
- **Structured diagnostics** — every violation carries its `ConstraintID`, `TemplateID`, scope, and severity.
- **High-throughput indexing** — O(1) constraint checks and in-place delta mutations (`ApplyMove`/`UndoMove`, `ApplySwap`/`UndoSwap`).

## Project Structure

```
Curra/
├── cmd/solver/                  CLI entry point
├── internal/scheduler/
│   ├── model/                   Domain entities — Terms, Classes, Rooms, Slots
│   ├── problem/                 Validation, SolutionIndex, move mutations
│   ├── diagnostics/             SolveStatus, Severity, structured Violations
│   ├── scorer/                  Solution scoring & penalty breakdown
│   ├── constraints/             Built-in + configurable constraint framework
│   └── solver/
│       ├── backtracking/        CSP solver
│       └── localsearch/         Tabu Search optimizer
└── tests/                       Unit, integration, property & benchmark tests
```

## Getting Started

**Requirements:** Go 1.22+

```bash
git clone https://github.com/sPreetham42/Curra.git
cd Curra

# Run on the bundled sample problem
go run ./cmd/solver

# Run with a custom problem and node limit
go run ./cmd/solver -input=path/to/problem.json -max-nodes=100000
```

**Sample output:**

```json
{
  "solution": {
    "assignments": [
      { "courseOfferingId": "offering-cs101", "facultyId": "fac-smith", "roomId": "room-101", "timeSlotId": "mon-1" }
    ],
    "score": { "hardViolations": 0, "softPenalty": 0 }
  },
  "diagnostics": { "status": "SOLVED", "message": "feasible timetable found" }
}
```

## Testing

```bash
go test ./...                              # unit, integration & property tests
go test -run=^$ -bench=. -benchmem ./tests # benchmarks
```

| Benchmark | Speed | Allocations |
|---|---|---|
| `EvaluateMove` | ~6.2 µs/op | 35 allocs/op |
| `TabuSearch_MediumProblem` (1,000 moves) | ~8.7 ms/op | 49,867 allocs/op |
| `SearchModes` (heuristic) | ~449 µs/op | 1,734 allocs/op |

## Roadmap

- [x] Domain model, validation & backtracking solver
- [x] MRV + Degree + LCV + Forward Checking heuristics
- [x] Tabu Search local optimizer
- [x] Configurable constraint framework (`SubjectMaxPerDay`, compiler, CSP/Tabu integration)
- [ ] Migrate remaining built-in hard constraints to `ConstraintDef` templates
- [ ] Soft constraint scoring bridge & weighted multi-objective optimization

## License

[MIT](https://github.com/sPreetham42/Curra/blob/master/LICENSE)
