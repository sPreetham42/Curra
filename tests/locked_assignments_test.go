package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

func lockedBaseProblem() problem.Problem {
	labFeatureID := model.RoomFeatureID("feature-lab")
	p := problem.Problem{
		TenantID: "tenant-locked",
		Term:     model.Term{ID: "term-locked", TenantID: "tenant-locked", Name: "Locked Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-locked", Name: "Dept 1"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "Prog 1"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-a": {
				ID:              "class-a",
				ProgramID:       "prog-1",
				Name:            "Class A",
				WholeGroupID:    "group-a-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-a-whole", "group-a-lab1", "group-a-lab2"},
			},
			"class-b": {
				ID:              "class-b",
				ProgramID:       "prog-1",
				Name:            "Class B",
				WholeGroupID:    "group-b-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-b-whole"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-a-whole": {ID: "group-a-whole", ClassID: "class-a", Name: "A Whole", Size: 40},
			"group-a-lab1":  {ID: "group-a-lab1", ClassID: "class-a", Name: "A Lab 1", Size: 20},
			"group-a-lab2":  {ID: "group-a-lab2", ClassID: "class-a", Name: "A Lab 2", Size: 20},
			"group-b-whole": {ID: "group-b-whole", ClassID: "class-b", Name: "B Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-theory": {ID: "subj-theory", Code: "T101", Name: "Theory"},
			"subj-lab":    {ID: "subj-lab", Code: "L101", Name: "Lab"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"offering-a-theory": {
				ID:                    "offering-a-theory",
				TermID:                "term-locked",
				ClassID:               "class-a",
				SubjectID:             "subj-theory",
				StudentGroupID:        "group-a-whole",
				FacultyID:             "faculty-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-theory"},
			},
			"offering-a-lab1": {
				ID:                    "offering-a-lab1",
				TermID:                "term-locked",
				ClassID:               "class-a",
				SubjectID:             "subj-lab",
				StudentGroupID:        "group-a-lab1",
				FacultyID:             "faculty-2",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab1"},
			},
			"offering-b-theory": {
				ID:                    "offering-b-theory",
				TermID:                "term-locked",
				ClassID:               "class-b",
				SubjectID:             "subj-theory",
				StudentGroupID:        "group-b-whole",
				FacultyID:             "faculty-3",
				SessionRequirementIDs: []model.SessionRequirementID{"req-b-theory"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-a-theory": {ID: "req-a-theory", CourseOfferingID: "offering-a-theory", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1, Consecutive: true},
			"req-a-lab1":   {ID: "req-a-lab1", CourseOfferingID: "offering-a-lab1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2, Consecutive: true, RequiredRoomFeatureIDs: []model.RoomFeatureID{labFeatureID}},
			"req-b-theory": {ID: "req-b-theory", CourseOfferingID: "offering-b-theory", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"faculty-1": {ID: "faculty-1", Name: "Faculty 1"},
			"faculty-2": {ID: "faculty-2", Name: "Faculty 2"},
			"faculty-3": {ID: "faculty-3", Name: "Faculty 3"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-lecture-1": {ID: "room-lecture-1", Name: "Lecture 1", Capacity: 60},
			"room-lecture-2": {ID: "room-lecture-2", Name: "Lecture 2", Capacity: 60},
			"room-lab-1":     {ID: "room-lab-1", Name: "Lab 1", Capacity: 30, FeatureIDs: []model.RoomFeatureID{labFeatureID}},
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
		PeriodsPerDay: 3,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "faculty-1", TimeSlotID: "mon-1"}, {FacultyID: "faculty-1", TimeSlotID: "mon-2"}, {FacultyID: "faculty-1", TimeSlotID: "mon-3"}, {FacultyID: "faculty-1", TimeSlotID: "tue-1"}, {FacultyID: "faculty-1", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-2", TimeSlotID: "mon-1"}, {FacultyID: "faculty-2", TimeSlotID: "mon-2"}, {FacultyID: "faculty-2", TimeSlotID: "mon-3"}, {FacultyID: "faculty-2", TimeSlotID: "tue-1"}, {FacultyID: "faculty-2", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-3", TimeSlotID: "mon-1"}, {FacultyID: "faculty-3", TimeSlotID: "mon-2"}, {FacultyID: "faculty-3", TimeSlotID: "mon-3"}, {FacultyID: "faculty-3", TimeSlotID: "tue-1"}, {FacultyID: "faculty-3", TimeSlotID: "tue-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-2"},
			{RoomID: "room-lecture-2", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-2"},
			{RoomID: "room-lab-1", TimeSlotID: "mon-1"}, {RoomID: "room-lab-1", TimeSlotID: "mon-2"}, {RoomID: "room-lab-1", TimeSlotID: "mon-3"}, {RoomID: "room-lab-1", TimeSlotID: "tue-1"}, {RoomID: "room-lab-1", TimeSlotID: "tue-2"},
		},
	}
	return p
}

func TestLockedAssignments_ValidSingle(t *testing.T) {
	p := lockedBaseProblem()
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
			Instance:             0,
		},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error = %v, diag = %+v", err, diag)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s, want SOLVED", diag.Status)
	}

	// Verify locked assignment is in solution
	found := false
	for _, a := range solution.Assignments {
		if a.ID == "locked-1" {
			found = true
			if a.TimeSlotID != "mon-1" || a.RoomID != "room-lecture-1" {
				t.Fatalf("locked assignment modified: %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("locked assignment not present in final solution")
	}
}

func TestLockedAssignments_TwoCompatible(t *testing.T) {
	p := lockedBaseProblem()
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
			Instance:             0,
		},
		{
			ID:                   "locked-2",
			CourseOfferingID:     "offering-b-theory",
			StudentGroupID:       "group-b-whole",
			FacultyID:            "faculty-3",
			RoomID:               "room-lecture-2",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-b-theory",
			Instance:             0,
		},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error = %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s, want SOLVED", diag.Status)
	}
	assertAllRequiredSessionsScheduled(t, p, solution)
}

func TestLockedAssignments_LockedVsLockedFacultyConflict(t *testing.T) {
	p := lockedBaseProblem()
	// Two locked assignments using faculty-1 at the same slot mon-1
	p.CourseOfferings["offering-b-theory"] = model.CourseOffering{
		ID:                    "offering-b-theory",
		TermID:                "term-locked",
		ClassID:               "class-b",
		SubjectID:             "subj-theory",
		StudentGroupID:        "group-b-whole",
		FacultyID:             "faculty-1",
		SessionRequirementIDs: []model.SessionRequirementID{"req-b-theory"},
	}

	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
		},
		{
			ID:                   "locked-2",
			CourseOfferingID:     "offering-b-theory",
			StudentGroupID:       "group-b-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-2",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-b-theory",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment faculty conflict") {
		t.Fatalf("expected locked assignment faculty conflict violation, got %+v", violations)
	}

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if !errors.Is(err, backtracking.ErrInvalidProblem) {
		t.Fatalf("expected ErrInvalidProblem, got %v", err)
	}
	if diag.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("status = %s, want INVALID_PROBLEM", diag.Status)
	}
}

func TestLockedAssignments_LockedVsLockedRoomConflict(t *testing.T) {
	p := lockedBaseProblem()
	// Two locked assignments using room-lecture-1 at the same slot mon-1
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
		},
		{
			ID:                   "locked-2",
			CourseOfferingID:     "offering-b-theory",
			StudentGroupID:       "group-b-whole",
			FacultyID:            "faculty-3",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-b-theory",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment room conflict") {
		t.Fatalf("expected locked assignment room conflict violation, got %+v", violations)
	}

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if !errors.Is(err, backtracking.ErrInvalidProblem) {
		t.Fatalf("expected ErrInvalidProblem, got %v", err)
	}
	if diag.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("status = %s, want INVALID_PROBLEM", diag.Status)
	}
}

func TestLockedAssignments_LockedVsLockedStudentGroupOverlapConflict(t *testing.T) {
	p := lockedBaseProblem()
	// Class A Whole group overlaps with Class A Lab 1. Both locked at mon-1.
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
		},
		{
			ID:                   "locked-2",
			CourseOfferingID:     "offering-a-lab1",
			StudentGroupID:       "group-a-lab1",
			FacultyID:            "faculty-2",
			RoomID:               "room-lab-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-lab1",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment student group overlap conflict") {
		t.Fatalf("expected locked assignment student group overlap conflict violation, got %+v", violations)
	}

	_, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if !errors.Is(err, backtracking.ErrInvalidProblem) {
		t.Fatalf("expected ErrInvalidProblem, got %v", err)
	}
	if diag.Status != diagnostics.SolveStatusInvalidProblem {
		t.Fatalf("status = %s, want INVALID_PROBLEM", diag.Status)
	}
}

func TestLockedAssignments_LockedCapacityViolation(t *testing.T) {
	p := lockedBaseProblem()
	// Group A Whole has size 40, Room has capacity 30
	p.Rooms["room-small"] = model.Room{ID: "room-small", Name: "Small", Capacity: 30}
	p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: "room-small", TimeSlotID: "mon-1"})

	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-small",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment room capacity is below student group size") {
		t.Fatalf("expected locked capacity violation, got %+v", violations)
	}
}

func TestLockedAssignments_LockedRoomFeatureViolation(t *testing.T) {
	p := lockedBaseProblem()
	// Lab requires labFeatureID, but room-lecture-1 has no features
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-lab1",
			StudentGroupID:       "group-a-lab1",
			FacultyID:            "faculty-2",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-lab1",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment room does not provide all required features") {
		t.Fatalf("expected locked room feature violation, got %+v", violations)
	}
}

func TestLockedAssignments_LockedAvailabilityViolation(t *testing.T) {
	p := lockedBaseProblem()
	// Faculty 1 is not available at tue-1 (remove from availability)
	p.FacultyAvailabilities = []model.FacultyAvailability{
		{FacultyID: "faculty-1", TimeSlotID: "mon-1"},
		{FacultyID: "faculty-2", TimeSlotID: "mon-1"},
		{FacultyID: "faculty-3", TimeSlotID: "mon-1"},
	}

	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-2", // faculty-1 unavailable
			SessionRequirementID: "req-a-theory",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked assignment faculty is not available") {
		t.Fatalf("expected faculty availability violation, got %+v", violations)
	}
}

func TestLockedAssignments_LockedSessionsExceedingRequirementCount(t *testing.T) {
	p := lockedBaseProblem()
	// req-b-theory only requires 1 session per week, but 2 locked assignments are supplied
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-1",
			CourseOfferingID:     "offering-b-theory",
			StudentGroupID:       "group-b-whole",
			FacultyID:            "faculty-3",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-b-theory",
		},
		{
			ID:                   "locked-2",
			CourseOfferingID:     "offering-b-theory",
			StudentGroupID:       "group-b-whole",
			FacultyID:            "faculty-3",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-2",
			SessionRequirementID: "req-b-theory",
		},
	}

	violations := problem.Validate(p)
	if !hasViolationMessageContaining(violations, "locked sessions exceed session requirement count") {
		t.Fatalf("expected locked sessions exceed requirement count violation, got %+v", violations)
	}
}

func TestLockedAssignments_PartiallyLockedRequirement(t *testing.T) {
	p := lockedBaseProblem()
	// req-a-theory needs 2 sessions. 1 is locked.
	p.LockedAssignments = []problem.Assignment{
		{
			ID:                   "locked-a1",
			CourseOfferingID:     "offering-a-theory",
			StudentGroupID:       "group-a-whole",
			FacultyID:            "faculty-1",
			RoomID:               "room-lecture-1",
			TimeSlotID:           "mon-1",
			SessionRequirementID: "req-a-theory",
			Instance:             0,
		},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s, want SOLVED", diag.Status)
	}

	// 2 sessions for req-a-theory, 1 for req-a-lab1, 1 for req-b-theory -> total 4
	if got, want := len(solution.Assignments), 4; got != want {
		t.Fatalf("assignment count = %d, want %d", got, want)
	}
	if got := solution.Index.ScheduledCount("req-a-theory"); got != 2 {
		t.Fatalf("req-a-theory scheduled count = %d, want 2", got)
	}
}

func TestLockedAssignments_FullyLockedRequirement(t *testing.T) {
	p := lockedBaseProblem()
	// All requirements fully locked
	p.LockedAssignments = []problem.Assignment{
		{ID: "locked-a1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory", Instance: 0},
		{ID: "locked-a2", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-2", SessionRequirementID: "req-a-theory", Instance: 1},
		{ID: "locked-lab", CourseOfferingID: "offering-a-lab1", StudentGroupID: "group-a-lab1", FacultyID: "faculty-2", RoomID: "room-lab-1", TimeSlotID: "tue-1", SessionRequirementID: "req-a-lab1", Instance: 0},
		{ID: "locked-b", CourseOfferingID: "offering-b-theory", StudentGroupID: "group-b-whole", FacultyID: "faculty-3", RoomID: "room-lecture-2", TimeSlotID: "mon-1", SessionRequirementID: "req-b-theory", Instance: 0},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s, want SOLVED", diag.Status)
	}
	if got, want := len(solution.Assignments), 4; got != want {
		t.Fatalf("assignment count = %d, want %d", got, want)
	}
}

func TestLockedAssignments_ExcludedFromSearchVariables(t *testing.T) {
	p := lockedBaseProblem()
	p.LockedAssignments = []problem.Assignment{
		{ID: "locked-a1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory"},
		{ID: "locked-a2", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-2", SessionRequirementID: "req-a-theory"},
		{ID: "locked-lab", CourseOfferingID: "offering-a-lab1", StudentGroupID: "group-a-lab1", FacultyID: "faculty-2", RoomID: "room-lab-1", TimeSlotID: "tue-1", SessionRequirementID: "req-a-lab1"},
		{ID: "locked-b", CourseOfferingID: "offering-b-theory", StudentGroupID: "group-b-whole", FacultyID: "faculty-3", RoomID: "room-lecture-2", TimeSlotID: "mon-1", SessionRequirementID: "req-b-theory"},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	// With all 4 sessions locked, nodes explored in search must be 0
	if diag.NodesExplored != 0 {
		t.Fatalf("expected 0 search nodes explored when all locked, got %d", diag.NodesExplored)
	}
	if len(solution.Assignments) != 4 {
		t.Fatalf("expected 4 assignments, got %d", len(solution.Assignments))
	}
}

func TestLockedAssignments_RemainInFinalSolution(t *testing.T) {
	p := lockedBaseProblem()
	p.LockedAssignments = []problem.Assignment{
		{ID: "locked-a1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory"},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s", diag.Status)
	}

	found := false
	for _, a := range solution.Assignments {
		if a.ID == "locked-a1" {
			found = true
			if a.TimeSlotID != "mon-1" || a.RoomID != "room-lecture-1" {
				t.Fatalf("locked assignment changed: %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("locked assignment not in final solution")
	}
}

func TestLockedAssignments_ContributeToStudentGapPenalty(t *testing.T) {
	p := lockedBaseProblem()
	// Lock session 1 at Mon P1 and session 2 at Mon P3 -> creates 1 gap at Mon P2
	p.LockedAssignments = []problem.Assignment{
		{ID: "locked-a1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory"},
		{ID: "locked-a2", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-3", SessionRequirementID: "req-a-theory"},
	}

	solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("status = %s", diag.Status)
	}

	// Verify solution.Score includes student gap penalty
	if solution.Score.Breakdown.StudentGapPenalty < 1 {
		t.Fatalf("expected at least 1 gap penalty from locked assignments, got %d", solution.Score.Breakdown.StudentGapPenalty)
	}
	if solution.Score.SoftPenalty < 1 {
		t.Fatalf("expected SoftPenalty >= 1, got %d", solution.Score.SoftPenalty)
	}
}
