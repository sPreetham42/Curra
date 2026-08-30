package scorer_test

import (
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

// Helper to build problem fixture for room change unit tests.
func buildRoomChangeTestProblem() problem.Problem {
	p := problem.Problem{
		TenantID:      "t1",
		Term:          model.Term{ID: "term1", TenantID: "t1", Name: "Fall 2026"},
		PeriodsPerDay: 5,
		Departments:   map[model.DepartmentID]model.Department{"d1": {ID: "d1", TenantID: "t1", Name: "CS"}},
		Programs:      map[model.ProgramID]model.Program{"p1": {ID: "p1", DepartmentID: "d1", Name: "BSCS"}},
		Classes: map[model.ClassID]model.Class{
			"c1": {ID: "c1", ProgramID: "p1", Name: "Class1", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1": {ID: "g1", ClassID: "c1", Name: "Group1", Size: 30},
			"g2": {ID: "g2", ClassID: "c1", Name: "Group2", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s1": {ID: "s1", Code: "CS101", Name: "Programming"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f1": {ID: "f1", Name: "Prof Alpha"},
			"f2": {ID: "f2", Name: "Prof Beta"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r1": {ID: "r1", Name: "Room 101", Capacity: 50},
			"r2": {ID: "r2", Name: "Room 102", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"ts1": {ID: "ts1", Day: time.Monday, Period: 1},
			"ts2": {ID: "ts2", Day: time.Monday, Period: 2},
			"ts3": {ID: "ts3", Day: time.Monday, Period: 3},
			"ts4": {ID: "ts4", Day: time.Monday, Period: 4},
			"ts5": {ID: "ts5", Day: time.Monday, Period: 5},
			"ts6": {ID: "ts6", Day: time.Tuesday, Period: 1},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co1": {ID: "co1", TermID: "term1", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g1", FacultyID: "f1"},
			"co2": {ID: "co2", TermID: "term1", ClassID: "c1", SubjectID: "s1", StudentGroupID: "g2", FacultyID: "f2"},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"sr1": {ID: "sr1", CourseOfferingID: "co1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"sr2": {ID: "sr2", CourseOfferingID: "co1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"sr3": {ID: "sr3", CourseOfferingID: "co1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2},
			"sr4": {ID: "sr4", CourseOfferingID: "co2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"sr5": {ID: "sr5", CourseOfferingID: "co2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
	}
	p.Prepare()
	return p
}

// Case A: No sessions -> RoomChangePenalty = 0
func TestRoomChange_A_NoSessions(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 0 {
		t.Fatalf("Expected RoomChangePenalty = 0 for empty solution, got %d", breakdown.RoomChangePenalty)
	}
}

// Case B: One session -> RoomChangePenalty = 0
func TestRoomChange_B_OneSession(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 0 {
		t.Fatalf("Expected RoomChangePenalty = 0 for single session, got %d", breakdown.RoomChangePenalty)
	}
}

// Case C: Two consecutive sessions, same room -> RoomChangePenalty = 0
func TestRoomChange_C_TwoConsecutive_SameRoom(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts2",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 0 {
		t.Fatalf("Expected RoomChangePenalty = 0 for same room consecutive sessions, got %d", breakdown.RoomChangePenalty)
	}
}

// Case D: Two consecutive sessions, different rooms -> RoomChangePenalty = 1
func TestRoomChange_D_TwoConsecutive_DifferentRooms(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r2",
		TimeSlotID:           "ts2",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 1 {
		t.Fatalf("Expected RoomChangePenalty = 1 for different room consecutive sessions, got %d", breakdown.RoomChangePenalty)
	}
}

// Case E: Gap between sessions on same day -> RoomChangePenalty = 1
func TestRoomChange_E_GapBetweenSessions_SameDay(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// ts1 (Period 1, Room r1) and ts3 (Period 3, Room r2)
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts1",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r2",
		TimeSlotID:           "ts3",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 1 {
		t.Fatalf("Expected RoomChangePenalty = 1 for gapped sessions on same day with different rooms, got %d", breakdown.RoomChangePenalty)
	}
}

// Case F: Day boundary isolation -> RoomChangePenalty = 0
func TestRoomChange_F_DayBoundaryIsolation(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// ts5 (Monday Period 5, Room r1) and ts6 (Tuesday Period 1, Room r2)
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr1#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r1",
		TimeSlotID:           "ts5",
		SessionRequirementID: "sr1",
		Instance:             0,
	})
	_ = sol.AddAssignment(&p, problem.Assignment{
		ID:                   "sr2#0",
		CourseOfferingID:     "co1",
		StudentGroupID:       "g1",
		FacultyID:            "f1",
		RoomID:               "r2",
		TimeSlotID:           "ts6",
		SessionRequirementID: "sr2",
		Instance:             0,
	})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 0 {
		t.Fatalf("Expected RoomChangePenalty = 0 across day boundary, got %d", breakdown.RoomChangePenalty)
	}
}

// Case G: Multiple sessions cumulative room changes
func TestRoomChange_G_MultipleSessionsCumulative(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// ts1(r1) -> ts2(r2) [1] -> ts3(r1) [1] -> ts4(r1) [0] => Total = 2
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr1#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts1", SessionRequirementID: "sr1", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr2#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r2", TimeSlotID: "ts2", SessionRequirementID: "sr2", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr3#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts3", SessionRequirementID: "sr3", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr4#0", CourseOfferingID: "co2", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts4", SessionRequirementID: "sr4", Instance: 0})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 2 {
		t.Fatalf("Expected cumulative RoomChangePenalty = 2, got %d", breakdown.RoomChangePenalty)
	}
}

// Case H: Multiple student groups isolation
func TestRoomChange_H_MultipleGroupsIsolation(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// Group g1: ts1(r1), ts2(r1) -> 0 penalty
	if err := sol.AddAssignment(&p, problem.Assignment{ID: "sr1#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts1", SessionRequirementID: "sr1", Instance: 0}); err != nil {
		t.Fatalf("AddAssignment sr1#0 failed: %v", err)
	}
	if err := sol.AddAssignment(&p, problem.Assignment{ID: "sr2#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts2", SessionRequirementID: "sr2", Instance: 0}); err != nil {
		t.Fatalf("AddAssignment sr2#0 failed: %v", err)
	}

	// Group g2: ts1(r2), ts3(r1) -> 1 penalty (different rooms on same day)
	if err := sol.AddAssignment(&p, problem.Assignment{ID: "sr4#0", CourseOfferingID: "co2", StudentGroupID: "g2", FacultyID: "f2", RoomID: "r2", TimeSlotID: "ts1", SessionRequirementID: "sr4", Instance: 0}); err != nil {
		t.Fatalf("AddAssignment sr4#0 failed: %v", err)
	}
	if err := sol.AddAssignment(&p, problem.Assignment{ID: "sr5#0", CourseOfferingID: "co2", StudentGroupID: "g2", FacultyID: "f2", RoomID: "r1", TimeSlotID: "ts3", SessionRequirementID: "sr5", Instance: 0}); err != nil {
		t.Fatalf("AddAssignment sr5#0 failed: %v", err)
	}

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 1 {
		t.Fatalf("Expected multi-group RoomChangePenalty = 1, got %d", breakdown.RoomChangePenalty)
	}
}

// Case I: Multi-period sessions (Duration = 2)
func TestRoomChange_I_MultiPeriodSessions(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// sr3 is Duration=2 starting at ts1 (Periods 1, 2) in Room r1
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr3#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts1", SessionRequirementID: "sr3", Instance: 0})

	// sr1 is Duration=1 starting at ts3 (Period 3) in Room r2
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr1#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r2", TimeSlotID: "ts3", SessionRequirementID: "sr1", Instance: 0})

	// Transition from Duration=2 (Room r1) to Duration=1 (Room r2) -> 1 room change
	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 1 {
		t.Fatalf("Expected multi-period RoomChangePenalty = 1, got %d", breakdown.RoomChangePenalty)
	}
}

// Case J: Same room repeated later (non-adjacent repetition)
func TestRoomChange_J_SameRoomRepeatedLater(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	// ts1(r1) -> ts2(r2) [1] -> ts3(r1) [1] => Total = 2
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr1#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts1", SessionRequirementID: "sr1", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr2#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r2", TimeSlotID: "ts2", SessionRequirementID: "sr2", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr3#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts3", SessionRequirementID: "sr3", Instance: 0})

	breakdown := p.CalculateScore(&sol)
	if breakdown.RoomChangePenalty != 2 {
		t.Fatalf("Expected non-adjacent repetition RoomChangePenalty = 2, got %d", breakdown.RoomChangePenalty)
	}
}

// Case K: Weight scaling
func TestRoomChange_K_WeightScaling(t *testing.T) {
	p := buildRoomChangeTestProblem()
	sol := problem.NewSolution()
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr1#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r1", TimeSlotID: "ts1", SessionRequirementID: "sr1", Instance: 0})
	_ = sol.AddAssignment(&p, problem.Assignment{ID: "sr2#0", CourseOfferingID: "co1", StudentGroupID: "g1", FacultyID: "f1", RoomID: "r2", TimeSlotID: "ts2", SessionRequirementID: "sr2", Instance: 0})

	cfg5 := scorer.ObjectiveConfig{
		Components: []scorer.ObjectiveComponent{
			{ID: scorer.ObjectiveRoomChange, Weight: 5},
		},
	}
	breakdown5 := p.CalculateScoreWithConfig(&sol, cfg5)
	if breakdown5.RoomChangePenalty != 1 {
		t.Fatalf("Expected Raw RoomChangePenalty = 1, got %d", breakdown5.RoomChangePenalty)
	}
	if breakdown5.SoftPenalty != 5 {
		t.Fatalf("Expected SoftPenalty = 5 (1*5), got %d", breakdown5.SoftPenalty)
	}
}
