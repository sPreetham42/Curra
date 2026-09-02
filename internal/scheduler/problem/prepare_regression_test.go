package problem_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/testutil"
)

// derivedState is the full set of maps Prepare() derives from the authoritative
// source collections.
type derivedState struct {
	slots            map[model.SlotKey]model.TimeSlotID
	facultyAvailable map[model.FacultyID]map[model.TimeSlotID]struct{}
	roomAvailable    map[model.RoomID]map[model.TimeSlotID]struct{}
	groupOverlaps    map[model.StudentGroupID]map[model.StudentGroupID]struct{}
}

func captureDerivedState(p *problem.Problem) derivedState {
	return derivedState{
		slots:            p.SlotsByDayPeriod,
		facultyAvailable: p.FacultyAvailable,
		roomAvailable:    p.RoomAvailable,
		groupOverlaps:    p.StudentGroupOverlaps,
	}
}

// TestPrepare_AvailabilityAddedAfterPrepare_IsIndexed proves availability appended
// after an earlier Prepare() is picked up, rather than being masked by the
// previously built allow-list index.
func TestPrepare_AvailabilityAddedAfterPrepare_IsIndexed(t *testing.T) {
	p := testutil.FeasibleProblem() // already Prepare()d by the fixture

	p.TimeSlots["tue-3"] = model.TimeSlot{ID: "tue-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"}
	p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{FacultyID: "faculty-a", TimeSlotID: "tue-3"})
	p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: "room-lecture", TimeSlotID: "tue-3"})

	p.Prepare()

	if !p.IsFacultyAvailable("faculty-a", []model.TimeSlotID{"tue-3"}) {
		t.Fatal("faculty availability added after a previous Prepare was not indexed")
	}
	if !p.IsRoomAvailable("room-lecture", []model.TimeSlotID{"tue-3"}) {
		t.Fatal("room availability added after a previous Prepare was not indexed")
	}
	if got, ok := p.SlotsByDayPeriod[model.SlotKey{Day: time.Tuesday, Period: 3}]; !ok || got != "tue-3" {
		t.Fatalf("slot added after a previous Prepare was not indexed: got %q ok=%v", got, ok)
	}
}

// TestPrepare_AvailabilityRemovedAfterPrepare_IsDropped proves a revoked
// availability record does not survive in the derived allow-list index.
func TestPrepare_AvailabilityRemovedAfterPrepare_IsDropped(t *testing.T) {
	p := testutil.FeasibleProblem()

	if !p.IsFacultyAvailable("faculty-a", []model.TimeSlotID{"tue-2"}) {
		t.Fatal("fixture precondition changed: faculty-a should start available at tue-2")
	}

	remaining := make([]model.FacultyAvailability, 0, len(p.FacultyAvailabilities))
	for _, a := range p.FacultyAvailabilities {
		if a.FacultyID == "faculty-a" && a.TimeSlotID == "tue-2" {
			continue
		}
		remaining = append(remaining, a)
	}
	p.FacultyAvailabilities = remaining

	remainingRooms := make([]model.RoomAvailability, 0, len(p.RoomAvailabilities))
	for _, a := range p.RoomAvailabilities {
		if a.RoomID == "room-lecture" && a.TimeSlotID == "tue-2" {
			continue
		}
		remainingRooms = append(remainingRooms, a)
	}
	p.RoomAvailabilities = remainingRooms

	p.Prepare()

	if p.IsFacultyAvailable("faculty-a", []model.TimeSlotID{"tue-2"}) {
		t.Fatal("stale faculty availability survived a repeated Prepare")
	}
	if p.IsRoomAvailable("room-lecture", []model.TimeSlotID{"tue-2"}) {
		t.Fatal("stale room availability survived a repeated Prepare")
	}
}

// TestPrepare_SlotMovedAfterPrepare_DropsStaleGridEntry proves that changing a
// slot's day/period does not leave the old (Day, Period) coordinate resolvable,
// which would otherwise let OccupiedSlotIDs expand a session onto a grid position
// no slot occupies any more.
func TestPrepare_SlotMovedAfterPrepare_DropsStaleGridEntry(t *testing.T) {
	p := testutil.FeasibleProblem()

	if got := p.SlotsByDayPeriod[model.SlotKey{Day: time.Monday, Period: 3}]; got != "mon-3" {
		t.Fatalf("fixture precondition changed: Monday P3 = %q, want mon-3", got)
	}

	// Move mon-3 to Tuesday period 3.
	p.TimeSlots["mon-3"] = model.TimeSlot{ID: "mon-3", Day: time.Tuesday, Period: 3, Label: "moved"}
	p.Prepare()

	if _, ok := p.SlotsByDayPeriod[model.SlotKey{Day: time.Monday, Period: 3}]; ok {
		t.Fatal("stale Monday P3 grid entry survived after the slot moved to Tuesday")
	}
	if got := p.SlotsByDayPeriod[model.SlotKey{Day: time.Tuesday, Period: 3}]; got != "mon-3" {
		t.Fatalf("moved slot not indexed at its new coordinate: got %q", got)
	}
}

// TestPrepare_SlotRemovedAfterPrepare_DropsStaleGridEntry covers outright removal.
func TestPrepare_SlotRemovedAfterPrepare_DropsStaleGridEntry(t *testing.T) {
	p := testutil.FeasibleProblem()

	delete(p.TimeSlots, "mon-3")
	p.Prepare()

	if _, ok := p.SlotsByDayPeriod[model.SlotKey{Day: time.Monday, Period: 3}]; ok {
		t.Fatal("removed slot left a stale grid entry in SlotsByDayPeriod")
	}
	if _, ok := p.OccupiedSlotIDs("mon-2", 2); ok {
		t.Fatal("a session must not expand onto a grid position whose slot was removed")
	}
}

// TestPrepare_RepeatedPrepare_EquivalentToFreshPrepare proves Prepare() is
// idempotent and that repeated preparation of a mutated problem lands in exactly
// the state a from-scratch preparation of the same source data produces.
func TestPrepare_RepeatedPrepare_EquivalentToFreshPrepare(t *testing.T) {
	mutate := func(p *problem.Problem) {
		p.TimeSlots["tue-3"] = model.TimeSlot{ID: "tue-3", Day: time.Tuesday, Period: 3, Label: "Tue P3"}
		delete(p.TimeSlots, "mon-3")
		p.FacultyAvailabilities = append(p.FacultyAvailabilities, model.FacultyAvailability{FacultyID: "faculty-a", TimeSlotID: "tue-3"})
		p.RoomAvailabilities = append(p.RoomAvailabilities, model.RoomAvailability{RoomID: "room-lab", TimeSlotID: "tue-3"})
		p.Classes["class-a"] = model.Class{
			ID:              "class-a",
			ProgramID:       "program-a",
			Name:            "A",
			WholeGroupID:    "group-a",
			StudentGroupIDs: []model.StudentGroupID{"group-a", "group-a-lab"},
		}
		p.StudentGroups["group-a-lab"] = model.StudentGroup{ID: "group-a-lab", ClassID: "class-a", Name: "A lab", Size: 15}
	}

	// Mutated after an earlier Prepare, then prepared repeatedly.
	reused := testutil.FeasibleProblem()
	mutate(&reused)
	reused.Prepare()
	afterFirst := captureDerivedState(&reused)
	reused.Prepare()
	reused.Prepare()
	afterRepeat := captureDerivedState(&reused)

	// Same source data, prepared exactly once from an unprepared problem.
	fresh := testutil.FeasibleProblem()
	mutate(&fresh)
	fresh.SlotsByDayPeriod = nil
	fresh.FacultyAvailable = nil
	fresh.RoomAvailable = nil
	fresh.StudentGroupOverlaps = nil
	fresh.Prepare()
	freshState := captureDerivedState(&fresh)

	if !reflect.DeepEqual(afterFirst, afterRepeat) {
		t.Fatalf("Prepare is not idempotent:\nfirst:  %+v\nrepeat: %+v", afterFirst, afterRepeat)
	}
	if !reflect.DeepEqual(afterRepeat, freshState) {
		t.Fatalf("repeated Prepare diverges from a from-scratch Prepare:\nrepeated: %+v\nfresh:    %+v", afterRepeat, freshState)
	}
}

// TestPrepare_SlotIndexDeterministicUnderDuplicateGridKeys proves the derived slot
// index does not depend on Go map iteration order even for the duplicate
// (Day, Period) inputs that Validate rejects.
func TestPrepare_SlotIndexDeterministicUnderDuplicateGridKeys(t *testing.T) {
	build := func() model.TimeSlotID {
		p := testutil.FeasibleProblem()
		p.TimeSlots["aaa-dup"] = model.TimeSlot{ID: "aaa-dup", Day: time.Friday, Period: 1, Label: "dup a"}
		p.TimeSlots["zzz-dup"] = model.TimeSlot{ID: "zzz-dup", Day: time.Friday, Period: 1, Label: "dup z"}
		p.Prepare()
		return p.SlotsByDayPeriod[model.SlotKey{Day: time.Friday, Period: 1}]
	}

	want := build()
	for i := 0; i < 50; i++ {
		if got := build(); got != want {
			t.Fatalf("slot index winner is not deterministic: got %q, want %q", got, want)
		}
	}
}
