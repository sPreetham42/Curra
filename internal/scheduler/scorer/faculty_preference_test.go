package scorer_test

import (
	"context"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// Helper to construct a basic test problem instance.
func createBasicTestProblem() problem.Problem {
	p := problem.Problem{
		TenantID:      "t1",
		Term:          model.Term{ID: "term1", TenantID: "t1", Name: "Fall 2026"},
		PeriodsPerDay: 5,
		Departments:   map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "t1", Name: "CS"}},
		Programs:      map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "BSCS"}},
		Classes: map[model.ClassID]model.Class{
			"c1": {ID: "c1", ProgramID: "p1", Name: "Class1", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1": {ID: "g1", ClassID: "c1", Name: "Group1", Size: 30},
			"g2": {ID: "g2", ClassID: "c1", Name: "Group2", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s1": {ID: "s1", Code: "CS101", Name: "Programming"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f1": {ID: "f1", Name: "Prof Alpha"},
			"f2": {ID: "f2", Name: "Prof Beta"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r1": {ID: "r1", Name: "Room 101", Capacity: 50},
			"r2": {ID: "r2", Name: "Room 102", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"ts1": {ID: "ts1", Day: time.Monday, Period: 1},
			"ts2": {ID: "ts2", Day: time.Monday, Period: 2},
			"ts3": {ID: "ts3", Day: time.Monday, Period: 3},
			"ts4": {ID: "ts4", Day: time.Tuesday, Period: 1},
			"ts5": {ID: "ts5", Day: time.Tuesday, Period: 2},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co1": {ID: "co1", TermID: "term1", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1"},
			"co2": {ID: "co2", TermID: "term1", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g2", FacultyID: "f2"},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"sr1": {ID: "sr1", CourseOfferingID: "co1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"sr2": {ID: "sr2", CourseOfferingID: "co2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f1", TimeSlotID: "ts1"},
			{FacultyID: "f1", TimeSlotID: "ts2"},
			{FacultyID: "f1", TimeSlotID: "ts3"},
			{FacultyID: "f1", TimeSlotID: "ts4"},
			{FacultyID: "f1", TimeSlotID: "ts5"},
			{FacultyID: "f2", TimeSlotID: "ts1"},
			{FacultyID: "f2", TimeSlotID: "ts2"},
			{FacultyID: "f2", TimeSlotID: "ts3"},
			{FacultyID: "f2", TimeSlotID: "ts4"},
			{FacultyID: "f2", TimeSlotID: "ts5"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r1", TimeSlotID: "ts1"},
			{RoomID: "r1", TimeSlotID: "ts2"},
			{RoomID: "r1", TimeSlotID: "ts3"},
			{RoomID: "r1", TimeSlotID: "ts4"},
			{RoomID: "r1", TimeSlotID: "ts5"},
			{RoomID: "r2", TimeSlotID: "ts1"},
			{RoomID: "r2", TimeSlotID: "ts2"},
			{RoomID: "r2", TimeSlotID: "ts3"},
			{RoomID: "r2", TimeSlotID: "ts4"},
			{RoomID: "r2", TimeSlotID: "ts5"},
		},
	}
	p.Prepare()
	return p
}

// ----------------------------------------------------------------------------
// A. No preferences
// ----------------------------------------------------------------------------

func TestFacultyPreference_NoPreferences(t *testing.T) {
	p := createBasicTestProblem()
	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})

	score := p.CalculateScore(&sol)
	if score.FacultyPreferencePenalty != 0 {
		t.Fatalf("expected FacultyPreferencePenalty=0 with no preferences, got %d", score.FacultyPreferencePenalty)
	}
}

// ----------------------------------------------------------------------------
// B. Preferred / Non-preferred slot
// ----------------------------------------------------------------------------

func TestFacultyPreference_PreferredVsNonPreferredSlot(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 5},
	}

	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})

	// Assigned to ts1 -> penalty should be 5
	score1 := p.CalculateScore(&sol)
	if score1.FacultyPreferencePenalty != 5 {
		t.Fatalf("expected FacultyPreferencePenalty=5 for ts1, got %d", score1.FacultyPreferencePenalty)
	}

	// Move to ts2 (no preference record) -> penalty should be 0
	sol2 := problem.NewSolution()
	sol2.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts2",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	score2 := p.CalculateScore(&sol2)
	if score2.FacultyPreferencePenalty != 0 {
		t.Fatalf("expected FacultyPreferencePenalty=0 for ts2, got %d", score2.FacultyPreferencePenalty)
	}
}

// ----------------------------------------------------------------------------
// C. Preference Weight Scaling
// ----------------------------------------------------------------------------

func TestFacultyPreference_WeightScaling(t *testing.T) {
	weights := []int{1, 3, 10, 50}
	for _, w := range weights {
		p := createBasicTestProblem()
		p.FacultyPreferences = []model.FacultyPreference{
			{FacultyID: "f1", TimeSlotID: "ts1", Weight: w},
		}

		sol := problem.NewSolution()
		sol.AddAssignment(&p, problem.Assignment{
			ID:                   "sr1#0",
			CourseOfferingID:     "co1",
			StudentGroupID:       "g1",
			FacultyID:            "f1",
			RoomID:               "r1",
			TimeSlotID:           "ts1",
			SessionRequirementID: "sr1",
			Instance:             0,
		})

		score := p.CalculateScore(&sol)
		if score.FacultyPreferencePenalty != w {
			t.Fatalf("expected raw penalty=%d for weight=%d, got %d", w, w, score.FacultyPreferencePenalty)
		}
	}
}

// ----------------------------------------------------------------------------
// D. Multiple Preferences for Same Faculty
// ----------------------------------------------------------------------------

func TestFacultyPreference_MultiplePreferences(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 4},
		{FacultyID: "f1", TimeSlotID: "ts2", Weight: 6},
	}

	p.SessionRequirements["sr1"] = model.SessionRequirement{
		ID: "sr1", CourseOfferingID: "co1", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1,
	}

	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#1",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts2",
		SessionRequirementID: "sr1",
		Instance:             1,
	})

	score := p.CalculateScore(&sol)
	// ts1 (4) + ts2 (6) = 10
	if score.FacultyPreferencePenalty != 10 {
		t.Fatalf("expected aggregated FacultyPreferencePenalty=10, got %d", score.FacultyPreferencePenalty)
	}
}

// ----------------------------------------------------------------------------
// E. Multiple Faculties Preference Isolation
// ----------------------------------------------------------------------------

func TestFacultyPreference_MultipleFacultiesIsolation(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 8},
		{FacultyID: "f2", TimeSlotID: "ts4", Weight: 12},
	}

	sol := problem.NewSolution()
	// f1 assigned to ts1 (penalty 8), f2 assigned to ts1 (no pref for f2 on ts1)
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co2",
		StudentGroupID:       "g2",
		FacultyID:            "f2",
		RoomID:               "r2",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	score := p.CalculateScore(&sol)
	if score.FacultyPreferencePenalty != 8 {
		t.Fatalf("expected penalty=8 (only f1 matched), got %d", score.FacultyPreferencePenalty)
	}
}

// ----------------------------------------------------------------------------
// F. Multi-Period Session Slot Expansion
// ----------------------------------------------------------------------------

func TestFacultyPreference_MultiPeriodSession(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 3},
		{FacultyID: "f1", TimeSlotID: "ts2", Weight: 5},
	}

	// 2-period lab session starting at ts1 (occupies ts1 and ts2)
	p.SessionRequirements["sr1"] = model.SessionRequirement{
		ID:               "sr1",
		CourseOfferingID: "co1",
		Type:             model.SessionTypeLab,
		SessionsPerWeek:  1,
		Duration:         2,
	}

	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})

	score := p.CalculateScore(&sol)
	// Duration=2 starting at ts1 occupies ts1 (3) and ts2 (5) -> 3 + 5 = 8
	if score.FacultyPreferencePenalty != 8 {
		t.Fatalf("expected multi-period penalty=8 (ts1:3 + ts2:5), got %d", score.FacultyPreferencePenalty)
	}
}

// ----------------------------------------------------------------------------
// G & H. Incremental Delta Equivalence (Move & Swap)
// ----------------------------------------------------------------------------

func TestFacultyPreference_IncrementalDeltaEquivalence(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 7},
		{FacultyID: "f1", TimeSlotID: "ts3", Weight: 3},
		{FacultyID: "f2", TimeSlotID: "ts4", Weight: 9},
	}

	sol := problem.NewSolution()
	a1 := problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	}
	a2 := problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co2",
		StudentGroupID:       "g2",
		FacultyID:            "f2",
		RoomID:               "r2",
		TimeSlotID:           "ts4",
		SessionRequirementID: "sr2",
		Instance:             0,
	}
	sol.AddAssignment(&p, a1)
	sol.AddAssignment(&p, a2)

	eval := localsearch.NewIncrementalScoreEvaluator(&p, &sol)
	fullBefore := p.CalculateScore(&sol)

	// Move a1 from ts1 to ts3
	move := localsearch.CandidateMove{
		Kind:        localsearch.MoveKindSingle,
		Assignment1: "sr1#0",
		From1:       problem.Placement{RoomID: "r1", TimeSlotID: "ts1"},
		To1:         problem.Placement{RoomID: "r1", TimeSlotID: "ts3"},
	}

	evalResult := eval.EvaluateCandidateMove(&p, &sol, move)

	// Apply move manually for full recomputation
	solAfter := sol.Clone()
	_ = solAfter.ApplyMove(&p, problem.Move{AssignmentID: "sr1#0", From: move.From1, To: move.To1})
	fullAfter := p.CalculateScore(&solAfter)

	if evalResult.SoftPenalty != fullAfter.SoftPenalty {
		t.Fatalf("Move incremental mismatch: eval=%d, full=%d (before=%d)", evalResult.SoftPenalty, fullAfter.SoftPenalty, fullBefore.SoftPenalty)
	}

	// Test Swap: swap a1 (at ts1) and a2 (at ts4)
	swap := localsearch.CandidateMove{
		Kind:        localsearch.MoveKindSwap,
		Assignment1: "sr1#0",
		From1:       problem.Placement{RoomID: "r1", TimeSlotID: "ts1"},
		To1:         problem.Placement{RoomID: "r2", TimeSlotID: "ts4"},
		Assignment2: "sr2#0",
		From2:       problem.Placement{RoomID: "r2", TimeSlotID: "ts4"},
		To2:         problem.Placement{RoomID: "r1", TimeSlotID: "ts1"},
	}

	swapEval := eval.EvaluateCandidateMove(&p, &sol, swap)

	solSwap := sol.Clone()
	_ = solSwap.ApplySwap(&p, problem.Move{AssignmentID: "sr1#0", From: swap.From1, To: swap.To1}, problem.Move{AssignmentID: "sr2#0", From: swap.From2, To: swap.To2})
	fullSwap := p.CalculateScore(&solSwap)

	if swapEval.SoftPenalty != fullSwap.SoftPenalty {
		t.Fatalf("Swap incremental mismatch: eval=%d, full=%d", swapEval.SoftPenalty, fullSwap.SoftPenalty)
	}
}

// ----------------------------------------------------------------------------
// I. Authoritative Verifier Parity
// ----------------------------------------------------------------------------

func TestFacultyPreference_AuthoritativeVerifierParity(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 6},
	}

	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co2",
		StudentGroupID:       "g2",
		FacultyID:            "f2",
		RoomID:               "r2",
		TimeSlotID:           "ts4",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	fullScore := p.CalculateScore(&sol)
	sol.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    fullScore.SoftPenalty,
		Breakdown:      fullScore,
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err != nil || !report.Valid {
		t.Fatalf("verifier failed solution verification: err=%v, report=%+v", err, report)
	}
}

// ----------------------------------------------------------------------------
// J. Hard Constraint Isolation
// ----------------------------------------------------------------------------

func TestFacultyPreference_HardConstraintIsolation(t *testing.T) {
	p := createBasicTestProblem()
	// Add extreme penalty weight (1000) for f1 on ts1
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 1000},
	}

	sol := problem.NewSolution()
	sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})

	// Check hard constraints directly
	hardRules := constraints.DefaultHardConstraints()
	var hardViolations []diagnostics.Violation
	for _, rule := range hardRules {
		hardViolations = append(hardViolations, rule.Check(&p, &sol, sol.Assignments[0])...)
	}

	if len(hardViolations) != 0 {
		t.Fatalf("FacultyPreference introduced unexpected hard violations: %+v", hardViolations)
	}
}

// ----------------------------------------------------------------------------
// K. Determinism
// ----------------------------------------------------------------------------

func TestFacultyPreference_Determinism(t *testing.T) {
	p := createBasicTestProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 5},
		{FacultyID: "f2", TimeSlotID: "ts4", Weight: 8},
	}

	initSol := problem.NewSolution()
	initSol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	initSol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co2",
		StudentGroupID:       "g2",
		FacultyID:            "f2",
		RoomID:               "r2",
		TimeSlotID:           "ts4",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 10,
		Seed:          12345,
	}

	ctx := context.Background()
	sol1, diag1, err1 := localsearch.TabuSearch(ctx, &p, initSol, opts)
	sol2, diag2, err2 := localsearch.TabuSearch(ctx, &p, initSol, opts)

	if err1 != nil || err2 != nil {
		t.Fatalf("TabuSearch failed: err1=%v, err2=%v", err1, err2)
	}

	if diag1.BestScore.SoftPenalty != diag2.BestScore.SoftPenalty {
		t.Fatalf("Non-deterministic score output: run1=%d, run2=%d", diag1.BestScore.SoftPenalty, diag2.BestScore.SoftPenalty)
	}

	for i := range sol1.Assignments {
		if sol1.Assignments[i] != sol2.Assignments[i] {
			t.Fatalf("Non-deterministic assignment output at index %d: %+v vs %+v", i, sol1.Assignments[i], sol2.Assignments[i])
		}
	}
}
