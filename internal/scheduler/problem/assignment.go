package problem

import (
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

// AssignmentID uniquely identifies a scheduled session instance.
type AssignmentID string

func NewAssignmentID(requirementID model.SessionRequirementID, instance int) AssignmentID {
	return AssignmentID(fmt.Sprintf("%s#%d", requirementID, instance))
}

// Assignment represents one scheduled session instance.
type Assignment struct {
	ID                   AssignmentID               `json:"id"`
	CourseOfferingID     model.CourseOfferingID     `json:"courseOfferingId"`
	StudentGroupID       model.StudentGroupID       `json:"studentGroupId"`
	FacultyID            model.FacultyID            `json:"facultyId"`
	RoomID               model.RoomID               `json:"roomId"`
	TimeSlotID           model.TimeSlotID           `json:"timeSlotId"`
	SessionRequirementID model.SessionRequirementID `json:"sessionRequirementId"`
	// Instance is the 0-based index within SessionsPerWeek for this requirement.
	Instance int `json:"instance"`
}

// OccupiedSlotIDs returns all weekly slots consumed by this assignment.
func (a Assignment) OccupiedSlotIDs(p *Problem) ([]model.TimeSlotID, bool) {
	req, ok := p.SessionRequirements[a.SessionRequirementID]
	if !ok {
		return nil, false
	}
	return p.OccupiedSlotIDs(a.TimeSlotID, req.Duration)
}
