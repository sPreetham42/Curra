package scorer

import (
	"sort"
	"time"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

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

// ScoreBreakdown provides a detailed breakdown of soft penalties.
type ScoreBreakdown struct {
	HardViolations    int                          `json:"hardViolations"`
	SoftPenalty       int                          `json:"softPenalty"`
	StudentGapPenalty int                          `json:"studentGapPenalty"`
	GroupGaps         map[model.StudentGroupID]int `json:"groupGaps,omitempty"`
	Details           []GroupDayGap                `json:"details,omitempty"`
}

// OccupiedPeriod represents an occupied period on a specific day for a student group.
type OccupiedPeriod struct {
	StudentGroupID model.StudentGroupID
	Day            time.Weekday
	Period         int
}

// CalculateStudentGapPenalty calculates the gap penalty from occupied group periods.
// For each StudentGroup and day:
// - find first occupied slot
// - find last occupied slot
// - count unoccupied slots strictly between them (leading/trailing free periods do not count)
func CalculateStudentGapPenalty(groups []model.StudentGroupID, occupied []OccupiedPeriod) ScoreBreakdown {
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

	return ScoreBreakdown{
		HardViolations:    0,
		SoftPenalty:       totalGaps,
		StudentGapPenalty: totalGaps,
		GroupGaps:         groupGaps,
		Details:           details,
	}
}
