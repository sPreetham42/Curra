package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// Request defines the input parameters for the canonical CURA engine solve orchestrator.
type Request struct {
	Problem         problem.Problem                    `json:"problem"`
	Constraints     []constraints.ConstraintInstance   `json:"constraints,omitempty"`
	SolveOptions    problem.SolveOptions               `json:"solveOptions,omitempty"`
	TabuOptions     localsearch.TabuSearchOptions      `json:"tabuOptions,omitempty"`
	ObjectiveConfig *scorer.ObjectiveConfig            `json:"objectiveConfig,omitempty"`
	DisableOptimize bool                               `json:"disableOptimize,omitempty"`
}

// Response contains the final solved timetable solution, objective score, and diagnostics.
type Response struct {
	Solution    problem.Solution        `json:"solution"`
	Diagnostics diagnostics.Diagnostics `json:"diagnostics"`
	Score       scorer.Score            `json:"score"`
}

// Solve executes the canonical CURA scheduling and optimization pipeline:
// 1. Problem validation
// 2. Problem preparation & derived indexing
// 3. Constraint compilation
// 4. Objective validation
// 5. CSP feasibility search
// 6. Tabu local search optimization (if not disabled)
// 7. Final authoritative result verification
// 8. Result & diagnostic synthesis
func Solve(ctx context.Context, req Request) (Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Structural Validation
	if violations := problem.Validate(req.Problem); len(violations) > 0 {
		return Response{
			Solution: problem.Solution{},
			Diagnostics: diagnostics.Diagnostics{
				Status:     diagnostics.SolveStatusInvalidProblem,
				Violations: violations,
				Message:    fmt.Sprintf("problem validation failed with %d violations", len(violations)),
			},
		}, backtracking.ErrInvalidProblem
	}

	// 2. Problem Preparation
	req.Problem.Prepare()

	// 3. Constraint Compilation
	compiled, _, compileErrors := constraints.Compile(&req.Problem, req.Constraints)
	if len(compileErrors) > 0 {
		err := constraints.CompileErrors(compileErrors)
		return Response{
			Solution: problem.Solution{},
			Diagnostics: diagnostics.Diagnostics{
				Status:  diagnostics.SolveStatusInvalidProblem,
				Message: fmt.Sprintf("constraint compilation: %v", err),
			},
		}, err
	}

	// 4. Objective Configuration Validation
	objConfig := scorer.DefaultObjectiveConfig()
	if req.ObjectiveConfig != nil {
		objConfig = *req.ObjectiveConfig
	}
	for _, comp := range objConfig.Components {
		if comp.Weight < 0 {
			return Response{
				Solution: problem.Solution{},
				Diagnostics: diagnostics.Diagnostics{
					Status:  diagnostics.SolveStatusInvalidProblem,
					Message: fmt.Sprintf("invalid objective configuration: negative weight %d for component '%s'", comp.Weight, comp.ID),
				},
			}, fmt.Errorf("invalid objective weight %d for '%s'", comp.Weight, comp.ID)
		}
		if comp.ID == "" {
			return Response{
				Solution: problem.Solution{},
				Diagnostics: diagnostics.Diagnostics{
					Status:  diagnostics.SolveStatusInvalidProblem,
					Message: "invalid objective configuration: empty component ID",
				},
			}, errors.New("empty objective component ID")
		}
	}

	// 5. CSP Feasibility Phase
	cspSolver := backtracking.NewWithCompiled(compiled)
	cspSol, cspDiag, cspErr := cspSolver.Solve(ctx, req.Problem, req.SolveOptions)

	if cspErr != nil || cspDiag.Status != diagnostics.SolveStatusSolved {
		return Response{
			Solution:    cspSol,
			Diagnostics: cspDiag,
			Score:       cspSol.Score,
		}, cspErr
	}

	currentSol := cspSol
	currentDiag := cspDiag

	// 6. Tabu Optimization Phase
	if !req.DisableOptimize {
		tabuOpts := req.TabuOptions
		tabuOpts.Compiled = compiled
		tabuOpts.ObjectiveConfig = &objConfig

		tabuSearcher := localsearch.NewWithCompiled(compiled)
		optSol, tabuDiag, tabuErr := tabuSearcher.Search(ctx, &req.Problem, cspSol, tabuOpts)

		if tabuErr != nil || (tabuDiag.Status != diagnostics.SolveStatusSolved && tabuDiag.Status != "") {
			if tabuDiag.Status == diagnostics.SolveStatusCancelled || tabuDiag.Status == diagnostics.SolveStatusDeadlineExceeded {
				return Response{
					Solution: optSol,
					Diagnostics: diagnostics.Diagnostics{
						Status:        tabuDiag.Status,
						NodesExplored: cspDiag.NodesExplored,
						Backtracks:    cspDiag.Backtracks,
						Candidates:    cspDiag.Candidates + tabuDiag.MovesGenerated,
						Violations:    tabuDiag.Violations,
						Message:       fmt.Sprintf("tabu search stopped: %s", tabuDiag.Status),
					},
					Score: optSol.Score,
				}, tabuErr
			}
			return Response{
				Solution: optSol,
				Diagnostics: diagnostics.Diagnostics{
					Status:     tabuDiag.Status,
					Violations: tabuDiag.Violations,
					Message:    fmt.Sprintf("tabu optimization: %v", tabuErr),
				},
				Score: optSol.Score,
			}, tabuErr
		}

		currentSol = optSol
		currentDiag.NodesExplored = cspDiag.NodesExplored
		currentDiag.Backtracks = cspDiag.Backtracks
		currentDiag.Candidates = cspDiag.Candidates + tabuDiag.MovesGenerated
	}

	// 7. Authoritative Final Verification
	report, vErr := verifier.VerifySolution(&req.Problem, &currentSol, verifier.VerifyOptions{
		Compiled:        compiled,
		ObjectiveConfig: &objConfig,
	})

	if vErr != nil || !report.Valid || report.Status != diagnostics.SolveStatusSolved {
		currentDiag.Status = report.Status
		currentDiag.Violations = report.Violations
		currentDiag.Message = report.Message
		return Response{
			Solution:    currentSol,
			Diagnostics: currentDiag,
			Score:       currentSol.Score,
		}, vErr
	}

	// 8. Return Final Verified Result
	currentDiag.Status = diagnostics.SolveStatusSolved
	currentDiag.Violations = nil
	currentDiag.Message = "feasible and verified timetable found"

	return Response{
		Solution:    currentSol,
		Diagnostics: currentDiag,
		Score:       currentSol.Score,
	}, nil
}
