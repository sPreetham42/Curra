package localsearch

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// MoveValidator checks whether an applied move satisfies all hard constraints.
type MoveValidator struct {
	Validators []constraints.ScopedValidator
}

func NewMoveValidator() MoveValidator {
	return MoveValidator{Validators: constraints.DefaultScopedValidators()}
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
	return violations
}
