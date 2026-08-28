<div align="center">

# Curra

### A deterministic academic timetabling engine, written in Go.

Curra solves university course scheduling in two clean stages — a **CSP backtracking solver** guarantees a feasible timetable, then a **Tabu Search optimizer** refines it against soft objectives, without ever compromising that feasibility.

<a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"></a>
<a href="https://github.com/sPreetham42/Curra/blob/master/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue?style=for-the-badge" alt="License"></a>
<img src="https://img.shields.io/badge/Tests-Passing-brightgreen?style=for-the-badge" alt="Tests">

</div>

<br>

## Table of Contents

- [Why curra](#why-curra)
- [Architecture](#architecture)
- [Key Features](#key-features)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Testing & Benchmarks](#testing--benchmarks)
- [Roadmap](#roadmap)
- [License](#license)

<br>

## Why curra

Most scheduling tools blur *feasibility* and *optimization* together, which makes their output hard to trust. curra keeps the two strictly separate:

| Stage | Responsibility | Guarantee |
|:---|:---|:---|
| **1 · Feasibility** | A backtracking CSP solver searches for a timetable that satisfies every hard constraint — faculty conflicts, room capacity, availability, and more | Either returns a feasible timetable, or proves none exists |
| **2 · Optimization** | A Tabu Search local optimizer takes that feasible timetable and improves it against soft objectives, such as minimizing student idle gaps | Never reintroduces a hard-constraint violation |

The result: every solution curra reports as `SOLVED` is provably correct *before* it's ever optimized.

<br>

## Architecture

```
                          curra PLATFORM ENGINE
┌──────────────────┐   ┌───────────────────────┐   ┌──────────────────────┐
│   Domain Model    │   │  Configurable Rules   │   │   Hard Constraints   │
│ Programs, Rooms,  │   │ ConstraintInstance,   │   │ Faculty / Room /     │
│ Groups, Slots     │   │ JSON Schema, Hash     │   │ Capacity / etc.      │
└─────────┬─────────┘   └───────────┬───────────┘   └───────────┬──────────┘
          └─────────────────────────┼──────────────────────────┘
                                     ▼
                     Problem Formulation & Validation
                                     │
                                     ▼
                    STAGE 1 · CSP Backtracking Solver
               MRV  +  Degree  +  LCV  +  Forward Checking
                                     │
                          (feasible seed timetable)
                                     ▼
                    STAGE 2 · Tabu Search Optimizer
        Neighborhood generation → validate → score → aspiration
                                     │
                                     ▼
                      Optimal, Feasible Timetable
                Score breakdown + full solve diagnostics
```

<br>

## Key Features

#### 🧩 CSP Backtracking Core
The feasibility stage is a full constraint-satisfaction solver, not a heuristic shortcut:
- **MRV (Minimum Remaining Values)** — always branches on the most constrained variable first, dramatically pruning the search tree
- **Degree heuristic** — breaks MRV ties by picking the variable involved in the most constraints
- **LCV (Least Constraining Value)** — orders candidate assignments to preserve future options
- **Forward checking** — propagates constraints eagerly, detecting dead ends before they're fully explored

#### 🎯 Guaranteed Feasibility
Every timetable reported as `SOLVED` is independently re-verified against all hard constraints — the solver never has to be "trusted," only checked.

#### 🔁 Tabu Search Optimization
Once feasibility is secured, a Tabu Search local optimizer explores neighboring timetables — generating candidate moves, validating them, scoring against soft objectives, and applying aspiration criteria — to improve overall quality without ever breaking a hard constraint.

#### 📌 Assignment Locking
Pin specific sessions to a fixed room and time slot, and the solver will work around them without interference.

#### ⚙️ Configurable Constraint Engine
Constraints are declarative, not hardcoded — defined via rule templates, validated at compile time, and fingerprinted with a deterministic SHA-256 `RuleSetHash` for full reproducibility.

#### 🔍 Structured Diagnostics
Every violation is fully traceable, carrying its `ConstraintID`, `TemplateID`, scope, and severity — no guessing why a timetable failed.

#### ⚡ High-Throughput Indexing
Constraint checks run in O(1) via indexed lookups, and moves are applied as in-place deltas (`ApplyMove` / `UndoMove`, `ApplySwap` / `UndoSwap`) rather than full re-evaluations — keeping the search loop fast.

<br>

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

<br>

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

<details>
<summary><strong>Sample output</strong></summary>

```json
{
  "solution": {
    "assignments": [
      {
        "courseOfferingId": "offering-cs101",
        "facultyId": "fac-smith",
        "roomId": "room-101",
        "timeSlotId": "mon-1"
      }
    ],
    "score": { "hardViolations": 0, "softPenalty": 0 }
  },
  "diagnostics": {
    "status": "SOLVED",
    "message": "feasible timetable found"
  }
}
```

</details>

<br>

## Testing & Benchmarks

```bash
go test ./...                              # unit, integration & property tests
go test -run=^$ -bench=. -benchmem ./tests # benchmarks
```

| Benchmark | Speed | Allocations |
|:---|:---:|:---:|
| `EvaluateMove` | ~6.2 µs/op | 35 allocs/op |
| `TabuSearch_MediumProblem` (1,000 moves) | ~8.7 ms/op | 49,867 allocs/op |
| `SearchModes` (heuristic) | ~449 µs/op | 1,734 allocs/op |

<br>

## Roadmap

- [x] Domain model, validation & backtracking solver
- [x] MRV + Degree + LCV + Forward Checking heuristics
- [x] Tabu Search local optimizer
- [x] Configurable constraint framework (`SubjectMaxPerDay`, compiler, CSP/Tabu integration)
- [ ] Migrate remaining built-in hard constraints to `ConstraintDef` templates
- [ ] Soft constraint scoring bridge & weighted multi-objective optimization

<br>

## License

Released under the [MIT License](https://github.com/sPreetham42/Curra/blob/master/LICENSE).

<div align="center">

<sub>Built with Go, backtracking, and a healthy respect for hard constraints.</sub>

</div>
