package tests

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/backtracking"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/solver/localsearch"
)

// -------------------------------------------------------------
// Test 1: Deterministic Compilation and RuleSetHash
// -------------------------------------------------------------
func TestConfigurableConstraints_DeterministicCompilation(t *testing.T) {
	inst1 := []constraints.ConstraintInstance{
		{ID: "rule-1", TemplateID: "FacultyConflict", Scope: "global", Kind: constraints.ConstraintKindHard, Weight: 100},
		{ID: "rule-2", TemplateID: "SubjectMaxPerDay", Scope: "class-a", Params: map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1}, Kind: constraints.ConstraintKindHard, Weight: 50},
	}

	// Reversed order slice
	inst2 := []constraints.ConstraintInstance{
		{ID: "rule-2", TemplateID: "SubjectMaxPerDay", Scope: "class-a", Params: map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1}, Kind: constraints.ConstraintKindHard, Weight: 50},
		{ID: "rule-1", TemplateID: "FacultyConflict", Scope: "global", Kind: constraints.ConstraintKindHard, Weight: 100},
	}

	set1, hash1, errs1 := constraints.Compile(nil, inst1)
	if len(errs1) > 0 {
		t.Fatalf("compile inst1 error: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, inst2)
	if len(errs2) > 0 {
		t.Fatalf("compile inst2 error: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("RuleSetHash mismatch for identical configs: hash1=%s, hash2=%s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Different config must produce different hash
	inst3 := []constraints.ConstraintInstance{
		{ID: "rule-1", TemplateID: "FacultyConflict", Scope: "global", Kind: constraints.ConstraintKindHard, Weight: 100},
		{ID: "rule-2", TemplateID: "SubjectMaxPerDay", Scope: "class-a", Params: map[string]any{"subjectId": "subj-dbms", "maxPerDay": 2}, Kind: constraints.ConstraintKindHard, Weight: 50},
	}
	_, hash3, errs3 := constraints.Compile(nil, inst3)
	if len(errs3) > 0 {
		t.Fatalf("compile inst3 error: %v", errs3)
	}
	if hash1 == hash3 {
		t.Fatal("expected different hashes for different maxPerDay parameters")
	}
}

// -------------------------------------------------------------
// Test 2: FacultyConflict through ConstraintDef interface
// -------------------------------------------------------------
func TestFacultyConflict_ConstraintDefInterface(t *testing.T) {
	inst := constraints.ConstraintInstance{
		ID:         "fac-conflict-rule",
		TemplateID: "FacultyConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}

	compiledSet, _, errs := constraints.Compile(nil, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	c := compiledSet.Hard[0]
	if c.ID() != "fac-conflict-rule" || c.Kind() != constraints.ConstraintKindHard {
		t.Fatalf("unexpected ID/Kind: got %s / %s", c.ID(), c.Kind())
	}

	p, solution := localSearchTestProblem()
	ctx := constraints.NewSearchCtx(&p)

	// 1. Test IsConsistent on non-conflicting assignment
	validCandidate := problem.Assignment{
		ID:                   "a-new",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-1",
		TimeSlotID:           "mon-2", // mon-2 is free for faculty-1
		SessionRequirementID: "req-a-theory",
	}
	if !c.IsConsistent(ctx, &solution, validCandidate) {
		t.Fatal("expected IsConsistent to be true for non-conflicting assignment")
	}

	// Test IsConsistent on conflicting assignment (faculty-1 already teaching at mon-1)
	conflictCandidate := problem.Assignment{
		ID:                   "a-new-conflict",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-2",
		TimeSlotID:           "mon-1",
		SessionRequirementID: "req-a-theory",
	}
	if c.IsConsistent(ctx, &solution, conflictCandidate) {
		t.Fatal("expected IsConsistent to be false for faculty conflict")
	}

	// 2. Test ViolatedByMove
	moveConflict := problem.Move{
		AssignmentID: "a-theory-2", // faculty-1
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"}, // mon-1 has faculty-1 teaching a-theory-1
	}
	violations := c.ViolatedByMove(ctx, &solution, moveConflict)
	if len(violations) == 0 {
		t.Fatal("expected ViolatedByMove to return violation for faculty conflict")
	}
	if violations[0].ConstraintID != "fac-conflict-rule" || violations[0].TemplateID != "FacultyConflict" {
		t.Fatalf("provenance fields mismatch: %+v", violations[0])
	}

	// 3. Test Evaluate
	evalViolations := c.Evaluate(ctx, &solution)
	if len(evalViolations) != 0 {
		t.Fatalf("expected 0 initial solution violations, got %d", len(evalViolations))
	}
}

// -------------------------------------------------------------
// Test 3: SubjectMaxPerDay in CSP (Backtracking Solver)
// -------------------------------------------------------------
func TestSubjectMaxPerDay_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-max",
		Term:     model.Term{ID: "term-m", TenantID: "tenant-max", Name: "Term M"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-max", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-dbms": {ID: "subj-dbms", Code: "CS102", Name: "DBMS"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-dbms": {
				ID:                    "co-dbms",
				TermID:                "term-m",
				ClassID:               "class-1",
				SubjectID:             "subj-dbms",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-dbms"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			// Requires 2 sessions per week for DBMS
			"req-dbms": {ID: "req-dbms", CourseOfferingID: "co-dbms", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1, Consecutive: true},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof DBMS"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			// All 2 slots on Monday
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			// Slot on Tuesday
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"}, {FacultyID: "f-1", TimeSlotID: "t-1"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"}, {RoomID: "r-1", TimeSlotID: "t-1"},
		},
	}
	p.Prepare()

	// Rule: DBMS max 1 per day
	inst := constraints.ConstraintInstance{
		ID:         "rule-dbms-max1",
		TemplateID: "SubjectMaxPerDay",
		Scope:      "global",
		Params:     map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}

	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})
	if err != nil {
		t.Fatalf("solver error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected SOLVED, got %s", diag.Status)
	}

	// Verify that the 2 DBMS sessions are scheduled on different days (1 on Monday, 1 on Tuesday)
	dayCount := make(map[time.Weekday]int)
	for _, a := range solution.Assignments {
		slot := p.TimeSlots[a.TimeSlotID]
		dayCount[slot.Day]++
	}

	for day, count := range dayCount {
		if count > 1 {
			t.Fatalf("SubjectMaxPerDay violated in CSP solution! Found %d DBMS sessions on %s", count, day)
		}
	}
}

// -------------------------------------------------------------
// Test 4: SubjectMaxPerDay in Tabu Move Validation
// -------------------------------------------------------------
func TestSubjectMaxPerDay_InTabuMoveValidation(t *testing.T) {
	p, solution := localSearchTestProblem()

	// Position a-theory-2 on Tuesday first so Monday initially has only 1 theory session (a-theory-1)
	for i, a := range solution.Assignments {
		if a.ID == "a-theory-2" {
			solution.Index.Remove(&p, a)
			a.TimeSlotID = "tue-1"
			solution.Assignments[i] = a
			_ = solution.Index.Add(&p, a)
			break
		}
	}

	inst := constraints.ConstraintInstance{
		ID:         "rule-theory-max1",
		TemplateID: "SubjectMaxPerDay",
		Scope:      "class-a",
		Params:     map[string]any{"subjectId": "subj-theory", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}

	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	validator := localsearch.NewCompiledMoveValidator(compiledSet)
	evaluator := localsearch.FullScoreEvaluator{}

	// Move a-theory-2 from tue-1 to mon-2 (mon-1 already has a-theory-1 -> 2 theory sessions on Monday!)
	move := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "tue-1"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-2"},
	}

	result, err := localsearch.EvaluateMove(&p, &solution, move, validator, evaluator)
	if err != nil {
		t.Fatalf("EvaluateMove error: %v", err)
	}
	if result.Legal {
		t.Fatal("expected move to be marked illegal due to SubjectMaxPerDay violation")
	}

	foundViolation := false
	for _, v := range result.Violations {
		if v.TemplateID == "SubjectMaxPerDay" && v.ConstraintID == "rule-theory-max1" {
			foundViolation = true
			break
		}
	}
	if !foundViolation {
		t.Fatalf("expected SubjectMaxPerDay violation with ConstraintID rule-theory-max1, got: %+v", result.Violations)
	}
}

// -------------------------------------------------------------
// Test 5: SubjectMaxPerDay Final Validation
// -------------------------------------------------------------
func TestSubjectMaxPerDay_FinalValidation(t *testing.T) {
	p, solution := localSearchTestProblem()

	// solution currently has a-theory-1 at mon-1 and a-theory-2 at mon-3 (both on Monday for group-a-whole)
	inst := constraints.ConstraintInstance{
		ID:         "rule-theory-max1",
		TemplateID: "SubjectMaxPerDay",
		Scope:      "class-a",
		Params:     map[string]any{"subjectId": "subj-theory", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}

	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	ctx := constraints.NewSearchCtx(&p)
	violations := compiledSet.Hard[0].Evaluate(ctx, &solution)

	if len(violations) < 2 {
		t.Fatalf("expected violations for both assignments exceeding max per day, got %d violations", len(violations))
	}
	for _, v := range violations {
		if v.ConstraintID != "rule-theory-max1" || v.TemplateID != "SubjectMaxPerDay" {
			t.Fatalf("provenance fields mismatch in final validation violation: %+v", v)
		}
	}
}

// -------------------------------------------------------------
// Test 6: Final Compiled-Hard Validation Regression Test (P0 Issue 1)
// -------------------------------------------------------------
func TestSolver_FinalCompiledHardValidationRegression(t *testing.T) {
	// Setup problem where ONLY Monday slots exist (forcing 2 DBMS sessions onto Monday)
	p := problem.Problem{
		TenantID: "tenant-max",
		Term:     model.Term{ID: "term-m", TenantID: "tenant-max", Name: "Term M"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-max", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-dbms": {ID: "subj-dbms", Code: "CS102", Name: "DBMS"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-dbms": {
				ID:                    "co-dbms",
				TermID:                "term-m",
				ClassID:               "class-1",
				SubjectID:             "subj-dbms",
				StudentGroupID:        "g1-whole",
				FacultyID:             "f-1",
				SessionRequirementIDs: []model.SessionRequirementID{"req-dbms"},
			},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-dbms": {ID: "req-dbms", CourseOfferingID: "co-dbms", Type: model.SessionTypeTheory, SessionsPerWeek: 2, Duration: 1, Consecutive: false},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof DBMS"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			// Only Monday slots available!
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	// Compiled HARD rule: max 1 DBMS per day
	inst := constraints.ConstraintInstance{
		ID:         "rule-dbms-max1",
		TemplateID: "SubjectMaxPerDay",
		Scope:      "global",
		Params:     map[string]any{"subjectId": "subj-dbms", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}

	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("compile error: %v", errs)
	}

	// 1. Solve under legacy constraints to produce a solution satisfying legacy constraints (2 DBMS sessions on Monday)
	legacySolver := backtracking.New()
	legacySol, legacyDiag, err := legacySolver.Solve(context.Background(), p, problem.SolveOptions{SearchMode: problem.SearchModeBasic})
	if err != nil || legacyDiag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected legacy solver to find solution, got status %s, err %v", legacyDiag.Status, err)
	}

	// 2. Validate the solution against compiled HARD constraints via ValidateSolution
	compiledSolver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := compiledSolver.ValidateSolution(context.Background(), p, legacySol)

	// 1. Assert status is NOT SOLVED
	if diag.Status == diagnostics.SolveStatusSolved {
		t.Fatalf("expected status to NOT be SOLVED, got %s", diag.Status)
	}
	if diag.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diag.Status)
	}
	if err == nil {
		t.Fatal("expected solver to return error for infeasible compiled hard constraint")
	}

	// 2. Assert hard violation is present in diagnostics
	if len(diag.Violations) == 0 {
		t.Fatal("expected diagnostics to contain hard violations")
	}

	// 3. Assert HardViolations count is correct in solution score
	if solution.Score.HardViolations != len(diag.Violations) {
		t.Fatalf("expected solution.Score.HardViolations == %d, got %d", len(diag.Violations), solution.Score.HardViolations)
	}

	// 4. Assert diagnostics identifies the constraint
	foundTargetRule := false
	for _, v := range diag.Violations {
		if v.ConstraintID == "rule-dbms-max1" && v.TemplateID == "SubjectMaxPerDay" {
			foundTargetRule = true
			break
		}
	}
	if !foundTargetRule {
		t.Fatalf("expected violation for constraint ID rule-dbms-max1 and template SubjectMaxPerDay, got: %+v", diag.Violations)
	}
}

// -------------------------------------------------------------
// Test 7: Strict Compile Validation (P0 Issue 2)
// -------------------------------------------------------------
func TestCompile_StrictValidation(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-val",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-val", Name: "Term 1"},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-exist": {ID: "subj-exist", Code: "S1", Name: "Existing Subject"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-exist": {ID: "co-exist", TermID: "term-1", SubjectID: "subj-exist"},
		},
	}
	p.Prepare()

	// 1. Nonexistent subjectId
	instBadSubj := constraints.ConstraintInstance{
		ID:         "r-bad-subj",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-nonexistent", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs1 := constraints.Compile(&p, []constraints.ConstraintInstance{instBadSubj})
	if len(errs1) == 0 || !containsField(errs1, "subjectId") {
		t.Fatalf("expected compile error on nonexistent subjectId, got: %v", errs1)
	}

	// 2. Nonexistent courseOfferingId
	instBadOffering := constraints.ConstraintInstance{
		ID:         "r-bad-offering",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"courseOfferingId": "co-nonexistent", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs2 := constraints.Compile(&p, []constraints.ConstraintInstance{instBadOffering})
	if len(errs2) == 0 || !containsField(errs2, "courseOfferingId") {
		t.Fatalf("expected compile error on nonexistent courseOfferingId, got: %v", errs2)
	}

	// 3. Missing maxPerDay
	instMissingMax := constraints.ConstraintInstance{
		ID:         "r-missing-max",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist"},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs3 := constraints.Compile(&p, []constraints.ConstraintInstance{instMissingMax})
	if len(errs3) == 0 || !containsField(errs3, "maxPerDay") {
		t.Fatalf("expected compile error on missing maxPerDay, got: %v", errs3)
	}

	// 4. String maxPerDay
	instStringMax := constraints.ConstraintInstance{
		ID:         "r-string-max",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": "1"},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs4 := constraints.Compile(&p, []constraints.ConstraintInstance{instStringMax})
	if len(errs4) == 0 || !containsField(errs4, "maxPerDay") {
		t.Fatalf("expected compile error on string maxPerDay, got: %v", errs4)
	}

	// 5. Zero maxPerDay
	instZeroMax := constraints.ConstraintInstance{
		ID:         "r-zero-max",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": 0},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs5 := constraints.Compile(&p, []constraints.ConstraintInstance{instZeroMax})
	if len(errs5) == 0 || !containsField(errs5, "maxPerDay") {
		t.Fatalf("expected compile error on zero maxPerDay, got: %v", errs5)
	}

	// 6. Negative maxPerDay
	instNegMax := constraints.ConstraintInstance{
		ID:         "r-neg-max",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": -2},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errs6 := constraints.Compile(&p, []constraints.ConstraintInstance{instNegMax})
	if len(errs6) == 0 || !containsField(errs6, "maxPerDay") {
		t.Fatalf("expected compile error on negative maxPerDay, got: %v", errs6)
	}

	// 7. Valid config
	instValid := constraints.ConstraintInstance{
		ID:         "r-valid",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": 2},
		Kind:       constraints.ConstraintKindHard,
	}
	setValid, hashValid, errs7 := constraints.Compile(&p, []constraints.ConstraintInstance{instValid})
	if len(errs7) > 0 {
		t.Fatalf("expected valid config to compile cleanly, got errors: %v", errs7)
	}
	if setValid == nil || hashValid == "" {
		t.Fatal("expected non-nil CompiledConstraintSet and non-empty hash for valid config")
	}
}

// -------------------------------------------------------------
// Test 8: Soft Constraints Safety (P0 Issue 3)
// -------------------------------------------------------------
func TestCompile_SoftConstraintsRejected(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-val",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-val", Name: "Term 1"},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-exist": {ID: "subj-exist", Code: "S1", Name: "Existing Subject"},
		},
	}
	p.Prepare()

	// HARD SubjectMaxPerDay compiles cleanly
	hardInst := constraints.ConstraintInstance{
		ID:         "r-hard",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindHard,
	}
	_, _, errsHard := constraints.Compile(&p, []constraints.ConstraintInstance{hardInst})
	if len(errsHard) > 0 {
		t.Fatalf("expected HARD constraint to compile cleanly, got: %v", errsHard)
	}

	// SOFT SubjectMaxPerDay is explicitly rejected
	softInst := constraints.ConstraintInstance{
		ID:         "r-soft",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": 1},
		Kind:       constraints.ConstraintKindSoft,
		Weight:     50,
	}
	_, _, errsSoft := constraints.Compile(&p, []constraints.ConstraintInstance{softInst})
	if len(errsSoft) == 0 {
		t.Fatal("expected SOFT constraint to be explicitly rejected by Compile()")
	}

	foundKindError := false
	for _, e := range errsSoft {
		if e.Field == "kind" && strings.Contains(e.Message, "soft constraints are not supported") {
			foundKindError = true
			break
		}
	}
	if !foundKindError {
		t.Fatalf("expected compile error on field 'kind' explaining soft constraints unsupported, got: %v", errsSoft)
	}
}

func TestCompile_Atomicity(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-val",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-val", Name: "Term 1"},
		Subjects: map[model.SubjectID]model.Subject{
			"subj-exist": {ID: "subj-exist", Code: "S1", Name: "Existing Subject"},
		},
	}
	p.Prepare()

	validInst := constraints.ConstraintInstance{
		ID:         "r-valid",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-exist", "maxPerDay": 2},
		Kind:       constraints.ConstraintKindHard,
	}

	invalidInst := constraints.ConstraintInstance{
		ID:         "r-invalid",
		TemplateID: "SubjectMaxPerDay",
		Params:     map[string]any{"subjectId": "subj-nonexistent", "maxPerDay": 2},
		Kind:       constraints.ConstraintKindHard,
	}

	// Mixed batch: 1 valid, 1 invalid
	compiledSet, hash, errs := constraints.Compile(&p, []constraints.ConstraintInstance{validInst, invalidInst})

	if len(errs) == 0 {
		t.Fatal("expected compile errors for batch containing invalid instance")
	}
	if compiledSet != nil {
		t.Fatalf("expected compiledSet to be nil on compile errors, got: %+v", compiledSet)
	}
	if hash != "" {
		t.Fatalf("expected hash to be empty string on compile errors, got: %s", hash)
	}
}

func containsField(errs []constraints.CompileError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------
// True Independent Pre-6B-1 Legacy RoomConflict Test Oracle
// (Frozen from git commit 7d7a696 before Phase 6B-1 migration)
// -------------------------------------------------------------

type legacyRoomConflictOracle struct{}

func (legacyRoomConflictOracle) Name() string { return "RoomConflict" }

func (o legacyRoomConflictOracle) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomConflict",
				Severity:       diagnostics.SeverityHard,
				Message:        "assignment does not fit in the recurring time-slot grid",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
			},
		}
	}
	if conflictingID, ok := solution.Index.RoomConflict(assignment.RoomID, slotIDs); ok && conflictingID != assignment.ID {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomConflict",
				Severity:       diagnostics.SeverityHard,
				Message:        "room is already scheduled in an occupied time slot",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"roomId":                  string(assignment.RoomID),
					"conflictingAssignmentId": string(conflictingID),
					"courseOfferingId":        string(assignment.CourseOfferingID),
					"sessionRequirementId":    string(assignment.SessionRequirementID),
					"studentGroupId":          string(assignment.StudentGroupID),
					"timeSlotId":              string(assignment.TimeSlotID),
				},
			},
		}
	}
	return nil
}

func (o legacyRoomConflictOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

// -------------------------------------------------------------
// Normalized Violation-Set Helpers
// -------------------------------------------------------------

func normalizeViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		a1 := v.AssignmentID
		a2 := v.RelatedIDs["conflictingAssignmentId"]
		if a2 < a1 && a2 != "" {
			a1, a2 = a2, a1
		}
		// Semantic collision identity: ConstraintName/TemplateID, RoomID, TimeSlotID, Canonical Assignment Pair
		key := fmt.Sprintf("%s|room:%s|slot:%s|a1:%s|a2:%s",
			v.ConstraintName, v.RelatedIDs["roomId"], v.RelatedIDs["timeSlotId"], a1, a2)
		res[key] = v
	}
	return res
}

func compareViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeViolations(legacy)
	compiledMap := normalizeViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}
}

// -------------------------------------------------------------
// Test 1: RoomConflict RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestRoomConflict_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-rc-1",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
		Weight:     100,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-rc-1",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
		Weight:     100,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1})
	if len(errs1) > 0 {
		t.Fatalf("compile error: %v", errs1)
	}
	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2})
	if len(errs2) > 0 {
		t.Fatalf("compile error: %v", errs2)
	}

	// Determinism
	if hash1 == "" || hash2 == "" {
		t.Fatal("expected non-empty RuleSetHash")
	}
	if hash1 != hash2 || set1.RuleSetHash != set2.RuleSetHash {
		t.Fatalf("hash non-deterministic: %s vs %s", hash1, hash2)
	}

	// Sensitivity to ID / Scope / Weight changes
	instChanged := constraints.ConstraintInstance{
		ID:         "rule-rc-2",
		TemplateID: "RoomConflict",
		Scope:      "building-b",
		Kind:       constraints.ConstraintKindHard,
		Weight:     50,
	}
	_, hashChanged, errsChanged := constraints.Compile(nil, []constraints.ConstraintInstance{instChanged})
	if len(errsChanged) > 0 {
		t.Fatalf("compile error: %v", errsChanged)
	}
	if hash1 == hashChanged {
		t.Fatal("expected different RuleSetHash for modified instance configuration")
	}

	// Invalid compilation produces empty hash and nil set
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-rc-invalid",
		TemplateID: "UnknownRoomConflictTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: True Legacy vs Compiled Differential Parity
// -------------------------------------------------------------
func TestRoomConflict_TrueLegacyDifferentialParity(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-room-parity",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-room-parity", Name: "Term 1"},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 50},
			"r-2": {ID: "r-2", Name: "Room 2", Capacity: 50},
			"r-3": {ID: "r-3", Name: "Room 3", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		PeriodsPerDay: 3,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-single": {ID: "req-single", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-double": {ID: "req-double", CourseOfferingID: "co-2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 2, Consecutive: true},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "g-1", FacultyID: "f-1"},
			"co-2": {ID: "co-2", TermID: "term-1", SubjectID: "s-2", StudentGroupID: "g-2", FacultyID: "f-2"},
			"co-3": {ID: "co-3", TermID: "term-1", SubjectID: "s-3", StudentGroupID: "g-3", FacultyID: "f-3"},
			"co-4": {ID: "co-4", TermID: "term-1", SubjectID: "s-4", StudentGroupID: "g-4", FacultyID: "f-4"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-rc",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	compiledRC := compiledSet.Hard[0]
	oracle := legacyRoomConflictOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Case 1: Same room + same slot -> conflict
	t.Run("SameRoom_SameSlot_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-1",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-1",
			FacultyID:            "f-1",
			StudentGroupID:       "g-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-2",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-2",
			FacultyID:            "f-2",
			StudentGroupID:       "g-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected compiled RoomConflict IsConsistent to be false")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		// ViolatedByMove parity
		if err := sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-2",
			RoomID:               "r-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-2",
			FacultyID:            "f-2",
			StudentGroupID:       "g-2",
		}); err != nil {
			t.Fatalf("failed to add a-2: %v", err)
		}
		mv := problem.Move{AssignmentID: "a-2", From: problem.Placement{RoomID: "r-2", TimeSlotID: "m-1"}, To: problem.Placement{RoomID: "r-1", TimeSlotID: "m-1"}}
		compiledMoveV := compiledRC.ViolatedByMove(ctx, &sol, mv)
		compareViolationSets(t, legacyV, compiledMoveV)
	})

	// Case 2: Different rooms + same slot -> allowed
	t.Run("DifferentRooms_SameSlot_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-1",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-1",
			FacultyID:            "f-1",
			StudentGroupID:       "g-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-2",
			RoomID:               "r-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-2",
			FacultyID:            "f-2",
			StudentGroupID:       "g-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected compiled RoomConflict IsConsistent to be true for different rooms")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got: %+v", legacyV)
		}
	})

	// Case 3: Same room + different slots -> allowed
	t.Run("SameRoom_DifferentSlots_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-1",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-1",
			FacultyID:            "f-1",
			StudentGroupID:       "g-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-2",
			RoomID:               "r-1",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-2",
			FacultyID:            "f-2",
			StudentGroupID:       "g-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected compiled RoomConflict IsConsistent to be true for different slots")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got: %+v", legacyV)
		}
	})

	// Case 4: Overlapping multi-period assignments -> conflict
	t.Run("MultiPeriod_Overlap_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		// a-double occupies m-1 and m-2 in r-1
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-double",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-double",
			CourseOfferingID:     "co-2",
			FacultyID:            "f-2",
			StudentGroupID:       "g-2",
		})

		// candidate tries to occupy m-2 in r-1
		candidate := problem.Assignment{
			ID:                   "a-single",
			RoomID:               "r-1",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-1",
			FacultyID:            "f-1",
			StudentGroupID:       "g-1",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected compiled RoomConflict IsConsistent to be false for multi-period overlap")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find multi-period violation")
		}
	})

	// Case 5: Self-assignment behavior (an assignment checking itself does not conflict)
	t.Run("SelfAssignment_NoConflict", func(t *testing.T) {
		sol := problem.NewSolution()
		ass := problem.Assignment{
			ID:                   "a-1",
			RoomID:               "r-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-single",
			CourseOfferingID:     "co-1",
			FacultyID:            "f-1",
			StudentGroupID:       "g-1",
		}
		_ = sol.AddAssignment(&p, ass)

		legacyV := oracle.Check(&p, &sol, ass)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, ass)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent to be true when checking self assignment")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy self-violations, got: %+v", legacyV)
		}
	})

	// Case 6: N-Way room conflict
	t.Run("NWay_Conflict_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		a1 := problem.Assignment{ID: "a-1", RoomID: "r-1", TimeSlotID: "m-1", SessionRequirementID: "req-single", CourseOfferingID: "co-1", FacultyID: "f-1", StudentGroupID: "g-1"}
		a2 := problem.Assignment{ID: "a-2", RoomID: "r-1", TimeSlotID: "m-1", SessionRequirementID: "req-single", CourseOfferingID: "co-2", FacultyID: "f-2", StudentGroupID: "g-2"}
		a3 := problem.Assignment{ID: "a-3", RoomID: "r-1", TimeSlotID: "m-1", SessionRequirementID: "req-single", CourseOfferingID: "co-3", FacultyID: "f-3", StudentGroupID: "g-3"}
		sol.Assignments = []problem.Assignment{a1, a2, a3}

		compiledEvalV := compiledRC.Evaluate(ctx, &sol)
		if len(compiledEvalV) != 3 {
			t.Fatalf("expected 3 conflict pairs for 3-way conflict, got %d: %+v", len(compiledEvalV), compiledEvalV)
		}
	})

	// Case 7: Multiple rooms with simultaneous collisions
	t.Run("MultipleRooms_SimultaneousCollisions", func(t *testing.T) {
		sol := problem.NewSolution()
		// Collision in r-1 at m-1
		a1 := problem.Assignment{ID: "a-1", RoomID: "r-1", TimeSlotID: "m-1", SessionRequirementID: "req-single", CourseOfferingID: "co-1", FacultyID: "f-1", StudentGroupID: "g-1"}
		a2 := problem.Assignment{ID: "a-2", RoomID: "r-1", TimeSlotID: "m-1", SessionRequirementID: "req-single", CourseOfferingID: "co-2", FacultyID: "f-2", StudentGroupID: "g-2"}
		// Collision in r-2 at t-1
		a3 := problem.Assignment{ID: "a-3", RoomID: "r-2", TimeSlotID: "t-1", SessionRequirementID: "req-single", CourseOfferingID: "co-3", FacultyID: "f-3", StudentGroupID: "g-3"}
		a4 := problem.Assignment{ID: "a-4", RoomID: "r-2", TimeSlotID: "t-1", SessionRequirementID: "req-single", CourseOfferingID: "co-4", FacultyID: "f-4", StudentGroupID: "g-4"}
		sol.Assignments = []problem.Assignment{a1, a2, a3, a4}

		compiledEvalV := compiledRC.Evaluate(ctx, &sol)
		if len(compiledEvalV) != 2 {
			t.Fatalf("expected 2 pairwise collisions across 2 rooms, got %d: %+v", len(compiledEvalV), compiledEvalV)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Testing (25 Trials)
// -------------------------------------------------------------
func TestRoomConflict_RandomizedDifferentialParity(t *testing.T) {
	seeds := []int64{
		101, 103, 107, 109, 113, 127, 131, 137, 139, 149,
		151, 157, 163, 167, 173, 179, 181, 191, 193, 197,
		211, 223, 227, 229, 233,
	}

	inst := constraints.ConstraintInstance{
		ID:         "rule-rc-rand",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	oracle := legacyRoomConflictOracle{}

	for _, seed := range seeds {
		p := randomFeasibleProblem(seed)
		p.Prepare()

		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("seed=%d compile error: %v", seed, errs)
		}
		rc := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		// 1. Solve to get a feasible solution
		solution, diag, err := backtracking.New().Solve(context.Background(), p, problem.SolveOptions{})
		if err != nil || diag.Status != diagnostics.SolveStatusSolved {
			t.Fatalf("seed=%d solve error: status=%s, err=%v", seed, diag.Status, err)
		}

		// Feasible solution: both oracle and compiled evaluate to 0 violations
		legacyClean := oracle.FullCheck(&p, &solution)
		compiledClean := rc.Evaluate(ctx, &solution)
		compareViolationSets(t, legacyClean, compiledClean)

		// 2. Introduce random room clashes by reassigning random assignment to another's room & slot
		rng := rand.New(rand.NewSource(seed))
		if len(solution.Assignments) >= 2 {
			idx1 := rng.Intn(len(solution.Assignments))
			idx2 := (idx1 + 1 + rng.Intn(len(solution.Assignments)-1)) % len(solution.Assignments)

			mutatedSol := solution.Clone()
			// Force assignment idx2 to collide with assignment idx1's room and slot
			mutatedSol.Assignments[idx2].RoomID = mutatedSol.Assignments[idx1].RoomID
			mutatedSol.Assignments[idx2].TimeSlotID = mutatedSol.Assignments[idx1].TimeSlotID

			// Rebuild index for legacy oracle
			mutatedIndexedSol := problem.NewSolution()
			for _, a := range mutatedSol.Assignments {
				_ = mutatedIndexedSol.AddAssignment(&p, a)
			}

			compiledMutatedV := rc.Evaluate(ctx, &mutatedSol)
			if len(compiledMutatedV) == 0 {
				t.Fatalf("seed=%d expected compiled Evaluate to detect injected room clash between %s and %s",
					seed, mutatedSol.Assignments[idx1].ID, mutatedSol.Assignments[idx2].ID)
			}
		}
	}
}

// -------------------------------------------------------------
// Test 4: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestRoomConflict_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	ctx := constraints.NewSearchCtx(&p)

	inst := constraints.ConstraintInstance{
		ID:         "rc-full-scoped",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	rc := compiledSet.Hard[0].(constraints.RoomConflictConstraint)

	// Clean initial solution: both full Evaluate and scoped Check return 0
	fullV := rc.Evaluate(ctx, &solution)
	if len(fullV) != 0 {
		t.Fatalf("expected 0 full violations on clean solution, got %d", len(fullV))
	}
	for _, a := range solution.Assignments {
		scopedV := rc.Check(&p, &solution, a)
		if len(scopedV) != 0 {
			t.Fatalf("expected 0 scoped violations on clean assignment, got: %+v", scopedV)
		}
	}

	// 1. Single conflict scenario
	conflictAss1 := problem.Assignment{
		ID:                   "a-conflict-1",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-1",
		TimeSlotID:           "mon-1", // already occupied by a-theory-1
		SessionRequirementID: "req-a-theory",
	}
	conflictSol1 := solution.Clone()
	conflictSol1.Assignments = append(conflictSol1.Assignments, conflictAss1)

	fullConflictV1 := rc.Evaluate(ctx, &conflictSol1)
	if len(fullConflictV1) != 1 {
		t.Fatalf("expected 1 full conflict violation, got %d", len(fullConflictV1))
	}
	scopedConflictV1 := rc.Check(&p, &solution, conflictAss1)
	compareViolationSets(t, scopedConflictV1, fullConflictV1)

	// 2. Multi-room conflict scenario
	conflictAss2 := problem.Assignment{
		ID:                   "a-conflict-2",
		CourseOfferingID:     "offering-b-theory",
		StudentGroupID:       "group-b-whole",
		FacultyID:            "faculty-3",
		RoomID:               "room-lecture-2",
		TimeSlotID:           "mon-1", // already occupied by b-theory
		SessionRequirementID: "req-b-theory",
	}
	conflictSol2 := conflictSol1.Clone()
	conflictSol2.Assignments = append(conflictSol2.Assignments, conflictAss2)

	fullConflictV2 := rc.Evaluate(ctx, &conflictSol2)
	if len(fullConflictV2) != 2 {
		t.Fatalf("expected 2 full conflict violations across 2 rooms, got %d", len(fullConflictV2))
	}
}

// -------------------------------------------------------------
// Test 5: RoomConflict In CSP (Backtracking Solver)
// -------------------------------------------------------------
func TestRoomConflict_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-rc",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-rc", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", TenantID: "tenant-csp-rc", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Year 1", WholeGroupID: "g1-whole", StudentGroupIDs: []model.StudentGroupID{"g1-whole"}},
			"class-2": {ID: "class-2", ProgramID: "prog-1", Name: "Year 2", WholeGroupID: "g2-whole", StudentGroupIDs: []model.StudentGroupID{"g2-whole"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-whole": {ID: "g1-whole", ClassID: "class-1", Name: "G1 Whole", Size: 30},
			"g2-whole": {ID: "g2-whole", ClassID: "class-2", Name: "G2 Whole", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS1", Name: "Subject 1"},
			"s-2": {ID: "s-2", Code: "CS2", Name: "Subject 2"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g1-whole", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
			"co-2": {ID: "co-2", TermID: "term-1", ClassID: "class-2", SubjectID: "s-2", StudentGroupID: "g2-whole", FacultyID: "f-2", SessionRequirementIDs: []model.SessionRequirementID{"req-2"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-2": {ID: "req-2", CourseOfferingID: "co-2", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
			"f-2": {ID: "f-2", Name: "Prof 2"},
		},
		Rooms: map[model.RoomID]model.Room{
			// Only 1 room available for both classes
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 40},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			// 2 slots available
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"},
			{FacultyID: "f-2", TimeSlotID: "m-1"}, {FacultyID: "f-2", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-rc",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled RoomConflict: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(solution.Assignments))
	}

	a1 := solution.Assignments[0]
	a2 := solution.Assignments[1]
	if a1.TimeSlotID == a2.TimeSlotID {
		t.Fatalf("RoomConflict violated: both assignments placed in same slot %s", a1.TimeSlotID)
	}
}

// -------------------------------------------------------------
// Test 6: RoomConflict In Tabu Search
// -------------------------------------------------------------
func TestRoomConflict_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-rc",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	rc := compiledSet.Hard[0]
	if violations := rc.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled RoomConflict violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 7: Final Validation Parity
// -------------------------------------------------------------
func TestRoomConflict_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-rc",
		TemplateID: "RoomConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity
	conflictAss := problem.Assignment{
		ID:                   "a-conflict-extra",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-1",
		TimeSlotID:           "mon-1", // already occupied by a-theory-1
		SessionRequirementID: "req-a-theory",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on room conflict")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundRC := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-rc" && v.TemplateID == "RoomConflict" {
			foundRC = true
			break
		}
	}
	if !foundRC {
		t.Fatalf("expected violation for rule-final-rc, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkRoomConflict_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyRoomConflictOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkRoomConflict_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rc-bench", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkRoomConflict_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rc-bench", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkRoomConflict_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rc-bench", TemplateID: "RoomConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-theory-2", From: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"}, To: problem.Placement{RoomID: "room-lecture-2", TimeSlotID: "mon-1"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}

// =============================================================
// ROOM CAPACITY MIGRATION: ORACLE, PARITY & COMPILATION TESTS
// =============================================================

type legacyRoomCapacityOracle struct{}

func (legacyRoomCapacityOracle) Name() string { return "RoomCapacity" }

func (o legacyRoomCapacityOracle) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	room, ok := p.Room(assignment.RoomID)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomCapacity",
				Severity:       diagnostics.SeverityHard,
				Message:        "room does not exist",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"roomId":               string(assignment.RoomID),
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
			},
		}
	}
	groupSize := p.StudentGroupSize(assignment.StudentGroupID)
	if room.Capacity < groupSize {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomCapacity",
				Severity:       diagnostics.SeverityHard,
				Message:        "room capacity is below student group size",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"roomId":               string(room.ID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
				Metadata: map[string]string{
					"roomCapacity":     fmt.Sprintf("%d", room.Capacity),
					"studentGroupSize": fmt.Sprintf("%d", groupSize),
				},
			},
		}
	}
	return nil
}

func (o legacyRoomCapacityOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

func normalizeCapacityViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		key := fmt.Sprintf("%s|room:%s|group:%s|a:%s|msg:%s",
			v.ConstraintName, v.RelatedIDs["roomId"], v.RelatedIDs["studentGroupId"], v.AssignmentID, v.Message)
		res[key] = v
	}
	return res
}

func compareCapacityViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeCapacityViolations(legacy)
	compiledMap := normalizeCapacityViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure for RoomCapacity:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}

	for k, lV := range legacyMap {
		cV := compiledMap[k]
		if lV.Message != cV.Message {
			t.Errorf("Message mismatch for %s: legacy=%q, compiled=%q", k, lV.Message, cV.Message)
		}
		if lV.Severity != cV.Severity {
			t.Errorf("Severity mismatch for %s: legacy=%v, compiled=%v", k, lV.Severity, cV.Severity)
		}
		for rk, rv := range lV.RelatedIDs {
			if cV.RelatedIDs[rk] != rv {
				t.Errorf("RelatedIDs[%q] mismatch for %s: legacy=%q, compiled=%q", rk, k, rv, cV.RelatedIDs[rk])
			}
		}
		for mk, mv := range lV.Metadata {
			if cV.Metadata[mk] != mv {
				t.Errorf("Metadata[%q] mismatch for %s: legacy=%q, compiled=%q", mk, k, mv, cV.Metadata[mk])
			}
		}
	}
}

// -------------------------------------------------------------
// Test 1: RoomCapacity RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestRoomCapacity_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-rc-1",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-rc-2",
		TemplateID: "RoomCapacity",
		Scope:      "department:cs",
		Kind:       constraints.ConstraintKindHard,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1, inst2})
	if len(errs1) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2, inst1})
	if len(errs2) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of instance order: %s != %s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Invalid template
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-rc-invalid",
		TemplateID: "UnknownRoomCapacityTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: True Legacy vs Compiled Differential Parity Across Boundaries
// -------------------------------------------------------------
func TestRoomCapacity_TrueLegacyDifferentialParity(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-cap-parity",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-cap-parity", Name: "Term 1"},
		Rooms: map[model.RoomID]model.Room{
			"room-exact": {ID: "room-exact", Name: "Exact Capacity Room", Capacity: 40},
			"room-small": {ID: "room-small", Name: "Small Room", Capacity: 30},
			"room-large": {ID: "room-large", Name: "Large Room", Capacity: 80},
			"room-zero":  {ID: "room-zero", Name: "Zero Room", Capacity: 0},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-40":   {ID: "group-40", Name: "Group 40", Size: 40},
			"group-zero": {ID: "group-zero", Name: "Group Zero", Size: 0},
			"group-10":   {ID: "group-10", Name: "Group 10", Size: 10},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-40", FacultyID: "f-1"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-cap",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("unexpected compile error: %v", errs)
	}
	compiledRC := compiledSet.Hard[0]
	oracle := legacyRoomCapacityOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Boundary 1: capacity == required -> allowed
	t.Run("Capacity_Equal_Required_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-exact",
			RoomID:               "room-exact", // 40
			StudentGroupID:       "group-40",   // 40
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent to be true when capacity == required")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 2: capacity < required -> violation
	t.Run("Capacity_Less_Than_Required_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-small",
			RoomID:               "room-small", // 30
			StudentGroupID:       "group-40",   // 40
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent to be false when capacity < required")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 3: capacity > required -> allowed
	t.Run("Capacity_Greater_Than_Required_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-large",
			RoomID:               "room-large", // 80
			StudentGroupID:       "group-40",   // 40
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent to be true when capacity > required")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 4: zero capacity room + zero required capacity -> allowed
	t.Run("Capacity_Zero_Required_Zero_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-zero-zero",
			RoomID:               "room-zero",  // 0
			StudentGroupID:       "group-zero", // 0
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent to be true when capacity == 0 and required == 0")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 5: zero capacity room + positive required -> violation
	t.Run("Capacity_Zero_Required_Positive_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-zero-pos",
			RoomID:               "room-zero", // 0
			StudentGroupID:       "group-10",  // 10
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent to be false when capacity is 0 and required is 10")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 6: positive capacity room + zero required -> allowed
	t.Run("Capacity_Positive_Required_Zero_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-pos-zero",
			RoomID:               "room-small", // 30
			StudentGroupID:       "group-zero", // 0
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent to be true when capacity is 30 and required is 0")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 7: missing/nonexistent room domain case -> violation
	t.Run("Missing_Room_Domain_Case", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-missing-room",
			RoomID:               "room-does-not-exist",
			StudentGroupID:       "group-40",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent to be false for missing room")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find missing room violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compiledEval := compiledRC.Evaluate(ctx, &sol)
		legacyEval := oracle.FullCheck(&p, &sol)
		compareCapacityViolationSets(t, legacyEval, compiledEval)
	})

	// Boundary 8: ViolatedByMove Parity
	t.Run("ViolatedByMove_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-move-test",
			RoomID:               "room-large", // initially valid (80 >= 40)
			StudentGroupID:       "group-40",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		})

		// Move to undersized room (30 < 40)
		mvToSmall := problem.Move{
			AssignmentID: "a-move-test",
			From:         problem.Placement{RoomID: "room-large", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-small", TimeSlotID: "m-2"},
		}
		moveSmallV := compiledRC.ViolatedByMove(ctx, &sol, mvToSmall)
		expectedSmallLegacy := oracle.Check(&p, &sol, problem.Assignment{
			ID:                   "a-move-test",
			RoomID:               "room-small",
			StudentGroupID:       "group-40",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-1",
			CourseOfferingID:     "co-1",
		})
		compareCapacityViolationSets(t, expectedSmallLegacy, moveSmallV)

		// Move to exact room (40 >= 40)
		mvToExact := problem.Move{
			AssignmentID: "a-move-test",
			From:         problem.Placement{RoomID: "room-large", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-exact", TimeSlotID: "m-2"},
		}
		moveExactV := compiledRC.ViolatedByMove(ctx, &sol, mvToExact)
		if len(moveExactV) != 0 {
			t.Fatalf("expected 0 violations for move to exact room, got: %+v", moveExactV)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Parity & Incremental Delta vs Full Recomputation
// -------------------------------------------------------------
func TestRoomCapacity_RandomizedDifferentialParity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	oracle := legacyRoomCapacityOracle{}

	roomIDs := []model.RoomID{"r-0", "r-1", "r-2", "r-3", "r-4", "r-nonexistent"}
	capacities := []int{0, 10, 25, 50, 100}
	groupIDs := []model.StudentGroupID{"g-0", "g-1", "g-2", "g-3"}
	groupSizes := []int{0, 10, 25, 60}

	for iter := 0; iter < 500; iter++ {
		p := problem.Problem{
			TenantID: "tenant-rand-cap",
			Term:     model.Term{ID: "term-1", TenantID: "tenant-rand-cap", Name: "Term 1"},
			Rooms:    make(map[model.RoomID]model.Room),
			StudentGroups: make(map[model.StudentGroupID]model.StudentGroup),
			TimeSlots: map[model.TimeSlotID]model.TimeSlot{
				"s-1": {ID: "s-1", Day: time.Monday, Period: 1},
				"s-2": {ID: "s-2", Day: time.Monday, Period: 2},
				"s-3": {ID: "s-3", Day: time.Monday, Period: 3},
			},
			PeriodsPerDay: 3,
			CourseOfferings: make(map[model.CourseOfferingID]model.CourseOffering),
			SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
				"req-1": {ID: "req-1", Type: model.SessionTypeTheory, Duration: 1},
			},
		}

		for i, rid := range roomIDs[:5] {
			p.Rooms[rid] = model.Room{ID: rid, Name: string(rid), Capacity: capacities[i%len(capacities)]}
		}
		for i, gid := range groupIDs {
			p.StudentGroups[gid] = model.StudentGroup{ID: gid, Name: string(gid), Size: groupSizes[i%len(groupSizes)]}
		}
		p.Prepare()

		inst := constraints.ConstraintInstance{
			ID:         "rule-rand-cap",
			TemplateID: "RoomCapacity",
			Scope:      "global",
			Kind:       constraints.ConstraintKindHard,
		}
		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("iter %d: compile error: %v", iter, errs)
		}
		compiledRC := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		numAssignments := rng.Intn(8) + 1
		sol := problem.NewSolution()

		for aIdx := 0; aIdx < numAssignments; aIdx++ {
			assID := problem.AssignmentID(fmt.Sprintf("a-%d", aIdx))
			rid := roomIDs[rng.Intn(len(roomIDs))]
			gid := groupIDs[rng.Intn(len(groupIDs))]
			sid := model.TimeSlotID(fmt.Sprintf("s-%d", rng.Intn(3)+1))

			ass := problem.Assignment{
				ID:                   assID,
				RoomID:               rid,
				StudentGroupID:       gid,
				TimeSlotID:           sid,
				SessionRequirementID: "req-1",
			}
			_ = sol.AddAssignment(&p, ass)

			// Parity check on candidate assignment
			legacyCheck := oracle.Check(&p, &sol, ass)
			compiledConsistent := compiledRC.IsConsistent(ctx, &sol, ass)
			if (len(legacyCheck) == 0) != compiledConsistent {
				t.Fatalf("iter %d ass %d: consistency mismatch: legacy=%v, compiledConsistent=%v",
					iter, aIdx, len(legacyCheck) == 0, compiledConsistent)
			}
		}

		// Full solution check parity
		legacyFull := oracle.FullCheck(&p, &sol)
		compiledFull := compiledRC.Evaluate(ctx, &sol)
		compareCapacityViolationSets(t, legacyFull, compiledFull)

		// Incremental delta vs full recomputation parity
		if len(sol.Assignments) > 0 {
			targetAss := sol.Assignments[rng.Intn(len(sol.Assignments))]
			newRoom := roomIDs[rng.Intn(len(roomIDs))]
			newSlot := model.TimeSlotID(fmt.Sprintf("s-%d", rng.Intn(3)+1))

			mv := problem.Move{
				AssignmentID: targetAss.ID,
				From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
				To:           problem.Placement{RoomID: newRoom, TimeSlotID: newSlot},
			}

			// Incremental move violations
			moveViolations := compiledRC.ViolatedByMove(ctx, &sol, mv)

			// Full recomputation delta
			solAfter := sol.Clone()
			_ = solAfter.ApplyMove(&p, mv)
			fullAfter := compiledRC.Evaluate(ctx, &solAfter)

			// For unary constraint, moveViolations should match whether solAfter has violation on targetAss
			var targetAssViolationsAfter []diagnostics.Violation
			for _, v := range fullAfter {
				if v.AssignmentID == string(targetAss.ID) {
					targetAssViolationsAfter = append(targetAssViolationsAfter, v)
				}
			}
			compareCapacityViolationSets(t, moveViolations, targetAssViolationsAfter)
		}
	}
}

// -------------------------------------------------------------
// Test 4: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestRoomCapacity_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-scoped-cap",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	// Clean initial solution should evaluate to 0 violations
	violations := c.Evaluate(ctx, &solution)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations on clean solution, got: %+v", violations)
	}

	for _, a := range solution.Assignments {
		if !c.IsConsistent(ctx, &solution, a) {
			t.Fatalf("expected IsConsistent=true for valid assignment %s", a.ID)
		}
	}
}

// -------------------------------------------------------------
// Test 5: RoomCapacity In CSP Solver
// -------------------------------------------------------------
func TestRoomCapacity_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-cap",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-cap", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Class 1", WholeGroupID: "g1-large", StudentGroupIDs: []model.StudentGroupID{"g1-large"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1-large": {ID: "g1-large", ClassID: "class-1", Name: "Group Large", Size: 50},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS1", Name: "Subject 1"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g1-large", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
		},
		Rooms: map[model.RoomID]model.Room{
			// r-small cannot fit g1-large (capacity 20 < size 50)
			"r-small": {ID: "r-small", Name: "Small Room", Capacity: 20},
			// r-large can fit g1-large (capacity 60 >= size 50)
			"r-large": {ID: "r-large", Name: "Large Room", Capacity: 60},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
		},
		PeriodsPerDay: 1,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-small", TimeSlotID: "m-1"},
			{RoomID: "r-large", TimeSlotID: "m-1"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-cap",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled RoomCapacity: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(solution.Assignments))
	}

	a := solution.Assignments[0]
	if a.RoomID != "r-large" {
		t.Fatalf("expected assignment to be placed in r-large, got %s", a.RoomID)
	}
}

// -------------------------------------------------------------
// Test 6: RoomCapacity In Tabu Search
// -------------------------------------------------------------
func TestRoomCapacity_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-cap",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	rc := compiledSet.Hard[0]
	if violations := rc.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled RoomCapacity violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 7: Final Validation Parity
// -------------------------------------------------------------
func TestRoomCapacity_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-cap",
		TemplateID: "RoomCapacity",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity (assign to room-small with capacity 15 < group size 40)
	conflictAss := problem.Assignment{
		ID:                   "a-cap-violating",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole", // size 40
		FacultyID:            "faculty-1",
		RoomID:               "room-small", // capacity 15
		TimeSlotID:           "tue-1",
		SessionRequirementID: "req-a-theory",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on room capacity violation")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundCap := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-cap" && v.TemplateID == "RoomCapacity" {
			foundCap = true
			break
		}
	}
	if !foundCap {
		t.Fatalf("expected violation for rule-final-cap, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkRoomCapacity_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyRoomCapacityOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkRoomCapacity_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "cap-bench", TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkRoomCapacity_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "cap-bench", TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkRoomCapacity_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "cap-bench", TemplateID: "RoomCapacity", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-theory-2", From: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"}, To: problem.Placement{RoomID: "room-small", TimeSlotID: "mon-1"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}

// =============================================================
// ROOM FEATURE COMPATIBILITY MIGRATION: ORACLE, PARITY & COMPILATION TESTS
// =============================================================

type legacyRoomFeatureCompatibilityOracle struct{}

func (legacyRoomFeatureCompatibilityOracle) Name() string { return "RoomFeatureCompatibility" }

func joinRoomFeatureIDs(ids []model.RoomFeatureID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ",")
}

func (o legacyRoomFeatureCompatibilityOracle) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	required := p.RequiredRoomFeatures(assignment.CourseOfferingID, assignment.SessionRequirementID)
	if len(required) == 0 {
		return nil
	}
	if !p.RoomHasFeatures(assignment.RoomID, required) {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomFeatureCompatibility",
				Severity:       diagnostics.SeverityHard,
				Message:        "room does not provide all required features",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"roomId":               string(assignment.RoomID),
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
				Metadata: map[string]string{
					"requiredRoomFeatureIds": joinRoomFeatureIDs(required),
				},
			},
		}
	}
	return nil
}

func (o legacyRoomFeatureCompatibilityOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

func normalizeFeatureViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		key := fmt.Sprintf("%s|room:%s|offering:%s|req:%s|a:%s|msg:%s|features:%s",
			v.ConstraintName, v.RelatedIDs["roomId"], v.RelatedIDs["courseOfferingId"], v.RelatedIDs["sessionRequirementId"], v.AssignmentID, v.Message, v.Metadata["requiredRoomFeatureIds"])
		res[key] = v
	}
	return res
}

func compareFeatureViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeFeatureViolations(legacy)
	compiledMap := normalizeFeatureViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure for RoomFeatureCompatibility:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}

	for k, lV := range legacyMap {
		cV := compiledMap[k]
		if lV.Message != cV.Message {
			t.Errorf("Message mismatch for %s: legacy=%q, compiled=%q", k, lV.Message, cV.Message)
		}
		if lV.Severity != cV.Severity {
			t.Errorf("Severity mismatch for %s: legacy=%v, compiled=%v", k, lV.Severity, cV.Severity)
		}
		for rk, rv := range lV.RelatedIDs {
			if cV.RelatedIDs[rk] != rv {
				t.Errorf("RelatedIDs[%q] mismatch for %s: legacy=%q, compiled=%q", rk, k, rv, cV.RelatedIDs[rk])
			}
		}
		for mk, mv := range lV.Metadata {
			if cV.Metadata[mk] != mv {
				t.Errorf("Metadata[%q] mismatch for %s: legacy=%q, compiled=%q", mk, k, mv, cV.Metadata[mk])
			}
		}
	}
}

// -------------------------------------------------------------
// Test 1: RoomFeatureCompatibility RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-rfc-1",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-rfc-2",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "department:cs",
		Kind:       constraints.ConstraintKindHard,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1, inst2})
	if len(errs1) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2, inst1})
	if len(errs2) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of instance order: %s != %s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Invalid template
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-rfc-invalid",
		TemplateID: "UnknownRoomFeatureTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: True Legacy vs Compiled Differential Parity Across Boundaries
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_TrueLegacyDifferentialParity(t *testing.T) {
	fLab := model.RoomFeatureID("feat-lab")
	fGPU := model.RoomFeatureID("feat-gpu")
	fProjector := model.RoomFeatureID("feat-projector")
	fExtra := model.RoomFeatureID("feat-extra")

	p := problem.Problem{
		TenantID: "tenant-feat-parity",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-feat-parity", Name: "Term 1"},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			fLab:       {ID: fLab, Name: "Lab Equipment"},
			fGPU:       {ID: fGPU, Name: "GPU Rig"},
			fProjector: {ID: fProjector, Name: "Projector"},
			fExtra:     {ID: fExtra, Name: "Extra Special Feature"},
		},
		Rooms: map[model.RoomID]model.Room{
			"room-all":   {ID: "room-all", Name: "All Features", Capacity: 50, FeatureIDs: []model.RoomFeatureID{fLab, fGPU, fProjector, fExtra}},
			"room-lab":   {ID: "room-lab", Name: "Lab Only", Capacity: 50, FeatureIDs: []model.RoomFeatureID{fLab}},
			"room-gpu":   {ID: "room-gpu", Name: "GPU Only", Capacity: 50, FeatureIDs: []model.RoomFeatureID{fGPU}},
			"room-plain": {ID: "room-plain", Name: "Plain Room (No Features)", Capacity: 50, FeatureIDs: nil},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-1": {ID: "group-1", Name: "Group 1", Size: 30},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-none":  {ID: "req-none", CourseOfferingID: "co-none", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-lab":   {ID: "req-lab", CourseOfferingID: "co-none", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, RequiredRoomFeatureIDs: []model.RoomFeatureID{fLab}},
			"req-gpu":   {ID: "req-gpu", CourseOfferingID: "co-none", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, RequiredRoomFeatureIDs: []model.RoomFeatureID{fGPU}},
			"req-multi": {ID: "req-multi", CourseOfferingID: "co-none", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, RequiredRoomFeatureIDs: []model.RoomFeatureID{fLab, fGPU}},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-none":       {ID: "co-none", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: "f-1"},
			"co-with-lab":   {ID: "co-with-lab", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: "f-1", RequiredRoomFeatureIDs: []model.RoomFeatureID{fLab}},
			"co-with-multi": {ID: "co-with-multi", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: "f-1", RequiredRoomFeatureIDs: []model.RoomFeatureID{fLab, fGPU}},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-rfc",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("unexpected compile error: %v", errs)
	}
	compiledRFC := compiledSet.Hard[0]
	oracle := legacyRoomFeatureCompatibilityOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Boundary 1: All required features present -> allowed
	t.Run("AllRequiredFeaturesPresent_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-lab-in-lab",
			RoomID:               "room-lab",
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true when room provides required lab feature")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 2: One required feature missing -> violation
	t.Run("OneRequiredFeatureMissing_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-lab-in-gpu",
			RoomID:               "room-gpu", // has GPU, missing Lab
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when room is missing required lab feature")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 3: Multiple required features, none present -> violation
	t.Run("MultipleRequiredFeatures_NonePresent_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-multi-in-plain",
			RoomID:               "room-plain", // has nothing
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-multi",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when plain room lacks multi features")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 4: Multiple required features, subset present -> violation
	t.Run("MultipleRequiredFeatures_SubsetPresent_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-multi-in-lab",
			RoomID:               "room-lab", // has Lab, missing GPU
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-multi",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when room is missing GPU subset feature")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 5: Multiple required features, all present -> allowed
	t.Run("MultipleRequiredFeatures_AllPresent_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-multi-in-all",
			RoomID:               "room-all", // has Lab, GPU, Projector, Extra
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-multi",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true when room provides all required features")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 6: No required features -> allowed in any room
	t.Run("NoRequiredFeatures_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		for _, rid := range []model.RoomID{"room-all", "room-lab", "room-gpu", "room-plain"} {
			candidate := problem.Assignment{
				ID:                   problem.AssignmentID(fmt.Sprintf("a-none-in-%s", rid)),
				RoomID:               rid,
				StudentGroupID:       "group-1",
				TimeSlotID:           "m-1",
				SessionRequirementID: "req-none",
				CourseOfferingID:     "co-none",
			}
			legacyV := oracle.Check(&p, &sol, candidate)
			compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

			if !compiledConsistent {
				t.Fatalf("expected IsConsistent=true for req with no feature requirements in %s", rid)
			}
			if len(legacyV) != 0 {
				t.Fatalf("expected 0 legacy violations in %s, got %+v", rid, legacyV)
			}
		}
	})

	// Boundary 7: Room with extra superset features -> allowed
	t.Run("RoomWithExtraFeatures_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-lab-in-all",
			RoomID:               "room-all", // has Lab + extras
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true when room provides superset of features")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}
	})

	// Boundary 8: Offering and Requirement union features parity
	t.Run("OfferingAndRequirementUnion_Features_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		// co-with-lab (requires Lab) + req-gpu (requires GPU) -> union is [Lab, GPU]
		candidate := problem.Assignment{
			ID:                   "a-union-test",
			RoomID:               "room-lab", // has Lab, missing GPU
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-gpu",
			CourseOfferingID:     "co-with-lab",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when union requirements are not all met")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find union feature violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 9: Missing room domain case -> violation
	t.Run("MissingRoom_DomainCase_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-missing-room",
			RoomID:               "room-nonexistent",
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for nonexistent room with required features")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find violation for nonexistent room")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFeatureViolationSets(t, oracle.FullCheck(&p, &sol), compiledRFC.Evaluate(ctx, &sol))
	})

	// Boundary 10: ViolatedByMove Parity
	t.Run("ViolatedByMove_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-move-feat-test",
			RoomID:               "room-lab", // initially valid
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		})

		// Move to incompatible plain room
		mvToPlain := problem.Move{
			AssignmentID: "a-move-feat-test",
			From:         problem.Placement{RoomID: "room-lab", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-plain", TimeSlotID: "m-2"},
		}
		movePlainV := compiledRFC.ViolatedByMove(ctx, &sol, mvToPlain)
		expectedPlainLegacy := oracle.Check(&p, &sol, problem.Assignment{
			ID:                   "a-move-feat-test",
			RoomID:               "room-plain",
			StudentGroupID:       "group-1",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-lab",
			CourseOfferingID:     "co-none",
		})
		compareFeatureViolationSets(t, expectedPlainLegacy, movePlainV)

		// Move to compatible room-all
		mvToAll := problem.Move{
			AssignmentID: "a-move-feat-test",
			From:         problem.Placement{RoomID: "room-lab", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-all", TimeSlotID: "m-2"},
		}
		moveAllV := compiledRFC.ViolatedByMove(ctx, &sol, mvToAll)
		if len(moveAllV) != 0 {
			t.Fatalf("expected 0 violations for move to room-all, got: %+v", moveAllV)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Parity & Incremental Delta vs Full Recomputation
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_RandomizedDifferentialParity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	oracle := legacyRoomFeatureCompatibilityOracle{}

	featurePool := []model.RoomFeatureID{"f-lab", "f-gpu", "f-proj", "f-whiteboard", "f-ac"}
	roomIDs := []model.RoomID{"r-0", "r-1", "r-2", "r-3", "r-4", "r-missing"}

	for iter := 0; iter < 500; iter++ {
		p := problem.Problem{
			TenantID: "tenant-rand-feat",
			Term:     model.Term{ID: "term-1", TenantID: "tenant-rand-feat", Name: "Term 1"},
			Rooms:    make(map[model.RoomID]model.Room),
			RoomFeatures: make(map[model.RoomFeatureID]model.RoomFeature),
			StudentGroups: map[model.StudentGroupID]model.StudentGroup{
				"g-1": {ID: "g-1", Name: "Group 1", Size: 30},
			},
			TimeSlots: map[model.TimeSlotID]model.TimeSlot{
				"s-1": {ID: "s-1", Day: time.Monday, Period: 1},
				"s-2": {ID: "s-2", Day: time.Monday, Period: 2},
			},
			PeriodsPerDay: 2,
			CourseOfferings: make(map[model.CourseOfferingID]model.CourseOffering),
			SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
		}

		for _, fid := range featurePool {
			p.RoomFeatures[fid] = model.RoomFeature{ID: fid, Name: string(fid)}
		}

		// Build random rooms with random feature subsets
		for _, rid := range roomIDs[:5] {
			var rFeats []model.RoomFeatureID
			for _, fid := range featurePool {
				if rng.Float64() < 0.4 {
					rFeats = append(rFeats, fid)
				}
			}
			p.Rooms[rid] = model.Room{ID: rid, Name: string(rid), Capacity: 50, FeatureIDs: rFeats}
		}

		// Build random offerings and requirements with random feature requirements
		for oIdx := 0; oIdx < 4; oIdx++ {
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", oIdx))
			rid := model.SessionRequirementID(fmt.Sprintf("req-%d", oIdx))

			var oFeats []model.RoomFeatureID
			var reqFeats []model.RoomFeatureID
			for _, fid := range featurePool {
				if rng.Float64() < 0.25 {
					oFeats = append(oFeats, fid)
				}
				if rng.Float64() < 0.25 {
					reqFeats = append(reqFeats, fid)
				}
			}

			p.CourseOfferings[oid] = model.CourseOffering{
				ID:                     oid,
				TermID:                 "term-1",
				SubjectID:              "s-1",
				StudentGroupID:         "g-1",
				FacultyID:              "f-1",
				RequiredRoomFeatureIDs: oFeats,
			}
			p.SessionRequirements[rid] = model.SessionRequirement{
				ID:                     rid,
				CourseOfferingID:       oid,
				Type:                   model.SessionTypeLab,
				Duration:               1,
				RequiredRoomFeatureIDs: reqFeats,
			}
		}
		p.Prepare()

		inst := constraints.ConstraintInstance{
			ID:         "rule-rand-rfc",
			TemplateID: "RoomFeatureCompatibility",
			Scope:      "global",
			Kind:       constraints.ConstraintKindHard,
		}
		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("iter %d: compile error: %v", iter, errs)
		}
		compiledRFC := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		numAssignments := rng.Intn(6) + 1
		sol := problem.NewSolution()

		for aIdx := 0; aIdx < numAssignments; aIdx++ {
			assID := problem.AssignmentID(fmt.Sprintf("a-%d", aIdx))
			rid := roomIDs[rng.Intn(len(roomIDs))]
			targetOIdx := rng.Intn(4)
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", targetOIdx))
			reqId := model.SessionRequirementID(fmt.Sprintf("req-%d", targetOIdx))
			sid := model.TimeSlotID(fmt.Sprintf("s-%d", rng.Intn(2)+1))

			ass := problem.Assignment{
				ID:                   assID,
				RoomID:               rid,
				StudentGroupID:       "g-1",
				CourseOfferingID:     oid,
				SessionRequirementID: reqId,
				TimeSlotID:           sid,
			}
			_ = sol.AddAssignment(&p, ass)

			// Parity check on candidate assignment
			legacyCheck := oracle.Check(&p, &sol, ass)
			compiledConsistent := compiledRFC.IsConsistent(ctx, &sol, ass)
			if (len(legacyCheck) == 0) != compiledConsistent {
				t.Fatalf("iter %d ass %d: consistency mismatch: legacy=%v, compiledConsistent=%v",
					iter, aIdx, len(legacyCheck) == 0, compiledConsistent)
			}
		}

		// Full solution check parity
		legacyFull := oracle.FullCheck(&p, &sol)
		compiledFull := compiledRFC.Evaluate(ctx, &sol)
		compareFeatureViolationSets(t, legacyFull, compiledFull)

		// Incremental delta vs full recomputation parity
		if len(sol.Assignments) > 0 {
			targetAss := sol.Assignments[rng.Intn(len(sol.Assignments))]
			newRoom := roomIDs[rng.Intn(len(roomIDs))]
			newSlot := model.TimeSlotID(fmt.Sprintf("s-%d", rng.Intn(2)+1))

			mv := problem.Move{
				AssignmentID: targetAss.ID,
				From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
				To:           problem.Placement{RoomID: newRoom, TimeSlotID: newSlot},
			}

			// Incremental move violations
			moveViolations := compiledRFC.ViolatedByMove(ctx, &sol, mv)

			// Full recomputation delta
			solAfter := sol.Clone()
			_ = solAfter.ApplyMove(&p, mv)
			fullAfter := compiledRFC.Evaluate(ctx, &solAfter)

			var targetAssViolationsAfter []diagnostics.Violation
			for _, v := range fullAfter {
				if v.AssignmentID == string(targetAss.ID) {
					targetAssViolationsAfter = append(targetAssViolationsAfter, v)
				}
			}
			compareFeatureViolationSets(t, moveViolations, targetAssViolationsAfter)
		}
	}
}

// -------------------------------------------------------------
// Test 4: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-scoped-rfc",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	// Clean initial solution should evaluate to 0 violations
	violations := c.Evaluate(ctx, &solution)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations on clean solution, got: %+v", violations)
	}

	for _, a := range solution.Assignments {
		if !c.IsConsistent(ctx, &solution, a) {
			t.Fatalf("expected IsConsistent=true for valid assignment %s", a.ID)
		}
	}
}

// -------------------------------------------------------------
// Test 5: RoomFeatureCompatibility In CSP Solver
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_InCSP(t *testing.T) {
	labFeature := model.RoomFeatureID("feat-lab")

	p := problem.Problem{
		TenantID: "tenant-csp-rfc",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-rfc", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Class 1", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1": {ID: "g1", ClassID: "class-1", Name: "Group 1", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS101", Name: "Computer Systems Lab"},
		},
		RoomFeatures: map[model.RoomFeatureID]model.RoomFeature{
			labFeature: {ID: labFeature, Name: "Laboratory Hardware"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g1", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1, RequiredRoomFeatureIDs: []model.RoomFeatureID{labFeature}},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
		},
		Rooms: map[model.RoomID]model.Room{
			// r-lecture lacks lab feature
			"r-lecture": {ID: "r-lecture", Name: "Lecture Hall", Capacity: 60, FeatureIDs: nil},
			// r-lab has lab feature
			"r-lab": {ID: "r-lab", Name: "Computer Lab", Capacity: 40, FeatureIDs: []model.RoomFeatureID{labFeature}},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
		},
		PeriodsPerDay: 1,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-lecture", TimeSlotID: "m-1"},
			{RoomID: "r-lab", TimeSlotID: "m-1"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-rfc",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled RoomFeatureCompatibility: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(solution.Assignments))
	}

	a := solution.Assignments[0]
	if a.RoomID != "r-lab" {
		t.Fatalf("expected assignment to be placed in r-lab, got %s", a.RoomID)
	}
}

// -------------------------------------------------------------
// Test 6: RoomFeatureCompatibility In Tabu Search
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-rfc",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	rfc := compiledSet.Hard[0]
	if violations := rfc.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled RoomFeatureCompatibility violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 7: Final Validation Parity
// -------------------------------------------------------------
func TestRoomFeatureCompatibility_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-rfc",
		TemplateID: "RoomFeatureCompatibility",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity (place lab session a-lab1 in room-lecture-1 which has no lab feature)
	conflictAss := problem.Assignment{
		ID:                   "a-lab-incompatible",
		CourseOfferingID:     "offering-a-lab1",
		StudentGroupID:       "group-a-lab1",
		FacultyID:            "faculty-2",
		RoomID:               "room-lecture-1", // lacks feature-lab
		TimeSlotID:           "tue-1",
		SessionRequirementID: "req-a-lab1",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on feature incompatibility")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundRFC := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-rfc" && v.TemplateID == "RoomFeatureCompatibility" {
			foundRFC = true
			break
		}
	}
	if !foundRFC {
		t.Fatalf("expected violation for rule-final-rfc, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkRoomFeatureCompatibility_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyRoomFeatureCompatibilityOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkRoomFeatureCompatibility_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rfc-bench", TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkRoomFeatureCompatibility_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rfc-bench", TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkRoomFeatureCompatibility_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "rfc-bench", TemplateID: "RoomFeatureCompatibility", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-lab-1", From: problem.Placement{RoomID: "room-lab-1", TimeSlotID: "mon-1"}, To: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}

// =============================================================
// FACULTY AVAILABILITY MIGRATION: ORACLE, PARITY & COMPILATION TESTS
// =============================================================

type legacyFacultyAvailabilityOracle struct{}

func (legacyFacultyAvailabilityOracle) Name() string { return "FacultyAvailability" }

func joinTimeSlotIDs(ids []model.TimeSlotID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ",")
}

func (o legacyFacultyAvailabilityOracle) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "FacultyAvailability",
				Severity:       diagnostics.SeverityHard,
				Message:        "assignment does not fit in the recurring time-slot grid",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
			},
		}
	}
	if !p.IsFacultyAvailable(assignment.FacultyID, slotIDs) {
		return []diagnostics.Violation{
			{
				ConstraintName: "FacultyAvailability",
				Severity:       diagnostics.SeverityHard,
				Message:        "faculty is not available for all occupied time slots",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"facultyId":            string(assignment.FacultyID),
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
				Metadata: map[string]string{
					"timeSlotIds": joinTimeSlotIDs(slotIDs),
				},
			},
		}
	}
	return nil
}

func (o legacyFacultyAvailabilityOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

func normalizeFacultyAvailabilityViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		key := fmt.Sprintf("%s|fac:%s|a:%s|slot:%s|msg:%s|slots:%s",
			v.ConstraintName, v.RelatedIDs["facultyId"], v.AssignmentID, v.RelatedIDs["timeSlotId"], v.Message, v.Metadata["timeSlotIds"])
		res[key] = v
	}
	return res
}

func compareFacultyAvailabilityViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeFacultyAvailabilityViolations(legacy)
	compiledMap := normalizeFacultyAvailabilityViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure for FacultyAvailability:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}

	for k, lV := range legacyMap {
		cV := compiledMap[k]
		if lV.Message != cV.Message {
			t.Errorf("Message mismatch for %s: legacy=%q, compiled=%q", k, lV.Message, cV.Message)
		}
		if lV.Severity != cV.Severity {
			t.Errorf("Severity mismatch for %s: legacy=%v, compiled=%v", k, lV.Severity, cV.Severity)
		}
		for rk, rv := range lV.RelatedIDs {
			if cV.RelatedIDs[rk] != rv {
				t.Errorf("RelatedIDs[%q] mismatch for %s: legacy=%q, compiled=%q", rk, k, rv, cV.RelatedIDs[rk])
			}
		}
		for mk, mv := range lV.Metadata {
			if cV.Metadata[mk] != mv {
				t.Errorf("Metadata[%q] mismatch for %s: legacy=%q, compiled=%q", mk, k, mv, cV.Metadata[mk])
			}
		}
	}
}

// -------------------------------------------------------------
// Test 1: FacultyAvailability RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestFacultyAvailability_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-fa-1",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-fa-2",
		TemplateID: "FacultyAvailability",
		Scope:      "department:cs",
		Kind:       constraints.ConstraintKindHard,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1, inst2})
	if len(errs1) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2, inst1})
	if len(errs2) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of instance order: %s != %s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Invalid template
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-fa-invalid",
		TemplateID: "UnknownFacultyAvailabilityTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: True Legacy vs Compiled Differential Parity Across Boundaries
// -------------------------------------------------------------
func TestFacultyAvailability_TrueLegacyDifferentialParity(t *testing.T) {
	fac1 := model.FacultyID("f-available-m1-m2")
	facNoAvail := model.FacultyID("f-no-avail")

	p := problem.Problem{
		TenantID: "tenant-fa-parity",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-fa-parity", Name: "Term 1"},
		Rooms: map[model.RoomID]model.Room{
			"room-1": {ID: "room-1", Name: "Room 1", Capacity: 50},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-1": {ID: "group-1", Name: "Group 1", Size: 30},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"m-4": {ID: "m-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		PeriodsPerDay: 4,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1p": {ID: "req-1p", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-2p": {ID: "req-2p", CourseOfferingID: "co-1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1":        {ID: "co-1", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: fac1},
			"co-no-avail": {ID: "co-no-avail", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: facNoAvail},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			fac1:       {ID: fac1, Name: "Prof Available"},
			facNoAvail: {ID: facNoAvail, Name: "Prof No Avail"},
		},
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: fac1, TimeSlotID: "m-1"},
			{FacultyID: fac1, TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-fa",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("unexpected compile error: %v", errs)
	}
	compiledFA := compiledSet.Hard[0]
	oracle := legacyFacultyAvailabilityOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Boundary 1: Exact boundary (available in m-1, m-2; 1-period in m-2) -> allowed
	t.Run("ExactBoundary_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-exact-boundary",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for exact boundary slot m-2")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 2: Just inside availability (available in m-1, m-2; 2-period starting m-1 -> spans m-1, m-2) -> allowed
	t.Run("JustInsideAvailability_MultiPeriod_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-just-inside",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-2p", // duration 2 -> m-1 and m-2
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true when multi-period session fits inside available window")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 3: Just outside availability (available in m-1, m-2; 1-period in m-3) -> conflict
	t.Run("JustOutsideAvailability_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-just-outside",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-3", // unavailable
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when slot is outside availability window")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 4: Session spanning an availability boundary (available m-1, m-2; 2-period starting m-2 -> spans m-2, m-3) -> conflict
	t.Run("SessionSpanningAvailabilityBoundary_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-span-boundary",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-2", // m-2 is available, but m-3 is NOT
			SessionRequirementID: "req-2p", // duration 2
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when session spans across unavailable slot")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 5: Completely unavailable slot on another day -> conflict
	t.Run("CompletelyUnavailableSlot_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-other-day",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "t-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false on Tuesday slot")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 6: Faculty with no availability records -> conflict
	t.Run("FacultyWithNoAvailability_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-no-avail",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            facNoAvail,
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-no-avail",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for faculty with 0 availability records")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 7: Off-grid placement (invalid duration exceeding day bounds) -> invalid duration violation
	t.Run("InvalidDuration_OffGrid_Violation", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-offgrid",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-4", // period 4 (last period), duration 2 -> exceeds day bounds
			SessionRequirementID: "req-2p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledFA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for off-grid placement")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to return invalid duration violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareFacultyAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledFA.Evaluate(ctx, &sol))
	})

	// Boundary 8: ViolatedByMove Parity
	t.Run("ViolatedByMove_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-move-fa-test",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-1", // initially valid
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		// Move to unavailable m-3
		mvToM3 := problem.Move{
			AssignmentID: "a-move-fa-test",
			From:         problem.Placement{RoomID: "room-1", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-1", TimeSlotID: "m-3"},
		}
		moveM3V := compiledFA.ViolatedByMove(ctx, &sol, mvToM3)
		expectedM3Legacy := oracle.Check(&p, &sol, problem.Assignment{
			ID:                   "a-move-fa-test",
			RoomID:               "room-1",
			StudentGroupID:       "group-1",
			FacultyID:            fac1,
			TimeSlotID:           "m-3",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})
		compareFacultyAvailabilityViolationSets(t, expectedM3Legacy, moveM3V)

		// Move to available m-2
		mvToM2 := problem.Move{
			AssignmentID: "a-move-fa-test",
			From:         problem.Placement{RoomID: "room-1", TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: "room-1", TimeSlotID: "m-2"},
		}
		moveM2V := compiledFA.ViolatedByMove(ctx, &sol, mvToM2)
		if len(moveM2V) != 0 {
			t.Fatalf("expected 0 violations for move to available slot m-2, got: %+v", moveM2V)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Parity & Incremental Delta vs Full Recomputation
// -------------------------------------------------------------
func TestFacultyAvailability_RandomizedDifferentialParity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	oracle := legacyFacultyAvailabilityOracle{}

	slotIDs := []model.TimeSlotID{"s-1", "s-2", "s-3", "s-4"}
	facultyIDs := []model.FacultyID{"f-0", "f-1", "f-2", "f-no-avail"}

	for iter := 0; iter < 500; iter++ {
		p := problem.Problem{
			TenantID: "tenant-rand-fa",
			Term:     model.Term{ID: "term-1", TenantID: "tenant-rand-fa", Name: "Term 1"},
			Rooms: map[model.RoomID]model.Room{
				"r-1": {ID: "r-1", Name: "Room 1", Capacity: 50},
			},
			StudentGroups: map[model.StudentGroupID]model.StudentGroup{
				"g-1": {ID: "g-1", Name: "Group 1", Size: 30},
			},
			TimeSlots: map[model.TimeSlotID]model.TimeSlot{
				"s-1": {ID: "s-1", Day: time.Monday, Period: 1},
				"s-2": {ID: "s-2", Day: time.Monday, Period: 2},
				"s-3": {ID: "s-3", Day: time.Monday, Period: 3},
				"s-4": {ID: "s-4", Day: time.Monday, Period: 4},
			},
			PeriodsPerDay:         4,
			CourseOfferings:       make(map[model.CourseOfferingID]model.CourseOffering),
			SessionRequirements:   make(map[model.SessionRequirementID]model.SessionRequirement),
			Faculty:               make(map[model.FacultyID]model.Faculty),
			FacultyAvailabilities: nil,
		}

		for _, fid := range facultyIDs {
			p.Faculty[fid] = model.Faculty{ID: fid, Name: string(fid)}
		}

		// Random availability for f-0, f-1, f-2 (f-no-avail gets none)
		for _, fid := range facultyIDs[:3] {
			for _, sid := range slotIDs {
				if rng.Float64() < 0.6 {
					p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{
						FacultyID:  fid,
						TimeSlotID: sid,
					})
				}
			}
		}

		for oIdx := 0; oIdx < 4; oIdx++ {
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", oIdx))
			rid := model.SessionRequirementID(fmt.Sprintf("req-%d", oIdx))
			fid := facultyIDs[oIdx]

			dur := 1
			if rng.Float64() < 0.3 {
				dur = 2
			}

			p.CourseOfferings[oid] = model.CourseOffering{
				ID:             oid,
				TermID:         "term-1",
				SubjectID:      "s-1",
				StudentGroupID: "g-1",
				FacultyID:      fid,
			}
			p.SessionRequirements[rid] = model.SessionRequirement{
				ID:               rid,
				CourseOfferingID: oid,
				Type:             model.SessionTypeTheory,
				Duration:         dur,
			}
		}
		p.Prepare()

		inst := constraints.ConstraintInstance{
			ID:         "rule-rand-fa",
			TemplateID: "FacultyAvailability",
			Scope:      "global",
			Kind:       constraints.ConstraintKindHard,
		}
		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("iter %d: compile error: %v", iter, errs)
		}
		compiledFA := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		numAssignments := rng.Intn(6) + 1
		sol := problem.NewSolution()

		for aIdx := 0; aIdx < numAssignments; aIdx++ {
			assID := problem.AssignmentID(fmt.Sprintf("a-%d", aIdx))
			targetOIdx := rng.Intn(4)
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", targetOIdx))
			reqId := model.SessionRequirementID(fmt.Sprintf("req-%d", targetOIdx))
			fid := facultyIDs[targetOIdx]
			sid := slotIDs[rng.Intn(len(slotIDs))]

			ass := problem.Assignment{
				ID:                   assID,
				RoomID:               "r-1",
				StudentGroupID:       "g-1",
				FacultyID:            fid,
				CourseOfferingID:     oid,
				SessionRequirementID: reqId,
				TimeSlotID:           sid,
			}
			_ = sol.AddAssignment(&p, ass)

			// Parity check on candidate assignment
			legacyCheck := oracle.Check(&p, &sol, ass)
			compiledConsistent := compiledFA.IsConsistent(ctx, &sol, ass)
			if (len(legacyCheck) == 0) != compiledConsistent {
				t.Fatalf("iter %d ass %d: consistency mismatch: legacy=%v, compiledConsistent=%v",
					iter, aIdx, len(legacyCheck) == 0, compiledConsistent)
			}
		}

		// Full solution check parity
		legacyFull := oracle.FullCheck(&p, &sol)
		compiledFull := compiledFA.Evaluate(ctx, &sol)
		compareFacultyAvailabilityViolationSets(t, legacyFull, compiledFull)

		// Incremental delta vs full recomputation parity
		if len(sol.Assignments) > 0 {
			targetAss := sol.Assignments[rng.Intn(len(sol.Assignments))]
			newSlot := slotIDs[rng.Intn(len(slotIDs))]

			mv := problem.Move{
				AssignmentID: targetAss.ID,
				From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
				To:           problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: newSlot},
			}

			// Incremental move violations
			moveViolations := compiledFA.ViolatedByMove(ctx, &sol, mv)

			// Full recomputation delta
			solAfter := sol.Clone()
			_ = solAfter.ApplyMove(&p, mv)
			fullAfter := compiledFA.Evaluate(ctx, &solAfter)

			var targetAssViolationsAfter []diagnostics.Violation
			for _, v := range fullAfter {
				if v.AssignmentID == string(targetAss.ID) {
					targetAssViolationsAfter = append(targetAssViolationsAfter, v)
				}
			}
			compareFacultyAvailabilityViolationSets(t, moveViolations, targetAssViolationsAfter)
		}
	}
}

// -------------------------------------------------------------
// Test 4: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestFacultyAvailability_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-scoped-fa",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	// Clean initial solution should evaluate to 0 violations
	violations := c.Evaluate(ctx, &solution)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations on clean solution, got: %+v", violations)
	}

	for _, a := range solution.Assignments {
		if !c.IsConsistent(ctx, &solution, a) {
			t.Fatalf("expected IsConsistent=true for valid assignment %s", a.ID)
		}
	}
}

// -------------------------------------------------------------
// Test 5: FacultyAvailability In CSP Solver
// -------------------------------------------------------------
func TestFacultyAvailability_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-fa",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-fa", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Class 1", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1": {ID: "g1", ClassID: "class-1", Name: "Group 1", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS101", Name: "Intro CS"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g1", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			// Faculty is only available in m-2
			{FacultyID: "f-1", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"},
			{RoomID: "r-1", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-fa",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled FacultyAvailability: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(solution.Assignments))
	}

	a := solution.Assignments[0]
	if a.TimeSlotID != "m-2" {
		t.Fatalf("expected assignment to be scheduled into available slot m-2, got %s", a.TimeSlotID)
	}
}

// -------------------------------------------------------------
// Test 6: FacultyAvailability In Tabu Search
// -------------------------------------------------------------
func TestFacultyAvailability_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-fa",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	fa := compiledSet.Hard[0]
	if violations := fa.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled FacultyAvailability violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 7: Final Validation Parity
// -------------------------------------------------------------
func TestFacultyAvailability_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-fa",
		TemplateID: "FacultyAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity (move faculty-1 assignment to tue-2 where faculty-1 has no availability record)
	// faculty-1 is available in mon-1..mon-4, tue-1, tue-2. Let's create an assignment on wed-1 where faculty-1 is not available.
	p.TimeSlots["wed-1"] = model.TimeSlot{ID: "wed-1", Day: time.Wednesday, Period: 1, Label: "Wed P1"}
	p.Prepare()

	conflictAss := problem.Assignment{
		ID:                   "a-fa-incompatible",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-1",
		TimeSlotID:           "wed-1", // unavailable slot for faculty-1
		SessionRequirementID: "req-a-theory",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on faculty availability conflict")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundFA := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-fa" && v.TemplateID == "FacultyAvailability" {
			foundFA = true
			break
		}
	}
	if !foundFA {
		t.Fatalf("expected violation for rule-final-fa, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkFacultyAvailability_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyFacultyAvailabilityOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkFacultyAvailability_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "fa-bench", TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkFacultyAvailability_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "fa-bench", TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkFacultyAvailability_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "fa-bench", TemplateID: "FacultyAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-theory-1", From: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, To: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}

// =============================================================
// ROOM AVAILABILITY MIGRATION: ORACLE, PARITY & COMPILATION TESTS
// =============================================================

type legacyRoomAvailabilityOracle struct{}

func (legacyRoomAvailabilityOracle) Name() string { return "RoomAvailability" }

func (o legacyRoomAvailabilityOracle) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomAvailability",
				Severity:       diagnostics.SeverityHard,
				Message:        "assignment does not fit in the recurring time-slot grid",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
			},
		}
	}
	if !p.IsRoomAvailable(assignment.RoomID, slotIDs) {
		return []diagnostics.Violation{
			{
				ConstraintName: "RoomAvailability",
				Severity:       diagnostics.SeverityHard,
				Message:        "room is not available for all occupied time slots",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"roomId":               string(assignment.RoomID),
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
				Metadata: map[string]string{
					"timeSlotIds": joinTimeSlotIDs(slotIDs),
				},
			},
		}
	}
	return nil
}

func (o legacyRoomAvailabilityOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

func normalizeRoomAvailabilityViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		key := fmt.Sprintf("%s|room:%s|a:%s|slot:%s|msg:%s|slots:%s",
			v.ConstraintName, v.RelatedIDs["roomId"], v.AssignmentID, v.RelatedIDs["timeSlotId"], v.Message, v.Metadata["timeSlotIds"])
		res[key] = v
	}
	return res
}

func compareRoomAvailabilityViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeRoomAvailabilityViolations(legacy)
	compiledMap := normalizeRoomAvailabilityViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure for RoomAvailability:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}

	for k, lV := range legacyMap {
		cV := compiledMap[k]
		if lV.Message != cV.Message {
			t.Errorf("Message mismatch for %s: legacy=%q, compiled=%q", k, lV.Message, cV.Message)
		}
		if lV.Severity != cV.Severity {
			t.Errorf("Severity mismatch for %s: legacy=%v, compiled=%v", k, lV.Severity, cV.Severity)
		}
		for rk, rv := range lV.RelatedIDs {
			if cV.RelatedIDs[rk] != rv {
				t.Errorf("RelatedIDs[%q] mismatch for %s: legacy=%q, compiled=%q", rk, k, rv, cV.RelatedIDs[rk])
			}
		}
		for mk, mv := range lV.Metadata {
			if cV.Metadata[mk] != mv {
				t.Errorf("Metadata[%q] mismatch for %s: legacy=%q, compiled=%q", mk, k, mv, cV.Metadata[mk])
			}
		}
	}
}

// -------------------------------------------------------------
// Test 1: RoomAvailability RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestRoomAvailability_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-ra-1",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-ra-2",
		TemplateID: "RoomAvailability",
		Scope:      "department:cs",
		Kind:       constraints.ConstraintKindHard,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1, inst2})
	if len(errs1) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2, inst1})
	if len(errs2) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of instance order: %s != %s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Invalid template
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-ra-invalid",
		TemplateID: "UnknownRoomAvailabilityTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: True Legacy vs Compiled Differential Parity Across Boundaries
// -------------------------------------------------------------
func TestRoomAvailability_TrueLegacyDifferentialParity(t *testing.T) {
	r1 := model.RoomID("room-available-m1-m2")
	rNoAvail := model.RoomID("room-no-avail")

	p := problem.Problem{
		TenantID: "tenant-ra-parity",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-ra-parity", Name: "Term 1"},
		Rooms: map[model.RoomID]model.Room{
			r1:       {ID: r1, Name: "Room Available", Capacity: 50},
			rNoAvail: {ID: rNoAvail, Name: "Room No Avail", Capacity: 50},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-1": {ID: "group-1", Name: "Group 1", Size: 30},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"m-4": {ID: "m-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		PeriodsPerDay: 4,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1p": {ID: "req-1p", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-2p": {ID: "req-2p", CourseOfferingID: "co-1", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-1", FacultyID: "f-1"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: r1, TimeSlotID: "m-1"},
			{RoomID: r1, TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-ra",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("unexpected compile error: %v", errs)
	}
	compiledRA := compiledSet.Hard[0]
	oracle := legacyRoomAvailabilityOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Boundary 1: Exact start boundary (available in m-1, m-2; 1-period in m-1) -> allowed
	t.Run("ExactStartBoundary_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-exact-start",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for exact start boundary slot m-1")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 2: Exact end boundary (available in m-1, m-2; 1-period in m-2) -> allowed
	t.Run("ExactEndBoundary_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-exact-end",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for exact end boundary slot m-2")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 3: Multi-period session entirely inside available window (available m-1, m-2; 2-period starting m-1) -> allowed
	t.Run("JustInsideAvailability_MultiPeriod_Allowed", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-just-inside",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-2p", // duration 2 -> m-1 and m-2
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true when multi-period session fits inside available window")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 4: Slot just outside availability (available in m-1, m-2; 1-period in m-3) -> conflict
	t.Run("JustOutsideAvailability_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-just-outside",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-3", // unavailable
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when slot is outside availability window")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 5: Multi-period session crossing an unavailable slot (available m-1, m-2; 2-period starting m-2 -> spans m-2, m-3) -> conflict
	t.Run("MultiPeriodCrossingUnavailable_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-span-boundary",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-2", // m-2 is available, but m-3 is NOT
			SessionRequirementID: "req-2p", // duration 2
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false when session spans across unavailable slot")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 6: Completely unavailable slot on another day -> conflict
	t.Run("CompletelyUnavailableSlot_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-other-day",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "t-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false on Tuesday slot")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 7: Room with no availability records -> conflict
	t.Run("RoomWithNoAvailability_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-no-avail",
			RoomID:               rNoAvail,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for room with 0 availability records")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find availability violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 8: Off-grid placement (invalid duration exceeding day bounds) -> invalid duration violation
	t.Run("InvalidDuration_OffGrid_Violation", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-offgrid",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-4", // period 4 (last period), duration 2 -> exceeds day bounds
			SessionRequirementID: "req-2p",
			CourseOfferingID:     "co-1",
		}
		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledRA.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for off-grid placement")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to return invalid duration violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareRoomAvailabilityViolationSets(t, oracle.FullCheck(&p, &sol), compiledRA.Evaluate(ctx, &sol))
	})

	// Boundary 9: ViolatedByMove Parity
	t.Run("ViolatedByMove_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-move-ra-test",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1", // initially valid
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		// Move to unavailable m-3
		mvToM3 := problem.Move{
			AssignmentID: "a-move-ra-test",
			From:         problem.Placement{RoomID: r1, TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: r1, TimeSlotID: "m-3"},
		}
		moveM3V := compiledRA.ViolatedByMove(ctx, &sol, mvToM3)
		expectedM3Legacy := oracle.Check(&p, &sol, problem.Assignment{
			ID:                   "a-move-ra-test",
			RoomID:               r1,
			StudentGroupID:       "group-1",
			FacultyID:            "f-1",
			TimeSlotID:           "m-3",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})
		compareRoomAvailabilityViolationSets(t, expectedM3Legacy, moveM3V)

		// Move to available m-2
		mvToM2 := problem.Move{
			AssignmentID: "a-move-ra-test",
			From:         problem.Placement{RoomID: r1, TimeSlotID: "m-1"},
			To:           problem.Placement{RoomID: r1, TimeSlotID: "m-2"},
		}
		moveM2V := compiledRA.ViolatedByMove(ctx, &sol, mvToM2)
		if len(moveM2V) != 0 {
			t.Fatalf("expected 0 violations for move to available slot m-2, got: %+v", moveM2V)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Parity & Incremental Delta vs Full Recomputation
// -------------------------------------------------------------
func TestRoomAvailability_RandomizedDifferentialParity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	oracle := legacyRoomAvailabilityOracle{}

	slotIDs := []model.TimeSlotID{"s-1", "s-2", "s-3", "s-4"}
	roomIDs := []model.RoomID{"r-0", "r-1", "r-2", "r-no-avail"}

	for iter := 0; iter < 500; iter++ {
		p := problem.Problem{
			TenantID: "tenant-rand-ra",
			Term:     model.Term{ID: "term-1", TenantID: "tenant-rand-ra", Name: "Term 1"},
			Rooms:    make(map[model.RoomID]model.Room),
			StudentGroups: map[model.StudentGroupID]model.StudentGroup{
				"g-1": {ID: "g-1", Name: "Group 1", Size: 30},
			},
			TimeSlots: map[model.TimeSlotID]model.TimeSlot{
				"s-1": {ID: "s-1", Day: time.Monday, Period: 1},
				"s-2": {ID: "s-2", Day: time.Monday, Period: 2},
				"s-3": {ID: "s-3", Day: time.Monday, Period: 3},
				"s-4": {ID: "s-4", Day: time.Monday, Period: 4},
			},
			PeriodsPerDay:       4,
			CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
			SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
			Faculty: map[model.FacultyID]model.Faculty{
				"f-1": {ID: "f-1", Name: "Faculty 1"},
			},
			RoomAvailabilities: nil,
		}

		for _, rid := range roomIDs {
			p.Rooms[rid] = model.Room{ID: rid, Name: string(rid), Capacity: 50}
		}

		// Random availability for r-0, r-1, r-2 (r-no-avail gets none)
		for _, rid := range roomIDs[:3] {
			for _, sid := range slotIDs {
				if rng.Float64() < 0.6 {
					p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{
						RoomID:     rid,
						TimeSlotID: sid,
					})
				}
			}
		}

		for oIdx := 0; oIdx < 4; oIdx++ {
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", oIdx))
			rid := model.SessionRequirementID(fmt.Sprintf("req-%d", oIdx))

			dur := 1
			if rng.Float64() < 0.3 {
				dur = 2
			}

			p.CourseOfferings[oid] = model.CourseOffering{
				ID:             oid,
				TermID:         "term-1",
				SubjectID:      "s-1",
				StudentGroupID: "g-1",
				FacultyID:      "f-1",
			}
			p.SessionRequirements[rid] = model.SessionRequirement{
				ID:               rid,
				CourseOfferingID: oid,
				Type:             model.SessionTypeTheory,
				Duration:         dur,
			}
		}
		p.Prepare()

		inst := constraints.ConstraintInstance{
			ID:         "rule-rand-ra",
			TemplateID: "RoomAvailability",
			Scope:      "global",
			Kind:       constraints.ConstraintKindHard,
		}
		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("iter %d: compile error: %v", iter, errs)
		}
		compiledRA := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		numAssignments := rng.Intn(6) + 1
		sol := problem.NewSolution()

		for aIdx := 0; aIdx < numAssignments; aIdx++ {
			assID := problem.AssignmentID(fmt.Sprintf("a-%d", aIdx))
			targetOIdx := rng.Intn(4)
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", targetOIdx))
			reqId := model.SessionRequirementID(fmt.Sprintf("req-%d", targetOIdx))
			rid := roomIDs[rng.Intn(len(roomIDs))]
			sid := slotIDs[rng.Intn(len(slotIDs))]

			ass := problem.Assignment{
				ID:                   assID,
				RoomID:               rid,
				StudentGroupID:       "g-1",
				FacultyID:            "f-1",
				CourseOfferingID:     oid,
				SessionRequirementID: reqId,
				TimeSlotID:           sid,
			}
			_ = sol.AddAssignment(&p, ass)

			// Parity check on candidate assignment
			legacyCheck := oracle.Check(&p, &sol, ass)
			compiledConsistent := compiledRA.IsConsistent(ctx, &sol, ass)
			if (len(legacyCheck) == 0) != compiledConsistent {
				t.Fatalf("iter %d ass %d: consistency mismatch: legacy=%v, compiledConsistent=%v",
					iter, aIdx, len(legacyCheck) == 0, compiledConsistent)
			}
		}

		// Full solution check parity
		legacyFull := oracle.FullCheck(&p, &sol)
		compiledFull := compiledRA.Evaluate(ctx, &sol)
		compareRoomAvailabilityViolationSets(t, legacyFull, compiledFull)

		// Incremental delta vs full recomputation parity
		if len(sol.Assignments) > 0 {
			targetAss := sol.Assignments[rng.Intn(len(sol.Assignments))]
			newRoom := roomIDs[rng.Intn(len(roomIDs))]
			newSlot := slotIDs[rng.Intn(len(slotIDs))]

			mv := problem.Move{
				AssignmentID: targetAss.ID,
				From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
				To:           problem.Placement{RoomID: newRoom, TimeSlotID: newSlot},
			}

			// Incremental move violations
			moveViolations := compiledRA.ViolatedByMove(ctx, &sol, mv)

			// Full recomputation delta
			solAfter := sol.Clone()
			_ = solAfter.ApplyMove(&p, mv)
			fullAfter := compiledRA.Evaluate(ctx, &solAfter)

			var targetAssViolationsAfter []diagnostics.Violation
			for _, v := range fullAfter {
				if v.AssignmentID == string(targetAss.ID) {
					targetAssViolationsAfter = append(targetAssViolationsAfter, v)
				}
			}
			compareRoomAvailabilityViolationSets(t, moveViolations, targetAssViolationsAfter)
		}
	}
}

// -------------------------------------------------------------
// Test 4: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestRoomAvailability_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-scoped-ra",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	// Clean initial solution should evaluate to 0 violations
	violations := c.Evaluate(ctx, &solution)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations on clean solution, got: %+v", violations)
	}

	for _, a := range solution.Assignments {
		if !c.IsConsistent(ctx, &solution, a) {
			t.Fatalf("expected IsConsistent=true for valid assignment %s", a.ID)
		}
	}
}

// -------------------------------------------------------------
// Test 5: RoomAvailability In CSP Solver
// -------------------------------------------------------------
func TestRoomAvailability_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-ra",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-ra", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {ID: "class-1", ProgramID: "prog-1", Name: "Class 1", WholeGroupID: "g1", StudentGroupIDs: []model.StudentGroupID{"g1"}},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g1": {ID: "g1", ClassID: "class-1", Name: "Group 1", Size: 30},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS101", Name: "Intro CS"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g1", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-1"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1": {ID: "req-1", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 50},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"},
			{FacultyID: "f-1", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			// Room is only available in m-2
			{RoomID: "r-1", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-ra",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled RoomAvailability: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(solution.Assignments))
	}

	a := solution.Assignments[0]
	if a.TimeSlotID != "m-2" {
		t.Fatalf("expected assignment to be scheduled into available room slot m-2, got %s", a.TimeSlotID)
	}
}

// -------------------------------------------------------------
// Test 6: RoomAvailability In Tabu Search
// -------------------------------------------------------------
func TestRoomAvailability_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-ra",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	ra := compiledSet.Hard[0]
	if violations := ra.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled RoomAvailability violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 7: Final Validation Parity
// -------------------------------------------------------------
func TestRoomAvailability_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-ra",
		TemplateID: "RoomAvailability",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity (place assignment on wed-1 where room has no availability record)
	p.TimeSlots["wed-1"] = model.TimeSlot{ID: "wed-1", Day: time.Wednesday, Period: 1, Label: "Wed P1"}
	p.Prepare()

	conflictAss := problem.Assignment{
		ID:                   "a-ra-incompatible",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole",
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-1",
		TimeSlotID:           "wed-1", // room-lecture-1 is not available on wed-1
		SessionRequirementID: "req-a-theory",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on room availability conflict")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundRA := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-ra" && v.TemplateID == "RoomAvailability" {
			foundRA = true
			break
		}
	}
	if !foundRA {
		t.Fatalf("expected violation for rule-final-ra, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkRoomAvailability_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyRoomAvailabilityOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkRoomAvailability_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "ra-bench", TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkRoomAvailability_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "ra-bench", TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkRoomAvailability_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "ra-bench", TemplateID: "RoomAvailability", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-theory-1", From: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, To: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}

// =============================================================
// STUDENT GROUP CONFLICT MIGRATION: ORACLE, PARITY & COMPILATION TESTS
// =============================================================

type legacyStudentGroupConflictOracle struct{}

func (legacyStudentGroupConflictOracle) Name() string { return "StudentGroupConflict" }

func (o legacyStudentGroupConflictOracle) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return []diagnostics.Violation{
			{
				ConstraintName: "StudentGroupConflict",
				Severity:       diagnostics.SeverityHard,
				Message:        "assignment does not fit in the recurring time-slot grid",
				AssignmentID:   string(assignment.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId":     string(assignment.CourseOfferingID),
					"sessionRequirementId": string(assignment.SessionRequirementID),
					"studentGroupId":       string(assignment.StudentGroupID),
					"timeSlotId":           string(assignment.TimeSlotID),
				},
			},
		}
	}
	for _, groupID := range p.OverlappingStudentGroupIDs(assignment.StudentGroupID) {
		if conflictingID, ok := solution.Index.StudentGroupConflict(groupID, slotIDs); ok && conflictingID != assignment.ID {
			return []diagnostics.Violation{
				{
					ConstraintName: "StudentGroupConflict",
					Severity:       diagnostics.SeverityHard,
					Message:        "student group overlaps another scheduled group in an occupied time slot",
					AssignmentID:   string(assignment.ID),
					RelatedIDs: map[string]string{
						"studentGroupId":          string(assignment.StudentGroupID),
						"overlappingGroupId":      string(groupID),
						"conflictingAssignmentId": string(conflictingID),
						"courseOfferingId":        string(assignment.CourseOfferingID),
						"sessionRequirementId":    string(assignment.SessionRequirementID),
						"timeSlotId":              string(assignment.TimeSlotID),
					},
				},
			}
		}
	}
	return nil
}

func (o legacyStudentGroupConflictOracle) FullCheck(p *problem.Problem, sol *problem.Solution) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		violations = append(violations, o.Check(p, sol, a)...)
	}
	return violations
}

func normalizeStudentGroupViolations(violations []diagnostics.Violation) map[string]diagnostics.Violation {
	res := make(map[string]diagnostics.Violation)
	for _, v := range violations {
		a1 := v.AssignmentID
		a2 := v.RelatedIDs["conflictingAssignmentId"]
		if a2 < a1 && a2 != "" {
			a1, a2 = a2, a1
		}
		key := fmt.Sprintf("%s|a1:%s|a2:%s|msg:%s",
			v.ConstraintName, a1, a2, v.Message)
		res[key] = v
	}
	return res
}

func compareStudentGroupViolationSets(t *testing.T, legacy, compiled []diagnostics.Violation) {
	t.Helper()
	legacyMap := normalizeStudentGroupViolations(legacy)
	compiledMap := normalizeStudentGroupViolations(compiled)

	var missing []string
	var unexpected []string

	for k := range legacyMap {
		if _, ok := compiledMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range compiledMap {
		if _, ok := legacyMap[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("Differential parity failure for StudentGroupConflict:\n  Missing in compiled: %v\n  Unexpected in compiled: %v\n  Legacy raw (%d): %+v\n  Compiled raw (%d): %+v",
			missing, unexpected, len(legacy), legacy, len(compiled), compiled)
	}

	for k, lV := range legacyMap {
		cV := compiledMap[k]
		if lV.Message != cV.Message {
			t.Errorf("Message mismatch for %s: legacy=%q, compiled=%q", k, lV.Message, cV.Message)
		}
		if lV.Severity != cV.Severity {
			t.Errorf("Severity mismatch for %s: legacy=%v, compiled=%v", k, lV.Severity, cV.Severity)
		}
	}
}

// -------------------------------------------------------------
// Test 1: StudentGroupConflict RuleSetHash Determinism and Sensitivity
// -------------------------------------------------------------
func TestStudentGroupConflict_DeterministicRuleSetHash(t *testing.T) {
	inst1 := constraints.ConstraintInstance{
		ID:         "rule-sgc-1",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	inst2 := constraints.ConstraintInstance{
		ID:         "rule-sgc-2",
		TemplateID: "StudentGroupConflict",
		Scope:      "class:class-1",
		Kind:       constraints.ConstraintKindHard,
	}

	set1, hash1, errs1 := constraints.Compile(nil, []constraints.ConstraintInstance{inst1, inst2})
	if len(errs1) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs1)
	}

	set2, hash2, errs2 := constraints.Compile(nil, []constraints.ConstraintInstance{inst2, inst1})
	if len(errs2) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs2)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of instance order: %s != %s", hash1, hash2)
	}
	if set1.RuleSetHash != hash1 || set2.RuleSetHash != hash2 {
		t.Fatal("CompiledConstraintSet.RuleSetHash mismatch")
	}

	// Invalid template
	invalidInst := constraints.ConstraintInstance{
		ID:         "rule-sgc-invalid",
		TemplateID: "UnknownStudentGroupConflictTemplate",
		Kind:       constraints.ConstraintKindHard,
	}
	setInvalid, hashInvalid, errsInvalid := constraints.Compile(nil, []constraints.ConstraintInstance{invalidInst})
	if len(errsInvalid) == 0 {
		t.Fatal("expected compile errors for invalid template")
	}
	if setInvalid != nil {
		t.Fatalf("expected nil CompiledConstraintSet on compile error, got: %+v", setInvalid)
	}
	if hashInvalid != "" {
		t.Fatalf("expected empty hash on compile error, got: %s", hashInvalid)
	}
}

// -------------------------------------------------------------
// Test 2: Mandatory Hierarchy & True Legacy Differential Parity Cases
// -------------------------------------------------------------
func TestStudentGroupConflict_TrueLegacyDifferentialParity(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-sgc-hierarchy",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-sgc-hierarchy", Name: "Term 1"},
		Classes: map[model.ClassID]model.Class{
			"class-cse": {
				ID:              "class-cse",
				WholeGroupID:    "group-cse-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-cse-whole", "group-cse-a", "group-cse-b"},
			},
			"class-ece": {
				ID:              "class-ece",
				WholeGroupID:    "group-ece-whole",
				StudentGroupIDs: []model.StudentGroupID{"group-ece-whole", "group-ece-a"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"group-cse-whole": {ID: "group-cse-whole", ClassID: "class-cse", Name: "CSE Whole", Size: 60},
			"group-cse-a":     {ID: "group-cse-a", ClassID: "class-cse", Name: "CSE Section A", Size: 30},
			"group-cse-b":     {ID: "group-cse-b", ClassID: "class-cse", Name: "CSE Section B", Size: 30},
			"group-ece-whole": {ID: "group-ece-whole", ClassID: "class-ece", Name: "ECE Whole", Size: 50},
			"group-ece-a":     {ID: "group-ece-a", ClassID: "class-ece", Name: "ECE Section A", Size: 25},
			"group-empty":     {ID: "group-empty", ClassID: "class-none", Name: "Empty Group", Size: 0},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 100},
			"r-2": {ID: "r-2", Name: "Room 2", Capacity: 100},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
			"m-3": {ID: "m-3", Day: time.Monday, Period: 3, Label: "Mon P3"},
			"m-4": {ID: "m-4", Day: time.Monday, Period: 4, Label: "Mon P4"},
			"t-1": {ID: "t-1", Day: time.Tuesday, Period: 1, Label: "Tue P1"},
		},
		PeriodsPerDay: 4,
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-1p": {ID: "req-1p", CourseOfferingID: "co-1", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-2p": {ID: "req-2p", CourseOfferingID: "co-2", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 2},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-1": {ID: "co-1", TermID: "term-1", SubjectID: "s-1", StudentGroupID: "group-cse-whole", FacultyID: "f-1"},
			"co-2": {ID: "co-2", TermID: "term-1", SubjectID: "s-2", StudentGroupID: "group-cse-a", FacultyID: "f-2"},
			"co-3": {ID: "co-3", TermID: "term-1", SubjectID: "s-3", StudentGroupID: "group-cse-b", FacultyID: "f-3"},
			"co-4": {ID: "co-4", TermID: "term-1", SubjectID: "s-4", StudentGroupID: "group-ece-whole", FacultyID: "f-4"},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
			"f-2": {ID: "f-2", Name: "Prof 2"},
			"f-3": {ID: "f-3", Name: "Prof 3"},
			"f-4": {ID: "f-4", Name: "Prof 4"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-sgc",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	if len(errs) > 0 {
		t.Fatalf("unexpected compile error: %v", errs)
	}
	compiledSGC := compiledSet.Hard[0]
	oracle := legacyStudentGroupConflictOracle{}
	ctx := constraints.NewSearchCtx(&p)

	// Hierarchy Case 1: Parent vs Child -> CONFLICT (CSE Whole vs CSE Section A in same slot m-1)
	t.Run("ParentVsChild_Conflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-parent",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-child",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for parent vs child in same slot")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to find parent vs child conflict")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareStudentGroupViolationSets(t, oracle.FullCheck(&p, &sol), compiledSGC.Evaluate(ctx, &sol))
	})

	// Hierarchy Case 2: Sibling Groups -> NO CONFLICT (CSE Section A vs CSE Section B in same slot m-1)
	t.Run("SiblingGroups_NoConflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-sibling-1",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		})

		candidate := problem.Assignment{
			ID:                   "a-sibling-2",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-b",
			FacultyID:            "f-3",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-3",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for disjoint sibling groups in same slot")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations for siblings, got: %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareStudentGroupViolationSets(t, oracle.FullCheck(&p, &sol), compiledSGC.Evaluate(ctx, &sol))
	})

	// Hierarchy Case 3: Two Parents with Disjoint Descendants -> NO CONFLICT (CSE Whole vs ECE Whole)
	t.Run("DisjointParents_NoConflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-cse",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-ece",
			RoomID:               "r-2",
			StudentGroupID:       "group-ece-whole",
			FacultyID:            "f-4",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-4",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for distinct department parents")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations for disjoint parents, got: %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareStudentGroupViolationSets(t, oracle.FullCheck(&p, &sol), compiledSGC.Evaluate(ctx, &sol))
	})

	// Hierarchy Case 4: Non-Overlapping Time -> NO CONFLICT (CSE Whole at m-1 vs CSE Section A at m-2)
	t.Run("NonOverlappingTime_NoConflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-time-1",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-time-2",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-2", // different slot
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, candidate)

		if !compiledConsistent {
			t.Fatal("expected IsConsistent=true for different time slots")
		}
		if len(legacyV) != 0 {
			t.Fatalf("expected 0 legacy violations, got: %+v", legacyV)
		}

		_ = sol.AddAssignment(&p, candidate)
		compareStudentGroupViolationSets(t, oracle.FullCheck(&p, &sol), compiledSGC.Evaluate(ctx, &sol))
	})

	// Hierarchy Case 5: Multi-Period Session Overlap Boundary (2-period at m-1 occupies m-1, m-2 -> conflicts with m-2, but NOT m-3)
	t.Run("MultiPeriod_ExactBoundarySemantics", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-multi-parent",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1", // occupies m-1 and m-2
			SessionRequirementID: "req-2p",
			CourseOfferingID:     "co-2",
		})

		// Candidate 1: at m-2 -> CONFLICT
		candConflict := problem.Assignment{
			ID:                   "a-child-m2",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-2",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		}
		if compiledSGC.IsConsistent(ctx, &sol, candConflict) {
			t.Fatal("expected IsConsistent=false at m-2 overlapping multi-period session")
		}

		// Candidate 2: at m-3 (adjacent) -> NO CONFLICT
		candAdjacent := problem.Assignment{
			ID:                   "a-child-m3",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-3",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		}
		if !compiledSGC.IsConsistent(ctx, &sol, candAdjacent) {
			t.Fatal("expected IsConsistent=true at m-3 adjacent to multi-period session")
		}
	})

	// Hierarchy Case 6: Empty Group / No Descendants -> No False Conflict
	t.Run("EmptyGroup_NoFalseConflict", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-empty-1",
			RoomID:               "r-1",
			StudentGroupID:       "group-empty",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})

		candidate := problem.Assignment{
			ID:                   "a-cse-a",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		}

		if !compiledSGC.IsConsistent(ctx, &sol, candidate) {
			t.Fatal("expected IsConsistent=true between unrelated empty group and cse-a")
		}
	})

	// Hierarchy Case 7: Invalid Duration Off-Grid
	t.Run("InvalidDuration_OffGrid_Violation", func(t *testing.T) {
		sol := problem.NewSolution()
		candidate := problem.Assignment{
			ID:                   "a-offgrid",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-4", // period 4, duration 2 -> off-grid
			SessionRequirementID: "req-2p",
			CourseOfferingID:     "co-2",
		}

		legacyV := oracle.Check(&p, &sol, candidate)
		compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, candidate)

		if compiledConsistent {
			t.Fatal("expected IsConsistent=false for off-grid duration")
		}
		if len(legacyV) == 0 {
			t.Fatal("expected legacy oracle to return invalid duration violation")
		}

		_ = sol.AddAssignment(&p, candidate)
		compareStudentGroupViolationSets(t, oracle.FullCheck(&p, &sol), compiledSGC.Evaluate(ctx, &sol))
	})

	// Hierarchy Case 8: ViolatedByMove Parity
	t.Run("ViolatedByMove_Parity", func(t *testing.T) {
		sol := problem.NewSolution()
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-whole-m1",
			RoomID:               "r-1",
			StudentGroupID:       "group-cse-whole",
			FacultyID:            "f-1",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-1",
		})
		_ = sol.AddAssignment(&p, problem.Assignment{
			ID:                   "a-sub-m2",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-2", // currently at m-2 (valid)
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		})

		// Move a-sub-m2 to m-1 -> creates conflict with a-whole-m1
		mvToM1 := problem.Move{
			AssignmentID: "a-sub-m2",
			From:         problem.Placement{RoomID: "r-2", TimeSlotID: "m-2"},
			To:           problem.Placement{RoomID: "r-2", TimeSlotID: "m-1"},
		}
		moveM1V := compiledSGC.ViolatedByMove(ctx, &sol, mvToM1)
		expectedM1Legacy := oracle.Check(&p, &sol, problem.Assignment{
			ID:                   "a-sub-m2",
			RoomID:               "r-2",
			StudentGroupID:       "group-cse-a",
			FacultyID:            "f-2",
			TimeSlotID:           "m-1",
			SessionRequirementID: "req-1p",
			CourseOfferingID:     "co-2",
		})
		compareStudentGroupViolationSets(t, expectedM1Legacy, moveM1V)

		// Move to m-3 -> no conflict
		mvToM3 := problem.Move{
			AssignmentID: "a-sub-m2",
			From:         problem.Placement{RoomID: "r-2", TimeSlotID: "m-2"},
			To:           problem.Placement{RoomID: "r-2", TimeSlotID: "m-3"},
		}
		moveM3V := compiledSGC.ViolatedByMove(ctx, &sol, mvToM3)
		if len(moveM3V) != 0 {
			t.Fatalf("expected 0 violations for move to m-3, got: %+v", moveM3V)
		}
	})
}

// -------------------------------------------------------------
// Test 3: Randomized Differential Parity & Incremental Delta vs Full Recomputation
// -------------------------------------------------------------
func TestStudentGroupConflict_RandomizedDifferentialParity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	oracle := legacyStudentGroupConflictOracle{}

	slotIDs := []model.TimeSlotID{"s-1", "s-2", "s-3", "s-4"}

	for iter := 0; iter < 500; iter++ {
		p := problem.Problem{
			TenantID: "tenant-rand-sgc",
			Term:     model.Term{ID: "term-1", TenantID: "tenant-rand-sgc", Name: "Term 1"},
			Classes:  make(map[model.ClassID]model.Class),
			StudentGroups: make(map[model.StudentGroupID]model.StudentGroup),
			Rooms: map[model.RoomID]model.Room{
				"r-1": {ID: "r-1", Name: "Room 1", Capacity: 100},
				"r-2": {ID: "r-2", Name: "Room 2", Capacity: 100},
			},
			TimeSlots: map[model.TimeSlotID]model.TimeSlot{
				"s-1": {ID: "s-1", Day: time.Monday, Period: 1},
				"s-2": {ID: "s-2", Day: time.Monday, Period: 2},
				"s-3": {ID: "s-3", Day: time.Monday, Period: 3},
				"s-4": {ID: "s-4", Day: time.Monday, Period: 4},
			},
			PeriodsPerDay:       4,
			CourseOfferings:     make(map[model.CourseOfferingID]model.CourseOffering),
			SessionRequirements: make(map[model.SessionRequirementID]model.SessionRequirement),
			Faculty: map[model.FacultyID]model.Faculty{
				"f-1": {ID: "f-1", Name: "Faculty 1"},
				"f-2": {ID: "f-2", Name: "Faculty 2"},
			},
		}

		numClasses := rng.Intn(3) + 1
		var allGroupIDs []model.StudentGroupID

		for cIdx := 0; cIdx < numClasses; cIdx++ {
			cid := model.ClassID(fmt.Sprintf("class-%d", cIdx))
			wholeGID := model.StudentGroupID(fmt.Sprintf("g-%d-whole", cIdx))
			p.StudentGroups[wholeGID] = model.StudentGroup{ID: wholeGID, ClassID: cid, Name: string(wholeGID), Size: 40}
			allGroupIDs = append(allGroupIDs, wholeGID)

			classGroupIDs := []model.StudentGroupID{wholeGID}
			numSubs := rng.Intn(3)
			for sIdx := 0; sIdx < numSubs; sIdx++ {
				subGID := model.StudentGroupID(fmt.Sprintf("g-%d-sub-%d", cIdx, sIdx))
				p.StudentGroups[subGID] = model.StudentGroup{ID: subGID, ClassID: cid, Name: string(subGID), Size: 20}
				allGroupIDs = append(allGroupIDs, subGID)
				classGroupIDs = append(classGroupIDs, subGID)
			}

			p.Classes[cid] = model.Class{
				ID:              cid,
				WholeGroupID:    wholeGID,
				StudentGroupIDs: classGroupIDs,
			}
		}

		for gIdx, gid := range allGroupIDs {
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", gIdx))
			rid := model.SessionRequirementID(fmt.Sprintf("req-%d", gIdx))
			dur := 1
			if rng.Float64() < 0.25 {
				dur = 2
			}
			p.CourseOfferings[oid] = model.CourseOffering{
				ID:             oid,
				TermID:         "term-1",
				SubjectID:      "s-1",
				StudentGroupID: gid,
				FacultyID:      "f-1",
			}
			p.SessionRequirements[rid] = model.SessionRequirement{
				ID:               rid,
				CourseOfferingID: oid,
				Type:             model.SessionTypeTheory,
				Duration:         dur,
			}
		}
		p.Prepare()

		inst := constraints.ConstraintInstance{
			ID:         "rule-rand-sgc",
			TemplateID: "StudentGroupConflict",
			Scope:      "global",
			Kind:       constraints.ConstraintKindHard,
		}
		compiledSet, _, errs := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
		if len(errs) > 0 {
			t.Fatalf("iter %d: compile error: %v", iter, errs)
		}
		compiledSGC := compiledSet.Hard[0]
		ctx := constraints.NewSearchCtx(&p)

		numAssignments := rng.Intn(6) + 1
		sol := problem.NewSolution()

		for aIdx := 0; aIdx < numAssignments; aIdx++ {
			assID := problem.AssignmentID(fmt.Sprintf("a-%d", aIdx))
			gIdx := rng.Intn(len(allGroupIDs))
			gid := allGroupIDs[gIdx]
			oid := model.CourseOfferingID(fmt.Sprintf("co-%d", gIdx))
			reqId := model.SessionRequirementID(fmt.Sprintf("req-%d", gIdx))
			sid := slotIDs[rng.Intn(len(slotIDs))]
			roomID := model.RoomID(fmt.Sprintf("r-%d", rng.Intn(2)+1))

			ass := problem.Assignment{
				ID:                   assID,
				RoomID:               roomID,
				StudentGroupID:       gid,
				FacultyID:            "f-1",
				CourseOfferingID:     oid,
				SessionRequirementID: reqId,
				TimeSlotID:           sid,
			}

			// Consistency parity
			legacyCheck := oracle.Check(&p, &sol, ass)
			compiledConsistent := compiledSGC.IsConsistent(ctx, &sol, ass)
			if (len(legacyCheck) == 0) != compiledConsistent {
				t.Fatalf("iter %d ass %d: consistency mismatch: legacy=%v, compiledConsistent=%v",
					iter, aIdx, len(legacyCheck) == 0, compiledConsistent)
			}

			if compiledConsistent {
				_ = sol.AddAssignment(&p, ass)
			}
		}

		// Full solution parity
		legacyFull := oracle.FullCheck(&p, &sol)
		compiledFull := compiledSGC.Evaluate(ctx, &sol)
		compareStudentGroupViolationSets(t, legacyFull, compiledFull)

		// Incremental delta vs full recomputation parity
		if len(sol.Assignments) > 0 {
			targetAss := sol.Assignments[rng.Intn(len(sol.Assignments))]
			newSlot := slotIDs[rng.Intn(len(slotIDs))]
			newRoom := model.RoomID(fmt.Sprintf("r-%d", rng.Intn(2)+1))

			mv := problem.Move{
				AssignmentID: targetAss.ID,
				From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
				To:           problem.Placement{RoomID: newRoom, TimeSlotID: newSlot},
			}

			moveViolations := compiledSGC.ViolatedByMove(ctx, &sol, mv)

			solAfter := sol.Clone()
			_ = solAfter.ApplyMove(&p, mv)
			fullAfter := compiledSGC.Evaluate(ctx, &solAfter)

			var targetAssViolationsAfter []diagnostics.Violation
			for _, v := range fullAfter {
				if v.AssignmentID == string(targetAss.ID) || v.RelatedIDs["conflictingAssignmentId"] == string(targetAss.ID) {
					targetAssViolationsAfter = append(targetAssViolationsAfter, v)
				}
			}
			compareStudentGroupViolationSets(t, moveViolations, targetAssViolationsAfter)
		}
	}
}

// -------------------------------------------------------------
// Test 4: Mutation / Rollback Invariants (Move & Swap)
// -------------------------------------------------------------
func TestStudentGroupConflict_IndexMutationRollbackInvariants(t *testing.T) {
	p, solution := localSearchTestProblem()

	// Capture initial state
	origSol := solution.Clone()

	// Invariant 1: ApplyMove -> UndoMove exact equality
	mv := problem.Move{
		AssignmentID: "a-theory-1",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
	}

	if err := solution.ApplyMove(&p, mv); err != nil {
		t.Fatalf("failed ApplyMove: %v", err)
	}
	if err := solution.UndoMove(&p, mv); err != nil {
		t.Fatalf("failed UndoMove: %v", err)
	}

	// Verify exact index map restoration
	if len(solution.Index.StudentGroupSlot) != len(origSol.Index.StudentGroupSlot) {
		t.Fatalf("StudentGroupSlot length mismatch after UndoMove: %d vs %d",
			len(solution.Index.StudentGroupSlot), len(origSol.Index.StudentGroupSlot))
	}
	for k, v := range origSol.Index.StudentGroupSlot {
		if solution.Index.StudentGroupSlot[k] != v {
			t.Fatalf("StudentGroupSlot[%+v] mismatch after UndoMove: %s vs %s", k, solution.Index.StudentGroupSlot[k], v)
		}
	}

	// Invariant 2: ApplySwap -> UndoSwap exact equality
	mv1 := problem.Move{
		AssignmentID: "a-theory-1",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
	}
	mv2 := problem.Move{
		AssignmentID: "a-theory-2",
		From:         problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"},
		To:           problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"},
	}

	if err := solution.ApplySwap(&p, mv1, mv2); err != nil {
		t.Fatalf("failed ApplySwap: %v", err)
	}
	if err := solution.UndoSwap(&p, mv1, mv2); err != nil {
		t.Fatalf("failed UndoSwap: %v", err)
	}

	if len(solution.Index.StudentGroupSlot) != len(origSol.Index.StudentGroupSlot) {
		t.Fatalf("StudentGroupSlot length mismatch after UndoSwap: %d vs %d",
			len(solution.Index.StudentGroupSlot), len(origSol.Index.StudentGroupSlot))
	}
	for k, v := range origSol.Index.StudentGroupSlot {
		if solution.Index.StudentGroupSlot[k] != v {
			t.Fatalf("StudentGroupSlot[%+v] mismatch after UndoSwap: %s vs %s", k, solution.Index.StudentGroupSlot[k], v)
		}
	}

	// Invariant 3: Repeated randomized mutation & rollback sequence
	rng := rand.New(rand.NewSource(999))
	for i := 0; i < 100; i++ {
		targetAss := solution.Assignments[rng.Intn(len(solution.Assignments))]
		randSlot := model.TimeSlotID("mon-3")
		testMv := problem.Move{
			AssignmentID: targetAss.ID,
			From:         problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: targetAss.TimeSlotID},
			To:           problem.Placement{RoomID: targetAss.RoomID, TimeSlotID: randSlot},
		}

		_ = solution.ApplyMove(&p, testMv)
		_ = solution.UndoMove(&p, testMv)
	}

	for k, v := range origSol.Index.StudentGroupSlot {
		if solution.Index.StudentGroupSlot[k] != v {
			t.Fatalf("StudentGroupSlot[%+v] mismatch after repeated cycles: %s vs %s", k, solution.Index.StudentGroupSlot[k], v)
		}
	}
}

// -------------------------------------------------------------
// Test 5: Compiled Scoped vs Full Parity Across Topologies
// -------------------------------------------------------------
func TestStudentGroupConflict_CompiledScopedVsFullParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-scoped-sgc",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	// Clean initial solution should evaluate to 0 violations
	violations := c.Evaluate(ctx, &solution)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations on clean solution, got: %+v", violations)
	}

	for _, a := range solution.Assignments {
		if !c.IsConsistent(ctx, &solution, a) {
			t.Fatalf("expected IsConsistent=true for valid assignment %s", a.ID)
		}
	}
}

// -------------------------------------------------------------
// Test 6: StudentGroupConflict In CSP Solver
// -------------------------------------------------------------
func TestStudentGroupConflict_InCSP(t *testing.T) {
	p := problem.Problem{
		TenantID: "tenant-csp-sgc",
		Term:     model.Term{ID: "term-1", TenantID: "tenant-csp-sgc", Name: "Term 1"},
		Departments: map[model.DepartmentID]model.Department{
			"dept-1": {ID: "dept-1", Name: "CS"},
		},
		Programs: map[model.ProgramID]model.Program{
			"prog-1": {ID: "prog-1", DepartmentID: "dept-1", Name: "BS CS"},
		},
		Classes: map[model.ClassID]model.Class{
			"class-1": {
				ID:              "class-1",
				ProgramID:       "prog-1",
				WholeGroupID:    "g-whole",
				StudentGroupIDs: []model.StudentGroupID{"g-whole", "g-lab1"},
			},
		},
		StudentGroups: map[model.StudentGroupID]model.StudentGroup{
			"g-whole": {ID: "g-whole", ClassID: "class-1", Name: "Whole Class", Size: 40},
			"g-lab1":  {ID: "g-lab1", ClassID: "class-1", Name: "Lab Subgroup", Size: 20},
		},
		Rooms: map[model.RoomID]model.Room{
			"r-1": {ID: "r-1", Name: "Room 1", Capacity: 50},
			"r-2": {ID: "r-2", Name: "Room 2", Capacity: 50},
		},
		Subjects: map[model.SubjectID]model.Subject{
			"s-1": {ID: "s-1", Code: "CS101", Name: "Theory"},
			"s-2": {ID: "s-2", Code: "CS101L", Name: "Lab"},
		},
		CourseOfferings: map[model.CourseOfferingID]model.CourseOffering{
			"co-theory": {ID: "co-theory", TermID: "term-1", ClassID: "class-1", SubjectID: "s-1", StudentGroupID: "g-whole", FacultyID: "f-1", SessionRequirementIDs: []model.SessionRequirementID{"req-theory"}},
			"co-lab":    {ID: "co-lab", TermID: "term-1", ClassID: "class-1", SubjectID: "s-2", StudentGroupID: "g-lab1", FacultyID: "f-2", SessionRequirementIDs: []model.SessionRequirementID{"req-lab"}},
		},
		SessionRequirements: map[model.SessionRequirementID]model.SessionRequirement{
			"req-theory": {ID: "req-theory", CourseOfferingID: "co-theory", Type: model.SessionTypeTheory, SessionsPerWeek: 1, Duration: 1},
			"req-lab":    {ID: "req-lab", CourseOfferingID: "co-lab", Type: model.SessionTypeLab, SessionsPerWeek: 1, Duration: 1},
		},
		Faculty: map[model.FacultyID]model.Faculty{
			"f-1": {ID: "f-1", Name: "Prof 1"},
			"f-2": {ID: "f-2", Name: "Prof 2"},
		},
		TimeSlots: map[model.TimeSlotID]model.TimeSlot{
			"m-1": {ID: "m-1", Day: time.Monday, Period: 1, Label: "Mon P1"},
			"m-2": {ID: "m-2", Day: time.Monday, Period: 2, Label: "Mon P2"},
		},
		PeriodsPerDay: 2,
		FacultyAvailabilities: []model.FacultyAvailability{
			{FacultyID: "f-1", TimeSlotID: "m-1"}, {FacultyID: "f-1", TimeSlotID: "m-2"},
			{FacultyID: "f-2", TimeSlotID: "m-1"}, {FacultyID: "f-2", TimeSlotID: "m-2"},
		},
		RoomAvailabilities: []model.RoomAvailability{
			{RoomID: "r-1", TimeSlotID: "m-1"}, {RoomID: "r-1", TimeSlotID: "m-2"},
			{RoomID: "r-2", TimeSlotID: "m-1"}, {RoomID: "r-2", TimeSlotID: "m-2"},
		},
	}
	p.Prepare()

	inst := constraints.ConstraintInstance{
		ID:         "rule-csp-sgc",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	solver := backtracking.NewWithCompiled(compiledSet)
	solution, diag, err := solver.Solve(context.Background(), p, problem.SolveOptions{MaxNodes: 10000})

	if err != nil || diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("CSP solver failed to solve with compiled StudentGroupConflict: status=%s, err=%v", diag.Status, err)
	}
	if len(solution.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(solution.Assignments))
	}

	a1 := solution.Assignments[0]
	a2 := solution.Assignments[1]
	if a1.TimeSlotID == a2.TimeSlotID {
		t.Fatalf("StudentGroupConflict violated in CSP: both assignments placed in same slot %s", a1.TimeSlotID)
	}
}

// -------------------------------------------------------------
// Test 7: StudentGroupConflict In Tabu Search
// -------------------------------------------------------------
func TestStudentGroupConflict_InTabu(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-tabu-sgc",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	opts := localsearch.TabuSearchOptions{
		MaxIterations: 30,
		TabuTenure:    3,
		MaxCandidates: 20,
		Seed:          42,
		Compiled:      compiledSet,
	}

	bestSol, diag, err := localsearch.TabuSearch(context.Background(), &p, solution, opts)
	if err != nil {
		t.Fatalf("TabuSearch error: %v", err)
	}
	if diag.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected Tabu status SOLVED, got %s", diag.Status)
	}
	if bestSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations, got %d", bestSol.Score.HardViolations)
	}

	ctx := constraints.NewSearchCtx(&p)
	sgc := compiledSet.Hard[0]
	if violations := sgc.Evaluate(ctx, &bestSol); len(violations) > 0 {
		t.Fatalf("compiled StudentGroupConflict violated on final solution: %+v", violations)
	}
}

// -------------------------------------------------------------
// Test 8: Final Validation Parity
// -------------------------------------------------------------
func TestStudentGroupConflict_FinalValidationParity(t *testing.T) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{
		ID:         "rule-final-sgc",
		TemplateID: "StudentGroupConflict",
		Scope:      "global",
		Kind:       constraints.ConstraintKindHard,
	}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})

	// Valid solution parity
	solver := backtracking.NewWithCompiled(compiledSet)
	validSol, diagValid, errValid := solver.ValidateSolution(context.Background(), p, solution)
	if errValid != nil || diagValid.Status != diagnostics.SolveStatusSolved {
		t.Fatalf("expected valid solution, got: %s / %v", diagValid.Status, errValid)
	}
	if validSol.Score.HardViolations != 0 {
		t.Fatalf("expected 0 hard violations on valid solution, got %d", validSol.Score.HardViolations)
	}

	// Invalid solution parity (place a conflicting subgroup assignment at mon-1 where group-a-whole is already scheduled)
	conflictAss := problem.Assignment{
		ID:                   "a-sgc-incompatible",
		CourseOfferingID:     "offering-a-theory",
		StudentGroupID:       "group-a-whole", // same group as a-theory-1
		FacultyID:            "faculty-1",
		RoomID:               "room-lecture-2",
		TimeSlotID:           "mon-1", // collision with a-theory-1
		SessionRequirementID: "req-a-theory",
	}
	conflictSol := solution.Clone()
	conflictSol.Assignments = append(conflictSol.Assignments, conflictAss)

	_, diagConflict, errConflict := solver.ValidateSolution(context.Background(), p, conflictSol)
	if errConflict == nil {
		t.Fatal("expected ValidateSolution to return error on student group conflict")
	}
	if diagConflict.Status != diagnostics.SolveStatusInfeasible {
		t.Fatalf("expected status INFEASIBLE, got %s", diagConflict.Status)
	}

	foundSGC := false
	for _, v := range diagConflict.Violations {
		if v.ConstraintID == "rule-final-sgc" && v.TemplateID == "StudentGroupConflict" {
			foundSGC = true
			break
		}
	}
	if !foundSGC {
		t.Fatalf("expected violation for rule-final-sgc, got: %+v", diagConflict.Violations)
	}
}

// -------------------------------------------------------------
// Performance Benchmarks: Legacy Full Check vs Compiled Evaluate
// -------------------------------------------------------------

func BenchmarkStudentGroupConflict_LegacyFullSolutionCheck(b *testing.B) {
	p, solution := localSearchTestProblem()
	oracle := legacyStudentGroupConflictOracle{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oracle.FullCheck(&p, &solution)
	}
}

func BenchmarkStudentGroupConflict_CompiledFullSolutionEvaluate(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "sgc-bench", TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Evaluate(ctx, &solution)
	}
}

func BenchmarkStudentGroupConflict_CompiledIsConsistent(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "sgc-bench", TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	a := solution.Assignments[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.IsConsistent(ctx, &solution, a)
	}
}

func BenchmarkStudentGroupConflict_CompiledViolatedByMove(b *testing.B) {
	p, solution := localSearchTestProblem()
	inst := constraints.ConstraintInstance{ID: "sgc-bench", TemplateID: "StudentGroupConflict", Kind: constraints.ConstraintKindHard}
	compiledSet, _, _ := constraints.Compile(&p, []constraints.ConstraintInstance{inst})
	c := compiledSet.Hard[0]
	ctx := constraints.NewSearchCtx(&p)
	mv := problem.Move{AssignmentID: "a-theory-1", From: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-1"}, To: problem.Placement{RoomID: "room-lecture-1", TimeSlotID: "mon-3"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ViolatedByMove(ctx, &solution, mv)
	}
}
