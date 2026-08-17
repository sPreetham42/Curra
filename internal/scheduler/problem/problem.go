package problem

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

// Problem is a self-contained academic scheduling instance.
type Problem struct {
	TenantID model.TenantID
	Term     model.Term

	Departments           map[model.DepartmentID]model.Department
	Programs              map[model.ProgramID]model.Program
	Classes               map[model.ClassID]model.Class
	StudentGroups         map[model.StudentGroupID]model.StudentGroup
	Subjects              map[model.SubjectID]model.Subject
	CourseOfferings       map[model.CourseOfferingID]model.CourseOffering
	SessionRequirements   map[model.SessionRequirementID]model.SessionRequirement
	Faculty               map[model.FacultyID]model.Faculty
	FacultyAvailabilities []model.FacultyAvailability
	FacultyPreferences    []model.FacultyPreference
	Rooms                 map[model.RoomID]model.Room
	RoomAvailabilities    []model.RoomAvailability
	RoomFeatures          map[model.RoomFeatureID]model.RoomFeature
	TimeSlots             map[model.TimeSlotID]model.TimeSlot

	FacultyAvailable map[model.FacultyID]map[model.TimeSlotID]struct{}
	RoomAvailable    map[model.RoomID]map[model.TimeSlotID]struct{}

	// SlotsByDayPeriod maps day+period to TimeSlotID for consecutive slot expansion.
	SlotsByDayPeriod map[model.SlotKey]model.TimeSlotID
	// StudentGroupOverlaps maps a group to groups that share students with it.
	StudentGroupOverlaps map[model.StudentGroupID]map[model.StudentGroupID]struct{}
	// PeriodsPerDay is the number of periods on each working day.
	PeriodsPerDay int
}

// Prepare builds derived indexes used by constraints and the solver.
func (p *Problem) Prepare() {
	p.BuildSlotIndex()
	p.BuildAvailabilityIndexes()
	p.BuildStudentGroupOverlaps()
}

// CourseOffering returns the offering for an assignment, or nil if missing.
func (p *Problem) CourseOffering(id model.CourseOfferingID) (model.CourseOffering, bool) {
	o, ok := p.CourseOfferings[id]
	return o, ok
}

// SessionRequirement returns the requirement for an assignment, or nil if missing.
func (p *Problem) SessionRequirement(id model.SessionRequirementID) (model.SessionRequirement, bool) {
	r, ok := p.SessionRequirements[id]
	return r, ok
}

// SubjectForOffering returns the subject taught by an offering.
func (p *Problem) SubjectForOffering(offeringID model.CourseOfferingID) (model.Subject, bool) {
	offering, ok := p.CourseOfferings[offeringID]
	if !ok {
		return model.Subject{}, false
	}
	subject, ok := p.Subjects[offering.SubjectID]
	return subject, ok
}

// Room returns a room by ID.
func (p *Problem) Room(id model.RoomID) (model.Room, bool) {
	room, ok := p.Rooms[id]
	return room, ok
}

// RequiredRoomFeatures returns the union of feature requirements from the
// course offering and session requirement.
func (p *Problem) RequiredRoomFeatures(offeringID model.CourseOfferingID, requirementID model.SessionRequirementID) []model.RoomFeatureID {
	required := make([]model.RoomFeatureID, 0)
	seen := make(map[model.RoomFeatureID]struct{})
	if offering, ok := p.CourseOfferings[offeringID]; ok {
		for _, id := range offering.RequiredRoomFeatureIDs {
			if _, exists := seen[id]; !exists {
				required = append(required, id)
				seen[id] = struct{}{}
			}
		}
	}
	if requirement, ok := p.SessionRequirements[requirementID]; ok {
		for _, id := range requirement.RequiredRoomFeatureIDs {
			if _, exists := seen[id]; !exists {
				required = append(required, id)
				seen[id] = struct{}{}
			}
		}
	}
	return required
}

// StudentGroupSize returns enrollment for a student group.
func (p *Problem) StudentGroupSize(id model.StudentGroupID) int {
	if g, ok := p.StudentGroups[id]; ok {
		return g.Size
	}
	return 0
}

// BuildStudentGroupOverlaps derives overlap from class membership. The whole
// class group overlaps with each subgroup, exact same groups overlap, and
// subgroups listed under the same class are otherwise treated as disjoint.
func (p *Problem) BuildStudentGroupOverlaps() {
	p.StudentGroupOverlaps = make(map[model.StudentGroupID]map[model.StudentGroupID]struct{}, len(p.StudentGroups))
	for id := range p.StudentGroups {
		p.addStudentGroupOverlap(id, id)
	}
	for _, class := range p.Classes {
		if class.WholeGroupID == "" {
			continue
		}
		p.addStudentGroupOverlap(class.WholeGroupID, class.WholeGroupID)
		for _, groupID := range class.StudentGroupIDs {
			p.addStudentGroupOverlap(groupID, groupID)
			p.addStudentGroupOverlap(class.WholeGroupID, groupID)
			p.addStudentGroupOverlap(groupID, class.WholeGroupID)
		}
	}
}

func (p *Problem) addStudentGroupOverlap(left model.StudentGroupID, right model.StudentGroupID) {
	if left == "" || right == "" {
		return
	}
	if p.StudentGroupOverlaps[left] == nil {
		p.StudentGroupOverlaps[left] = make(map[model.StudentGroupID]struct{})
	}
	p.StudentGroupOverlaps[left][right] = struct{}{}
}

// StudentGroupsOverlap reports whether two groups share students according to
// the current Phase 1 class/group model.
func (p *Problem) StudentGroupsOverlap(left model.StudentGroupID, right model.StudentGroupID) bool {
	if left == right {
		return true
	}
	if p.StudentGroupOverlaps != nil {
		overlaps, ok := p.StudentGroupOverlaps[left]
		if !ok {
			return false
		}
		_, ok = overlaps[right]
		return ok
	}
	leftGroup, leftOK := p.StudentGroups[left]
	rightGroup, rightOK := p.StudentGroups[right]
	if !leftOK || !rightOK || leftGroup.ClassID != rightGroup.ClassID {
		return false
	}
	class, ok := p.Classes[leftGroup.ClassID]
	if !ok {
		return false
	}
	return class.WholeGroupID == left || class.WholeGroupID == right
}

// OverlappingStudentGroupIDs returns groups that must not overlap in time with id.
func (p *Problem) OverlappingStudentGroupIDs(id model.StudentGroupID) []model.StudentGroupID {
	if p.StudentGroupOverlaps == nil {
		p.BuildStudentGroupOverlaps()
	}
	overlaps := p.StudentGroupOverlaps[id]
	ids := make([]model.StudentGroupID, 0, len(overlaps))
	for overlapID := range overlaps {
		ids = append(ids, overlapID)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

// OccupiedSlotIDs returns all TimeSlotIDs occupied by a session starting at startSlot
// with the given duration. Returns false if slots would extend beyond the day grid.
func (p *Problem) OccupiedSlotIDs(startSlot model.TimeSlotID, duration int) ([]model.TimeSlotID, bool) {
	if duration <= 0 {
		return nil, false
	}
	start, ok := p.TimeSlots[startSlot]
	if !ok {
		return nil, false
	}
	ids := make([]model.TimeSlotID, 0, duration)
	for i := 0; i < duration; i++ {
		key := model.SlotKey{Day: start.Day, Period: start.Period + i}
		id, ok := p.SlotsByDayPeriod[key]
		if !ok {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

// BuildSlotIndex populates SlotsByDayPeriod from TimeSlots.
func (p *Problem) BuildSlotIndex() {
	if p.SlotsByDayPeriod == nil {
		p.SlotsByDayPeriod = make(map[model.SlotKey]model.TimeSlotID, len(p.TimeSlots))
	}
	for id, slot := range p.TimeSlots {
		p.SlotsByDayPeriod[slot.Key()] = id
	}
}

// BuildAvailabilityIndexes populates explicit allow-list indexes from
// availability records when the indexes were not supplied directly.
func (p *Problem) BuildAvailabilityIndexes() {
	if p.FacultyAvailable == nil {
		p.FacultyAvailable = make(map[model.FacultyID]map[model.TimeSlotID]struct{})
		for _, availability := range p.FacultyAvailabilities {
			if p.FacultyAvailable[availability.FacultyID] == nil {
				p.FacultyAvailable[availability.FacultyID] = make(map[model.TimeSlotID]struct{})
			}
			p.FacultyAvailable[availability.FacultyID][availability.TimeSlotID] = struct{}{}
		}
	}
	if p.RoomAvailable == nil {
		p.RoomAvailable = make(map[model.RoomID]map[model.TimeSlotID]struct{})
		for _, availability := range p.RoomAvailabilities {
			if p.RoomAvailable[availability.RoomID] == nil {
				p.RoomAvailable[availability.RoomID] = make(map[model.TimeSlotID]struct{})
			}
			p.RoomAvailable[availability.RoomID][availability.TimeSlotID] = struct{}{}
		}
	}
}

// IsFacultyAvailable reports whether faculty is available for all given slots.
func (p *Problem) IsFacultyAvailable(facultyID model.FacultyID, slotIDs []model.TimeSlotID) bool {
	avail, ok := p.FacultyAvailable[facultyID]
	if !ok {
		return false
	}
	for _, sid := range slotIDs {
		if _, ok := avail[sid]; !ok {
			return false
		}
	}
	return true
}

// IsRoomAvailable reports whether room is available for all given slots.
func (p *Problem) IsRoomAvailable(roomID model.RoomID, slotIDs []model.TimeSlotID) bool {
	avail, ok := p.RoomAvailable[roomID]
	if !ok {
		return false
	}
	for _, sid := range slotIDs {
		if _, ok := avail[sid]; !ok {
			return false
		}
	}
	return true
}

// RoomHasFeatures reports whether room provides all required features.
func (p *Problem) RoomHasFeatures(roomID model.RoomID, required []model.RoomFeatureID) bool {
	room, ok := p.Rooms[roomID]
	if !ok {
		return false
	}
	features := make(map[model.RoomFeatureID]struct{}, len(room.FeatureIDs))
	for _, fid := range room.FeatureIDs {
		features[fid] = struct{}{}
	}
	for _, req := range required {
		if _, ok := features[req]; !ok {
			return false
		}
	}
	return true
}
