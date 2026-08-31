package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// ----------------------------------------------------------------------------
// 1. Fixture Validation Test
// ----------------------------------------------------------------------------

func TestPerformanceFixtures_Validation(t *testing.T) {
	// Small
	pSmall := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	if violations := problem.Validate(pSmall); len(violations) > 0 {
		t.Fatalf("Small problem failed validation with %d violations: %+v", len(violations), violations[0])
	}
	sessionsSmall := testutil.CountTotalSessions(&pSmall)
	if sessionsSmall < 20 || sessionsSmall > 30 {
		t.Fatalf("expected Small problem to have ~20-30 sessions, got %d", sessionsSmall)
	}

	// Medium
	pMedium := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	if violations := problem.Validate(pMedium); len(violations) > 0 {
		t.Fatalf("Medium problem failed validation with %d violations: %+v", len(violations), violations[0])
	}
	sessionsMedium := testutil.CountTotalSessions(&pMedium)
	if sessionsMedium < 250 || sessionsMedium > 350 {
		t.Fatalf("expected Medium problem to have ~300 sessions, got %d", sessionsMedium)
	}
	if len(pMedium.Faculty) != 50 {
		t.Fatalf("expected 50 faculty in Medium problem, got %d", len(pMedium.Faculty))
	}
	if len(pMedium.Rooms) != 30 {
		t.Fatalf("expected 30 rooms in Medium problem, got %d", len(pMedium.Rooms))
	}

	// Large
	pLarge := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())
	if violations := problem.Validate(pLarge); len(violations) > 0 {
		t.Fatalf("Large problem failed validation with %d violations: %+v", len(violations), violations[0])
	}
	sessionsLarge := testutil.CountTotalSessions(&pLarge)
	if sessionsLarge < 2500 || sessionsLarge > 3500 {
		t.Fatalf("expected Large problem to have ~3000 sessions, got %d", sessionsLarge)
	}
	if len(pLarge.Faculty) != 300 {
		t.Fatalf("expected 300 faculty in Large problem, got %d", len(pLarge.Faculty))
	}
	if len(pLarge.Rooms) != 150 {
		t.Fatalf("expected 150 rooms in Large problem, got %d", len(pLarge.Rooms))
	}
}

// ----------------------------------------------------------------------------
// 2. A. Benchmark Problem.Prepare()
// ----------------------------------------------------------------------------

func BenchmarkProblemPrepare_Small(b *testing.B) {
	cfg := testutil.DefaultSmallProblemConfig()
	p := testutil.GenerateSyntheticProblem(cfg)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Prepare()
	}
}

func BenchmarkProblemPrepare_Medium(b *testing.B) {
	cfg := testutil.DefaultMediumProblemConfig()
	p := testutil.GenerateSyntheticProblem(cfg)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Prepare()
	}
}

func BenchmarkProblemPrepare_Large(b *testing.B) {
	cfg := testutil.DefaultLargeProblemConfig()
	p := testutil.GenerateSyntheticProblem(cfg)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Prepare()
	}
}

// ----------------------------------------------------------------------------
// 2. B. Benchmark CSP solve-to-feasible
// ----------------------------------------------------------------------------

func BenchmarkCSPSolve_Small_Basic(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	solver := backtracking.New()
	opts := problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := solver.Solve(context.Background(), p, opts)
		if err != nil {
			b.Fatalf("CSP solve failed: %v", err)
		}
	}
}

func BenchmarkCSPSolve_Small_Heuristic(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	solver := backtracking.New()
	opts := problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeHeuristic}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := solver.Solve(context.Background(), p, opts)
		if err != nil {
			b.Fatalf("CSP solve failed: %v", err)
		}
	}
}

func BenchmarkCSPSolve_Medium_Basic(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	solver := backtracking.New()
	opts := problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := solver.Solve(context.Background(), p, opts)
		if err != nil {
			b.Fatalf("CSP solve failed: %v", err)
		}
	}
}

// ----------------------------------------------------------------------------
// 2. C. Benchmark Tabu Search Optimization
// ----------------------------------------------------------------------------

func BenchmarkTabuOptimization_Small(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	solver := backtracking.New()
	sol, _, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000})
	if err != nil {
		b.Fatalf("CSP solve failed: %v", err)
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
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

func BenchmarkTabuOptimization_Medium(b *testing.B) {
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

// ----------------------------------------------------------------------------
// 2. D. Benchmark FullScoreEvaluator Alone
// ----------------------------------------------------------------------------

func BenchmarkFullScoreEvaluator_Small(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	solver := backtracking.New()
	sol, _, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic})
	if err != nil {
		b.Fatalf("CSP solve failed: %v", err)
	}
	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		evaluator.Evaluate(&p, &sol)
	}
}

func BenchmarkFullScoreEvaluator_Medium(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	solver := backtracking.New()
	sol, _, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic})
	if err != nil {
		b.Fatalf("CSP solve failed: %v", err)
	}
	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		evaluator.Evaluate(&p, &sol)
	}
}

func BenchmarkFullScoreEvaluator_Large(b *testing.B) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())
	// Build a valid synthetic solution directly for Large problem scoring
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
			a := problem.Assignment{
				ID:                   problem.NewAssignmentID(req.ID, inst),
				CourseOfferingID:     offering.ID,
				StudentGroupID:       offering.StudentGroupID,
				FacultyID:            offering.FacultyID,
				RoomID:               roomList[roomIdx%len(roomList)],
				TimeSlotID:           slotList[slotIdx%len(slotList)],
				SessionRequirementID: req.ID,
				Instance:             inst,
			}
			sol.Assignments = append(sol.Assignments, a)
			slotIdx++
			roomIdx++
		}
	}

	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		evaluator.Evaluate(&p, &sol)
	}
}

// ----------------------------------------------------------------------------
// 2. E. Benchmark FullScoreEvaluator Cost as Student-Group Count Increases
// ----------------------------------------------------------------------------

func benchmarkScalingByGroups(b *testing.B, numGroups int) {
	// Create problem with target group count
	numClasses := numGroups / 3
	if numClasses < 1 {
		numClasses = 1
	}
	cfg := testutil.SyntheticProblemConfig{
		Seed:              int64(numGroups * 17),
		TenantID:          fmt.Sprintf("tenant-scaling-%d", numGroups),
		NumDepartments:    1,
		ProgramsPerDept:   1,
		ClassesPerProgram: numClasses,
		SubgroupsPerClass: 2,
		NumFaculty:        numClasses * 2,
		NumRooms:          numClasses * 2,
		NumLabRooms:       numClasses / 2,
		DaysCount:         5,
		PeriodsPerDay:     8,
		OfferingsPerClass: 4,
		SessionsPerWeek:   3,
		LabRatio:          0.2,
	}

	p := testutil.GenerateSyntheticProblem(cfg)

	// Build a valid solution
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
			a := problem.Assignment{
				ID:                   problem.NewAssignmentID(req.ID, inst),
				CourseOfferingID:     offering.ID,
				StudentGroupID:       offering.StudentGroupID,
				FacultyID:            offering.FacultyID,
				RoomID:               roomList[roomIdx%len(roomList)],
				TimeSlotID:           slotList[slotIdx%len(slotList)],
				SessionRequirementID: req.ID,
				Instance:             inst,
			}
			sol.Assignments = append(sol.Assignments, a)
			slotIdx++
			roomIdx++
		}
	}

	evaluator := localsearch.FullScoreEvaluator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		evaluator.Evaluate(&p, &sol)
	}
}

func BenchmarkFullScoreEvaluator_Scaling_010Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 10)
}

func BenchmarkFullScoreEvaluator_Scaling_025Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 25)
}

func BenchmarkFullScoreEvaluator_Scaling_050Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 50)
}

func BenchmarkFullScoreEvaluator_Scaling_100Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 100)
}

func BenchmarkFullScoreEvaluator_Scaling_200Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 200)
}

func BenchmarkFullScoreEvaluator_Scaling_300Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 300)
}

func BenchmarkFullScoreEvaluator_Scaling_500Groups(b *testing.B) {
	benchmarkScalingByGroups(b, 500)
}

// ----------------------------------------------------------------------------
// 3. Evidence-Based Comprehensive Performance Measurement Test
// ----------------------------------------------------------------------------

func TestPerformanceMeasurement_EvidenceReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance baseline evidence report in short mode")
	}

	t.Log("================================================================================")
	t.Log("CURA PERFORMANCE BASELINE & EVIDENCE MEASUREMENT SUITE")
	t.Log("================================================================================")

	// --- 1. Small Problem Measurement ---
	t.Log("\n--- MEASURING SMALL PROBLEM (~24 sessions) ---")
	pSmall := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())

	prepStart := time.Now()
	pSmall.Prepare()
	prepSmallDur := time.Since(prepStart)

	cspSolver := backtracking.New()
	cspStart := time.Now()
	solSmall, cspDiagSmall, err := cspSolver.Solve(context.Background(), pSmall, problem.SolveOptions{MaxNodes: 100000})
	cspSmallDur := time.Since(cspStart)
	if err != nil {
		t.Fatalf("CSP Small solve failed: %v", err)
	}

	tabuOptsSmall := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               42,
	}
	tabuStart := time.Now()
	bestSolSmall, tabuDiagSmall, err := localsearch.TabuSearch(context.Background(), &pSmall, solSmall, tabuOptsSmall)
	tabuSmallDur := time.Since(tabuStart)
	if err != nil {
		t.Fatalf("Tabu Small failed: %v", err)
	}

	evaluator := localsearch.FullScoreEvaluator{}
	evalStart := time.Now()
	const evalRunsSmall = 1000
	for i := 0; i < evalRunsSmall; i++ {
		_ = evaluator.Evaluate(&pSmall, &bestSolSmall)
	}
	evalScoreSmallPerOp := time.Since(evalStart) / evalRunsSmall

	timePerCandSmall := time.Duration(0)
	if tabuDiagSmall.MovesGenerated > 0 {
		timePerCandSmall = tabuSmallDur / time.Duration(tabuDiagSmall.MovesGenerated)
	}

	t.Logf("SMALL RESULTS: Sessions=%d, Classes=%d, Faculty=%d, Rooms=%d, Slots=%d",
		len(bestSolSmall.Assignments), len(pSmall.Classes), len(pSmall.Faculty), len(pSmall.Rooms), len(pSmall.TimeSlots))
	t.Logf("  Prepare Duration:        %v", prepSmallDur)
	t.Logf("  CSP Duration:            %v", cspSmallDur)
	t.Logf("  CSP Nodes Explored:      %d", cspDiagSmall.NodesExplored)
	t.Logf("  CSP Backtracks:          %d", cspDiagSmall.Backtracks)
	t.Logf("  Tabu Duration:           %v", tabuSmallDur)
	t.Logf("  Tabu Iterations:         %d", tabuDiagSmall.Iterations)
	t.Logf("  Candidates Evaluated:    %d (Legal: %d, Illegal: %d, TabuRejected: %d, Accepted: %d)",
		tabuDiagSmall.MovesGenerated, tabuDiagSmall.LegalMoves, tabuDiagSmall.IllegalMoves, tabuDiagSmall.TabuRejectedMoves, tabuDiagSmall.AcceptedMoves)
	t.Logf("  Time Per Candidate:      %v", timePerCandSmall)
	t.Logf("  FullScoreEvaluator/Op:   %v", evalScoreSmallPerOp)
	t.Logf("  Initial Gap Score:       %d -> Best Gap Score: %d",
		tabuDiagSmall.InitialScore.StudentGapPenalty, tabuDiagSmall.BestScore.StudentGapPenalty)
	t.Logf("  Total Solve Time:        %v", cspSmallDur+tabuSmallDur)

	// --- 2. Medium Problem Measurement ---
	t.Log("\n--- MEASURING MEDIUM PROBLEM (~300 sessions) ---")
	pMedium := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())

	prepStart = time.Now()
	pMedium.Prepare()
	prepMedDur := time.Since(prepStart)

	cspStart = time.Now()
	solMed, cspDiagMed, err := cspSolver.Solve(context.Background(), pMedium, problem.SolveOptions{MaxNodes: 100000, SearchMode: problem.SearchModeBasic})
	cspMedDur := time.Since(cspStart)
	if err != nil {
		t.Fatalf("CSP Medium solve failed: %v", err)
	}

	tabuOptsMed := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 25,
		TabuTenure:         7,
		MaxCandidates:      50,
		Seed:               42,
	}
	tabuStart = time.Now()
	bestSolMed, tabuDiagMed, err := localsearch.TabuSearch(context.Background(), &pMedium, solMed, tabuOptsMed)
	tabuMedDur := time.Since(tabuStart)
	if err != nil {
		t.Fatalf("Tabu Medium failed: %v", err)
	}

	evalStart = time.Now()
	const evalRunsMed = 500
	for i := 0; i < evalRunsMed; i++ {
		_ = evaluator.Evaluate(&pMedium, &bestSolMed)
	}
	evalScoreMedPerOp := time.Since(evalStart) / evalRunsMed

	timePerCandMed := time.Duration(0)
	if tabuDiagMed.MovesGenerated > 0 {
		timePerCandMed = tabuMedDur / time.Duration(tabuDiagMed.MovesGenerated)
	}

	t.Logf("MEDIUM RESULTS: Sessions=%d, Classes=%d, Faculty=%d, Rooms=%d, Slots=%d",
		len(bestSolMed.Assignments), len(pMedium.Classes), len(pMedium.Faculty), len(pMedium.Rooms), len(pMedium.TimeSlots))
	t.Logf("  Prepare Duration:        %v", prepMedDur)
	t.Logf("  CSP Duration:            %v", cspMedDur)
	t.Logf("  CSP Nodes Explored:      %d", cspDiagMed.NodesExplored)
	t.Logf("  CSP Backtracks:          %d", cspDiagMed.Backtracks)
	t.Logf("  Tabu Duration:           %v", tabuMedDur)
	t.Logf("  Tabu Iterations:         %d", tabuDiagMed.Iterations)
	t.Logf("  Candidates Evaluated:    %d (Legal: %d, Illegal: %d, TabuRejected: %d, Accepted: %d)",
		tabuDiagMed.MovesGenerated, tabuDiagMed.LegalMoves, tabuDiagMed.IllegalMoves, tabuDiagMed.TabuRejectedMoves, tabuDiagMed.AcceptedMoves)
	t.Logf("  Time Per Candidate:      %v", timePerCandMed)
	t.Logf("  FullScoreEvaluator/Op:   %v", evalScoreMedPerOp)
	t.Logf("  Initial Gap Score:       %d -> Best Gap Score: %d",
		tabuDiagMed.InitialScore.StudentGapPenalty, tabuDiagMed.BestScore.StudentGapPenalty)
	t.Logf("  Total Solve Time:        %v", cspMedDur+tabuMedDur)

	// --- 3. Large Problem Measurement ---
	t.Log("\n--- MEASURING LARGE PROBLEM (~3000 sessions) ---")
	pLarge := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())

	prepStart = time.Now()
	pLarge.Prepare()
	prepLargeDur := time.Since(prepStart)

	// Build Large Solution
	solLarge := problem.NewSolution()
	slotList := make([]model.TimeSlotID, 0, len(pLarge.TimeSlots))
	for s := range pLarge.TimeSlots {
		slotList = append(slotList, s)
	}
	roomList := make([]model.RoomID, 0, len(pLarge.Rooms))
	for r := range pLarge.Rooms {
		roomList = append(roomList, r)
	}
	slotIdx := 0
	roomIdx := 0
	for _, req := range pLarge.SessionRequirements {
		offering := pLarge.CourseOfferings[req.CourseOfferingID]
		for inst := 0; inst < req.SessionsPerWeek; inst++ {
			a := problem.Assignment{
				ID:                   problem.NewAssignmentID(req.ID, inst),
				CourseOfferingID:     offering.ID,
				StudentGroupID:       offering.StudentGroupID,
				FacultyID:            offering.FacultyID,
				RoomID:               roomList[roomIdx%len(roomList)],
				TimeSlotID:           slotList[slotIdx%len(slotList)],
				SessionRequirementID: req.ID,
				Instance:             inst,
			}
			solLarge.Assignments = append(solLarge.Assignments, a)
			slotIdx++
			roomIdx++
		}
	}

	evalStart = time.Now()
	const evalRunsLarge = 100
	for i := 0; i < evalRunsLarge; i++ {
		_ = evaluator.Evaluate(&pLarge, &solLarge)
	}
	evalScoreLargePerOp := time.Since(evalStart) / evalRunsLarge

	t.Logf("LARGE RESULTS: Sessions=%d, Classes=%d, Faculty=%d, Rooms=%d, Slots=%d",
		len(solLarge.Assignments), len(pLarge.Classes), len(pLarge.Faculty), len(pLarge.Rooms), len(pLarge.TimeSlots))
	t.Logf("  Prepare Duration:        %v", prepLargeDur)
	t.Logf("  FullScoreEvaluator/Op:   %v", evalScoreLargePerOp)

	// --- 4. Scaling Breakdown Across Student Group Counts ---
	t.Log("\n--- MEASURING SCALING OF FullScoreEvaluator ACROSS GROUP COUNTS ---")
	groupCounts := []int{10, 25, 50, 100, 200, 300, 500}
	for _, gc := range groupCounts {
		numClasses := gc / 3
		if numClasses < 1 {
			numClasses = 1
		}
		cfg := testutil.SyntheticProblemConfig{
			Seed:              int64(gc * 17),
			TenantID:          fmt.Sprintf("tenant-scaling-%d", gc),
			NumDepartments:    1,
			ProgramsPerDept:   1,
			ClassesPerProgram: numClasses,
			SubgroupsPerClass: 2,
			NumFaculty:        numClasses * 2,
			NumRooms:          numClasses * 2,
			NumLabRooms:       numClasses / 2,
			DaysCount:         5,
			PeriodsPerDay:     8,
			OfferingsPerClass: 4,
			SessionsPerWeek:   3,
			LabRatio:          0.2,
		}
		pScale := testutil.GenerateSyntheticProblem(cfg)
		solScale := problem.NewSolution()
		sList := make([]model.TimeSlotID, 0, len(pScale.TimeSlots))
		for s := range pScale.TimeSlots {
			sList = append(sList, s)
		}
		rList := make([]model.RoomID, 0, len(pScale.Rooms))
		for r := range pScale.Rooms {
			rList = append(rList, r)
		}
		sIdx, rIdx := 0, 0
		for _, req := range pScale.SessionRequirements {
			offering := pScale.CourseOfferings[req.CourseOfferingID]
			for inst := 0; inst < req.SessionsPerWeek; inst++ {
				solScale.Assignments = append(solScale.Assignments, problem.Assignment{
					ID:                   problem.NewAssignmentID(req.ID, inst),
					CourseOfferingID:     offering.ID,
					StudentGroupID:       offering.StudentGroupID,
					FacultyID:            offering.FacultyID,
					RoomID:               rList[rIdx%len(rList)],
					TimeSlotID:           sList[sIdx%len(sList)],
					SessionRequirementID: req.ID,
					Instance:             inst,
				})
				sIdx++
				rIdx++
			}
		}

		runs := 200
		start := time.Now()
		for r := 0; r < runs; r++ {
			_ = scorer.CalculateStudentGapPenalty(nil, nil)
			_ = evaluator.Evaluate(&pScale, &solScale)
		}
		avgTime := time.Since(start) / time.Duration(runs)
		t.Logf("  Groups: %4d | Sessions: %4d | FullScoreEvaluator time: %8v",
			len(pScale.StudentGroups), len(solScale.Assignments), avgTime)
	}
}
