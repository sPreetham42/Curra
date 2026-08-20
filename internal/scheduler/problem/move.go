package problem

import (
	"errors"
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

var (
	ErrLockedAssignment   = errors.New("cannot move locked assignment")
	ErrAssignmentNotFound = errors.New("assignment not found in solution")
)

// Placement represents the room and time slot assigned to a session.
type Placement struct {
	RoomID     model.RoomID     `json:"roomId"`
	TimeSlotID model.TimeSlotID `json:"timeSlotId"`
}

// Move represents moving an assignment from one placement to another.
type Move struct {
	AssignmentID AssignmentID `json:"assignmentId"`
	From         Placement    `json:"from"`
	To           Placement    `json:"to"`
}

// IsLocked reports whether the given assignment ID belongs to a locked assignment.
func (p *Problem) IsLocked(id AssignmentID) bool {
	for _, locked := range p.LockedAssignments {
		if locked.ID == id {
			return true
		}
	}
	return false
}

// ApplyMove mutates solution and solution.Index in place to move an assignment to Move.To.
func (s *Solution) ApplyMove(p *Problem, move Move) error {
	if p.IsLocked(move.AssignmentID) {
		return ErrLockedAssignment
	}

	idx := -1
	for i := range s.Assignments {
		if s.Assignments[i].ID == move.AssignmentID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrAssignmentNotFound
	}

	current := s.Assignments[idx]
	if move.From.RoomID != "" && current.RoomID != move.From.RoomID {
		return fmt.Errorf("assignment room %s does not match move.From %s", current.RoomID, move.From.RoomID)
	}
	if move.From.TimeSlotID != "" && current.TimeSlotID != move.From.TimeSlotID {
		return fmt.Errorf("assignment time slot %s does not match move.From %s", current.TimeSlotID, move.From.TimeSlotID)
	}

	// 1. Remove old assignment from index
	s.Index.Remove(p, current)

	// 2. Update assignment to new placement
	updated := current
	updated.RoomID = move.To.RoomID
	updated.TimeSlotID = move.To.TimeSlotID
	s.Assignments[idx] = updated

	// 3. Index new placement
	indexAssignment(p, s, updated)

	return nil
}

// UndoMove mutates solution and solution.Index in place to restore an assignment back to Move.From.
func (s *Solution) UndoMove(p *Problem, move Move) error {
	idx := -1
	for i := range s.Assignments {
		if s.Assignments[i].ID == move.AssignmentID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrAssignmentNotFound
	}

	current := s.Assignments[idx]

	// 1. Clean up new placement from index
	unindexAssignment(p, s, current)

	// 2. Restore assignment back to move.From
	restored := current
	restored.RoomID = move.From.RoomID
	restored.TimeSlotID = move.From.TimeSlotID
	s.Assignments[idx] = restored

	// 3. Re-index at original placement
	indexAssignment(p, s, restored)

	return nil
}

// ApplySwap mutates solution and index in place to swap placements of two assignments.
func (s *Solution) ApplySwap(p *Problem, move1, move2 Move) error {
	if move1.AssignmentID == move2.AssignmentID {
		return errors.New("cannot swap assignment with itself")
	}
	if p.IsLocked(move1.AssignmentID) || p.IsLocked(move2.AssignmentID) {
		return ErrLockedAssignment
	}

	idx1 := -1
	idx2 := -1
	for i := range s.Assignments {
		if s.Assignments[i].ID == move1.AssignmentID {
			idx1 = i
		}
		if s.Assignments[i].ID == move2.AssignmentID {
			idx2 = i
		}
	}
	if idx1 == -1 || idx2 == -1 {
		return ErrAssignmentNotFound
	}

	a1 := s.Assignments[idx1]
	a2 := s.Assignments[idx2]

	if move1.From.RoomID != "" && a1.RoomID != move1.From.RoomID {
		return fmt.Errorf("assignment room %s does not match move1.From %s", a1.RoomID, move1.From.RoomID)
	}
	if move1.From.TimeSlotID != "" && a1.TimeSlotID != move1.From.TimeSlotID {
		return fmt.Errorf("assignment time slot %s does not match move1.From %s", a1.TimeSlotID, move1.From.TimeSlotID)
	}
	if move2.From.RoomID != "" && a2.RoomID != move2.From.RoomID {
		return fmt.Errorf("assignment room %s does not match move2.From %s", a2.RoomID, move2.From.RoomID)
	}
	if move2.From.TimeSlotID != "" && a2.TimeSlotID != move2.From.TimeSlotID {
		return fmt.Errorf("assignment time slot %s does not match move2.From %s", a2.TimeSlotID, move2.From.TimeSlotID)
	}

	// 1. Remove both assignments from index
	s.Index.Remove(p, a1)
	s.Index.Remove(p, a2)

	// 2. Update placements
	a1.RoomID = move1.To.RoomID
	a1.TimeSlotID = move1.To.TimeSlotID

	a2.RoomID = move2.To.RoomID
	a2.TimeSlotID = move2.To.TimeSlotID

	s.Assignments[idx1] = a1
	s.Assignments[idx2] = a2

	// 3. Re-index both assignments
	indexAssignment(p, s, a1)
	indexAssignment(p, s, a2)

	return nil
}

// UndoSwap mutates solution and index in place to restore placements of two swapped assignments.
func (s *Solution) UndoSwap(p *Problem, move1, move2 Move) error {
	if move1.AssignmentID == move2.AssignmentID {
		return errors.New("cannot swap assignment with itself")
	}
	idx1 := -1
	idx2 := -1
	for i := range s.Assignments {
		if s.Assignments[i].ID == move1.AssignmentID {
			idx1 = i
		}
		if s.Assignments[i].ID == move2.AssignmentID {
			idx2 = i
		}
	}
	if idx1 == -1 || idx2 == -1 {
		return ErrAssignmentNotFound
	}

	a1 := s.Assignments[idx1]
	a2 := s.Assignments[idx2]

	// 1. Clean up current placements from index
	unindexAssignment(p, s, a1)
	unindexAssignment(p, s, a2)

	// 2. Restore original placements
	a1.RoomID = move1.From.RoomID
	a1.TimeSlotID = move1.From.TimeSlotID

	a2.RoomID = move2.From.RoomID
	a2.TimeSlotID = move2.From.TimeSlotID

	s.Assignments[idx1] = a1
	s.Assignments[idx2] = a2

	// 3. Re-index at original placements
	indexAssignment(p, s, a1)
	indexAssignment(p, s, a2)

	return nil
}

func indexAssignment(p *Problem, s *Solution, updated Assignment) {
	slotIDs, ok := updated.OccupiedSlotIDs(p)
	if ok {
		for _, sid := range slotIDs {
			fKey := facultyKey(updated.FacultyID, sid)
			if _, exists := s.Index.FacultySlot[fKey]; !exists {
				s.Index.FacultySlot[fKey] = updated.ID
			}
			rKey := roomKey(updated.RoomID, sid)
			if _, exists := s.Index.RoomSlot[rKey]; !exists {
				s.Index.RoomSlot[rKey] = updated.ID
			}
			gKey := groupKey(updated.StudentGroupID, sid)
			if _, exists := s.Index.StudentGroupSlot[gKey]; !exists {
				s.Index.StudentGroupSlot[gKey] = updated.ID
			}
		}
	}
	s.Index.RequirementCount[updated.SessionRequirementID]++
	s.Index.byID[updated.ID] = updated
}

func unindexAssignment(p *Problem, s *Solution, current Assignment) {
	slotIDs, ok := current.OccupiedSlotIDs(p)
	if ok {
		for _, sid := range slotIDs {
			fKey := facultyKey(current.FacultyID, sid)
			if id, ok := s.Index.FacultySlot[fKey]; ok && id == current.ID {
				delete(s.Index.FacultySlot, fKey)
			}
			rKey := roomKey(current.RoomID, sid)
			if id, ok := s.Index.RoomSlot[rKey]; ok && id == current.ID {
				delete(s.Index.RoomSlot, rKey)
			}
			gKey := groupKey(current.StudentGroupID, sid)
			if id, ok := s.Index.StudentGroupSlot[gKey]; ok && id == current.ID {
				delete(s.Index.StudentGroupSlot, gKey)
			}
		}
	}
	if s.Index.RequirementCount[current.SessionRequirementID] > 0 {
		s.Index.RequirementCount[current.SessionRequirementID]--
	}
	delete(s.Index.byID, current.ID)
}
