package verifier_test

import (
	"context"
	"fmt"
	"math/rand"
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

// Helper to generate a valid solved solution on small problem.
func getSolvedSmallInstance(t *testing.T) (problem.Problem, problem.Solution) {
	t.Helper()
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()

	solver := backtracking.New()
	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("failed to solve small problem: %v (status=%s)", err, diag.Status)
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err != nil || !report.Valid || report.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("freshly solved instance failed initial verification: %v (report=%+v)", err, report)
	}

	return p, sol
}

func TestVerifier_1_MissingOneSessionRequirementAssignment(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Delete first assignment
	sol.Assignments = sol.Assignments[1:]

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for missing assignment, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_2_TooManyAssignmentsForRequirement(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Append a duplicate assignment with a new ID
	extra := sol.Assignments[0]
	extra.ID = "extra-assignment#99"
	sol.Assignments = append(sol.Assignments, extra)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for excess assignment, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_3_DuplicateAssignmentID(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Duplicate first assignment ID onto second assignment
	sol.Assignments[1].ID = sol.Assignments[0].ID

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for duplicate ID, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_4_InvalidSessionRequirementReference(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	sol.Assignments[0].SessionRequirementID = "non-existent-requirement"

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for invalid requirement reference, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_5_InvalidRoomID(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	sol.Assignments[0].RoomID = "non-existent-room"

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for invalid room reference, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_6_InvalidTimeSlotID(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	sol.Assignments[0].TimeSlotID = "non-existent-slot"

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for invalid time slot reference, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_7_InvalidOffGridDuration(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Place an assignment on the last period of the day for a 2-period requirement
	reqID := sol.Assignments[0].SessionRequirementID
	req := p.SessionRequirements[reqID]
	req.Duration = 2
	p.SessionRequirements[reqID] = req
	sol.Assignments[0].TimeSlotID = "mon-6" // Last period of day (out of 6)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for off-grid duration placement, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_8_MissingLockedAssignment(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Add locked assignment to problem that is NOT in solution
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-mandatory#0",
			CourseOfferingID:     sol.Assignments[0].CourseOfferingID,
			StudentGroupID:       sol.Assignments[0].StudentGroupID,
			FacultyID:            sol.Assignments[0].FacultyID,
			RoomID:               sol.Assignments[0].RoomID,
			TimeSlotID:           sol.Assignments[0].TimeSlotID,
			SessionRequirementID: sol.Assignments[0].SessionRequirementID,
			Instance:             0,
		},
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for missing locked assignment, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_9_MutatedLockedAssignment(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Lock the first assignment, but mutate its room in the solution
	locked := sol.Assignments[0]
	p.LockedAssignments = []problem.Assignment{locked}

	// Mutate room in solution
	sol.Assignments[0].RoomID = "room-2"
	if locked.RoomID == "room-2" {
		sol.Assignments[0].RoomID = "room-1"
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for mutated locked assignment, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_10_InjectedHardConstraintViolation(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Put two assignments for different groups/faculty into the same room and slot
	sol.Assignments[1].RoomID = sol.Assignments[0].RoomID
	sol.Assignments[1].TimeSlotID = sol.Assignments[0].TimeSlotID

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for room conflict hard violation, got valid")
	}
	if report.Status != diagnostics.SolveStatusInfeasible && report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INFEASIBLE or INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_11_TamperedStudentGapPenalty(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Tamper with reported student gap penalty
	sol.Score.Breakdown.StudentGapPenalty += 50
	sol.Score.SoftPenalty += 50

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for tampered student gap penalty, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_12_TamperedWeightedSoftPenalty(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Tamper only with top-level SoftPenalty
	sol.Score.SoftPenalty = 999

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for tampered SoftPenalty, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_13_TamperedScoreBreakdown(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	// Tamper with Breakdown.GroupGaps
	if sol.Score.Breakdown.GroupGaps == nil {
		sol.Score.Breakdown.GroupGaps = make(map[model.StudentGroupID]int)
	}
	sol.Score.Breakdown.GroupGaps["group-fake"] = 123

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("expected verification failure for tampered ScoreBreakdown, got valid")
	}
	if report.Status != diagnostics.SolveStatusInvalidResult {
		t.Fatalf("expected status INVALID_RESULT, got %s", report.Status)
	}
}

func TestVerifier_14_ValidCompleteSolutionPasses(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err != nil {
		t.Fatalf("expected clean verification, got error: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected report.Valid=true, got false (violations=%+v)", report.Violations)
	}
	if report.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected status SOLVED, got %s", report.Status)
	}
}

// ----------------------------------------------------------------------------
// INTEGRATION: CSP -> TABU -> VERIFY_SOLUTION
// ----------------------------------------------------------------------------

func TestVerifier_Integration_CSP_Tabu_VerifySolution(t *testing.T) {
	p := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	p.Prepare()

	// 1. CSP Solve
	cspSolver := backtracking.New()
	initialSol, cspDiag, err := cspSolver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil || cspDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solve failed: %v", err)
	}

	// 2. Tabu Search Optimize
	optSol, tabuDiag, err := localsearch.TabuSearch(context.Background(), &p, initialSol, localsearch.TabuSearchOptions{
		MaxIterations: 50,
		Seed:          42,
	})
	if err != nil || tabuDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("Tabu search failed: %v", err)
	}

	// 3. Authoritative Verification
	report, err := verifier.VerifySolution(&p, &optSol, verifier.VerifyOptions{})
	if err != nil || !report.Valid || report.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP -> Tabu solution failed authoritative verification: %v (report=%+v)", err, report)
	}

	t.Logf("Integration verification passed: assignments=%d, softPenalty=%d, status=%s",
		len(optSol.Assignments), optSol.Score.SoftPenalty, report.Status)
}

// ----------------------------------------------------------------------------
// RANDOMIZED PROPERTY / FUZZ TEST (500+ CORRUPTIONS)
// ----------------------------------------------------------------------------

func TestRandomizedVerificationProperty(t *testing.T) {
	pBase := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	pBase.Prepare()

	solver := backtracking.New()
	validSol, _, err := solver.Solve(context.Background(), pBase, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil {
		t.Fatalf("base solve failed: %v", err)
	}

	rng := rand.New(rand.NewSource(12345))
	numTrials := 500

	corruptionKinds := []string{
		"delete_assignment",
		"duplicate_assignment",
		"modify_assignment_id",
		"modify_room",
		"modify_slot",
		"modify_locked_assignment",
		"tamper_soft_penalty",
		"tamper_hard_violations",
		"tamper_group_gaps",
	}

	for i := 0; i < numTrials; i++ {
		p := pBase
		sol := validSol.Clone()

		kind := corruptionKinds[rng.Intn(len(corruptionKinds))]

		switch kind {
		case "delete_assignment":
			if len(sol.Assignments) > 0 {
				idx := rng.Intn(len(sol.Assignments))
				sol.Assignments = append(sol.Assignments[:idx], sol.Assignments[idx+1:]...)
			}
		case "duplicate_assignment":
			if len(sol.Assignments) > 0 {
				idx := rng.Intn(len(sol.Assignments))
				dup := sol.Assignments[idx]
				dup.ID = problem.AssignmentID(fmt.Sprintf("dup-%d", rng.Intn(10000)))
				sol.Assignments = append(sol.Assignments, dup)
			}
		case "modify_assignment_id":
			if len(sol.Assignments) > 0 {
				idx := rng.Intn(len(sol.Assignments))
				sol.Assignments[idx].ID = ""
			}
		case "modify_room":
			if len(sol.Assignments) > 0 {
				idx := rng.Intn(len(sol.Assignments))
				sol.Assignments[idx].RoomID = "non-existent-room-xyz"
			}
		case "modify_slot":
			if len(sol.Assignments) > 0 {
				idx := rng.Intn(len(sol.Assignments))
				sol.Assignments[idx].TimeSlotID = "non-existent-slot-xyz"
			}
		case "modify_locked_assignment":
			if len(sol.Assignments) > 0 {
				p.LockedAssignments = []problem.Assignment{sol.Assignments[0]}
				sol.Assignments[0].RoomID = "non-existent-locked-room"
			}
		case "tamper_soft_penalty":
			sol.Score.SoftPenalty += (rng.Intn(50) + 1)
		case "tamper_hard_violations":
			sol.Score.HardViolations += (rng.Intn(5) + 1)
		case "tamper_group_gaps":
			if sol.Score.Breakdown.GroupGaps == nil {
				sol.Score.Breakdown.GroupGaps = make(map[model.StudentGroupID]int)
			}
			sol.Score.Breakdown.GroupGaps["fake-group"] = 999
		}

		report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
		if err == nil && report.Valid {
			t.Fatalf("trial %d [corruption=%s] was not detected by verifier!", i, kind)
		}
	}

	t.Logf("Randomized property test passed: %d corruption variants successfully detected", numTrials)
}

// ----------------------------------------------------------------------------
// ADVERSARIAL VERIFIER INDEPENDENCE TEST
// ----------------------------------------------------------------------------

func TestVerifier_AdversarialScorerDecoupling(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: sol.Assignments[0].FacultyID, TimeSlotID: sol.Assignments[0].TimeSlotID, Weight: 10},
	}

	// 1. Valid recalculation using independent verifier
	fullScore := p.CalculateScore(&sol)
	sol.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    fullScore.SoftPenalty,
		Breakdown:      fullScore,
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err != nil || !report.Valid {
		t.Fatalf("Valid solution failed verification: %v (report=%+v)", err, report)
	}

	// 2. Simulate a production scorer bug where FacultyPreferencePenalty reported in solution is wrong (e.g. 99 instead of 10)
	tamperedSol := sol.Clone()
	tamperedSol.Score.Breakdown.FacultyPreferencePenalty = 99
	tamperedSol.Score.SoftPenalty = tamperedSol.Score.Breakdown.StudentGapPenalty + 99

	tamperedReport, tamperedErr := verifier.VerifySolution(&p, &tamperedSol, verifier.VerifyOptions{})
	if tamperedErr == nil || tamperedReport.Valid {
		t.Fatal("Verifier failed to catch tampered FacultyPreferencePenalty!")
	}

	// 3. Simulate a production scorer bug where StudentGapPenalty reported in solution is wrong
	tamperedGapSol := sol.Clone()
	tamperedGapSol.Score.Breakdown.StudentGapPenalty += 5
	tamperedGapSol.Score.SoftPenalty += 5

	tamperedGapReport, tamperedGapErr := verifier.VerifySolution(&p, &tamperedGapSol, verifier.VerifyOptions{})
	if tamperedGapErr == nil || tamperedGapReport.Valid {
		t.Fatal("Verifier failed to catch tampered StudentGapPenalty!")
	}

	// 4. Simulate a production scorer bug where RoomChangePenalty reported in solution is wrong
	tamperedRCSol := sol.Clone()
	tamperedRCSol.Score.Breakdown.RoomChangePenalty += 7
	tamperedRCSol.Score.SoftPenalty += 7

	tamperedRCReport, tamperedRCErr := verifier.VerifySolution(&p, &tamperedRCSol, verifier.VerifyOptions{})
	if tamperedRCErr == nil || tamperedRCReport.Valid {
		t.Fatal("Verifier failed to catch tampered RoomChangePenalty!")
	}
}

// ----------------------------------------------------------------------------
// BENCHMARK: VERIFICATION OVERHEAD ON SMALL, MEDIUM, LARGE INSTANCES
// ----------------------------------------------------------------------------

func TestBenchmarkVerifierOverhead(t *testing.T) {
	type benchTarget struct {
		name     string
		sessions int
		p        problem.Problem
		sol      problem.Solution
	}

	// 1. Small Problem
	pSmall := testutil.GenerateSyntheticProblem(testutil.DefaultSmallProblemConfig())
	pSmall.Prepare()
	sSmall, _, _ := backtracking.New().Solve(context.Background(), pSmall, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})

	// 2. Medium Problem
	pMed := testutil.GenerateSyntheticProblem(testutil.DefaultMediumProblemConfig())
	pMed.Prepare()
	sMed, _, _ := backtracking.New().Solve(context.Background(), pMed, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})

	// 3. Large Problem (Generate valid packed timetable for scoring/verification benchmark)
	pLarge := testutil.GenerateSyntheticProblem(testutil.DefaultLargeProblemConfig())
	pLarge.Prepare()
	var sLarge problem.Solution
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
			sLarge.Assignments = append(sLarge.Assignments, a)
			slotIdx++
			roomIdx++
		}
	}
	sLarge.Score.HardViolations = 0
	expectedLarge := pLarge.CalculateScore(&sLarge)
	sLarge.Score.SoftPenalty = expectedLarge.SoftPenalty
	sLarge.Score.Breakdown = expectedLarge

	targets := []benchTarget{
		{"Small Problem (24 sessions)", 24, pSmall, sSmall},
		{"Medium Problem (300 sessions)", 300, pMed, sMed},
		{"Large Problem (3,000 sessions)", 3000, pLarge, sLarge},
	}

	fmt.Println("\n=========================================================================================")
	fmt.Println("AUTHORITATIVE VERIFIER PERFORMANCE BENCHMARK (VerifySolution)")
	fmt.Println("=========================================================================================")
	fmt.Printf("%-35s | %-12s | %-15s\n", "Instance Size", "Verify Time", "Result Status")
	fmt.Println("-----------------------------------------------------------------------------------------")

	for _, target := range targets {
		start := time.Now()
		report, _ := verifier.VerifySolution(&target.p, &target.sol, verifier.VerifyOptions{})
		duration := time.Since(start)

		fmt.Printf("%-35s | %-12v | %-15s\n", target.name, duration.Round(time.Microsecond), report.Status)
	}
	fmt.Println("=========================================================================================")
}
