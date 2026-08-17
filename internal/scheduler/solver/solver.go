package solver

import (
	"context"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

type Solver interface {
	Solve(ctx context.Context, problem problem.Problem, options problem.SolveOptions) (problem.Solution, diagnostics.Diagnostics, error)
}
