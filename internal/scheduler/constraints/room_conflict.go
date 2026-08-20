package constraints

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// RoomConflictConstraint implements ConstraintDef and ScopedValidator for room conflicts.
type RoomConflictConstraint struct {
	instance ConstraintInstance
}

type RoomConflict = RoomConflictConstraint

// NewRoomConflictConstraint creates a RoomConflictConstraint from a ConstraintInstance.
func NewRoomConflictConstraint(inst ConstraintInstance) RoomConflictConstraint {
	if inst.ID == "" {
		inst.ID = "RoomConflict"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "RoomConflict"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return RoomConflictConstraint{instance: inst}
}

func (c RoomConflictConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "RoomConflict"
}

func (c RoomConflictConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c RoomConflictConstraint) Scope() string {
	return c.instance.Scope
}

func (c RoomConflictConstraint) TemplateID() string {
	return "RoomConflict"
}

// Core implementation path for checking room conflicts.
func (c RoomConflictConstraint) checkAssignment(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) (problem.AssignmentID, bool) {
	slotIDs, ok := assignment.OccupiedSlotIDs(p)
	if !ok {
		return "", false
	}
	conflictingID, ok := solution.Index.RoomConflict(assignment.RoomID, slotIDs)
	if ok && conflictingID != assignment.ID {
		return conflictingID, true
	}
	return "", false
}

// IsConsistent checks if candidate assignment is consistent with partial solution.
func (c RoomConflictConstraint) IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil || partial == nil {
		return true
	}
	_, hasConflict := c.checkAssignment(p, partial, candidate)
	return !hasConflict
}

// ViolatedByMove checks if applying move causes a room conflict.
func (c RoomConflictConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
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

	if conflictingID, ok := sol.Index.RoomConflict(candidate.RoomID, slotIDs); ok && conflictingID != assignment.ID {
		return []diagnostics.Violation{
			c.buildViolation(candidate, conflictingID),
		}
	}
	return nil
}

// Evaluate checks the full solution for room conflicts.
func (c RoomConflictConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	type roomSlotKey struct {
		RoomID     model.RoomID
		TimeSlotID model.TimeSlotID
	}

	slotOccupants := make(map[roomSlotKey][]problem.Assignment)
	for _, a := range sol.Assignments {
		slotIDs, ok := a.OccupiedSlotIDs(p)
		if !ok {
			continue
		}
		for _, sid := range slotIDs {
			k := roomSlotKey{RoomID: a.RoomID, TimeSlotID: sid}
			slotOccupants[k] = append(slotOccupants[k], a)
		}
	}

	var violations []diagnostics.Violation
	seenPairs := make(map[string]bool)

	for _, occupants := range slotOccupants {
		if len(occupants) > 1 {
			for i := 0; i < len(occupants); i++ {
				for j := i + 1; j < len(occupants); j++ {
					a1 := occupants[i]
					a2 := occupants[j]
					pairKey := string(a1.ID) + "|" + string(a2.ID)
					altKey := string(a2.ID) + "|" + string(a1.ID)
					if !seenPairs[pairKey] && !seenPairs[altKey] {
						seenPairs[pairKey] = true
						violations = append(violations, c.buildViolation(a1, a2.ID))
					}
				}
			}
		}
	}
	return violations
}

func (c RoomConflictConstraint) buildViolation(assignment problem.Assignment, conflictingID problem.AssignmentID) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	return diagnostics.Violation{
		ConstraintName: "RoomConflict",
		ConstraintID:   c.ID(),
		TemplateID:     "RoomConflict",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        "room is already scheduled in an occupied time slot",
		AssignmentID:   string(assignment.ID),
		RelatedIDs: map[string]string{
			"roomId":                  string(assignment.RoomID),
			"conflictingAssignmentId": string(conflictingID),
			"courseOfferingId":        string(assignment.CourseOfferingID),
			"sessionRequirementId":    string(assignment.SessionRequirementID),
			"studentGroupId":          string(assignment.StudentGroupID),
			"timeSlotId":              string(assignment.TimeSlotID),
		},
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:
func (c RoomConflictConstraint) Name() string { return "RoomConflict" }

func (c RoomConflictConstraint) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	if _, ok := assignment.OccupiedSlotIDs(p); !ok {
		return invalidDurationViolation(c.Name(), assignment)
	}
	if conflictingID, hasConflict := c.checkAssignment(p, solution, assignment); hasConflict {
		return []diagnostics.Violation{c.buildViolation(assignment, conflictingID)}
	}
	return nil
}

func (c RoomConflictConstraint) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}
