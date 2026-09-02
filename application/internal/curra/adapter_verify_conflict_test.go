package curra

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

// conflictProblemJSON returns a valid two-offering problem in the wire format the
// adapter consumes (CURRA model types carry no JSON tags, so field names are the Go
// struct field names).
func conflictProblemJSON(t *testing.T) json.RawMessage {
	t.Helper()

	p := map[string]any{
		"TenantID":      "tenant-conflict",
		"PeriodsPerDay": 2,
		"Term":          map[string]any{"ID": "term-1", "TenantID": "tenant-conflict", "Name": "Term 1"},
		"TimeSlots": map[string]any{
			"ts-1": map[string]any{"ID": "ts-1", "Day": 1, "Period": 1, "Label": "Mon P1"},
			"ts-2": map[string]any{"ID": "ts-2", "Day": 1, "Period": 2, "Label": "Mon P2"},
		},
		"Departments": map[string]any{
			"dept-1": map[string]any{"ID": "dept-1", "TenantID": "tenant-conflict", "Name": "Dept"},
		},
		"Programs": map[string]any{
			"prog-1": map[string]any{"ID": "prog-1", "DepartmentID": "dept-1", "Name": "Prog"},
		},
		"Classes": map[string]any{
			"class-1": map[string]any{"ID": "class-1", "ProgramID": "prog-1", "Name": "C1", "WholeGroupID": "sg-1", "StudentGroupIDs": []any{"sg-1"}},
			"class-2": map[string]any{"ID": "class-2", "ProgramID": "prog-1", "Name": "C2", "WholeGroupID": "sg-2", "StudentGroupIDs": []any{"sg-2"}},
		},
		"StudentGroups": map[string]any{
			"sg-1": map[string]any{"ID": "sg-1", "ClassID": "class-1", "Name": "G1", "Size": 20},
			"sg-2": map[string]any{"ID": "sg-2", "ClassID": "class-2", "Name": "G2", "Size": 20},
		},
		"Subjects": map[string]any{
			"subj-1": map[string]any{"ID": "subj-1", "Code": "S1", "Name": "Subject 1"},
			"subj-2": map[string]any{"ID": "subj-2", "Code": "S2", "Name": "Subject 2"},
		},
		"Faculty": map[string]any{
			"fac-1": map[string]any{"ID": "fac-1", "Name": "Faculty 1"},
			"fac-2": map[string]any{"ID": "fac-2", "Name": "Faculty 2"},
		},
		"Rooms": map[string]any{
			"room-1": map[string]any{"ID": "room-1", "Name": "Room 1", "Capacity": 50},
			"room-2": map[string]any{"ID": "room-2", "Name": "Room 2", "Capacity": 50},
		},
		"RoomFeatures": map[string]any{},
		"CourseOfferings": map[string]any{
			"off-1": map[string]any{
				"ID": "off-1", "TermID": "term-1", "ClassID": "class-1", "SubjectID": "subj-1",
				"StudentGroupID": "sg-1", "FacultyID": "fac-1",
				"RequiredRoomFeatureIDs": []any{}, "SessionRequirementIDs": []any{"req-1"},
			},
			"off-2": map[string]any{
				"ID": "off-2", "TermID": "term-1", "ClassID": "class-2", "SubjectID": "subj-2",
				"StudentGroupID": "sg-2", "FacultyID": "fac-2",
				"RequiredRoomFeatureIDs": []any{}, "SessionRequirementIDs": []any{"req-2"},
			},
		},
		"SessionRequirements": map[string]any{
			"req-1": map[string]any{"ID": "req-1", "CourseOfferingID": "off-1", "Type": "THEORY", "SessionsPerWeek": 1, "Duration": 1},
			"req-2": map[string]any{"ID": "req-2", "CourseOfferingID": "off-2", "Type": "THEORY", "SessionsPerWeek": 1, "Duration": 1},
		},
		"FacultyAvailabilities": []any{
			map[string]any{"FacultyID": "fac-1", "TimeSlotID": "ts-1"},
			map[string]any{"FacultyID": "fac-1", "TimeSlotID": "ts-2"},
			map[string]any{"FacultyID": "fac-2", "TimeSlotID": "ts-1"},
			map[string]any{"FacultyID": "fac-2", "TimeSlotID": "ts-2"},
		},
		"RoomAvailabilities": []any{
			map[string]any{"RoomID": "room-1", "TimeSlotID": "ts-1"},
			map[string]any{"RoomID": "room-1", "TimeSlotID": "ts-2"},
			map[string]any{"RoomID": "room-2", "TimeSlotID": "ts-1"},
			map[string]any{"RoomID": "room-2", "TimeSlotID": "ts-2"},
		},
		"FacultyPreferences": []any{},
		"LockedAssignments":  []any{},
	}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal problem: %v", err)
	}
	return raw
}

func assignmentJSON(id, offering, group, faculty, room, slot, requirement string) map[string]any {
	return map[string]any{
		"id":                   id,
		"courseOfferingId":     offering,
		"studentGroupId":       group,
		"facultyId":            faculty,
		"roomId":               room,
		"timeSlotId":           slot,
		"sessionRequirementId": requirement,
		"instance":             0,
	}
}

// TestAdapter_Verify_RejectsJSONDeserializedConflicts exercises the production
// verification path end to end. Solution.Index is `json:"-"`, so every solution the
// adapter reconstructs from stored JSON arrives with no index at all; authoritative
// verification must still reject each resource conflict on the raw assignments.
func TestAdapter_Verify_RejectsJSONDeserializedConflicts(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	adapter := New(logger)
	problemJSON := conflictProblemJSON(t)

	cases := []struct {
		name        string
		assignments []map[string]any
		constraint  string
	}{
		{
			name: "faculty double booking",
			assignments: []map[string]any{
				assignmentJSON("a1", "off-1", "sg-1", "fac-1", "room-1", "ts-1", "req-1"),
				// fac-1 teaching off-2 as well: same faculty, same slot, different room and group.
				assignmentJSON("a2", "off-2", "sg-2", "fac-1", "room-2", "ts-1", "req-2"),
			},
			constraint: "FacultyConflict",
		},
		{
			name: "room double booking",
			assignments: []map[string]any{
				assignmentJSON("a1", "off-1", "sg-1", "fac-1", "room-1", "ts-1", "req-1"),
				assignmentJSON("a2", "off-2", "sg-2", "fac-2", "room-1", "ts-1", "req-2"),
			},
			constraint: "RoomConflict",
		},
		{
			name: "student group double booking",
			assignments: []map[string]any{
				assignmentJSON("a1", "off-1", "sg-1", "fac-1", "room-1", "ts-1", "req-1"),
				// sg-1 attending off-2 as well: same group, same slot, different faculty and room.
				assignmentJSON("a2", "off-2", "sg-1", "fac-2", "room-2", "ts-1", "req-2"),
			},
			constraint: "StudentGroupConflict",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			solutionJSON, err := json.Marshal(map[string]any{
				"assignments": tc.assignments,
				"score":       map[string]any{"hardViolations": 0, "softPenalty": 0},
			})
			if err != nil {
				t.Fatalf("marshal solution: %v", err)
			}

			resp, err := adapter.Verify(context.Background(), VerifyRequest{
				ProblemJSON:  problemJSON,
				SolutionJSON: solutionJSON,
			})
			if err == nil {
				t.Fatalf("expected verification error for %s, got nil (resp=%+v)", tc.name, resp)
			}
			if resp.Valid {
				t.Fatalf("%s verified as valid through the adapter", tc.name)
			}

			found := false
			for _, v := range resp.Violations {
				if v.ConstraintName == tc.constraint {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected a %s violation, got %+v", tc.constraint, resp.Violations)
			}
		})
	}
}

// TestAdapter_Verify_AcceptsConflictFreeJSONSolution guards the same path against
// false positives.
func TestAdapter_Verify_AcceptsConflictFreeJSONSolution(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	adapter := New(logger)

	solutionJSON, err := json.Marshal(map[string]any{
		"assignments": []map[string]any{
			assignmentJSON("a1", "off-1", "sg-1", "fac-1", "room-1", "ts-1", "req-1"),
			assignmentJSON("a2", "off-2", "sg-2", "fac-2", "room-2", "ts-2", "req-2"),
		},
		"score": map[string]any{"hardViolations": 0, "softPenalty": 0},
	})
	if err != nil {
		t.Fatalf("marshal solution: %v", err)
	}

	resp, err := adapter.Verify(context.Background(), VerifyRequest{
		ProblemJSON:  conflictProblemJSON(t),
		SolutionJSON: solutionJSON,
	})
	if err != nil {
		t.Fatalf("conflict-free solution failed adapter verification: %v (violations: %+v)", err, resp.Violations)
	}
	if !resp.Valid {
		t.Fatalf("conflict-free solution reported invalid: %+v", resp)
	}
}
