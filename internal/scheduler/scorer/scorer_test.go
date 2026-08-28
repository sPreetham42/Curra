package scorer_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

func gapTestProblem() problem.Problem {
	p := problem.Problem{
		TenantID: "tenant-gap",
		Term:     model.Term{ID: "term-gap", TenantID: "tenant-gap", Name: "Gap Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-gap", Name: "Dept 1"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "Prog 1"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {
				ID:              "class-1",
				ProgramID:       "prog-1",
				Name:            "Class 1",
				WholeGroupID:    "group-1",
				StudentGroupIDs: []model.StudentGroupID{"group-1", "group-2"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-1": {ID: "group-1", ClassID: "class-1", Name: "Group 1", Size: 30},
			"group-2": {ID: "group-2", ClassID: "class-1", Name: "Group 2", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-1": {ID: "subj-1", Code: "S1", Name: "Subject 1"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"off-1":   {ID: "off-1", TermID: "term-gap", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "group-1", FacultyID: "fac-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
			"off-2":   {ID: "off-2", TermID: "term-gap", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "group-2", FacultyID: "fac-2", SessionRequirementIDs: []model.SessionRequirementID{"req-2"}},
			"off-lab": {ID: "off-lab", TermID: "term-gap", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "group-1", FacultyID: "fac-1", SessionRequirementIDs: []model.SessionRequirementID{"req-lab"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1":   {ID: "req-1", CourseOfferingID: "off-1", Type: model.SessionTypeTheory, SessionsPerWeek: 4, Duration: 1, Consecutive: true},
			"req-2":   {ID: "req-2", CourseOfferingID: "off-2", Type: model.SessionTypeTheory, SessionsPerWeek: 4, Duration: 1, Consecutive: true},
			"req-lab": {ID: "req-lab", CourseOfferingID: "off-lab", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"fac-1": {ID: "fac-1", Name: "Faculty 1"},
			"fac-2": {ID: "fac-2", Name: "Faculty 2"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-1": {ID: "room-1", Name: "Room 1", Capacity: 60},
			"room-2": {ID: "room-2", Name: "Room 2", Capacity: 60},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"mon-3": {ID: "mon-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"mon-4": {ID: "mon-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"mon-5": {ID: "mon-5", Day: time.Monday, Period: 5, Label: "Mon P5"},
			"mon-6": {ID: "mon-6", Day: time.Monday, Period: 6, Label: "Mon P6"},
			"tue-1": {ID: "tue-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"tue-2": {ID: "tue-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
			"tue-3": {ID: "tue-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"},
			"tue-4": {ID: "tue-4", Day: time.Tuesday, Period: 4, Label: "Tue P4"},
			"tue-5": {ID: "tue-5", Day: time.Tuesday, Period: 5, Label: "Tue P5"},
			"tue-6": {ID: "tue-6", Day: time.Tuesday, Period: 6, Label: "Tue P6"},
		},
		PeriodsPerDay: 6,
	}
	p.Prepare()
	return p
}

func TestStudentGapPenalty_ZeroGaps(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Continuous sessions on Monday: Periods 1, 2, 3
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-2", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a3", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 0 {
		t.Fatalf("expected 0 gap penalty, got %d", breakdown.StudentGapPenalty)
	}
	if len(breakdown.Details) != 0 {
		t.Fatalf("expected 0 gap details, got %d", len(breakdown.Details))
	}
}

func TestStudentGapPenalty_OneGap(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Sessions at Mon P1 and Mon P3 -> 1 gap at Mon P2
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 1 {
		t.Fatalf("expected 1 gap penalty, got %d", breakdown.StudentGapPenalty)
	}
	if breakdown.GroupGaps["group-1"] != 1 {
		t.Fatalf("expected group-1 gap = 1, got %d", breakdown.GroupGaps["group-1"])
	}
}

func TestStudentGapPenalty_MultipleGaps(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Sessions at Mon P1, Mon P3, Mon P6 -> gap at P2 (1), gaps at P4, P5 (2) -> total 3 gaps
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a3", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-6", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 3 {
		t.Fatalf("expected 3 gap penalty, got %d", breakdown.StudentGapPenalty)
	}
	if breakdown.GroupGaps["group-1"] != 3 {
		t.Fatalf("expected group-1 gap = 3, got %d", breakdown.GroupGaps["group-1"])
	}
}

func TestStudentGapPenalty_DifferentGroups(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Group 1: Mon P1, Mon P3 -> 1 gap
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})

	// Group 2: Mon P2, Mon P5 -> gaps at P3, P4 (2 gaps)
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a3", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "mon-2", SessionRequirementID: "req-2"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a4", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "mon-5", SessionRequirementID: "req-2"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 3 {
		t.Fatalf("expected 3 total gap penalty, got %d", breakdown.StudentGapPenalty)
	}
	if breakdown.GroupGaps["group-1"] != 1 {
		t.Fatalf("expected group-1 gap = 1, got %d", breakdown.GroupGaps["group-1"])
	}
	if breakdown.GroupGaps["group-2"] != 2 {
		t.Fatalf("expected group-2 gap = 2, got %d", breakdown.GroupGaps["group-2"])
	}
}

func TestStudentGapPenalty_DifferentDays(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Group 1 Mon: P1, P3 -> 1 gap
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})

	// Group 1 Tue: P2, P5 -> 2 gaps (P3, P4)
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a3", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "tue-2", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a4", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "tue-5", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 3 {
		t.Fatalf("expected 3 total gap penalty, got %d", breakdown.StudentGapPenalty)
	}
	if breakdown.GroupGaps["group-1"] != 3 {
		t.Fatalf("expected group-1 gap = 3, got %d", breakdown.GroupGaps["group-1"])
	}
	if len(breakdown.Details) != 2 {
		t.Fatalf("expected 2 day details, got %d", len(breakdown.Details))
	}
}

func TestStudentGapPenalty_MultiPeriodSessions(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Multi-period session at Mon P2 (duration 2 -> occupies Mon P2 and Mon P3)
	// Single session at Mon P5
	// First period = 2, Last period = 5
	// Occupied: 2, 3, 5
	// Unoccupied between them: 4 -> 1 gap!
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-lab", CourseOfferingID: "off-lab", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-2", SessionRequirementID: "req-lab"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-single", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-5", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 1 {
		t.Fatalf("expected 1 gap penalty, got %d", breakdown.StudentGapPenalty)
	}
}

func TestStudentGapPenalty_LeadingTrailingFreePeriodsIgnored(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	// Grid has periods 1 to 6.
	// Sessions at Mon P3 and Mon P4 (consecutive, no gaps between them).
	// Leading periods P1, P2 are free.
	// Trailing periods P5, P6 are free.
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-4", SessionRequirementID: "req-1"})

	breakdown := p.StudentGapPenalty(&solution)
	if breakdown.StudentGapPenalty != 0 {
		t.Fatalf("expected 0 gap penalty (leading/trailing free ignored), got %d", breakdown.StudentGapPenalty)
	}
}

func TestStudentGapPenalty_DeterministicRepeatedScoring(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a3", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "tue-1", SessionRequirementID: "req-2"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a4", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "tue-4", SessionRequirementID: "req-2"})

	first := p.StudentGapPenalty(&solution)
	for i := 0; i < 5; i++ {
		repeat := p.StudentGapPenalty(&solution)
		if !reflect.DeepEqual(first, repeat) {
			t.Fatalf("iteration %d scoring non-deterministic: %+v != %+v", i, first, repeat)
		}
	}
}

func TestStudentGapPenalty_DoesNotMutateSolution(t *testing.T) {
	p := gapTestProblem()
	solution := problem.NewSolution()
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-3", SessionRequirementID: "req-1"})

	countBefore := len(solution.Assignments)
	indexCountBefore := solution.Index.ScheduledCount("req-1")

	_ = p.StudentGapPenalty(&solution)

	if len(solution.Assignments) != countBefore {
		t.Fatalf("solution assignments modified: %d != %d", len(solution.Assignments), countBefore)
	}
	if solution.Index.ScheduledCount("req-1") != indexCountBefore {
		t.Fatalf("solution index modified: %d != %d", solution.Index.ScheduledCount("req-1"), indexCountBefore)
	}
}
