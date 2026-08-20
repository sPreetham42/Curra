package localsearch

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

var ErrLockedAssignment = problem.ErrLockedAssignment

// ScoreEvaluator evaluates soft constraint score for a solution.
type ScoreEvaluator interface {
	Evaluate(p *problem.Problem, solution *problem.Solution) scorer.ScoreBreakdown
}

// FullScoreEvaluator calculates full student gap penalty with optional objective weighting.
type FullScoreEvaluator struct {
	Config scorer.ObjectiveConfig
}

func (e FullScoreEvaluator) Evaluate(p *problem.Problem, solution *problem.Solution) scorer.ScoreBreakdown {
	cfg := e.Config
	if len(cfg.Components) == 0 {
		cfg = scorer.DefaultObjectiveConfig()
	}
	return p.StudentGapPenaltyWithConfig(solution, cfg)
}

// EvaluationResult contains the validation and scoring results of evaluating a candidate move.
type EvaluationResult struct {
	Legal      bool                    `json:"legal"`
	Violations []diagnostics.Violation `json:"violations,omitempty"`
	Score      scorer.ScoreBreakdown   `json:"score,omitempty"`
	Message    string                  `json:"message,omitempty"`
}

// EvaluateMove executes the single apply/validate/score/undo evaluation window for a candidate move.
// CRITICAL: UndoMove is always protected by defer.
// Illegal moves are NOT scored.
func EvaluateMove(p *problem.Problem, solution *problem.Solution, move problem.Move, validator MoveValidator, evaluator ScoreEvaluator) (EvaluationResult, error) {
	if p.IsLocked(move.AssignmentID) {
		return EvaluationResult{
			Legal:   false,
			Message: "cannot move locked assignment",
			Violations: []diagnostics.Violation{
				{
					ConstraintName: "LockedAssignment",
					Severity:       diagnostics.SeverityHard,
					Message:        "cannot move locked assignment",
					AssignmentID:   string(move.AssignmentID),
				},
			},
		}, ErrLockedAssignment
	}

	if err := solution.ApplyMove(p, move); err != nil {
		return EvaluationResult{Legal: false, Message: err.Error()}, err
	}
	defer func() {
		_ = solution.UndoMove(p, move)
	}()

	violations := validator.Validate(p, solution, move)
	if len(violations) > 0 {
		return EvaluationResult{
			Legal:      false,
			Violations: violations,
			Message:    "move violates hard constraints",
		}, nil
	}

	var score scorer.ScoreBreakdown
	if cse, ok := evaluator.(CandidateScoreEvaluator); ok {
		score = cse.EvaluateCandidateMove(p, solution, CandidateMove{
			Kind:        MoveKindSingle,
			Assignment1: move.AssignmentID,
			From1:       move.From,
			To1:         move.To,
		})
	} else {
		score = evaluator.Evaluate(p, solution)
	}

	return EvaluationResult{
		Legal: true,
		Score: score,
	}, nil
}
