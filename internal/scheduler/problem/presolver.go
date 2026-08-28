package problem

import (
	"fmt"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
)

const presolveConstraint = "PreSolveAnalysis"

// PreSolve performs lightweight feasibility checks before CSP search.
// It detects obviously impossible problems without exhaustively searching.
// Returns violations if the problem is provably infeasible.
func PreSolve(p *Problem) []diagnostics.Violation {
	var violations []diagnostics.Violation

	if len(p.CourseOfferings) == 0 || len(p.Rooms) == 0 || len(p.TimeSlots) == 0 {
		return violations
	}

	violations = append(violations, checkZeroDomain(p)...)
	violations = append(violations, checkFacultyOverload(p)...)
	violations = append(violations, checkRoomFeatureBottleneck(p)...)

	return violations
}

// checkZeroDomain verifies that every session requirement has at least one
// legal placement (room × slot) considering availability, capacity, and features.
func checkZeroDomain(p *Problem) []diagnostics.Violation {
	var violations []diagnostics.Violation

	for reqID, req := range p.SessionRequirements {
		if req.SessionsPerWeek <= 0 {
			continue
		}
		offering, ok := p.CourseOfferings[req.CourseOfferingID]
		if !ok {
			continue
		}

		groupSize := p.StudentGroupSize(offering.StudentGroupID)
		requiredFeatures := p.RequiredRoomFeatures(offering.ID, reqID)
		facultyID := offering.FacultyID

		// Find eligible rooms
		var eligibleRooms []model.RoomID
		for roomID, room := range p.Rooms {
			if room.Capacity < groupSize {
				continue
			}
			if !p.RoomHasFeatures(roomID, requiredFeatures) {
				continue
			}
			eligibleRooms = append(eligibleRooms, roomID)
		}

		if len(eligibleRooms) == 0 {
			violations = append(violations, presolveViolation(
				fmt.Sprintf("session requirement %s: no room satisfies capacity (%d) and feature requirements", reqID, groupSize),
				map[string]string{
					"sessionRequirementId": string(reqID),
					"courseOfferingId":     string(offering.ID),
					"facultyId":            string(facultyID),
				},
				map[string]string{
					"requiredCapacity": fmt.Sprintf("%d", groupSize),
					"totalRooms":       fmt.Sprintf("%d", len(p.Rooms)),
				},
			))
			continue
		}

		// Count eligible (room, slot) pairs
		placementCount := 0
		for slotID := range p.TimeSlots {
			// Quick check: does the session fit starting at this slot?
			slotIDs, ok := p.OccupiedSlotIDs(slotID, req.Duration)
			if !ok {
				continue
			}

			// Check faculty availability
			if !p.IsFacultyAvailable(facultyID, slotIDs) {
				continue
			}

			// Check if any eligible room is available
			for _, roomID := range eligibleRooms {
				if p.IsRoomAvailable(roomID, slotIDs) {
					placementCount++
					break // one valid placement per slot is enough
				}
			}

			if placementCount >= req.SessionsPerWeek {
				break // enough placements found
			}
		}

		if placementCount < req.SessionsPerWeek {
			violations = append(violations, presolveViolation(
				fmt.Sprintf("session requirement %s: only %d legal placements but needs %d sessions per week", reqID, placementCount, req.SessionsPerWeek),
				map[string]string{
					"sessionRequirementId": string(reqID),
					"courseOfferingId":     string(offering.ID),
					"facultyId":            string(facultyID),
				},
				map[string]string{
					"availablePlacements": fmt.Sprintf("%d", placementCount),
					"sessionsPerWeek":     fmt.Sprintf("%d", req.SessionsPerWeek),
				},
			))
		}
	}

	return violations
}

// checkFacultyOverload verifies each faculty has enough available time slots
// to teach all their assigned sessions.
func checkFacultyOverload(p *Problem) []diagnostics.Violation {
	var violations []diagnostics.Violation

	// Count required sessions per faculty
	facultySessions := make(map[model.FacultyID]int)
	for _, offering := range p.CourseOfferings {
		totalSessions := 0
		for _, reqID := range offering.SessionRequirementIDs {
			if req, ok := p.SessionRequirements[reqID]; ok {
				totalSessions += req.SessionsPerWeek
			}
		}
		facultySessions[offering.FacultyID] += totalSessions
	}

	// Count available slots per faculty
	for facultyID, requiredSessions := range facultySessions {
		availableSlots := 0
		if avail, ok := p.FacultyAvailable[facultyID]; ok {
			availableSlots = len(avail)
		}
		if availableSlots < requiredSessions {
			violations = append(violations, presolveViolation(
				fmt.Sprintf("faculty %s needs %d session slots but only has %d available", facultyID, requiredSessions, availableSlots),
				map[string]string{
					"facultyId": string(facultyID),
				},
				map[string]string{
					"requiredSessions": fmt.Sprintf("%d", requiredSessions),
					"availableSlots":   fmt.Sprintf("%d", availableSlots),
				},
			))
		}
	}

	return violations
}

// checkRoomFeatureBottleneck detects situations where too many sessions require
// specific features but too few rooms provide them.
func checkRoomFeatureBottleneck(p *Problem) []diagnostics.Violation {
	var violations []diagnostics.Violation

	// Count sessions requiring each feature set
	featureDemand := make(map[string]int) // feature set hash → session count
	featureSetToIDs := make(map[string][]model.RoomFeatureID)

	for reqID, req := range p.SessionRequirements {
		offering, ok := p.CourseOfferings[req.CourseOfferingID]
		if !ok {
			continue
		}
		requiredFeatures := p.RequiredRoomFeatures(offering.ID, reqID)
		if len(requiredFeatures) == 0 {
			continue
		}

		key := featureSetKey(requiredFeatures)
		featureDemand[key] += req.SessionsPerWeek
		if _, seen := featureSetToIDs[key]; !seen {
			featureSetToIDs[key] = requiredFeatures
		}
	}

	// Count rooms providing each feature set
	featureSupply := make(map[string]int) // feature set hash → room count
	for roomID, room := range p.Rooms {
		// For each required feature set, check if this room satisfies it
		for key, featureIDs := range featureSetToIDs {
			if p.RoomHasFeatures(roomID, featureIDs) {
				featureSupply[key] += room.Capacity
			}
		}
		_ = room
	}

	// Check each feature set: total room capacity for those features vs. total demand
	for key, demand := range featureDemand {
		supply := featureSupply[key]
		// Rough check: if total capacity of rooms with these features is less than
		// total student-hours demanded, there's likely a bottleneck
		// This is a heuristic — a full check would need slot-level analysis
		featureIDs := featureSetToIDs[key]
		if supply == 0 && demand > 0 {
			violations = append(violations, presolveViolation(
				fmt.Sprintf("no room provides required features [%s] but %d sessions need them", joinFeatureIDs(featureIDs), demand),
				nil,
				map[string]string{
					"requiredFeatures": joinFeatureIDs(featureIDs),
					"demandSessions":   fmt.Sprintf("%d", demand),
				},
			))
		}
	}

	return violations
}

func featureSetKey(features []model.RoomFeatureID) string {
	if len(features) == 0 {
		return ""
	}
	// features are already sorted by the model
	key := ""
	for _, f := range features {
		if key != "" {
			key += ","
		}
		key += string(f)
	}
	return key
}

func joinFeatureIDs(ids []model.RoomFeatureID) string {
	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ", "
		}
		result += string(id)
	}
	return result
}

func presolveViolation(message string, related map[string]string, metadata map[string]string) diagnostics.Violation {
	return diagnostics.Violation{
		ConstraintName: presolveConstraint,
		Severity:       diagnostics.SeverityHard,
		Message:        message,
		RelatedIDs:     related,
		Metadata:       metadata,
	}
}
