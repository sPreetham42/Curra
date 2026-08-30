package tests

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// Red-Team Attack 1: 1,000+ Randomized Moves and Swaps Parity Audit
func TestRedTeam_1000RandomizedMutationsParity(t *testing.T) {
	seeds := []int64{42, 12345, 99999, 777777}

	for _, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())

		slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
		for s := range p.TimeSlots {
			slotList = append(slotList, s)
		}
		roomList := make([]model.RoomID, 0, len(p.Rooms))
		for r := range p.Rooms {
			roomList = append(roomList, r)
		}
		facList := make([]model.FacultyID, 0, len(p.Faculty))
		for f := range p.Faculty {
			facList = append(facList, f)
		}

		// Inject random preferences
		p.FacultyPreferences = nil
		for _, f := range facList {
			prefCount := rng.Intn(4)
			for k := 0; k < prefCount; k++ {
				randSlot := slotList[rng.Intn(len(slotList))]
				randWeight := rng.Intn(15) + 1
				p.FacultyPreferences = append(p.FacultyPreferences, model.FacultyPreference{
					FacultyID:  f,
					TimeSlotID: randSlot,
					Weight:     randWeight,
				})
			}
		}

		// Initial solution
		sol := problem.NewSolution()
		slotIdx, roomIdx := 0, 0
		for _, req := range p.SessionRequirements {
			offering := p.CourseOfferings[req.CourseOfferingID]
			for inst := 0; inst < req.SessionsPerWeek; inst++ {
				_ = sol.AddAssignment(&p, problem.Assignment{
					ID:                   problem.NewAssignmentID(req.ID, inst),
					CourseOfferingID:     offering.ID,
					StudentGroupID:       offering.StudentGroupID,
					FacultyID:            offering.FacultyID,
					RoomID:               roomList[roomIdx%len(roomList)],
					TimeSlotID:           slotList[slotIdx%len(slotList)],
					SessionRequirementID: req.ID,
					Instance:             inst,
				})
				slotIdx++
				roomIdx++
			}
		}

		fullEval := localsearch.FullScoreEvaluator{}
		incEval := localsearch.NewIncrementalScoreEvaluator(&p, &sol)

		currentSol := sol.Clone()

		for i := 0; i < 1000; i++ {
			isSwap := rng.Float64() < 0.3
			var cm localsearch.CandidateMove

			if !isSwap {
				randIdx := rng.Intn(len(currentSol.Assignments))
				a := currentSol.Assignments[randIdx]
				if p.IsLocked(a.ID) {
					continue
				}
				randSlot := slotList[rng.Intn(len(slotList))]
				randRoom := roomList[rng.Intn(len(roomList))]
				cm = localsearch.CandidateMove{
					Kind:        localsearch.MoveKindSingle,
					Assignment1: a.ID,
					From1:       problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
					To1:         problem.Placement{RoomID: randRoom, TimeSlotID: randSlot},
				}
			} else {
				randIdx1 := rng.Intn(len(currentSol.Assignments))
				randIdx2 := rng.Intn(len(currentSol.Assignments))
				if randIdx1 == randIdx2 {
					randIdx2 = (randIdx1 + 1) % len(currentSol.Assignments)
				}
				a1 := currentSol.Assignments[randIdx1]
				a2 := currentSol.Assignments[randIdx2]
				if p.IsLocked(a1.ID) || p.IsLocked(a2.ID) {
					continue
				}
				cm = localsearch.CandidateMove{
					Kind:        localsearch.MoveKindSwap,
					Assignment1: a1.ID,
					From1:       problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
					To1:         problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
					Assignment2: a2.ID,
					From2:       problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
					To2:         problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
				}
			}

			// Evaluate candidate move
			evalRes := incEval.EvaluateCandidateMove(&p, &currentSol, cm)

			// Apply move to temporary solution for full recomputation
			var errApply error
			tempSol := currentSol.Clone()
			if cm.Kind == localsearch.MoveKindSingle {
				errApply = tempSol.ApplyMove(&p, problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1})
			} else {
				errApply = tempSol.ApplySwap(&p, problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1}, problem.Move{AssignmentID: cm.Assignment2, From: cm.From2, To: cm.To2})
			}
			if errApply != nil {
				continue
			}

			fullBreakdown := fullEval.Evaluate(&p, &tempSol)

			// Parity checks
			if evalRes.SoftPenalty != fullBreakdown.SoftPenalty {
				t.Fatalf("[Seed %d, Iter %d] SoftPenalty mismatch: inc=%d, full=%d", seed, i, evalRes.SoftPenalty, fullBreakdown.SoftPenalty)
			}
			if evalRes.FacultyPreferencePenalty != fullBreakdown.FacultyPreferencePenalty {
				t.Fatalf("[Seed %d, Iter %d] FacultyPreferencePenalty mismatch: inc=%d, full=%d", seed, i, evalRes.FacultyPreferencePenalty, fullBreakdown.FacultyPreferencePenalty)
			}
			if evalRes.StudentGapPenalty != fullBreakdown.StudentGapPenalty {
				t.Fatalf("[Seed %d, Iter %d] StudentGapPenalty mismatch: inc=%d, full=%d", seed, i, evalRes.StudentGapPenalty, fullBreakdown.StudentGapPenalty)
			}
			if evalRes.RoomChangePenalty != fullBreakdown.RoomChangePenalty {
				t.Fatalf("[Seed %d, Iter %d] RoomChangePenalty mismatch: inc=%d, full=%d", seed, i, evalRes.RoomChangePenalty, fullBreakdown.RoomChangePenalty)
			}

			// Actually apply move to state
			var realApplyErr error
			if cm.Kind == localsearch.MoveKindSingle {
				realApplyErr = currentSol.ApplyMove(&p, problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1})
			} else {
				realApplyErr = currentSol.ApplySwap(&p, problem.Move{AssignmentID: cm.Assignment1, From: cm.From1, To: cm.To1}, problem.Move{AssignmentID: cm.Assignment2, From: cm.From2, To: cm.To2})
			}
			if realApplyErr != nil {
				t.Fatalf("[Seed %d, Iter %d] realApplyErr: %v", seed, i, realApplyErr)
			}
			incEval.ApplyCandidateMove(&p, &currentSol, cm)

			// Rebuild check every 100 iterations
			if i%100 == 99 {
				rebuiltEval := localsearch.NewIncrementalScoreEvaluator(&p, &currentSol)
				rebuiltBreakdown := rebuiltEval.Evaluate(&p, &currentSol)
				incBreakdown := incEval.Evaluate(&p, &currentSol)
				if !reflect.DeepEqual(rebuiltBreakdown, incBreakdown) {
					t.Fatalf("[Seed %d, Iter %d] Rebuild drift! Rebuilt=%+v, Inc=%+v", seed, i, rebuiltBreakdown, incBreakdown)
				}
			}
		}
	}
}

// Red-Team Attack 2: Objective Config Matrix Audit (Configurations 1 to 6)
func TestRedTeam_ObjectiveConfigMatrix(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	var sampleFac model.FacultyID
	for f := range p.Faculty {
		sampleFac = f
		break
	}
	var sampleSlot model.TimeSlotID
	for s := range p.TimeSlots {
		sampleSlot = s
		break
	}
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: sampleFac, TimeSlotID: sampleSlot, Weight: 10},
	}
	p.Prepare()

	// Solve CSP to get a fully valid solution
	cspSolver := backtracking.New()
	sol, _, err := cspSolver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeBasic})
	if err != nil {
		t.Fatalf("CSP solve failed for test setup: %v", err)
	}

	configs := []struct {
		name                 string
		cfg                  scorer.ObjectiveConfig
		expectedComponentIDs []scorer.ObjectiveID
	}{
		{
			name:                 "Config 1: Default/Empty Config",
			cfg:                  scorer.ObjectiveConfig{},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveStudentGapPenalty, scorer.ObjectiveFacultyPreference, scorer.ObjectiveRoomChange},
		},
		{
			name: "Config 2: Only StudentGapPenalty configured",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{{ID: scorer.ObjectiveStudentGapPenalty, Weight: 2}},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveStudentGapPenalty},
		},
		{
			name: "Config 3: Only FacultyPreference configured",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{{ID: scorer.ObjectiveFacultyPreference, Weight: 3}},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveFacultyPreference},
		},
		{
			name: "Config 4: Both Gap and Pref configured",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: scorer.ObjectiveStudentGapPenalty, Weight: 2},
					{ID: scorer.ObjectiveFacultyPreference, Weight: 5},
				},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveStudentGapPenalty, scorer.ObjectiveFacultyPreference},
		},
		{
			name: "Config 5: FacultyPreference explicitly weight 0",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: scorer.ObjectiveStudentGapPenalty, Weight: 1},
					{ID: scorer.ObjectiveFacultyPreference, Weight: 0},
				},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveStudentGapPenalty},
		},
		{
			name: "Config 6: RoomChange only configured",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: scorer.ObjectiveRoomChange, Weight: 4},
				},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveRoomChange},
		},
		{
			name: "Config 7: All 3 Objectives configured",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: scorer.ObjectiveStudentGapPenalty, Weight: 1},
					{ID: scorer.ObjectiveFacultyPreference, Weight: 2},
					{ID: scorer.ObjectiveRoomChange, Weight: 3},
				},
			},
			expectedComponentIDs: []scorer.ObjectiveID{scorer.ObjectiveStudentGapPenalty, scorer.ObjectiveFacultyPreference, scorer.ObjectiveRoomChange},
		},
		{
			name: "Config 6: Unknown objective ID",
			cfg: scorer.ObjectiveConfig{
				Components: []scorer.ObjectiveComponent{
					{ID: "UnknownObjective", Weight: 5},
				},
			},
			expectedComponentIDs: nil,
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			breakdown := p.CalculateScoreWithConfig(&sol, tc.cfg)
			var actualIDs []scorer.ObjectiveID
			for _, c := range breakdown.Components {
				actualIDs = append(actualIDs, c.ID)
			}
			if !reflect.DeepEqual(actualIDs, tc.expectedComponentIDs) {
				t.Fatalf("Component ID list mismatch: expected %v, got %v", tc.expectedComponentIDs, actualIDs)
			}

			// Verify verifier parity for this config
			solWithScore := sol.Clone()
			solWithScore.Score = scorer.Score{
				HardViolations: 0,
				SoftPenalty:    breakdown.SoftPenalty,
				Breakdown:      breakdown,
			}
			report, err := verifier.VerifySolution(&p, &solWithScore, verifier.VerifyOptions{ObjectiveConfig: &tc.cfg})
			if err != nil || !report.Valid {
				t.Fatalf("Verifier failed for %s: err=%v, report=%+v", tc.name, err, report)
			}
		})
	}
}

// Red-Team Attack 3: Multi-Period 3-Slot Edge Expansion
func TestRedTeam_MultiPeriodEdgeExpansion(t *testing.T) {
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
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s1": {ID: "s1", Code: "CS101", Name: "Programming"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f1": {ID: "f1", Name: "Prof Alpha"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r1": {ID: "r1", Name: "Room 101", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"ts1": {ID: "ts1", Day: time.Monday, Period: 1},
			"ts2": {ID: "ts2", Day: time.Monday, Period: 2},
			"ts3": {ID: "ts3", Day: time.Monday, Period: 3},
			"ts4": {ID: "ts4", Day: time.Monday, Period: 4},
			"ts5": {ID: "ts5", Day: time.Monday, Period: 5},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co1": {ID: "co1", TermID: "term1", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1"},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"sr1": {ID: "sr1", CourseOfferingID: "co1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 3},
		},
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f1", TimeSlotID: "ts1"},
			{FacultyID: "f1", TimeSlotID: "ts2"},
			{FacultyID: "f1", TimeSlotID: "ts3"},
			{FacultyID: "f1", TimeSlotID: "ts4"},
			{FacultyID: "f1", TimeSlotID: "ts5"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r1", TimeSlotID: "ts1"},
			{RoomID: "r1", TimeSlotID: "ts2"},
			{RoomID: "r1", TimeSlotID: "ts3"},
			{RoomID: "r1", TimeSlotID: "ts4"},
			{RoomID: "r1", TimeSlotID: "ts5"},
		},
	}
	p.Prepare()

	// Preferences on slot 1 only, slot 2 only, slot 3 only, all slots
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "f1", TimeSlotID: "ts1", Weight: 2},
		{FacultyID: "f1", TimeSlotID: "ts2", Weight: 4},
		{FacultyID: "f1", TimeSlotID: "ts3", Weight: 8},
	}

	sol := problem.NewSolution()
	// Duration=3 starting at ts1 occupies ts1 (2), ts2 (4), ts3 (8) -> Total = 14
	_ = sol.AddAssignment(&p, problem.Assignment{
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
	if score.FacultyPreferencePenalty != 14 {
		t.Fatalf("Expected 3-period penalty 14 (2+4+8), got %d", score.FacultyPreferencePenalty)
	}

	incEval := localsearch.NewIncrementalScoreEvaluator(&p, &sol)
	incScore := incEval.Evaluate(&p, &sol)
	if incScore.FacultyPreferencePenalty != 14 {
		t.Fatalf("Expected incremental 3-period penalty 14, got %d", incScore.FacultyPreferencePenalty)
	}

	// Move duration=3 session to ts3 (occupies ts3:8, ts4:0, ts5:0 -> Total = 8)
	move := localsearch.CandidateMove{
		Kind:        localsearch.MoveKindSingle,
		Assignment1: "sr1#0",
		From1:       problem.Placement{RoomID: "r1", TimeSlotID: "ts1"},
		To1:         problem.Placement{RoomID: "r1", TimeSlotID: "ts3"},
	}

	evalRes := incEval.EvaluateCandidateMove(&p, &sol, move)
	_ = sol.ApplyMove(&p, problem.Move{AssignmentID: "sr1#0", From: move.From1, To: move.To1})
	fullAfter := p.CalculateScore(&sol)

	if evalRes.FacultyPreferencePenalty != 8 || fullAfter.FacultyPreferencePenalty != 8 {
		t.Fatalf("Expected moved penalty 8, got inc=%d, full=%d", evalRes.FacultyPreferencePenalty, fullAfter.FacultyPreferencePenalty)
	}
}

// Red-Team Attack 4: Tabu Search Optimization Pressure Verification
func TestRedTeam_TabuSearchOptimizationPressure(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

	// Pick a faculty and add preference penalty to whatever slot they are currently placed on in initial CSP solution
	cspSolver := backtracking.New()
	sol, _, err := cspSolver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeBasic})
	if err != nil {
		t.Fatalf("CSP solve failed: %v", err)
	}

	firstAssn := sol.Assignments[0]
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: firstAssn.FacultyID, TimeSlotID: firstAssn.TimeSlotID, Weight: 500},
	}

	initScore := p.CalculateScore(&sol)
	if initScore.FacultyPreferencePenalty != 500 {
		t.Fatalf("Initial penalty expected 500, got %d", initScore.FacultyPreferencePenalty)
	}

	// Run Tabu Search with high FacultyPreference weight
	tabuOpts := localsearch.TabuSearchOptions{
		MaxIterations:      100,
		NoImprovementLimit: 30,
		Seed:               42,
		ObjectiveConfig: &scorer.ObjectiveConfig{
			Components: []scorer.ObjectiveComponent{
				{ID: scorer.ObjectiveStudentGapPenalty, Weight: 1},
				{ID: scorer.ObjectiveFacultyPreference, Weight: 10},
			},
		},
	}

	optSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, sol, tabuOpts)
	if err != nil || tabuDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("TabuSearch failed: %v", err)
	}

	optScore := p.CalculateScoreWithConfig(&optSol, *tabuOpts.ObjectiveConfig)
	if optScore.FacultyPreferencePenalty >= initScore.FacultyPreferencePenalty {
		t.Fatalf("Tabu search failed to reduce preference penalty! Initial=%d, Optimized=%d", initScore.FacultyPreferencePenalty, optScore.FacultyPreferencePenalty)
	}
}
