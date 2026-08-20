package localsearch

import (
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
	groupID  model.StudentGroupID
	duration int
}

type groupDayKey struct {
	groupID model.StudentGroupID
	day     time.Weekday
}

// IncrementalScoreEvaluator maintains group-day period occupancy and computes StudentGapPenalty delta incrementally.
type IncrementalScoreEvaluator struct {
	schedules      map[model.StudentGroupID]*[7]DaySchedule
	assignmentMeta map[problem.AssignmentID]assignmentMeta
	periodsPerDay  int
	totalGaps      int
	config         scorer.ObjectiveConfig
	weight         int
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

// NewIncrementalScoreEvaluator initializes an incremental evaluator with default single-objective weight 1.
func NewIncrementalScoreEvaluator(p *problem.Problem, solution *problem.Solution) *IncrementalScoreEvaluator {
	return NewIncrementalScoreEvaluatorWithConfig(p, solution, scorer.DefaultObjectiveConfig())
}

// NewIncrementalScoreEvaluatorWithConfig initializes an incremental evaluator with a specific ObjectiveConfig.
func NewIncrementalScoreEvaluatorWithConfig(p *problem.Problem, solution *problem.Solution, cfg scorer.ObjectiveConfig) *IncrementalScoreEvaluator {
	weight := cfg.GetWeight(scorer.ObjectiveStudentGapPenalty)
	eval := &IncrementalScoreEvaluator{
		schedules:      make(map[model.StudentGroupID]*[7]DaySchedule),
		assignmentMeta: make(map[problem.AssignmentID]assignmentMeta),
		periodsPerDay:  p.PeriodsPerDay,
		config:         cfg,
		weight:         weight,
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

// Rebuild resets and synchronizes the evaluator state with the given solution.
func (e *IncrementalScoreEvaluator) Rebuild(p *problem.Problem, solution *problem.Solution) {
	e.schedules = make(map[model.StudentGroupID]*[7]DaySchedule)
	e.assignmentMeta = make(map[problem.AssignmentID]assignmentMeta)
	e.periodsPerDay = p.PeriodsPerDay
	e.totalGaps = 0

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
			groupID:  a.StudentGroupID,
			duration: duration,
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
	}

	// Compute initial gaps for each group and day
	for _, sched := range e.schedules {
		for d := 0; d < 7; d++ {
			sched[d].Gaps = CalculateDayGaps(sched[d].PeriodCounts)
			e.totalGaps += sched[d].Gaps
		}
	}
}

// Evaluate returns the current score breakdown for the tracked solution state.
func (e *IncrementalScoreEvaluator) Evaluate(p *problem.Problem, solution *problem.Solution) scorer.ScoreBreakdown {
	raw := e.totalGaps
	weighted := raw * e.weight
	return scorer.ScoreBreakdown{
		HardViolations:    0,
		SoftPenalty:       weighted,
		StudentGapPenalty: raw,
		Components: []scorer.ObjectiveComponentScore{
			{
				ID:            scorer.ObjectiveStudentGapPenalty,
				RawScore:      raw,
				Weight:        e.weight,
				WeightedScore: weighted,
			},
		},
	}
}

// TotalGaps returns the current total unweighted gap penalty.
func (e *IncrementalScoreEvaluator) TotalGaps() int {
	return e.totalGaps
}

// WeightedPenalty returns the current total weighted soft penalty.
func (e *IncrementalScoreEvaluator) WeightedPenalty() int {
	return e.totalGaps * e.weight
}

func (e *IncrementalScoreEvaluator) getMeta(p *problem.Problem, solution *problem.Solution, id problem.AssignmentID) (model.StudentGroupID, int, bool) {
	if meta, ok := e.assignmentMeta[id]; ok {
		return meta.groupID, meta.duration, true
	}
	for _, a := range solution.Assignments {
		if a.ID == id {
			duration := 1
			if req, ok := p.SessionRequirements[a.SessionRequirementID]; ok && req.Duration > 0 {
				duration = req.Duration
			}
			meta := assignmentMeta{groupID: a.StudentGroupID, duration: duration}
			e.assignmentMeta[id] = meta
			return meta.groupID, meta.duration, true
		}
	}
	return "", 0, false
}

// EvaluateCandidateMove computes the exact StudentGapPenalty that would result from applying cm, without permanently mutating state.
func (e *IncrementalScoreEvaluator) EvaluateCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) scorer.ScoreBreakdown {
	deltaRaw := e.calculateDelta(p, solution, cm, false)
	raw := e.totalGaps + deltaRaw
	weighted := raw * e.weight
	return scorer.ScoreBreakdown{
		HardViolations:    0,
		SoftPenalty:       weighted,
		StudentGapPenalty: raw,
	}
}

// ApplyCandidateMove permanently updates the evaluator state when a candidate move is accepted.
func (e *IncrementalScoreEvaluator) ApplyCandidateMove(p *problem.Problem, solution *problem.Solution, cm CandidateMove) {
	delta := e.calculateDelta(p, solution, cm, true)
	e.totalGaps += delta
}

// calculateDelta computes the change in gap penalty for affected group-day pairs.
// If apply is true, the changes to PeriodCounts and Gaps are made permanent.
func (e *IncrementalScoreEvaluator) calculateDelta(p *problem.Problem, solution *problem.Solution, cm CandidateMove, apply bool) int {
	g1, dur1, ok1 := e.getMeta(p, solution, cm.Assignment1)
	if !ok1 {
		return 0
	}

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
		var ok2 bool
		g2, dur2, ok2 = e.getMeta(p, solution, cm.Assignment2)
		if !ok2 {
			return 0
		}
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
