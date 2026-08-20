package constraints

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// FacultyConflictConstraint implements ConstraintDef and ScopedValidator for faculty conflicts.
type FacultyConflictConstraint struct {
	instance ConstraintInstance
}

type FacultyConflict = FacultyConflictConstraint

// NewFacultyConflictConstraint creates a FacultyConflictConstraint from a ConstraintInstance.
func NewFacultyConflictConstraint(inst ConstraintInstance) FacultyConflictConstraint {
	if inst.ID == "" {
		inst.ID = "FacultyConflict"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "FacultyConflict"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return FacultyConflictConstraint{instance: inst}
}

func (c FacultyConflictConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "FacultyConflict"
}

func (c FacultyConflictConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c FacultyConflictConstraint) Scope() string {
	return c.instance.Scope
}

func (c FacultyConflictConstraint) TemplateID() string {
	return "FacultyConflict"
}

// Core implementation path for checking faculty conflicts.
func (c FacultyConflictConstraint) checkAssignment(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) (problem.AssignmentID, bool) {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return "", false
	}
	conflictingID, ok := solution.Index.FacultyConflict(assignment.FacultyID, slotIDs)
	if ok && conflictingID != assignment.ID {
		return conflictingID, true
	}
	return "", false
}

// IsConsistent checks if candidate assignment is consistent with partial solution.
func (c FacultyConflictConstraint) IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil || partial == nil {
		return true
	}
	_, hasConflict := c.checkAssignment(p, partial, candidate)
	return !hasConflict
}

// ViolatedByMove checks if applying move causes a faculty conflict.
func (c FacultyConflictConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
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

	if conflictingID, ok := sol.Index.FacultyConflict(candidate.FacultyID, slotIDs); ok && conflictingID != assignment.ID {
		return []diagnostics.Violation{
			c.buildViolation(candidate, conflictingID),
		}
	}
	return nil
}

// Evaluate checks the full solution for faculty conflicts.
func (c FacultyConflictConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	var violations []diagnostics.Violation
	seenPairs := make(map[string]bool)

	for _, a := range sol.Assignments {
		if conflictingID, hasConflict := c.checkAssignment(p, sol, a); hasConflict {
			pairKey := string(a.ID) + "|" + string(conflictingID)
			altKey := string(conflictingID) + "|" + string(a.ID)
			if !seenPairs[pairKey] && !seenPairs[altKey] {
				seenPairs[pairKey] = true
				violations = append(violations, c.buildViolation(a, conflictingID))
			}
		}
	}
	return violations
}

func (c FacultyConflictConstraint) buildViolation(assignment problem.Assignment, conflictingID problem.AssignmentID) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	return diagnostics.Violation{
		ConstraintName: "FacultyConflict",
		ConstraintID:   c.ID(),
		TemplateID:     "FacultyConflict",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "faculty is already scheduled in an occupied time slot",
		AssignmentID:   string(assignment.ID),
		RelatedIDs: map[string]string{
			"facultyId":               string(assignment.FacultyID),
			"conflictingAssignmentId": string(conflictingID),
			"courseOfferingId":        string(assignment.CourseOfferingID),
			"sessionRequirementId":    string(assignment.SessionRequirementID),
			"studentGroupId":          string(assignment.StudentGroupID),
			"timeSlotId":              string(assignment.TimeSlotID),
		},
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:
func (c FacultyConflictConstraint) Name() string { return "FacultyConflict" }

func (c FacultyConflictConstraint) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	if conflictingID, hasConflict := c.checkAssignment(p, solution, assignment); hasConflict {
		return []diagnostics.Violation{c.buildViolation(assignment, conflictingID)}
	}
	return nil
}

func (c FacultyConflictConstraint) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}
