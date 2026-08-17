package problem

import (
	"fmt"
	"sort"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

const validationConstraint = "ProblemValidation"

// Validate reports structural problem issues before any solver search starts.
func Validate(p Problem) []diagnostics.Violation {
	var violations []diagnostics.Violation

	for _, id := range sortedClassIDs(p.Classes) {
		class := p.Classes[id]
		if class.ID == "" {
			violations = append(violations, validationViolation("class has empty ID", map[string]string{"classMapKey": string(id)}, nil))
		}
		if class.ProgramID == "" || !hasProgram(p, class.ProgramID) {
			violations = append(violations, validationViolation("class references missing program", map[string]string{"classId": string(id), "programId": string(class.ProgramID)}, nil))
		}
		if class.WholeGroupID == "" || !hasStudentGroup(p, class.WholeGroupID) {
			violations = append(violations, validationViolation("class references missing whole student group", map[string]string{"classId": string(id), "studentGroupId": string(class.WholeGroupID)}, nil))
		} else if wholeGroup := p.StudentGroups[class.WholeGroupID]; wholeGroup.ClassID != id {
			violations = append(violations, validationViolation("class whole student group belongs to a different class", map[string]string{
				"classId":             string(id),
				"studentGroupId":      string(class.WholeGroupID),
				"studentGroupClassId": string(wholeGroup.ClassID),
			}, nil))
		}
		if !classContainsListedGroup(class, class.WholeGroupID) {
			violations = append(violations, validationViolation("class student group list must include whole group", map[string]string{"classId": string(id), "studentGroupId": string(class.WholeGroupID)}, nil))
		}
		seenGroups := make(map[model.StudentGroupID]struct{}, len(class.StudentGroupIDs))
		for _, groupID := range class.StudentGroupIDs {
			if _, seen := seenGroups[groupID]; seen {
				violations = append(violations, validationViolation("class contains duplicate student group", map[string]string{"classId": string(id), "studentGroupId": string(groupID)}, nil))
			}
			seenGroups[groupID] = struct{}{}
			group, ok := p.StudentGroups[groupID]
			if !ok {
				violations = append(violations, validationViolation("class references missing student group", map[string]string{"classId": string(id), "studentGroupId": string(groupID)}, nil))
				continue
			}
			if group.ClassID != id {
				violations = append(violations, validationViolation("class student group belongs to a different class", map[string]string{
					"classId":             string(id),
					"studentGroupId":      string(groupID),
					"studentGroupClassId": string(group.ClassID),
				}, nil))
			}
		}
	}

	for _, id := range sortedStudentGroupIDs(p.StudentGroups) {
		group := p.StudentGroups[id]
		if group.ID == "" {
			violations = append(violations, validationViolation("student group has empty ID", map[string]string{"studentGroupMapKey": string(id)}, nil))
		}
		if group.ClassID == "" || !hasClass(p, group.ClassID) {
			violations = append(violations, validationViolation("student group references missing class", map[string]string{"studentGroupId": string(id), "classId": string(group.ClassID)}, nil))
		}
		if group.Size < 0 {
			violations = append(violations, validationViolation("student group has negative size", map[string]string{"studentGroupId": string(id)}, map[string]string{"size": fmt.Sprintf("%d", group.Size)}))
		}
	}

	for _, id := range sortedCourseOfferingIDs(p.CourseOfferings) {
		offering := p.CourseOfferings[id]
		if offering.ID == "" {
			violations = append(violations, validationViolation("course offering has empty ID", map[string]string{"courseOfferingMapKey": string(id)}, nil))
		}
		if offering.TermID == "" || offering.TermID != p.Term.ID {
			violations = append(violations, validationViolation("course offering references invalid term", map[string]string{"courseOfferingId": string(id), "termId": string(offering.TermID)}, nil))
		}
		class, classOK := p.Classes[offering.ClassID]
		if offering.ClassID == "" || !classOK {
			violations = append(violations, validationViolation("course offering references missing class", map[string]string{"courseOfferingId": string(id), "classId": string(offering.ClassID)}, nil))
		}
		if offering.SubjectID == "" || !hasSubject(p, offering.SubjectID) {
			violations = append(violations, validationViolation("course offering references missing subject", map[string]string{"courseOfferingId": string(id), "subjectId": string(offering.SubjectID)}, nil))
		}
		group, groupOK := p.StudentGroups[offering.StudentGroupID]
		if offering.StudentGroupID == "" || !groupOK {
			violations = append(violations, validationViolation("course offering references missing student group", map[string]string{"courseOfferingId": string(id), "studentGroupId": string(offering.StudentGroupID)}, nil))
		}
		if classOK && groupOK {
			if group.ClassID != offering.ClassID {
				violations = append(violations, validationViolation("course offering student group belongs to a different class", map[string]string{
					"courseOfferingId":    string(id),
					"classId":             string(offering.ClassID),
					"studentGroupId":      string(offering.StudentGroupID),
					"studentGroupClassId": string(group.ClassID),
				}, nil))
			}
			if !classContainsGroup(class, offering.StudentGroupID) {
				violations = append(violations, validationViolation("course offering student group is not listed on its class", map[string]string{
					"courseOfferingId": string(id),
					"classId":          string(offering.ClassID),
					"studentGroupId":   string(offering.StudentGroupID),
				}, nil))
			}
		}
		if offering.FacultyID == "" || !hasFaculty(p, offering.FacultyID) {
			violations = append(violations, validationViolation("course offering references missing faculty", map[string]string{"courseOfferingId": string(id), "facultyId": string(offering.FacultyID)}, nil))
		}
		if len(offering.SessionRequirementIDs) == 0 {
			violations = append(violations, validationViolation("course offering has no session requirements", map[string]string{"courseOfferingId": string(id)}, nil))
		}
		seenRequirements := make(map[model.SessionRequirementID]struct{}, len(offering.SessionRequirementIDs))
		for _, requirementID := range offering.SessionRequirementIDs {
			if _, seen := seenRequirements[requirementID]; seen {
				violations = append(violations, validationViolation("course offering contains duplicate session requirement", map[string]string{"courseOfferingId": string(id), "sessionRequirementId": string(requirementID)}, nil))
			}
			seenRequirements[requirementID] = struct{}{}
			requirement, ok := p.SessionRequirements[requirementID]
			if !ok {
				violations = append(violations, validationViolation("course offering references missing session requirement", map[string]string{"courseOfferingId": string(id), "sessionRequirementId": string(requirementID)}, nil))
				continue
			}
			if requirement.CourseOfferingID != id {
				violations = append(violations, validationViolation("session requirement belongs to a different course offering", map[string]string{
					"courseOfferingId":            string(id),
					"sessionRequirementId":        string(requirementID),
					"requirementCourseOfferingId": string(requirement.CourseOfferingID),
				}, nil))
			}
		}
		for _, featureID := range offering.RequiredRoomFeatureIDs {
			if !hasRoomFeature(p, featureID) {
				violations = append(violations, validationViolation("course offering references missing room feature", map[string]string{"courseOfferingId": string(id), "roomFeatureId": string(featureID)}, nil))
			}
		}
	}

	for _, id := range sortedSessionRequirementIDs(p.SessionRequirements) {
		requirement := p.SessionRequirements[id]
		if requirement.ID == "" {
			violations = append(violations, validationViolation("session requirement has empty ID", map[string]string{"sessionRequirementMapKey": string(id)}, nil))
		}
		if requirement.CourseOfferingID == "" || !hasCourseOffering(p, requirement.CourseOfferingID) {
			violations = append(violations, validationViolation("session requirement references missing course offering", map[string]string{"sessionRequirementId": string(id), "courseOfferingId": string(requirement.CourseOfferingID)}, nil))
		}
		if requirement.SessionsPerWeek <= 0 {
			violations = append(violations, validationViolation("session requirement has non-positive sessions per week", map[string]string{"sessionRequirementId": string(id)}, map[string]string{"sessionsPerWeek": fmt.Sprintf("%d", requirement.SessionsPerWeek)}))
		}
		if requirement.Duration <= 0 {
			violations = append(violations, validationViolation("session requirement has non-positive duration", map[string]string{"sessionRequirementId": string(id)}, map[string]string{"duration": fmt.Sprintf("%d", requirement.Duration)}))
		}
		if requirement.Type != model.SessionTypeTheory && requirement.Type != model.SessionTypeLab {
			violations = append(violations, validationViolation("session requirement has invalid type", map[string]string{"sessionRequirementId": string(id)}, map[string]string{"type": string(requirement.Type)}))
		}
		for _, featureID := range requirement.RequiredRoomFeatureIDs {
			if !hasRoomFeature(p, featureID) {
				violations = append(violations, validationViolation("session requirement references missing room feature", map[string]string{"sessionRequirementId": string(id), "roomFeatureId": string(featureID)}, nil))
			}
		}
	}

	for _, id := range sortedRoomIDs(p.Rooms) {
		room := p.Rooms[id]
		if room.ID == "" {
			violations = append(violations, validationViolation("room has empty ID", map[string]string{"roomMapKey": string(id)}, nil))
		}
		if room.Capacity < 0 {
			violations = append(violations, validationViolation("room has negative capacity", map[string]string{"roomId": string(id)}, map[string]string{"capacity": fmt.Sprintf("%d", room.Capacity)}))
		}
		for _, featureID := range room.FeatureIDs {
			if !hasRoomFeature(p, featureID) {
				violations = append(violations, validationViolation("room references missing room feature", map[string]string{"roomId": string(id), "roomFeatureId": string(featureID)}, nil))
			}
		}
	}

	seenSlots := make(map[model.SlotKey]model.TimeSlotID, len(p.TimeSlots))
	for _, id := range sortedTimeSlotIDs(p.TimeSlots) {
		slot := p.TimeSlots[id]
		if slot.ID == "" {
			violations = append(violations, validationViolation("time slot has empty ID", map[string]string{"timeSlotMapKey": string(id)}, nil))
		}
		if slot.Period <= 0 {
			violations = append(violations, validationViolation("time slot has non-positive period", map[string]string{"timeSlotId": string(id)}, map[string]string{"period": fmt.Sprintf("%d", slot.Period)}))
		}
		key := slot.Key()
		if existingID, exists := seenSlots[key]; exists {
			violations = append(violations, validationViolation("duplicate day/period time slot", map[string]string{
				"timeSlotId":          string(id),
				"duplicateTimeSlotId": string(existingID),
			}, map[string]string{
				"day":    slot.Day.String(),
				"period": fmt.Sprintf("%d", slot.Period),
			}))
		}
		seenSlots[key] = id
	}

	for _, availability := range p.FacultyAvailabilities {
		if !hasFaculty(p, availability.FacultyID) {
			violations = append(violations, validationViolation("faculty availability references missing faculty", map[string]string{"facultyId": string(availability.FacultyID), "timeSlotId": string(availability.TimeSlotID)}, nil))
		}
		if !hasTimeSlot(p, availability.TimeSlotID) {
			violations = append(violations, validationViolation("faculty availability references missing time slot", map[string]string{"facultyId": string(availability.FacultyID), "timeSlotId": string(availability.TimeSlotID)}, nil))
		}
	}
	for _, facultyID := range sortedFacultyAvailabilityIndexIDs(p.FacultyAvailable) {
		slotIDs := p.FacultyAvailable[facultyID]
		if !hasFaculty(p, facultyID) {
			violations = append(violations, validationViolation("faculty availability index references missing faculty", map[string]string{"facultyId": string(facultyID)}, nil))
		}
		for _, slotID := range sortedIndexedTimeSlotIDs(slotIDs) {
			if !hasTimeSlot(p, slotID) {
				violations = append(violations, validationViolation("faculty availability index references missing time slot", map[string]string{"facultyId": string(facultyID), "timeSlotId": string(slotID)}, nil))
			}
		}
	}
	for _, availability := range p.RoomAvailabilities {
		if !hasRoom(p, availability.RoomID) {
			violations = append(violations, validationViolation("room availability references missing room", map[string]string{"roomId": string(availability.RoomID), "timeSlotId": string(availability.TimeSlotID)}, nil))
		}
		if !hasTimeSlot(p, availability.TimeSlotID) {
			violations = append(violations, validationViolation("room availability references missing time slot", map[string]string{"roomId": string(availability.RoomID), "timeSlotId": string(availability.TimeSlotID)}, nil))
		}
	}
	for _, roomID := range sortedRoomAvailabilityIndexIDs(p.RoomAvailable) {
		slotIDs := p.RoomAvailable[roomID]
		if !hasRoom(p, roomID) {
			violations = append(violations, validationViolation("room availability index references missing room", map[string]string{"roomId": string(roomID)}, nil))
		}
		for _, slotID := range sortedIndexedTimeSlotIDs(slotIDs) {
			if !hasTimeSlot(p, slotID) {
				violations = append(violations, validationViolation("room availability index references missing time slot", map[string]string{"roomId": string(roomID), "timeSlotId": string(slotID)}, nil))
			}
		}
	}

	for _, preference := range p.FacultyPreferences {
		if !hasFaculty(p, preference.FacultyID) {
			violations = append(violations, validationViolation("faculty preference references missing faculty", map[string]string{"facultyId": string(preference.FacultyID), "timeSlotId": string(preference.TimeSlotID)}, nil))
		}
		if !hasTimeSlot(p, preference.TimeSlotID) {
			violations = append(violations, validationViolation("faculty preference references missing time slot", map[string]string{"facultyId": string(preference.FacultyID), "timeSlotId": string(preference.TimeSlotID)}, nil))
		}
	}

	if p.Term.ID == "" {
		violations = append(violations, validationViolation("term has empty ID", nil, nil))
	}
	if len(p.Rooms) == 0 {
		violations = append(violations, validationViolation("problem has no rooms", nil, nil))
	}
	if len(p.TimeSlots) == 0 {
		violations = append(violations, validationViolation("problem has no time slots", nil, nil))
	}

	return violations
}

func validationViolation(message string, related map[string]string, metadata map[string]string) diagnostics.Violation {
	return diagnostics.Violation{
		ConstraintName: validationConstraint,
		Severity:       diagnostics.SeverityHard,
		Message:        message,
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}

func classContainsGroup(class model.Class, groupID model.StudentGroupID) bool {
	if class.WholeGroupID == groupID {
		return true
	}
	return classContainsListedGroup(class, groupID)
}

func classContainsListedGroup(class model.Class, groupID model.StudentGroupID) bool {
	for _, id := range class.StudentGroupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

func hasProgram(p Problem, id model.ProgramID) bool {
	_, ok := p.Programs[id]
	return ok
}

func hasClass(p Problem, id model.ClassID) bool {
	_, ok := p.Classes[id]
	return ok
}

func hasStudentGroup(p Problem, id model.StudentGroupID) bool {
	_, ok := p.StudentGroups[id]
	return ok
}

func hasSubject(p Problem, id model.SubjectID) bool {
	_, ok := p.Subjects[id]
	return ok
}

func hasCourseOffering(p Problem, id model.CourseOfferingID) bool {
	_, ok := p.CourseOfferings[id]
	return ok
}

func hasFaculty(p Problem, id model.FacultyID) bool {
	_, ok := p.Faculty[id]
	return ok
}

func hasRoom(p Problem, id model.RoomID) bool {
	_, ok := p.Rooms[id]
	return ok
}

func hasRoomFeature(p Problem, id model.RoomFeatureID) bool {
	_, ok := p.RoomFeatures[id]
	return ok
}

func hasTimeSlot(p Problem, id model.TimeSlotID) bool {
	_, ok := p.TimeSlots[id]
	return ok
}

func sortedClassIDs(values map[model.ClassID]model.Class) []model.ClassID {
	ids := make([]model.ClassID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedStudentGroupIDs(values map[model.StudentGroupID]model.StudentGroup) []model.StudentGroupID {
	ids := make([]model.StudentGroupID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedCourseOfferingIDs(values map[model.CourseOfferingID]model.CourseOffering) []model.CourseOfferingID {
	ids := make([]model.CourseOfferingID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedSessionRequirementIDs(values map[model.SessionRequirementID]model.SessionRequirement) []model.SessionRequirementID {
	ids := make([]model.SessionRequirementID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedRoomIDs(values map[model.RoomID]model.Room) []model.RoomID {
	ids := make([]model.RoomID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedTimeSlotIDs(values map[model.TimeSlotID]model.TimeSlot) []model.TimeSlotID {
	ids := make([]model.TimeSlotID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left := values[ids[i]]
		right := values[ids[j]]
		if left.Day != right.Day {
			return left.Day < right.Day
		}
		if left.Period != right.Period {
			return left.Period < right.Period
		}
		return ids[i] < ids[j]
	})
	return ids
}

func sortedFacultyAvailabilityIndexIDs(values map[model.FacultyID]map[model.TimeSlotID]struct{}) []model.FacultyID {
	ids := make([]model.FacultyID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedRoomAvailabilityIndexIDs(values map[model.RoomID]map[model.TimeSlotID]struct{}) []model.RoomID {
	ids := make([]model.RoomID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedIndexedTimeSlotIDs(values map[model.TimeSlotID]struct{}) []model.TimeSlotID {
	ids := make([]model.TimeSlotID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
