package localsearch_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// 1. Known Case Unit Tests
// ----------------------------------------------------------------------------

func TestIncrementalScorer_KnownCases(t *testing.T) {
	// Case 1: Pure CalculateDayGaps unit tests
	tests := []struct {
		name     string
		counts   []uint16
		expected int
	}{
		{"Empty", []uint16{0, 0, 0, 0, 0, 0, 0}, 0},
		{"SinglePeriod", []uint16{0, 1, 0, 0, 0, 0, 0}, 0},
		{"ConsecutivePeriods", []uint16{0, 1, 1, 1, 0, 0, 0}, 0},
		{"SingleGap", []uint16{0, 1, 0, 1, 0, 0, 0}, 1},               // P1 and P3 -> 1 gap (P2)
		{"DoubleGap", []uint16{0, 1, 0, 0, 1, 0, 0}, 2},               // P1 and P4 -> 2 gaps (P2, P3)
		{"MultipleGapsMixed", []uint16{0, 1, 0, 1, 0, 1, 0}, 2},       // P1, P3, P5 -> 2 gaps (P2, P4)
		{"OverlappingSessions", []uint16{0, 2, 0, 1, 0, 0, 0}, 1},     // P1(x2), P3(x1) -> 1 gap (P2)
		{"TrailingLeadingIgnored", []uint16{0, 0, 1, 0, 1, 0, 0, 0}, 1}, // P2 and P4 -> 1 gap (P3)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := localsearch.CalculateDayGaps(tc.counts)
			if actual != tc.expected {
				t.Fatalf("expected %d gaps, got %d for counts %v", tc.expected, actual, tc.counts)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// 2. Differential Parity Tests: Single Moves
// ----------------------------------------------------------------------------

func TestIncrementalScorer_SingleMove_DifferentialParity(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	sol := problem.NewSolution()

	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

	slotIdx := 0
	roomIdx := 0
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

	fullEvaluator := localsearch.FullScoreEvaluator{}
	incEvaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)

	// Baseline equality
	fullInitial := fullEvaluator.Evaluate(&p, &sol).StudentGapPenalty
	incInitial := incEvaluator.Evaluate(&p, &sol).StudentGapPenalty
	if fullInitial != incInitial {
		t.Fatalf("Initial score mismatch: Full=%d, Inc=%d", fullInitial, incInitial)
	}

	rng := rand.New(rand.NewSource(12345))

	for i := 0; i < 200; i++ {
		// Pick random assignment
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

		// 1. Incremental candidate score
		incScore := incEvaluator.EvaluateCandidateMove(&p, &sol, cm).StudentGapPenalty

		// 2. Full candidate score by applying move to clone
		clone := sol.Clone()
		move := problem.Move{AssignmentID: a.ID, From: cm.From1, To: cm.To1}
		_ = clone.ApplyMove(&p, move)
		fullScore := fullEvaluator.Evaluate(&p, &clone).StudentGapPenalty

		if incScore != fullScore {
			t.Fatalf("Move %d parity mismatch: Incremental=%d, FullOracle=%d (assignment=%s, from=%s, to=%s, group=%s)",
				i, incScore, fullScore, a.ID, cm.From1.TimeSlotID, cm.To1.TimeSlotID, a.StudentGroupID)
		}
	}
}

// ----------------------------------------------------------------------------
// 3. Differential Parity Tests: Swaps
// ----------------------------------------------------------------------------

func TestIncrementalScorer_Swap_DifferentialParity(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	sol := problem.NewSolution()

	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

	slotIdx := 0
	roomIdx := 0
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

	fullEvaluator := localsearch.FullScoreEvaluator{}
	incEvaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)

	rng := rand.New(rand.NewSource(67890))

	for i := 0; i < 200; i++ {
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

		// 1. Incremental score
		incScore := incEvaluator.EvaluateCandidateMove(&p, &sol, cm).StudentGapPenalty

		// 2. Full score on cloned solution with swap applied
		clone := sol.Clone()
		m1 := problem.Move{AssignmentID: a1.ID, From: cm.From1, To: cm.To1}
		m2 := problem.Move{AssignmentID: a2.ID, From: cm.From2, To: cm.To2}
		_ = clone.ApplySwap(&p, m1, m2)
		fullScore := fullEvaluator.Evaluate(&p, &clone).StudentGapPenalty

		if incScore != fullScore {
			t.Fatalf("Swap %d parity mismatch: Incremental=%d, FullOracle=%d (A1=%s group=%s, A2=%s group=%s)",
				i, incScore, fullScore, a1.ID, a1.StudentGroupID, a2.ID, a2.StudentGroupID)
		}
	}
}

// ----------------------------------------------------------------------------
// 4. Sequential Mutation Tests
// ----------------------------------------------------------------------------

func TestIncrementalScorer_SequentialMutations_Parity(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	sol := problem.NewSolution()

	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

	slotIdx := 0
	roomIdx := 0
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

	fullEvaluator := localsearch.FullScoreEvaluator{}
	incEvaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)
	rng := rand.New(rand.NewSource(54321))

	for step := 0; step < 100; step++ {
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

			expectedCandidateScore := incEvaluator.EvaluateCandidateMove(&p, &sol, cm).StudentGapPenalty

			// Apply to solution
			_ = localsearch.ApplyCandidateMove(&p, &sol, cm)
			// Apply to incremental evaluator
			incEvaluator.ApplyCandidateMove(&p, &sol, cm)

			currentIncScore := incEvaluator.TotalGaps()
			currentFullScore := fullEvaluator.Evaluate(&p, &sol).StudentGapPenalty

			if currentIncScore != expectedCandidateScore {
				t.Fatalf("Step %d Single Move: candidate score %d != state after apply %d",
					step, expectedCandidateScore, currentIncScore)
			}
			if currentIncScore != currentFullScore {
				t.Fatalf("Step %d Single Move: Incremental %d != Full Oracle %d",
					step, currentIncScore, currentFullScore)
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

			expectedCandidateScore := incEvaluator.EvaluateCandidateMove(&p, &sol, cm).StudentGapPenalty

			_ = localsearch.ApplyCandidateMove(&p, &sol, cm)
			incEvaluator.ApplyCandidateMove(&p, &sol, cm)

			currentIncScore := incEvaluator.TotalGaps()
			currentFullScore := fullEvaluator.Evaluate(&p, &sol).StudentGapPenalty

			if currentIncScore != expectedCandidateScore {
				t.Fatalf("Step %d Swap Move: candidate score %d != state after apply %d",
					step, expectedCandidateScore, currentIncScore)
			}
			if currentIncScore != currentFullScore {
				t.Fatalf("Step %d Swap Move: Incremental %d != Full Oracle %d",
					step, currentIncScore, currentFullScore)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 5. Multi-Period Session Parity Test
// ----------------------------------------------------------------------------

func TestIncrementalScorer_MultiPeriod_Parity(t *testing.T) {
	// Create problem with lab sessions having duration 2
	cfg := testutil.DefaultSmallProblemConfig()
	cfg.LabRatio = 0.5
	p := testutil.GenerateSyntheticProblem(cfg)

	sol := problem.NewSolution()
	slotList := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for s := range p.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(p.Rooms))
	for r := range p.Rooms {
		roomList = append(roomList, r)
	}

	slotIdx := 0
	roomIdx := 0
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

	fullEvaluator := localsearch.FullScoreEvaluator{}
	incEvaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)

	if incEvaluator.TotalGaps() != fullEvaluator.Evaluate(&p, &sol).StudentGapPenalty {
		t.Fatalf("Initial multi-period score mismatch: inc=%d, full=%d",
			incEvaluator.TotalGaps(), fullEvaluator.Evaluate(&p, &sol).StudentGapPenalty)
	}

	// Move lab session
	for _, a := range sol.Assignments {
		req := p.SessionRequirements[a.SessionRequirementID]
		if req.Duration > 1 {
			for _, targetSlot := range slotList {
				cm := localsearch.CandidateMove{
					Kind:        localsearch.MoveKindSingle,
					Assignment1: a.ID,
					From1:       problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
					To1:         problem.Placement{RoomID: a.RoomID, TimeSlotID: targetSlot},
				}

				incScore := incEvaluator.EvaluateCandidateMove(&p, &sol, cm).StudentGapPenalty

				clone := sol.Clone()
				_ = clone.ApplyMove(&p, problem.Move{AssignmentID: a.ID, From: cm.From1, To: cm.To1})
				fullScore := fullEvaluator.Evaluate(&p, &clone).StudentGapPenalty

				if incScore != fullScore {
					t.Fatalf("Multi-period move mismatch for assignment %s (duration=%d): inc=%d, full=%d",
						a.ID, req.Duration, incScore, fullScore)
				}
			}
			break
		}
	}
}

// ----------------------------------------------------------------------------
// 6. Comparative Micro-Benchmarks: Direct Scoring
// ----------------------------------------------------------------------------

func BenchmarkScorer_FullScore_Medium(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
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

	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = evaluator.Evaluate(&p, &sol)
	}
}

func BenchmarkScorer_IncrementalDelta_Medium(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
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

	evaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)
	a := sol.Assignments[0]
	cm := localsearch.CandidateMove{
		Kind:        localsearch.MoveKindSingle,
		Assignment1: a.ID,
		From1:       problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
		To1:         problem.Placement{RoomID: roomList[1], TimeSlotID: slotList[1]},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = evaluator.EvaluateCandidateMove(&p, &sol, cm)
	}
}

func BenchmarkScorer_FullScore_Large(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())
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

	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = evaluator.Evaluate(&p, &sol)
	}
}

func BenchmarkScorer_IncrementalDelta_Large(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())
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

	evaluator := localsearch.NewIncrementalScoreEvaluator(&p, &sol)
	a := sol.Assignments[0]
	cm := localsearch.CandidateMove{
		Kind:        localsearch.MoveKindSingle,
		Assignment1: a.ID,
		From1:       problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
		To1:         problem.Placement{RoomID: roomList[1], TimeSlotID: slotList[1]},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = evaluator.EvaluateCandidateMove(&p, &sol, cm)
	}
}

// ----------------------------------------------------------------------------
// 7. Comparative Micro-Benchmarks: Tabu Search Optimization
// ----------------------------------------------------------------------------

func BenchmarkTabuOptimization_IncrementalScoring_Medium(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	solver := backtracking.New()
	sol, _, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic})
	if err != nil {
		b.Fatalf("CSP solve failed: %v", err)
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      30,
		NoImprovementLimit: 15,
		TabuTenure:         7,
		MaxCandidates:      50,
		Seed:               42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := localsearch.TabuSearch(context.Background(), &p, sol, opts)
		if err != nil {
			b.Fatalf("TabuSearch failed: %v", err)
		}
	}
}
