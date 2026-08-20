package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

// ----------------------------------------------------------------------------
// Stress & Pathological CSP Test Suite
// ----------------------------------------------------------------------------

func stressBaseProblem(roomCount, slotCount, sessionCount int) problem.Problem {
	tenantID := model.TenantID("tenant-stress")
	deptID := model.DepartmentID("dept-1")
	progID := model.ProgramID("prog-1")
	classID := model.ClassID("class-1")
	groupID := model.StudentGroupID("group-1")
	subjID := model.SubjectID("subj-1")
	offeringID := model.CourseOfferingID("offering-1")
	reqID := model.SessionRequirementID("req-1")
	facID := model.FacultyID("faculty-1")

	p := problem.Problem{
		TenantID: tenantID,
		Term:     model.Term{ID: "term-1", TenantID: tenantID, Name: "Term 1"},
		PeriodsPerDay: slotCount,
		Departments: map[model.DepartmentID]model.Department{
			deptID: {ID: deptID, TenantID: tenantID, Name: "Dept"},
		},
		Programs: map[model.ProgramID]model.Program{
			progID: {ID: progID, DepartmentID: deptID, Name: "Prog"},
		},
		Classes: map[model.ClassID]model.Class{
			classID: {ID: classID, ProgramID: progID, Name: "Class", WholeGroupID: groupID, StudentGroupIDs: []model.StudentGroupID{groupID}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			groupID: {ID: groupID, ClassID: classID, Name: "Group", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			subjID: {ID: subjID, Name: "Subj", Code: "S1"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			facID: {ID: facID, Name: "Prof 1"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			offeringID: {
				ID:                    offeringID,
				TermID:                "term-1",
				ClassID:               classID,
				SubjectID:             subjID,
				StudentGroupID:        groupID,
				FacultyID:             facID,
				SessionRequirementIDs: []model.SessionRequirementID{reqID},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			reqID: {
				ID:               reqID,
				CourseOfferingID: offeringID,
				Type:             model.SessionTypeTheory,
				Duration:         1,
				SessionsPerWeek:  sessionCount,
			},
		},
		Rooms:     make(map[model.RoomID]model.Room),
		TimeSlots: make(map[model.TimeSlotID]model.TimeSlot),
	}

	for r := 1; r <= roomCount; r++ {
		rID := model.RoomID(fmt.Sprintf("room-%d", r))
		p.Rooms[rID] = model.Room{ID: rID, Name: fmt.Sprintf("Room %d", r), Capacity: 50}
	}
	for s := 1; s <= slotCount; s++ {
		sID := model.TimeSlotID(fmt.Sprintf("slot-%d", s))
		p.TimeSlots[sID] = model.TimeSlot{ID: sID, Day: time.Monday, Period: s}
		p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
			FacultyID:  facID,
			TimeSlotID: sID,
		})
		for r := 1; r <= roomCount; r++ {
			rID := model.RoomID(fmt.Sprintf("room-%d", r))
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{
				RoomID:     rID,
				TimeSlotID: sID,
			})
		}
	}

	return p
}

func TestStress_VeryTightRoomAvailability(t *testing.T) {
	// Exactly 1 room available for 5 sessions across 5 timeslots (100% room occupancy required)
	p := stressBaseProblem(1, 5, 5)
	p.Prepare()

	solver := backtracking.New()
	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil {
		t.Fatalf("tight room availability solve failed: %v (status=%s msg=%s violations=%+v)", err, diag.Status, diag.Message, diag.Violations)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", diag.Status)
	}
	if len(sol.Assignments) != 5 {
		t.Fatalf("expected 5 assignments, got %d", len(sol.Assignments))
	}
}

func TestStress_VeryTightFacultyAvailability(t *testing.T) {
	// Faculty only available in exact required slots
	p := stressBaseProblem(2, 5, 2)
	p.FacultyAvailabilities = []model.FacultyAvailability{
		{FacultyID: "faculty-1", TimeSlotID: "slot-1"},
		{FacultyID: "faculty-1", TimeSlotID: "slot-2"},
	}
	p.Prepare()

	solver := backtracking.New()
	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil {
		t.Fatalf("tight faculty availability solve failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", diag.Status)
	}
	for _, a := range sol.Assignments {
		if a.FacultyID == "faculty-1" && a.TimeSlotID != "slot-1" && a.TimeSlotID != "slot-2" {
			t.Fatalf("faculty scheduled in unavailable slot: %s", a.TimeSlotID)
		}
	}
}

func TestStress_HeavyLockedAssignments(t *testing.T) {
	// Problem where assignments are pre-locked
	p := stressBaseProblem(2, 5, 3)
	p.LockedAssignments = []problem.Assignment{
		{ID: "req-1#0", CourseOfferingID: "offering-1", StudentGroupID: "group-1", FacultyID: "faculty-1", RoomID: "room-1", TimeSlotID: "slot-1", SessionRequirementID: "req-1", Instance: 0},
		{ID: "req-1#1", CourseOfferingID: "offering-1", StudentGroupID: "group-1", FacultyID: "faculty-1", RoomID: "room-1", TimeSlotID: "slot-2", SessionRequirementID: "req-1", Instance: 1},
	}
	p.Prepare()

	solver := backtracking.New()
	sol, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err != nil {
		t.Fatalf("locked assignment solve failed: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", diag.Status)
	}

	// Verify all locked assignments remain exact
	for _, locked := range p.LockedAssignments {
		found, ok := sol.Index.AssignmentByID(locked.ID)
		if !ok || found.RoomID != locked.RoomID || found.TimeSlotID != locked.TimeSlotID {
			t.Fatalf("locked assignment %s corrupted in solution: %+v", locked.ID, found)
		}
	}
}

func TestStress_ProvablyInfeasible_OverbookedFaculty(t *testing.T) {
	// Faculty required for 6 sessions, but only 5 total timeslots exist in the entire problem
	p := stressBaseProblem(1, 5, 6)
	p.Prepare()

	solver := backtracking.New()
	_, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic})
	if err == nil {
		t.Fatal("expected solve to fail on overbooked faculty, got nil")
	}
	if diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected INFEASIBLE status, got %s", diag.Status)
	}
}

func TestStress_ProvablyInfeasible_OverbookedRoom(t *testing.T) {
	// 1 room of capacity 100, but only 4 slots exist and 5 sessions required across different faculties
	p := stressBaseProblem(1, 4, 3)

	// Add second offering/faculty that also needs 3 sessions
	fac2 := model.FacultyID("faculty-2")
	offering2 := model.CourseOfferingID("offering-2")
	req2 := model.SessionRequirementID("req-2")

	p.Faculty[fac2] = model.Faculty{ID: fac2, Name: "Prof 2"}
	p.CourseOfferings[offering2] = model.CourseOffering{
		ID:                    offering2,
		TermID:                p.Term.ID,
		ClassID:               "class-1",
		SubjectID:             "subj-1",
		StudentGroupID:        "group-1",
		FacultyID:             fac2,
		SessionRequirementIDs: []model.SessionRequirementID{req2},
	}
	p.SessionRequirements[req2] = model.SessionRequirement{
		ID:               req2,
		CourseOfferingID: offering2,
		Type:             model.SessionTypeTheory,
		Duration:         1,
		SessionsPerWeek:  3,
	}
	for s := 1; s <= 4; s++ {
		p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
			FacultyID:  fac2,
			TimeSlotID: model.TimeSlotID(fmt.Sprintf("slot-%d", s)),
		})
	}
	p.Prepare()

	solver := backtracking.New()
	_, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeHeuristic, MaxNodes: 1000})
	if err == nil {
		t.Fatal("expected solve to fail on overbooked room, got nil")
	}
	if diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected INFEASIBLE status, got %s", diag.Status)
	}
}

func TestStress_CleanTimeoutAndNodeLimitPreservation(t *testing.T) {
	// Ensure solver respects context cancellation cleanly and returns exact valid diagnostic state
	p := GenerateSyntheticProblem(DefaultMediumProblemConfig())
	p.Prepare()

	// 1. MaxNodes limit
	solver := backtracking.New()
	opts := problem.SolveOptions{MaxNodes: 5, SearchMode: problem.SearchModeBasic}
	_, diagNode, errNode := solver.Solve(context.Background(), p, opts)
	if !errors.Is(errNode, backtracking.ErrNodeLimit) {
		t.Fatalf("expected ErrNodeLimit, got %v", errNode)
	}
	if diagNode.Status != diagnostics.SolveStatusNodeLimit {
		t.Fatalf("expected NODE_LIMIT status, got %s", diagNode.Status)
	}
	if diagNode.NodesExplored > 6 {
		t.Fatalf("exceeded max nodes limit: got %d", diagNode.NodesExplored)
	}

	// 2. Context Cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, diagCancel, errCancel := solver.Solve(ctx, p, problem.SolveOptions{})
	if !errors.Is(errCancel, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", errCancel)
	}
	if diagCancel.Status != diagnostics.SolveStatusCancelled {
		t.Fatalf("expected CANCELLED status, got %s", diagCancel.Status)
	}
}
