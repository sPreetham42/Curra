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
	// ObjectiveFacultyPreference identifies the faculty slot preference objective.
	ObjectiveFacultyPreference ObjectiveID = "FacultyPreference"
	// ObjectiveRoomChange identifies the room change penalty objective.
	ObjectiveRoomChange ObjectiveID = "RoomChange"
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

// DefaultObjectiveConfig returns the default multi-objective configuration with StudentGapPenalty weight = 1, FacultyPreference weight = 1, and RoomChange weight = 1.
func DefaultObjectiveConfig() ObjectiveConfig {
	return ObjectiveConfig{
		Components: []ObjectiveComponent{
			{ID: ObjectiveStudentGapPenalty, Weight: 1},
			{ID: ObjectiveFacultyPreference, Weight: 1},
			{ID: ObjectiveRoomChange, Weight: 1},
		},
	}
}

// GetWeight returns the weight for the given objective ID. If cfg.Components is empty, defaults to 1 for built-in objectives.
func (cfg ObjectiveConfig) GetWeight(id ObjectiveID) int {
	if len(cfg.Components) > 0 {
		for _, c := range cfg.Components {
			if c.ID == id {
				return c.Weight
			}
		}
		return 0
	}
	if id == ObjectiveStudentGapPenalty || id == ObjectiveFacultyPreference || id == ObjectiveRoomChange {
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
	HardViolations           int                       `json:"hardViolations"`
	SoftPenalty              int                       `json:"softPenalty"`
	StudentGapPenalty        int                       `json:"studentGapPenalty"`
	FacultyPreferencePenalty int                       `json:"facultyPreferencePenalty,omitempty"`
	RoomChangePenalty        int                       `json:"roomChangePenalty,omitempty"`
	GroupGaps                map[model.StudentGroupID]int `json:"groupGaps,omitempty"`
	Details                  []GroupDayGap             `json:"details,omitempty"`
	Components               []ObjectiveComponentScore `json:"components,omitempty"`
}

// OccupiedPeriod represents an occupied period on a specific day for a student group.
type OccupiedPeriod struct {
	StudentGroupID model.StudentGroupID
	Day            time.Weekday
	Period         int
}

// FacultySlotOccupancy represents an occupied slot for a specific faculty member.
type FacultySlotOccupancy struct {
	FacultyID  model.FacultyID
	TimeSlotID model.TimeSlotID
}

// OccupiedSession represents a scheduled session for a student group on a specific day.
type OccupiedSession struct {
	SessionID      string
	StudentGroupID model.StudentGroupID
	Day            time.Weekday
	StartPeriod    int
	EndPeriod      int
	RoomID         model.RoomID
}

// CalculateRoomChangePenaltyWithConfig calculates room change penalties between adjacent scheduled sessions on the same day for student groups.
func CalculateRoomChangePenaltyWithConfig(sessions []OccupiedSession, cfg ObjectiveConfig) ScoreBreakdown {
	type groupDayKey struct {
		groupID model.StudentGroupID
		day     time.Weekday
	}
	grid := make(map[groupDayKey][]OccupiedSession)
	for _, s := range sessions {
		key := groupDayKey{groupID: s.StudentGroupID, day: s.Day}
		grid[key] = append(grid[key], s)
	}

	rawPenalty := 0
	for _, list := range grid {
		if len(list) < 2 {
			continue
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].StartPeriod != list[j].StartPeriod {
				return list[i].StartPeriod < list[j].StartPeriod
			}
			if list[i].EndPeriod != list[j].EndPeriod {
				return list[i].EndPeriod < list[j].EndPeriod
			}
			if list[i].RoomID != list[j].RoomID {
				return list[i].RoomID < list[j].RoomID
			}
			return list[i].SessionID < list[j].SessionID
		})
		for i := 0; i < len(list)-1; i++ {
			if list[i].RoomID != list[i+1].RoomID {
				rawPenalty++
			}
		}
	}

	weight := cfg.GetWeight(ObjectiveRoomChange)
	weightedPenalty := rawPenalty * weight

	return ScoreBreakdown{
		HardViolations:    0,
		SoftPenalty:       weightedPenalty,
		RoomChangePenalty: rawPenalty,
		Components: []ObjectiveComponentScore{
			{
				ID:            ObjectiveRoomChange,
				RawScore:      rawPenalty,
				Weight:        weight,
				WeightedScore: weightedPenalty,
			},
		},
	}
}

// CalculateFacultyPreferencePenaltyWithConfig calculates the faculty preference penalty from preferences and faculty slot occupancies.
func CalculateFacultyPreferencePenaltyWithConfig(preferences []model.FacultyPreference, occupancies []FacultySlotOccupancy, cfg ObjectiveConfig) ScoreBreakdown {
	prefMap := make(map[model.FacultyID]map[model.TimeSlotID]int)
	for _, p := range preferences {
		if prefMap[p.FacultyID] == nil {
			prefMap[p.FacultyID] = make(map[model.TimeSlotID]int)
		}
		prefMap[p.FacultyID][p.TimeSlotID] += p.Weight
	}

	rawPenalty := 0
	for _, occ := range occupancies {
		if slots, ok := prefMap[occ.FacultyID]; ok {
			if w, ok := slots[occ.TimeSlotID]; ok {
				rawPenalty += w
			}
		}
	}

	weight := cfg.GetWeight(ObjectiveFacultyPreference)
	weightedPenalty := rawPenalty * weight

	return ScoreBreakdown{
		HardViolations:           0,
		SoftPenalty:              weightedPenalty,
		FacultyPreferencePenalty: rawPenalty,
		Components: []ObjectiveComponentScore{
			{
				ID:            ObjectiveFacultyPreference,
				RawScore:      rawPenalty,
				Weight:        weight,
				WeightedScore: weightedPenalty,
			},
		},
	}
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
