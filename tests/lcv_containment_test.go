package tests

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

// ----------------------------------------------------------------------------
// LCV Containment & Heuristic Default Test Suite
// ----------------------------------------------------------------------------

func TestLCVContainment_DefaultHeuristicBypassesLCV(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.Prepare()

	solver := backtracking.New()

	// Default heuristic options (SearchModeHeuristic)
	start := time.Now()
	var mBefore, mAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	runtime.ReadMemStats(&mAfter)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("default heuristic solve failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected status SOLVED, got %s", diag.Status)
	}
	if len(sol.Assignments) != 24 {
		t.Fatalf("expected 24 assignments, got %d", len(sol.Assignments))
	}
	if sol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", sol.Score.HardViolations)
	}

	allocBytes := mAfter.TotalAlloc - mBefore.TotalAlloc
	allocCount := mAfter.Mallocs - mBefore.Mallocs

	t.Logf("DEFAULT (MRV+Degree+FC): Duration=%v, Bytes=%d, Allocs=%d, Nodes=%d, Backtracks=%d",
		duration, allocBytes, allocCount, diag.NodesExplored, diag.Backtracks)

	// In LCV mode, small problem takes > 2s and > 100MB / 7M allocs.
	// Without LCV, default mode must complete in under 50ms and < 10MB.
	if duration > 200*time.Millisecond {
		t.Fatalf("default heuristic mode is suspiciously slow (%v); LCV may have been invoked", duration)
	}
	if allocBytes > 20*1024*1024 {
		t.Fatalf("default heuristic mode allocated too much memory (%d bytes); LCV may have been invoked", allocBytes)
	}
}

func TestLCVContainment_ExplicitLCVModeExecutes(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.Prepare()

	solver := backtracking.New()

	start := time.Now()
	var mBefore, mAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristicLCV})
	runtime.ReadMemStats(&mAfter)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("explicit LCV solve failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected status SOLVED, got %s", diag.Status)
	}
	if len(sol.Assignments) != 24 {
		t.Fatalf("expected 24 assignments, got %d", len(sol.Assignments))
	}
	if sol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", sol.Score.HardViolations)
	}

	allocBytes := mAfter.TotalAlloc - mBefore.TotalAlloc
	allocCount := mAfter.Mallocs - mBefore.Mallocs

	t.Logf("EXPLICIT LCV (MRV+Degree+LCV+FC): Duration=%v, Bytes=%d, Allocs=%d, Nodes=%d, Backtracks=%d",
		duration, allocBytes, allocCount, diag.NodesExplored, diag.Backtracks)

	// LCV mode does speculative counting and performs millions of allocations
	if allocCount < 1000000 {
		t.Fatalf("explicit LCV mode did not perform expected speculative counting (allocs=%d)", allocCount)
	}
}

func TestLCVContainment_BothModesZeroHardViolations(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.Prepare()

	solver := backtracking.New()

	// 1. Default Mode
	solDefault, diagDefault, errDefault := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if errDefault != nil || diagDefault.Status != diagnostics.SolveStatusSolved || solDefault.Score.HardViolations != 0 {
		t.Fatalf("default mode failed zero-violation check: err=%v status=%s violations=%d",
			errDefault, diagDefault.Status, solDefault.Score.HardViolations)
	}

	// 2. Explicit LCV Mode
	solLCV, diagLCV, errLCV := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristicLCV})
	if errLCV != nil || diagLCV.Status != diagnostics.SolveStatusSolved || solLCV.Score.HardViolations != 0 {
		t.Fatalf("LCV mode failed zero-violation check: err=%v status=%s violations=%d",
			errLCV, diagLCV.Status, solLCV.Score.HardViolations)
	}
}

func TestLCVContainment_DeterministicReplay(t *testing.T) {
	p := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	p.Prepare()

	solver := backtracking.New()

	// Replay Default Mode
	sol1, _, err1 := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	sol2, _, err2 := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err1 != nil || err2 != nil {
		t.Fatalf("default mode solve error: err1=%v err2=%v", err1, err2)
	}
	if !reflect.DeepEqual(sol1.Assignments, sol2.Assignments) {
		t.Fatal("default mode replay is non-deterministic")
	}

	// Replay Explicit LCV Mode
	solLCV1, _, errLCV1 := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristicLCV})
	solLCV2, _, errLCV2 := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristicLCV})
	if errLCV1 != nil || errLCV2 != nil {
		t.Fatalf("LCV mode solve error: err1=%v err2=%v", errLCV1, errLCV2)
	}
	if !reflect.DeepEqual(solLCV1.Assignments, solLCV2.Assignments) {
		t.Fatal("LCV mode replay is non-deterministic")
	}
}

func TestLCVContainment_LockedAssignmentsPreserved(t *testing.T) {
	p, _ := localSearchTestProblem()
	p.LockedAssignments = []problem.Assignment{
		{ID: "req-a-theory#0", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory", Instance: 0},
	}
	p.Prepare()

	solver := backtracking.New()

	for _, mode := range []problem.SearchMode{problem.SearchModeHeuristic, problem.SearchModeHeuristicLCV} {
		sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: mode})
		if err != nil || diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("mode %s solve failed: %v", mode, err)
		}
		found, ok := sol.Index.AssignmentByID("req-a-theory#0")
		if !ok || found.RoomID != "room-lecture-1" || found.TimeSlotID != "mon-1" {
			t.Fatalf("mode %s corrupted locked assignment: %+v", mode, found)
		}
	}
}

func TestBenchmarkComparativeHeuristics(t *testing.T) {
	type runResult struct {
		name       string
		mode       problem.SearchMode
		sessions   int
		duration   time.Duration
		allocBytes uint64
		allocCount uint64
		nodes      int
		backtracks int
	}

	results := make([]runResult, 0)

	measure := func(name string, p problem.Problem, mode problem.SearchMode) runResult {
		p.Prepare()
		solver := backtracking.New()

		runtime.GC()
		var mBefore, mAfter runtime.MemStats
		runtime.ReadMemStats(&mBefore)
		start := time.Now()

		sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: mode})
		duration := time.Since(start)
		runtime.ReadMemStats(&mAfter)

		if err != nil || diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("benchmark run %s failed: err=%v status=%s", name, err, diag.Status)
		}
		if sol.Score.HardViolations != 0 {
			t.Fatalf("benchmark run %s produced hard violations: %d", name, sol.Score.HardViolations)
		}

		return runResult{
			name:       name,
			mode:       mode,
			sessions:   len(sol.Assignments),
			duration:   duration,
			allocBytes: mAfter.TotalAlloc - mBefore.TotalAlloc,
			allocCount: mAfter.Mallocs - mBefore.Mallocs,
			nodes:      diag.NodesExplored,
			backtracks: diag.Backtracks,
		}
	}

	// 1. Small Problem (24 sessions)
	pSmall := GenerateSyntheticProblem(DefaultSmallProblemConfig())
	resSmallDefault := measure("Small Problem - Default (MRV+Degree+FC)", pSmall, problem.SearchModeHeuristic)
	resSmallLCV := measure("Small Problem - Explicit LCV (MRV+Degree+LCV+FC)", pSmall, problem.SearchModeHeuristicLCV)

	results = append(results, resSmallDefault, resSmallLCV)

	// 2. Medium Problem (300 sessions)
	pMedium := GenerateSyntheticProblem(DefaultMediumProblemConfig())
	resMediumDefault := measure("Medium Problem - Default (MRV+Degree+FC)", pMedium, problem.SearchModeHeuristic)
	results = append(results, resMediumDefault)

	fmt.Println("\n=========================================================================================")
	fmt.Println("CSP HEURISTIC COMPARATIVE BENCHMARK: DEFAULT (MRV+Degree+FC) vs EXPLICIT LCV")
	fmt.Println("=========================================================================================")
	fmt.Printf("%-45s | %-12s | %-12s | %-10s | %-6s | %-10s\n",
		"Configuration", "Runtime", "Memory", "Allocs", "Nodes", "Backtracks")
	fmt.Println("-----------------------------------------------------------------------------------------")
	for _, r := range results {
		memStr := fmt.Sprintf("%.2f MB", float64(r.allocBytes)/(1024*1024))
		if r.allocBytes < 1024*1024 {
			memStr = fmt.Sprintf("%.2f KB", float64(r.allocBytes)/1024)
		}
		fmt.Printf("%-45s | %-12v | %-12s | %-10d | %-6d | %-10d\n",
			r.name, r.duration.Round(time.Microsecond), memStr, r.allocCount, r.nodes, r.backtracks)
	}
	fmt.Println("=========================================================================================")
}
