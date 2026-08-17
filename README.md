# Timetable Platform

Phase 1 implements the pure Go scheduling core for an academic timetable platform.

Included now:

- Domain model for departments, programs, classes, student groups, subjects, offerings, session requirements, faculty, rooms, terms, and recurring weekly time slots
- Problem, assignment, solution, indexed lookup structures, solve options, diagnostics, violations, and score placeholder
- Hard constraints for faculty conflicts, room conflicts, student-group conflicts, room capacity, faculty availability, room availability, and room feature compatibility
- Basic deterministic backtracking solver
- CLI runner with an embedded sample problem or JSON input
- Focused Go tests

Not included in Phase 1:

- HTTP APIs
- React
- PostgreSQL, Redis, Docker, queues, workers, authentication, or cloud infrastructure
- CP-SAT, genetic algorithms, soft-constraint optimization, AI, or speculative service abstractions

## Run Tests

```sh
go test ./...
```

## Run Solver CLI

Run the embedded sample:

```sh
go run ./cmd/solver
```

Run a JSON-encoded `problem.Problem`:

```sh
go run ./cmd/solver -input ./problem.json -max-nodes 100000
```

The CLI prints a JSON object containing the feasible solution, if found, and search diagnostics.

## Architecture

The scheduling core lives under `internal/scheduler` and is intentionally independent of transport, persistence, frontend, workers, and infrastructure concerns.

Current package layout:

- `model`: academic domain entities and IDs
- `problem`: scheduling problem, assignments, solution index, and solve options
- `constraints`: extensible hard-constraint abstraction and Phase 1 constraints
- `solver/backtracking`: basic backtracking implementation
- `diagnostics`: structured violations and solver diagnostics
- `scorer`: score placeholder for future optimization phases
