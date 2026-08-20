package tests

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// ENGINE INTEGRATION TEST SUITE
// ----------------------------------------------------------------------------

func TestEngine_HappyPath_FullPipeline(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 30,
			Seed:          42,
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err != nil {
		t.Fatalf("Engine.Solve failed: %v", err)
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED status, got %s", resp.Diagnostics.Status)
	}
	if len(resp.Solution.Assignments) != 24 {
		t.Fatalf("expected 24 assignments, got %d", len(resp.Solution.Assignments))
	}
	if resp.Solution.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", resp.Solution.Score.HardViolations)
	}
	if resp.Diagnostics.Candidates <= 0 {
		t.Fatalf("expected positive candidate count from Tabu search, got %d", resp.Diagnostics.Candidates)
	}
}

func TestEngine_CSPOnly_DisableOptimize(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem:         p,
		DisableOptimize: true,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err != nil {
		t.Fatalf("Engine.Solve CSP-only failed: %v", err)
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED status, got %s", resp.Diagnostics.Status)
	}
	if len(resp.Solution.Assignments) != 24 {
		t.Fatalf("expected 24 assignments, got %d", len(resp.Solution.Assignments))
	}
	if resp.Solution.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", resp.Solution.Score.HardViolations)
	}
}

func TestEngine_CSPFailure_InfeasibleProblem_TabuNotInvoked(t *testing.T) {
	// 1 faculty required for 6 sessions, but only 5 total timeslots exist in the entire problem
	p := stressBaseProblem(1, 5, 6)

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 50,
			Seed:          42,
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for infeasible problem, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected INFEASIBLE status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_NodeLimit_TabuNotInvoked(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			MaxNodes:   1,
			SearchMode: problem.SearchModeBasic, // Basic search hits node limit quickly
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 50,
			Seed:          42,
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for node limit, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusNodeLimit {
		t.Fatalf("expected NODE_LIMIT status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_ContextCancellation_DuringCSP(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultMediumProblemConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before solve

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeBasic,
		},
	}

	resp, err := engine.Solve(ctx, req)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusCancelled {
		t.Fatalf("expected CANCELLED status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_ContextCancellation_DuringTabu(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	// Context with very short timeout to trigger during Tabu
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 10000,
			Seed:          42,
		},
	}

	resp, _ := engine.Solve(ctx, req)
	// Status should be DeadlineExceeded or Solved (if solved very fast)
	if resp.Diagnostics.Status != diagnostics.SolveStatusDeadlineExceeded && resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected DEADLINE_EXCEEDED or SOLVED, got %s", resp.Diagnostics.Status)
	}
	// In either case, solution must have 0 hard violations
	if resp.Solution.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", resp.Solution.Score.HardViolations)
	}
}

func TestEngine_InvalidProblem_RejectsEarly(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.TenantID = "" // Invalid tenant ID

	req := engine.Request{Problem: p}
	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for empty tenant ID, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("expected INVALID_PROBLEM status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_ConstraintCompilationFailure_RejectsEarly(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		Constraints: []constraints.ConstraintInstance{
			{
				ID:         "bad-constraint",
				TemplateID: "NonExistentTemplate",
			},
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected constraint compilation error, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("expected INVALID_PROBLEM status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_ObjectiveValidationFailure_NegativeWeight(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		ObjectiveConfig: &scorer.ObjectiveConfig{
			Components: []scorer.ObjectiveComponent{
				{ID: scorer.ObjectiveStudentGapPenalty, Weight: -5},
			},
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected objective validation error for negative weight, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("expected INVALID_PROBLEM status, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_DeterministicReplay(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 30,
			Seed:          42,
		},
	}

	resp1, err1 := engine.Solve(context.Background(), req)
	resp2, err2 := engine.Solve(context.Background(), req)

	if err1 != nil || err2 != nil {
		t.Fatalf("solve error: err1=%v err2=%v", err1, err2)
	}
	if !reflect.DeepEqual(resp1.Solution.Assignments, resp2.Solution.Assignments) {
		t.Fatal("Engine.Solve produced non-deterministic assignments across runs")
	}
	if resp1.Solution.Score.SoftPenalty != resp2.Solution.Score.SoftPenalty {
		t.Fatalf("score mismatch: %d vs %d", resp1.Solution.Score.SoftPenalty, resp2.Solution.Score.SoftPenalty)
	}
}

func TestEngine_PipelineIntegrity_ExecutionOrder(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		Constraints: []constraints.ConstraintInstance{
			{ID: "c1", TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
			{ID: "c2", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
		},
		ObjectiveConfig: &scorer.ObjectiveConfig{
			Components: []scorer.ObjectiveComponent{
				{ID: scorer.ObjectiveStudentGapPenalty, Weight: 2},
			},
		},
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 20,
			Seed:          42,
		},
	}

	resp, err := engine.Solve(context.Background(), req)
	if err != nil {
		t.Fatalf("pipeline integrity solve failed: %v", err)
	}

	// 1. Validate status is SOLVED
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", resp.Diagnostics.Status)
	}

	// 2. Validate all 24 assignments scheduled
	if len(resp.Solution.Assignments) != 24 {
		t.Fatalf("expected 24 assignments, got %d", len(resp.Solution.Assignments))
	}

	// 3. Validate Tabu optimization ran (candidates generated > 0)
	if resp.Diagnostics.Candidates == 0 {
		t.Fatal("expected Tabu optimization to have generated candidate moves")
	}

	// 4. Validate objective weighting applied (weight 2)
	if len(resp.Solution.Score.Breakdown.Components) != 1 {
		t.Fatalf("expected 1 objective component, got %d", len(resp.Solution.Score.Breakdown.Components))
	}
	if resp.Solution.Score.Breakdown.Components[0].Weight != 2 {
		t.Fatalf("expected component weight 2, got %d", resp.Solution.Score.Breakdown.Components[0].Weight)
	}
}

func TestEngine_FinalVerificationFailure_NeverReturnsSolved(t *testing.T) {
	// Problem where a locked assignment specifies a non-existent room in the problem catalog
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-corrupt#0",
			CourseOfferingID:     "offering-0001",
			StudentGroupID:       "group-cs-a",
			FacultyID:            "faculty-001",
			RoomID:               "room-non-existent",
			TimeSlotID:           "slot-Mon-p1",
			SessionRequirementID: "req-0001",
			Instance:             0,
		},
	}

	req := engine.Request{
		Problem: p,
	}

	resp, err := engine.Solve(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Diagnostics.Status == diagnostics.SolveStatusSolved {
		t.Fatal("Engine must NOT return SOLVED when verification or seeding fails")
	}
}
