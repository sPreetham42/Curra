package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
)

func main() {
	inputPath := flag.String("input", "", "path to a JSON-encoded scheduling problem")
	maxNodes := flag.Int("max-nodes", 100000, "maximum backtracking nodes to explore")
	flag.Parse()

	p, err := loadProblem(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load problem: %v\n", err)
		os.Exit(2)
	}

	s := backtracking.New()
	solution, diag, err := s.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: *maxNodes})
	writeOutput(solution, diag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "solve: %v\n", err)
		os.Exit(1)
	}
}

func loadProblem(path string) (problem.Problem, error) {
	if path == "" {
		return sampleProblem(), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return problem.Problem{}, err
	}
	defer file.Close()

	var p problem.Problem
	if err := json.NewDecoder(file).Decode(&p); err != nil {
		return problem.Problem{}, err
	}
	return p, nil
}

func writeOutput(solution problem.Solution, diag diagnostics.Diagnostics) {
	output := struct {
		Solution    problem.Solution        `json:"solution"`
		Diagnostics diagnostics.Diagnostics `json:"diagnostics"`
	}{
		Solution:    solution,
		Diagnostics: diag,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(output)
}

func sampleProblem() problem.Problem {
	slot1 := model.TimeSlotID("mon-1")
	slot2 := model.TimeSlotID("mon-2")
	slot3 := model.TimeSlotID("tue-1")
	facultyID := model.FacultyID("fac-smith")
	roomID := model.RoomID("room-101")
	groupID := model.StudentGroupID("group-cs-a")
	offeringID := model.CourseOfferingID("offering-cs101")
	reqID := model.SessionRequirementID("req-cs101-theory")

	return problem.Problem{
		TenantID: "tenant-demo",
		Term: model.Term{
			ID:       "term-2026-fall",
			TenantID: "tenant-demo",
			Name:     "Fall 2026",
		},
		Departments: map[model.DepartmentID]model.Department{
			"dept-cs": {ID: "dept-cs", TenantID: "tenant-demo", Name: "Computer Science"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-btech-cs": {ID: "prog-btech-cs", DepartmentID: "dept-cs", Name: "B.Tech CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-cs-a": {
				ID:              "class-cs-a",
				ProgramID:       "prog-btech-cs",
				Name:            "CS A",
				WholeGroupID:    groupID,
				StudentGroupIDs: []model.StudentGroupID{groupID},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			groupID: {ID: groupID, ClassID: "class-cs-a", Name: "CS A - Whole Class", Size: 40},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-cs101": {ID: "subj-cs101", Code: "CS101", Name: "Programming Fundamentals"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			offeringID: {
				ID:                    offeringID,
				TermID:                "term-2026-fall",
				ClassID:               "class-cs-a",
				SubjectID:             "subj-cs101",
				StudentGroupID:        groupID,
				FacultyID:             facultyID,
				SessionRequirementIDs: []model.SessionRequirementID{reqID},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			reqID: {
				ID:               reqID,
				CourseOfferingID: offeringID,
				Type:             model.SessionTypeTheory,
				SessionsPerWeek:  2,
				Duration:         1,
				Consecutive:      true,
			},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			facultyID: {ID: facultyID, Name: "Dr. Smith"},
		},
		Rooms: map[model.RoomID]model.Room{
			roomID: {ID: roomID, Name: "Room 101", Capacity: 60},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			slot1: {ID: slot1, Day: time.Monday, Period: 1, Label: "Mon P1"},
			slot2: {ID: slot2, Day: time.Monday, Period: 2, Label: "Mon P2"},
			slot3: {ID: slot3, Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: facultyID, TimeSlotID: slot1},
			{FacultyID: facultyID, TimeSlotID: slot2},
			{FacultyID: facultyID, TimeSlotID: slot3},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: roomID, TimeSlotID: slot1},
			{RoomID: roomID, TimeSlotID: slot2},
			{RoomID: roomID, TimeSlotID: slot3},
		},
		PeriodsPerDay: 2,
	}
}
