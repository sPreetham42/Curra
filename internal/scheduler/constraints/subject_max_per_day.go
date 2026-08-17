package constraints

import (
	"fmt"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// SubjectMaxPerDayConstraint limits how many sessions of a specific subject/course can be scheduled on any single day for a student group.
type SubjectMaxPerDayConstraint struct {
	instance   ConstraintInstance
	SubjectID  model.SubjectID
	OfferingID model.CourseOfferingID
	MaxPerDay  int
}

// NewSubjectMaxPerDayConstraint parses params and returns a SubjectMaxPerDayConstraint.
func NewSubjectMaxPerDayConstraint(inst ConstraintInstance) SubjectMaxPerDayConstraint {
	if inst.ID == "" {
		inst.ID = "SubjectMaxPerDay"
	}
	if inst.TemplateID == "" {
		inst.TemplateID = "SubjectMaxPerDay"
	}
	if inst.Kind == "" {
		inst.Kind = ConstraintKindHard
	}

	c := SubjectMaxPerDayConstraint{
		instance:  inst,
		MaxPerDay: 1,
	}

	if params := inst.Params; params != nil {
		if s, ok := params["subjectId"].(string); ok {
			c.SubjectID = model.SubjectID(s)
		}
		if o, ok := params["courseOfferingId"].(string); ok {
			c.OfferingID = model.CourseOfferingID(o)
		}
		if m, ok := params["maxPerDay"].(int); ok {
			c.MaxPerDay = m
		} else if m, ok := params["maxPerDay"].(float64); ok {
			c.MaxPerDay = int(m)
		}
	}

	return c
}

func (c SubjectMaxPerDayConstraint) ID() string {
	if c.instance.ID != "" {
		return c.instance.ID
	}
	return "SubjectMaxPerDay"
}

func (c SubjectMaxPerDayConstraint) Kind() ConstraintKind {
	if c.instance.Kind != "" {
		return c.instance.Kind
	}
	return ConstraintKindHard
}

func (c SubjectMaxPerDayConstraint) Scope() string {
	return c.instance.Scope
}

func (c SubjectMaxPerDayConstraint) TemplateID() string {
	return "SubjectMaxPerDay"
}

// Matches checks whether an assignment belongs to the target subject/course offering.
func (c SubjectMaxPerDayConstraint) Matches(p *problem.Problem, a problem.Assignment) bool {
	if c.OfferingID != "" && a.CourseOfferingID == c.OfferingID {
		return true
	}
	if c.SubjectID != "" {
		if offering, ok := p.CourseOfferings[a.CourseOfferingID]; ok {
			if offering.SubjectID == c.SubjectID {
				return true
			}
		}
	}
	return c.SubjectID == "" && c.OfferingID == ""
}

// IsConsistent checks if candidate assignment causes subject occurrences on candidate's day to exceed MaxPerDay for student group.
func (c SubjectMaxPerDayConstraint) IsConsistent(ctx *SearchCtx, partial *problem.Solution, candidate problem.Assignment) bool {
	p := ctx.Problem
	if p == nil || partial == nil {
		return true
	}

	if !c.Matches(p, candidate) {
		return true
	}

	slot, ok := p.TimeSlots[candidate.TimeSlotID]
	if !ok {
		return true
	}

	count := c.countOnDay(ctx, partial, candidate.StudentGroupID, slot.Day, candidate.ID)
	return (count + 1) <= c.MaxPerDay
}

// ViolatedByMove checks if applying move causes subject max per day violation.
func (c SubjectMaxPerDayConstraint) ViolatedByMove(ctx *SearchCtx, sol *problem.Solution, mv problem.Move) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	if err := sol.ApplyMove(p, mv); err != nil {
		return nil
	}
	defer func() {
		_ = sol.UndoMove(p, mv)
	}()

	assignment, ok := sol.Index.AssignmentByID(mv.AssignmentID)
	if !ok || !c.Matches(p, assignment) {
		return nil
	}

	slot, ok := p.TimeSlots[assignment.TimeSlotID]
	if !ok {
		return nil
	}

	count := c.countOnDay(ctx, sol, assignment.StudentGroupID, slot.Day, "")
	if count > c.MaxPerDay {
		return []diagnostics.Violation{
			c.buildViolation(assignment, slot.Day, count),
		}
	}
	return nil
}

// Evaluate checks the full solution for subject max per day violations.
func (c SubjectMaxPerDayConstraint) Evaluate(ctx *SearchCtx, sol *problem.Solution) []diagnostics.Violation {
	p := ctx.Problem
	if p == nil || sol == nil {
		return nil
	}

	type groupDayKey struct {
		GroupID model.StudentGroupID
		Day     time.Weekday
	}

	groupDayAssignments := make(map[groupDayKey][]problem.Assignment)

	for _, a := range sol.Assignments {
		if !c.Matches(p, a) {
			continue
		}
		slot, ok := p.TimeSlots[a.TimeSlotID]
		if !ok {
			continue
		}
		key := groupDayKey{GroupID: a.StudentGroupID, Day: slot.Day}
		groupDayAssignments[key] = append(groupDayAssignments[key], a)
	}

	var violations []diagnostics.Violation
	for key, assignments := range groupDayAssignments {
		if len(assignments) > c.MaxPerDay {
			for _, a := range assignments {
				violations = append(violations, c.buildViolation(a, key.Day, len(assignments)))
			}
		}
	}

	return violations
}

func (c SubjectMaxPerDayConstraint) countOnDay(ctx *SearchCtx, sol *problem.Solution, groupID model.StudentGroupID, day time.Weekday, ignoreID problem.AssignmentID) int {
	p := ctx.Problem
	count := 0
	for _, a := range sol.Assignments {
		if a.ID == ignoreID {
			continue
		}
		if !c.Matches(p, a) {
			continue
		}
		if ctx != nil && ctx.Membership != nil && !ctx.Membership.GroupsOverlap(a.StudentGroupID, groupID) {
			continue
		}
		slot, ok := p.TimeSlots[a.TimeSlotID]
		if ok && slot.Day == day {
			count++
		}
	}
	return count
}

func (c SubjectMaxPerDayConstraint) buildViolation(a problem.Assignment, day time.Weekday, count int) diagnostics.Violation {
	severity := diagnostics.SeverityHard
	if c.Kind() == ConstraintKindSoft {
		severity = diagnostics.SeveritySoft
	}

	return diagnostics.Violation{
		ConstraintName: "SubjectMaxPerDay",
		ConstraintID:   c.ID(),
		TemplateID:     "SubjectMaxPerDay",
		Scope:          c.Scope(),
		Severity:       severity,
		Message:        fmt.Sprintf("subject exceeds max per day limit of %d (found %d on %s)", c.MaxPerDay, count, day),
		AssignmentID:   string(a.ID),
		RelatedIDs: map[string]string{
			"subjectId":            string(c.SubjectID),
			"courseOfferingId":     string(a.CourseOfferingID),
			"sessionRequirementId": string(a.SessionRequirementID),
			"studentGroupId":       string(a.StudentGroupID),
			"timeSlotId":           string(a.TimeSlotID),
		},
		Metadata: map[string]string{
			"day":       day.String(),
			"count":     fmt.Sprintf("%d", count),
			"maxPerDay": fmt.Sprintf("%d", c.MaxPerDay),
		},
	}
}
