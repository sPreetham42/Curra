# Cura — High-Performance Academic Timetable Scheduling Platform

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=for-the-badge)]()
[![Tests](https://img.shields.io/badge/Tests-100%25%20Passing-success?style=for-the-badge)]()
[![Architecture](https://img.shields.io/badge/Engine-CSP%20%2B%20Tabu%20Search-blueviolet?style=for-the-badge)]()
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)

**Cura** is an industrial-grade, deterministic academic timetabling and scheduling engine built in Go. It combines **Constraint Satisfaction Programming (CSP)** with heuristic search for hard-constraint feasibility and **Tabu Search** local search optimization to minimize student idle gaps and optimize resource allocation.

---

## 🏛️ System Architecture

```
+---------------------------------------------------------------------------------------------------+
|                                        CURA PLATFORM ENGINE                                       |
+---------------------------------------------------------------------------------------------------+
|                                                                                                   |
|   +-----------------------+     +-----------------------+     +-------------------------------+   |
|   |     Domain Model      |     |  Configurable Rules   |     |    Legacy Hard Constraints    |   |
|   |   (Programs, Classes, |     | (ConstraintInstance,  |     | (Faculty, Room, StudentGroup, |   |
|   |  Groups, Offerings,   |     |    JSON Schema,       |     |  Availabilities, Capacity)    |   |
|   |   Rooms, Slots)       |     |   RuleSetHash SHA256) |     |                               |   |
|   +-----------+-----------+     +-----------+-----------+     +---------------+---------------+   |
|               |                             |                                 |                   |
|               +----------------------+      |      +--------------------------+                   |
|                                      |      |      |                                              |
|                                      v      v      v                                              |
|                       +-------------------------------------------+                               |
|                       |            Problem Formulation            |                               |
|                       |   Validation, Locked Assignments, Index   |                               |
|                       +---------------------+---------------------+                               |
|                                             |                                                     |
|                                             v                                                     |
|                       +-------------------------------------------+                               |
|                       |         STAGE 1: CSP BACKTRACKING         |                               |
|                       |   MRV + Degree + LCV + Forward Checking   |                               |
|                       |   + Post-Search Final Hard Validation     |                               |
|                       +---------------------+---------------------+                               |
|                                             | (Feasible Seed Timetable)                           |
|                                             v                                                     |
|                       +-------------------------------------------+                               |
|                       |          STAGE 2: TABU SEARCH             |                               |
|                       |  Neighborhood Gen (Single/Swap Moves)     |                               |
|                       |  Apply -> Compiled Validation -> Score    |                               |
|                       |  Tabu List + Aspiration Criteria          |                               |
|                       |  StudentGapPenalty Minimization           |                               |
|                       +---------------------+---------------------+                               |
|                                             |                                                     |
|                                             v                                                     |
|                       +-------------------------------------------+                               |
|                       |            OPTIMAL SOLUTION               |                               |
|                       |   Score (HardViolations: 0, SoftPenalty)  |                               |
|                       |   Diagnostics (Nodes, Backtracks, Status) |                               |
|                       +-------------------------------------------+                               |
|                                                                                                   |
+---------------------------------------------------------------------------------------------------+
```

---

## ✨ Key Features & Capabilities

- 🎯 **Guaranteed Hard-Constraint Feasibility**: Every generated timetable is strictly verified against all physical, temporal, and catalog constraints before being reported as `SOLVED`.
- ⚡ **Two-Stage Solving Pipeline**:
  1. **CSP Backtracking**: Employs **MRV** (Minimum Remaining Values), **Degree Heuristic**, **LCV** (Least Constraining Value), and **Forward Checking** domain pruning for rapid initial feasibility.
  2. **Tabu Search Optimizer**: Iteratively improves soft objective metrics (e.g. `StudentGapPenalty`) via short-term memory tenure and aspiration criteria without ever violating hard constraints.
- 🔒 **Solver-Level Assignment Locking**: Lock specific lecture sessions to predetermined rooms and slots without search interference.
- 🧩 **Configurable Constraint Engine**: Declarative rule templates with strict compile-time validation, atomic rejection on invalid schemas, and deterministic SHA-256 `RuleSetHash` generation.
- 🔍 **Rich Diagnostics & Explainability**: Structured `Violation` provenance tracking identifying exact `ConstraintID`, `TemplateID`, `Scope`, `Severity`, and `Message`.
- 🚀 **High-Throughput In-Memory Indexing**: $O(1)$ constraint validation and in-place delta move mutations (`ApplyMove` / `UndoMove`, `ApplySwap` / `UndoSwap`).

---

## 📁 Repository Structure

```
timetable-platform/
├── cmd/
│   └── solver/                       # CLI executable for loading problem JSON and running solver
├── internal/
│   └── scheduler/
│       ├── model/                    # Core academic domain entities (Terms, Classes, Rooms, Slots)
│       ├── problem/                  # Problem validation, SolutionIndex, Move mutation engine
│       ├── diagnostics/              # SolveStatus, Severity, structured Violations & search metrics
│       ├── scorer/                   # Solution scoring & penalty breakdown (StudentGapPenalty)
│       ├── constraints/              # Built-in constraints, Scoped Validators, MembershipIndex,
│       │                             # Configurable Constraint Framework & Compiler
│       └── solver/
│           ├── backtracking/         # CSP Backtracking Solver (MRV, Degree, LCV, Forward Checking)
│           └── localsearch/          # Tabu Search optimizer, neighborhood generator, move validator
└── tests/                            # Unit tests, integration tests, property tests & benchmarks
```

---

## 🛠️ Engine Pipeline & Implemented Modules

### 1. Domain Model (`model`)
Represents the complete academic hierarchy:
- `Department` $\rightarrow$ `Program` $\rightarrow$ `Class` $\rightarrow$ `StudentGroup`
- `Subject` $\rightarrow$ `CourseOffering` $\rightarrow$ `SessionRequirement` (Theory, Lab, multi-period duration, consecutive flags)
- `Room` (capacity, feature tags), `TimeSlot` (weekday, period)
- `FacultyAvailability` & `RoomAvailability` matrices

### 2. Constraint Engine (`constraints`)
- **Built-in Hard Constraints**:
  - `FacultyConflict`: Prevents faculty double-booking.
  - `RoomConflict`: Prevents room collisions.
  - `StudentGroupConflict`: Prevents overlapping student group schedules via `MembershipIndex`.
  - `RoomCapacity`: Enforces room capacity $\ge$ group size.
  - `FacultyAvailability` & `RoomAvailability`: Enforces availability windows.
  - `RoomFeatureCompatibility`: Matches session requirement tags (e.g. `LAB`) with room features.
- **Configurable Constraint Framework**:
  - `SubjectMaxPerDay`: Enforces that a given subject is scheduled at most $N$ times per day for any student group.
  - `Compile(p, instances)`: Deterministically compiles rule instances into an immutable `CompiledConstraintSet` with canonical JSON SHA-256 hashing and atomic failure semantics.

### 3. Solvers & Algorithms
- **Stage 1 — Backtracking CSP Solver**:
  - `MRV + Degree`: Selects the most constrained requirement first.
  - `LCV`: Orders room/slot candidate values to minimize future variable domain reduction.
  - `Forward Checking`: Prunes inconsistent values from unassigned variables.
  - `ValidateSolution`: Post-search double-check ensuring zero compiled hard constraint violations.
- **Stage 2 — Tabu Search Local Optimizer**:
  - `NewNeighborhoodGenerator`: Generates single-assignment and swap moves.
  - `EvaluateCandidateMove`: Apply $\rightarrow$ Compiled Move Validation $\rightarrow$ Score $\rightarrow$ defer Undo.
  - `TabuList`: Configurable `TabuTenure` tracking reverse move signatures.
  - `Aspiration`: Overrides tabu state if a move yields a global best score.
  - `Final Validation`: Full verification across all termination paths (iterations, duration, no-improvement, cancellation).

---

## 🚀 Quick Start

### Prerequisites
- **Go 1.22+** installed.

### Build and Run

```bash
# Clone repository
git clone https://github.com/sPreetham42/Curra.git
cd Curra

# Run the CLI solver on sample problem
go run ./cmd/solver

# Run with custom input problem JSON and node limits
go run ./cmd/solver -input=path/to/problem.json -max-nodes=100000
```

### Sample Problem & Output

```json
{
  "solution": {
    "assignments": [
      {
        "id": "req-cs101-theory#0",
        "courseOfferingId": "offering-cs101",
        "studentGroupId": "group-cs-a",
        "facultyId": "fac-smith",
        "roomId": "room-101",
        "timeSlotId": "mon-1",
        "sessionRequirementId": "req-cs101-theory",
        "instance": 0
      },
      {
        "id": "req-cs101-theory#1",
        "courseOfferingId": "offering-cs101",
        "studentGroupId": "group-cs-a",
        "facultyId": "fac-smith",
        "roomId": "room-101",
        "timeSlotId": "mon-2",
        "sessionRequirementId": "req-cs101-theory",
        "instance": 1
      }
    ],
    "score": {
      "hardViolations": 0,
      "softPenalty": 0,
      "breakdown": {
        "hardViolations": 0,
        "softPenalty": 0,
        "studentGapPenalty": 0
      }
    }
  },
  "diagnostics": {
    "status": "SOLVED",
    "nodesExplored": 0,
    "candidates": 0,
    "backtracks": 0,
    "message": "feasible timetable found"
  }
}
```

---

## 🧪 Testing & Verification

```bash
# Run all unit, integration, and property tests
go test ./...

# Run performance benchmarks with memory allocations
go test -run=^$ -bench=. -benchmem ./tests
```

### Benchmark Highlights
| Benchmark | Speed | Allocations |
|---|---|---|
| `BenchmarkEvaluateMove` | **~6.2 µs/op** | 35 allocs/op |
| `BenchmarkTabuSearch_MediumProblem` | **~8.7 ms/op** (1,000 candidate moves evaluated) | 49,867 allocs/op |
| `BenchmarkSearchModes (Heuristic)` | **~449 µs/op** | 1,734 allocs/op |

---

## 🗺️ Project Roadmap

- [x] **Phase 1**: Domain Model, In-Memory Problem Validation & Normalization.
- [x] **Phase 2**: Core Hard Constraints & Backtracking Solver.
- [x] **Phase 3**: MRV + Degree + LCV + Forward Checking Heuristic Engine.
- [x] **Phase 4**: Scoped Validators, SolutionIndex & In-Place Move Mutations.
- [x] **Phase 5**: Tabu Search Local Optimizer & StudentGapPenalty.
- [x] **Phase 6A**: Configurable Constraint Framework (`SubjectMaxPerDay`, Compiler, CSP & Tabu Integration).
- [ ] **Phase 6B**: Migration of Remaining 5 Built-in Hard Constraints to `ConstraintDef` Templates.
- [ ] **Phase 6C**: Soft Constraint Scoring Bridge & Weighted Multi-Objective Optimization.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
