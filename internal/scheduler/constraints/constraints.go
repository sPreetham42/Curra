package constraints

import (
	"strings"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// Constraint checks whether a candidate assignment can be added to a partial solution.
type Constraint interface {
	Name() string
	Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation
}

// ScopedValidator checks resource-local hard constraints for an assignment at a slot.
type ScopedValidator interface {
	Constraint
	CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation
}

func DefaultHardConstraints() []Constraint {
	return []Constraint{
		FacultyConflict{},
		RoomConflict{},
		StudentGroupConflict{},
		RoomCapacity{},
		FacultyAvailability{},
		RoomAvailability{},
		RoomFeatureCompatibility{},
	}
}

func DefaultScopedValidators() []ScopedValidator {
	return []ScopedValidator{
		FacultyConflict{},
		RoomConflict{},
		StudentGroupConflict{},
		RoomCapacity{},
		FacultyAvailability{},
		RoomAvailability{},
		RoomFeatureCompatibility{},
	}
}

func CheckAll(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment, constraints []Constraint) []diagnostics.Violation {
	var violations []diagnostics.Violation
	for _, constraint := range constraints {
		violations = append(violations, constraint.Check(p, solution, assignment)...)
	}
	return violations
}

func baseViolation(name string, assignment problem.Assignment, message string, related map[string]string, metadata map[string]string) diagnostics.Violation {
	if related == nil {
		related = make(map[string]string)
	}
	related["courseOfferingId"] = string(assignment.CourseOfferingID)
	related["sessionRequirementId"] = string(assignment.SessionRequirementID)
	related["studentGroupId"] = string(assignment.StudentGroupID)
	related["timeSlotId"] = string(assignment.TimeSlotID)
	return diagnostics.Violation{
		ConstraintName: name,
		Severity:       diagnostics.SeverityHard,
		Message:        message,
		AssignmentID:   string(assignment.ID),
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}

func invalidDurationViolation(name string, assignment problem.Assignment) []diagnostics.Violation {
	return []diagnostics.Violation{baseViolation(name, assignment, "assignment does not fit in the recurring time-slot grid", nil, nil)}
}

func joinTimeSlotIDs(ids []model.TimeSlotID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ",")
}

func joinRoomFeatureIDs(ids []model.RoomFeatureID) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ",")
}
