package constraints

import (
	"fmt"
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

type RoomConflict struct{}

func (RoomConflict) Name() string { return "RoomConflict" }

func (c RoomConflict) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return invalidDurationViolation(c.Name(), assignment)
	}
	if conflictingID, ok := solution.Index.RoomConflict(assignment.RoomID, slotIDs); ok && conflictingID != assignment.ID {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "room is already scheduled in an occupied time slot", map[string]string{
			"roomId":                  string(assignment.RoomID),
			"conflictingAssignmentId": string(conflictingID),
		}, nil)}
	}
	return nil
}

func (c RoomConflict) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}

type StudentGroupConflict struct{}

func (StudentGroupConflict) Name() string { return "StudentGroupConflict" }

func (c StudentGroupConflict) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return invalidDurationViolation(c.Name(), assignment)
	}
	for _, groupID := range p.OverlappingStudentGroupIDs(assignment.StudentGroupID) {
		if conflictingID, ok := solution.Index.StudentGroupConflict(groupID, slotIDs); ok && conflictingID != assignment.ID {
			return []diagnostics.Violation{baseViolation(c.Name(), assignment, "student group overlaps another scheduled group in an occupied time slot", map[string]string{
				"studentGroupId":          string(assignment.StudentGroupID),
				"overlappingGroupId":      string(groupID),
				"conflictingAssignmentId": string(conflictingID),
			}, nil)}
		}
	}
	return nil
}

func (c StudentGroupConflict) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}

type RoomCapacity struct{}

func (RoomCapacity) Name() string { return "RoomCapacity" }

func (c RoomCapacity) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	room, ok := p.Room(assignment.RoomID)
	if !ok {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "room does not exist", map[string]string{
			"roomId": string(assignment.RoomID),
		}, nil)}
	}
	groupSize := p.StudentGroupSize(assignment.StudentGroupID)
	if room.Capacity < groupSize {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "room capacity is below student group size", map[string]string{
			"roomId":         string(room.ID),
			"studentGroupId": string(assignment.StudentGroupID),
		}, map[string]string{
			"roomCapacity":     fmt.Sprintf("%d", room.Capacity),
			"studentGroupSize": fmt.Sprintf("%d", groupSize),
		})}
	}
	return nil
}

func (c RoomCapacity) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}

type FacultyAvailability struct{}

func (FacultyAvailability) Name() string { return "FacultyAvailability" }

func (c FacultyAvailability) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return invalidDurationViolation(c.Name(), assignment)
	}
	if !p.IsFacultyAvailable(assignment.FacultyID, slotIDs) {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "faculty is not available for all occupied time slots", map[string]string{
			"facultyId": string(assignment.FacultyID),
		}, map[string]string{
			"timeSlotIds": joinTimeSlotIDs(slotIDs),
		})}
	}
	return nil
}

func (c FacultyAvailability) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}

type RoomAvailability struct{}

func (RoomAvailability) Name() string { return "RoomAvailability" }

func (c RoomAvailability) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return invalidDurationViolation(c.Name(), assignment)
	}
	if !p.IsRoomAvailable(assignment.RoomID, slotIDs) {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "room is not available for all occupied time slots", map[string]string{
			"roomId": string(assignment.RoomID),
		}, map[string]string{
			"timeSlotIds": joinTimeSlotIDs(slotIDs),
		})}
	}
	return nil
}

func (c RoomAvailability) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}

type RoomFeatureCompatibility struct{}

func (RoomFeatureCompatibility) Name() string { return "RoomFeatureCompatibility" }

func (c RoomFeatureCompatibility) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	required := p.RequiredRoomFeatures(assignment.CourseOfferingID, assignment.SessionRequirementID)
	if len(required) == 0 {
		return nil
	}
	if !p.RoomHasFeatures(assignment.RoomID, required) {
		return []diagnostics.Violation{baseViolation(c.Name(), assignment, "room does not provide all required features", map[string]string{
			"roomId":               string(assignment.RoomID),
			"courseOfferingId":     string(assignment.CourseOfferingID),
			"sessionRequirementId": string(assignment.SessionRequirementID),
		}, map[string]string{
			"requiredRoomFeatureIds": joinRoomFeatureIDs(required),
		})}
	}
	return nil
}

func (c RoomFeatureCompatibility) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
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
