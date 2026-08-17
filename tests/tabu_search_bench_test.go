package tests

import (
	"context"
	"testing"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

func BenchmarkTabuSearch_MediumProblem(b *testing.B) {
	p := mediumTestProblem()
	solver := backtracking.New()
	initialSol, _, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 100000})
	if err != nil {
		b.Fatalf("failed to solve initial problem: %v", err)
	}

	opts := localsearch.TabuSearchOptions{
		MaxIterations:      50,
		NoImprovementLimit: 20,
		TabuTenure:         5,
		MaxCandidates:      50,
		Seed:               12345,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, diag, err := localsearch.TabuSearch(context.Background(), &p, initialSol, opts)
		if err != nil {
			b.Fatalf("TabuSearch error: %v", err)
		}
		if i == 0 {
			b.Logf("Benchmark stats: Iterations=%d, MovesGenerated=%d, LegalMoves=%d, IllegalMoves=%d, TabuRejected=%d, Accepted=%d, InitialScore=%d, BestScore=%d, Duration=%v",
				diag.Iterations, diag.MovesGenerated, diag.LegalMoves, diag.IllegalMoves, diag.TabuRejectedMoves, diag.AcceptedMoves, diag.InitialScore.StudentGapPenalty, diag.BestScore.StudentGapPenalty, diag.Duration)
		}
	}
}
