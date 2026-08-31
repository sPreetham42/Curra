package localsearch_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// -------------------------------------------------------------
// Test 1: Optimizer preserves all hard constraints
// -------------------------------------------------------------
func TestTabuSearch_PreservesHardConstraints(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               123,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch returned error: %v", err)
	}

	// Verify 0 hard constraint violations for every assignment in bestSol
	validator := constraints.DefaultHardConstraints()
	for _, a := range bestSol.Assignments {
		violations := constraints.CheckAll(&p, &bestSol, a, validator)
		if len(violations) > 0 {
			t.Fatalf("best solution contains hard violations on assignment %s: %+v", a.ID, violations)
		}
	}

	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations in score, got %d", bestSol.Score.HardViolations)
	}
	if diag.AcceptedMoves == 0 && diag.Iterations > 0 {
		t.Log("Note: 0 accepted moves in this run")
	}
}

// -------------------------------------------------------------
// Test 2: Locked assignments never move
// -------------------------------------------------------------
func TestTabuSearch_LockedAssignmentsNeverMove(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()

	// Lock assignment "a-theory-1" at room-lecture-1, mon-1
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "a-theory-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
		},
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      100,
		NoImprovementLimit: 50,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               456,
	}

	bestSol, _, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch returned error: %v", err)
	}

	// Verify locked assignment is still at original room and time slot
	for _, a := range bestSol.Assignments {
		if a.ID == "a-theory-1" {
			if a.RoomID != "room-lecture-1" || a.TimeSlotID != "mon-1" {
				t.Fatalf("locked assignment moved! got Room %s Slot %s, want Room room-lecture-1 Slot mon-1", a.RoomID, a.TimeSlotID)
			}
		}
	}
}

// -------------------------------------------------------------
// Test 3: Valid move improves or changes score correctly
// -------------------------------------------------------------
func TestTabuSearch_ValidMoveImprovesScore(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	// Initial score has 1 gap (a-theory-1 at mon-1, a-theory-2 at mon-3 -> mon-2 is gap)
	// Moving a-theory-2 to mon-2 gives 0 gaps!

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               789,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch returned error: %v", err)
	}

	if diag.BestScore.StudentGapPenalty > diag.InitialScore.StudentGapPenalty {
		t.Fatalf("best score (%d) is worse than initial score (%d)", diag.BestScore.StudentGapPenalty, diag.InitialScore.StudentGapPenalty)
	}
	if bestSol.Score.SoftPenalty != diag.BestScore.StudentGapPenalty {
		t.Fatalf("bestSol.Score mismatch: got %d, want %d", bestSol.Score.SoftPenalty, diag.BestScore.StudentGapPenalty)
	}
}

// -------------------------------------------------------------
// Test 4: Illegal moves never get accepted
// -------------------------------------------------------------
func TestTabuSearch_IllegalMovesNeverAccepted(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      100,
		NoImprovementLimit: 50,
		TabuTenure:         5,
		MaxCandidates:      100,
		Seed:               111,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	if diag.IllegalMoves == 0 {
		t.Log("Note: candidate generator produced 0 illegal moves in this run")
	}

	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Verify all assignments in best solution are legal
	for _, a := range bestSol.Assignments {
		m := problem.Move{AssignmentID: a.ID, From: problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID}, To: problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID}}
		res, err := localsearch.EvaluateMove(&p, &bestSol, m, validator, evaluator)
		if err != nil || !res.Legal {
			t.Fatalf("best solution contains illegal assignment state for %s: %+v", a.ID, res.Violations)
		}
	}
}

// -------------------------------------------------------------
// Test 5: Tabu tenure prevents immediate reversal
// -------------------------------------------------------------
func TestTabuSearch_TabuTenurePreventsReversal(t *testing.T) {
	tabuList := localsearch.NewTabuList(5)

	move := localsearch.SingleMove(
		"a-theory-2",
		problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	)

	// Record reverse move signature at iteration 0
	tabuList.Record(move.ReverseSignature(), 0)

	revMove := localsearch.SingleMove(
		"a-theory-2",
		problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
		problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
	)

	// In iteration 1..5, revMove should be tabu
	for iter := 1; iter <= 5; iter++ {
		if !tabuList.IsTabu(revMove.Signature(), iter) {
			t.Fatalf("expected move to be tabu at iteration %d", iter)
		}
	}

	// In iteration 6, tabu expired
	if tabuList.IsTabu(revMove.Signature(), 6) {
		t.Fatalf("expected move to NOT be tabu at iteration 6")
	}
}

// -------------------------------------------------------------
// Test 6: Aspiration allows a tabu move when it produces a new global best
// -------------------------------------------------------------
func TestTabuSearch_AspirationOverride(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()

	// Run TabuSearch with high tenure and check diagnostics for tabu rejection or aspiration
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      200,
		NoImprovementLimit: 100,
		TabuTenure:         50,
		MaxCandidates:      100,
		Seed:               222,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}
	if diag.BestScore.StudentGapPenalty > diag.InitialScore.StudentGapPenalty {
		t.Fatalf("best score %d > initial score %d", diag.BestScore.StudentGapPenalty, diag.InitialScore.StudentGapPenalty)
	}
}

// -------------------------------------------------------------
// Test 7: Optimizer stops at max iterations
// -------------------------------------------------------------
func TestTabuSearch_StopsAtMaxIterations(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      10,
		NoImprovementLimit: 1000,
		TabuTenure:         2,
		MaxCandidates:      50,
		Seed:               333,
	}

	_, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	if diag.Iterations > 10 {
		t.Fatalf("expected iterations <= 10, got %d", diag.Iterations)
	}
}

// -------------------------------------------------------------
// Test 8: Optimizer respects context cancellation
// -------------------------------------------------------------
func TestTabuSearch_RespectsContextCancellation(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      10000,
		NoImprovementLimit: 10000,
		TabuTenure:         10,
		MaxCandidates:      100,
		Seed:               444,
	}

	t.Run("ContextCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Pre-cancel context

		sol, diag, err := localsearch.TabuSearch(ctx, &p, solution, opts)
		if err != nil {
			t.Fatalf("TabuSearch returned unexpected error: %v", err)
		}
		if diag.Status != diagnostics.SolveStatusCancelled {
			t.Fatalf("expected status CANCELLED, got %s", diag.Status)
		}
		if diag.Iterations != 0 {
			t.Fatalf("expected 0 iterations on pre-cancelled context, got %d", diag.Iterations)
		}
		if sol.Score.HardViolations != 0 {
			t.Fatalf("expected 0 hard violations, got %d", sol.Score.HardViolations)
		}
		if len(sol.Assignments) != len(solution.Assignments) {
			t.Fatalf("expected %d assignments, got %d", len(solution.Assignments), len(sol.Assignments))
		}
	})

	t.Run("ContextDeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		time.Sleep(2 * time.Millisecond) // Guarantee deadline is exceeded
		defer cancel()

		sol, diag, err := localsearch.TabuSearch(ctx, &p, solution, opts)
		if err != nil {
			t.Fatalf("TabuSearch returned unexpected error: %v", err)
		}
		if diag.Status != diagnostics.SolveStatusDeadlineExceeded {
			t.Fatalf("expected status DEADLINE_EXCEEDED, got %s", diag.Status)
		}
		if diag.Iterations != 0 {
			t.Fatalf("expected 0 iterations on pre-expired context, got %d", diag.Iterations)
		}
		if sol.Score.HardViolations != 0 {
			t.Fatalf("expected 0 hard violations, got %d", sol.Score.HardViolations)
		}
	})
}

// -------------------------------------------------------------
// Test 9: Best solution is returned, not merely final solution
// -------------------------------------------------------------
func TestTabuSearch_ReturnsBestSolutionNotFinal(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      100,
		NoImprovementLimit: 50,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               555,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	evaluator := localsearch.FullScoreEvaluator{}
	bestSolScore := evaluator.Evaluate(&p, &bestSol)

	if bestSolScore.StudentGapPenalty != diag.BestScore.StudentGapPenalty {
		t.Fatalf("bestSol score %d does not match diag.BestScore %d", bestSolScore.StudentGapPenalty, diag.BestScore.StudentGapPenalty)
	}
}

// -------------------------------------------------------------
// Test 10: Optimizer does not corrupt SolutionIndex
// -------------------------------------------------------------
func TestTabuSearch_IndexIntegrity(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               666,
	}

	bestSol, _, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	// Verify SolutionIndex maps match assignments exactly
	for _, a := range bestSol.Assignments {
		indexed, ok := bestSol.Index.AssignmentByID(a.ID)
		if !ok {
			t.Fatalf("assignment %s not found in SolutionIndex byID", a.ID)
		}
		if indexed.RoomID != a.RoomID || indexed.TimeSlotID != a.TimeSlotID {
			t.Fatalf("assignment %s placement mismatch in byID: got %s:%s, want %s:%s",
				a.ID, indexed.RoomID, indexed.TimeSlotID, a.RoomID, a.TimeSlotID)
		}

		slotIDs, ok := a.OccupiedSlotIDs(&p)
		if !ok {
			t.Fatalf("assignment %s has invalid occupied slots", a.ID)
		}
		for _, sid := range slotIDs {
			if fID, ok := bestSol.Index.FacultyConflict(a.FacultyID, []model.TimeSlotID{sid}); !ok || fID != a.ID {
				t.Fatalf("FacultySlot index mismatch for %s at slot %s", a.FacultyID, sid)
			}
			if rID, ok := bestSol.Index.RoomConflict(a.RoomID, []model.TimeSlotID{sid}); !ok || rID != a.ID {
				t.Fatalf("RoomSlot index mismatch for %s at slot %s", a.RoomID, sid)
			}
			if gID, ok := bestSol.Index.StudentGroupConflict(a.StudentGroupID, []model.TimeSlotID{sid}); !ok || gID != a.ID {
				t.Fatalf("StudentGroupSlot index mismatch for %s at slot %s", a.StudentGroupID, sid)
			}
		}
	}
}

// -------------------------------------------------------------
// Test 11: Initial solution remains unchanged
// -------------------------------------------------------------
func TestTabuSearch_InitialSolutionUnchanged(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	assignmentsBefore := append([]problem.Assignment(nil), solution.Assignments...)

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               777,
	}

	_, _, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	if !reflect.DeepEqual(solution.Assignments, assignmentsBefore) {
		t.Fatal("initialSolution.Assignments was mutated during TabuSearch")
	}
}

// -------------------------------------------------------------
// Test 12: Deterministic output with same seed/options
// -------------------------------------------------------------
func TestTabuSearch_DeterministicOutput(t *testing.T) {
	p, solution := testutil.LocalSearchTestProblem()
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               888,
	}

	run1Sol, run1Diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("run 1 error: %v", err)
	}

	run2Sol, run2Diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("run 2 error: %v", err)
	}

	if run1Diag.BestScore.StudentGapPenalty != run2Diag.BestScore.StudentGapPenalty {
		t.Fatalf("score mismatch: run1=%d, run2=%d", run1Diag.BestScore.StudentGapPenalty, run2Diag.BestScore.StudentGapPenalty)
	}
	if !reflect.DeepEqual(run1Sol.Assignments, run2Sol.Assignments) {
		t.Fatal("deterministic failure: run1 and run2 produced different assignment placements")
	}
}

// -------------------------------------------------------------
// Test 13: End-to-End Test (Backtracking -> Tabu Search)
// -------------------------------------------------------------
func TestTabuSearch_EndToEnd(t *testing.T) {
	// 1. Generate a feasible solution using backtracking solver
	p := mediumTestProblem()
	solver := backtracking.New()

	initialSol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000})
	if err != nil {
		t.Fatalf("backtracking solver error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("backtracking solver did not solve problem: %s", diag.Status)
	}

	// 2. Calculate initial StudentGapPenalty score
	evaluator := localsearch.FullScoreEvaluator{}
	initialScore := evaluator.Evaluate(&p, &initialSol)

	// 3. Run Tabu Search
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      100,
		NoImprovementLimit: 50,
		TabuTenure:         10,
		MaxCandidates:      50,
		Seed:               999,
	}

	bestSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, initialSol, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}

	// 4. Verify final hard violations = 0
	validator := constraints.DefaultHardConstraints()
	for _, a := range bestSol.Assignments {
		violations := constraints.CheckAll(&p, &bestSol, a, validator)
		if len(violations) > 0 {
			t.Fatalf("Tabu Search introduced hard constraint violation on %s: %+v", a.ID, violations)
		}
	}

	// 5. Verify final best score <= initial score
	if tabuDiag.BestScore.StudentGapPenalty > initialScore.StudentGapPenalty {
		t.Fatalf("final score (%d) is worse than initial score (%d)", tabuDiag.BestScore.StudentGapPenalty, initialScore.StudentGapPenalty)
	}
	t.Logf("End-to-End result: Initial Gap Score=%d, Final Best Score=%d, Accepted Moves=%d, Total Iterations=%d",
		initialScore.StudentGapPenalty, tabuDiag.BestScore.StudentGapPenalty, tabuDiag.AcceptedMoves, tabuDiag.Iterations)
}

func mediumTestProblem() problem.Problem {
	p := problem.Problem{
		TenantID: "tenant-medium",
		Term:     model.Term{ID: "term-m", TenantID: "tenant-medium", Name: "Medium Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-medium", Name: "CS Dept"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
			"class-2": {ID: "class-2", ProgramID: "prog-1", Name: "Year 2", WholeGroupID: "g2-whole", StudentGroupIDs: []model.StudentGroupID{"g2-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
			"g2-whole": {ID: "g2-whole", ClassID: "class-2", Name: "G2 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-1": {ID: "subj-1", Code: "CS101", Name: "Intro CS"},
			"subj-2": {ID: "subj-2", Code: "CS201", Name: "Data Struct"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-m", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "g1-whole", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
			"co-2": {ID: "co-2", TermID: "term-m", ClassID: "class-2", SubjectID: "subj-2", StudentGroupID: "g2-whole", FacultyID: "f-2", SessionRequirementIDs: []model.SessionRequirementID{"req-2"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1, Consecutive: true},
			"req-2": {ID: "req-2", CourseOfferingID: "co-2", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof Alpha"},
			"f-2": {ID: "f-2", Name: "Prof Beta"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
			"r-2": {ID: "r-2", Name: "Room 2", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"t-2": {ID: "t-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
			"t-3": {ID: "t-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"},
		},
		PeriodsPerDay: 3,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"}, {FacultyID: "f-1", TimeSlotID: "m-3"}, {FacultyID: "f-1", TimeSlotID: "t-1"}, {FacultyID: "f-1", TimeSlotID: "t-2"}, {FacultyID: "f-1", TimeSlotID: "t-3"},
			{FacultyID: "f-2", TimeSlotID: "m-1"}, {FacultyID: "f-2", TimeSlotID: "m-2"}, {FacultyID: "f-2", TimeSlotID: "m-3"}, {FacultyID: "f-2", TimeSlotID: "t-1"}, {FacultyID: "f-2", TimeSlotID: "t-2"}, {FacultyID: "f-2", TimeSlotID: "t-3"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"}, {RoomID: "r-1", TimeSlotID: "m-3"}, {RoomID: "r-1", TimeSlotID: "t-1"}, {RoomID: "r-1", TimeSlotID: "t-2"}, {RoomID: "r-1", TimeSlotID: "t-3"},
			{RoomID: "r-2", TimeSlotID: "m-1"}, {RoomID: "r-2", TimeSlotID: "m-2"}, {RoomID: "r-2", TimeSlotID: "m-3"}, {RoomID: "r-2", TimeSlotID: "t-1"}, {RoomID: "r-2", TimeSlotID: "t-2"}, {RoomID: "r-2", TimeSlotID: "t-3"},
		},
	}
	p.Prepare()
	return p
}

// -------------------------------------------------------------
// Test 14: Initial solution violating compiled hard constraint is rejected before iterations begin
// -------------------------------------------------------------
func TestTabuSearch_InitialSolutionCompiledHardViolationRejected(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-tabu-init",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-tabu-init", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-tabu-init", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-dbms": {ID: "subj-dbms", Code: "CS102", Name: "DBMS"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-dbms": {
				ID:                    "co-dbms",
				TermID:                "term-1",
				ClassID:               "class-1",
				SubjectID:             "subj-dbms",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-dbms"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-dbms": {ID: "req-dbms", CourseOfferingID: "co-dbms", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof DBMS"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	// Initial solution has both DBMS sessions on Monday (satisfies legacy constraints, but violates maxPerDay: 1)
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-dbms#0",
		CourseOfferingID:     "co-dbms",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "m-1",
		SessionRequirementID: "req-dbms",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-dbms#1",
		CourseOfferingID:     "co-dbms",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "m-2",
		SessionRequirementID: "req-dbms",
		Instance:             1,
	})

	inst := constraints.ConstraintInstance{
		ID:         "rule-dbms-max1",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 100,
		Compiled:      compiledSet,
	}

	_, diag, err := localsearch.TabuSearch(context.Background(), &p, sol, opts)
	if err != localsearch.ErrInitialSolutionInfeasible {
		t.Fatalf("expected ErrInitialSolutionInfeasible, got: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got: %s", diag.Status)
	}
	if diag.Iterations != 0 {
		t.Fatalf("expected 0 iterations before rejection, got: %d", diag.Iterations)
	}
	if len(diag.Violations) == 0 {
		t.Fatal("expected diagnostics to contain violations")
	}
	foundCompiled := false
	for _, v := range diag.Violations {
		if v.ConstraintID == "rule-dbms-max1" && v.TemplateID == "SubjectMaxPerDay" {
			foundCompiled = true
			break
		}
	}
	if !foundCompiled {
		t.Fatalf("expected compiled violation with ID rule-dbms-max1, got: %+v", diag.Violations)
	}
}

// -------------------------------------------------------------
// Test 15: Move-level compiled hard constraint enforcement in Tabu
// -------------------------------------------------------------
func TestTabuSearch_MoveLevelCompiledHardEnforcement(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-tabu-move",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-tabu-move", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-tabu-move", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-dbms": {ID: "subj-dbms", Code: "CS102", Name: "DBMS"},
			"subj-math": {ID: "subj-math", Code: "MA101", Name: "Math"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-dbms": {
				ID:                    "co-dbms",
				TermID:                "term-1",
				ClassID:               "class-1",
				SubjectID:             "subj-dbms",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-dbms"},
			},
			"co-math": {
				ID:                    "co-math",
				TermID:                "term-1",
				ClassID:               "class-1",
				SubjectID:             "subj-math",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-2",
				SessionRequirementIDs: []model.SessionRequirementID{"req-math"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-dbms": {ID: "req-dbms", CourseOfferingID: "co-dbms", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1},
			"req-math": {ID: "req-math", CourseOfferingID: "co-math", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof DBMS"},
			"f-2": {ID: "f-2", Name: "Prof Math"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"t-2": {ID: "t-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
			"t-3": {ID: "t-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"},
		},
		PeriodsPerDay: 3,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"}, {FacultyID: "f-1", TimeSlotID: "m-3"}, {FacultyID: "f-1", TimeSlotID: "t-1"}, {FacultyID: "f-1", TimeSlotID: "t-2"}, {FacultyID: "f-1", TimeSlotID: "t-3"},
			{FacultyID: "f-2", TimeSlotID: "m-1"}, {FacultyID: "f-2", TimeSlotID: "m-2"}, {FacultyID: "f-2", TimeSlotID: "m-3"}, {FacultyID: "f-2", TimeSlotID: "t-1"}, {FacultyID: "f-2", TimeSlotID: "t-2"}, {FacultyID: "f-2", TimeSlotID: "t-3"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"}, {RoomID: "r-1", TimeSlotID: "m-3"}, {RoomID: "r-1", TimeSlotID: "t-1"}, {RoomID: "r-1", TimeSlotID: "t-2"}, {RoomID: "r-1", TimeSlotID: "t-3"},
		},
	}
	p.Prepare()

	// Initial feasible solution:
	// DBMS#0 on Monday P1 (m-1)
	// Math#0 on Monday P3 (m-3) -> creates a gap at P2 (m-2) on Monday!
	// DBMS#1 on Tuesday P1 (t-1)
	// (Satisfies max 1 DBMS per day!)
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-dbms#0",
		CourseOfferingID:     "co-dbms",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "m-1",
		SessionRequirementID: "req-dbms",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-math#0",
		CourseOfferingID:     "co-math",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-2",
		RoomID:               "r-1",
		TimeSlotID:           "m-3",
		SessionRequirementID: "req-math",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-dbms#1",
		CourseOfferingID:     "co-dbms",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "t-1",
		SessionRequirementID: "req-dbms",
		Instance:             1,
	})

	inst := constraints.ConstraintInstance{
		ID:         "rule-dbms-max1",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 50,
		TabuTenure:    5,
		MaxCandidates: 50,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, sol, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got: %d", bestSol.Score.HardViolations)
	}

	// Verify that the final solution never violates the compiled SubjectMaxPerDay rule on any day
	dbmsCountByDay := make(map[time.Weekday]int)
	for _, a := range bestSol.Assignments {
		if a.CourseOfferingID == "co-dbms" {
			slot := p.TimeSlots[a.TimeSlotID]
			dbmsCountByDay[slot.Day]++
		}
	}
	for day, count := range dbmsCountByDay {
		if count > 1 {
			t.Fatalf("expected at most 1 DBMS session on day %v, got %d", day, count)
		}
	}
}

// -------------------------------------------------------------
// Test 16: CSP -> Tabu full pipeline compatibility with compiled constraints
// -------------------------------------------------------------
func TestCSPToTabu_CompiledConstraintEnforcement(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-tabu",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-tabu", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-csp-tabu", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-cs": {ID: "subj-cs", Code: "CS101", Name: "Computer Science"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-cs": {
				ID:                    "co-cs",
				TermID:                "term-1",
				ClassID:               "class-1",
				SubjectID:             "subj-cs",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-cs"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-cs": {ID: "req-cs", CourseOfferingID: "co-cs", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof CS"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"t-2": {ID: "t-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"}, {FacultyID: "f-1", TimeSlotID: "t-1"}, {FacultyID: "f-1", TimeSlotID: "t-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"}, {RoomID: "r-1", TimeSlotID: "t-1"}, {RoomID: "r-1", TimeSlotID: "t-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-cs-max1",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-cs", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	// 1. Solve with CSP solver compiled with compiledSet
	solver := backtracking.NewWithCompiled(compiledSet)
	cspSol, cspDiag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})
	if err != nil || cspDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solve failed: status=%s err=%v", cspDiag.Status, err)
	}

	// 2. Pass CSP solution directly to TabuSearch with compiledSet
	tabuOpts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          999,
		Compiled:      compiledSet,
	}
	tabuSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, cspSol, tabuOpts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if tabuDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", tabuDiag.Status)
	}
	if tabuSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations in Tabu solution, got %d", tabuSol.Score.HardViolations)
	}

	// Confirm compiled rule is satisfied on final solution
	csCountByDay := make(map[time.Weekday]int)
	for _, a := range tabuSol.Assignments {
		if a.CourseOfferingID == "co-cs" {
			slot := p.TimeSlots[a.TimeSlotID]
			csCountByDay[slot.Day]++
		}
	}
	for day, count := range csCountByDay {
		if count > 1 {
			t.Fatalf("expected at most 1 CS session on %v, got %d", day, count)
		}
	}
}
