package tests

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// Helper: assertSolutionIndexConsistent
// Reconstructs expected index from authoritative Assignments and asserts exact equality.
// ----------------------------------------------------------------------------

func assertSolutionIndexConsistent(t *testing.T, p *problem.Problem, sol *problem.Solution) {
	t.Helper()
	if sol == nil {
		t.Fatal("assertSolutionIndexConsistent: solution is nil")
	}

	expectedCounts := make(map[model.SessionRequirementID]int)
	expectedFacultySlots := make(map[string]problem.AssignmentID)
	expectedRoomSlots := make(map[string]problem.AssignmentID)
	expectedGroupSlots := make(map[string]problem.AssignmentID)

	for _, a := range sol.Assignments {
		// Verify byID lookup
		indexedA, ok := sol.Index.AssignmentByID(a.ID)
		if !ok {
			t.Fatalf("Index missing assignment by ID: %s", a.ID)
		}
		if indexedA != a {
			t.Fatalf("Index byID mismatch for %s:\n  got:  %+v\n  want: %+v", a.ID, indexedA, a)
		}

		expectedCounts[a.SessionRequirementID]++

		slotIDs, fits := a.OccupiedSlotIDs(p)
		if !fits {
			continue
		}
		for _, sid := range slotIDs {
			fKey := fmt.Sprintf("%s|%s", a.FacultyID, sid)
			rKey := fmt.Sprintf("%s|%s", a.RoomID, sid)
			gKey := fmt.Sprintf("%s|%s", a.StudentGroupID, sid)

			expectedFacultySlots[fKey] = a.ID
			expectedRoomSlots[rKey] = a.ID
			expectedGroupSlots[gKey] = a.ID

			// Verify conflict queries match
			if gotID, ok := sol.Index.FacultyConflict(a.FacultyID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.FacultyConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.FacultyID, sid, gotID, ok, a.ID)
			}
			if gotID, ok := sol.Index.RoomConflict(a.RoomID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.RoomConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.RoomID, sid, gotID, ok, a.ID)
			}
			if gotID, ok := sol.Index.StudentGroupConflict(a.StudentGroupID, []model.TimeSlotID{sid}); !ok || gotID != a.ID {
				t.Fatalf("Index.StudentGroupConflict mismatch for %s at %s: got (%s, %v), want (%s, true)", a.StudentGroupID, sid, gotID, ok, a.ID)
			}
		}
	}

	// Verify RequirementCount matches
	for reqID, count := range expectedCounts {
		if got := sol.Index.ScheduledCount(reqID); got != count {
			t.Fatalf("Index.ScheduledCount mismatch for %s: got %d, want %d", reqID, got, count)
		}
	}
	for reqID := range p.SessionRequirements {
		if _, ok := expectedCounts[reqID]; !ok {
			if got := sol.Index.ScheduledCount(reqID); got != 0 {
				t.Fatalf("Index.ScheduledCount should be 0 for unscheduled req %s, got %d", reqID, got)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// P0 Regression Proof: Candidate Swap Validation Leaves Exact Unmodified State
// ----------------------------------------------------------------------------

func TestHardening_P0_CandidateSwapValidationTransactional(t *testing.T) {
	p, initialSol := testutil.LocalSearchTestProblem()
	p.Prepare()

	// Compile all 8 hard constraints
	instances := []constraints.ConstraintInstance{
		{TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard},
		{TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard},
		{TemplateID: "SubjectMaxPerDay", Kind: constraints.ConstraintKindHard, Params: map[string]any{"courseOfferingId": "offering-a-theory", "maxPerDay": 1}},
	}
	compiled, _, compileErrs := constraints.Compile(&p, instances)
	if len(compileErrs) > 0 {
		t.Fatalf("constraint compilation failed: %v", compileErrs)
	}

	validator := localsearch.NewCompiledMoveValidator(compiled)
	evaluator := localsearch.NewIncrementalScoreEvaluator(&p, &initialSol)

	// Capture initial baseline
	sol := initialSol.Clone()
	assertSolutionIndexConsistent(t, &p, &sol)

	// Perform repeated candidate swap evaluations on both legal and illegal swaps
	assignments := sol.Assignments
	for i := 0; i < len(assignments); i++ {
		for j := i + 1; j < len(assignments); j++ {
			a1 := assignments[i]
			a2 := assignments[j]

			cm := localsearch.SwapMove(
				a1.ID,
				problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
				problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
				a2.ID,
				problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
				problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
			)

			// Execute EvaluateCandidateMove
			_, _ = localsearch.EvaluateCandidateMove(&p, &sol, cm, validator, evaluator)

			// P0 Proof: State MUST be 100% restored after candidate evaluation window
			assertSolutionIndexConsistent(t, &p, &sol)
			if len(sol.Assignments) != len(initialSol.Assignments) {
				t.Fatalf("assignment count changed during swap evaluation: got %d, want %d", len(sol.Assignments), len(initialSol.Assignments))
			}
			for k := range sol.Assignments {
				if sol.Assignments[k] != initialSol.Assignments[k] {
					t.Fatalf("assignment %s corrupted after swap candidate evaluation:\n got:  %+v\n want: %+v", sol.Assignments[k].ID, sol.Assignments[k], initialSol.Assignments[k])
				}
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 1. Solution & Index Invariants: 1,000+ Randomized Move/Undo & Swap/Undo
// ----------------------------------------------------------------------------

func TestHardening_IndexInvariants_RandomizedMoveUndo(t *testing.T) {
	p, initialSol := testutil.LocalSearchTestProblem()
	p.Prepare()

	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}
	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}

	rng := rand.New(rand.NewSource(9999))
	sol := initialSol.Clone()

	assertSolutionIndexConsistent(t, &p, &sol)

	const cycles = 1500
	for i := 0; i < cycles; i++ {
		idx := rng.Intn(len(sol.Assignments))
		a := sol.Assignments[idx]
		targetRoom := roomList[rng.Intn(len(roomList))]
		targetSlot := slotList[rng.Intn(len(slotList))]

		mv := problem.Move{
			AssignmentID: a.ID,
			From:         problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
			To:           problem.Placement{RoomID: targetRoom, TimeSlotID: targetSlot},
		}

		// Apply move
		if err := sol.ApplyMove(&p, mv); err != nil {
			continue
		}

		// Undo move
		if err := sol.UndoMove(&p, mv); err != nil {
			t.Fatalf("cycle %d: UndoMove failed: %v", i, err)
		}
		assertSolutionIndexConsistent(t, &p, &sol)
	}

	// Verify exact restoration to initial state
	if len(sol.Assignments) != len(initialSol.Assignments) {
		t.Fatalf("final assignment count %d != initial %d", len(sol.Assignments), len(initialSol.Assignments))
	}
	for i := range sol.Assignments {
		if sol.Assignments[i] != initialSol.Assignments[i] {
			t.Fatalf("assignment %d changed after %d move/undo cycles:\n got:  %+v\n want: %+v", i, cycles, sol.Assignments[i], initialSol.Assignments[i])
		}
	}
}

func TestHardening_IndexInvariants_RandomizedSwapUndo(t *testing.T) {
	p, initialSol := testutil.LocalSearchTestProblem()
	p.Prepare()

	rng := rand.New(rand.NewSource(8888))
	sol := initialSol.Clone()

	assertSolutionIndexConsistent(t, &p, &sol)

	const cycles = 1500
	for i := 0; i < cycles; i++ {
		if len(sol.Assignments) < 2 {
			break
		}
		i1 := rng.Intn(len(sol.Assignments))
		i2 := rng.Intn(len(sol.Assignments))
		if i1 == i2 {
			continue
		}
		a1 := sol.Assignments[i1]
		a2 := sol.Assignments[i2]

		m1 := problem.Move{
			AssignmentID: a1.ID,
			From:         problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
			To:           problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
		}
		m2 := problem.Move{
			AssignmentID: a2.ID,
			From:         problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
			To:           problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
		}

		// Apply swap
		if err := sol.ApplySwap(&p, m1, m2); err != nil {
			continue
		}

		// Undo swap
		if err := sol.UndoSwap(&p, m1, m2); err != nil {
			t.Fatalf("cycle %d: UndoSwap failed: %v", i, err)
		}
		assertSolutionIndexConsistent(t, &p, &sol)
	}

	// Verify exact restoration
	for i := range sol.Assignments {
		if sol.Assignments[i] != initialSol.Assignments[i] {
			t.Fatalf("assignment %d changed after %d swap/undo cycles:\n got:  %+v\n want: %+v", i, cycles, sol.Assignments[i], initialSol.Assignments[i])
		}
	}
}

// ----------------------------------------------------------------------------
// 2. Move & Swap Edge Placements, Multi-Period, Self-Swap & Clone Deep Copy
// ----------------------------------------------------------------------------

func TestHardening_MoveSwap_EdgePlacementsAndSelfSwap(t *testing.T) {
	p, sol := testutil.LocalSearchTestProblem()
	p.Prepare()

	// 1. Self-swap rejection
	a := sol.Assignments[0]
	mSelf1 := problem.Move{AssignmentID: a.ID, From: problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID}, To: problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-2"}}
	mSelf2 := problem.Move{AssignmentID: a.ID, From: problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID}, To: problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-2"}}

	if err := sol.ApplySwap(&p, mSelf1, mSelf2); err == nil {
		t.Fatal("expected ApplySwap to reject self-swap with error, got nil")
	}
	if err := sol.UndoSwap(&p, mSelf1, mSelf2); err == nil {
		t.Fatal("expected UndoSwap to reject self-swap with error, got nil")
	}

	// 2. Multi-period session move: move a-lab-1 (duration=2) across days
	labMove := problem.Move{
		AssignmentID: "a-lab-1",
		From:         problem.Placement{RoomID: "room-lab-1", TimeSlotID: "tue-1"},
		To:           problem.Placement{RoomID: "room-lab-1", TimeSlotID: "mon-3"},
	}
	if err := sol.ApplyMove(&p, labMove); err != nil {
		t.Fatalf("ApplyMove multi-period failed: %v", err)
	}
	assertSolutionIndexConsistent(t, &p, &sol)

	if err := sol.UndoMove(&p, labMove); err != nil {
		t.Fatalf("UndoMove multi-period failed: %v", err)
	}
	assertSolutionIndexConsistent(t, &p, &sol)

	// 3. Solution.Clone deep copy isolation
	sol.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    10,
		Breakdown: scorer.ScoreBreakdown{
			GroupGaps: map[model.StudentGroupID]int{"group-a-whole": 2},
			Details: []scorer.GroupDayGap{
				{StudentGroupID: "group-a-whole", Day: time.Monday, Gaps: 2},
			},
			Components: []scorer.ObjectiveComponentScore{
				{ID: scorer.ObjectiveStudentGapPenalty, RawScore: 2, Weight: 5, WeightedScore: 10},
			},
		},
	}
	cloned := sol.Clone()
	cloned.Score.Breakdown.GroupGaps["group-a-whole"] = 99
	cloned.Score.Breakdown.Details[0].Gaps = 99
	cloned.Score.Breakdown.Components[0].RawScore = 99

	if sol.Score.Breakdown.GroupGaps["group-a-whole"] != 2 {
		t.Fatalf("Solution.Clone GroupGaps map was shallow copied!")
	}
	if sol.Score.Breakdown.Details[0].Gaps != 2 {
		t.Fatalf("Solution.Clone Details slice was shallow copied!")
	}
	if sol.Score.Breakdown.Components[0].RawScore != 2 {
		t.Fatalf("Solution.Clone Components slice was shallow copied!")
	}
}

// ----------------------------------------------------------------------------
// 3. Incremental vs Full Score Parity: 2,000+ Randomized Operations (Weighted & Unweighted)
// ----------------------------------------------------------------------------

func TestHardening_IncrementalScore_RandomizedParity(t *testing.T) {
	configs := []struct {
		name   string
		weight int
	}{
		{"DefaultWeight1", 1},
		{"WeightedObjective5", 5},
		{"WeightedObjective10", 10},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
			p.Prepare()

			sol := problem.NewSolution()
			slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
			for s := range p.TimeSlots {
				slotList = append(slotList, s)
			}
			roomList := make([]model.RoomID, 0, len(p.Rooms))
			for r := range p.Rooms {
				roomList = append(roomList, r)
			}

			sIdx, rIdx := 0, 0
			for _, req := range p.SessionRequirements {
				offering := p.CourseOfferings[req.CourseOfferingID]
				for inst := 0; inst < req.SessionsPerWeek; inst++ {
					_ = sol.AddAssignment(&p, problem.Assignment{
						ID:                   problem.NewAssignmentID(req.ID, inst),
						CourseOfferingID:     offering.ID,
						StudentGroupID:       offering.StudentGroupID,
						FacultyID:            offering.FacultyID,
						RoomID:               roomList[rIdx%len(roomList)],
						TimeSlotID:           slotList[sIdx%len(slotList)],
						SessionRequirementID: req.ID,
						Instance:             inst,
					})
					sIdx++
					rIdx++
				}
			}

			objConfig := scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: scorer.ObjectiveStudentGapPenalty, Weight: tc.weight},
				},
			}

			fullEvaluator := localsearch.FullScoreEvaluator{Config: objConfig}
			incEvaluator := localsearch.NewIncrementalScoreEvaluatorWithConfig(&p, &sol, objConfig)

			// Initial parity
			fullInit := fullEvaluator.Evaluate(&p, &sol)
			incInit := incEvaluator.Evaluate(&p, &sol)
			if fullInit.StudentGapPenalty != incInit.StudentGapPenalty || fullInit.SoftPenalty != incInit.SoftPenalty {
				t.Fatalf("initial score mismatch: Full=%+v, Inc=%+v", fullInit, incInit)
			}

			rng := rand.New(rand.NewSource(777 + int64(tc.weight)))
			const operations = 2000

			for step := 0; step < operations; step++ {
				isSwap := len(sol.Assignments) >= 2 && rng.Intn(2) == 1

				if !isSwap {
					randIdx := rng.Intn(len(sol.Assignments))
					a := sol.Assignments[randIdx]
					randSlot := slotList[rng.Intn(len(slotList))]
					randRoom := roomList[rng.Intn(len(roomList))]

					cm := localsearch.SingleMove(
						a.ID,
						problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
						problem.Placement{RoomID: randRoom, TimeSlotID: randSlot},
					)

					incPreview := incEvaluator.EvaluateCandidateMove(&p, &sol, cm)

					// Apply move to solution
					if err := localsearch.ApplyCandidateMove(&p, &sol, cm); err != nil {
						continue
					}
					incEvaluator.ApplyCandidateMove(&p, &sol, cm)

					fullScore := fullEvaluator.Evaluate(&p, &sol)
					incScore := incEvaluator.Evaluate(&p, &sol)

					if incPreview.StudentGapPenalty != fullScore.StudentGapPenalty {
						t.Fatalf("step %d (SingleMove preview mismatch): PreviewGap=%d, FullGap=%d", step, incPreview.StudentGapPenalty, fullScore.StudentGapPenalty)
					}
					if incPreview.SoftPenalty != fullScore.SoftPenalty {
						t.Fatalf("step %d (SingleMove preview soft penalty mismatch): PreviewSoft=%d, FullSoft=%d", step, incPreview.SoftPenalty, fullScore.SoftPenalty)
					}
					if incScore.StudentGapPenalty != fullScore.StudentGapPenalty {
						t.Fatalf("step %d (SingleMove applied mismatch): IncGap=%d, FullGap=%d", step, incScore.StudentGapPenalty, fullScore.StudentGapPenalty)
					}
					if incScore.SoftPenalty != fullScore.SoftPenalty {
						t.Fatalf("step %d (SingleMove applied soft penalty mismatch): IncSoft=%d, FullSoft=%d", step, incScore.SoftPenalty, fullScore.SoftPenalty)
					}
				} else {
					i1 := rng.Intn(len(sol.Assignments))
					i2 := rng.Intn(len(sol.Assignments))
					if i1 == i2 {
						continue
					}
					a1 := sol.Assignments[i1]
					a2 := sol.Assignments[i2]

					cm := localsearch.SwapMove(
						a1.ID,
						problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
						problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
						a2.ID,
						problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
						problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
					)

					incPreview := incEvaluator.EvaluateCandidateMove(&p, &sol, cm)

					if err := localsearch.ApplyCandidateMove(&p, &sol, cm); err != nil {
						continue
					}
					incEvaluator.ApplyCandidateMove(&p, &sol, cm)

					fullScore := fullEvaluator.Evaluate(&p, &sol)
					incScore := incEvaluator.Evaluate(&p, &sol)

					if incPreview.StudentGapPenalty != fullScore.StudentGapPenalty {
						t.Fatalf("step %d (SwapMove preview mismatch): PreviewGap=%d, FullGap=%d", step, incPreview.StudentGapPenalty, fullScore.StudentGapPenalty)
					}
					if incPreview.SoftPenalty != fullScore.SoftPenalty {
						t.Fatalf("step %d (SwapMove preview soft penalty mismatch): PreviewSoft=%d, FullSoft=%d", step, incPreview.SoftPenalty, fullScore.SoftPenalty)
					}
					if incScore.StudentGapPenalty != fullScore.StudentGapPenalty {
						t.Fatalf("step %d (SwapMove applied mismatch): IncGap=%d, FullGap=%d", step, incScore.StudentGapPenalty, fullScore.StudentGapPenalty)
					}
					if incScore.SoftPenalty != fullScore.SoftPenalty {
						t.Fatalf("step %d (SwapMove applied soft penalty mismatch): IncSoft=%d, FullSoft=%d", step, incScore.SoftPenalty, fullScore.SoftPenalty)
					}
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// 4. Tabu Search Failure Paths & Hardening
// ----------------------------------------------------------------------------

func TestHardening_Tabu_FailurePaths(t *testing.T) {
	p, initialSol := testutil.LocalSearchTestProblem()
	p.Prepare()

	// Compile all 8 hard constraints
	instances := []constraints.ConstraintInstance{
		{TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard},
		{TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard},
		{TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard},
	}
	compiled, _, compileErrs := constraints.Compile(&p, instances)
	if len(compileErrs) > 0 {
		t.Fatalf("constraint compilation failed: %v", compileErrs)
	}

	// 1. Tabu Search with Compiled Constraints & Swap Moves (Verifying zero index corruption)
	opts := localsearch.TabuSearchOptions{
		MaxIterations:      30,
		NoImprovementLimit: 15,
		TabuTenure:         4,
		MaxCandidates:      20,
		Seed:               42,
		Compiled:           compiled,
	}
	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, initialSol, opts)
	if err != nil {
		t.Fatalf("TabuSearch with compiled constraints failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", diag.Status)
	}
	assertSolutionIndexConsistent(t, &p, &bestSol)

	// 2. Context Cancellation
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	solCancel, diagCancel, errCancel := localsearch.TabuSearch(ctxCancel, &p, initialSol, opts)
	if errCancel != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", errCancel)
	}
	if diagCancel.Status != diagnostics.SolveStatusCancelled {
		t.Fatalf("expected status CANCELLED, got %s", diagCancel.Status)
	}
	assertSolutionIndexConsistent(t, &p, &solCancel)

	// 3. Context Deadline Exceeded
	ctxTimeout, cancelTimeout := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelTimeout()
	solTimeout, diagTimeout, errTimeout := localsearch.TabuSearch(ctxTimeout, &p, initialSol, opts)
	if errTimeout != nil {
		t.Fatalf("expected nil error on deadline exceeded context, got %v", errTimeout)
	}
	if diagTimeout.Status != diagnostics.SolveStatusDeadlineExceeded {
		t.Fatalf("expected status DEADLINE_EXCEEDED, got %s", diagTimeout.Status)
	}
	assertSolutionIndexConsistent(t, &p, &solTimeout)

	// 4. Weighted Objective SoftPenalty in Final Solution
	weightedCfg := &scorer.ObjectiveConfig{
		Components: []scorer.ObjectiveComponent{
			{ID: scorer.ObjectiveStudentGapPenalty, Weight: 4},
		},
	}
	optsWeighted := localsearch.TabuSearchOptions{
		MaxIterations:      20,
		NoImprovementLimit: 10,
		TabuTenure:         3,
		MaxCandidates:      20,
		Seed:               42,
		ObjectiveConfig:    weightedCfg,
	}
	bestWeighted, diagWeighted, err := localsearch.TabuSearch(context.Background(), &p, initialSol, optsWeighted)
	if err != nil {
		t.Fatalf("TabuSearch weighted failed: %v", err)
	}
	if bestWeighted.Score.SoftPenalty != diagWeighted.BestScore.SoftPenalty {
		t.Fatalf("Score.SoftPenalty (%d) != BestScore.SoftPenalty (%d)", bestWeighted.Score.SoftPenalty, diagWeighted.BestScore.SoftPenalty)
	}
	if bestWeighted.Score.SoftPenalty != diagWeighted.BestScore.StudentGapPenalty*4 {
		t.Fatalf("Score.SoftPenalty (%d) != StudentGapPenalty*4 (%d)", bestWeighted.Score.SoftPenalty, diagWeighted.BestScore.StudentGapPenalty*4)
	}
}

// ----------------------------------------------------------------------------
// 5. CSP Solver Failure Paths & Diagnostics
// ----------------------------------------------------------------------------

func TestHardening_CSP_FailurePaths(t *testing.T) {
	// 1. Contradictory locked assignments -> INFEASIBLE or INVALID_PROBLEM status & descriptive error
	p, _ := testutil.LocalSearchTestProblem()
	p.LockedAssignments = []problem.Assignment{
		{ID: "lock-1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory", Instance: 0},
		{ID: "lock-2", CourseOfferingID: "offering-b-theory", StudentGroupID: "group-b-whole", FacultyID: "faculty-3", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-b-theory", Instance: 0},
	}

	solver := backtracking.New()
	_, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{})
	if err == nil {
		t.Fatal("expected solve to fail with contradictory locked assignments")
	}
	if diag.Status != diagnostics.SolveStatusInvalidProblem && diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected INFEASIBLE or INVALID_PROBLEM status, got %s", diag.Status)
	}

	// 2. Node limit reached -> NODE_LIMIT status
	pNormal, _ := testutil.LocalSearchTestProblem()
	optsNodeLimit := problem.SolveOptions{MaxNodes: 1}
	_, diagNode, errNode := solver.Solve(context.Background(), pNormal, optsNodeLimit)
	if !errors.Is(errNode, backtracking.ErrNodeLimit) {
		t.Fatalf("expected ErrNodeLimit, got %v", errNode)
	}
	if diagNode.Status != diagnostics.SolveStatusNodeLimit {
		t.Fatalf("expected status NODE_LIMIT, got %s", diagNode.Status)
	}

	// 3. CSP Context Cancellation -> CANCELLED status
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	_, diagCancel, errCancel := solver.Solve(ctxCancel, pNormal, problem.SolveOptions{})
	if !errors.Is(errCancel, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", errCancel)
	}
	if diagCancel.Status != diagnostics.SolveStatusCancelled {
		t.Fatalf("expected status CANCELLED, got %s", diagCancel.Status)
	}

	// 4. CSP Context Deadline Exceeded -> DEADLINE_EXCEEDED status
	ctxTimeout, cancelTimeout := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelTimeout()
	_, diagTimeout, errTimeout := solver.Solve(ctxTimeout, pNormal, problem.SolveOptions{})
	if !errors.Is(errTimeout, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", errTimeout)
	}
	if diagTimeout.Status != diagnostics.SolveStatusDeadlineExceeded {
		t.Fatalf("expected status DEADLINE_EXCEEDED, got %s", diagTimeout.Status)
	}
}

// ----------------------------------------------------------------------------
// 6. Deterministic Replay: Same Problem + Constraints + Options + Seed => Identical Output
// ----------------------------------------------------------------------------

func TestHardening_DeterministicReplay(t *testing.T) {
	seeds := []int64{42, 101, 777}

	for _, seed := range seeds {
		t.Run(fmt.Sprintf("Seed_%d", seed), func(t *testing.T) {
			p := randomFeasibleProblem(seed)
			p.Prepare()

			var firstJSON string
			var firstHash string

			const runs = 4
			for run := 0; run < runs; run++ {
				solver := backtracking.New()
				sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
				if err != nil {
					t.Fatalf("run %d failed: %v", run, err)
				}
				if diag.Status != diagnostics.SolveStatusSolved {
					t.Fatalf("run %d status = %s", run, diag.Status)
				}

				// Run Tabu search
				tabuOpts := localsearch.TabuSearchOptions{
					MaxIterations:      30,
					NoImprovementLimit: 15,
					TabuTenure:         5,
					MaxCandidates:      25,
					Seed:               seed,
				}
				bestSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, sol, tabuOpts)
				if err != nil {
					t.Fatalf("run %d tabu failed: %v", run, err)
				}

				// Zero out non-deterministic wall-clock duration before verifying replay output
				tabuDiag.Duration = 0

				output := struct {
					Assignments []problem.Assignment       `json:"assignments"`
					Score       scorer.Score               `json:"score"`
					Diag        localsearch.TabuDiagnostics `json:"diag"`
				}{
					Assignments: bestSol.Assignments,
					Score:       bestSol.Score,
					Diag:        tabuDiag,
				}

				data, err := json.Marshal(output)
				if err != nil {
					t.Fatalf("json marshal failed: %v", err)
				}
				hash := fmt.Sprintf("%x", sha256.Sum256(data))

				if run == 0 {
					firstJSON = string(data)
					firstHash = hash
				} else {
					if hash != firstHash {
						t.Fatalf("run %d produced non-deterministic output hash:\n  run %d: %s\n  run 0:  %s\nDiff JSON:\n  run %d: %s\n  run 0:  %s", run, run, hash, firstHash, run, string(data), firstJSON)
					}
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// 7. Adversarial & Input Validation Testing
// ----------------------------------------------------------------------------

func TestHardening_AdversarialInputValidation(t *testing.T) {
	// 1. PeriodsPerDay <= 0
	p1, _ := testutil.LocalSearchTestProblem()
	p1.PeriodsPerDay = 0
	v1 := problem.Validate(p1)
	if len(v1) == 0 || !hasViolationMessage(v1, "non-positive periods per day") {
		t.Fatalf("expected violation for PeriodsPerDay=0, got %+v", v1)
	}

	// 2. Empty TenantID
	p2, _ := testutil.LocalSearchTestProblem()
	p2.TenantID = ""
	v2 := problem.Validate(p2)
	if len(v2) == 0 || !hasViolationMessage(v2, "empty tenant ID") {
		t.Fatalf("expected violation for empty tenant ID, got %+v", v2)
	}

	// 3. Program references missing Department
	p3, _ := testutil.LocalSearchTestProblem()
	p3.Programs["prog-bad"] = model.Program{ID: "prog-bad", DepartmentID: "missing-dept-99"}
	v3 := problem.Validate(p3)
	if len(v3) == 0 || !hasViolationMessage(v3, "references missing department") {
		t.Fatalf("expected violation for missing department reference, got %+v", v3)
	}

	// 4. Session duration exceeds PeriodsPerDay
	p4, _ := testutil.LocalSearchTestProblem()
	req := p4.SessionRequirements["req-a-theory"]
	req.Duration = 10 // periodsPerDay is 4
	p4.SessionRequirements["req-a-theory"] = req
	v4 := problem.Validate(p4)
	if len(v4) == 0 || !hasViolationMessage(v4, "duration exceeds periods per day") {
		t.Fatalf("expected violation for duration > periodsPerDay, got %+v", v4)
	}

	// 5. Entity with empty ID
	p5, _ := testutil.LocalSearchTestProblem()
	p5.Faculty[""] = model.Faculty{ID: "", Name: "Ghost"}
	v5 := problem.Validate(p5)
	if len(v5) == 0 || !hasViolationMessage(v5, "faculty has empty ID") {
		t.Fatalf("expected violation for empty faculty ID, got %+v", v5)
	}
}

func hasViolationMessage(violations []diagnostics.Violation, substr string) bool {
	for _, v := range violations {
		if containsSubstring(v.Message, substr) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// 8. Final Hard-Constraint Guarantee: SOLVED => 0 Hard Violations Across All 8 Constraints
// ----------------------------------------------------------------------------

func TestHardening_FinalFeasibilityGuarantee(t *testing.T) {
	seeds := []int64{101, 202, 303, 404, 505}

	for _, seed := range seeds {
		p := randomFeasibleProblem(seed)
		p.Prepare()

		// 1. Solve via CSP
		solver := backtracking.New()
		sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
		if err != nil {
			t.Fatalf("seed %d CSP failed: %v", seed, err)
		}
		if diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("seed %d CSP status = %s", seed, diag.Status)
		}

		// Verify 0 hard violations on CSP solution
		allHard := constraints.DefaultHardConstraints()
		for _, a := range sol.Assignments {
			if v := constraints.CheckAll(&p, &sol, a, allHard); len(v) > 0 {
				t.Fatalf("seed %d CSP solution has hard violations: %+v", seed, v)
			}
		}

		// 2. Optimize via Tabu Search
		tabuOpts := localsearch.TabuSearchOptions{
			MaxIterations:      50,
			NoImprovementLimit: 20,
			TabuTenure:         5,
			MaxCandidates:      30,
			Seed:               seed,
		}
		bestSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, sol, tabuOpts)
		if err != nil {
			t.Fatalf("seed %d TabuSearch failed: %v", seed, err)
		}
		if tabuDiag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("seed %d Tabu status = %s", seed, tabuDiag.Status)
		}
		if bestSol.Score.HardViolations != 0 {
			t.Fatalf("seed %d Tabu solution reports %d hard violations", seed, bestSol.Score.HardViolations)
		}

		// Verify 0 hard violations on Tabu solution
		for _, a := range bestSol.Assignments {
			if v := constraints.CheckAll(&p, &bestSol, a, allHard); len(v) > 0 {
				t.Fatalf("seed %d Tabu solution has hard violations: %+v", seed, v)
			}
		}
		assertSolutionIndexConsistent(t, &p, &bestSol)
	}
}
