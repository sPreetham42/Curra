package localsearch

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// MoveValidator checks whether an applied move satisfies all hard constraints.
type MoveValidator struct {
	Validators []constraints.ScopedValidator
	Compiled   *constraints.CompiledConstraintSet
}

func NewMoveValidator() MoveValidator {
	return MoveValidator{Validators: constraints.DefaultScopedValidators()}
}

func NewCompiledMoveValidator(compiled *constraints.CompiledConstraintSet) MoveValidator {
	return MoveValidator{
		Validators: constraints.DefaultScopedValidators(),
		Compiled:   compiled,
	}
}

// Validate checks all scoped hard constraints on the moved assignment.
func (v MoveValidator) Validate(p *problem.Problem, solution *problem.Solution, move problem.Move) []diagnostics.Violation {
	if p.IsLocked(move.AssignmentID) {
		return []diagnostics.Violation{
			{
				ConstraintName: "LockedAssignment",
				Severity:       diagnostics.SeverityHard,
				Message:        "cannot move locked assignment",
				AssignmentID:   string(move.AssignmentID),
			},
		}
	}

	assignment, ok := solution.Index.AssignmentByID(move.AssignmentID)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "MoveValidation",
				Severity:       diagnostics.SeverityHard,
				Message:        "assignment not found in solution index",
				AssignmentID:   string(move.AssignmentID),
			},
		}
	}

	var violations []diagnostics.Violation
	for _, validator := range v.Validators {
		violations = append(violations, validator.Check(p, solution, assignment)...)
	}

	if v.Compiled != nil && len(v.Compiled.Hard) > 0 {
		ctx := constraints.NewSearchCtx(p)
		if err := solution.UndoMove(p, move); err == nil {
			for _, c := range v.Compiled.Hard {
				violations = append(violations, c.ViolatedByMove(ctx, solution, move)...)
			}
			_ = solution.ApplyMove(p, move)
		}
	}

	return violations
}
