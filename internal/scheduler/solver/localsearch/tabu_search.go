package localsearch

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

var (
	ErrInitialSolutionInfeasible = errors.New("initial solution has hard constraint violations")
)

// TabuSearchOptions holds settings for Tabu Search optimizer.
type TabuSearchOptions struct {
	MaxIterations      int           `json:"maxIterations"`
	MaxDuration        time.Duration `json:"maxDuration"`
	NoImprovementLimit int           `json:"noImprovementLimit"`
	TabuTenure         int           `json:"tabuTenure"`
	MaxCandidates      int           `json:"maxCandidates"`
	Seed               int64         `json:"seed"`
}

// TabuDiagnostics holds performance and execution metrics.
type TabuDiagnostics struct {
	Iterations        int                   `json:"iterations"`
	MovesGenerated    int                   `json:"movesGenerated"`
	LegalMoves        int                   `json:"legalMoves"`
	IllegalMoves      int                   `json:"illegalMoves"`
	TabuRejectedMoves int                   `json:"tabuRejectedMoves"`
	AcceptedMoves     int                   `json:"acceptedMoves"`
	InitialScore      scorer.ScoreBreakdown `json:"initialScore"`
	BestScore         scorer.ScoreBreakdown `json:"bestScore"`
	Duration          time.Duration         `json:"duration"`
}

// TabuSearch optimizes a feasible solution by minimizing StudentGapPenalty.
func TabuSearch(ctx context.Context, p *problem.Problem, initialSolution problem.Solution, opts TabuSearchOptions) (problem.Solution, TabuDiagnostics, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Validate initial solution feasibility
	validator := NewMoveValidator()
	evaluator := FullScoreEvaluator{}
	hardConstraints := constraints.DefaultHardConstraints()

	for _, a := range initialSolution.Assignments {
		if violations := constraints.CheckAll(p, &initialSolution, a, hardConstraints); len(violations) > 0 {
			return initialSolution, TabuDiagnostics{}, ErrInitialSolutionInfeasible
		}
	}


	// 2. Set option defaults
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 1000
	}
	if opts.NoImprovementLimit <= 0 {
		opts.NoImprovementLimit = 100
	}
	if opts.TabuTenure <= 0 {
		opts.TabuTenure = 10
	}
	if opts.MaxCandidates <= 0 {
		opts.MaxCandidates = 100
	}
	if opts.Seed == 0 {
		opts.Seed = 42
	}

	rng := rand.New(rand.NewSource(opts.Seed))
	startTime := time.Now()

	currentSolution := initialSolution.Clone()
	initialScore := evaluator.Evaluate(p, &currentSolution)
	currentScore := initialScore

	bestSolution := currentSolution.Clone()
	bestScore := initialScore

	tabuList := NewTabuList(opts.TabuTenure)
	generator := NewNeighborhoodGenerator()

	diag := TabuDiagnostics{
		InitialScore: initialScore,
		BestScore:    initialScore,
	}

	noImprovementCount := 0

	for iteration := 0; iteration < opts.MaxIterations; iteration++ {
		select {
		case <-ctx.Done():
			diag.Duration = time.Since(startTime)
			bestSolution.Score = scorer.Score{
				HardViolations: 0,
				SoftPenalty:    bestScore.StudentGapPenalty,
				Breakdown:      bestScore,
			}
			return bestSolution, diag, nil
		default:
		}

		if opts.MaxDuration > 0 && time.Since(startTime) >= opts.MaxDuration {
			break
		}

		if noImprovementCount >= opts.NoImprovementLimit {
			break
		}

		diag.Iterations++

		candidates := generator.GenerateNeighbors(p, &currentSolution, rng, opts.MaxCandidates)
		diag.MovesGenerated += len(candidates)

		var bestAdmissibleCandidate *CandidateMove
		var bestAdmissibleResult EvaluationResult
		foundAdmissible := false

		for i := range candidates {
			cm := candidates[i]
			res, err := EvaluateCandidateMove(p, &currentSolution, cm, validator, evaluator)
			if err != nil || !res.Legal {
				diag.IllegalMoves++
				continue
			}
			diag.LegalMoves++

			isTabu := tabuList.IsTabu(cm.Signature(), iteration)

			// Aspiration: allowed if it produces a solution better than global best score
			if isTabu && res.Score.StudentGapPenalty < bestScore.StudentGapPenalty {
				isTabu = false
			}

			if isTabu {
				diag.TabuRejectedMoves++
				continue
			}

			if !foundAdmissible || res.Score.StudentGapPenalty < bestAdmissibleResult.Score.StudentGapPenalty {
				foundAdmissible = true
				bestAdmissibleCandidate = &candidates[i]
				bestAdmissibleResult = res
			}
		}

		if !foundAdmissible {
			noImprovementCount++
			continue
		}

		_ = ApplyCandidateMove(p, &currentSolution, *bestAdmissibleCandidate)
		currentScore = bestAdmissibleResult.Score
		diag.AcceptedMoves++

		tabuList.Record(bestAdmissibleCandidate.ReverseSignature(), iteration)

		if currentScore.StudentGapPenalty < bestScore.StudentGapPenalty {
			bestScore = currentScore
			bestSolution = currentSolution.Clone()
			noImprovementCount = 0
		} else {
			noImprovementCount++
		}
	}

	diag.Duration = time.Since(startTime)
	diag.BestScore = bestScore
	bestSolution.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    bestScore.StudentGapPenalty,
		Breakdown:      bestScore,
	}

	return bestSolution, diag, nil
}
