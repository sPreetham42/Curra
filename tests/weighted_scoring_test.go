package tests

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// 1. Default Weighting Parity Test
// ----------------------------------------------------------------------------

func TestWeightedScoring_DefaultWeightParity(t *testing.T) {
	groups := []model.StudentGroupID{"g1"}
	occupied := []scorer.OccupiedPeriod{
		{StudentGroupID: "g1", Day: time.Monday, Period: 1},
		{StudentGroupID: "g1", Day: time.Monday, Period: 3}, // 1 gap at P2
		{StudentGroupID: "g1", Day: time.Tuesday, Period: 2},
		{StudentGroupID: "g1", Day: time.Tuesday, Period: 5}, // 2 gaps at P3, P4
	}

	scoreDefault := scorer.CalculateStudentGapPenalty(groups, occupied)
	if scoreDefault.StudentGapPenalty != 3 {
		t.Fatalf("expected raw 3 gaps, got %d", scoreDefault.StudentGapPenalty)
	}
	if scoreDefault.SoftPenalty != 3 {
		t.Fatalf("expected SoftPenalty 3 under weight 1, got %d", scoreDefault.SoftPenalty)
	}
	if len(scoreDefault.Components) != 1 {
		t.Fatalf("expected 1 objective component, got %d", len(scoreDefault.Components))
	}
	c := scoreDefault.Components[0]
	if c.ID != scorer.ObjectiveStudentGapPenalty || c.RawScore != 3 || c.Weight != 1 || c.WeightedScore != 3 {
		t.Fatalf("unexpected component score: %+v", c)
	}
}

// ----------------------------------------------------------------------------
// 2. Custom Weighting Scaling Test
// ----------------------------------------------------------------------------

func TestWeightedScoring_CustomWeights(t *testing.T) {
	groups := []model.StudentGroupID{"g1"}
	occupied := []scorer.OccupiedPeriod{
		{StudentGroupID: "g1", Day: time.Monday, Period: 1},
		{StudentGroupID: "g1", Day: time.Monday, Period: 4}, // 2 gaps (P2, P3)
	}

	weights := []int{1, 2, 5, 10, 50}
	for _, w := range weights {
		cfg := scorer.ObjectiveConfig{
			Components: []scorer.ObjectiveComponent{
				{ID: scorer.ObjectiveStudentGapPenalty, Weight: w},
			},
		}

		score := scorer.CalculateStudentGapPenaltyWithConfig(groups, occupied, cfg)
		if score.StudentGapPenalty != 2 {
			t.Fatalf("expected raw gaps=2, got %d for weight %d", score.StudentGapPenalty, w)
		}
		expectedWeighted := 2 * w
		if score.SoftPenalty != expectedWeighted {
			t.Fatalf("expected weighted SoftPenalty=%d, got %d for weight %d", expectedWeighted, score.SoftPenalty, w)
		}
		if len(score.Components) != 1 {
			t.Fatalf("expected 1 component, got %d", len(score.Components))
		}
		comp := score.Components[0]
		if comp.ID != scorer.ObjectiveStudentGapPenalty || comp.RawScore != 2 || comp.Weight != w || comp.WeightedScore != expectedWeighted {
			t.Fatalf("component mismatch for weight %d: %+v", w, comp)
		}
	}
}

// ----------------------------------------------------------------------------
// 3. Incremental vs Full Evaluator Weighted Parity Across Mutations
// ----------------------------------------------------------------------------

func TestWeightedScoring_IncrementalVsFull_Parity(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultMediumProblemConfig())
	sol := problem.NewSolution()

	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

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

	weights := []int{1, 3, 7}

	for _, w := range weights {
		cfg := scorer.ObjectiveConfig{
			Components: []scorer.ObjectiveComponent{
				{ID: scorer.ObjectiveStudentGapPenalty, Weight: w},
			},
		}

		fullEval := localsearch.FullScoreEvaluator{Config: cfg}
		incEval := localsearch.NewIncrementalScoreEvaluatorWithConfig(&p, &sol, cfg)

		// Initial check
		initFull := fullEval.Evaluate(&p, &sol)
		initInc := incEval.Evaluate(&p, &sol)
		if initFull.SoftPenalty != initInc.SoftPenalty || initFull.StudentGapPenalty != initInc.StudentGapPenalty {
			t.Fatalf("Weight %d initial mismatch: Full=%+v, Inc=%+v", w, initFull, initInc)
		}

		rng := rand.New(rand.NewSource(int64(9999 + w)))

		// 100 random candidate move evaluations
		for i := 0; i < 100; i++ {
			isSwap := rng.Float64() < 0.5
			if !isSwap {
				randIdx := rng.Intn(len(sol.Assignments))
				a := sol.Assignments[randIdx]
				randSlot := slotList[rng.Intn(len(slotList))]
				randRoom := roomList[rng.Intn(len(roomList))]

				cm := localsearch.CandidateMove{
					Kind:        localsearch.MoveKindSingle,
					Assignment1: a.ID,
					From1:       problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
					To1:         problem.Placement{RoomID: randRoom, TimeSlotID: randSlot},
				}

				incScore := incEval.EvaluateCandidateMove(&p, &sol, cm)

				clone := sol.Clone()
				move := problem.Move{AssignmentID: a.ID, From: cm.From1, To: cm.To1}
				_ = clone.ApplyMove(&p, move)
				fullScore := fullEval.Evaluate(&p, &clone)

				if incScore.SoftPenalty != fullScore.SoftPenalty || incScore.StudentGapPenalty != fullScore.StudentGapPenalty {
					t.Fatalf("Weight %d Single Move %d mismatch: Inc=%+v, Full=%+v", w, i, incScore, fullScore)
				}
			} else {
				idx1 := rng.Intn(len(sol.Assignments))
				idx2 := rng.Intn(len(sol.Assignments))
				if idx1 == idx2 {
					continue
				}
				a1 := sol.Assignments[idx1]
				a2 := sol.Assignments[idx2]

				cm := localsearch.CandidateMove{
					Kind:        localsearch.MoveKindSwap,
					Assignment1: a1.ID,
					From1:       problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
					To1:         problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
					Assignment2: a2.ID,
					From2:       problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
					To2:         problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
				}

				incScore := incEval.EvaluateCandidateMove(&p, &sol, cm)

				clone := sol.Clone()
				m1 := problem.Move{AssignmentID: a1.ID, From: cm.From1, To: cm.To1}
				m2 := problem.Move{AssignmentID: a2.ID, From: cm.From2, To: cm.To2}
				_ = clone.ApplySwap(&p, m1, m2)
				fullScore := fullEval.Evaluate(&p, &clone)

				if incScore.SoftPenalty != fullScore.SoftPenalty || incScore.StudentGapPenalty != fullScore.StudentGapPenalty {
					t.Fatalf("Weight %d Swap Move %d mismatch: Inc=%+v, Full=%+v", w, i, incScore, fullScore)
				}
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 4. Determinism Test
// ----------------------------------------------------------------------------

func TestWeightedScoring_Determinism(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultMediumProblemConfig())
	sol := problem.NewSolution()
	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

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

	cfg := scorer.ObjectiveConfig{
		Components: []scorer.ObjectiveComponent{
			{ID: scorer.ObjectiveStudentGapPenalty, Weight: 4},
		},
	}

	full1 := (&p).StudentGapPenaltyWithConfig(&sol, cfg)
	full2 := (&p).StudentGapPenaltyWithConfig(&sol, cfg)
	if full1.SoftPenalty != full2.SoftPenalty || full1.StudentGapPenalty != full2.StudentGapPenalty {
		t.Fatalf("non-deterministic full scoring: %+v vs %+v", full1, full2)
	}

	inc1 := localsearch.NewIncrementalScoreEvaluatorWithConfig(&p, &sol, cfg).Evaluate(&p, &sol)
	inc2 := localsearch.NewIncrementalScoreEvaluatorWithConfig(&p, &sol, cfg).Evaluate(&p, &sol)
	if inc1.SoftPenalty != inc2.SoftPenalty || inc1.StudentGapPenalty != inc2.StudentGapPenalty {
		t.Fatalf("non-deterministic incremental scoring: %+v vs %+v", inc1, inc2)
	}
}

// ----------------------------------------------------------------------------
// 5. Tabu Search with Weighted Objective Configuration
// ----------------------------------------------------------------------------

func TestWeightedScoring_TabuSearchIntegration(t *testing.T) {
	p := mediumTestProblem()
	cfg := scorer.ObjectiveConfig{
		Components: []scorer.ObjectiveComponent{
			{ID: scorer.ObjectiveStudentGapPenalty, Weight: 5},
		},
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      30,
		NoImprovementLimit: 15,
		TabuTenure:         5,
		MaxCandidates:      20,
		Seed:               42,
		ObjectiveConfig:    &cfg,
	}

	initialSol := problem.NewSolution()
	_ = initialSol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-1#0",
		CourseOfferingID:     "co-1",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "m-1",
		SessionRequirementID: "req-1",
		Instance:             0,
	})
	_ = initialSol.AddAssignment(&p, problem.Assignment{
		ID:                   "req-1#1",
		CourseOfferingID:     "co-1",
		StudentGroupID:       "g1-whole",
		FacultyID:            "f-1",
		RoomID:               "r-1",
		TimeSlotID:           "m-3", // 1 gap at m-2 -> raw=1, weighted=5
		SessionRequirementID: "req-1",
		Instance:             1,
	})

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, initialSol, opts)
	if err != nil {
		t.Fatalf("TabuSearch failed: %v", err)
	}

	if diag.InitialScore.SoftPenalty != 5 {
		t.Fatalf("expected initial SoftPenalty=5, got %d", diag.InitialScore.SoftPenalty)
	}
	if diag.BestScore.SoftPenalty > diag.InitialScore.SoftPenalty {
		t.Fatalf("best score %d should be <= initial score %d", diag.BestScore.SoftPenalty, diag.InitialScore.SoftPenalty)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations in final solution, got %d", bestSol.Score.HardViolations)
	}
}
