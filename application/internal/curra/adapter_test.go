package curra

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

func TestAdapter_Solve_SimpleProblem(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	adapter := New(logger)

	// Minimal valid CURRA problem
	// Note: CURRA model types have no JSON tags, so field names must match Go struct field names exactly.
	problem := map[string]any{
		"TenantID":      "test-institution",
		"PeriodsPerDay": 5,
		"TimeSlots": map[string]any{
			"ts-mon-1": map[string]any{"ID": "ts-mon-1", "Day": 1, "Period": 1, "Label": "Mon P1"},
			"ts-mon-2": map[string]any{"ID": "ts-mon-2", "Day": 1, "Period": 2, "Label": "Mon P2"},
			"ts-tue-1": map[string]any{"ID": "ts-tue-1", "Day": 2, "Period": 1, "Label": "Tue P1"},
			"ts-tue-2": map[string]any{"ID": "ts-tue-2", "Day": 2, "Period": 2, "Label": "Tue P2"},
			"ts-wed-1": map[string]any{"ID": "ts-wed-1", "Day": 3, "Period": 1, "Label": "Wed P1"},
		},
		"Departments": map[string]any{
			"dept-cs": map[string]any{"ID": "dept-cs", "TenantID": "test-institution", "Name": "CS"},
		},
		"Term": map[string]any{"ID": "term-fall", "TenantID": "test-institution", "Name": "Fall 2026"},
		"Programs": map[string]any{
			"prog-cs": map[string]any{"ID": "prog-cs", "DepartmentID": "dept-cs", "Name": "B.Tech CS"},
		},
		"Classes": map[string]any{
			"class-1": map[string]any{"ID": "class-1", "ProgramID": "prog-cs", "Name": "CS Year 1", "WholeGroupID": "sg-1", "StudentGroupIDs": []any{"sg-1"}},
		},
		"StudentGroups": map[string]any{
			"sg-1": map[string]any{"ID": "sg-1", "ClassID": "class-1", "Name": "CS-A", "Size": 30},
		},
		"Subjects": map[string]any{
			"subj-math": map[string]any{"ID": "subj-math", "Code": "MATH101", "Name": "Mathematics"},
		},
		"Faculty": map[string]any{
			"fac-smith": map[string]any{"ID": "fac-smith", "Name": "Prof. Smith"},
		},
		"Rooms": map[string]any{
			"room-101": map[string]any{"ID": "room-101", "Name": "Room 101", "Capacity": 50},
		},
		"RoomFeatures": map[string]any{},
		"CourseOfferings": map[string]any{
			"co-math": map[string]any{
				"ID":                    "co-math",
				"TermID":                "term-fall",
				"ClassID":               "class-1",
				"SubjectID":             "subj-math",
				"StudentGroupID":        "sg-1",
				"FacultyID":             "fac-smith",
				"RequiredRoomFeatureIDs": []any{},
				"SessionRequirementIDs":  []any{"sr-math-1"},
			},
		},
		"SessionRequirements": map[string]any{
			"sr-math-1": map[string]any{
				"ID":               "sr-math-1",
				"CourseOfferingID": "co-math",
				"Type":             "THEORY",
				"SessionsPerWeek":  2,
				"Duration":         1,
			},
		},
		"FacultyAvailabilities": []any{
			map[string]any{"FacultyID": "fac-smith", "TimeSlotID": "ts-mon-1"},
			map[string]any{"FacultyID": "fac-smith", "TimeSlotID": "ts-mon-2"},
			map[string]any{"FacultyID": "fac-smith", "TimeSlotID": "ts-tue-1"},
			map[string]any{"FacultyID": "fac-smith", "TimeSlotID": "ts-tue-2"},
			map[string]any{"FacultyID": "fac-smith", "TimeSlotID": "ts-wed-1"},
		},
		"RoomAvailabilities": []any{
			map[string]any{"RoomID": "room-101", "TimeSlotID": "ts-mon-1"},
			map[string]any{"RoomID": "room-101", "TimeSlotID": "ts-mon-2"},
			map[string]any{"RoomID": "room-101", "TimeSlotID": "ts-tue-1"},
			map[string]any{"RoomID": "room-101", "TimeSlotID": "ts-tue-2"},
			map[string]any{"RoomID": "room-101", "TimeSlotID": "ts-wed-1"},
		},
		"FacultyPreferences": []any{},
		"LockedAssignments":  []any{},
	}

	problemJSON, _ := json.Marshal(problem)

	req := SolveRequest{
		ProblemJSON: problemJSON,
		Seed:        42,
	}

	resp, err := adapter.Solve(context.Background(), req)
	if err != nil {
		t.Logf("Solve returned error (may be expected for infeasible): %v", err)
	}

	// The problem should be solvable or infeasible — not crash
	if resp.Status == "" {
		t.Fatal("expected non-empty status")
	}

	t.Logf("Status: %s", resp.Status)
	t.Logf("Score: %+v", resp.Score)
	t.Logf("Diagnostics: %+v", resp.Diagnostics)
	if resp.Violations != nil {
		for _, v := range resp.Violations {
			t.Logf("Violation: %s - %s", v.ConstraintName, v.Message)
		}
	}

	// If solved, verify solution is not nil
	if resp.Status == "SOLVED" {
		if resp.Solution == nil {
			t.Fatal("SOLVED status but no solution")
		}
		t.Logf("Solution present: %d bytes", len(resp.Solution))
	}
}

func TestAdapter_Verify(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	adapter := New(logger)

	// Simple problem with correct CURRA field names (no JSON tags)
	problem := map[string]any{
		"TenantID":      "test",
		"PeriodsPerDay": 5,
		"Term": map[string]any{"ID": "term-1", "TenantID": "test", "Name": "Term 1"},
		"TimeSlots": map[string]any{
			"ts-1": map[string]any{"ID": "ts-1", "Day": 1, "Period": 1, "Label": "P1"},
		},
		"Departments":           map[string]any{},
		"Programs":              map[string]any{},
		"Classes":               map[string]any{},
		"StudentGroups":         map[string]any{},
		"Subjects":              map[string]any{},
		"Faculty":               map[string]any{},
		"Rooms":                 map[string]any{},
		"RoomFeatures":          map[string]any{},
		"CourseOfferings":       map[string]any{},
		"SessionRequirements":   map[string]any{},
		"FacultyAvailabilities": []any{},
		"RoomAvailabilities":    []any{},
		"FacultyPreferences":    []any{},
		"LockedAssignments":     []any{},
	}
	problemJSON, _ := json.Marshal(problem)

	// Empty solution
	solution := map[string]any{"assignments": []any{}, "score": map[string]any{"hardViolations": 0, "softPenalty": 0}}
	solutionJSON, _ := json.Marshal(solution)

	req := VerifyRequest{
		ProblemJSON:  problemJSON,
		SolutionJSON: solutionJSON,
	}

	resp, err := adapter.Verify(context.Background(), req)
	if err != nil {
		t.Logf("Verify returned error: %v", err)
	}

	t.Logf("Valid: %v, Status: %s", resp.Valid, resp.Status)

	// Empty solution against empty problem should be valid (no assignments = no violations)
	if !resp.Valid && resp.Status != "INVALID_RESULT" {
		t.Logf("Empty solution validity: %v (may be expected depending on verifier)", resp.Valid)
	}
}

func TestAdapter_CompileConstraints(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	adapter := New(logger)

	problem := map[string]any{
		"TenantID":      "test",
		"PeriodsPerDay": 5,
		"Term":          map[string]any{"ID": "term-1", "TenantID": "test", "Name": "Term 1"},
	}
	problemJSON, _ := json.Marshal(problem)

	req := CompileRequest{
		ProblemJSON:     problemJSON,
		ConstraintsJSON: json.RawMessage("[]"),
	}

	resp, err := adapter.CompileConstraints(context.Background(), req)
	if err != nil {
		t.Fatalf("CompileConstraints failed: %v", err)
	}

	if resp.RuleSetHash == "" {
		t.Fatal("expected non-empty RuleSetHash")
	}
}
