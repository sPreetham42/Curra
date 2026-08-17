package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

func TestBacktrackingSolverFindsFeasibleSolution(t *testing.T) {
	p := feasibleProblem()
	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve returned error: %v\nDiagnostics: %+v", err, diag)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusSolved)
	}
	if got, want := len(solution.Assignments), 2; got != want {
		t.Fatalf("assignment count = %d, want %d", got, want)
	}
	if got := solution.Index.ScheduledCount("req-theory"); got != 1 {
		t.Fatalf("theory scheduled count = %d, want 1", got)
	}
	if got := solution.Index.ScheduledCount("req-lab"); got != 1 {
		t.Fatalf("lab scheduled count = %d, want 1", got)
	}

	prepared := feasibleProblem()
	prepared.Prepare()
	var lab problem.Assignment
	for _, assignment := range solution.Assignments {
		if assignment.SessionRequirementID == "req-lab" {
			lab = assignment
			break
		}
	}
	if lab.ID == "" {
		t.Fatal("lab assignment not found")
	}
	occupied, ok := lab.OccupiedSlotIDs(&prepared)
	if !ok {
		t.Fatal("lab assignment did not fit in the slot grid")
	}
	if got, want := len(occupied), 2; got != want {
		t.Fatalf("lab occupied slots = %d, want %d", got, want)
	}
	assertAllRequiredSessionsScheduled(t, p, solution)
}

func TestBacktrackingSolverReturnsCapacityDiagnosticsWhenUnsatisfied(t *testing.T) {
	p := feasibleProblem()
	p.StudentGroups["group-a"] = model.StudentGroup{ID: "group-a", ClassID: "class-a", Name: "A", Size: 120}
	p.Rooms = map[model.RoomID]model.Room{
		"room-small": {ID: "room-small", Name: "Small", Capacity: 20},
	}
	p.RoomAvailabilities = []model.RoomAvailability{
		{RoomID: "room-small", TimeSlotID: "mon-1"},
		{RoomID: "room-small", TimeSlotID: "mon-2"},
		{RoomID: "room-small", TimeSlotID: "mon-3"},
		{RoomID: "room-small", TimeSlotID: "tue-1"},
		{RoomID: "room-small", TimeSlotID: "tue-2"},
	}
	p.RoomAvailable = nil

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 1000})
	if !errors.Is(err, backtracking.ErrNoSolution) {
		t.Fatalf("Solve error = %v, want ErrNoSolution", err)
	}
	if diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusInfeasible)
	}
	if !hasViolation(diag.Violations, "RoomCapacity") {
		t.Fatalf("expected RoomCapacity violation, got %+v", diag.Violations)
	}
}

func TestBacktrackingSolverReportsCancelledStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, diag, err := backtracking.New().Solve(ctx, feasibleProblem(), problem.SolveOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Solve error = %v, want context.Canceled", err)
	}
	if diag.Status != diagnostics.SolveStatusCancelled {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusCancelled)
	}
}

func TestBacktrackingSolverReportsDeadlineExceededStatus(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	defer cancel()

	_, diag, err := backtracking.New().Solve(ctx, feasibleProblem(), problem.SolveOptions{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Solve error = %v, want context.DeadlineExceeded", err)
	}
	if diag.Status != diagnostics.SolveStatusDeadlineExceeded {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusDeadlineExceeded)
	}
}

func TestBacktrackingSolverReportsNodeLimitStatus(t *testing.T) {
	_, diag, err := backtracking.New().Solve(context.Background(), feasibleProblem(), problem.SolveOptions{MaxNodes: 1, SearchMode: problem.SearchModeBasic})
	if !errors.Is(err, backtracking.ErrNodeLimit) {
		t.Fatalf("Solve error = %v, want ErrNodeLimit", err)
	}
	if diag.Status != diagnostics.SolveStatusNodeLimit {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusNodeLimit)
	}
}

func TestBacktrackingSolverReportsInvalidProblemStatus(t *testing.T) {
	p := feasibleProblem()
	p.SessionRequirements["req-theory"] = model.SessionRequirement{
		ID:               "req-theory",
		CourseOfferingID: "offering-theory",
		Type:             model.SessionTypeTheory,
		SessionsPerWeek:  0,
		Duration:         1,
		Consecutive:      true,
	}

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if !errors.Is(err, backtracking.ErrInvalidProblem) {
		t.Fatalf("Solve error = %v, want ErrInvalidProblem", err)
	}
	if diag.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusInvalidProblem)
	}
}

func TestProblemValidationRejectsInvalidStructure(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*problem.Problem)
		wantPhrase string
	}{
		{
			name: "invalid sessions per week",
			mutate: func(p *problem.Problem) {
				req := p.SessionRequirements["req-theory"]
				req.SessionsPerWeek = 0
				p.SessionRequirements["req-theory"] = req
			},
			wantPhrase: "non-positive sessions per week",
		},
		{
			name: "invalid duration",
			mutate: func(p *problem.Problem) {
				req := p.SessionRequirements["req-theory"]
				req.Duration = 0
				p.SessionRequirements["req-theory"] = req
			},
			wantPhrase: "non-positive duration",
		},
		{
			name: "missing requirement ID",
			mutate: func(p *problem.Problem) {
				offering := p.CourseOfferings["offering-theory"]
				offering.SessionRequirementIDs = []model.SessionRequirementID{"req-missing"}
				p.CourseOfferings["offering-theory"] = offering
			},
			wantPhrase: "references missing session requirement",
		},
		{
			name: "course offering class mismatch",
			mutate: func(p *problem.Problem) {
				p.Classes["class-b"] = model.Class{ID: "class-b", ProgramID: "program-a", Name: "B", WholeGroupID: "group-b", StudentGroupIDs: []model.StudentGroupID{"group-b"}}
				p.StudentGroups["group-b"] = model.StudentGroup{ID: "group-b", ClassID: "class-b", Name: "B", Size: 30}
				offering := p.CourseOfferings["offering-theory"]
				offering.ClassID = "class-b"
				p.CourseOfferings["offering-theory"] = offering
			},
			wantPhrase: "student group belongs to a different class",
		},
		{
			name: "course offering student group mismatch",
			mutate: func(p *problem.Problem) {
				p.StudentGroups["group-b"] = model.StudentGroup{ID: "group-b", ClassID: "class-a", Name: "B", Size: 30}
				offering := p.CourseOfferings["offering-theory"]
				offering.StudentGroupID = "group-b"
				p.CourseOfferings["offering-theory"] = offering
			},
			wantPhrase: "student group is not listed on its class",
		},
		{
			name: "student group class mismatch",
			mutate: func(p *problem.Problem) {
				group := p.StudentGroups["group-a"]
				group.ClassID = "class-missing"
				p.StudentGroups["group-a"] = group
			},
			wantPhrase: "student group references missing class",
		},
		{
			name: "duplicate time slot",
			mutate: func(p *problem.Problem) {
				p.TimeSlots["mon-1-dup"] = model.TimeSlot{ID: "mon-1-dup", Day: time.Monday, Period: 1, Label: "Mon P1 duplicate"}
			},
			wantPhrase: "duplicate day/period time slot",
		},
		{
			name: "missing referenced entities",
			mutate: func(p *problem.Problem) {
				delete(p.Faculty, "faculty-a")
				delete(p.RoomFeatures, "feature-lab")
				p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: "room-missing", TimeSlotID: "slot-missing"})
			},
			wantPhrase: "references missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := feasibleProblem()
			tt.mutate(&p)
			violations := problem.Validate(p)
			if len(violations) == 0 {
				t.Fatal("expected validation violations")
			}
			if !hasViolationMessageContaining(violations, tt.wantPhrase) {
				t.Fatalf("expected validation message containing %q, got %+v", tt.wantPhrase, violations)
			}

			_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
			if !errors.Is(err, backtracking.ErrInvalidProblem) {
				t.Fatalf("Solve error = %v, want ErrInvalidProblem", err)
			}
			if diag.Status != diagnostics.SolveStatusInvalidProblem {
				t.Fatalf("status = %s, want %s", diag.Status, diagnostics.SolveStatusInvalidProblem)
			}
		})
	}
}

func TestSolvedTimetableInvariantForDeterministicFeasibleProblems(t *testing.T) {
	problems := []problem.Problem{
		feasibleProblem(),
		feasibleProblemWithExtraTheorySession(),
		feasibleProblemWithLabOnlyRoomFirst(),
	}

	for i, p := range problems {
		solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
		if err != nil {
			t.Fatalf("case %d Solve returned error: %v\nDiagnostics: %+v", i, err, diag)
		}
		if diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("case %d status = %s, want %s", i, diag.Status, diagnostics.SolveStatusSolved)
		}
		assertAllRequiredSessionsScheduled(t, p, solution)
		assertNoHardConstraintViolations(t, p, solution)
	}
}

func TestStudentGroupOverlapRules(t *testing.T) {
	p := overlapProblem()
	p.Prepare()

	if !p.StudentGroupsOverlap("class-a-whole", "class-a-lab-1") {
		t.Fatal("whole group should overlap subgroup")
	}
	if !p.StudentGroupsOverlap("class-a-lab-1", "class-a-lab-1") {
		t.Fatal("same subgroup should overlap itself")
	}
	if p.StudentGroupsOverlap("class-a-lab-1", "class-a-lab-2") {
		t.Fatal("disjoint subgroups should not overlap")
	}
	if p.StudentGroupsOverlap("class-a-lab-1", "class-b-whole") {
		t.Fatal("groups from different classes should not overlap")
	}
}

func TestDisjointSubgroupsMayRunTogetherButWholeGroupConflicts(t *testing.T) {
	p := overlapProblem()
	p.Prepare()
	solution := problem.NewSolution()

	first := problem.Assignment{ID: "a1", CourseOfferingID: "offering-a-lab-1", StudentGroupID: "class-a-lab-1", FacultyID: "faculty-a1", RoomID: "room-lab-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-lab-1"}
	secondDisjoint := problem.Assignment{ID: "a2", CourseOfferingID: "offering-a-lab-2", StudentGroupID: "class-a-lab-2", FacultyID: "faculty-a2", RoomID: "room-lab-2", TimeSlotID: "mon-1", SessionRequirementID: "req-a-lab-2"}
	whole := problem.Assignment{ID: "a3", CourseOfferingID: "offering-a-whole", StudentGroupID: "class-a-whole", FacultyID: "faculty-a3", RoomID: "room-lecture", TimeSlotID: "mon-1", SessionRequirementID: "req-a-whole"}

	if err := solution.AddAssignment(&p, first); err != nil {
		t.Fatalf("first assignment: %v", err)
	}
	if violations := constraints.CheckAll(&p, &solution, secondDisjoint, constraints.DefaultHardConstraints()); len(violations) != 0 {
		t.Fatalf("disjoint subgroup should be allowed, got %+v", violations)
	}
	if err := solution.AddAssignment(&p, secondDisjoint); err != nil {
		t.Fatalf("disjoint assignment: %v", err)
	}
	if violations := constraints.CheckAll(&p, &solution, whole, constraints.DefaultHardConstraints()); !hasViolation(violations, "StudentGroupConflict") {
		t.Fatalf("whole group should conflict with subgroup, got %+v", violations)
	}
}

func feasibleProblem() problem.Problem {
	groupID := model.StudentGroupID("group-a")
	facultyID := model.FacultyID("faculty-a")
	labFeatureID := model.RoomFeatureID("feature-lab")

	return problem.Problem{
		TenantID: "tenant-a",
		Term: model.Term{
			ID:       "term-a",
			TenantID: "tenant-a",
			Name:     "Term A",
		},
		Departments: map[model.DepartmentID]model.Department{
			"dept-a": {ID: "dept-a", TenantID: "tenant-a", Name: "Engineering"},
		},
		Programs: map[model.ProgramID]model.Program{
			"program-a": {ID: "program-a", DepartmentID: "dept-a", Name: "B.Tech"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-a": {
				ID:              "class-a",
				ProgramID:       "program-a",
				Name:            "A",
				WholeGroupID:    groupID,
				StudentGroupIDs: []model.StudentGroupID{groupID},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			groupID: {ID: groupID, ClassID: "class-a", Name: "A", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subject-theory": {ID: "subject-theory", Code: "T101", Name: "Theory"},
			"subject-lab":    {ID: "subject-lab", Code: "L101", Name: "Lab"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"offering-theory": {
				ID:                    "offering-theory",
				TermID:                "term-a",
				ClassID:               "class-a",
				SubjectID:             "subject-theory",
				StudentGroupID:        groupID,
				FacultyID:             facultyID,
				SessionRequirementIDs: []model.SessionRequirementID{"req-theory"},
			},
			"offering-lab": {
				ID:                    "offering-lab",
				TermID:                "term-a",
				ClassID:               "class-a",
				SubjectID:             "subject-lab",
				StudentGroupID:        groupID,
				FacultyID:             facultyID,
				SessionRequirementIDs: []model.SessionRequirementID{"req-lab"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-theory": {
				ID:               "req-theory",
				CourseOfferingID: "offering-theory",
				Type:             model.SessionTypeTheory,
				SessionsPerWeek:  1,
				Duration:         1,
				Consecutive:      true,
			},
			"req-lab": {
				ID:                     "req-lab",
				CourseOfferingID:       "offering-lab",
				Type:                   model.SessionTypeLab,
				SessionsPerWeek:        1,
				Duration:               2,
				Consecutive:            true,
				RequiredRoomFeatureIDs: []model.RoomFeatureID{labFeatureID},
			},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			facultyID: {ID: facultyID, Name: "Faculty A"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-lecture": {ID: "room-lecture", Name: "Lecture", Capacity: 60},
			"room-lab":     {ID: "room-lab", Name: "Lab", Capacity: 40, FeatureIDs: []model.RoomFeatureID{labFeatureID}},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"mon-3": {ID: "mon-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"tue-1": {ID: "tue-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"tue-2": {ID: "tue-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
		},
		FacultyAvailabilities: availabilityForFaculty(facultyID),
		RoomAvailabilities:    availabilityForRooms("room-lecture", "room-lab"),
		PeriodsPerDay:         3,
	}
}

func feasibleProblemWithExtraTheorySession() problem.Problem {
	p := feasibleProblem()
	req := p.SessionRequirements["req-theory"]
	req.SessionsPerWeek = 2
	p.SessionRequirements["req-theory"] = req
	return p
}

func feasibleProblemWithLabOnlyRoomFirst() problem.Problem {
	p := feasibleProblem()
	delete(p.Rooms, "room-lecture")
	p.RoomAvailabilities = availabilityForRooms("room-lab")
	return p
}

func overlapProblem() problem.Problem {
	p := feasibleProblem()
	p.Classes = map[model.ClassID]model.Class{
		"class-a": {
			ID:              "class-a",
			ProgramID:       "program-a",
			Name:            "A",
			WholeGroupID:    "class-a-whole",
			StudentGroupIDs: []model.StudentGroupID{"class-a-whole", "class-a-lab-1", "class-a-lab-2"},
		},
		"class-b": {
			ID:              "class-b",
			ProgramID:       "program-a",
			Name:            "B",
			WholeGroupID:    "class-b-whole",
			StudentGroupIDs: []model.StudentGroupID{"class-b-whole"},
		},
	}
	p.StudentGroups = map[model.StudentGroupID]model.StudentGroup{
		"class-a-whole": {ID: "class-a-whole", ClassID: "class-a", Name: "A whole", Size: 40},
		"class-a-lab-1": {ID: "class-a-lab-1", ClassID: "class-a", Name: "A lab 1", Size: 20},
		"class-a-lab-2": {ID: "class-a-lab-2", ClassID: "class-a", Name: "A lab 2", Size: 20},
		"class-b-whole": {ID: "class-b-whole", ClassID: "class-b", Name: "B whole", Size: 30},
	}
	p.Faculty["faculty-a1"] = model.Faculty{ID: "faculty-a1", Name: "Faculty A1"}
	p.Faculty["faculty-a2"] = model.Faculty{ID: "faculty-a2", Name: "Faculty A2"}
	p.Faculty["faculty-a3"] = model.Faculty{ID: "faculty-a3", Name: "Faculty A3"}
	p.Rooms["room-lab-1"] = model.Room{ID: "room-lab-1", Name: "Lab 1", Capacity: 30, FeatureIDs: []model.RoomFeatureID{"feature-lab"}}
	p.Rooms["room-lab-2"] = model.Room{ID: "room-lab-2", Name: "Lab 2", Capacity: 30, FeatureIDs: []model.RoomFeatureID{"feature-lab"}}
	p.Rooms["room-lecture"] = model.Room{ID: "room-lecture", Name: "Lecture", Capacity: 80}
	p.CourseOfferings = map[model.CourseOfferingID]model.CourseOffering{
		"offering-a-lab-1": {ID: "offering-a-lab-1", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-lab", StudentGroupID: "class-a-lab-1", FacultyID: "faculty-a1", SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab-1"}},
		"offering-a-lab-2": {ID: "offering-a-lab-2", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-lab", StudentGroupID: "class-a-lab-2", FacultyID: "faculty-a2", SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab-2"}},
		"offering-a-whole": {ID: "offering-a-whole", TermID: "term-a", ClassID: "class-a", SubjectID: "subject-theory", StudentGroupID: "class-a-whole", FacultyID: "faculty-a3", SessionRequirementIDs: []model.SessionRequirementID{"req-a-whole"}},
	}
	p.SessionRequirements = map[model.SessionRequirementID]model.SessionRequirement{
		"req-a-lab-1": {ID: "req-a-lab-1", CourseOfferingID: "offering-a-lab-1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{"feature-lab"}},
		"req-a-lab-2": {ID: "req-a-lab-2", CourseOfferingID: "offering-a-lab-2", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{"feature-lab"}},
		"req-a-whole": {ID: "req-a-whole", CourseOfferingID: "offering-a-whole", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1, Consecutive: true},
	}
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, availabilityForFaculty("faculty-a1")...)
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, availabilityForFaculty("faculty-a2")...)
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, availabilityForFaculty("faculty-a3")...)
	p.RoomAvailabilities = availabilityForRooms("room-lab-1", "room-lab-2", "room-lecture")
	p.FacultyAvailable = nil
	p.RoomAvailable = nil
	return p
}

func availabilityForFaculty(facultyID model.FacultyID) []model.FacultyAvailability {
	slotIDs := []model.TimeSlotID{"mon-1", "mon-2", "mon-3", "tue-1", "tue-2"}
	availability := make([]model.FacultyAvailability, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		availability = append(availability, model.FacultyAvailability{FacultyID: facultyID, TimeSlotID: slotID})
	}
	return availability
}

func availabilityForRooms(roomIDs ...model.RoomID) []model.RoomAvailability {
	slotIDs := []model.TimeSlotID{"mon-1", "mon-2", "mon-3", "tue-1", "tue-2"}
	availability := make([]model.RoomAvailability, 0, len(roomIDs)*len(slotIDs))
	for _, roomID := range roomIDs {
		for _, slotID := range slotIDs {
			availability = append(availability, model.RoomAvailability{RoomID: roomID, TimeSlotID: slotID})
		}
	}
	return availability
}

func hasViolation(violations []diagnostics.Violation, name string) bool {
	for _, violation := range violations {
		if violation.ConstraintName == name {
			return true
		}
	}
	return false
}

func hasViolationMessageContaining(violations []diagnostics.Violation, phrase string) bool {
	for _, violation := range violations {
		if contains(violation.Message, phrase) {
			return true
		}
	}
	return false
}

func contains(s string, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func assertAllRequiredSessionsScheduled(t *testing.T, p problem.Problem, solution problem.Solution) {
	t.Helper()
	expected := 0
	for _, offering := range p.CourseOfferings {
		for _, requirementID := range offering.SessionRequirementIDs {
			requirement := p.SessionRequirements[requirementID]
			expected += requirement.SessionsPerWeek
			if got := solution.Index.ScheduledCount(requirementID); got != requirement.SessionsPerWeek {
				t.Fatalf("scheduled count for %s = %d, want %d", requirementID, got, requirement.SessionsPerWeek)
			}
		}
	}
	if got := len(solution.Assignments); got != expected {
		t.Fatalf("assignment count = %d, want %d", got, expected)
	}
}

func assertNoHardConstraintViolations(t *testing.T, p problem.Problem, solution problem.Solution) {
	t.Helper()
	p.Prepare()
	partial := problem.NewSolution()
	for _, assignment := range solution.Assignments {
		violations := constraints.CheckAll(&p, &partial, assignment, constraints.DefaultHardConstraints())
		if len(violations) > 0 {
			t.Fatalf("assignment %s violates hard constraints: %+v", assignment.ID, violations)
		}
		if err := partial.AddAssignment(&p, assignment); err != nil {
			t.Fatalf("failed to index assignment %s: %v", assignment.ID, err)
		}
	}
}
