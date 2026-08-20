package constraints

import (
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// RoomCapacityConstraint implements ConstraintDef and ScopedValidator for room capacity constraints.
type RoomCapacityConstraint struct {
	instance ConstraintInstance
}

type RoomCapacity = RoomCapacityConstraint

// NewRoomCapacityConstraint creates a RoomCapacityConstraint from a ConstraintInstance.
func NewRoomCapacityConstraint(inst ConstraintInstance) RoomCapacityConstraint {
	if inst.ID == "" {
		inst.ID = "RoomCapacity"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "RoomCapacity"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}
	return RoomCapacityConstraint{instance: inst}
}

func (c RoomCapacityConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "RoomCapacity"
}

func (c RoomCapacityConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c RoomCapacityConstraint) Scope() string {
	return c.instance.Scope
}

func (c RoomCapacityConstraint) TemplateID() string {
	return "RoomCapacity"
}

// IsConsistent checks if candidate assignment satisfies room capacity.
func (c RoomCapacityConstraint) IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil {
		return true
	}
	room, ok := p.Room(candidate.RoomID)
	if !ok {
		return false
	}
	groupSize := p.StudentGroupSize(candidate.StudentGroupID)
	return room.Capacity >= groupSize
}

// ViolatedByMove checks if applying move causes a room capacity violation.
func (c RoomCapacityConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
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

	room, ok := p.Room(candidate.RoomID)
	if !ok {
		return []diagnostics.Violation{
			c.buildViolation(candidate, candidate.RoomID, 0, 0, true),
		}
	}
	groupSize := p.StudentGroupSize(candidate.StudentGroupID)
	if room.Capacity < groupSize {
		return []diagnostics.Violation{
			c.buildViolation(candidate, room.ID, room.Capacity, groupSize, false),
		}
	}
	return nil
}

// Evaluate checks the full solution for room capacity violations.
func (c RoomCapacityConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	var violations []diagnostics.Violation
	for _, a := range sol.Assignments {
		room, ok := p.Room(a.RoomID)
		if !ok {
			violations = append(violations, c.buildViolation(a, a.RoomID, 0, 0, true))
			continue
		}
		groupSize := p.StudentGroupSize(a.StudentGroupID)
		if room.Capacity < groupSize {
			violations = append(violations, c.buildViolation(a, room.ID, room.Capacity, groupSize, false))
		}
	}
	return violations
}

func (c RoomCapacityConstraint) buildViolation(assignment problem.Assignment, roomID model.RoomID, roomCapacity int, groupSize int, missingRoom bool) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	related := map[string]string{
		"roomId":               string(roomID),
		"courseOfferingId":     string(assignment.CourseOfferingID),
		"sessionRequirementId": string(assignment.SessionRequirementID),
		"studentGroupId":       string(assignment.StudentGroupID),
		"timeSlotId":           string(assignment.TimeSlotID),
	}

	var metadata map[string]string
	msg := "room capacity is below student group size"
	if missingRoom {
		msg = "room does not exist"
	} else {
		metadata = map[string]string{
			"roomCapacity":     fmt.Sprintf("%d", roomCapacity),
			"studentGroupSize": fmt.Sprintf("%d", groupSize),
		}
	}

	return diagnostics.Violation{
		ConstraintName: "RoomCapacity",
		ConstraintID:   c.ID(),
		TemplateID:     "RoomCapacity",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        msg,
		AssignmentID:   string(assignment.ID),
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}

// Legacy Constraint and ScopedValidator interface compatibility:
func (c RoomCapacityConstraint) Name() string { return "RoomCapacity" }

func (c RoomCapacityConstraint) Check(p *problem.Problem, solution *problem.Solution, assignment problem.Assignment) []diagnostics.Violation {
	room, ok := p.Room(assignment.RoomID)
	if !ok {
		return []diagnostics.Violation{c.buildViolation(assignment, assignment.RoomID, 0, 0, true)}
	}
	groupSize := p.StudentGroupSize(assignment.StudentGroupID)
	if room.Capacity < groupSize {
		return []diagnostics.Violation{c.buildViolation(assignment, room.ID, room.Capacity, groupSize, false)}
	}
	return nil
}

func (c RoomCapacityConstraint) CheckAtSlot(p *problem.Problem, solution *problem.Solution, a problem.Assignment, slot model.TimeSlotID) []diagnostics.Violation {
	a.TimeSlotID = slot
	return c.Check(p, solution, a)
}
