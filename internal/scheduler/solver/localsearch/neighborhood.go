package localsearch

import (
	"math/rand"
	"sort"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
)

// NeighborhoodGenerator creates candidate single and swap moves.
type NeighborhoodGenerator struct{}

func NewNeighborhoodGenerator() NeighborhoodGenerator {
	return NeighborhoodGenerator{}
}

// GenerateNeighbors generates a bounded set of unique candidate moves for unlocked assignments.
func (g NeighborhoodGenerator) GenerateNeighbors(p *problem.Problem, solution *problem.Solution, rng *rand.Rand, maxCandidates int) []CandidateMove {
	if maxCandidates <= 0 {
		maxCandidates = 100
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	// 1. Gather unlocked assignments sorted by ID
	unlocked := make([]problem.Assignment, 0, len(solution.Assignments))
	for _, a := range solution.Assignments {
		if !p.IsLocked(a.ID) {
			unlocked = append(unlocked, a)
		}
	}
	sort.Slice(unlocked, func(i, j int) bool {
		return unlocked[i].ID < unlocked[j].ID
	})

	if len(unlocked) == 0 {
		return nil
	}

	// 2. Gather sorted room IDs and timeslot IDs
	roomIDs := make([]model.RoomID, 0, len(p.Rooms))
	for rid := range p.Rooms {
		roomIDs = append(roomIDs, rid)
	}
	sort.Slice(roomIDs, func(i, j int) bool {
		return roomIDs[i] < roomIDs[j]
	})

	slotIDs := make([]model.TimeSlotID, 0, len(p.TimeSlots))
	for sid := range p.TimeSlots {
		slotIDs = append(slotIDs, sid)
	}
	sort.Slice(slotIDs, func(i, j int) bool {
		return slotIDs[i] < slotIDs[j]
	})

	if len(roomIDs) == 0 || len(slotIDs) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var candidates []CandidateMove

	maxAttempts := maxCandidates * 10
	for attempt := 0; attempt < maxAttempts && len(candidates) < maxCandidates; attempt++ {
		isSwap := len(unlocked) >= 2 && rng.Intn(2) == 1

		if !isSwap {
			aIdx := rng.Intn(len(unlocked))
			a := unlocked[aIdx]

			rIdx := rng.Intn(len(roomIDs))
			targetRoom := roomIDs[rIdx]

			sIdx := rng.Intn(len(slotIDs))
			targetSlot := slotIDs[sIdx]

			if targetRoom == a.RoomID && targetSlot == a.TimeSlotID {
				continue
			}

			cm := SingleMove(a.ID,
				problem.Placement{RoomID: a.RoomID, TimeSlotID: a.TimeSlotID},
				problem.Placement{RoomID: targetRoom, TimeSlotID: targetSlot},
			)
			sig := cm.Signature()
			if !seen[sig] {
				seen[sig] = true
				candidates = append(candidates, cm)
			}
		} else {
			i1 := rng.Intn(len(unlocked))
			i2 := rng.Intn(len(unlocked))
			if i1 == i2 {
				continue
			}
			a1 := unlocked[i1]
			a2 := unlocked[i2]

			if a1.RoomID == a2.RoomID && a1.TimeSlotID == a2.TimeSlotID {
				continue
			}

			cm := SwapMove(
				a1.ID, problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID}, problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID},
				a2.ID, problem.Placement{RoomID: a2.RoomID, TimeSlotID: a2.TimeSlotID}, problem.Placement{RoomID: a1.RoomID, TimeSlotID: a1.TimeSlotID},
			)
			sig := cm.Signature()
			if !seen[sig] {
				seen[sig] = true
				candidates = append(candidates, cm)
			}
		}
	}

	return candidates
}
