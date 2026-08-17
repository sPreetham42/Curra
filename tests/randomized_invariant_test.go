package tests

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

func TestRandomizedSolvedTimetableInvariants(t *testing.T) {
	seeds := []int64{11, 29, 47, 83, 101, 149, 211, 277, 353, 431}
	for _, seed := range seeds {
		p := randomFeasibleProblem(seed)
		solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
		if err != nil {
			t.Fatalf("seed=%d Solve returned error: %v\nDiagnostics: %+v", seed, err, diag)
		}
		if diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("seed=%d status = %s, want %s", seed, diag.Status, diagnostics.SolveStatusSolved)
		}
		if err := checkSolvedInvariants(p, solution); err != nil {
			t.Fatalf("seed=%d invariant failure: %v", seed, err)
		}
	}
}

func randomFeasibleProblem(seed int64) problem.Problem {
	r := rand.New(rand.NewSource(seed))
	labFeatureID := model.RoomFeatureID("feature-lab")
	classCount := 2 + r.Intn(2)

	p := problem.Problem{
		TenantID: "tenant-random",
		Term:     model.Term{ID: "term-random", TenantID: "tenant-random", Name: "Random Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-random": {ID: "dept-random", TenantID: "tenant-random", Name: "Random Department"},
		},
		Programs: map[model.ProgramID]model.Program{
			"program-random": {ID: "program-random", DepartmentID: "dept-random", Name: "Random Program"},
		},
		Classes:             make(map[model.ClassID]model.Class),
		StudentGroups:       make(map[model.StudentGroupID]model.StudentGroup),
		Subjects:            make(map[model.SubjectID]model.Subject),
		CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
		SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		Faculty:             make(map[model.FacultyID]model.Faculty),
		Rooms:               make(map[model.RoomID]model.Room),
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: make(map[model.TimeSlotID]model.TimeSlot),
	}

	for day := time.Monday; day <= time.Friday; day++ {
		for period := 1; period <= 6; period++ {
			id := model.TimeSlotID(fmt.Sprintf("%s-%d", day.String()[:3], period))
			p.TimeSlots[id] = model.TimeSlot{ID: id, Day: day, Period: period, Label: string(id)}
		}
	}
	for i := 0; i < classCount+2; i++ {
		id := model.RoomID(fmt.Sprintf("room-lecture-%d", i))
		p.Rooms[id] = model.Room{ID: id, Name: string(id), Capacity: 120}
	}
	for i := 0; i < classCount*2+2; i++ {
		id := model.RoomID(fmt.Sprintf("room-lab-%d", i))
		p.Rooms[id] = model.Room{ID: id, Name: string(id), Capacity: 40, FeatureIDs: []model.RoomFeatureID{labFeatureID}}
	}

	p.Subjects["subject-theory"] = model.Subject{ID: "subject-theory", Code: "THEORY", Name: "Theory"}
	p.Subjects["subject-lab"] = model.Subject{ID: "subject-lab", Code: "LAB", Name: "Lab"}

	for c := 0; c < classCount; c++ {
		classID := model.ClassID(fmt.Sprintf("class-%d", c))
		wholeID := model.StudentGroupID(fmt.Sprintf("%s-whole", classID))
		subgroup1 := model.StudentGroupID(fmt.Sprintf("%s-lab-1", classID))
		subgroup2 := model.StudentGroupID(fmt.Sprintf("%s-lab-2", classID))
		p.Classes[classID] = model.Class{
			ID:              classID,
			ProgramID:       "program-random",
			Name:            string(classID),
			WholeGroupID:    wholeID,
			StudentGroupIDs: []model.StudentGroupID{wholeID, subgroup1, subgroup2},
		}
		p.StudentGroups[wholeID] = model.StudentGroup{ID: wholeID, ClassID: classID, Name: string(wholeID), Size: 60 + r.Intn(20)}
		p.StudentGroups[subgroup1] = model.StudentGroup{ID: subgroup1, ClassID: classID, Name: string(subgroup1), Size: 25 + r.Intn(10)}
		p.StudentGroups[subgroup2] = model.StudentGroup{ID: subgroup2, ClassID: classID, Name: string(subgroup2), Size: 25 + r.Intn(10)}

		addRandomOffering(&p, fmt.Sprintf("c%d-theory", c), classID, wholeID, "subject-theory", model.SessionTypeTheory, 1+r.Intn(2), 1, nil)
		addRandomOffering(&p, fmt.Sprintf("c%d-lab-1", c), classID, subgroup1, "subject-lab", model.SessionTypeLab, 1, 1+r.Intn(2), []model.RoomFeatureID{labFeatureID})
		addRandomOffering(&p, fmt.Sprintf("c%d-lab-2", c), classID, subgroup2, "subject-lab", model.SessionTypeLab, 1, 1+r.Intn(2), []model.RoomFeatureID{labFeatureID})
	}

	for facultyID := range p.Faculty {
		for slotID := range p.TimeSlots {
			p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{FacultyID: facultyID, TimeSlotID: slotID})
		}
	}
	for roomID := range p.Rooms {
		for slotID := range p.TimeSlots {
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: roomID, TimeSlotID: slotID})
		}
	}
	p.PeriodsPerDay = 6
	return p
}

func addRandomOffering(p *problem.Problem, suffix string, classID model.ClassID, groupID model.StudentGroupID, subjectID model.SubjectID, sessionType model.SessionType, sessions int, duration int, features []model.RoomFeatureID) {
	offeringID := model.CourseOfferingID("offering-" + suffix)
	requirementID := model.SessionRequirementID("req-" + suffix)
	facultyID := model.FacultyID("faculty-" + suffix)
	p.Faculty[facultyID] = model.Faculty{ID: facultyID, Name: string(facultyID)}
	p.CourseOfferings[offeringID] = model.CourseOffering{
		ID:                    offeringID,
		TermID:                p.Term.ID,
		ClassID:               classID,
		SubjectID:             subjectID,
		StudentGroupID:        groupID,
		FacultyID:             facultyID,
		SessionRequirementIDs: []model.SessionRequirementID{requirementID},
	}
	p.SessionRequirements[requirementID] = model.SessionRequirement{
		ID:                     requirementID,
		CourseOfferingID:       offeringID,
		Type:                   sessionType,
		SessionsPerWeek:        sessions,
		Duration:               duration,
		Consecutive:            true,
		RequiredRoomFeatureIDs: features,
	}
}

func checkSolvedInvariants(p problem.Problem, solution problem.Solution) error {
	p.Prepare()
	expected := 0
	for _, offering := range p.CourseOfferings {
		for _, requirementID := range offering.SessionRequirementIDs {
			requirement := p.SessionRequirements[requirementID]
			expected += requirement.SessionsPerWeek
			if got := solution.Index.ScheduledCount(requirementID); got != requirement.SessionsPerWeek {
				return fmt.Errorf("scheduled count for %s = %d, want %d", requirementID, got, requirement.SessionsPerWeek)
			}
		}
	}
	if len(solution.Assignments) != expected {
		return fmt.Errorf("assignment count = %d, want %d", len(solution.Assignments), expected)
	}

	partial := problem.NewSolution()
	for _, assignment := range solution.Assignments {
		if violations := constraints.CheckAll(&p, &partial, assignment, constraints.DefaultHardConstraints()); len(violations) != 0 {
			return fmt.Errorf("hard violation on %s: %+v", assignment.ID, violations)
		}
		room := p.Rooms[assignment.RoomID]
		if room.Capacity < p.StudentGroupSize(assignment.StudentGroupID) {
			return fmt.Errorf("capacity violated for %s", assignment.ID)
		}
		if !p.RoomHasFeatures(assignment.RoomID, p.RequiredRoomFeatures(assignment.CourseOfferingID, assignment.SessionRequirementID)) {
			return fmt.Errorf("features violated for %s", assignment.ID)
		}
		if err := partial.AddAssignment(&p, assignment); err != nil {
			return fmt.Errorf("resource conflict on %s: %w", assignment.ID, err)
		}
	}

	for i, left := range solution.Assignments {
		leftSlots, ok := left.OccupiedSlotIDs(&p)
		if !ok {
			return fmt.Errorf("invalid occupied slots for %s", left.ID)
		}
		for j := i + 1; j < len(solution.Assignments); j++ {
			right := solution.Assignments[j]
			rightSlots, ok := right.OccupiedSlotIDs(&p)
			if !ok {
				return fmt.Errorf("invalid occupied slots for %s", right.ID)
			}
			if !slotsIntersect(leftSlots, rightSlots) {
				continue
			}
			if left.FacultyID == right.FacultyID {
				return fmt.Errorf("faculty conflict between %s and %s", left.ID, right.ID)
			}
			if left.RoomID == right.RoomID {
				return fmt.Errorf("room conflict between %s and %s", left.ID, right.ID)
			}
			if p.StudentGroupsOverlap(left.StudentGroupID, right.StudentGroupID) {
				return fmt.Errorf("student group overlap conflict between %s and %s", left.ID, right.ID)
			}
		}
	}
	return nil
}

func slotsIntersect(left []model.TimeSlotID, right []model.TimeSlotID) bool {
	seen := make(map[model.TimeSlotID]struct{}, len(left))
	for _, id := range left {
		seen[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := seen[id]; ok {
			return true
		}
	}
	return false
}
