package tests

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

func localSearchTestProblem() (problem.Problem, problem.Solution) {
	labFeatureID := model.RoomFeatureID("feature-lab")
	p := problem.Problem{
		TenantID: "tenant-ls",
		Term:     model.Term{ID: "term-ls", TenantID: "tenant-ls", Name: "LS Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-ls", Name: "Dept 1"},
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
				TermID:                "term-ls",
				ClassID:               "class-a",
				SubjectID:             "subj-theory",
				StudentGroupID:        "group-a-whole",
				FacultyID:             "faculty-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-theory"},
			},
			"offering-a-lab1": {
				ID:                    "offering-a-lab1",
				TermID:                "term-ls",
				ClassID:               "class-a",
				SubjectID:             "subj-lab",
				StudentGroupID:        "group-a-lab1",
				FacultyID:             "faculty-2",
				SessionRequirementIDs: []model.SessionRequirementID{"req-a-lab1"},
			},
			"offering-b-theory": {
				ID:                    "offering-b-theory",
				TermID:                "term-ls",
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
			"room-small":     {ID: "room-small", Name: "Small", Capacity: 15},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeatureID: {ID: labFeatureID, Name: "Lab"},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"mon-3": {ID: "mon-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"mon-4": {ID: "mon-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"tue-1": {ID: "tue-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
			"tue-2": {ID: "tue-2", Day: time.Tuesday, Period: 2, Label: "Tue P2"},
		},
		PeriodsPerDay: 4,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "faculty-1", TimeSlotID: "mon-1"}, {FacultyID: "faculty-1", TimeSlotID: "mon-2"}, {FacultyID: "faculty-1", TimeSlotID: "mon-3"}, {FacultyID: "faculty-1", TimeSlotID: "mon-4"}, {FacultyID: "faculty-1", TimeSlotID: "tue-1"}, {FacultyID: "faculty-1", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-2", TimeSlotID: "mon-1"}, {FacultyID: "faculty-2", TimeSlotID: "mon-2"}, {FacultyID: "faculty-2", TimeSlotID: "mon-3"}, {FacultyID: "faculty-2", TimeSlotID: "mon-4"}, {FacultyID: "faculty-2", TimeSlotID: "tue-1"}, {FacultyID: "faculty-2", TimeSlotID: "tue-2"},
			{FacultyID: "faculty-3", TimeSlotID: "mon-1"}, {FacultyID: "faculty-3", TimeSlotID: "mon-2"}, {FacultyID: "faculty-3", TimeSlotID: "mon-3"}, {FacultyID: "faculty-3", TimeSlotID: "mon-4"}, {FacultyID: "faculty-3", TimeSlotID: "tue-1"},
			// Note: faculty-3 is unavailable at tue-2
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-1", TimeSlotID: "mon-4"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-1", TimeSlotID: "tue-2"},
			{RoomID: "room-lecture-2", TimeSlotID: "mon-1"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-2"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-3"}, {RoomID: "room-lecture-2", TimeSlotID: "mon-4"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-1"}, {RoomID: "room-lecture-2", TimeSlotID: "tue-2"},
			{RoomID: "room-lab-1", TimeSlotID: "mon-1"}, {RoomID: "room-lab-1", TimeSlotID: "mon-2"}, {RoomID: "room-lab-1", TimeSlotID: "mon-3"}, {RoomID: "room-lab-1", TimeSlotID: "mon-4"}, {RoomID: "room-lab-1", TimeSlotID: "tue-1"}, {RoomID: "room-lab-1", TimeSlotID: "tue-2"},
			{RoomID: "room-small", TimeSlotID: "mon-1"}, {RoomID: "room-small", TimeSlotID: "mon-2"}, {RoomID: "room-small", TimeSlotID: "mon-3"}, {RoomID: "room-small", TimeSlotID: "mon-4"}, {RoomID: "room-small", TimeSlotID: "tue-1"},
			// Note: room-small is unavailable at tue-2
		},
	}
	p.Prepare()

	solution := problem.NewSolution()
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-theory-1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory", Instance: 0})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-theory-2", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-3", SessionRequirementID: "req-a-theory", Instance: 1})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "a-lab-1", CourseOfferingID: "offering-a-lab1", StudentGroupID: "group-a-lab1", FacultyID: "faculty-2", RoomID: "room-lab-1", TimeSlotID: "tue-1", SessionRequirementID: "req-a-lab1", Instance: 0})
	_ = solution.AddAssignment(&p, problem.Assignment{ID: "b-theory", CourseOfferingID: "offering-b-theory", StudentGroupID: "group-b-whole", FacultyID: "faculty-3", RoomID: "room-lecture-2", TimeSlotID: "mon-1", SessionRequirementID: "req-b-theory", Instance: 0})

	return p, solution
}

// -------------------------------------------------------------
// MoveValidator Tests
// -------------------------------------------------------------

func TestMoveValidator_ValidMove(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-theory-2 from mon-3 to mon-2 (now mon-1 and mon-2 -> zero gaps!)
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if !result.Legal {
		t.Fatalf("expected move to be legal, got violations: %+v", result.Violations)
	}
	if result.Score.StudentGapPenalty != 0 {
		t.Fatalf("expected gap penalty 0, got %d", result.Score.StudentGapPenalty)
	}
}

func TestMoveValidator_FacultyConflict(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-theory-2 (faculty-1) to mon-1 (faculty-1 is already teaching a-theory-1 at mon-1)
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to faculty conflict")
	}
	if !hasViolation(result.Violations, "FacultyConflict") {
		t.Fatalf("expected FacultyConflict violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_RoomConflict(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move b-theory from room-lecture-2/mon-1 to room-lecture-1/mon-1 (room-lecture-1 is occupied by a-theory-1 at mon-1)
	move := problem.Move{
		AssignmentID: "b-theory",
		From:         problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to room conflict")
	}
	if !hasViolation(result.Violations, "RoomConflict") {
		t.Fatalf("expected RoomConflict violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_GroupConflict(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-lab-1 (group-a-lab1) from tue-1 to mon-1 (group-a-whole is occupied at mon-1!)
	move := problem.Move{
		AssignmentID: "a-lab-1",
		From:         problem.Placement{RoomID: "room-lab-1", TimeSlotID: "tue-1"},
		To:           problem.Placement{RoomID: "room-lab-1", TimeSlotID: "mon-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to group overlap conflict")
	}
	if !hasViolation(result.Violations, "StudentGroupConflict") {
		t.Fatalf("expected StudentGroupConflict violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_CapacityViolation(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-theory-2 (group-a-whole size 40) to room-small (capacity 15)
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-small", TimeSlotID: "mon-4"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to room capacity violation")
	}
	if !hasViolation(result.Violations, "RoomCapacity") {
		t.Fatalf("expected RoomCapacity violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_RoomFeatureViolation(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-lab-1 (requires feature-lab) to room-lecture-2 (no lab features)
	move := problem.Move{
		AssignmentID: "a-lab-1",
		From:         problem.Placement{RoomID: "room-lab-1", TimeSlotID: "tue-1"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "tue-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to room feature incompatibility")
	}
	if !hasViolation(result.Violations, "RoomFeatureCompatibility") {
		t.Fatalf("expected RoomFeatureCompatibility violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_FacultyAvailabilityViolation(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move b-theory (faculty-3) to tue-2 (faculty-3 is unavailable at tue-2)
	move := problem.Move{
		AssignmentID: "b-theory",
		From:         problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "tue-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to faculty availability violation")
	}
	if !hasViolation(result.Violations, "FacultyAvailability") {
		t.Fatalf("expected FacultyAvailability violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_RoomAvailabilityViolation(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-lab-1 (group-a-lab1 size 20) to room-small at tue-2 (room-small unavailable at tue-2)
	// (Note: remove lab requirement on off-lab to test pure room availability)
	p.SessionRequirements["req-a-lab1"] = model.SessionRequirement{
		ID:               "req-a-lab1",
		CourseOfferingID: "offering-a-lab1",
		Type:             model.SessionTypeTheory,
		SessionsPerWeek:  1,
		Duration:         1,
	}

	move := problem.Move{
		AssignmentID: "a-lab-1",
		From:         problem.Placement{RoomID: "room-lab-1", TimeSlotID: "tue-1"},
		To:           problem.Placement{RoomID: "room-small", TimeSlotID: "tue-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove returned error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be illegal due to room availability violation")
	}
	if !hasViolation(result.Violations, "RoomAvailability") {
		t.Fatalf("expected RoomAvailability violation, got %+v", result.Violations)
	}
}

func TestMoveValidator_LockedAssignmentRejection(t *testing.T) {
	p, solution := localSearchTestProblem()
	p.LockedAssignments = []problem.Assignment{
		{ID: "a-theory-1", CourseOfferingID: "offering-a-theory", StudentGroupID: "group-a-whole", FacultyID: "faculty-1", RoomID: "room-lecture-1", TimeSlotID: "mon-1", SessionRequirementID: "req-a-theory"},
	}

	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Attempt to move locked assignment a-theory-1
	move := problem.Move{
		AssignmentID: "a-theory-1",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if !errors.Is(err, localsearch.ErrLockedAssignment) {
		t.Fatalf("expected ErrLockedAssignment, got err=%v", err)
	}
	if result.Legal {
		t.Fatal("expected locked assignment move to be marked illegal")
	}
}

// -------------------------------------------------------------
// ScoreEvaluator Tests
// -------------------------------------------------------------

func TestScoreEvaluator_KnownScoreCases(t *testing.T) {
	p, solution := localSearchTestProblem()
	evaluator := localsearch.FullScoreEvaluator{}

	// Current solution has a-theory-1 at mon-1 and a-theory-2 at mon-3 -> 1 gap (mon-2)
	score := evaluator.Evaluate(&p, &solution)
	if score.StudentGapPenalty != 1 {
		t.Fatalf("expected initial gap penalty 1, got %d", score.StudentGapPenalty)
	}
}

func TestScoreEvaluator_ScoreChangesAfterLegalMove(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-theory-2 from mon-3 to mon-2 -> gap eliminated (score becomes 0)
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove error: %v", err)
	}
	if !result.Legal {
		t.Fatalf("expected move to be legal, got violations: %+v", result.Violations)
	}
	if result.Score.StudentGapPenalty != 0 {
		t.Fatalf("expected score to change to 0, got %d", result.Score.StudentGapPenalty)
	}
}

func TestScoreEvaluator_IllegalMoveNeverScored(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()

	mockEvaluatorCalled := false
	mockEvaluator := mockScoreEvaluator{
		evalFunc: func(p *problem.Problem, solution *problem.Solution) {
			mockEvaluatorCalled = true
		},
	}

	// Move that causes faculty conflict
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, mockEvaluator)
	if err != nil {
		t.Fatalf("EvaluateMove error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected illegal move")
	}
	if mockEvaluatorCalled {
		t.Fatal("ScoreEvaluator was called on an illegal move; illegal moves must NOT be scored")
	}
}

type mockScoreEvaluator struct {
	evalFunc func(p *problem.Problem, solution *problem.Solution)
}

func (m mockScoreEvaluator) Evaluate(p *problem.Problem, solution *problem.Solution) scorer.ScoreBreakdown {
	if m.evalFunc != nil {
		m.evalFunc(p, solution)
	}
	return scorer.ScoreBreakdown{}
}

func TestScoreEvaluator_DoesNotMutateSolutionOrIndex(t *testing.T) {
	p, solution := localSearchTestProblem()
	evaluator := localsearch.FullScoreEvaluator{}

	assignmentsBefore := append([]problem.Assignment(nil), solution.Assignments...)
	_ = evaluator.Evaluate(&p, &solution)

	if !reflect.DeepEqual(solution.Assignments, assignmentsBefore) {
		t.Fatal("ScoreEvaluator modified solution.Assignments")
	}
}

// -------------------------------------------------------------
// Apply/Undo Property Tests
// -------------------------------------------------------------

func TestApplyUndo_ExactStateEquality(t *testing.T) {
	p, solution := localSearchTestProblem()

	// Snapshot original state
	assignmentsBefore := append([]problem.Assignment(nil), solution.Assignments...)
	facultySlotBefore := copyResourceMap(solution.Index.FacultySlot)
	roomSlotBefore := copyResourceMap(solution.Index.RoomSlot)
	groupSlotBefore := copyResourceMap(solution.Index.StudentGroupSlot)
	reqCountBefore := copyReqCountMap(solution.Index.RequirementCount)

	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	// 1. Apply move
	if err := solution.ApplyMove(&p, move); err != nil {
		t.Fatalf("ApplyMove error: %v", err)
	}

	// Verify state changed
	if reflect.DeepEqual(solution.Assignments, assignmentsBefore) {
		t.Fatal("solution.Assignments did not change after ApplyMove")
	}

	// 2. Undo move
	if err := solution.UndoMove(&p, move); err != nil {
		t.Fatalf("UndoMove error: %v", err)
	}

	// 3. Exact state equality verification
	if !reflect.DeepEqual(solution.Assignments, assignmentsBefore) {
		t.Fatalf("Assignments mismatch after undo:\nGot:  %+v\nWant: %+v", solution.Assignments, assignmentsBefore)
	}
	if !reflect.DeepEqual(solution.Index.FacultySlot, facultySlotBefore) {
		t.Fatalf("FacultySlot mismatch after undo:\nGot:  %+v\nWant: %+v", solution.Index.FacultySlot, facultySlotBefore)
	}
	if !reflect.DeepEqual(solution.Index.RoomSlot, roomSlotBefore) {
		t.Fatalf("RoomSlot mismatch after undo:\nGot:  %+v\nWant: %+v", solution.Index.RoomSlot, roomSlotBefore)
	}
	if !reflect.DeepEqual(solution.Index.StudentGroupSlot, groupSlotBefore) {
		t.Fatalf("StudentGroupSlot mismatch after undo:\nGot:  %+v\nWant: %+v", solution.Index.StudentGroupSlot, groupSlotBefore)
	}
	if !reflect.DeepEqual(solution.Index.RequirementCount, reqCountBefore) {
		t.Fatalf("RequirementCount mismatch after undo:\nGot:  %+v\nWant: %+v", solution.Index.RequirementCount, reqCountBefore)
	}
}

func TestApplyUndo_InvalidMoveLeavesIndexUnchanged(t *testing.T) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	// Snapshot original state
	assignmentsBefore := append([]problem.Assignment(nil), solution.Assignments...)
	facultySlotBefore := copyResourceMap(solution.Index.FacultySlot)
	roomSlotBefore := copyResourceMap(solution.Index.RoomSlot)
	groupSlotBefore := copyResourceMap(solution.Index.StudentGroupSlot)

	// Move causing conflict
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected illegal move")
	}

	// Verify index is 100% unchanged
	if !reflect.DeepEqual(solution.Assignments, assignmentsBefore) {
		t.Fatal("solution.Assignments changed after evaluating illegal move")
	}
	if !reflect.DeepEqual(solution.Index.FacultySlot, facultySlotBefore) {
		t.Fatal("FacultySlot changed after evaluating illegal move")
	}
	if !reflect.DeepEqual(solution.Index.RoomSlot, roomSlotBefore) {
		t.Fatal("RoomSlot changed after evaluating illegal move")
	}
	if !reflect.DeepEqual(solution.Index.StudentGroupSlot, groupSlotBefore) {
		t.Fatal("StudentGroupSlot changed after evaluating illegal move")
	}
}

func copyResourceMap[K comparable, V comparable](m map[K]V) map[K]V {
	copied := make(map[K]V, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

func copyReqCountMap(m map[model.SessionRequirementID]int) map[model.SessionRequirementID]int {
	copied := make(map[model.SessionRequirementID]int, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

func BenchmarkEvaluateMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	validator := localsearch.NewMoveValidator()
	evaluator := localsearch.FullScoreEvaluator{}

	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	}
}
