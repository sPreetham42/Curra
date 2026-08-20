package localsearch

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

var (
	ErrInitialSolutionInfeasible = errors.New("initial solution has hard constraint violations")
)

// TabuSearchOptions holds settings for Tabu Search optimizer.
type TabuSearchOptions struct {
	MaxIterations      int                                `json:"maxIterations"`
	MaxDuration        time.Duration                      `json:"maxDuration"`
	NoImprovementLimit int                                `json:"noImprovementLimit"`
	TabuTenure         int                                `json:"tabuTenure"`
	MaxCandidates      int                                `json:"maxCandidates"`
	Seed               int64                              `json:"seed"`
	Compiled           *constraints.CompiledConstraintSet `json:"-"`
	ObjectiveConfig    *scorer.ObjectiveConfig            `json:"objectiveConfig,omitempty"`
}

// TabuDiagnostics holds performance and execution metrics.
type TabuDiagnostics struct {
	Status            diagnostics.SolveStatus `json:"status,omitempty"`
	Iterations        int                     `json:"iterations"`
	MovesGenerated    int                     `json:"movesGenerated"`
	LegalMoves        int                     `json:"legalMoves"`
	IllegalMoves      int                     `json:"illegalMoves"`
	TabuRejectedMoves int                     `json:"tabuRejectedMoves"`
	AcceptedMoves     int                     `json:"acceptedMoves"`
	InitialScore      scorer.ScoreBreakdown   `json:"initialScore"`
	BestScore         scorer.ScoreBreakdown   `json:"bestScore"`
	Violations        []diagnostics.Violation `json:"violations,omitempty"`
	Duration          time.Duration           `json:"duration"`
}

// TabuSearcher executes tabu search with optional compiled constraints.
type TabuSearcher struct {
	Compiled *constraints.CompiledConstraintSet
}

func New() *TabuSearcher {
	return &TabuSearcher{}
}

func NewWithCompiled(compiled *constraints.CompiledConstraintSet) *TabuSearcher {
	return &TabuSearcher{Compiled: compiled}
}

// Search executes Tabu Search to optimize a solution under hard constraints.
func (s *TabuSearcher) Search(ctx context.Context, p *problem.Problem, initialSolution problem.Solution, opts TabuSearchOptions) (problem.Solution, TabuDiagnostics, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Validate initial solution feasibility against legacy hard constraints and compiled hard constraints
	hardConstraints := constraints.DefaultHardConstraints()
	var initialViolations []diagnostics.Violation

	for _, a := range initialSolution.Assignments {
		if violations := constraints.CheckAll(p, &initialSolution, a, hardConstraints); len(violations) > 0 {
			initialViolations = append(initialViolations, violations...)
		}
	}

	if s.Compiled != nil && len(s.Compiled.Hard) > 0 {
		searchCtx := constraints.NewSearchCtx(p)
		for _, c := range s.Compiled.Hard {
			initialViolations = append(initialViolations, c.Evaluate(searchCtx, &initialSolution)...)
		}
	}

	if len(initialViolations) > 0 {
		initialSolution.Score = scorer.Score{
			HardViolations: len(initialViolations),
			SoftPenalty:    initialSolution.Score.SoftPenalty,
			Breakdown:      initialSolution.Score.Breakdown,
		}
		return initialSolution, TabuDiagnostics{
			Status:     diagnostics.SolveStatusInfeasible,
			Violations: initialViolations,
		}, ErrInitialSolutionInfeasible
	}

	// 2. Set validator
	var validator MoveValidator
	if s.Compiled != nil {
		validator = NewCompiledMoveValidator(s.Compiled)
	} else {
		validator = NewMoveValidator()
	}
	// 3. Set option defaults
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

	objConfig := scorer.DefaultObjectiveConfig()
	if opts.ObjectiveConfig != nil {
		objConfig = *opts.ObjectiveConfig
	}

	currentSolution := initialSolution.Clone()
	incEvaluator := NewIncrementalScoreEvaluatorWithConfig(p, &currentSolution, objConfig)
	initialScore := incEvaluator.Evaluate(p, &currentSolution)
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
			sol, d, _ := finalizeSolution(p, &bestSolution, bestScore, s.Compiled, &diag)
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				d.Status = diagnostics.SolveStatusDeadlineExceeded
			} else {
				d.Status = diagnostics.SolveStatusCancelled
			}
			return sol, d, nil
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
			res, err := EvaluateCandidateMove(p, &currentSolution, cm, validator, incEvaluator)
			if err != nil || !res.Legal {
				diag.IllegalMoves++
				continue
			}
			diag.LegalMoves++

			isTabu := tabuList.IsTabu(cm.Signature(), iteration)

			// Aspiration: allowed if it produces a solution better than global best score
			if isTabu && res.Score.SoftPenalty < bestScore.SoftPenalty {
				isTabu = false
			}

			if isTabu {
				diag.TabuRejectedMoves++
				continue
			}

			if !foundAdmissible || res.Score.SoftPenalty < bestAdmissibleResult.Score.SoftPenalty {
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
		incEvaluator.ApplyCandidateMove(p, &currentSolution, *bestAdmissibleCandidate)
		currentScore = bestAdmissibleResult.Score
		diag.AcceptedMoves++

		tabuList.Record(bestAdmissibleCandidate.ReverseSignature(), iteration)

		if currentScore.SoftPenalty < bestScore.SoftPenalty {
			bestScore = currentScore
			bestSolution = currentSolution.Clone()
			noImprovementCount = 0
		} else {
			noImprovementCount++
		}
	}

	diag.Duration = time.Since(startTime)
	diag.BestScore = bestScore
	return finalizeSolution(p, &bestSolution, bestScore, s.Compiled, &diag)
}

func finalizeSolution(p *problem.Problem, bestSolution *problem.Solution, bestScore scorer.ScoreBreakdown, compiled *constraints.CompiledConstraintSet, diag *TabuDiagnostics) (problem.Solution, TabuDiagnostics, error) {
	var hardViolations []diagnostics.Violation

	hardConstraints := constraints.DefaultHardConstraints()
	for _, a := range bestSolution.Assignments {
		hardViolations = append(hardViolations, constraints.CheckAll(p, bestSolution, a, hardConstraints)...)
	}

	if compiled != nil && len(compiled.Hard) > 0 {
		searchCtx := constraints.NewSearchCtx(p)
		for _, c := range compiled.Hard {
			hardViolations = append(hardViolations, c.Evaluate(searchCtx, bestSolution)...)
		}
	}

	if len(hardViolations) > 0 {
		bestSolution.Score = scorer.Score{
			HardViolations: len(hardViolations),
			SoftPenalty:    bestScore.SoftPenalty,
			Breakdown:      bestScore,
		}
		diag.Status = diagnostics.SolveStatusInfeasible
		diag.Violations = hardViolations
		return *bestSolution, *diag, ErrInitialSolutionInfeasible
	}

	bestSolution.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    bestScore.SoftPenalty,
		Breakdown:      bestScore,
	}
	diag.Status = diagnostics.SolveStatusSolved
	diag.Violations = nil
	return *bestSolution, *diag, nil
}

// TabuSearch optimizes a feasible solution by minimizing StudentGapPenalty.
func TabuSearch(ctx context.Context, p *problem.Problem, initialSolution problem.Solution, opts TabuSearchOptions) (problem.Solution, TabuDiagnostics, error) {
	if opts.Compiled != nil {
		return NewWithCompiled(opts.Compiled).Search(ctx, p, initialSolution, opts)
	}
	return New().Search(ctx, p, initialSolution, opts)
}
