package problem_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
)

// =============================================================================
// INVARIANT 1: Add then Remove restores exact prior state
// =============================================================================

func TestInvariant_AddRemove_RestoresExactState(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := problem.NewSolution()

	sortedSlots := sortedTimeSlotIDsForTest(&p)
	sortedRooms := sortedRoomIDsForTest(&p)
	reqs := sortedSessionRequirementsForTest(&p)

	assignIdx := 0
	for _, req := range reqs {
		offering := p.CourseOfferings[req.CourseOfferingID]
		for inst := 0; inst < req.SessionsPerWeek; inst++ {
			slot := sortedSlots[assignIdx%len(sortedSlots)]
			room := sortedRooms[assignIdx%len(sortedRooms)]
			a := problem.Assignment{
				ID:                   problem.NewAssignmentID(req.ID, inst),
				CourseOfferingID:     offering.ID,
				StudentGroupID:       offering.StudentGroupID,
				FacultyID:            offering.FacultyID,
				RoomID:               room,
				TimeSlotID:           slot,
				SessionRequirementID: req.ID,
				Instance:             inst,
			}
			snapshotAssignments := cloneAssignments(sol.Assignments)
			snapshotFaculty := copyResourceMap(sol.Index.FacultySlot)
			snapshotRoom := copyResourceMap(sol.Index.RoomSlot)
			snapshotGroup := copyResourceMap(sol.Index.StudentGroupSlot)
			snapshotReqCount := copyReqCountMap(sol.Index.RequirementCount)

			addErr := sol.AddAssignment(&p, a)
			if addErr != nil {
				// If add failed, state should be unchanged
				if len(sol.Assignments) != len(snapshotAssignments) {
					t.Fatalf("Add/Remove: add failed but assignments changed at idx %d", assignIdx)
				}
				if !reflect.DeepEqual(sol.Index.RequirementCount, snapshotReqCount) {
					t.Fatalf("Add/Remove: add failed but RequirementCount changed at idx %d", assignIdx)
				}
				assignIdx++
				continue
			}

			sol.RemoveLastAssignment(&p)

			if len(sol.Assignments) != len(snapshotAssignments) {
				t.Fatalf("Add/Remove: count mismatch at idx %d", assignIdx)
			}
			for i := range sol.Assignments {
				if sol.Assignments[i] != snapshotAssignments[i] {
					t.Fatalf("Add/Remove: assignment %d changed at idx %d", i, assignIdx)
				}
			}
			if !reflect.DeepEqual(sol.Index.FacultySlot, snapshotFaculty) {
				t.Fatalf("Add/Remove: FacultySlot mismatch at idx %d", assignIdx)
			}
			if !reflect.DeepEqual(sol.Index.RoomSlot, snapshotRoom) {
				t.Fatalf("Add/Remove: RoomSlot mismatch at idx %d", assignIdx)
			}
			if !reflect.DeepEqual(sol.Index.StudentGroupSlot, snapshotGroup) {
				t.Fatalf("Add/Remove: StudentGroupSlot mismatch at idx %d", assignIdx)
			}
		if !requirementCountEqual(sol.Index.RequirementCount, snapshotReqCount) {
			t.Fatalf("Add/Remove: RequirementCount mismatch at idx %d: before=%v after=%v", assignIdx, snapshotReqCount, sol.Index.RequirementCount)
		}
			assignIdx++
		}
	}
}

// =============================================================================
// INVARIANT 2: ApplyMove then Undo restores exact prior state
// =============================================================================

func TestInvariant_ApplyMoveUndo_RestoresExactState(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := buildInvariantTestSolution(t, &p)

	snapshotAssignments := cloneAssignments(sol.Assignments)
	snapshotFaculty := copyResourceMap(sol.Index.FacultySlot)
	snapshotRoom := copyResourceMap(sol.Index.RoomSlot)
	snapshotGroup := copyResourceMap(sol.Index.StudentGroupSlot)
	snapshotReqCount := copyReqCountMap(sol.Index.RequirementCount)

	var target problem.Assignment
	for _, a := range sol.Assignments {
		if !p.IsLocked(a.ID) {
			target = a
			break
		}
	}
	if target.ID == "" {
		t.Skip("No unlocked assignments")
	}

	sortedSlots := sortedTimeSlotIDsForTest(&p)
	sortedRooms := sortedRoomIDsForTest(&p)
	targetRoom := sortedRooms[0]
	targetSlot := sortedSlots[0]
	if target.RoomID == targetRoom && target.TimeSlotID == targetSlot {
		targetSlot = sortedSlots[1]
	}

	mv := problem.Move{
		AssignmentID: target.ID,
		From:         problem.Placement{RoomID: target.RoomID, TimeSlotID: target.TimeSlotID},
		To:           problem.Placement{RoomID: targetRoom, TimeSlotID: targetSlot},
	}

	if err := sol.ApplyMove(&p, mv); err != nil {
		t.Fatalf("ApplyMove failed: %v", err)
	}
	if err := sol.UndoMove(&p, mv); err != nil {
		t.Fatalf("UndoMove failed: %v", err)
	}

	if len(sol.Assignments) != len(snapshotAssignments) {
		t.Fatalf("Move/Undo: count mismatch")
	}
	for i := range sol.Assignments {
		if sol.Assignments[i] != snapshotAssignments[i] {
			t.Fatalf("Move/Undo: assignment %d changed", i)
		}
	}
	if !reflect.DeepEqual(sol.Index.FacultySlot, snapshotFaculty) {
		t.Fatalf("Move/Undo: FacultySlot mismatch")
	}
	if !reflect.DeepEqual(sol.Index.RoomSlot, snapshotRoom) {
		t.Fatalf("Move/Undo: RoomSlot mismatch")
	}
	if !reflect.DeepEqual(sol.Index.StudentGroupSlot, snapshotGroup) {
		t.Fatalf("Move/Undo: StudentGroupSlot mismatch")
	}
	if !requirementCountEqual(sol.Index.RequirementCount, snapshotReqCount) {
		t.Fatalf("Move/Undo: RequirementCount mismatch")
	}
}

// =============================================================================
// INVARIANT 3: ApplySwap then Undo restores exact prior state
// =============================================================================

func TestInvariant_ApplySwapUndo_RestoresExactState(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := buildInvariantTestSolution(t, &p)

	snapshotAssignments := cloneAssignments(sol.Assignments)
	snapshotFaculty := copyResourceMap(sol.Index.FacultySlot)
	snapshotRoom := copyResourceMap(sol.Index.RoomSlot)
	snapshotGroup := copyResourceMap(sol.Index.StudentGroupSlot)
	snapshotReqCount := copyReqCountMap(sol.Index.RequirementCount)

	var a1, a2 problem.Assignment
	for _, a := range sol.Assignments {
		if !p.IsLocked(a.ID) {
			if a1.ID == "" {
				a1 = a
			} else if a2.ID == "" {
				a2 = a
				break
			}
		}
	}
	if a1.ID == "" || a2.ID == "" {
		t.Skip("Not enough unlocked assignments")
	}

	mv1 := problem.Move{AssignmentID: a1.ID, From: problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID}, To: problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID}}
	mv2 := problem.Move{AssignmentID: a2.ID, From: problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID}, To: problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID}}

	if err := sol.ApplySwap(&p, mv1, mv2); err != nil {
		t.Fatalf("ApplySwap failed: %v", err)
	}
	if err := sol.UndoSwap(&p, mv1, mv2); err != nil {
		t.Fatalf("UndoSwap failed: %v", err)
	}

	if len(sol.Assignments) != len(snapshotAssignments) {
		t.Fatalf("Swap/Undo: count mismatch")
	}
	for i := range sol.Assignments {
		if sol.Assignments[i] != snapshotAssignments[i] {
			t.Fatalf("Swap/Undo: assignment %d changed", i)
		}
	}
	if !reflect.DeepEqual(sol.Index.FacultySlot, snapshotFaculty) {
		t.Fatalf("Swap/Undo: FacultySlot mismatch")
	}
	if !reflect.DeepEqual(sol.Index.RoomSlot, snapshotRoom) {
		t.Fatalf("Swap/Undo: RoomSlot mismatch")
	}
	if !reflect.DeepEqual(sol.Index.StudentGroupSlot, snapshotGroup) {
		t.Fatalf("Swap/Undo: StudentGroupSlot mismatch")
	}
	if !requirementCountEqual(sol.Index.RequirementCount, snapshotReqCount) {
		t.Fatalf("Swap/Undo: RequirementCount mismatch")
	}
}

// =============================================================================
// INVARIANT 4: SolutionIndex matches actual assignments
// =============================================================================

func TestInvariant_SolutionIndex_MatchesAssignments(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := buildInvariantTestSolution(t, &p)
	testutil.AssertSolutionIndexConsistent(t, &p, &sol)
}

// =============================================================================
// INVARIANT 5: RequirementCount correctness
// =============================================================================

func TestInvariant_RequirementCount_FullLifecycle(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := problem.NewSolution()
	sortedSlots := sortedTimeSlotIDsForTest(&p)
	reqs := sortedSessionRequirementsForTest(&p)

	// Add all assignments, tracking which actually succeed
	added := make(map[model.SessionRequirementID]int)
	for _, req := range reqs {
		offering := p.CourseOfferings[req.CourseOfferingID]
		for inst := 0; inst < req.SessionsPerWeek; inst++ {
			a := problem.Assignment{
				ID:                   problem.NewAssignmentID(req.ID, inst),
				CourseOfferingID:     offering.ID,
				StudentGroupID:       offering.StudentGroupID,
				FacultyID:            offering.FacultyID,
				RoomID:               sortedRoomIDsForTest(&p)[0],
				TimeSlotID:           sortedSlots[inst%len(sortedSlots)],
				SessionRequirementID: req.ID,
				Instance:             inst,
			}
			if err := sol.AddAssignment(&p, a); err == nil {
				added[req.ID]++
			}
		}
	}

	// Verify counts match what was actually added
	for reqID, expectedCount := range added {
		if got := sol.Index.ScheduledCount(reqID); got != expectedCount {
			t.Fatalf("Req %s: expected %d, got %d", reqID, expectedCount, got)
		}
	}

	// Remove all and verify counts go to 0
	for len(sol.Assignments) > 0 {
		sol.RemoveLastAssignment(&p)
	}
	for reqID := range added {
		if got := sol.Index.ScheduledCount(reqID); got != 0 {
			t.Fatalf("After remove: req %s should be 0, got %d", reqID, got)
		}
	}
}

// =============================================================================
// INVARIANT 6: SOLVED passes verification
// =============================================================================

func TestInvariant_SolvedResult_PassesVerification(t *testing.T) {
	problems := []struct {
		name string
		p    problem.Problem
	}{
		{"small", testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())},
		{"feasible", testutil.FeasibleProblem()},
		{"overlap", testutil.OverlapProblem()},
	}
	for _, tc := range problems {
		t.Run(tc.name, func(t *testing.T) {
			tc.p.Prepare()
			resp, err := engine.Solve(context.Background(), engine.Request{
				Problem:         tc.p,
				SolveOptions:    problem.SolveOptions{SearchMode: problem.SearchModeHeuristic},
				DisableOptimize: true,
			})
			if err != nil {
				t.Fatalf("engine.Solve failed: %v", err)
			}
			if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
				t.Fatalf("expected SOLVED, got %s", resp.Diagnostics.Status)
			}
			report, vErr := verifier.VerifySolution(&tc.p, &resp.Solution, verifier.VerifyOptions{})
			if vErr != nil {
				t.Fatalf("verification failed: %v", vErr)
			}
			if !report.Valid || report.Status != diagnostics.SolveStatusSolved {
				t.Fatalf("verification: %s - %s", report.Status, report.Message)
			}
		})
	}
}

// =============================================================================
// INVARIANT 7: Deterministic replay
// =============================================================================

func TestInvariant_DeterministicReplay(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	for seed := int64(1); seed <= 5; seed++ {
		makeReq := func() engine.Request {
			return engine.Request{
				Problem:      p,
				SolveOptions: problem.SolveOptions{SearchMode: problem.SearchModeHeuristic},
				TabuOptions:  localsearch.TabuSearchOptions{MaxIterations: 20, Seed: seed},
			}
		}
		resp1, _ := engine.Solve(context.Background(), makeReq())
		resp2, _ := engine.Solve(context.Background(), makeReq())
		if resp1.Diagnostics.Status != diagnostics.SolveStatusSolved ||
			resp2.Diagnostics.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("seed=%d: not both SOLVED", seed)
		}
		if len(resp1.Solution.Assignments) != len(resp2.Solution.Assignments) {
			t.Fatalf("seed=%d: count differs", seed)
		}
		for i := range resp1.Solution.Assignments {
			if resp1.Solution.Assignments[i] != resp2.Solution.Assignments[i] {
				t.Fatalf("seed=%d: assignment %d differs", seed, i)
			}
		}
	}
}

// =============================================================================
// INVARIANT 8: Domain wipeout detected
// =============================================================================

func TestInvariant_DomainWipeout_Detected(t *testing.T) {
	p := problem.Problem{
		TenantID: "wp", Term: model.Term{ID: "wp-term", TenantID: "wp", Name: "WP"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "wp", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 30}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "wp-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1},
		},
		Faculty:      map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:         map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 60}},
		RoomFeatures:  map[model.RoomFeatureID]model.RoomFeature{},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
			"t2": {ID: "t2", Day: 1, Period: 2, Label: "P2"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}, {FacultyID: "f1", TimeSlotID: "t2"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}, {RoomID: "rm1", TimeSlotID: "t2"}},
		PeriodsPerDay:         2,
	}

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 1000})
	if !errors.Is(err, backtracking.ErrNoSolution) && !errors.Is(err, backtracking.ErrNodeLimit) {
		t.Fatalf("expected ErrNoSolution or ErrNodeLimit, got: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusInfeasible && diag.Status != diagnostics.SolveStatusNodeLimit {
		t.Fatalf("expected INFEASIBLE or NODE_LIMIT, got %s", diag.Status)
	}
}

// =============================================================================
// INVARIANT 9: Move after Swap preserves index
// =============================================================================

func TestInvariant_MoveAfterSwap_IndexIntegrity(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	sol := buildInvariantTestSolution(t, &p)

	var unlocked []problem.Assignment
	for _, a := range sol.Assignments {
		if !p.IsLocked(a.ID) {
			unlocked = append(unlocked, a)
		}
	}
	if len(unlocked) < 2 {
		t.Skip("Not enough unlocked")
	}

	a1, a2 := unlocked[0], unlocked[1]
	mv1 := problem.Move{AssignmentID: a1.ID, From: problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID}, To: problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID}}
	mv2 := problem.Move{AssignmentID: a2.ID, From: problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID}, To: problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID}}
	_ = sol.ApplySwap(&p, mv1, mv2)
	_ = sol.UndoSwap(&p, mv1, mv2)

	if len(unlocked) > 2 {
		a3 := unlocked[2]
		slots := sortedTimeSlotIDsForTest(&p)
		targetSlot := slots[0]
		if a3.TimeSlotID == targetSlot {
			targetSlot = slots[1]
		}
		mv := problem.Move{AssignmentID: a3.ID, From: problem.Placement{RoomID: a3.RoomID, TimeSlotID: a3.TimeSlotID}, To: problem.Placement{RoomID: a3.RoomID, TimeSlotID: targetSlot}}
		_ = sol.ApplyMove(&p, mv)
		_ = sol.UndoMove(&p, mv)
	}

	testutil.AssertSolutionIndexConsistent(t, &p, &sol)
}

// =============================================================================
// INVARIANT 10: Tabu preserves hard constraints
// =============================================================================

func TestInvariant_TabuSearch_HardConstraintsPreserved(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	instances := []constraints.ConstraintInstance{
		{TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard},
		{TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard},
	}
	compiled, _, errs := constraints.Compile(&p, instances)
	if len(errs) > 0 {
		t.Fatalf("compile: %v", errs)
	}

	solver := backtracking.NewWithCompiled(compiled)
	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000})
	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP: %v / %s", err, diag.Status)
	}

	bestSol, tabuDiag, tabuErr := localsearch.TabuSearch(context.Background(), &p, sol, localsearch.TabuSearchOptions{
		MaxIterations: 50, NoImprovementLimit: 20, TabuTenure: 5, MaxCandidates: 30, Seed: 42, Compiled: compiled,
	})
	if tabuErr != nil && tabuDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("Tabu: %v / %s", tabuErr, tabuDiag.Status)
	}

	searchCtx := constraints.NewSearchCtx(&p)
	for _, c := range compiled.Hard {
		if v := c.Evaluate(searchCtx, &bestSol); len(v) > 0 {
			t.Fatalf("Tabu violates %s: %d", c.ID(), len(v))
		}
	}
}

// =============================================================================
// Helpers
// =============================================================================

func buildInvariantTestSolution(t *testing.T, p *problem.Problem) problem.Solution {
	t.Helper()
	solver := backtracking.New()
	sol, diag, err := solver.Solve(context.Background(), *p, problem.SolveOptions{MaxNodes: 100000})
	if err != nil {
		t.Fatalf("CSP failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("not SOLVED: %s", diag.Status)
	}
	return sol
}

func cloneAssignments(a []problem.Assignment) []problem.Assignment {
	clone := make([]problem.Assignment, len(a))
	copy(clone, a)
	return clone
}

func copyResourceMap[K comparable, V comparable](m map[K]V) map[K]V {
	copied := make(map[K]V, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

func copyReqCountMap(m map[model.SessionRequirementID]int) map[model.SessionRequirementID]int {
	copied := make(map[model.SessionRequirementID]int, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

func sortedTimeSlotIDsForTest(p *problem.Problem) []model.TimeSlotID {
	ids := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for id := range p.TimeSlots {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

// =============================================================================
// PRE-SOLVER: detects obviously impossible problems fast
// =============================================================================

func TestPreSolve_ZeroDomain_NoRoomForCapacity(t *testing.T) {
	p := problem.Problem{
		TenantID: "pre", Term: model.Term{ID: "pre-term", TenantID: "pre", Name: "Pre"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "pre", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 200}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "pre-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:   map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 50}}, // too small
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
			"t2": {ID: "t2", Day: 1, Period: 2, Label: "P2"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}, {FacultyID: "f1", TimeSlotID: "t2"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}, {RoomID: "rm1", TimeSlotID: "t2"}},
		PeriodsPerDay:         2,
	}
	v := problem.PreSolve(&p)
	if len(v) == 0 {
		t.Fatal("expected pre-solve violations for undersized room")
	}
	found := false
	for _, vi := range v {
		if vi.ConstraintName == "PreSolveAnalysis" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected PreSolveAnalysis violation")
	}
}

func TestPreSolve_FacultyOverload(t *testing.T) {
	p := problem.Problem{
		TenantID: "pre", Term: model.Term{ID: "pre-term", TenantID: "pre", Name: "Pre"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "pre", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 30}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "pre-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1", "r2", "r3"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1},
			"r2": {ID: "r2", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1},
			"r3": {ID: "r3", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 3, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:   map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 60}},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
			"t2": {ID: "t2", Day: 1, Period: 2, Label: "P2"},
		},
		// Faculty only available for 2 slots, but needs 9 sessions
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}, {FacultyID: "f1", TimeSlotID: "t2"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}, {RoomID: "rm1", TimeSlotID: "t2"}},
		PeriodsPerDay:         2,
	}
	v := problem.PreSolve(&p)
	if len(v) == 0 {
		t.Fatal("expected pre-solve violations for faculty overload")
	}
	foundOverload := false
	for _, vi := range v {
		if vi.ConstraintName == "PreSolveAnalysis" {
			foundOverload = true
		}
	}
	if !foundOverload {
		t.Fatal("expected PreSolveAnalysis violation for faculty overload")
	}
}

func TestPreSolve_FeasibleProblem_NoViolations(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()
	v := problem.PreSolve(&p)
	if len(v) > 0 {
		t.Fatalf("expected no pre-solve violations for feasible problem, got %d", len(v))
	}
}

// =============================================================================
// Helpers
// =============================================================================

func TestProductionHarden_EmptyProblem_SafeHandling(t *testing.T) {
	p := problem.Problem{
		TenantID: "empty", Term: model.Term{ID: "empty-term", TenantID: "empty", Name: "E"},
		Departments:         make(map[model.DepartmentID]model.Department),
		Programs:            make(map[model.ProgramID]model.Program),
		Classes:             make(map[model.ClassID]model.Class),
		StudentGroups:       make(map[model.StudentGroupID]model.StudentGroup),
		Subjects:            make(map[model.SubjectID]model.Subject),
		CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
		SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		Faculty:             make(map[model.FacultyID]model.Faculty),
		Rooms:               make(map[model.RoomID]model.Room),
		TimeSlots:           make(map[model.TimeSlotID]model.TimeSlot),
	}
	// Should not panic and should return valid status
	resp, err := engine.Solve(context.Background(), engine.Request{
		Problem:         p,
		DisableOptimize: true,
	})
	// Empty problem is either invalid or has no assignments to schedule
	if resp.Diagnostics.Status != diagnostics.SolveStatusInvalidProblem &&
		resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("empty problem: unexpected status %s (err=%v)", resp.Diagnostics.Status, err)
	}
}

func TestProductionHarden_SingleAssignment_MinimalProblem(t *testing.T) {
	p := problem.Problem{
		TenantID: "min", Term: model.Term{ID: "min-term", TenantID: "min", Name: "M"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "min", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 10}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "min-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:   map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 100}},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}},
		PeriodsPerDay:         6,
	}
	resp, err := engine.Solve(context.Background(), engine.Request{
		Problem:         p,
		DisableOptimize: true,
	})
	if err != nil {
		t.Fatalf("single assignment solve failed: %v (status=%s)", err, resp.Diagnostics.Status)
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED for minimal problem, got %s", resp.Diagnostics.Status)
	}
	if len(resp.Solution.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(resp.Solution.Assignments))
	}
}

func TestProductionHarden_NilContext_DefaultBackground(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	// Pass nil context — engine should handle gracefully
	resp, err := engine.Solve(nil, engine.Request{
		Problem:         p,
		DisableOptimize: true,
	})
	if err != nil {
		t.Fatalf("nil context solve failed: %v", err)
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED with nil context, got %s", resp.Diagnostics.Status)
	}
}

func TestProductionHarden_LargeStudentGroup_ExactCapacity(t *testing.T) {
	// Room capacity exactly equals group size — edge case for room capacity constraint
	p := problem.Problem{
		TenantID: "cap", Term: model.Term{ID: "cap-term", TenantID: "cap", Name: "C"},
		Departments: map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "cap", Name: "D"}},
		Programs:    map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "P"}},
		Classes:     map[model.ClassID]model.Class{"c1": {ID: "c1", ProgramID: "p1", Name: "C", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}}},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{"g1": {ID: "g1", ClassID: "c1", Name: "G", Size: 50}},
		Subjects:      map[model.SubjectID]model.Subject{"s1": {ID: "s1", Code: "S", Name: "S"}},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"o1": {ID: "o1", TermID: "cap-term", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1", SessionRequirementIDs: []model.SessionRequirementID{"r1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"r1": {ID: "r1", CourseOfferingID: "o1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{"f1": {ID: "f1", Name: "F"}},
		Rooms:   map[model.RoomID]model.Room{"rm1": {ID: "rm1", Name: "R", Capacity: 50}}, // exactly matches group
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"t1": {ID: "t1", Day: 1, Period: 1, Label: "P1"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{{FacultyID: "f1", TimeSlotID: "t1"}},
		RoomAvailabilities:    []model.RoomAvailability{{RoomID: "rm1", TimeSlotID: "t1"}},
		PeriodsPerDay:         6,
	}
	resp, err := engine.Solve(context.Background(), engine.Request{
		Problem:         p,
		DisableOptimize: true,
	})
	if err != nil {
		t.Fatalf("exact capacity solve failed: %v", err)
	}
	if resp.Diagnostics.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED for exact capacity, got %s", resp.Diagnostics.Status)
	}
}

func sortedRoomIDsForTest(p *problem.Problem) []model.RoomID {
	ids := make([]model.RoomID, 0, len(p.Rooms))
	for id := range p.Rooms {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func sortedSessionRequirementsForTest(p *problem.Problem) []model.SessionRequirement {
	ids := make([]model.SessionRequirementID, 0, len(p.SessionRequirements))
	for id := range p.SessionRequirements {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	reqs := make([]model.SessionRequirement, 0, len(ids))
	for _, id := range ids {
		reqs = append(reqs, p.SessionRequirements[id])
	}
	return reqs
}

// requirementCountEqual compares two RequirementCount maps semantically,
// ignoring entries that are 0 in one map but absent in the other.
func requirementCountEqual(a, b map[model.SessionRequirementID]int) bool {
	// Check that all non-zero entries in a exist in b with same value
	for k, v := range a {
		if v != 0 {
			if bv, ok := b[k]; !ok || bv != v {
				return false
			}
		}
	}
	// Check that all non-zero entries in b exist in a with same value
	for k, v := range b {
		if v != 0 {
			if bv, ok := a[k]; !ok || bv != v {
				return false
			}
		}
	}
	return true
}
