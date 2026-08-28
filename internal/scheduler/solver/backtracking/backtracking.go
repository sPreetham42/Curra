package backtracking

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

var (
	ErrNoSolution     = errors.New("no feasible timetable found")
	ErrNodeLimit      = errors.New("search node limit reached")
	ErrInvalidProblem = errors.New("invalid scheduling problem")
)

type Solver struct {
	Constraints []constraints.Constraint
	Compiled    *constraints.CompiledConstraintSet
}

func New() *Solver {
	return &Solver{Constraints: constraints.DefaultHardConstraints()}
}

func NewWithCompiled(compiled *constraints.CompiledConstraintSet) *Solver {
	return &Solver{
		Constraints: constraints.DefaultHardConstraints(),
		Compiled:    compiled,
	}
}

func (s *Solver) Solve(ctx context.Context, p problem.Problem, options problem.SolveOptions) (problem.Solution, diagnostics.Diagnostics, error) {
	options = options.Normalize()
	diag := diagnostics.Diagnostics{}

	if violations := problem.Validate(p); len(violations) > 0 {
		diag.Status = diagnostics.SolveStatusInvalidProblem
		diag.Message = ErrInvalidProblem.Error()
		diag.Violations = violations
		return problem.Solution{}, diag, ErrInvalidProblem
	}

	if len(s.Constraints) == 0 {
		s.Constraints = constraints.DefaultHardConstraints()
	}

	p.Prepare()
	solution := problem.NewSolution()
	for _, locked := range p.LockedAssignments {
		if err := solution.AddAssignment(&p, locked); err != nil {
			diag.Status = diagnostics.SolveStatusInfeasible
			diag.Message = fmt.Sprintf("seed locked assignment: %v", err)
			return problem.Solution{}, diag, fmt.Errorf("seed locked assignment: %w", err)
		}
	}

	decisions := buildDecisions(&p)

	var err error
	if len(decisions) == 0 {
		err = nil
	} else if options.SearchMode == problem.SearchModeBasic {
		sortedRooms := sortedRoomIDs(&p)
		sortedSlots := sortedTimeSlotIDs(&p)
		err = s.searchBasic(ctx, &p, options, decisions, sortedRooms, sortedSlots, 0, &solution, &diag)
	} else {
		domains := buildInitialDomains(&p, decisions, &solution, s.Constraints, s.Compiled, &diag, options.ViolationLimit)
		roomConflicts := buildRoomConflictMap(&p, decisions)
		err = s.searchHeuristic(ctx, &p, options, decisions, domains, make(map[int]struct{}, len(decisions)), roomConflicts, &solution, &diag)
	}

	if err != nil {
		switch {
		case errors.Is(err, ErrNoSolution):
			diag.Status = diagnostics.SolveStatusInfeasible
			diag.Message = ErrNoSolution.Error()
		case errors.Is(err, ErrNodeLimit):
			diag.Status = diagnostics.SolveStatusNodeLimit
			diag.Message = ErrNodeLimit.Error()
		case errors.Is(err, context.Canceled):
			diag.Status = diagnostics.SolveStatusCancelled
			diag.Message = context.Canceled.Error()
		case errors.Is(err, context.DeadlineExceeded):
			diag.Status = diagnostics.SolveStatusDeadlineExceeded
			diag.Message = context.DeadlineExceeded.Error()
		}
		solution.Score.HardViolations = len(diag.Violations)
		return solution, diag, err
	}

	return s.ValidateSolution(ctx, p, solution)
}

// ValidateSolution executes the final compiled HARD constraint evaluation pipeline on a solution.
func (s *Solver) ValidateSolution(ctx context.Context, p problem.Problem, solution problem.Solution) (problem.Solution, diagnostics.Diagnostics, error) {
	diag := diagnostics.Diagnostics{}
	breakdown := p.StudentGapPenalty(&solution)

	var hardViolations []diagnostics.Violation
	if s.Compiled != nil && len(s.Compiled.Hard) > 0 {
		searchCtx := constraints.NewSearchCtx(&p)
		for _, c := range s.Compiled.Hard {
			hardViolations = append(hardViolations, c.Evaluate(searchCtx, &solution)...)
		}
	} else {
		for _, a := range solution.Assignments {
			hardViolations = append(hardViolations, constraints.CheckAll(&p, &solution, a, s.Constraints)...)
		}
	}

	if len(hardViolations) > 0 {
		solution.Score = scorer.Score{
			HardViolations: len(hardViolations),
			SoftPenalty:    breakdown.SoftPenalty,
			Breakdown:      breakdown,
		}
		diag.Status = diagnostics.SolveStatusInfeasible
		diag.Violations = hardViolations
		diag.Message = ErrNoSolution.Error()
		return solution, diag, ErrNoSolution
	}

	solution.Score = scorer.Score{
		HardViolations: 0,
		SoftPenalty:    breakdown.SoftPenalty,
		Breakdown:      breakdown,
	}

	diag.Status = diagnostics.SolveStatusSolved
	diag.Violations = nil
	diag.Message = "feasible timetable found"
	return solution, diag, nil
}

func (s *Solver) searchBasic(ctx context.Context, p *problem.Problem, options problem.SolveOptions, decisions []decision, sortedRooms []model.RoomID, sortedSlots []model.TimeSlotID, position int, solution *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if position == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= options.MaxNodes {
		diag.Message = ErrNodeLimit.Error()
		return ErrNodeLimit
	}
	diag.NodesExplored++

	current := decisions[position]

	for _, roomID := range sortedRooms {
		for _, slotID := range sortedSlots {
			slotIDs, ok := p.OccupiedSlotIDs(slotID, current.Requirement.Duration)
			if !ok {
				continue
			}
			diag.Candidates++
			assignment := problem.Assignment{
				ID:                   problem.NewAssignmentID(current.Requirement.ID, current.Instance),
				CourseOfferingID:     current.Offering.ID,
				StudentGroupID:       current.Offering.StudentGroupID,
				FacultyID:            current.Offering.FacultyID,
				RoomID:               roomID,
				TimeSlotID:           slotIDs[0],
				SessionRequirementID: current.Requirement.ID,
				Instance:             current.Instance,
			}

			violations := constraints.CheckAll(p, solution, assignment, s.Constraints)
			if len(violations) > 0 {
				for _, violation := range violations {
					diag.AddViolation(options.ViolationLimit, violation)
				}
				continue
			}

			if s.Compiled != nil && len(s.Compiled.Hard) > 0 {
				searchCtx := constraints.NewSearchCtx(p)
				inconsistent := false
				for _, c := range s.Compiled.Hard {
					if !c.IsConsistent(searchCtx, solution, assignment) {
						inconsistent = true
						for _, v := range c.ViolatedByMove(searchCtx, solution, problem.Move{AssignmentID: assignment.ID, To: problem.Placement{RoomID: assignment.RoomID, TimeSlotID: assignment.TimeSlotID}}) {
							diag.AddViolation(options.ViolationLimit, v)
						}
						break
					}
				}
				if inconsistent {
					continue
				}
			}

			if err := solution.AddAssignment(p, assignment); err != nil {
				return fmt.Errorf("index assignment: %w", err)
			}

			if err := s.searchBasic(ctx, p, options, decisions, sortedRooms, sortedSlots, position+1, solution, diag); err == nil {
				return nil
			} else if !errors.Is(err, ErrNoSolution) {
				return err
			}
			solution.RemoveLastAssignment(p)
		}
	}

	diag.Backtracks++
	return ErrNoSolution
}

func (s *Solver) searchHeuristic(ctx context.Context, p *problem.Problem, options problem.SolveOptions, decisions []decision, domains map[int][]candidate, assigned map[int]struct{}, roomConflicts map[int]map[int]struct{}, solution *problem.Solution, diag *diagnostics.Diagnostics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assigned) == len(decisions) {
		return nil
	}
	if diag.NodesExplored >= options.MaxNodes {
		diag.Message = ErrNodeLimit.Error()
		return ErrNodeLimit
	}

	selected, ok := selectMRVDegree(p, decisions, domains, assigned, roomConflicts)
	if !ok {
		diag.Backtracks++
		return ErrNoSolution
	}
	if len(domains[selected]) == 0 {
		diag.Backtracks++
		return ErrNoSolution
	}
	diag.NodesExplored++

	var values []candidate
	if options.SearchMode == problem.SearchModeHeuristicLCV {
		values = orderLCV(p, decisions, domains, assigned, selected, solution, s.Constraints, s.Compiled)
	} else {
		values = domains[selected]
	}
	for _, value := range values {
		diag.Candidates++
		assignment := buildAssignment(decisions[selected], value)
		violations := constraints.CheckAll(p, solution, assignment, s.Constraints)
		if len(violations) > 0 {
			for _, violation := range violations {
				diag.AddViolation(options.ViolationLimit, violation)
			}
			continue
		}

		if s.Compiled != nil && len(s.Compiled.Hard) > 0 {
			searchCtx := constraints.NewSearchCtx(p)
			inconsistent := false
			for _, c := range s.Compiled.Hard {
				if !c.IsConsistent(searchCtx, solution, assignment) {
					inconsistent = true
					for _, v := range c.ViolatedByMove(searchCtx, solution, problem.Move{AssignmentID: assignment.ID, To: problem.Placement{RoomID: assignment.RoomID, TimeSlotID: assignment.TimeSlotID}}) {
						diag.AddViolation(options.ViolationLimit, v)
					}
					break
				}
			}
			if inconsistent {
				continue
			}
		}

		if err := solution.AddAssignment(p, assignment); err != nil {
			return fmt.Errorf("index assignment: %w", err)
		}

		nextAssigned := copyAssigned(assigned)
		nextAssigned[selected] = struct{}{}
		nextDomains, viable := pruneDomains(p, decisions, domains, nextAssigned, solution, s.Constraints, s.Compiled)
		if viable {
			if err := s.searchHeuristic(ctx, p, options, decisions, nextDomains, nextAssigned, roomConflicts, solution, diag); err == nil {
				return nil
			} else if !errors.Is(err, ErrNoSolution) {
				return err
			}
		} else {
			diag.Backtracks++
		}
		solution.RemoveLastAssignment(p)
	}

	diag.Backtracks++
	return ErrNoSolution
}

type decision struct {
	Offering    model.CourseOffering
	Requirement model.SessionRequirement
	Instance    int
}

type candidate struct {
	RoomID     model.RoomID
	TimeSlotID model.TimeSlotID
}

func buildAssignment(current decision, value candidate) problem.Assignment {
	return problem.Assignment{
		ID:                   problem.NewAssignmentID(current.Requirement.ID, current.Instance),
		CourseOfferingID:     current.Offering.ID,
		StudentGroupID:       current.Offering.StudentGroupID,
		FacultyID:            current.Offering.FacultyID,
		RoomID:               value.RoomID,
		TimeSlotID:           value.TimeSlotID,
		SessionRequirementID: current.Requirement.ID,
		Instance:             current.Instance,
	}
}

func buildInitialDomains(p *problem.Problem, decisions []decision, solution *problem.Solution, checks []constraints.Constraint, compiled *constraints.CompiledConstraintSet, diag *diagnostics.Diagnostics, violationLimit int) map[int][]candidate {
	domains := make(map[int][]candidate, len(decisions))
	rooms := sortedRoomIDs(p)
	slots := sortedTimeSlotIDs(p)

	for i, current := range decisions {
		for _, roomID := range rooms {
			for _, slotID := range slots {
				slotIDs, ok := p.OccupiedSlotIDs(slotID, current.Requirement.Duration)
				if !ok {
					continue
				}
				value := candidate{RoomID: roomID, TimeSlotID: slotIDs[0]}
				a := buildAssignment(current, value)
				if !isAssignmentConsistent(p, solution, a, checks, compiled) {
					if diag != nil {
						violations := constraints.CheckAll(p, solution, a, checks)
						for _, violation := range violations {
							diag.AddViolation(violationLimit, violation)
						}
					}
					continue
				}
				domains[i] = append(domains[i], value)
			}
		}
	}
	return domains
}

func selectMRVDegree(p *problem.Problem, decisions []decision, domains map[int][]candidate, assigned map[int]struct{}, roomConflictMap map[int]map[int]struct{}) (int, bool) {
	best := -1
	bestDomain := 0
	bestDegree := -1
	for i := range decisions {
		if _, done := assigned[i]; done {
			continue
		}
		size := len(domains[i])
		degree := degreeFor(p, decisions, assigned, i, roomConflictMap)
		if best == -1 || size < bestDomain || (size == bestDomain && degree > bestDegree) || (size == bestDomain && degree == bestDegree && decisionLess(decisions[i], decisions[best])) {
			best = i
			bestDomain = size
			bestDegree = degree
		}
	}
	return best, best != -1
}

func degreeFor(p *problem.Problem, decisions []decision, assigned map[int]struct{}, selected int, roomConflictMap map[int]map[int]struct{}) int {
	degree := 0
	left := decisions[selected]
	for i, right := range decisions {
		if i == selected {
			continue
		}
		if _, done := assigned[i]; done {
			continue
		}
		if left.Offering.FacultyID == right.Offering.FacultyID || p.StudentGroupsOverlap(left.Offering.StudentGroupID, right.Offering.StudentGroupID) {
			degree++
		} else if roomConflictMap != nil {
			if _, shares := roomConflictMap[selected][i]; shares {
				degree++
			}
		}
	}
	return degree
}

func orderLCV(p *problem.Problem, decisions []decision, domains map[int][]candidate, assigned map[int]struct{}, selected int, solution *problem.Solution, checks []constraints.Constraint, compiled *constraints.CompiledConstraintSet) []candidate {
	values := append([]candidate(nil), domains[selected]...)
	type ranked struct {
		value        candidate
		eliminations int
	}
	rankedValues := make([]ranked, 0, len(values))
	for _, value := range values {
		eliminations := countEliminations(p, decisions, domains, assigned, selected, value, solution, checks, compiled)
		rankedValues = append(rankedValues, ranked{value: value, eliminations: eliminations})
	}
	sort.SliceStable(rankedValues, func(i, j int) bool {
		if rankedValues[i].eliminations != rankedValues[j].eliminations {
			return rankedValues[i].eliminations < rankedValues[j].eliminations
		}
		if rankedValues[i].value.RoomID != rankedValues[j].value.RoomID {
			return rankedValues[i].value.RoomID < rankedValues[j].value.RoomID
		}
		return rankedValues[i].value.TimeSlotID < rankedValues[j].value.TimeSlotID
	})
	for i := range rankedValues {
		values[i] = rankedValues[i].value
	}
	return values
}

func countEliminations(p *problem.Problem, decisions []decision, domains map[int][]candidate, assigned map[int]struct{}, selected int, value candidate, solution *problem.Solution, checks []constraints.Constraint, compiled *constraints.CompiledConstraintSet) int {
	assignment := buildAssignment(decisions[selected], value)
	if !isAssignmentConsistent(p, solution, assignment, checks, compiled) {
		return 1 << 30
	}
	if err := solution.AddAssignment(p, assignment); err != nil {
		return 1 << 30
	}
	defer solution.RemoveLastAssignment(p)

	eliminations := 0
	for i, values := range domains {
		if i == selected {
			continue
		}
		if _, done := assigned[i]; done {
			continue
		}
		for _, other := range values {
			if !isAssignmentConsistent(p, solution, buildAssignment(decisions[i], other), checks, compiled) {
				eliminations++
			}
		}
	}
	return eliminations
}

func pruneDomains(p *problem.Problem, decisions []decision, domains map[int][]candidate, assigned map[int]struct{}, solution *problem.Solution, checks []constraints.Constraint, compiled *constraints.CompiledConstraintSet) (map[int][]candidate, bool) {
	next := make(map[int][]candidate, len(domains))
	for i, values := range domains {
		if _, done := assigned[i]; done {
			continue
		}
		filtered := make([]candidate, 0, len(values))
		for _, value := range values {
			if isAssignmentConsistent(p, solution, buildAssignment(decisions[i], value), checks, compiled) {
				filtered = append(filtered, value)
			}
		}
		next[i] = filtered
		if len(filtered) == 0 {
			return next, false
		}
	}
	return next, true
}

func copyAssigned(assigned map[int]struct{}) map[int]struct{} {
	copied := make(map[int]struct{}, len(assigned)+1)
	for key := range assigned {
		copied[key] = struct{}{}
	}
	return copied
}

// buildRoomConflictMap precomputes which decisions compete for rooms requiring
// the same features. Two decisions conflict if they both need rooms with
// overlapping feature requirements (e.g., both need a lab).
func buildRoomConflictMap(p *problem.Problem, decisions []decision) map[int]map[int]struct{} {
	if len(decisions) == 0 {
		return nil
	}
	result := make(map[int]map[int]struct{}, len(decisions))
	for i := range decisions {
		result[i] = make(map[int]struct{})
	}
	// Precompute required features per decision
	decisionFeatures := make([][]model.RoomFeatureID, len(decisions))
	for i, d := range decisions {
		decisionFeatures[i] = p.RequiredRoomFeatures(d.Offering.ID, d.Requirement.ID)
	}
	for i := 0; i < len(decisions); i++ {
		for j := i + 1; j < len(decisions); j++ {
			if decisions[i].Offering.FacultyID == decisions[j].Offering.FacultyID {
				continue // already counted by faculty degree
			}
			if p.StudentGroupsOverlap(decisions[i].Offering.StudentGroupID, decisions[j].Offering.StudentGroupID) {
				continue // already counted by group degree
			}
			// Two decisions compete for rooms if they have overlapping feature requirements
			if featuresOverlap(decisionFeatures[i], decisionFeatures[j]) {
				result[i][j] = struct{}{}
				result[j][i] = struct{}{}
			}
		}
	}
	return result
}

func featuresOverlap(a, b []model.RoomFeatureID) bool {
	if len(a) == 0 && len(b) == 0 {
		return true // both need any room
	}
	if len(a) == 0 || len(b) == 0 {
		return false // one needs specific features, other doesn't
	}
	// Check if there's a room that satisfies both
	// If both need the same features, they overlap
	set := make(map[model.RoomFeatureID]struct{}, len(a))
	for _, f := range a {
		set[f] = struct{}{}
	}
	for _, f := range b {
		if _, ok := set[f]; ok {
			return true
		}
	}
	return false
}

// isAssignmentConsistent checks whether a candidate assignment satisfies all
// active hard constraints (both legacy and compiled). Returns true if no
// constraint is violated.
func isAssignmentConsistent(p *problem.Problem, solution *problem.Solution, a problem.Assignment, legacy []constraints.Constraint, compiled *constraints.CompiledConstraintSet) bool {
	// Check legacy constraints
	if len(constraints.CheckAll(p, solution, a, legacy)) > 0 {
		return false
	}
	// Check compiled constraints
	if compiled != nil && len(compiled.Hard) > 0 {
		searchCtx := constraints.NewSearchCtx(p)
		for _, c := range compiled.Hard {
			if !c.IsConsistent(searchCtx, solution, a) {
				return false
			}
		}
	}
	return true
}

func decisionLess(left decision, right decision) bool {
	if left.Requirement.Duration != right.Requirement.Duration {
		return left.Requirement.Duration > right.Requirement.Duration
	}
	if left.Offering.ID != right.Offering.ID {
		return left.Offering.ID < right.Offering.ID
	}
	if left.Requirement.ID != right.Requirement.ID {
		return left.Requirement.ID < right.Requirement.ID
	}
	return left.Instance < right.Instance
}

func buildDecisions(p *problem.Problem) []decision {
	offerings := sortedOfferings(p)
	decisions := make([]decision, 0)

	lockedCounts := make(map[model.SessionRequirementID]int)
	for _, locked := range p.LockedAssignments {
		lockedCounts[locked.SessionRequirementID]++
	}

	for _, offering := range offerings {
		requirements := requirementsForOffering(p, offering)
		for _, requirement := range requirements {
			locked := lockedCounts[requirement.ID]
			for instance := locked; instance < requirement.SessionsPerWeek; instance++ {
				decisions = append(decisions, decision{
					Offering:    offering,
					Requirement: requirement,
					Instance:    instance,
				})
			}
		}
	}

	sort.SliceStable(decisions, func(i, j int) bool {
		return decisionLess(decisions[i], decisions[j])
	})

	return decisions
}

func sortedOfferings(p *problem.Problem) []model.CourseOffering {
	ids := make([]string, 0, len(p.CourseOfferings))
	for id := range p.CourseOfferings {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	offerings := make([]model.CourseOffering, 0, len(ids))
	for _, id := range ids {
		offerings = append(offerings, p.CourseOfferings[model.CourseOfferingID(id)])
	}
	return offerings
}

func requirementsForOffering(p *problem.Problem, offering model.CourseOffering) []model.SessionRequirement {
	requirements := make([]model.SessionRequirement, 0, len(offering.SessionRequirementIDs))
	for _, id := range offering.SessionRequirementIDs {
		if requirement, ok := p.SessionRequirements[id]; ok {
			requirements = append(requirements, requirement)
		}
	}
	if len(requirements) == 0 {
		for _, requirement := range p.SessionRequirements {
			if requirement.CourseOfferingID == offering.ID {
				requirements = append(requirements, requirement)
			}
		}
	}
	sort.Slice(requirements, func(i, j int) bool {
		return requirements[i].ID < requirements[j].ID
	})
	return requirements
}

func sortedRoomIDs(p *problem.Problem) []model.RoomID {
	ids := make([]string, 0, len(p.Rooms))
	for id := range p.Rooms {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	roomIDs := make([]model.RoomID, 0, len(ids))
	for _, id := range ids {
		roomIDs = append(roomIDs, model.RoomID(id))
	}
	return roomIDs
}

func sortedTimeSlotIDs(p *problem.Problem) []model.TimeSlotID {
	slots := make([]model.TimeSlot, 0, len(p.TimeSlots))
	for _, slot := range p.TimeSlots {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Day != slots[j].Day {
			return slots[i].Day < slots[j].Day
		}
		if slots[i].Period != slots[j].Period {
			return slots[i].Period < slots[j].Period
		}
		return slots[i].ID < slots[j].ID
	})

	ids := make([]model.TimeSlotID, 0, len(slots))
	for _, slot := range slots {
		ids = append(ids, slot.ID)
	}
	return ids
}
