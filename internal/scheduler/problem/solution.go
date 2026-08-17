package problem

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

// Solution holds scheduled assignments and efficient lookup indexes.
type Solution struct {
	Assignments []Assignment  `json:"assignments"`
	Index       SolutionIndex `json:"-"`
	Score       scorer.Score  `json:"score"`
}

// NewSolution creates an empty solution with initialized indexes.
func NewSolution() Solution {
	return Solution{
		Index: NewSolutionIndex(),
	}
}

// Clone returns a deep copy of the solution and its indexes.
func (s Solution) Clone() Solution {
	cloned := Solution{
		Assignments: make([]Assignment, len(s.Assignments)),
		Index:       NewSolutionIndex(),
		Score:       s.Score,
	}
	copy(cloned.Assignments, s.Assignments)
	for k, v := range s.Index.FacultySlot {
		cloned.Index.FacultySlot[k] = v
	}
	for k, v := range s.Index.RoomSlot {
		cloned.Index.RoomSlot[k] = v
	}
	for k, v := range s.Index.StudentGroupSlot {
		cloned.Index.StudentGroupSlot[k] = v
	}
	for k, v := range s.Index.RequirementCount {
		cloned.Index.RequirementCount[k] = v
	}
	for k, v := range s.Index.byID {
		cloned.Index.byID[k] = v
	}
	return cloned
}

// CalculateScore evaluates the solution against the problem soft constraints without mutating anything.
func (s *Solution) CalculateScore(p *Problem) scorer.ScoreBreakdown {
	return p.StudentGapPenalty(s)
}

// AddAssignment appends and indexes an assignment.
func (s *Solution) AddAssignment(p *Problem, a Assignment) error {
	if err := s.Index.Add(p, a); err != nil {
		return err
	}
	s.Assignments = append(s.Assignments, a)
	return nil
}

// RemoveLastAssignment removes and unindexes the most recent assignment.
func (s *Solution) RemoveLastAssignment(p *Problem) {
	if len(s.Assignments) == 0 {
		return
	}
	last := s.Assignments[len(s.Assignments)-1]
	s.Assignments = s.Assignments[:len(s.Assignments)-1]
	s.Index.Remove(p, last)
}

// SolutionIndex supports O(1) conflict and count lookups.
type SolutionIndex struct {
	FacultySlot      map[resourceSlotKey]AssignmentID
	RoomSlot         map[resourceSlotKey]AssignmentID
	StudentGroupSlot map[resourceSlotKey]AssignmentID
	RequirementCount map[model.SessionRequirementID]int
	byID             map[AssignmentID]Assignment
}

type resourceSlotKey struct {
	Resource string
	Slot     model.TimeSlotID
}

func NewSolutionIndex() SolutionIndex {
	return SolutionIndex{
		FacultySlot:      make(map[resourceSlotKey]AssignmentID),
		RoomSlot:         make(map[resourceSlotKey]AssignmentID),
		StudentGroupSlot: make(map[resourceSlotKey]AssignmentID),
		RequirementCount: make(map[model.SessionRequirementID]int),
		byID:             make(map[AssignmentID]Assignment),
	}
}

func facultyKey(id model.FacultyID, slot model.TimeSlotID) resourceSlotKey {
	return resourceSlotKey{Resource: string(id), Slot: slot}
}

func roomKey(id model.RoomID, slot model.TimeSlotID) resourceSlotKey {
	return resourceSlotKey{Resource: string(id), Slot: slot}
}

func groupKey(id model.StudentGroupID, slot model.TimeSlotID) resourceSlotKey {
	return resourceSlotKey{Resource: string(id), Slot: slot}
}

// Add indexes an assignment and all occupied slots. Caller must ensure no conflicts.
func (idx *SolutionIndex) Add(p *Problem, a Assignment) error {
	slotIDs, ok := a.OccupiedSlotIDs(p)
	if !ok {
		return ErrInvalidAssignment
	}
	for _, sid := range slotIDs {
		if _, exists := idx.FacultySlot[facultyKey(a.FacultyID, sid)]; exists {
			return ErrFacultyConflict
		}
		if _, exists := idx.RoomSlot[roomKey(a.RoomID, sid)]; exists {
			return ErrRoomConflict
		}
		for _, groupID := range p.OverlappingStudentGroupIDs(a.StudentGroupID) {
			if _, exists := idx.StudentGroupSlot[groupKey(groupID, sid)]; exists {
				return ErrGroupConflict
			}
		}
	}
	for _, sid := range slotIDs {
		idx.FacultySlot[facultyKey(a.FacultyID, sid)] = a.ID
		idx.RoomSlot[roomKey(a.RoomID, sid)] = a.ID
		idx.StudentGroupSlot[groupKey(a.StudentGroupID, sid)] = a.ID
	}
	idx.RequirementCount[a.SessionRequirementID]++
	idx.byID[a.ID] = a
	return nil
}

// Remove unindexes an assignment.
func (idx *SolutionIndex) Remove(p *Problem, a Assignment) {
	slotIDs, ok := a.OccupiedSlotIDs(p)
	if !ok {
		return
	}
	for _, sid := range slotIDs {
		delete(idx.FacultySlot, facultyKey(a.FacultyID, sid))
		delete(idx.RoomSlot, roomKey(a.RoomID, sid))
		delete(idx.StudentGroupSlot, groupKey(a.StudentGroupID, sid))
	}
	if idx.RequirementCount[a.SessionRequirementID] > 0 {
		idx.RequirementCount[a.SessionRequirementID]--
	}
	delete(idx.byID, a.ID)
}

// HasFacultyConflict reports whether faculty is already scheduled at any slot.
func (idx *SolutionIndex) HasFacultyConflict(facultyID model.FacultyID, slotIDs []model.TimeSlotID) bool {
	_, ok := idx.FacultyConflict(facultyID, slotIDs)
	return ok
}

// FacultyConflict returns the first assignment using the faculty in any slot.
func (idx *SolutionIndex) FacultyConflict(facultyID model.FacultyID, slotIDs []model.TimeSlotID) (AssignmentID, bool) {
	for _, sid := range slotIDs {
		if id, ok := idx.FacultySlot[facultyKey(facultyID, sid)]; ok {
			return id, true
		}
	}
	return "", false
}

// HasRoomConflict reports whether room is already scheduled at any slot.
func (idx *SolutionIndex) HasRoomConflict(roomID model.RoomID, slotIDs []model.TimeSlotID) bool {
	_, ok := idx.RoomConflict(roomID, slotIDs)
	return ok
}

// RoomConflict returns the first assignment using the room in any slot.
func (idx *SolutionIndex) RoomConflict(roomID model.RoomID, slotIDs []model.TimeSlotID) (AssignmentID, bool) {
	for _, sid := range slotIDs {
		if id, ok := idx.RoomSlot[roomKey(roomID, sid)]; ok {
			return id, true
		}
	}
	return "", false
}

// HasStudentGroupConflict reports whether group is already scheduled at any slot.
func (idx *SolutionIndex) HasStudentGroupConflict(groupID model.StudentGroupID, slotIDs []model.TimeSlotID) bool {
	_, ok := idx.StudentGroupConflict(groupID, slotIDs)
	return ok
}

// StudentGroupConflict returns the first assignment using the group in any slot.
func (idx *SolutionIndex) StudentGroupConflict(groupID model.StudentGroupID, slotIDs []model.TimeSlotID) (AssignmentID, bool) {
	for _, sid := range slotIDs {
		if id, ok := idx.StudentGroupSlot[groupKey(groupID, sid)]; ok {
			return id, true
		}
	}
	return "", false
}

// ScheduledCount returns how many sessions are scheduled for a requirement.
func (idx *SolutionIndex) ScheduledCount(requirementID model.SessionRequirementID) int {
	return idx.RequirementCount[requirementID]
}

// AssignmentByID returns an indexed assignment.
func (idx *SolutionIndex) AssignmentByID(id AssignmentID) (Assignment, bool) {
	a, ok := idx.byID[id]
	return a, ok
}
