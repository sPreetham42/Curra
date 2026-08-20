package constraints

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// StudentGroupConflictConstraint implements ConstraintDef and ScopedValidator for student group conflicts.
type StudentGroupConflictConstraint struct {
	instance ConstraintInstance
}

type StudentGroupConflict = StudentGroupConflictConstraint

// NewStudentGroupConflictConstraint creates a StudentGroupConflictConstraint from a ConstraintInstance.
func NewStudentGroupConflictConstraint(inst ConstraintInstance) StudentGroupConflictConstraint {
	if inst.ID == "" {
		inst.ID = "StudentGroupConflict"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "StudentGroupConflict"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return StudentGroupConflictConstraint{instance: inst}
}

func (c StudentGroupConflictConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "StudentGroupConflict"
}

func (c StudentGroupConflictConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c StudentGroupConflictConstraint) Scope() string {
	return c.instance.Scope
}

func (c StudentGroupConflictConstraint) TemplateID() string {
	return "StudentGroupConflict"
}

// Single internal semantic decision function for student group conflict detection.
func (c StudentGroupConflictConstraint) checkAssignment(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) (problem.AssignmentID, model.StudentGroupID, bool) {
	if p == nil || solution == nil {
		return "", "", false
	}
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return "", "", false
	}
	for _, groupID := range p.OverlappingStudentGroupIDs(assignment.StudentGroupID) {
		if conflictingID, ok := solution.Index.StudentGroupConflict(groupID, slotIDs); ok && conflictingID != assignment.ID {
			return conflictingID, groupID, true
		}
	}
	return "", "", false
}

// IsConsistent checks if candidate assignment is consistent with partial solution.
func (c StudentGroupConflictConstraint) IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil || partial == nil {
		return true
	}
	if _, ok := candidate.OccupiedSlotIDs(p); !ok {
		return false
	}
	_, _, hasConflict := c.checkAssignment(p, partial, candidate)
	return !hasConflict
}

// ViolatedByMove checks if applying move causes a student group conflict.
func (c StudentGroupConflictConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
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

	slotIDs, ok := candidate.OccupiedSlotIDs(p)
	if !ok {
		return nil
	}

	for _, groupID := range p.OverlappingStudentGroupIDs(candidate.StudentGroupID) {
		if conflictingID, ok := sol.Index.StudentGroupConflict(groupID, slotIDs); ok && conflictingID != assignment.ID {
			return []diagnostics.Violation{
				c.buildViolation(candidate, conflictingID, groupID),
			}
		}
	}
	return nil
}

// Evaluate checks the full solution for student group conflicts.
func (c StudentGroupConflictConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	var violations []diagnostics.Violation
	seenPairs := make(map[string]bool)

	for _, a := range sol.Assignments {
		if conflictingID, groupID, hasConflict := c.checkAssignment(p, sol, a); hasConflict {
			pairKey := string(a.ID) + "|" + string(conflictingID)
			altKey := string(conflictingID) + "|" + string(a.ID)
			if !seenPairs[pairKey] && !seenPairs[altKey] {
				seenPairs[pairKey] = true
				violations = append(violations, c.buildViolation(a, conflictingID, groupID))
			}
		}
	}
	return violations
}

func (c StudentGroupConflictConstraint) buildViolation(assignment problem.Assignment, conflictingID problem.AssignmentID, overlappingGroupID model.StudentGroupID) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	return diagnostics.Violation{
		ConstraintName: "StudentGroupConflict",
		ConstraintID:   c.ID(),
		TemplateID:     "StudentGroupConflict",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "student group overlaps another scheduled group in an occupied time slot",
		AssignmentID:   string(assignment.ID),
		RelatedIDs: map[string]string{
			"studentGroupId":          string(assignment.StudentGroupID),
			"overlappingGroupId":      string(overlappingGroupID),
			"conflictingAssignmentId": string(conflictingID),
			"courseOfferingId":        string(assignment.CourseOfferingID),
			"sessionRequirementId":    string(assignment.SessionRequirementID),
			"timeSlotId":              string(assignment.TimeSlotID),
		},
	}
}

func (c StudentGroupConflictConstraint) buildInvalidDurationViolation(assignment problem.Assignment) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	return diagnostics.Violation{
		ConstraintName: "StudentGroupConflict",
		ConstraintID:   c.ID(),
		TemplateID:     "StudentGroupConflict",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "assignment does not fit in the recurring time-slot grid",
		AssignmentID:   string(assignment.ID),
		RelatedIDs: map[string]string{
			"courseOfferingId":     string(assignment.CourseOfferingID),
			"sessionRequirementId": string(assignment.SessionRequirementID),
			"studentGroupId":       string(assignment.StudentGroupID),
			"timeSlotId":           string(assignment.TimeSlotID),
		},
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:

func (c StudentGroupConflictConstraint) Name() string { return "StudentGroupConflict" }

func (c StudentGroupConflictConstraint) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	if _, ok := assignment.OccupiedSlotIDs(p); !ok {
		return []diagnostics.Violation{c.buildInvalidDurationViolation(assignment)}
	}
	if conflictingID, groupID, hasConflict := c.checkAssignment(p, solution, assignment); hasConflict {
		return []diagnostics.Violation{c.buildViolation(assignment, conflictingID, groupID)}
	}
	return nil
}

func (c StudentGroupConflictConstraint) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}
