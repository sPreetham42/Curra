package verifier_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/verifier"
)

// ---------------------------------------------------------------------------
// Authoritative verification must not depend on solver-maintained Solution.Index
// ---------------------------------------------------------------------------
//
// FacultyConflict, RoomConflict and StudentGroupConflict answer conflict questions
// through Solution.Index. That index is solver-maintained state: it carries
// `json:"-"`, so it is empty on every deserialized solution, and it is empty on any
// solution assembled without incremental AddAssignment calls. The invariant proved
// here is that an invalid solution never becomes valid merely because Solution.Index
// is missing.

// conflictProblem builds a two-offering problem with full mutual availability so
// that resource conflicts can be injected in isolation, without dragging in
// capacity, feature or availability violations.
func conflictProblem(t *testing.T) problem.Problem {
	t.Helper()

	p := problem.Problem{
		TenantID: "tenant-conflict",
		Term:     model.Term{ID: "term-conflict", TenantID: "tenant-conflict", Name: "Conflict Term"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-conflict", Name: "Dept"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "Prog"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Class 1", WholeGroupID: "group-1", StudentGroupIDs: []model.StudentGroupID{"group-1"}},
			"class-2": {ID: "class-2", ProgramID: "prog-1", Name: "Class 2", WholeGroupID: "group-2", StudentGroupIDs: []model.StudentGroupID{"group-2"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-1": {ID: "group-1", ClassID: "class-1", Name: "G1", Size: 20},
			"group-2": {ID: "group-2", ClassID: "class-2", Name: "G2", Size: 20},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-1": {ID: "subj-1", Code: "S1", Name: "Subject 1"},
			"subj-2": {ID: "subj-2", Code: "S2", Name: "Subject 2"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"off-1": {ID: "off-1", TermID: "term-conflict", ClassID: "class-1", SubjectID: "subj-1", StudentGroupID: "group-1", FacultyID: "fac-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
			"off-2": {ID: "off-2", TermID: "term-conflict", ClassID: "class-2", SubjectID: "subj-2", StudentGroupID: "group-2", FacultyID: "fac-2", SessionRequirementIDs: []model.SessionRequirementID{"req-2"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "off-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-2": {ID: "req-2", CourseOfferingID: "off-2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"fac-1": {ID: "fac-1", Name: "Faculty 1"},
			"fac-2": {ID: "fac-2", Name: "Faculty 2"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-1": {ID: "room-1", Name: "Room 1", Capacity: 50},
			"room-2": {ID: "room-2", Name: "Room 2", Capacity: 50},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"mon-1": {ID: "mon-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"mon-2": {ID: "mon-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
	}

	for _, facultyID := range []model.FacultyID{"fac-1", "fac-2"} {
		for _, slotID := range []model.TimeSlotID{"mon-1", "mon-2"} {
			p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{FacultyID: facultyID, TimeSlotID: slotID})
		}
	}
	for _, roomID := range []model.RoomID{"room-1", "room-2"} {
		for _, slotID := range []model.TimeSlotID{"mon-1", "mon-2"} {
			p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: roomID, TimeSlotID: slotID})
		}
	}

	p.Prepare()
	if violations := problem.Validate(p); len(violations) != 0 {
		t.Fatalf("conflict fixture must validate clean, got %+v", violations)
	}
	return p
}

func hasConstraintViolation(violations []diagnostics.Violation, name string) bool {
	for _, v := range violations {
		if v.ConstraintName == name {
			return true
		}
	}
	return false
}

// unindexedSolution returns a Solution built the way a deserialized or externally
// assembled result arrives: raw assignments, zero-value (nil-map) Index.
func unindexedSolution(assignments ...problem.Assignment) problem.Solution {
	return problem.Solution{
		Assignments: assignments,
		Index:       problem.SolutionIndex{},
	}
}

// reportCleanScore makes the solution's own score self-consistent while still
// claiming zero hard violations — the shape a buggy or hostile producer emits.
// With the soft breakdown correct, the resource conflict is the ONLY defect left,
// so nothing but conflict detection can make verification fail.
func reportCleanScore(p *problem.Problem, sol *problem.Solution) {
	breakdown := p.CalculateScore(sol)
	sol.Score.HardViolations = 0
	sol.Score.SoftPenalty = breakdown.SoftPenalty
	sol.Score.Breakdown = breakdown
}

func TestVerifier_UnpopulatedIndex_DetectsFacultyDoubleBooking(t *testing.T) {
	p := conflictProblem(t)
	// Both offerings taught by fac-1, different groups, different rooms, same slot:
	// a faculty double-booking and nothing else.
	off2 := p.CourseOfferings["off-2"]
	off2.FacultyID = "fac-1"
	p.CourseOfferings["off-2"] = off2

	sol := unindexedSolution(
		problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
		problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-1", RoomID: "room-2", TimeSlotID: "mon-1", SessionRequirementID: "req-2"},
	)
	if len(sol.Index.FacultySlot) != 0 {
		t.Fatal("test precondition: Solution.Index must be unpopulated")
	}
	reportCleanScore(&p, &sol)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("faculty double-booking verified as valid with an unpopulated Solution.Index")
	}
	if !hasConstraintViolation(report.Violations, "FacultyConflict") {
		t.Fatalf("expected a FacultyConflict violation, got %+v", report.Violations)
	}
	if report.Status == diagnostics.SolveStatusSolved {
		t.Fatal("a solution carrying a faculty double-booking must never report SOLVED")
	}
}

func TestVerifier_UnpopulatedIndex_DetectsRoomDoubleBooking(t *testing.T) {
	p := conflictProblem(t)

	sol := unindexedSolution(
		problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
		problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-2"},
	)
	reportCleanScore(&p, &sol)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("room double-booking verified as valid with an unpopulated Solution.Index")
	}
	if !hasConstraintViolation(report.Violations, "RoomConflict") {
		t.Fatalf("expected a RoomConflict violation, got %+v", report.Violations)
	}
	if report.Status == diagnostics.SolveStatusSolved {
		t.Fatal("a solution carrying a room double-booking must never report SOLVED")
	}
}

func TestVerifier_UnpopulatedIndex_DetectsStudentGroupDoubleBooking(t *testing.T) {
	p := conflictProblem(t)
	// Both offerings taught to group-1 by different faculty in different rooms at the
	// same slot: a student-group double-booking and nothing else.
	off2 := p.CourseOfferings["off-2"]
	off2.ClassID = "class-1"
	off2.StudentGroupID = "group-1"
	p.CourseOfferings["off-2"] = off2

	sol := unindexedSolution(
		problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
		problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-1", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "mon-1", SessionRequirementID: "req-2"},
	)
	reportCleanScore(&p, &sol)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("student-group double-booking verified as valid with an unpopulated Solution.Index")
	}
	if !hasConstraintViolation(report.Violations, "StudentGroupConflict") {
		t.Fatalf("expected a StudentGroupConflict violation, got %+v", report.Violations)
	}
	if report.Status == diagnostics.SolveStatusSolved {
		t.Fatal("a solution carrying a student-group double-booking must never report SOLVED")
	}
}

// TestVerifier_UnpopulatedIndex_ValidSolutionStillPasses guards the rebuilt index
// against false positives: a conflict-free solution with no index must verify clean.
func TestVerifier_UnpopulatedIndex_ValidSolutionStillPasses(t *testing.T) {
	p := conflictProblem(t)

	sol := unindexedSolution(
		problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
		problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "mon-2", SessionRequirementID: "req-2"},
	)

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})
	if err != nil || !report.Valid {
		t.Fatalf("conflict-free unindexed solution failed verification: %v (violations: %+v)", err, report.Violations)
	}
}

// TestVerifier_StaleIndex_DoesNotMaskConflict proves verification does not trust a
// populated-but-wrong index either: a hand-forged index claiming both assignments
// own their slots must not suppress the real room conflict.
func TestVerifier_StaleIndex_DoesNotMaskConflict(t *testing.T) {
	p := conflictProblem(t)

	clean := problem.NewSolution()
	if err := clean.AddAssignment(&p, problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"}); err != nil {
		t.Fatalf("seed a1: %v", err)
	}
	if err := clean.AddAssignment(&p, problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-2", TimeSlotID: "mon-2", SessionRequirementID: "req-2"}); err != nil {
		t.Fatalf("seed a2: %v", err)
	}

	// Move a2 on top of a1's room and slot in the raw assignments only, leaving the
	// index describing the previous, conflict-free placement.
	clean.Assignments[1].RoomID = "room-1"
	clean.Assignments[1].TimeSlotID = "mon-1"

	report, err := verifier.VerifySolution(&p, &clean, verifier.VerifyOptions{})
	if err == nil || report.Valid {
		t.Fatal("room conflict masked by a stale Solution.Index")
	}
	if !hasConstraintViolation(report.Violations, "RoomConflict") {
		t.Fatalf("expected a RoomConflict violation, got %+v", report.Violations)
	}
}

// TestVerifier_JSONRoundTrippedConflictingSolution_Rejected exercises the exact
// production path: Solution.Index is `json:"-"`, so a solution reconstructed from
// JSON arrives with no index at all.
func TestVerifier_JSONRoundTrippedConflictingSolution_Rejected(t *testing.T) {
	p := conflictProblem(t)

	original := problem.Solution{
		Assignments: []problem.Assignment{
			{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
			{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-2"},
		},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal solution: %v", err)
	}

	var restored problem.Solution
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal solution: %v", err)
	}
	if restored.Index.RoomSlot != nil {
		t.Fatal("test precondition: a deserialized Solution must carry no index")
	}

	report, vErr := verifier.VerifySolution(&p, &restored, verifier.VerifyOptions{})
	if vErr == nil || report.Valid {
		t.Fatal("JSON-deserialized conflicting solution verified as valid")
	}
	if !hasConstraintViolation(report.Violations, "RoomConflict") {
		t.Fatalf("expected a RoomConflict violation, got %+v", report.Violations)
	}
}

// TestVerifier_VerificationDoesNotMutateSolution keeps the verifier a pure
// read-only oracle: rebuilding the occupancy index must not write back into the
// caller's solution.
func TestVerifier_VerificationDoesNotMutateSolution(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)
	sol.Index = problem.SolutionIndex{}

	if _, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{}); err != nil {
		t.Fatalf("valid solution failed verification with a wiped index: %v", err)
	}
	if sol.Index.FacultySlot != nil || sol.Index.RoomSlot != nil || sol.Index.StudentGroupSlot != nil || sol.Index.RequirementCount != nil {
		t.Fatal("verification mutated the caller's Solution.Index")
	}
}

// TestVerifier_SolvedInstance_WipedIndex_StillVerifies is the end-to-end guard: a
// genuine solver output must verify identically with and without its index.
func TestVerifier_SolvedInstance_WipedIndex_StillVerifies(t *testing.T) {
	p, sol := getSolvedSmallInstance(t)

	withIndex, errWith := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{})

	wiped := sol
	wiped.Index = problem.SolutionIndex{}
	withoutIndex, errWithout := verifier.VerifySolution(&p, &wiped, verifier.VerifyOptions{})

	if errWith != nil || errWithout != nil {
		t.Fatalf("solved instance failed verification: with index %v, without index %v", errWith, errWithout)
	}
	if withIndex.Valid != withoutIndex.Valid || withIndex.Status != withoutIndex.Status {
		t.Fatalf("verification outcome depends on Solution.Index: %+v vs %+v", withIndex, withoutIndex)
	}
}

// TestVerifier_UnpopulatedIndex_CompiledConstraintSet covers the compiled-rule path
// through the same rebuilt index.
func TestVerifier_UnpopulatedIndex_CompiledConstraintSet(t *testing.T) {
	p := conflictProblem(t)

	sol := unindexedSolution(
		problem.Assignment{ID: "a1", CourseOfferingID: "off-1", StudentGroupID: "group-1", FacultyID: "fac-1", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-1"},
		problem.Assignment{ID: "a2", CourseOfferingID: "off-2", StudentGroupID: "group-2", FacultyID: "fac-2", RoomID: "room-1", TimeSlotID: "mon-1", SessionRequirementID: "req-2"},
	)

	compiled, _, compileErrs := constraints.Compile(&p, []constraints.ConstraintInstance{
		{ID: "faculty-conflict", TemplateID: "FacultyConflict", Kind: constraints.ConstraintKindHard},
		{ID: "room-conflict", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard},
		{ID: "student-group-conflict", TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard},
	})
	if len(compileErrs) > 0 {
		t.Fatalf("unexpected compile errors: %v", compileErrs)
	}

	report, err := verifier.VerifySolution(&p, &sol, verifier.VerifyOptions{Compiled: compiled})
	if err == nil || report.Valid {
		t.Fatal("compiled-rule verification accepted a room conflict with an unpopulated index")
	}
	if !hasConstraintViolation(report.Violations, "RoomConflict") {
		t.Fatalf("expected a RoomConflict violation, got %+v", report.Violations)
	}
}
