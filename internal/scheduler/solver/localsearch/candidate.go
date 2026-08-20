package localsearch

import (
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

type MoveKind int

const (
	MoveKindSingle MoveKind = iota
	MoveKindSwap
)

// CandidateMove represents a candidate local search move (single or swap).
type CandidateMove struct {
	Kind MoveKind

	Assignment1 problem.AssignmentID
	From1       problem.Placement
	To1         problem.Placement

	// Used only when Kind == MoveKindSwap
	Assignment2 problem.AssignmentID
	From2       problem.Placement
	To2         problem.Placement
}

// SingleMove constructs a CandidateMove for moving a single assignment.
func SingleMove(id problem.AssignmentID, from, to problem.Placement) CandidateMove {
	return CandidateMove{
		Kind:        MoveKindSingle,
		Assignment1: id,
		From1:       from,
		To1:         to,
	}
}

// SwapMove constructs a CandidateMove for swapping two assignments.
func SwapMove(id1 problem.AssignmentID, from1, to1 problem.Placement, id2 problem.AssignmentID, from2, to2 problem.Placement) CandidateMove {
	return CandidateMove{
		Kind:        MoveKindSwap,
		Assignment1: id1,
		From1:       from1,
		To1:         to1,
		Assignment2: id2,
		From2:       from2,
		To2:         to2,
	}
}

// Signature returns a stable string representation of the move.
func (cm CandidateMove) Signature() string {
	if cm.Kind == MoveKindSingle {
		return fmt.Sprintf("MOVE|%s|%s:%s|%s:%s",
			cm.Assignment1,
			cm.From1.RoomID, cm.From1.TimeSlotID,
			cm.To1.RoomID, cm.To1.TimeSlotID,
		)
	}

	a1, p1From, p1To := cm.Assignment1, cm.From1, cm.To1
	a2, p2From, p2To := cm.Assignment2, cm.From2, cm.To2

	if a2 < a1 {
		a1, a2 = a2, a1
		p1From, p2From = p2From, p1From
		p1To, p2To = p2To, p1To
	}

	return fmt.Sprintf("SWAP|%s(%s:%s->%s:%s)|%s(%s:%s->%s:%s)",
		a1, p1From.RoomID, p1From.TimeSlotID, p1To.RoomID, p1To.TimeSlotID,
		a2, p2From.RoomID, p2From.TimeSlotID, p2To.RoomID, p2To.TimeSlotID,
	)
}

// ReverseSignature returns the stable string representation of the move that reverses cm.
func (cm CandidateMove) ReverseSignature() string {
	if cm.Kind == MoveKindSingle {
		return fmt.Sprintf("MOVE|%s|%s:%s|%s:%s",
			cm.Assignment1,
			cm.To1.RoomID, cm.To1.TimeSlotID,
			cm.From1.RoomID, cm.From1.TimeSlotID,
		)
	}

	a1, p1From, p1To := cm.Assignment1, cm.To1, cm.From1
	a2, p2From, p2To := cm.Assignment2, cm.To2, cm.From2

	if a2 < a1 {
		a1, a2 = a2, a1
		p1From, p2From = p2From, p1From
		p1To, p2To = p2To, p1To
	}

	return fmt.Sprintf("SWAP|%s(%s:%s->%s:%s)|%s(%s:%s->%s:%s)",
		a1, p1From.RoomID, p1From.TimeSlotID, p1To.RoomID, p1To.TimeSlotID,
		a2, p2From.RoomID, p2From.TimeSlotID, p2To.RoomID, p2To.TimeSlotID,
	)
}

// ApplyCandidateMove mutates solution and index in place to apply candidate move.
func ApplyCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) error {
	if cm.Kind == MoveKindSingle {
		return solution.ApplyMove(p, problem.Move{
			AssignmentID: cm.Assignment1,
			From:         cm.From1,
			To:           cm.To1,
		})
	}

	m1 := problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1}
	m2 := problem.Move{AssignmentID: cm.Assignment2, From: cm.From2, To: cm.To2}
	return solution.ApplySwap(p, m1, m2)
}

// UndoCandidateMove mutates solution and index in place to restore candidate move.
func UndoCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) error {
	if cm.Kind == MoveKindSingle {
		return solution.UndoMove(p, problem.Move{
			AssignmentID: cm.Assignment1,
			From:         cm.From1,
			To:           cm.To1,
		})
	}

	m1 := problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1}
	m2 := problem.Move{AssignmentID: cm.Assignment2, From: cm.From2, To: cm.To2}
	return solution.UndoSwap(p, m1, m2)
}

// EvaluateCandidateMove executes the apply/validate/score/undo evaluation window for a candidate move.
func EvaluateCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove, validator MoveValidator, evaluator ScoreEvaluator) (EvaluationResult, error) {
	if p.IsLocked(cm.Assignment1) || (cm.Kind == MoveKindSwap && p.IsLocked(cm.Assignment2)) {
		return EvaluationResult{
			Legal:   false,
			Message: "cannot move locked assignment",
			Violations: []diagnostics.Violation{
				{
					ConstraintName: "LockedAssignment",
					Severity:       diagnostics.SeverityHard,
					Message:        "cannot move locked assignment",
				},
			},
		}, ErrLockedAssignment
	}

	if err := ApplyCandidateMove(p, solution, cm); err != nil {
		return EvaluationResult{Legal: false, Message: err.Error()}, err
	}
	defer func() {
		_ = UndoCandidateMove(p, solution, cm)
	}()

	m1 := problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1}
	violations := validator.Validate(p, solution, m1)
	if cm.Kind == MoveKindSwap {
		m2 := problem.Move{AssignmentID: cm.Assignment2, From: cm.From2, To: cm.To2}
		violations = append(violations, validator.Validate(p, solution, m2)...)
	}

	if len(violations) > 0 {
		return EvaluationResult{
			Legal:      false,
			Violations: violations,
			Message:    "candidate move violates hard constraints",
		}, nil
	}

	var score scorer.ScoreBreakdown
	if cse, ok := evaluator.(CandidateScoreEvaluator); ok {
		score = cse.EvaluateCandidateMove(p, solution, cm)
	} else {
		score = evaluator.Evaluate(p, solution)
	}

	return EvaluationResult{
		Legal: true,
		Score: score,
	}, nil
}
