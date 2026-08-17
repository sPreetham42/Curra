package tests

import (
	"context"
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
