package curra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// Adapter implements the CurraAdapter interface.
// It is stateless, has no database dependencies, and is the ONLY package
// that imports CURRA solver packages.
type Adapter struct {
	logger *slog.Logger
}

// New creates a new CURRA adapter.
func New(logger *slog.Logger) *Adapter {
	return &Adapter{logger: logger}
}

// Solve runs the complete CURRA pipeline: validate -> presolve -> CSP -> Tabu -> verify.
func (a *Adapter) Solve(ctx context.Context, req SolveRequest) (SolveResponse, error) {
	start := time.Now()

	// Parse problem from JSON
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return SolveResponse{
			Status: "INVALID_PROBLEM",
			Diagnostics: DiagnosticsDTO{
				Status:  "INVALID_PROBLEM",
				Message: fmt.Sprintf("failed to parse problem: %v", err),
			},
		}, fmt.Errorf("parse problem: %w", err)
	}

	// Parse constraints
	var constraintInstances []constraints.ConstraintInstance
	if len(req.ConstraintsJSON) > 0 {
		if err := json.Unmarshal(req.ConstraintsJSON, &constraintInstances); err != nil {
			return SolveResponse{
				Status: "INVALID_PROBLEM",
				Diagnostics: DiagnosticsDTO{
					Status:  "INVALID_PROBLEM",
					Message: fmt.Sprintf("failed to parse constraints: %v", err),
				},
			}, fmt.Errorf("parse constraints: %w", err)
		}
	}

	// Compute RuleSetHash
	_, ruleSetHash, _ := constraints.Compile(&p, constraintInstances)

	metadata := SolveMetadata{
		Version:     engine.Version,
		Commit:      engine.Commit,
		BuildAt:     engine.BuildAt,
		RuleSetHash: ruleSetHash,
	}

	// Build engine request
	engineReq := engine.Request{
		Problem:     p,
		Constraints: constraintInstances,
		SolveOptions: problem.SolveOptions{
			MaxNodes:   req.MaxNodes,
			SearchMode: problem.SearchMode(req.SearchMode),
		},
		TabuOptions: localsearch.TabuSearchOptions{
			Seed: req.Seed,
		},
		DisableOptimize: req.DisableOptimize,
	}

	if req.ObjectiveWeights != nil {
		objConfig := scorer.DefaultObjectiveConfig()
		for id, weight := range req.ObjectiveWeights {
			objConfig.Components = append(objConfig.Components, scorer.ObjectiveComponent{
				ID:     scorer.ObjectiveID(id),
				Weight: weight,
			})
		}
		engineReq.ObjectiveConfig = &objConfig
	}

	// Execute CURRA
	resp, err := engine.Solve(ctx, engineReq)
	if err != nil {
		return SolveResponse{
			Status:      string(resp.Diagnostics.Status),
			Diagnostics: mapDiagnostics(resp.Diagnostics),
			Score:       mapScore(resp.Score),
			Violations:  mapViolations(resp.Diagnostics.Violations),
			Metadata:    metadata,
		}, err
	}

	// Marshal solution back to JSON
	var solutionJSON json.RawMessage
	if len(resp.Solution.Assignments) > 0 {
		solutionJSON, err = json.Marshal(resp.Solution)
		if err != nil {
			return SolveResponse{}, fmt.Errorf("marshal solution: %w", err)
		}
	}

	duration := time.Since(start)
	a.logger.Info("curra solve complete",
		"status", resp.Diagnostics.Status,
		"duration", duration,
		"nodes", resp.Diagnostics.NodesExplored,
		"backtracks", resp.Diagnostics.Backtracks,
	)

	return SolveResponse{
		Status:      string(resp.Diagnostics.Status),
		Solution:    solutionJSON,
		Score:       mapScore(resp.Score),
		Diagnostics: mapDiagnostics(resp.Diagnostics),
		Violations:  mapViolations(resp.Diagnostics.Violations),
		Metadata:    metadata,
	}, nil
}

// Verify independently checks a stored solution against a snapshot.
func (a *Adapter) Verify(ctx context.Context, req VerifyRequest) (VerifyResponse, error) {
	// Parse problem
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return VerifyResponse{
			Valid:  false,
			Status: "INVALID_PROBLEM",
		}, fmt.Errorf("parse problem: %w", err)
	}

	// Parse solution
	var sol problem.Solution
	if err := json.Unmarshal(req.SolutionJSON, &sol); err != nil {
		return VerifyResponse{
			Valid:  false,
			Status: "INVALID_RESULT",
		}, fmt.Errorf("parse solution: %w", err)
	}

	// Parse optional constraints
	var compiledConstraints *constraints.CompiledConstraintSet
	if len(req.ConstraintsJSON) > 0 {
		var constraintInstances []constraints.ConstraintInstance
		if err := json.Unmarshal(req.ConstraintsJSON, &constraintInstances); err != nil {
			return VerifyResponse{
				Valid:  false,
				Status: "INVALID_PROBLEM",
			}, fmt.Errorf("parse constraints: %w", err)
		}
		compiled, _, compileErrs := constraints.Compile(&p, constraintInstances)
		if len(compileErrs) > 0 {
			return VerifyResponse{
				Valid:  false,
				Status: "INVALID_PROBLEM",
			}, fmt.Errorf("compile constraints: %v", compileErrs)
		}
		compiledConstraints = compiled
	}

	var objConfig *scorer.ObjectiveConfig
	if req.ObjectiveWeights != nil {
		cfg := scorer.DefaultObjectiveConfig()
		for id, weight := range req.ObjectiveWeights {
			cfg.Components = append(cfg.Components, scorer.ObjectiveComponent{
				ID:     scorer.ObjectiveID(id),
				Weight: weight,
			})
		}
		objConfig = &cfg
	}

	// Build verify options
	opts := verifier.VerifyOptions{
		Compiled:        compiledConstraints,
		ObjectiveConfig: objConfig,
	}

	// Execute verification
	report, err := verifier.VerifySolution(&p, &sol, opts)
	if err != nil {
		return VerifyResponse{
			Valid:      false,
			Status:     string(report.Status),
			Violations: mapViolations(report.Violations),
		}, err
	}

	var breakdown scorer.ScoreBreakdown
	if objConfig != nil {
		breakdown = p.StudentGapPenaltyWithConfig(&sol, *objConfig)
	} else {
		breakdown = p.StudentGapPenalty(&sol)
	}

	computedScore := scorer.Score{
		HardViolations: len(report.Violations),
		SoftPenalty:    breakdown.SoftPenalty,
		Breakdown:      breakdown,
	}

	return VerifyResponse{
		Valid:      report.Valid,
		Status:     string(report.Status),
		Violations: mapViolations(report.Violations),
		Score:      mapScore(computedScore),
	}, nil
}

// ValidateMove tests a manual move without mutating the original solution.
func (a *Adapter) ValidateMove(ctx context.Context, req ValidateMoveRequest) (ValidateMoveResponse, error) {
	// Parse problem
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("parse problem: %w", err)
	}

	// Parse solution
	var sol problem.Solution
	if err := json.Unmarshal(req.SolutionJSON, &sol); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_RESULT"}, fmt.Errorf("parse solution: %w", err)
	}

	// Prepare problem indexes if needed
	p.Prepare()

	// Clone solution (Clone-before-mutate)
	cloned := sol.Clone()

	// Build problem.Move
	move := problem.Move{
		AssignmentID: problem.AssignmentID(req.Move.AssignmentID),
		From: problem.Placement{
			RoomID:     model.RoomID(req.Move.From.RoomID),
			TimeSlotID: model.TimeSlotID(req.Move.From.TimeSlotID),
		},
		To: problem.Placement{
			RoomID:     model.RoomID(req.Move.To.RoomID),
			TimeSlotID: model.TimeSlotID(req.Move.To.TimeSlotID),
		},
	}

	// Apply move to clone
	if err := cloned.ApplyMove(&p, move); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_RESULT"}, fmt.Errorf("apply move: %w", err)
	}

	// Parse optional constraints
	var compiledConstraints *constraints.CompiledConstraintSet
	if len(req.ConstraintsJSON) > 0 {
		var constraintInstances []constraints.ConstraintInstance
		if err := json.Unmarshal(req.ConstraintsJSON, &constraintInstances); err != nil {
			return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("parse constraints: %w", err)
		}
		compiled, _, compileErrs := constraints.Compile(&p, constraintInstances)
		if len(compileErrs) > 0 {
			return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("compile constraints: %v", compileErrs)
		}
		compiledConstraints = compiled
	}

	// Verify cloned solution
	opts := verifier.VerifyOptions{Compiled: compiledConstraints}
	report, err := verifier.VerifySolution(&p, &cloned, opts)
	if err != nil {
		return ValidateMoveResponse{
			Valid:      false,
			Status:     string(report.Status),
			Violations: mapViolations(report.Violations),
		}, err
	}

	clonedJSON, err := json.Marshal(cloned)
	if err != nil {
		return ValidateMoveResponse{}, fmt.Errorf("marshal updated solution: %w", err)
	}

	breakdown := p.StudentGapPenalty(&cloned)
	computedScore := scorer.Score{
		HardViolations: len(report.Violations),
		SoftPenalty:    breakdown.SoftPenalty,
		Breakdown:      breakdown,
	}

	return ValidateMoveResponse{
		Valid:      report.Valid,
		Status:     string(report.Status),
		Violations: mapViolations(report.Violations),
		Score:      mapScore(computedScore),
		Solution:   clonedJSON,
	}, nil
}

// ValidateSwap tests a manual swap of two assignments without mutating the original solution.
func (a *Adapter) ValidateSwap(ctx context.Context, req ValidateSwapRequest) (ValidateMoveResponse, error) {
	// Parse problem
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("parse problem: %w", err)
	}

	// Parse solution
	var sol problem.Solution
	if err := json.Unmarshal(req.SolutionJSON, &sol); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_RESULT"}, fmt.Errorf("parse solution: %w", err)
	}

	// Prepare problem indexes if needed
	p.Prepare()

	// Clone solution (Clone-before-mutate)
	cloned := sol.Clone()

	move1 := problem.Move{
		AssignmentID: problem.AssignmentID(req.Swap.Assignment1ID),
		From: problem.Placement{
			RoomID:     model.RoomID(req.Swap.Placement1.RoomID),
			TimeSlotID: model.TimeSlotID(req.Swap.Placement1.TimeSlotID),
		},
		To: problem.Placement{
			RoomID:     model.RoomID(req.Swap.Placement2.RoomID),
			TimeSlotID: model.TimeSlotID(req.Swap.Placement2.TimeSlotID),
		},
	}

	move2 := problem.Move{
		AssignmentID: problem.AssignmentID(req.Swap.Assignment2ID),
		From: problem.Placement{
			RoomID:     model.RoomID(req.Swap.Placement2.RoomID),
			TimeSlotID: model.TimeSlotID(req.Swap.Placement2.TimeSlotID),
		},
		To: problem.Placement{
			RoomID:     model.RoomID(req.Swap.Placement1.RoomID),
			TimeSlotID: model.TimeSlotID(req.Swap.Placement1.TimeSlotID),
		},
	}

	// Apply swap to clone
	if err := cloned.ApplySwap(&p, move1, move2); err != nil {
		return ValidateMoveResponse{Valid: false, Status: "INVALID_RESULT"}, fmt.Errorf("apply swap: %w", err)
	}

	// Parse optional constraints
	var compiledConstraints *constraints.CompiledConstraintSet
	if len(req.ConstraintsJSON) > 0 {
		var constraintInstances []constraints.ConstraintInstance
		if err := json.Unmarshal(req.ConstraintsJSON, &constraintInstances); err != nil {
			return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("parse constraints: %w", err)
		}
		compiled, _, compileErrs := constraints.Compile(&p, constraintInstances)
		if len(compileErrs) > 0 {
			return ValidateMoveResponse{Valid: false, Status: "INVALID_PROBLEM"}, fmt.Errorf("compile constraints: %v", compileErrs)
		}
		compiledConstraints = compiled
	}

	// Verify cloned solution
	opts := verifier.VerifyOptions{Compiled: compiledConstraints}
	report, err := verifier.VerifySolution(&p, &cloned, opts)
	if err != nil {
		return ValidateMoveResponse{
			Valid:      false,
			Status:     string(report.Status),
			Violations: mapViolations(report.Violations),
		}, err
	}

	clonedJSON, err := json.Marshal(cloned)
	if err != nil {
		return ValidateMoveResponse{}, fmt.Errorf("marshal updated solution: %w", err)
	}

	breakdown := p.StudentGapPenalty(&cloned)
	computedScore := scorer.Score{
		HardViolations: len(report.Violations),
		SoftPenalty:    breakdown.SoftPenalty,
		Breakdown:      breakdown,
	}

	return ValidateMoveResponse{
		Valid:      report.Valid,
		Status:     string(report.Status),
		Violations: mapViolations(report.Violations),
		Score:      mapScore(computedScore),
		Solution:   clonedJSON,
	}, nil
}

// CompileConstraints validates and compiles constraint instances.
func (a *Adapter) CompileConstraints(ctx context.Context, req CompileRequest) (CompileResponse, error) {
	var p problem.Problem
	if err := json.Unmarshal(req.ProblemJSON, &p); err != nil {
		return CompileResponse{}, fmt.Errorf("parse problem: %w", err)
	}

	var constraintInstances []constraints.ConstraintInstance
	if len(req.ConstraintsJSON) > 0 {
		if err := json.Unmarshal(req.ConstraintsJSON, &constraintInstances); err != nil {
			return CompileResponse{}, fmt.Errorf("parse constraints: %w", err)
		}
	}

	compiled, hash, compileErrs := constraints.Compile(&p, constraintInstances)
	if len(compileErrs) > 0 {
		var dtos []CompileError
		for _, e := range compileErrs {
			dtos = append(dtos, CompileError{
				TemplateID: string(e.TemplateID),
				Field:      e.Field,
				Message:    e.Message,
			})
		}
		return CompileResponse{Errors: dtos}, nil
	}

	_ = compiled
	return CompileResponse{
		RuleSetHash: hash,
	}, nil
}

// Capabilities returns the engine version and algorithm capabilities manifest.
func (a *Adapter) Capabilities() SolverCapabilities {
	return SolverCapabilities{
		Version:    engine.Version,
		Commit:     engine.Commit,
		BuildAt:    engine.BuildAt,
		Stages:     []string{"CSP Backtracking", "Tabu Search", "Independent Verification"},
		Algorithms: []string{"MRV", "Degree Heuristic", "LCV", "Forward Checking", "Tabu Search"},
	}
}
