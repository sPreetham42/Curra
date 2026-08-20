package constraints

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// RoomFeatureCompatibilityConstraint implements ConstraintDef and ScopedValidator for room feature compatibility.
type RoomFeatureCompatibilityConstraint struct {
	instance ConstraintInstance
}

type RoomFeatureCompatibility = RoomFeatureCompatibilityConstraint

// NewRoomFeatureCompatibilityConstraint creates a RoomFeatureCompatibilityConstraint from a ConstraintInstance.
func NewRoomFeatureCompatibilityConstraint(inst ConstraintInstance) RoomFeatureCompatibilityConstraint {
	if inst.ID == "" {
		inst.ID = "RoomFeatureCompatibility"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "RoomFeatureCompatibility"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return RoomFeatureCompatibilityConstraint{instance: inst}
}

func (c RoomFeatureCompatibilityConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "RoomFeatureCompatibility"
}

func (c RoomFeatureCompatibilityConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c RoomFeatureCompatibilityConstraint) Scope() string {
	return c.instance.Scope
}

func (c RoomFeatureCompatibilityConstraint) TemplateID() string {
	return "RoomFeatureCompatibility"
}

// Single internal semantic decision function for room feature compatibility.
func (c RoomFeatureCompatibilityConstraint) checkAssignment(p *problem.Problem, assignment problem.Assignment) []diagnostics.Violation {
	if p == nil {
		return nil
	}
	required := p.RequiredRoomFeatures(assignment.CourseOfferingID, assignment.SessionRequirementID)
	if len(required) == 0 {
		return nil
	}
	if !p.RoomHasFeatures(assignment.RoomID, required) {
		return []diagnostics.Violation{
			c.buildViolation(assignment, required),
		}
	}
	return nil
}

// IsConsistent delegates to the single semantic compatibility function.
func (c RoomFeatureCompatibilityConstraint) IsConsistent(ctx *SearchCtx, _ *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil {
		return true
	}
	return len(c.checkAssignment(p, candidate)) == 0
}

// ViolatedByMove delegates to the single semantic compatibility function.
func (c RoomFeatureCompatibilityConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
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
func (c RoomFeatureCompatibilityConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
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

func (c RoomFeatureCompatibilityConstraint) buildViolation(assignment problem.Assignment, required []model.RoomFeatureID) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	related := map[string]string{
		"roomId":               string(assignment.RoomID),
		"courseOfferingId":     string(assignment.CourseOfferingID),
		"sessionRequirementId": string(assignment.SessionRequirementID),
		"studentGroupId":       string(assignment.StudentGroupID),
		"timeSlotId":           string(assignment.TimeSlotID),
	}

	metadata := map[string]string{
		"requiredRoomFeatureIds": joinRoomFeatureIDs(required),
	}

	return diagnostics.Violation{
		ConstraintName: "RoomFeatureCompatibility",
		ConstraintID:   c.ID(),
		TemplateID:     "RoomFeatureCompatibility",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "room does not provide all required features",
		AssignmentID:   string(assignment.ID),
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:

func (c RoomFeatureCompatibilityConstraint) Name() string { return "RoomFeatureCompatibility" }

func (c RoomFeatureCompatibilityConstraint) Check(p *problem.Problem, _ *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	return c.checkAssignment(p, assignment)
}

func (c RoomFeatureCompatibilityConstraint) CheckAtSlot(p *problem.Problem, _ *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.checkAssignment(p, a)
}
