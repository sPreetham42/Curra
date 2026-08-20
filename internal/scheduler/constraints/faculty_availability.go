package constraints

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// FacultyAvailabilityConstraint implements ConstraintDef and ScopedValidator for faculty availability.
type FacultyAvailabilityConstraint struct {
	instance ConstraintInstance
}

type FacultyAvailability = FacultyAvailabilityConstraint

// NewFacultyAvailabilityConstraint creates a FacultyAvailabilityConstraint from a ConstraintInstance.
func NewFacultyAvailabilityConstraint(inst ConstraintInstance) FacultyAvailabilityConstraint {
	if inst.ID == "" {
		inst.ID = "FacultyAvailability"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "FacultyAvailability"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return FacultyAvailabilityConstraint{instance: inst}
}

func (c FacultyAvailabilityConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "FacultyAvailability"
}

func (c FacultyAvailabilityConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c FacultyAvailabilityConstraint) Scope() string {
	return c.instance.Scope
}

func (c FacultyAvailabilityConstraint) TemplateID() string {
	return "FacultyAvailability"
}

// Single internal semantic decision function for faculty availability.
func (c FacultyAvailabilityConstraint) checkAssignment(p *problem.Problem, assignment problem.Assignment) []diagnostics.Violation {
	if p == nil {
		return nil
	}
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return []diagnostics.Violation{
			c.buildInvalidDurationViolation(assignment),
		}
	}
	if !p.IsFacultyAvailable(assignment.FacultyID, slotIDs) {
		return []diagnostics.Violation{
			c.buildUnavailableViolation(assignment, slotIDs),
		}
	}
	return nil
}

// IsConsistent delegates to the single semantic compatibility function.
func (c FacultyAvailabilityConstraint) IsConsistent(ctx *SearchCtx, _ *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil {
		return true
	}
	return len(c.checkAssignment(p, candidate)) == 0
}

// ViolatedByMove delegates to the single semantic compatibility function.
func (c FacultyAvailabilityConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	assignment, ok := sol.Index.AssignmentByID(mv.AssignmentID)
	if !ok {
		return nil
	}

	candidate := assignment
	candidate.RoomID = mv.To.RoomID
	candidate.TimeSlotID = mv.To.TimeSlotID

	return c.checkAssignment(p, candidate)
}

// Evaluate delegates to the single semantic compatibility function across all solution assignments.
func (c FacultyAvailabilityConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		if v := c.checkAssignment(p, a); len(v) > 0 {
			violations = append(violations, v...)
		}
	}
	return violations
}

func (c FacultyAvailabilityConstraint) buildUnavailableViolation(assignment problem.Assignment, slotIDs []model.TimeSlotID) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	related := map[string]string{
		"facultyId":            string(assignment.FacultyID),
		"courseOfferingId":     string(assignment.CourseOfferingID),
		"sessionRequirementId": string(assignment.SessionRequirementID),
		"studentGroupId":       string(assignment.StudentGroupID),
		"timeSlotId":           string(assignment.TimeSlotID),
	}

	metadata := map[string]string{
		"timeSlotIds": joinTimeSlotIDs(slotIDs),
	}

	return diagnostics.Violation{
		ConstraintName: "FacultyAvailability",
		ConstraintID:   c.ID(),
		TemplateID:     "FacultyAvailability",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "faculty is not available for all occupied time slots",
		AssignmentID:   string(assignment.ID),
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}

func (c FacultyAvailabilityConstraint) buildInvalidDurationViolation(assignment problem.Assignment) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	related := map[string]string{
		"courseOfferingId":     string(assignment.CourseOfferingID),
		"sessionRequirementId": string(assignment.SessionRequirementID),
		"studentGroupId":       string(assignment.StudentGroupID),
		"timeSlotId":           string(assignment.TimeSlotID),
	}

	return diagnostics.Violation{
		ConstraintName: "FacultyAvailability",
		ConstraintID:   c.ID(),
		TemplateID:     "FacultyAvailability",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "assignment does not fit in the recurring time-slot grid",
		AssignmentID:   string(assignment.ID),
		RelatedIDs:     related,
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:

func (c FacultyAvailabilityConstraint) Name() string { return "FacultyAvailability" }

func (c FacultyAvailabilityConstraint) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	return c.checkAssignment(p, assignment)
}

func (c FacultyAvailabilityConstraint) CheckAtSlot(p *problem.Problem, _ *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.checkAssignment(p, a)
}
