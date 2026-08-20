package verifier

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/constraints"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/model"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/problem"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

var (
	// ErrInvalidResult indicates that the solution failed structural completeness, uniqueness, domain validity, lock preservation, or score consistency checks.
	ErrInvalidResult = errors.New("solution result integrity verification failed")

	// ErrHardConstraintViolation indicates that the solution violates one or more hard constraints.
	ErrHardConstraintViolation = errors.New("solution violates hard constraints")
)

// VerifyOptions configures the authoritative verification pass.
type VerifyOptions struct {
	Compiled        *constraints.CompiledConstraintSet
	ObjectiveConfig *scorer.ObjectiveConfig
}

// VerificationReport contains the result of the authoritative verification pass.
type VerificationReport struct {
	Valid      bool                    `json:"valid"`
	Status     diagnostics.SolveStatus `json:"status"`
	Violations []diagnostics.Violation `json:"violations,omitempty"`
	Message    string                  `json:"message,omitempty"`
}

// VerifySolution performs an authoritative, pure/read-only verification pass proving:
// 1. Requirement completeness
// 2. Total assignment count matches requirement sums
// 3. Assignment ID uniqueness
// 4. Foreign-key & domain catalog validity
// 5. Placement & grid duration validity
// 6. Locked assignment preservation
// 7. Hard constraint compliance
// 8. Full score & breakdown consistency
func VerifySolution(p *problem.Problem, solution *problem.Solution, opts VerifyOptions) (VerificationReport, error) {
	if p == nil {
		return VerificationReport{
			Valid:   false,
			Status:  diagnostics.SolveStatusInvalidProblem,
			Message: "problem is nil",
		}, errors.New("problem is nil")
	}
	if solution == nil {
		return VerificationReport{
			Valid:   false,
			Status:  diagnostics.SolveStatusInvalidResult,
			Message: "solution is nil",
		}, ErrInvalidResult
	}

	p.Prepare()

	var integrityViolations []diagnostics.Violation
	var hardViolations []diagnostics.Violation

	// 1. Assignment ID Uniqueness & Count Tracking
	seenIDs := make(map[problem.AssignmentID]struct{}, len(solution.Assignments))
	assignmentMap := make(map[problem.AssignmentID]problem.Assignment, len(solution.Assignments))
	scheduledPerReq := make(map[model.SessionRequirementID]int, len(p.SessionRequirements))

	for _, a := range solution.Assignments {
		if a.ID == "" {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        "solution contains assignment with empty ID",
			})
		} else if _, seen := seenIDs[a.ID]; seen {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("duplicate assignment ID '%s'", a.ID),
				AssignmentID:   string(a.ID),
			})
		}
		seenIDs[a.ID] = struct{}{}
		assignmentMap[a.ID] = a
		scheduledPerReq[a.SessionRequirementID]++

		// 4 & 5. Foreign-Key, Domain & Placement Validity
		offering, hasOffering := p.CourseOfferings[a.CourseOfferingID]
		if a.CourseOfferingID == "" || !hasOffering {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing course offering '%s'", a.CourseOfferingID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId": string(a.CourseOfferingID),
				},
			})
		}

		req, hasReq := p.SessionRequirements[a.SessionRequirementID]
		if a.SessionRequirementID == "" || !hasReq {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing session requirement '%s'", a.SessionRequirementID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"sessionRequirementId": string(a.SessionRequirementID),
				},
			})
		} else if hasOffering && req.CourseOfferingID != a.CourseOfferingID {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        "session requirement does not belong to the assigned course offering",
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"courseOfferingId":     string(a.CourseOfferingID),
					"sessionRequirementId": string(a.SessionRequirementID),
				},
			})
		}

		group, hasGroup := p.StudentGroups[a.StudentGroupID]
		if a.StudentGroupID == "" || !hasGroup {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing student group '%s'", a.StudentGroupID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"studentGroupId": string(a.StudentGroupID),
				},
			})
		} else if hasOffering && offering.StudentGroupID != a.StudentGroupID {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        "student group does not match course offering student group",
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"studentGroupId":         string(a.StudentGroupID),
					"offeringStudentGroupId": string(offering.StudentGroupID),
				},
			})
		}

		fac, hasFaculty := p.Faculty[a.FacultyID]
		if a.FacultyID == "" || !hasFaculty {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing faculty '%s'", a.FacultyID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"facultyId": string(a.FacultyID),
				},
			})
		} else if hasOffering && offering.FacultyID != a.FacultyID {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        "faculty does not match course offering faculty",
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"facultyId":         string(a.FacultyID),
					"offeringFacultyId": string(offering.FacultyID),
				},
			})
		}

		_, hasRoom := p.Rooms[a.RoomID]
		if a.RoomID == "" || !hasRoom {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing room '%s'", a.RoomID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"roomId": string(a.RoomID),
				},
			})
		}

		_, hasSlot := p.TimeSlots[a.TimeSlotID]
		if a.TimeSlotID == "" || !hasSlot {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ResultIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("assignment references missing time slot '%s'", a.TimeSlotID),
				AssignmentID:   string(a.ID),
				RelatedIDs: map[string]string{
					"timeSlotId": string(a.TimeSlotID),
				},
			})
		}

		if hasReq && hasSlot {
			if _, fits := p.OccupiedSlotIDs(a.TimeSlotID, req.Duration); !fits {
				integrityViolations = append(integrityViolations, diagnostics.Violation{
					ConstraintName: "ResultIntegrity",
					Severity:       diagnostics.SeverityHard,
					Message:        "assignment does not fit in the recurring time-slot grid",
					AssignmentID:   string(a.ID),
					RelatedIDs: map[string]string{
						"timeSlotId":           string(a.TimeSlotID),
						"sessionRequirementId": string(a.SessionRequirementID),
					},
					Metadata: map[string]string{
						"duration": fmt.Sprintf("%d", req.Duration),
					},
				})
			}
			if a.Instance < 0 || a.Instance >= req.SessionsPerWeek {
				integrityViolations = append(integrityViolations, diagnostics.Violation{
					ConstraintName: "ResultIntegrity",
					Severity:       diagnostics.SeverityHard,
					Message:        fmt.Sprintf("assignment instance %d out of bounds for requirement (sessionsPerWeek=%d)", a.Instance, req.SessionsPerWeek),
					AssignmentID:   string(a.ID),
				})
			}
		}
		_ = fac
		_ = group
	}

	// 2. Requirement Completeness & Total Counts
	expectedTotal := 0
	for reqID, req := range p.SessionRequirements {
		expectedTotal += req.SessionsPerWeek
		actual := scheduledPerReq[reqID]
		if actual < req.SessionsPerWeek {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "RequirementCompleteness",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("missing assignments for session requirement '%s' (expected %d, scheduled %d)", reqID, req.SessionsPerWeek, actual),
				RelatedIDs: map[string]string{
					"sessionRequirementId": string(reqID),
				},
				Metadata: map[string]string{
					"expected": fmt.Sprintf("%d", req.SessionsPerWeek),
					"actual":   fmt.Sprintf("%d", actual),
				},
			})
		} else if actual > req.SessionsPerWeek {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "RequirementCompleteness",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("excess assignments for session requirement '%s' (expected %d, scheduled %d)", reqID, req.SessionsPerWeek, actual),
				RelatedIDs: map[string]string{
					"sessionRequirementId": string(reqID),
				},
				Metadata: map[string]string{
					"expected": fmt.Sprintf("%d", req.SessionsPerWeek),
					"actual":   fmt.Sprintf("%d", actual),
				},
			})
		}
	}

	if len(solution.Assignments) != expectedTotal {
		integrityViolations = append(integrityViolations, diagnostics.Violation{
			ConstraintName: "RequirementCompleteness",
			Severity:       diagnostics.SeverityHard,
			Message:        fmt.Sprintf("total assignment count mismatch (expected %d, scheduled %d)", expectedTotal, len(solution.Assignments)),
			Metadata: map[string]string{
				"expectedTotal": fmt.Sprintf("%d", expectedTotal),
				"actualTotal":   fmt.Sprintf("%d", len(solution.Assignments)),
			},
		})
	}

	// 6. Locked Assignments Preservation
	for _, locked := range p.LockedAssignments {
		actual, exists := assignmentMap[locked.ID]
		if !exists {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "LockedAssignmentIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("missing locked assignment '%s'", locked.ID),
				AssignmentID:   string(locked.ID),
			})
			continue
		}

		if actual.RoomID != locked.RoomID ||
			actual.TimeSlotID != locked.TimeSlotID ||
			actual.CourseOfferingID != locked.CourseOfferingID ||
			actual.SessionRequirementID != locked.SessionRequirementID ||
			actual.FacultyID != locked.FacultyID ||
			actual.StudentGroupID != locked.StudentGroupID ||
			actual.Instance != locked.Instance {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "LockedAssignmentIntegrity",
				Severity:       diagnostics.SeverityHard,
				Message:        fmt.Sprintf("locked assignment '%s' was mutated", locked.ID),
				AssignmentID:   string(locked.ID),
				Metadata: map[string]string{
					"expectedRoom": string(locked.RoomID),
					"actualRoom":   string(actual.RoomID),
					"expectedSlot": string(locked.TimeSlotID),
					"actualSlot":   string(actual.TimeSlotID),
				},
			})
		}
	}

	// 7. Hard Constraint Compliance
	searchCtx := constraints.NewSearchCtx(p)
	if opts.Compiled != nil && len(opts.Compiled.Hard) > 0 {
		for _, c := range opts.Compiled.Hard {
			hardViolations = append(hardViolations, c.Evaluate(searchCtx, solution)...)
		}
	} else {
		defaultHard := []constraints.ConstraintDef{
			constraints.NewFacultyConflictConstraint(constraints.ConstraintInstance{}),
			constraints.NewRoomConflictConstraint(constraints.ConstraintInstance{}),
			constraints.NewStudentGroupConflictConstraint(constraints.ConstraintInstance{}),
			constraints.NewRoomCapacityConstraint(constraints.ConstraintInstance{}),
			constraints.NewRoomFeatureCompatibilityConstraint(constraints.ConstraintInstance{}),
			constraints.NewFacultyAvailabilityConstraint(constraints.ConstraintInstance{}),
			constraints.NewRoomAvailabilityConstraint(constraints.ConstraintInstance{}),
		}
		for _, c := range defaultHard {
			hardViolations = append(hardViolations, c.Evaluate(searchCtx, solution)...)
		}
	}

	// 8. Score Consistency
	objConfig := scorer.DefaultObjectiveConfig()
	if opts.ObjectiveConfig != nil {
		objConfig = *opts.ObjectiveConfig
	}

	expectedBreakdown := p.StudentGapPenaltyWithConfig(solution, objConfig)
	expectedHardViolations := len(hardViolations)

	if solution.Score.HardViolations != expectedHardViolations {
		integrityViolations = append(integrityViolations, diagnostics.Violation{
			ConstraintName: "ScoreConsistency",
			Severity:       diagnostics.SeverityHard,
			Message:        fmt.Sprintf("reported HardViolations (%d) does not match actual count (%d)", solution.Score.HardViolations, expectedHardViolations),
		})
	}

	if solution.Score.SoftPenalty != expectedBreakdown.SoftPenalty {
		integrityViolations = append(integrityViolations, diagnostics.Violation{
			ConstraintName: "ScoreConsistency",
			Severity:       diagnostics.SeverityHard,
			Message:        fmt.Sprintf("reported SoftPenalty (%d) does not match authoritative calculation (%d)", solution.Score.SoftPenalty, expectedBreakdown.SoftPenalty),
		})
	}

	if solution.Score.Breakdown.StudentGapPenalty != expectedBreakdown.StudentGapPenalty {
		integrityViolations = append(integrityViolations, diagnostics.Violation{
			ConstraintName: "ScoreConsistency",
			Severity:       diagnostics.SeverityHard,
			Message:        fmt.Sprintf("reported StudentGapPenalty (%d) does not match authoritative calculation (%d)", solution.Score.Breakdown.StudentGapPenalty, expectedBreakdown.StudentGapPenalty),
		})
	}

	if solution.Score.Breakdown.SoftPenalty != expectedBreakdown.SoftPenalty {
		integrityViolations = append(integrityViolations, diagnostics.Violation{
			ConstraintName: "ScoreConsistency",
			Severity:       diagnostics.SeverityHard,
			Message:        fmt.Sprintf("reported Breakdown.SoftPenalty (%d) does not match authoritative calculation (%d)", solution.Score.Breakdown.SoftPenalty, expectedBreakdown.SoftPenalty),
		})
	}

	if len(solution.Score.Breakdown.Components) > 0 {
		if !reflect.DeepEqual(solution.Score.Breakdown.Components, expectedBreakdown.Components) {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ScoreConsistency",
				Severity:       diagnostics.SeverityHard,
				Message:        "reported objective components do not match authoritative calculation",
			})
		}
	}

	if len(solution.Score.Breakdown.GroupGaps) > 0 {
		if !reflect.DeepEqual(solution.Score.Breakdown.GroupGaps, expectedBreakdown.GroupGaps) {
			integrityViolations = append(integrityViolations, diagnostics.Violation{
				ConstraintName: "ScoreConsistency",
				Severity:       diagnostics.SeverityHard,
				Message:        "reported group gaps breakdown does not match authoritative calculation",
			})
		}
	}

	// Result synthesis
	allViolations := append(integrityViolations, hardViolations...)

	if len(allViolations) == 0 {
		return VerificationReport{
			Valid:   true,
			Status:  diagnostics.SolveStatusSolved,
			Message: "solution is complete, feasible, and score-consistent",
		}, nil
	}

	if len(integrityViolations) > 0 {
		return VerificationReport{
			Valid:      false,
			Status:     diagnostics.SolveStatusInvalidResult,
			Violations: allViolations,
			Message:    fmt.Sprintf("solution failed result integrity verification with %d violations", len(allViolations)),
		}, ErrInvalidResult
	}

	return VerificationReport{
		Valid:      false,
		Status:     diagnostics.SolveStatusInfeasible,
		Violations: allViolations,
		Message:    fmt.Sprintf("solution failed hard constraint verification with %d violations", len(allViolations)),
	}, ErrHardConstraintViolation
}
