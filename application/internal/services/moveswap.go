package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type MoveSwapService struct {
	repos   *repositories.Repos
	adapter curra.CurraAdapter
}

func NewMoveSwapService(repos *repositories.Repos, adapter curra.CurraAdapter) *MoveSwapService {
	return &MoveSwapService{repos: repos, adapter: adapter}
}

func (s *MoveSwapService) Move(
	ctx context.Context,
	versionID uuid.UUID,
	move domain.MoveDTO,
	ifMatchVersion int,
	dryRun bool,
) (curra.ValidateMoveResponse, domain.ScheduleVersion, error) {
	// 1. Load version
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}

	// 2. Validate state & precondition
	if ver.Status != domain.VersionStatusDraft {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("%w: cannot edit non-DRAFT version", ErrInvalidState)
	}
	if ifMatchVersion <= 0 {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, ErrConflict
	}

	// 3. Load snapshot
	snap, err := s.repos.Snapshots.GetByID(ctx, ver.SnapshotID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("load snapshot: %w", err)
	}

	// 4. Load current assignments and build solution JSON
	assignments, err := s.repos.ScheduleAssignments.ListByVersion(ctx, versionID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("load assignments: %w", err)
	}

	solutionJSON, err := buildSolutionJSON(assignments, ver.Score)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("build solution JSON: %w", err)
	}

	// 5. Test move via CURRA adapter
	moveReq := curra.ValidateMoveRequest{
		ProblemJSON:  snap.ProblemJSON,
		SolutionJSON: solutionJSON,
		Move: curra.MoveDTO{
			AssignmentID: move.AssignmentID,
			From:         curra.PlacementDTO{RoomID: move.From.RoomID, TimeSlotID: move.From.TimeSlotID},
			To:           curra.PlacementDTO{RoomID: move.To.RoomID, TimeSlotID: move.To.TimeSlotID},
		},
		ConstraintsJSON: snap.ConstraintInstances,
	}

	resp, err := s.adapter.ValidateMove(ctx, moveReq)
	if err != nil {
		return resp, ver, err
	}
	if !resp.Valid || resp.Score.HardViolations > 0 {
		return resp, ver, fmt.Errorf("%w: move violates hard constraints", ErrValidation)
	}

	if dryRun {
		return resp, ver, nil
	}

	// 6. Persist changes atomically in a single PostgreSQL transaction
	newAssignments, err := parseAssignmentsFromSolution(versionID, resp.Solution)
	if err != nil {
		return resp, ver, fmt.Errorf("parse solution assignments: %w", err)
	}

	scoreJSON, _ := json.Marshal(resp.Score)
	user, _ := UserFromContext(ctx)
	audit := domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		UserID:        &user.ID,
		Action:        "version.assignment_move",
		ResourceType:  "schedule_version",
		ResourceID:    versionID,
		Details:       json.RawMessage(fmt.Sprintf(`{"assignmentId":"%s","fromRoom":"%s","fromSlot":"%s","toRoom":"%s","toSlot":"%s"}`, move.AssignmentID, move.From.RoomID, move.From.TimeSlotID, move.To.RoomID, move.To.TimeSlotID)),
		CreatedAt:     time.Now(),
	}

	if err := s.repos.ScheduleVersions.ApplyAssignmentUpdateTx(ctx, versionID, ifMatchVersion, scoreJSON, newAssignments, audit); err != nil {
		if errors.Is(err, repositories.ErrOptimisticLock) {
			return resp, ver, ErrConflict
		}
		return resp, ver, fmt.Errorf("apply move tx: %w", err)
	}

	updatedVer, _ := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	return resp, updatedVer, nil
}

func (s *MoveSwapService) Swap(
	ctx context.Context,
	versionID uuid.UUID,
	swap domain.SwapDTO,
	ifMatchVersion int,
	dryRun bool,
) (curra.ValidateMoveResponse, domain.ScheduleVersion, error) {
	// 1. Load version
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, err
	}

	// 2. Validate state & precondition
	if ver.Status != domain.VersionStatusDraft {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("%w: cannot edit non-DRAFT version", ErrInvalidState)
	}
	if ifMatchVersion <= 0 {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, ErrConflict
	}

	// 3. Load snapshot
	snap, err := s.repos.Snapshots.GetByID(ctx, ver.SnapshotID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("load snapshot: %w", err)
	}

	// 4. Load current assignments and build solution JSON
	assignments, err := s.repos.ScheduleAssignments.ListByVersion(ctx, versionID)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("load assignments: %w", err)
	}

	solutionJSON, err := buildSolutionJSON(assignments, ver.Score)
	if err != nil {
		return curra.ValidateMoveResponse{}, domain.ScheduleVersion{}, fmt.Errorf("build solution JSON: %w", err)
	}

	// 5. Test swap via CURRA adapter
	swapReq := curra.ValidateSwapRequest{
		ProblemJSON:  snap.ProblemJSON,
		SolutionJSON: solutionJSON,
		Swap: curra.SwapDTO{
			Assignment1ID: swap.Assignment1ID,
			Assignment2ID: swap.Assignment2ID,
			Placement1:    curra.PlacementDTO{RoomID: swap.Placement1.RoomID, TimeSlotID: swap.Placement1.TimeSlotID},
			Placement2:    curra.PlacementDTO{RoomID: swap.Placement2.RoomID, TimeSlotID: swap.Placement2.TimeSlotID},
		},
		ConstraintsJSON: snap.ConstraintInstances,
	}

	resp, err := s.adapter.ValidateSwap(ctx, swapReq)
	if err != nil {
		return resp, ver, err
	}
	if !resp.Valid || resp.Score.HardViolations > 0 {
		return resp, ver, fmt.Errorf("%w: swap violates hard constraints", ErrValidation)
	}

	if dryRun {
		return resp, ver, nil
	}

	// 6. Persist changes atomically in a single PostgreSQL transaction
	newAssignments, err := parseAssignmentsFromSolution(versionID, resp.Solution)
	if err != nil {
		return resp, ver, fmt.Errorf("parse solution assignments: %w", err)
	}

	scoreJSON, _ := json.Marshal(resp.Score)
	user, _ := UserFromContext(ctx)
	audit := domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		UserID:        &user.ID,
		Action:        "version.assignment_swap",
		ResourceType:  "schedule_version",
		ResourceID:    versionID,
		Details:       json.RawMessage(fmt.Sprintf(`{"assignment1Id":"%s","assignment2Id":"%s"}`, swap.Assignment1ID, swap.Assignment2ID)),
		CreatedAt:     time.Now(),
	}

	if err := s.repos.ScheduleVersions.ApplyAssignmentUpdateTx(ctx, versionID, ifMatchVersion, scoreJSON, newAssignments, audit); err != nil {
		if errors.Is(err, repositories.ErrOptimisticLock) {
			return resp, ver, ErrConflict
		}
		return resp, ver, fmt.Errorf("apply swap tx: %w", err)
	}

	updatedVer, _ := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	return resp, updatedVer, nil
}

func buildSolutionJSON(assignments []domain.ScheduleAssignment, scoreJSON json.RawMessage) (json.RawMessage, error) {
	type rawAssignment struct {
		ID                   string `json:"id"`
		CourseOfferingID     string `json:"courseOfferingId"`
		SessionRequirementID string `json:"sessionRequirementId"`
		StudentGroupID       string `json:"studentGroupId"`
		FacultyID            string `json:"facultyId"`
		RoomID               string `json:"roomId"`
		TimeSlotID           string `json:"timeSlotId"`
		Instance             int    `json:"instance"`
	}

	rawAssignments := make([]rawAssignment, len(assignments))
	for i, a := range assignments {
		rawAssignments[i] = rawAssignment{
			ID:                   a.AssignmentID,
			CourseOfferingID:     a.CourseOfferingID,
			SessionRequirementID: a.SessionRequirementID,
			StudentGroupID:       a.StudentGroupID,
			FacultyID:            a.FacultyID,
			RoomID:               a.RoomID,
			TimeSlotID:           a.TimeSlotID,
			Instance:             a.Instance,
		}
	}

	payload := map[string]any{
		"assignments": rawAssignments,
	}
	if len(scoreJSON) > 0 {
		var s any
		_ = json.Unmarshal(scoreJSON, &s)
		payload["score"] = s
	}

	return json.Marshal(payload)
}

func parseAssignmentsFromSolution(versionID uuid.UUID, solutionJSON json.RawMessage) ([]domain.ScheduleAssignment, error) {
	var sol struct {
		Assignments []struct {
			ID                   string `json:"id"`
			CourseOfferingID     string `json:"courseOfferingId"`
			SessionRequirementID string `json:"sessionRequirementId"`
			StudentGroupID       string `json:"studentGroupId"`
			FacultyID            string `json:"facultyId"`
			RoomID               string `json:"roomId"`
			TimeSlotID           string `json:"timeSlotId"`
			Instance             int    `json:"instance"`
		} `json:"assignments"`
	}

	if err := json.Unmarshal(solutionJSON, &sol); err != nil {
		return nil, err
	}

	res := make([]domain.ScheduleAssignment, len(sol.Assignments))
	for i, a := range sol.Assignments {
		res[i] = domain.ScheduleAssignment{
			ID:                   uuid.New(),
			VersionID:            versionID,
			AssignmentID:         a.ID,
			CourseOfferingID:     a.CourseOfferingID,
			SessionRequirementID: a.SessionRequirementID,
			StudentGroupID:       a.StudentGroupID,
			FacultyID:            a.FacultyID,
			RoomID:               a.RoomID,
			TimeSlotID:           a.TimeSlotID,
			Instance:             a.Instance,
			CreatedAt:            time.Now(),
		}
	}

	return res, nil
}
