package tests

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

// ----------------------------------------------------------------------------
// CSP Heuristic Exploration Harness
// Allows fine-grained isolation and benchmarking of:
// - Basic Chronological Backtracking
// - MRV + Degree (Variable selection only)
// - MRV + Degree + LCV (Variable selection + Value ordering, no FC)
// - MRV + Degree + FC (Variable selection + Forward checking, natural value ordering)
// - MRV + Degree + LCV + FC (Full Heuristic search)
// ----------------------------------------------------------------------------

type HeuristicStrategy string

const (
	StrategyBasic        HeuristicStrategy = "Basic"
	StrategyMRVDegree    HeuristicStrategy = "MRV+Degree"
	StrategyMRVDegreeLCV HeuristicStrategy = "MRV+Degree+LCV"
	StrategyMRVDegreeFC  HeuristicStrategy = "MRV+Degree+FC"
	StrategyFullHeuristic HeuristicStrategy = "MRV+Degree+LCV+FC"
)

type CSPMetrics struct {
	Strategy       HeuristicStrategy
	ProblemName    string
	SolveTime      time.Duration
	NodesExplored  int
	Backtracks     int
	Candidates     int
	AllocBytes     uint64
	AllocsCount    uint64
	Success        bool
	Status         diagnostics.SolveStatus
	FeasibleErrors string
}

type customCSPSolver struct {
	strategy    HeuristicStrategy
	constraints []constraints.Constraint
}

type cspDecision struct {
	offering    model.CourseOffering
	requirement model.SessionRequirement
	instance    int
}

type cspCandidate struct {
	roomID     model.RoomID
	timeSlotID model.TimeSlotID
}

func (s *customCSPSolver) SolveCustom(ctx context.Context, p problem.Problem, maxNodes int) (problem.Solution, diagnostics.Diagnostics, CSPMetrics) {
	p.Prepare()
	solution := problem.NewSolution()
	diag := diagnostics.Diagnostics{}

	var mStart, mEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mStart)
	startTime := time.Now()

	decisions := buildCSPDecisions(&p)
	var err error

	switch s.strategy {
	case StrategyBasic:
		err = s.searchBasicCustom(ctx, &p, maxNodes, decisions, 0, &solution, &diag)
	case StrategyMRVDegree:
		domains := buildCSPInitialDomains(&p, decisions, &solution, s.constraints)
		err = s.searchMRVDegree(ctx, &p, maxNodes, decisions, domains, make(map[int]struct{}), &solution, &diag)
	case StrategyMRVDegreeLCV:
		domains := buildCSPInitialDomains(&p, decisions, &solution, s.constraints)
		err = s.searchMRVDegreeLCV(ctx, &p, maxNodes, decisions, domains, make(map[int]struct{}), &solution, &diag)
	case StrategyMRVDegreeFC:
		domains := buildCSPInitialDomains(&p, decisions, &solution, s.constraints)
		err = s.searchMRVDegreeFC(ctx, &p, maxNodes, decisions, domains, make(map[int]struct{}), &solution, &diag)
	case StrategyFullHeuristic:
		domains := buildCSPInitialDomains(&p, decisions, &solution, s.constraints)
		err = s.searchFullHeuristic(ctx, &p, maxNodes, decisions, domains, make(map[int]struct{}), &solution, &diag)
	}

	runtime.ReadMemStats(&mEnd)
	duration := time.Since(startTime)

	metrics := CSPMetrics{
		Strategy:      s.strategy,
		SolveTime:     duration,
		NodesExplored: diag.NodesExplored,
		Backtracks:    diag.Backtracks,
		Candidates:    diag.Candidates,
		AllocBytes:    mEnd.TotalAlloc - mStart.TotalAlloc,
		AllocsCount:   mEnd.Mallocs - mStart.Mallocs,
		Success:       err == nil,
	}

	if err == nil {
		diag.Status = diagnostics.SolveStatusSolved
	} else if errors.Is(err, backtracking.ErrNodeLimit) {
		diag.Status = diagnostics.SolveStatusNodeLimit
		metrics.FeasibleErrors = err.Error()
	} else {
		diag.Status = diagnostics.SolveStatusInfeasible
		metrics.FeasibleErrors = err.Error()
	}
	metrics.Status = diag.Status

	return solution, diag, metrics
}

func buildCSPDecisions(p *problem.Problem) []cspDecision {
	var decisions []cspDecision
	for _, req := range p.SessionRequirements {
		offering := p.CourseOfferings[req.CourseOfferingID]
		for inst := 0; inst < req.SessionsPerWeek; inst++ {
			decisions = append(decisions, cspDecision{
				offering:    offering,
				requirement: req,
				instance:    inst,
			})
		}
	}
	return decisions
}

func buildCSPAssignment(d cspDecision, val cspCandidate) problem.Assignment {
	return problem.Assignment{
		ID:                   problem.NewAssignmentID(d.requirement.ID, d.instance),
		CourseOfferingID:     d.offering.ID,
		StudentGroupID:       d.offering.StudentGroupID,
		FacultyID:            d.offering.FacultyID,
		RoomID:               val.roomID,
		TimeSlotID:           val.timeSlotID,
		SessionRequirementID: d.requirement.ID,
		Instance:             d.instance,
	}
}

func buildCSPInitialDomains(p *problem.Problem, decisions []cspDecision, sol *problem.Solution, checks []constraints.Constraint) map[int][]cspCandidate {
	domains := make(map[int][]cspCandidate, len(decisions))
	for i, d := range decisions {
		for rID := range p.Rooms {
			for sID := range p.TimeSlots {
				slotIDs, ok := p.OccupiedSlotIDs(sID, d.requirement.Duration)
				if !ok {
					continue
				}
				cand := cspCandidate{roomID: rID, timeSlotID: slotIDs[0]}
				a := buildCSPAssignment(d, cand)
				if len(constraints.CheckAll(p, sol, a, checks)) == 0 {
					domains[i] = append(domains[i], cand)
				}
			}
		}
	}
	return domains
}

func (s *customCSPSolver) searchBasicCustom(ctx context.Context, p *problem.Problem, maxNodes int, decisions []cspDecision, pos int, sol *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if pos == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= maxNodes {
		return backtracking.ErrNodeLimit
	}
	diag.NodesExplored++

	d := decisions[pos]
	for rID := range p.Rooms {
		for sID := range p.TimeSlots {
			slotIDs, ok := p.OccupiedSlotIDs(sID, d.requirement.Duration)
			if !ok {
				continue
			}
			diag.Candidates++
			cand := cspCandidate{roomID: rID, timeSlotID: slotIDs[0]}
			a := buildCSPAssignment(d, cand)
			if len(constraints.CheckAll(p, sol, a, s.constraints)) > 0 {
				continue
			}
			if err := sol.AddAssignment(p, a); err != nil {
				continue
			}
			if err := s.searchBasicCustom(ctx, p, maxNodes, decisions, pos+1, sol, diag); err == nil {
				return nil
			} else if !errors.Is(err, backtracking.ErrNoSolution) {
				return err
			}
			sol.RemoveLastAssignment(p)
		}
	}
	diag.Backtracks++
	return backtracking.ErrNoSolution
}

func (s *customCSPSolver) searchMRVDegree(ctx context.Context, p *problem.Problem, maxNodes int, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, sol *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assigned) == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= maxNodes {
		return backtracking.ErrNodeLimit
	}

	selected := selectMRVVar(decisions, domains, assigned)
	if selected == -1 || len(domains[selected]) == 0 {
		diag.Backtracks++
		return backtracking.ErrNoSolution
	}
	diag.NodesExplored++

	for _, cand := range domains[selected] {
		diag.Candidates++
		a := buildCSPAssignment(decisions[selected], cand)
		if len(constraints.CheckAll(p, sol, a, s.constraints)) > 0 {
			continue
		}
		if err := sol.AddAssignment(p, a); err != nil {
			continue
		}
		assigned[selected] = struct{}{}
		if err := s.searchMRVDegree(ctx, p, maxNodes, decisions, domains, assigned, sol, diag); err == nil {
			return nil
		} else if !errors.Is(err, backtracking.ErrNoSolution) {
			return err
		}
		delete(assigned, selected)
		sol.RemoveLastAssignment(p)
	}
	diag.Backtracks++
	return backtracking.ErrNoSolution
}

func (s *customCSPSolver) searchMRVDegreeLCV(ctx context.Context, p *problem.Problem, maxNodes int, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, sol *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assigned) == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= maxNodes {
		return backtracking.ErrNodeLimit
	}

	selected := selectMRVVar(decisions, domains, assigned)
	if selected == -1 || len(domains[selected]) == 0 {
		diag.Backtracks++
		return backtracking.ErrNoSolution
	}
	diag.NodesExplored++

	orderedValues := orderValuesByLCV(p, decisions, domains, assigned, selected, sol, s.constraints)

	for _, cand := range orderedValues {
		diag.Candidates++
		a := buildCSPAssignment(decisions[selected], cand)
		if len(constraints.CheckAll(p, sol, a, s.constraints)) > 0 {
			continue
		}
		if err := sol.AddAssignment(p, a); err != nil {
			continue
		}
		assigned[selected] = struct{}{}
		if err := s.searchMRVDegreeLCV(ctx, p, maxNodes, decisions, domains, assigned, sol, diag); err == nil {
			return nil
		} else if !errors.Is(err, backtracking.ErrNoSolution) {
			return err
		}
		delete(assigned, selected)
		sol.RemoveLastAssignment(p)
	}
	diag.Backtracks++
	return backtracking.ErrNoSolution
}

func (s *customCSPSolver) searchMRVDegreeFC(ctx context.Context, p *problem.Problem, maxNodes int, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, sol *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assigned) == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= maxNodes {
		return backtracking.ErrNodeLimit
	}

	selected := selectMRVVar(decisions, domains, assigned)
	if selected == -1 || len(domains[selected]) == 0 {
		diag.Backtracks++
		return backtracking.ErrNoSolution
	}
	diag.NodesExplored++

	for _, cand := range domains[selected] {
		diag.Candidates++
		a := buildCSPAssignment(decisions[selected], cand)
		if len(constraints.CheckAll(p, sol, a, s.constraints)) > 0 {
			continue
		}
		if err := sol.AddAssignment(p, a); err != nil {
			continue
		}
		assigned[selected] = struct{}{}
		nextDomains, viable := pruneCSPDomains(p, decisions, domains, assigned, sol, s.constraints)
		if viable {
			if err := s.searchMRVDegreeFC(ctx, p, maxNodes, decisions, nextDomains, assigned, sol, diag); err == nil {
				return nil
			} else if !errors.Is(err, backtracking.ErrNoSolution) {
				return err
			}
		} else {
			diag.Backtracks++
		}
		delete(assigned, selected)
		sol.RemoveLastAssignment(p)
	}
	diag.Backtracks++
	return backtracking.ErrNoSolution
}

func (s *customCSPSolver) searchFullHeuristic(ctx context.Context, p *problem.Problem, maxNodes int, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, sol *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assigned) == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= maxNodes {
		return backtracking.ErrNodeLimit
	}

	selected := selectMRVVar(decisions, domains, assigned)
	if selected == -1 || len(domains[selected]) == 0 {
		diag.Backtracks++
		return backtracking.ErrNoSolution
	}
	diag.NodesExplored++

	orderedValues := orderValuesByLCV(p, decisions, domains, assigned, selected, sol, s.constraints)

	for _, cand := range orderedValues {
		diag.Candidates++
		a := buildCSPAssignment(decisions[selected], cand)
		if len(constraints.CheckAll(p, sol, a, s.constraints)) > 0 {
			continue
		}
		if err := sol.AddAssignment(p, a); err != nil {
			continue
		}
		assigned[selected] = struct{}{}
		nextDomains, viable := pruneCSPDomains(p, decisions, domains, assigned, sol, s.constraints)
		if viable {
			if err := s.searchFullHeuristic(ctx, p, maxNodes, decisions, nextDomains, assigned, sol, diag); err == nil {
				return nil
			} else if !errors.Is(err, backtracking.ErrNoSolution) {
				return err
			}
		} else {
			diag.Backtracks++
		}
		delete(assigned, selected)
		sol.RemoveLastAssignment(p)
	}
	diag.Backtracks++
	return backtracking.ErrNoSolution
}

func selectMRVVar(decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}) int {
	best := -1
	bestSize := 1 << 30
	for i := range decisions {
		if _, done := assigned[i]; done {
			continue
		}
		size := len(domains[i])
		if size < bestSize {
			best = i
			bestSize = size
		}
	}
	return best
}

func orderValuesByLCV(p *problem.Problem, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, selected int, sol *problem.Solution, checks []constraints.Constraint) []cspCandidate {
	values := append([]cspCandidate(nil), domains[selected]...)
	type ranked struct {
		cand         cspCandidate
		eliminations int
	}
	rankedList := make([]ranked, 0, len(values))
	for _, val := range values {
		a := buildCSPAssignment(decisions[selected], val)
		if len(constraints.CheckAll(p, sol, a, checks)) > 0 {
			rankedList = append(rankedList, ranked{cand: val, eliminations: 1 << 30})
			continue
		}
		if err := sol.AddAssignment(p, a); err != nil {
			rankedList = append(rankedList, ranked{cand: val, eliminations: 1 << 30})
			continue
		}
		elims := 0
		for i, dom := range domains {
			if i == selected {
				continue
			}
			if _, done := assigned[i]; done {
				continue
			}
			for _, other := range dom {
				if len(constraints.CheckAll(p, sol, buildCSPAssignment(decisions[i], other), checks)) > 0 {
					elims++
				}
			}
		}
		sol.RemoveLastAssignment(p)
		rankedList = append(rankedList, ranked{cand: val, eliminations: elims})
	}
	sort.SliceStable(rankedList, func(i, j int) bool {
		return rankedList[i].eliminations < rankedList[j].eliminations
	})
	res := make([]cspCandidate, len(rankedList))
	for i := range rankedList {
		res[i] = rankedList[i].cand
	}
	return res
}

func pruneCSPDomains(p *problem.Problem, decisions []cspDecision, domains map[int][]cspCandidate, assigned map[int]struct{}, sol *problem.Solution, checks []constraints.Constraint) (map[int][]cspCandidate, bool) {
	next := make(map[int][]cspCandidate, len(domains))
	for i, vals := range domains {
		if _, done := assigned[i]; done {
			continue
		}
		filtered := make([]cspCandidate, 0, len(vals))
		for _, v := range vals {
			if len(constraints.CheckAll(p, sol, buildCSPAssignment(decisions[i], v), checks)) == 0 {
				filtered = append(filtered, v)
			}
		}
		next[i] = filtered
		if len(filtered) == 0 {
			return next, false
		}
	}
	return next, true
}

// ----------------------------------------------------------------------------
// Test & Measure CSP Heuristics Combinations
// ----------------------------------------------------------------------------

func TestCSP_HeuristicInvestigation_Breakdown(t *testing.T) {
	fixtures := []struct {
		name string
		prob problem.Problem
	}{
		{"Small_24Sessions", GenerateSyntheticProblem(DefaultSmallProblemConfig())},
		{"Medium_300Sessions", GenerateSyntheticProblem(DefaultMediumProblemConfig())},
	}

	strategies := []HeuristicStrategy{
		StrategyBasic,
		StrategyMRVDegree,
		StrategyMRVDegreeFC,
		StrategyMRVDegreeLCV,
		StrategyFullHeuristic,
	}

	fmt.Println("\n================================================================================")
	fmt.Println("CSP HEURISTIC COST & BENEFIT INVESTIGATION")
	fmt.Println("================================================================================")
	fmt.Printf("%-20s | %-18s | %-12s | %-6s | %-6s | %-10s | %-10s\n",
		"Problem", "Strategy", "Time", "Nodes", "B-Tracks", "Alloc(KB)", "Allocs")
	fmt.Println("--------------------------------------------------------------------------------")

	for _, f := range fixtures {
		for _, strat := range strategies {
			// Skip basic on medium if too slow
			if f.name == "Medium_300Sessions" && (strat == StrategyBasic || strat == StrategyMRVDegreeLCV || strat == StrategyFullHeuristic) {
				// We run a fast test with bounded nodes
			}

			solver := &customCSPSolver{
				strategy:    strat,
				constraints: constraints.DefaultHardConstraints(),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			maxNodes := 10000
			if f.name == "Medium_300Sessions" {
				maxNodes = 5000
			}

			_, _, metrics := solver.SolveCustom(ctx, f.prob, maxNodes)
			cancel()

			fmt.Printf("%-20s | %-18s | %-12s | %-6d | %-6d | %-10.1f | %-10d\n",
				f.name,
				metrics.Strategy,
				metrics.SolveTime.String(),
				metrics.NodesExplored,
				metrics.Backtracks,
				float64(metrics.AllocBytes)/1024.0,
				metrics.AllocsCount,
			)

			if f.name == "Small_24Sessions" && !metrics.Success && metrics.Status != diagnostics.SolveStatusNodeLimit {
				t.Errorf("expected strategy %s to solve Small problem, got status %s", strat, metrics.Status)
			}
		}
		fmt.Println("--------------------------------------------------------------------------------")
	}
}
