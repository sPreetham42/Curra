package localsearch

import (
	"fmt"
	"sort"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

// CandidateScoreEvaluator is an interface for evaluators that can compute scores incrementally for candidate moves.
type CandidateScoreEvaluator interface {
	ScoreEvaluator
	EvaluateCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) scorer.ScoreBreakdown
}

// DaySchedule tracks the period occupancy counts and gap penalty for a single student group on a single weekday.
type DaySchedule struct {
	PeriodCounts []uint16
	Gaps         int
}

type assignmentMeta struct {
	groupID   model.StudentGroupID
	facultyID model.FacultyID
	duration  int
}

type groupDayKey struct {
	groupID model.StudentGroupID
	day     time.Weekday
}

// IncrementalScoreEvaluator maintains group-day period occupancy and computes StudentGapPenalty & FacultyPreference delta incrementally.
type sessionPlacement struct {
	id          problem.AssignmentID
	roomID      model.RoomID
	startPeriod int
	duration    int
}

// IncrementalScoreEvaluator maintains group-day period occupancy and computes StudentGapPenalty, FacultyPreference, & RoomChange delta incrementally.
type IncrementalScoreEvaluator struct {
	schedules              map[model.StudentGroupID]*[7]DaySchedule
	groupDaySessions       map[model.StudentGroupID]*[7][]sessionPlacement
	assignmentMeta         map[problem.AssignmentID]assignmentMeta
	prefIndex              map[model.FacultyID]map[model.TimeSlotID]int
	periodsPerDay          int
	totalGaps              int
	totalPrefPenalty       int
	totalRoomChangePenalty int
	config                 scorer.ObjectiveConfig
	gapWeight              int
	prefWeight             int
	rcWeight               int
}

// CalculateDayGaps computes the student gap penalty for a single group on a single day given its 1-indexed period occupancy counts.
func CalculateDayGaps(counts []uint16) int {
	first := -1
	last := -1
	occupiedCount := 0

	for p := 1; p < len(counts); p++ {
		if counts[p] > 0 {
			if first == -1 {
				first = p
			}
			last = p
			occupiedCount++
		}
	}

	if occupiedCount < 2 || first == -1 || first == last {
		return 0
	}

	return (last - first + 1) - occupiedCount
}

// NewIncrementalScoreEvaluator initializes an incremental evaluator with default objective weights.
func NewIncrementalScoreEvaluator(p *problem.Problem, solution *problem.Solution) *IncrementalScoreEvaluator {
	return NewIncrementalScoreEvaluatorWithConfig(p, solution, scorer.DefaultObjectiveConfig())
}

// NewIncrementalScoreEvaluatorWithConfig initializes an incremental evaluator with a specific ObjectiveConfig.
func NewIncrementalScoreEvaluatorWithConfig(p *problem.Problem, solution *problem.Solution, cfg scorer.ObjectiveConfig) *IncrementalScoreEvaluator {
	gapWeight := cfg.GetWeight(scorer.ObjectiveStudentGapPenalty)
	prefWeight := cfg.GetWeight(scorer.ObjectiveFacultyPreference)
	rcWeight := cfg.GetWeight(scorer.ObjectiveRoomChange)
	eval := &IncrementalScoreEvaluator{
		schedules:        make(map[model.StudentGroupID]*[7]DaySchedule),
		groupDaySessions: make(map[model.StudentGroupID]*[7][]sessionPlacement),
		assignmentMeta:   make(map[problem.AssignmentID]assignmentMeta),
		prefIndex:        make(map[model.FacultyID]map[model.TimeSlotID]int),
		periodsPerDay:    p.PeriodsPerDay,
		config:           cfg,
		gapWeight:        gapWeight,
		prefWeight:       prefWeight,
		rcWeight:         rcWeight,
	}
	eval.Rebuild(p, solution)
	return eval
}

func (e *IncrementalScoreEvaluator) getOrCreateSchedule(groupID model.StudentGroupID) *[7]DaySchedule {
	sched, ok := e.schedules[groupID]
	if !ok {
		sched = &[7]DaySchedule{}
		for d := 0; d < 7; d++ {
			sched[d].PeriodCounts = make([]uint16, e.periodsPerDay+1)
		}
		e.schedules[groupID] = sched
	}
	return sched
}

func (e *IncrementalScoreEvaluator) getOrCreateSessions(groupID model.StudentGroupID) *[7][]sessionPlacement {
	sessions, ok := e.groupDaySessions[groupID]
	if !ok {
		sessions = &[7][]sessionPlacement{}
		e.groupDaySessions[groupID] = sessions
	}
	return sessions
}

func calcDayRoomChanges(list []sessionPlacement) int {
	if len(list) < 2 {
		return 0
	}
	sorted := make([]sessionPlacement, len(list))
	copy(sorted, list)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].startPeriod != sorted[j].startPeriod {
			return sorted[i].startPeriod < sorted[j].startPeriod
		}
		if sorted[i].duration != sorted[j].duration {
			return sorted[i].duration < sorted[j].duration
		}
		if sorted[i].roomID != sorted[j].roomID {
			return sorted[i].roomID < sorted[j].roomID
		}
		return sorted[i].id < sorted[j].id
	})

	penalty := 0
	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i].roomID != sorted[i+1].roomID {
			penalty++
		}
	}
	return penalty
}

// Rebuild resets and synchronizes the evaluator state with the given solution.
func (e *IncrementalScoreEvaluator) Rebuild(p *problem.Problem, solution *problem.Solution) {
	e.schedules = make(map[model.StudentGroupID]*[7]DaySchedule)
	e.groupDaySessions = make(map[model.StudentGroupID]*[7][]sessionPlacement)
	e.assignmentMeta = make(map[problem.AssignmentID]assignmentMeta)
	e.prefIndex = make(map[model.FacultyID]map[model.TimeSlotID]int)
	e.periodsPerDay = p.PeriodsPerDay
	e.totalGaps = 0
	e.totalPrefPenalty = 0
	e.totalRoomChangePenalty = 0

	// Index faculty preferences
	for _, pref := range p.FacultyPreferences {
		if e.prefIndex[pref.FacultyID] == nil {
			e.prefIndex[pref.FacultyID] = make(map[model.TimeSlotID]int)
		}
		e.prefIndex[pref.FacultyID][pref.TimeSlotID] += pref.Weight
	}

	// Pre-populate all known student groups
	for g := range p.StudentGroups {
		e.getOrCreateSchedule(g)
	}

	if solution == nil {
		return
	}

	for _, a := range solution.Assignments {
		duration := 1
		if req, ok := p.SessionRequirements[a.SessionRequirementID]; ok && req.Duration > 0 {
			duration = req.Duration
		}
		e.assignmentMeta[a.ID] = assignmentMeta{
			groupID:   a.StudentGroupID,
			facultyID: a.FacultyID,
			duration:  duration,
		}

		sched := e.getOrCreateSchedule(a.StudentGroupID)
		slot, ok := p.TimeSlots[a.TimeSlotID]
		if !ok {
			continue
		}
		if slot.Period+duration-1 > e.periodsPerDay {
			continue
		}

		dayIdx := int(slot.Day)
		for i := 0; i < duration; i++ {
			period := slot.Period + i
			sched[dayIdx].PeriodCounts[period]++
		}

		// Track room placement for room change calculation
		groupSessions := e.getOrCreateSessions(a.StudentGroupID)
		groupSessions[dayIdx] = append(groupSessions[dayIdx], sessionPlacement{
			id:          a.ID,
			roomID:      a.RoomID,
			startPeriod: slot.Period,
			duration:    duration,
		})

		// Calculate preference penalty for current assignment
		if slotIDs, ok := a.OccupiedSlotIDs(p); ok {
			if slots, ok := e.prefIndex[a.FacultyID]; ok {
				for _, sid := range slotIDs {
					if w, ok := slots[sid]; ok {
						e.totalPrefPenalty += w
					}
				}
			}
		}
	}

	// Compute initial gaps for each group and day
	for _, sched := range e.schedules {
		for d := 0; d < 7; d++ {
			sched[d].Gaps = CalculateDayGaps(sched[d].PeriodCounts)
			e.totalGaps += sched[d].Gaps
		}
	}

	// Compute initial room change penalties for each group and day
	for _, groupSessions := range e.groupDaySessions {
		for d := 0; d < 7; d++ {
			sort.Slice(groupSessions[d], func(i, j int) bool {
				if groupSessions[d][i].startPeriod != groupSessions[d][j].startPeriod {
					return groupSessions[d][i].startPeriod < groupSessions[d][j].startPeriod
				}
				if groupSessions[d][i].duration != groupSessions[d][j].duration {
					return groupSessions[d][i].duration < groupSessions[d][j].duration
				}
				if groupSessions[d][i].roomID != groupSessions[d][j].roomID {
					return groupSessions[d][i].roomID < groupSessions[d][j].roomID
				}
				return groupSessions[d][i].id < groupSessions[d][j].id
			})
			e.totalRoomChangePenalty += calcDayRoomChanges(groupSessions[d])
		}
	}
}

// Evaluate returns the current score breakdown for the tracked solution state.
func (e *IncrementalScoreEvaluator) Evaluate(p *problem.Problem, solution *problem.Solution) scorer.ScoreBreakdown {
	rawGap := e.totalGaps
	weightedGap := rawGap * e.gapWeight
	rawPref := e.totalPrefPenalty
	weightedPref := rawPref * e.prefWeight
	rawRC := e.totalRoomChangePenalty
	weightedRC := rawRC * e.rcWeight
	totalSoft := weightedGap + weightedPref + weightedRC

	var components []scorer.ObjectiveComponentScore
	if e.gapWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveStudentGapPenalty,
			RawScore:      rawGap,
			Weight:        e.gapWeight,
			WeightedScore: weightedGap,
		})
	}
	if e.prefWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveFacultyPreference,
			RawScore:      rawPref,
			Weight:        e.prefWeight,
			WeightedScore: weightedPref,
		})
	}
	if e.rcWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveRoomChange,
			RawScore:      rawRC,
			Weight:        e.rcWeight,
			WeightedScore: weightedRC,
		})
	}

	return scorer.ScoreBreakdown{
		HardViolations:           0,
		SoftPenalty:              totalSoft,
		StudentGapPenalty:        rawGap,
		FacultyPreferencePenalty: rawPref,
		RoomChangePenalty:        rawRC,
		Components:               components,
	}
}

// TotalGaps returns the current total unweighted gap penalty.
func (e *IncrementalScoreEvaluator) TotalGaps() int {
	return e.totalGaps
}

// TotalPrefPenalty returns the current total unweighted faculty preference penalty.
func (e *IncrementalScoreEvaluator) TotalPrefPenalty() int {
	return e.totalPrefPenalty
}

// TotalRoomChangePenalty returns the current total unweighted room change penalty.
func (e *IncrementalScoreEvaluator) TotalRoomChangePenalty() int {
	return e.totalRoomChangePenalty
}

// WeightedPenalty returns the current total weighted soft penalty.
func (e *IncrementalScoreEvaluator) WeightedPenalty() int {
	return e.totalGaps*e.gapWeight + e.totalPrefPenalty*e.prefWeight + e.totalRoomChangePenalty*e.rcWeight
}

func (e *IncrementalScoreEvaluator) getMeta(p *problem.Problem, solution *problem.Solution, id problem.AssignmentID) (assignmentMeta, bool) {
	if meta, ok := e.assignmentMeta[id]; ok {
		return meta, true
	}
	for _, a := range solution.Assignments {
		if a.ID == id {
			duration := 1
			if req, ok := p.SessionRequirements[a.SessionRequirementID]; ok && req.Duration > 0 {
				duration = req.Duration
			}
			meta := assignmentMeta{groupID: a.StudentGroupID, facultyID: a.FacultyID, duration: duration}
			e.assignmentMeta[id] = meta
			return meta, true
		}
	}
	return assignmentMeta{}, false
}

func (e *IncrementalScoreEvaluator) getPlacementPrefPenalty(p *problem.Problem, facultyID model.FacultyID, startSlotID model.TimeSlotID, duration int) int {
	slots, ok := e.prefIndex[facultyID]
	if !ok {
		return 0
	}
	slotIDs, ok := p.OccupiedSlotIDs(startSlotID, duration)
	if !ok {
		return 0
	}
	penalty := 0
	for _, sid := range slotIDs {
		if w, ok := slots[sid]; ok {
			penalty += w
		}
	}
	return penalty
}

func (e *IncrementalScoreEvaluator) calculatePrefDelta(p *problem.Problem, solution *problem.Solution, cm CandidateMove) int {
	meta1, ok1 := e.getMeta(p, solution, cm.Assignment1)
	if !ok1 {
		return 0
	}

	oldPref1 := e.getPlacementPrefPenalty(p, meta1.facultyID, cm.From1.TimeSlotID, meta1.duration)
	newPref1 := e.getPlacementPrefPenalty(p, meta1.facultyID, cm.To1.TimeSlotID, meta1.duration)
	delta := newPref1 - oldPref1

	if cm.Kind == MoveKindSwap {
		meta2, ok2 := e.getMeta(p, solution, cm.Assignment2)
		if ok2 {
			oldPref2 := e.getPlacementPrefPenalty(p, meta2.facultyID, cm.From2.TimeSlotID, meta2.duration)
			newPref2 := e.getPlacementPrefPenalty(p, meta2.facultyID, cm.To2.TimeSlotID, meta2.duration)
			delta += (newPref2 - oldPref2)
		}
	}

	return delta
}

type keyGD struct {
	groupID model.StudentGroupID
	day     int
}

func (e *IncrementalScoreEvaluator) calculateRoomChangeDelta(p *problem.Problem, solution *problem.Solution, cm CandidateMove, apply bool) int {
	meta1, ok1 := e.getMeta(p, solution, cm.Assignment1)
	if !ok1 {
		return 0
	}

	fromSlot1, hasFrom1 := p.TimeSlots[cm.From1.TimeSlotID]
	toSlot1, hasTo1 := p.TimeSlots[cm.To1.TimeSlotID]

	validFrom1 := hasFrom1 && (fromSlot1.Period+meta1.duration-1 <= e.periodsPerDay)
	validTo1 := hasTo1 && (toSlot1.Period+meta1.duration-1 <= e.periodsPerDay)

	affectedKeys := make(map[keyGD]struct{})
	if validFrom1 {
		affectedKeys[keyGD{groupID: meta1.groupID, day: int(fromSlot1.Day)}] = struct{}{}
	}
	if validTo1 {
		affectedKeys[keyGD{groupID: meta1.groupID, day: int(toSlot1.Day)}] = struct{}{}
	}

	var meta2 assignmentMeta
	var fromSlot2, toSlot2 model.TimeSlot
	var hasFrom2, hasTo2 bool
	var validFrom2, validTo2 bool

	if cm.Kind == MoveKindSwap {
		meta2, _ = e.getMeta(p, solution, cm.Assignment2)
		fromSlot2, hasFrom2 = p.TimeSlots[cm.From2.TimeSlotID]
		toSlot2, hasTo2 = p.TimeSlots[cm.To2.TimeSlotID]
		validFrom2 = hasFrom2 && (fromSlot2.Period+meta2.duration-1 <= e.periodsPerDay)
		validTo2 = hasTo2 && (toSlot2.Period+meta2.duration-1 <= e.periodsPerDay)

		if validFrom2 {
			affectedKeys[keyGD{groupID: meta2.groupID, day: int(fromSlot2.Day)}] = struct{}{}
		}
		if validTo2 {
			affectedKeys[keyGD{groupID: meta2.groupID, day: int(toSlot2.Day)}] = struct{}{}
		}
	}

	totalDelta := 0
	updatedLists := make(map[keyGD][]sessionPlacement)

	for kgd := range affectedKeys {
		currentList := e.getOrCreateSessions(kgd.groupID)[kgd.day]
		oldPenalty := calcDayRoomChanges(currentList)

		newList := make([]sessionPlacement, 0, len(currentList)+1)
		found1 := false
		found2 := false
		for _, sp := range currentList {
			if sp.id == cm.Assignment1 {
				found1 = true
				continue
			}
			if cm.Kind == MoveKindSwap && sp.id == cm.Assignment2 {
				found2 = true
				continue
			}
			newList = append(newList, sp)
		}
		if validFrom1 && meta1.groupID == kgd.groupID && int(fromSlot1.Day) == kgd.day && !found1 {
			panic(fmt.Sprintf("BUG: Assignment1 %s expected on day %d of group %s but NOT found in groupDaySessions!", cm.Assignment1, kgd.day, kgd.groupID))
		}
		if cm.Kind == MoveKindSwap && validFrom2 && meta2.groupID == kgd.groupID && int(fromSlot2.Day) == kgd.day && !found2 {
			panic(fmt.Sprintf("BUG: Assignment2 %s expected on day %d of group %s but NOT found in groupDaySessions!", cm.Assignment2, kgd.day, kgd.groupID))
		}

		if validTo1 && meta1.groupID == kgd.groupID && int(toSlot1.Day) == kgd.day {
			newList = append(newList, sessionPlacement{
				id:          cm.Assignment1,
				roomID:      cm.To1.RoomID,
				startPeriod: toSlot1.Period,
				duration:    meta1.duration,
			})
		}

		if cm.Kind == MoveKindSwap && validTo2 && meta2.groupID == kgd.groupID && int(toSlot2.Day) == kgd.day {
			newList = append(newList, sessionPlacement{
				id:          cm.Assignment2,
				roomID:      cm.To2.RoomID,
				startPeriod: toSlot2.Period,
				duration:    meta2.duration,
			})
		}

		newPenalty := calcDayRoomChanges(newList)
		totalDelta += (newPenalty - oldPenalty)

		if apply {
			sort.Slice(newList, func(i, j int) bool {
				if newList[i].startPeriod != newList[j].startPeriod {
					return newList[i].startPeriod < newList[j].startPeriod
				}
				if newList[i].duration != newList[j].duration {
					return newList[i].duration < newList[j].duration
				}
				if newList[i].roomID != newList[j].roomID {
					return newList[i].roomID < newList[j].roomID
				}
				return newList[i].id < newList[j].id
			})
			updatedLists[kgd] = newList
		}
	}

	if apply {
		for kgd, newList := range updatedLists {
			e.groupDaySessions[kgd.groupID][kgd.day] = newList
		}
	}

	return totalDelta
}

// EvaluateCandidateMove computes the exact ScoreBreakdown that would result from applying cm, without permanently mutating state.
func (e *IncrementalScoreEvaluator) EvaluateCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) scorer.ScoreBreakdown {
	deltaGap := e.calculateDelta(p, solution, cm, false)
	deltaPref := e.calculatePrefDelta(p, solution, cm)
	deltaRC := e.calculateRoomChangeDelta(p, solution, cm, false)

	rawGap := e.totalGaps + deltaGap
	weightedGap := rawGap * e.gapWeight
	rawPref := e.totalPrefPenalty + deltaPref
	weightedPref := rawPref * e.prefWeight
	rawRC := e.totalRoomChangePenalty + deltaRC
	weightedRC := rawRC * e.rcWeight
	totalSoft := weightedGap + weightedPref + weightedRC

	var components []scorer.ObjectiveComponentScore
	if e.gapWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveStudentGapPenalty,
			RawScore:      rawGap,
			Weight:        e.gapWeight,
			WeightedScore: weightedGap,
		})
	}
	if e.prefWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveFacultyPreference,
			RawScore:      rawPref,
			Weight:        e.prefWeight,
			WeightedScore: weightedPref,
		})
	}
	if e.rcWeight > 0 {
		components = append(components, scorer.ObjectiveComponentScore{
			ID:            scorer.ObjectiveRoomChange,
			RawScore:      rawRC,
			Weight:        e.rcWeight,
			WeightedScore: weightedRC,
		})
	}

	return scorer.ScoreBreakdown{
		HardViolations:           0,
		SoftPenalty:              totalSoft,
		StudentGapPenalty:        rawGap,
		FacultyPreferencePenalty: rawPref,
		RoomChangePenalty:        rawRC,
		Components:               components,
	}
}

// ApplyCandidateMove permanently updates the evaluator state when a candidate move is accepted.
func (e *IncrementalScoreEvaluator) ApplyCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) {
	deltaGap := e.calculateDelta(p, solution, cm, true)
	deltaPref := e.calculatePrefDelta(p, solution, cm)
	deltaRC := e.calculateRoomChangeDelta(p, solution, cm, true)
	e.totalGaps += deltaGap
	e.totalPrefPenalty += deltaPref
	e.totalRoomChangePenalty += deltaRC
}

// calculateDelta computes the change in gap penalty for affected group-day pairs.
// If apply is true, the changes to PeriodCounts and Gaps are made permanent.
func (e *IncrementalScoreEvaluator) calculateDelta(p *problem.Problem, solution *problem.Solution, cm CandidateMove, apply bool) int {
	meta1, ok1 := e.getMeta(p, solution, cm.Assignment1)
	if !ok1 {
		return 0
	}
	g1, dur1 := meta1.groupID, meta1.duration

	fromSlot1, hasFrom1 := p.TimeSlots[cm.From1.TimeSlotID]
	toSlot1, hasTo1 := p.TimeSlots[cm.To1.TimeSlotID]
	validFrom1 := hasFrom1 && (fromSlot1.Period+dur1-1 <= e.periodsPerDay)
	validTo1 := hasTo1 && (toSlot1.Period+dur1-1 <= e.periodsPerDay)

	var (
		g2         model.StudentGroupID
		dur2       int
		fromSlot2  model.TimeSlot
		toSlot2    model.TimeSlot
		validFrom2 bool
		validTo2   bool
		isSwap     = cm.Kind == MoveKindSwap
	)

	if isSwap {
		meta2, ok2 := e.getMeta(p, solution, cm.Assignment2)
		if !ok2 {
			return 0
		}
		g2, dur2 = meta2.groupID, meta2.duration
		var hasFrom2, hasTo2 bool
		fromSlot2, hasFrom2 = p.TimeSlots[cm.From2.TimeSlotID]
		toSlot2, hasTo2 = p.TimeSlots[cm.To2.TimeSlotID]
		validFrom2 = hasFrom2 && (fromSlot2.Period+dur2-1 <= e.periodsPerDay)
		validTo2 = hasTo2 && (toSlot2.Period+dur2-1 <= e.periodsPerDay)
	}

	// Collect unique affected (group, day) pairs (at most 4)
	var affectedKeys [4]groupDayKey
	numKeys := 0

	if !isSwap {
		if validFrom1 && validTo1 {
			affectedKeys[0] = groupDayKey{groupID: g1, day: fromSlot1.Day}
			if fromSlot1.Day == toSlot1.Day {
				numKeys = 1
			} else {
				affectedKeys[1] = groupDayKey{groupID: g1, day: toSlot1.Day}
				numKeys = 2
			}
		} else if validFrom1 {
			affectedKeys[0] = groupDayKey{groupID: g1, day: fromSlot1.Day}
			numKeys = 1
		} else if validTo1 {
			affectedKeys[0] = groupDayKey{groupID: g1, day: toSlot1.Day}
			numKeys = 1
		}
	} else {
		addKey := func(g model.StudentGroupID, d time.Weekday) {
			for i := 0; i < numKeys; i++ {
				if affectedKeys[i].groupID == g && affectedKeys[i].day == d {
					return
				}
			}
			affectedKeys[numKeys] = groupDayKey{groupID: g, day: d}
			numKeys++
		}

		if validFrom1 {
			addKey(g1, fromSlot1.Day)
		}
		if validTo1 {
			addKey(g1, toSlot1.Day)
		}
		if validFrom2 {
			addKey(g2, fromSlot2.Day)
		}
		if validTo2 {
			addKey(g2, toSlot2.Day)
		}
	}

	delta := 0

	for k := 0; k < numKeys; k++ {
		key := affectedKeys[k]
		sched := e.getOrCreateSchedule(key.groupID)
		dayIdx := int(key.day)
		oldGaps := sched[dayIdx].Gaps

		// 1. Apply movements to PeriodCounts
		// Movement 1: Assignment1
		if key.groupID == g1 {
			if validFrom1 && fromSlot1.Day == key.day {
				for i := 0; i < dur1; i++ {
					period := fromSlot1.Period + i
					if sched[dayIdx].PeriodCounts[period] > 0 {
						sched[dayIdx].PeriodCounts[period]--
					}
				}
			}
			if validTo1 && toSlot1.Day == key.day {
				for i := 0; i < dur1; i++ {
					period := toSlot1.Period + i
					sched[dayIdx].PeriodCounts[period]++
				}
			}
		}

		// Movement 2: Assignment2 (if swap)
		if isSwap && key.groupID == g2 {
			if validFrom2 && fromSlot2.Day == key.day {
				for i := 0; i < dur2; i++ {
					period := fromSlot2.Period + i
					if sched[dayIdx].PeriodCounts[period] > 0 {
						sched[dayIdx].PeriodCounts[period]--
					}
				}
			}
			if validTo2 && toSlot2.Day == key.day {
				for i := 0; i < dur2; i++ {
					period := toSlot2.Period + i
					sched[dayIdx].PeriodCounts[period]++
				}
			}
		}

		newGaps := CalculateDayGaps(sched[dayIdx].PeriodCounts)
		delta += (newGaps - oldGaps)

		if apply {
			sched[dayIdx].Gaps = newGaps
		} else {
			// Revert PeriodCounts back to original state
			if key.groupID == g1 {
				if validFrom1 && fromSlot1.Day == key.day {
					for i := 0; i < dur1; i++ {
						period := fromSlot1.Period + i
						sched[dayIdx].PeriodCounts[period]++
					}
				}
				if validTo1 && toSlot1.Day == key.day {
					for i := 0; i < dur1; i++ {
						period := toSlot1.Period + i
						if sched[dayIdx].PeriodCounts[period] > 0 {
							sched[dayIdx].PeriodCounts[period]--
						}
					}
				}
			}

			if isSwap && key.groupID == g2 {
				if validFrom2 && fromSlot2.Day == key.day {
					for i := 0; i < dur2; i++ {
						period := fromSlot2.Period + i
						sched[dayIdx].PeriodCounts[period]++
					}
				}
				if validTo2 && toSlot2.Day == key.day {
					for i := 0; i < dur2; i++ {
						period := toSlot2.Period + i
						if sched[dayIdx].PeriodCounts[period] > 0 {
							sched[dayIdx].PeriodCounts[period]--
						}
					}
				}
			}
		}
	}

	return delta
}
