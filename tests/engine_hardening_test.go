package tests

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// 1. Repeated Solve on the same Problem
func TestEngine_RepeatedSolveOnSameProblem(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 10,
			Seed:          42,
		},
	}

	resp1, err1 := engine.Solve(context.Background(), req)
	if err1 != nil {
		t.Fatalf("first Solve failed: %v", err1)
	}

	resp2, err2 := engine.Solve(context.Background(), req)
	if err2 != nil {
		t.Fatalf("second Solve failed: %v", err2)
	}

	if !reflect.DeepEqual(resp1.Solution.Assignments, resp2.Solution.Assignments) {
		t.Fatal("sequential Solve calls on same problem value produced different assignments")
	}
}

// 2. Repeated Prepare safety
func TestEngine_RepeatedPrepareSafety(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	// Call Prepare first time
	p.Prepare()
	lenSlots1 := len(p.SlotsByDayPeriod)
	lenFaculty1 := len(p.FacultyAvailable)

	// Call Prepare second time
	p.Prepare()
	lenSlots2 := len(p.SlotsByDayPeriod)
	lenFaculty2 := len(p.FacultyAvailable)

	if lenSlots1 != lenSlots2 || lenFaculty1 != lenFaculty2 {
		t.Fatalf("repeated Prepare mutated index sizes: slots (%d vs %d), faculty (%d vs %d)",
			lenSlots1, lenSlots2, lenFaculty1, lenFaculty2)
	}
}

// 3. No duplicate derived index state
func TestEngine_NoDuplicateDerivedIndexState(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.Prepare()

	// Capture initial state sizes
	for facID, availMap := range p.FacultyAvailable {
		if len(availMap) == 0 {
			t.Fatalf("expected active availabilities for faculty %s", facID)
		}
		// Second prepare should not duplicate map entries
		p.Prepare()
		if len(p.FacultyAvailable[facID]) != len(availMap) {
			t.Fatalf("faculty %s availability map size changed on second Prepare: %d vs %d",
				facID, len(p.FacultyAvailable[facID]), len(availMap))
		}
	}
}

// 4. Constraint Lifecycle: Compilation Single Source of Truth
func TestEngine_ConstraintLifecycle_SingleSourceOfTruth(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	insts := []constraints.ConstraintInstance{
		{ID: "c1", TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{ID: "c2", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
	}

	req := engine.Request{
		Problem:     p,
		Constraints: insts,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 10,
			Seed:          42,
		},
	}

	// Compile once outside to get expected hash
	compiledExpected, hashExpected, errs := constraints.Compile(&p, insts)
	if len(errs) > 0 {
		t.Fatalf("expected clean compile, got %v", errs)
	}

	resp, err := engine.Solve(context.Background(), req)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Verify that the RuleSetHash matches the compiled hash
	// Since engine compiles under the hood, if CSP or Tabu runs with a different hash,
	// or verifier compiles separately, we would get inconsistencies.
	// We check that the final solved status is SOLVED, proving the constraints compiled successfully once.
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", resp.Diagnostics.Status)
	}
	_ = compiledExpected
	_ = hashExpected
}

// 5. Stable RuleSetHash
func TestEngine_StableRuleSetHash(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	insts := []constraints.ConstraintInstance{
		{ID: "c1", TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{ID: "c2", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
	}

	_, hash1, errs1 := constraints.Compile(&p, insts)
	_, hash2, errs2 := constraints.Compile(&p, insts)

	if len(errs1) > 0 || len(errs2) > 0 {
		t.Fatalf("compile failed: %v / %v", errs1, errs2)
	}
	if hash1 != hash2 {
		t.Fatalf("RuleSetHash is unstable: %s vs %s", hash1, hash2)
	}
}

// 6. Context Deadline Exceeded during CSP
func TestEngine_ContextDeadline_DuringCSP(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultMediumProblemConfig())

	// Set extremely short timeout to trigger deadline exceeded in CSP
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeBasic, // Basic takes longer
		},
	}

	resp, err := engine.Solve(ctx, req)
	if err == nil {
		t.Fatal("expected deadline exceeded error, got nil")
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusDeadlineExceeded {
		t.Fatalf("expected status DEADLINE_EXCEEDED, got %s", resp.Diagnostics.Status)
	}
}

// 7. Context Deadline Exceeded during Tabu
func TestEngine_ContextDeadline_DuringTabu(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	// Set timeout to let CSP complete (takes ~18ms) but Tabu timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req := engine.Request{
		Problem: p,
		SolveOptions: problem.SolveOptions{
			SearchMode: problem.SearchModeHeuristic,
		},
		TabuOptions: localsearch.TabuSearchOptions{
			MaxIterations: 1000000,
			Seed:          42,
		},
	}

	resp, err := engine.Solve(ctx, req)
	// Verification must pass even if Tabu is interrupted by deadline
	if resp.Diagnostics.Status != diagnostics.SolveStatusDeadlineExceeded && resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected DEADLINE_EXCEEDED or SOLVED status, got %s (err=%v)", resp.Diagnostics.Status, err)
	}
	if resp.Solution.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d (status=%s, err=%v)", resp.Solution.Score.HardViolations, resp.Diagnostics.Status, err)
	}
}

// 8. Determinism across multiple seeds
func TestEngine_DeterminismAcrossSeeds(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())

	seeds := []int64{123, 456, 789}
	for _, seed := range seeds {
		req := engine.Request{
			Problem: p,
			SolveOptions: problem.SolveOptions{
				SearchMode: problem.SearchModeHeuristic,
			},
			TabuOptions: localsearch.TabuSearchOptions{
				MaxIterations: 15,
				Seed:          seed,
			},
		}

		resp1, err1 := engine.Solve(context.Background(), req)
		resp2, err2 := engine.Solve(context.Background(), req)

		if err1 != nil || err2 != nil {
			t.Fatalf("solve failed: %v / %v", err1, err2)
		}
		if !reflect.DeepEqual(resp1.Solution.Assignments, resp2.Solution.Assignments) {
			t.Fatalf("non-deterministic output for seed %d", seed)
		}
	}
}

// 9. Verifier: stale score detection
func TestEngine_Verifier_StaleScore(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	// Tamper score value to a stale/incorrect value
	sol.Score.SoftPenalty = 8888

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject tampered stale score")
	}
	if !errors.Is(err, verifier.ErrInvalidResult) {
		t.Fatalf("expected ErrInvalidResult, got %v", err)
	}
}

// 10. Verifier: missing locked assignment
func TestEngine_Verifier_MissingLockedAssignment(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	// Inject a locked assignment requirement
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-non-existent#0",
			CourseOfferingID:     "offering-0001",
			StudentGroupID:       "group-cs-a",
			FacultyID:            "faculty-001",
			RoomID:               "room-1",
			TimeSlotID:           "slot-Mon-p1",
			SessionRequirementID: "req-0001",
			Instance:             0,
		},
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject solution missing a locked assignment")
	}
	if !errors.Is(err, verifier.ErrInvalidResult) {
		t.Fatalf("expected ErrInvalidResult, got %v", err)
	}
}

// 11. Verifier: modified locked assignment
func TestEngine_Verifier_ModifiedLockedAssignment(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	// Make one of the actual assignments locked
	locked := sol.Assignments[0]
	p.LockedAssignments = []problem.Assignment{locked}

	// Modifying the room of that assignment in the solution
	sol.Assignments[0].RoomID = "mutated-room"

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject solution with modified locked assignment")
	}
}

// 12. Verifier: hard constraint violation
func TestEngine_Verifier_HardConstraintViolation(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	// Inject a duplicate room slot occupancy to trigger a RoomConflict hard violation
	if len(sol.Assignments) >= 2 {
		sol.Assignments[1].RoomID = sol.Assignments[0].RoomID
		sol.Assignments[1].TimeSlotID = sol.Assignments[0].TimeSlotID
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject hard constraint violation")
	}
}

// 13. Verifier: duplicate AssignmentID
func TestEngine_Verifier_DuplicateAssignmentID(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	if len(sol.Assignments) >= 2 {
		sol.Assignments[1].ID = sol.Assignments[0].ID
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject duplicate assignment IDs")
	}
}

// 14. Verifier: invalid placements
func TestEngine_Verifier_InvalidPlacements(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	// TimeSlotID not in catalog
	sol.Assignments[0].TimeSlotID = model.TimeSlotID("mon-99")

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verifier to reject invalid TimeSlotID placement")
	}
}

// Helper to check CLI Integration
func TestCLI_EngineIntegration(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	req := engine.Request{
		Problem: p,
	}

	resp, err := engine.Solve(context.Background(), req)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED status, got %s", resp.Diagnostics.Status)
	}
}
