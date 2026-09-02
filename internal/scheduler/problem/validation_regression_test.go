package problem_test

import (
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
)

// requireCleanBaseline asserts the fixture used by the regression tests below is
// itself valid, so any violation observed after a mutation is attributable to it.
func requireCleanBaseline(t *testing.T, p problem.Problem) {
	t.Helper()
	if violations := problem.Validate(p); len(violations) != 0 {
		t.Fatalf("baseline fixture must validate clean, got %d violations: %+v", len(violations), violations)
	}
}

// ---------------------------------------------------------------------------
// Bidirectional course offering <-> session requirement membership
// ---------------------------------------------------------------------------

// TestValidate_OrphanSessionRequirement_Rejected proves that a session requirement
// pointing at an existing offering that does not list it back is rejected as an
// invalid problem instead of reaching the solver.
func TestValidate_OrphanSessionRequirement_Rejected(t *testing.T) {
	p := testutil.FeasibleProblem()
	requireCleanBaseline(t, p)

	// req-orphan references offering-theory, but offering-theory only lists req-theory.
	p.SessionRequirements["req-orphan"] = model.SessionRequirement{
		ID:               "req-orphan",
		CourseOfferingID: "offering-theory",
		Type:             model.SessionTypeTheory,
		SessionsPerWeek:  1,
		Duration:         1,
	}

	violations := problem.Validate(p)
	if !testutil.HasViolationMessageContaining(violations, "session requirement is not listed on its course offering") {
		t.Fatalf("expected orphan session requirement violation, got %+v", violations)
	}
}

// TestValidate_SessionRequirementListedOnOffering_Accepted guards the reverse
// membership check against false positives on a well-formed problem.
func TestValidate_SessionRequirementListedOnOffering_Accepted(t *testing.T) {
	p := testutil.FeasibleProblem()

	offering := p.CourseOfferings["offering-theory"]
	offering.SessionRequirementIDs = append(offering.SessionRequirementIDs, "req-theory-extra")
	p.CourseOfferings["offering-theory"] = offering
	p.SessionRequirements["req-theory-extra"] = model.SessionRequirement{
		ID:               "req-theory-extra",
		CourseOfferingID: "offering-theory",
		Type:             model.SessionTypeTheory,
		SessionsPerWeek:  1,
		Duration:         1,
	}

	if violations := problem.Validate(p); len(violations) != 0 {
		t.Fatalf("expected no violations for a properly linked requirement, got %+v", violations)
	}
}

// ---------------------------------------------------------------------------
// Time slot grid bounds
// ---------------------------------------------------------------------------

// TestValidate_TimeSlotPeriodBeyondGrid_Rejected proves a slot outside the
// configured period grid is rejected. Such a slot is unreachable through
// SlotsByDayPeriod expansion and is silently dropped by the scorers.
func TestValidate_TimeSlotPeriodBeyondGrid_Rejected(t *testing.T) {
	p := testutil.FeasibleProblem()
	requireCleanBaseline(t, p)

	if p.PeriodsPerDay != 3 {
		t.Fatalf("fixture precondition changed: PeriodsPerDay = %d, want 3", p.PeriodsPerDay)
	}
	p.TimeSlots["mon-4"] = model.TimeSlot{ID: "mon-4", Day: time.Monday, Period: 4, Label: "Mon P4"}

	violations := problem.Validate(p)
	if !testutil.HasViolationMessageContaining(violations, "time slot period exceeds periods per day") {
		t.Fatalf("expected out-of-grid time slot violation, got %+v", violations)
	}
}

// TestValidate_TimeSlotPeriodAtGridBoundary_Accepted proves the bound is inclusive
// and that the pre-existing lower-bound semantics are preserved.
func TestValidate_TimeSlotPeriodAtGridBoundary_Accepted(t *testing.T) {
	p := testutil.FeasibleProblem()
	p.TimeSlots["tue-3"] = model.TimeSlot{ID: "tue-3", Day: time.Tuesday, Period: p.PeriodsPerDay, Label: "Tue last"}

	if violations := problem.Validate(p); len(violations) != 0 {
		t.Fatalf("expected slot at the grid boundary to validate clean, got %+v", violations)
	}
}

func TestValidate_TimeSlotNonPositivePeriod_StillRejected(t *testing.T) {
	p := testutil.FeasibleProblem()
	p.TimeSlots["bad-0"] = model.TimeSlot{ID: "bad-0", Day: time.Wednesday, Period: 0, Label: "Bad"}

	violations := problem.Validate(p)
	if !testutil.HasViolationMessageContaining(violations, "time slot has non-positive period") {
		t.Fatalf("expected non-positive period violation, got %+v", violations)
	}
	if testutil.HasViolationMessageContaining(violations, "time slot period exceeds periods per day") {
		t.Fatal("non-positive period must not also report an upper-bound violation")
	}
}

// ---------------------------------------------------------------------------
// Faculty preference weight semantics
// ---------------------------------------------------------------------------

// TestValidate_NegativeFacultyPreferenceWeight_Rejected proves the non-negative
// penalty contract for FacultyPreference.Weight is enforced at validation time.
func TestValidate_NegativeFacultyPreferenceWeight_Rejected(t *testing.T) {
	p := testutil.FeasibleProblem()
	requireCleanBaseline(t, p)

	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "faculty-a", TimeSlotID: "mon-1", Weight: -5},
	}

	violations := problem.Validate(p)
	if !testutil.HasViolationMessageContaining(violations, "faculty preference has negative weight") {
		t.Fatalf("expected negative preference weight violation, got %+v", violations)
	}
}

func TestValidate_NonNegativeFacultyPreferenceWeights_Accepted(t *testing.T) {
	p := testutil.FeasibleProblem()
	p.FacultyPreferences = []model.FacultyPreference{
		{FacultyID: "faculty-a", TimeSlotID: "mon-1", Weight: 0},
		{FacultyID: "faculty-a", TimeSlotID: "mon-2", Weight: 7},
	}

	if violations := problem.Validate(p); len(violations) != 0 {
		t.Fatalf("expected zero and positive preference weights to validate clean, got %+v", violations)
	}
}

// ---------------------------------------------------------------------------
// Catalog map key / entity ID consistency
// ---------------------------------------------------------------------------

// TestValidate_CatalogMapKeyMismatch_Rejected proves that every ID-keyed catalog
// rejects an entity filed under a key other than its own ID. Each lookup in the
// engine treats the map key as the entity's identity, so a mismatch makes a
// non-existent ID resolvable and the real ID unresolvable.
func TestValidate_CatalogMapKeyMismatch_Rejected(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(p *problem.Problem)
		message string
	}{
		{
			name: "faculty",
			mutate: func(p *problem.Problem) {
				p.Faculty["faculty-mismatch"] = model.Faculty{ID: "faculty-other", Name: "Mismatch"}
			},
			message: "faculty map key does not match entity ID",
		},
		{
			name: "room",
			mutate: func(p *problem.Problem) {
				p.Rooms["room-mismatch"] = model.Room{ID: "room-other", Name: "Mismatch", Capacity: 10}
			},
			message: "room map key does not match entity ID",
		},
		{
			name: "time slot",
			mutate: func(p *problem.Problem) {
				p.TimeSlots["tue-3"] = model.TimeSlot{ID: "slot-other", Day: time.Tuesday, Period: 3, Label: "Mismatch"}
			},
			message: "time slot map key does not match entity ID",
		},
		{
			name: "student group",
			mutate: func(p *problem.Problem) {
				group := p.StudentGroups["group-a"]
				group.ID = "group-other"
				p.StudentGroups["group-a"] = group
			},
			message: "student group map key does not match entity ID",
		},
		{
			name: "class",
			mutate: func(p *problem.Problem) {
				class := p.Classes["class-a"]
				class.ID = "class-other"
				p.Classes["class-a"] = class
			},
			message: "class map key does not match entity ID",
		},
		{
			name: "course offering",
			mutate: func(p *problem.Problem) {
				offering := p.CourseOfferings["offering-theory"]
				offering.ID = "offering-other"
				p.CourseOfferings["offering-theory"] = offering
			},
			message: "course offering map key does not match entity ID",
		},
		{
			name: "session requirement",
			mutate: func(p *problem.Problem) {
				req := p.SessionRequirements["req-theory"]
				req.ID = "req-other"
				p.SessionRequirements["req-theory"] = req
			},
			message: "session requirement map key does not match entity ID",
		},
		{
			name: "subject",
			mutate: func(p *problem.Problem) {
				subject := p.Subjects["subject-theory"]
				subject.ID = "subject-other"
				p.Subjects["subject-theory"] = subject
			},
			message: "subject map key does not match entity ID",
		},
		{
			name: "program",
			mutate: func(p *problem.Problem) {
				prog := p.Programs["program-a"]
				prog.ID = "program-other"
				p.Programs["program-a"] = prog
			},
			message: "program map key does not match entity ID",
		},
		{
			name: "department",
			mutate: func(p *problem.Problem) {
				dept := p.Departments["dept-a"]
				dept.ID = "dept-other"
				p.Departments["dept-a"] = dept
			},
			message: "department map key does not match entity ID",
		},
		{
			name: "room feature",
			mutate: func(p *problem.Problem) {
				feature := p.RoomFeatures["feature-lab"]
				feature.ID = "feature-other"
				p.RoomFeatures["feature-lab"] = feature
			},
			message: "room feature map key does not match entity ID",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.FeasibleProblem()
			requireCleanBaseline(t, p)

			tc.mutate(&p)

			violations := problem.Validate(p)
			if !testutil.HasViolationMessageContaining(violations, tc.message) {
				t.Fatalf("expected %q, got %+v", tc.message, violations)
			}
		})
	}
}

// TestValidate_EmptyEntityID_ReportsOnlyEmptyIDViolation preserves the pre-existing
// empty-ID message and keeps the two checks mutually exclusive.
func TestValidate_EmptyEntityID_ReportsOnlyEmptyIDViolation(t *testing.T) {
	p := testutil.FeasibleProblem()
	p.Faculty["faculty-empty"] = model.Faculty{ID: "", Name: "No ID"}

	violations := problem.Validate(p)
	if !testutil.HasViolationMessageContaining(violations, "faculty has empty ID") {
		t.Fatalf("expected empty faculty ID violation, got %+v", violations)
	}
	if testutil.HasViolationMessageContaining(violations, "faculty map key does not match entity ID") {
		t.Fatal("empty entity ID must not additionally report a key mismatch")
	}
}
