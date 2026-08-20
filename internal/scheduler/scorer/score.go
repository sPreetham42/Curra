package scorer

import (
	"sort"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

// ObjectiveID identifies a soft constraint objective.
type ObjectiveID string

const (
	// ObjectiveStudentGapPenalty identifies the student gap penalty objective.
	ObjectiveStudentGapPenalty ObjectiveID = "StudentGapPenalty"
)

// ObjectiveComponent specifies a soft objective and its integer weight.
type ObjectiveComponent struct {
	ID     ObjectiveID `json:"id"`
	Weight int         `json:"weight"`
}

// ObjectiveConfig defines the configured soft objective components and their weights.
type ObjectiveConfig struct {
	Components []ObjectiveComponent `json:"components"`
}

// DefaultObjectiveConfig returns the default single-objective configuration with StudentGapPenalty weight = 1.
func DefaultObjectiveConfig() ObjectiveConfig {
	return ObjectiveConfig{
		Components: []ObjectiveComponent{
			{ID: ObjectiveStudentGapPenalty, Weight: 1},
		},
	}
}

// GetWeight returns the weight for the given objective ID, defaulting to 1 for StudentGapPenalty if unspecified.
func (cfg ObjectiveConfig) GetWeight(id ObjectiveID) int {
	for _, c := range cfg.Components {
		if c.ID == id {
			return c.Weight
		}
	}
	if id == ObjectiveStudentGapPenalty {
		return 1
	}
	return 0
}

// ObjectiveComponentScore records the raw and weighted contribution of a specific objective.
type ObjectiveComponentScore struct {
	ID            ObjectiveID `json:"id"`
	RawScore      int         `json:"rawScore"`
	Weight        int         `json:"weight"`
	WeightedScore int         `json:"weightedScore"`
}

// Score records feasibility and soft-constraint penalties.
type Score struct {
	HardViolations int            `json:"hardViolations"`
	SoftPenalty    int            `json:"softPenalty"`
	Breakdown      ScoreBreakdown `json:"breakdown,omitempty"`
}

// GroupDayGap records gap details for a specific group on a specific day.
type GroupDayGap struct {
	StudentGroupID model.StudentGroupID `json:"studentGroupId"`
	Day            time.Weekday         `json:"day"`
	Gaps           int                  `json:"gaps"`
	FirstPeriod    int                  `json:"firstPeriod"`
	LastPeriod     int                  `json:"lastPeriod"`
}

// ScoreBreakdown provides a detailed breakdown of soft penalties and objective components.
type ScoreBreakdown struct {
	HardViolations    int                       `json:"hardViolations"`
	SoftPenalty       int                       `json:"softPenalty"`
	StudentGapPenalty int                       `json:"studentGapPenalty"`
	GroupGaps         map[model.StudentGroupID]int `json:"groupGaps,omitempty"`
	Details           []GroupDayGap             `json:"details,omitempty"`
	Components        []ObjectiveComponentScore `json:"components,omitempty"`
}

// OccupiedPeriod represents an occupied period on a specific day for a student group.
type OccupiedPeriod struct {
	StudentGroupID model.StudentGroupID
	Day            time.Weekday
	Period         int
}

// CalculateStudentGapPenalty calculates the gap penalty with default weight 1.
func CalculateStudentGapPenalty(groups []model.StudentGroupID, occupied []OccupiedPeriod) ScoreBreakdown {
	return CalculateStudentGapPenaltyWithConfig(groups, occupied, DefaultObjectiveConfig())
}

// CalculateStudentGapPenaltyWithConfig calculates the gap penalty and scales it by the configured weight.
func CalculateStudentGapPenaltyWithConfig(groups []model.StudentGroupID, occupied []OccupiedPeriod, cfg ObjectiveConfig) ScoreBreakdown {
	groupMap := make(map[model.StudentGroupID]map[time.Weekday]map[int]struct{})
	for _, g := range groups {
		groupMap[g] = make(map[time.Weekday]map[int]struct{})
	}

	for _, occ := range occupied {
		if groupMap[occ.StudentGroupID] == nil {
			groupMap[occ.StudentGroupID] = make(map[time.Weekday]map[int]struct{})
		}
		if groupMap[occ.StudentGroupID][occ.Day] == nil {
			groupMap[occ.StudentGroupID][occ.Day] = make(map[int]struct{})
		}
		groupMap[occ.StudentGroupID][occ.Day][occ.Period] = struct{}{}
	}

	sortedGroups := make([]model.StudentGroupID, 0, len(groupMap))
	for g := range groupMap {
		sortedGroups = append(sortedGroups, g)
	}
	sort.Slice(sortedGroups, func(i, j int) bool {
		return sortedGroups[i] < sortedGroups[j]
	})

	weekdays := []time.Weekday{
		time.Monday,
		time.Tuesday,
		time.Wednesday,
		time.Thursday,
		time.Friday,
		time.Saturday,
		time.Sunday,
	}

	totalGaps := 0
	groupGaps := make(map[model.StudentGroupID]int)
	var details []GroupDayGap

	for _, g := range sortedGroups {
		daysMap := groupMap[g]
		for _, day := range weekdays {
			periodSet := daysMap[day]
			if len(periodSet) < 2 {
				continue
			}

			periods := make([]int, 0, len(periodSet))
			for p := range periodSet {
				periods = append(periods, p)
			}
			sort.Ints(periods)

			first := periods[0]
			last := periods[len(periods)-1]

			dayGaps := 0
			for p := first + 1; p < last; p++ {
				if _, ok := periodSet[p]; !ok {
					dayGaps++
				}
			}

			if dayGaps > 0 {
				totalGaps += dayGaps
				groupGaps[g] += dayGaps
				details = append(details, GroupDayGap{
					StudentGroupID: g,
					Day:            day,
					Gaps:           dayGaps,
					FirstPeriod:    first,
					LastPeriod:     last,
				})
			}
		}
	}

	weight := cfg.GetWeight(ObjectiveStudentGapPenalty)
	weightedPenalty := totalGaps * weight

	return ScoreBreakdown{
		HardViolations:    0,
		SoftPenalty:       weightedPenalty,
		StudentGapPenalty: totalGaps,
		GroupGaps:         groupGaps,
		Details:           details,
		Components: []ObjectiveComponentScore{
			{
				ID:            ObjectiveStudentGapPenalty,
				RawScore:      totalGaps,
				Weight:        weight,
				WeightedScore: weightedPenalty,
			},
		},
	}
}
