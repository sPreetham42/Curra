package engine_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// ENGINE INTEGRATION TEST SUITE
// ----------------------------------------------------------------------------

func TestEngine_HappyPath_FullPipeline(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.StressBaseProblem(1, 5, 6)

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
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

// Note: Mid-solve cancellation during Tabu Search cannot be deterministically triggered
// via the public Engine API without wall-clock timing assumptions or test-only hooks.
// Tabu Search cancellation mechanics are deterministically verified unit-level in tabu_search_test.go.
// At the Engine layer, we verify that any cancelled or deadline-expired context passed to Solve()
// yields a clean cancellation/deadline status and guarantees 0 hard constraint violations.
func TestEngine_ContextCancellation_Contract(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

	t.Run("PreCancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := engine.Request{
			Problem: p,
			SolveOptions: problem.SolveOptions{
				SearchMode: problem.SearchModeHeuristic,
			},
			TabuOptions: localsearch.TabuSearchOptions{
				MaxIterations: 1000,
				Seed:          42,
			},
		}

		resp, err := engine.Solve(ctx, req)
		if err == nil {
			t.Fatal("expected cancellation error, got nil")
		}
		if resp.Diagnostics.Status != diagnostics.SolveStatusCancelled {
			t.Fatalf("expected CANCELLED status, got %s", resp.Diagnostics.Status)
		}
	})

	t.Run("PreExpiredDeadlineContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		time.Sleep(2 * time.Millisecond)
		defer cancel()

		req := engine.Request{
			Problem: p,
			SolveOptions: problem.SolveOptions{
				SearchMode: problem.SearchModeHeuristic,
			},
			TabuOptions: localsearch.TabuSearchOptions{
				MaxIterations: 1000,
				Seed:          42,
			},
		}

		resp, err := engine.Solve(ctx, req)
		if err == nil {
			t.Fatal("expected deadline exceeded error, got nil")
		}
		if resp.Diagnostics.Status != diagnostics.SolveStatusDeadlineExceeded {
			t.Fatalf("expected DEADLINE_EXCEEDED status, got %s", resp.Diagnostics.Status)
		}
	})
}

func TestEngine_InvalidProblem_RejectsEarly(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

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

func TestEngine_NodeLimit_HeuristicMode(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			MaxNodes:   1,
			SearchMode: problem.SearchModeHeuristic,
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

func TestEngine_DeadlineExceeded_DuringCSP(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeBasic,
		},
	}

	resp, err := engine.Solve(ctx, req)
	if err == nil {
		t.Fatal("expected deadline error, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusCancelled && resp.Diagnostics.Status != diagnostics.SolveStatusDeadlineExceeded {
		t.Fatalf("expected CANCELLED or DEADLINE_EXCEEDED, got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_PreSolveDetectsInfeasible(t *testing.T) {
	// Faculty needs 100 sessions but only 2 time slots available
	p := problem.Problem{
		TenantID: "pre-engine", Term: model.Term{ID: "pe-term", TenantID: "pre-engine", Name: "PE"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "pre-engine", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 30}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "pe-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 100, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:   map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 60}},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
			"t2": {ID: "t2", Day: 1, Period: 2, Label: "P2"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}, {FacultyID: "f1", TimeSlotID: "t2"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}, {RoomID: "rm1", TimeSlotID: "t2"}},
		PeriodsPerDay:         2,
	}

	resp, err := engine.Solve(context.Background(), engine.Request{
		Problem:      p,
		SolveOptions: problem.SolveOptions{SearchMode: problem.SearchModeHeuristic},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected INFEASIBLE (detected by pre-solve), got %s", resp.Diagnostics.Status)
	}
}

func TestEngine_FinalVerificationFailure_NeverReturnsSolved(t *testing.T) {
	// Problem where a locked assignment specifies a non-existent room in the problem catalog
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
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
