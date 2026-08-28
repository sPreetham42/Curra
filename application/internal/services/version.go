package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type VersionService struct {
	repos *repositories.Repos
}

func NewVersionService(repos *repositories.Repos) *VersionService {
	return &VersionService{repos: repos}
}

func (s *VersionService) CreateVersion(
	ctx context.Context,
	timetableID uuid.UUID,
	sourceRunID *uuid.UUID,
	snapshotID uuid.UUID,
	name string,
	createdBy uuid.UUID,
	idempotencyKey string,
) (domain.ScheduleVersion, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ScheduleVersion{}, err
	}

	// 2. Atomic Idempotency Lock
	var lockToken *uuid.UUID
	if idempotencyKey != "" {
		keyRecord, isCompleted, err := s.repos.Idempotency.Acquire(ctx, tt.InstitutionID, idempotencyKey, "schedule_version")
		if err != nil {
			if errors.Is(err, repositories.ErrIdempotencyConflict) {
				// Await completion of in-flight concurrent request
				for i := 0; i < 5; i++ {
					time.Sleep(50 * time.Millisecond)
					if existing, getErr := s.repos.Idempotency.Get(ctx, tt.InstitutionID, idempotencyKey); getErr == nil && existing != nil && existing.Status == domain.IdempotencyStatusCompleted {
						var v domain.ScheduleVersion
						if unmarshalErr := json.Unmarshal(existing.ResponseBody, &v); unmarshalErr == nil {
							return v, nil
						}
					}
				}
				return domain.ScheduleVersion{}, ErrConflict
			}
			return domain.ScheduleVersion{}, fmt.Errorf("acquire idempotency key: %w", err)
		}
		if keyRecord != nil {
			lockToken = keyRecord.LockToken
		}
		if isCompleted && keyRecord != nil && len(keyRecord.ResponseBody) > 0 {
			var v domain.ScheduleVersion
			if err := json.Unmarshal(keyRecord.ResponseBody, &v); err == nil {
				return v, nil
			}
		}
	}

	var scoreJSON json.RawMessage
	var assignmentsToCopy []domain.ScheduleAssignment

	if sourceRunID != nil {
		run, err := s.repos.ScheduleRuns.GetByID(ctx, *sourceRunID)
		if err != nil {
			if idempotencyKey != "" {
				_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
			}
			return domain.ScheduleVersion{}, fmt.Errorf("source run not found: %w", err)
		}
		if run.TimetableID != timetableID {
			if idempotencyKey != "" {
				_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
			}
			return domain.ScheduleVersion{}, ErrNotFound
		}
		scoreJSON = run.Score

		// Parse assignments from run.Result solution JSON if present
		if len(run.Result) > 0 {
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
			if err := json.Unmarshal(run.Result, &sol); err == nil {
				for _, a := range sol.Assignments {
					assignmentsToCopy = append(assignmentsToCopy, domain.ScheduleAssignment{
						ID:                   uuid.New(),
						AssignmentID:         a.ID,
						CourseOfferingID:     a.CourseOfferingID,
						SessionRequirementID: a.SessionRequirementID,
						StudentGroupID:       a.StudentGroupID,
						FacultyID:            a.FacultyID,
						RoomID:               a.RoomID,
						TimeSlotID:           a.TimeSlotID,
						Instance:             a.Instance,
					})
				}
			}
		}
	}

	if name == "" {
		name = fmt.Sprintf("Draft %s", time.Now().Format("2006-01-02 15:04"))
	}

	ver := domain.ScheduleVersion{
		ID:            uuid.New(),
		TimetableID:   timetableID,
		InstitutionID: tt.InstitutionID,
		SourceRunID:   sourceRunID,
		SnapshotID:    snapshotID,
		Status:        domain.VersionStatusDraft,
		Name:          name,
		Score:         scoreJSON,
		Version:       1,
		CreatedBy:     createdBy,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repos.ScheduleVersions.Create(ctx, ver); err != nil {
		if idempotencyKey != "" {
			_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
		}
		return domain.ScheduleVersion{}, fmt.Errorf("create version: %w", err)
	}

	// Copy assignments
	if len(assignmentsToCopy) > 0 {
		for i := range assignmentsToCopy {
			assignmentsToCopy[i].VersionID = ver.ID
		}
		_ = s.repos.ScheduleAssignments.CreateBatch(ctx, assignmentsToCopy)
	}

	// Complete Idempotency Key
	if idempotencyKey != "" {
		body, _ := json.Marshal(ver)
		_ = s.repos.Idempotency.Complete(ctx, tt.InstitutionID, idempotencyKey, lockToken, ver.ID, 201, body)
	}

	return ver, nil
}

func (s *VersionService) GetVersion(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, []domain.ScheduleAssignment, error) {
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleVersion{}, nil, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, nil, err
	}
	assignments, err := s.repos.ScheduleAssignments.ListByVersion(ctx, id)
	if err != nil {
		return ver, nil, err
	}
	return ver, assignments, nil
}

func (s *VersionService) ListVersions(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return nil, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return nil, err
	}
	return s.repos.ScheduleVersions.ListByTimetable(ctx, timetableID)
}

func (s *VersionService) UpdateVersionName(ctx context.Context, id uuid.UUID, name string, ifMatchVersion int) (domain.ScheduleVersion, error) {
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if ifMatchVersion <= 0 {
		return domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return domain.ScheduleVersion{}, ErrConflict
	}
	ver.Name = name
	if err := s.repos.ScheduleVersions.Update(ctx, ver); err != nil {
		if err == repositories.ErrOptimisticLock {
			return domain.ScheduleVersion{}, ErrConflict
		}
		return domain.ScheduleVersion{}, err
	}
	return s.repos.ScheduleVersions.GetByID(ctx, id)
}

func (s *VersionService) SubmitReview(ctx context.Context, id uuid.UUID, ifMatchVersion int) (domain.ScheduleVersion, error) {
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if ifMatchVersion <= 0 {
		return domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return domain.ScheduleVersion{}, ErrConflict
	}
	if ver.Status != domain.VersionStatusDraft {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: can only submit review from DRAFT", ErrInvalidState)
	}

	if err := s.repos.ScheduleVersions.UpdateStatus(ctx, id, domain.VersionStatusReview, ifMatchVersion); err != nil {
		if err == repositories.ErrOptimisticLock {
			return domain.ScheduleVersion{}, ErrConflict
		}
		return domain.ScheduleVersion{}, err
	}

	_ = s.repos.AuditEvents.Create(ctx, domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		Action:        "version.submit_review",
		ResourceType:  "schedule_version",
		ResourceID:    id,
		Details:       json.RawMessage(`{"from":"DRAFT","to":"REVIEW"}`),
		CreatedAt:     time.Now(),
	})

	return s.repos.ScheduleVersions.GetByID(ctx, id)
}

func (s *VersionService) SendBack(ctx context.Context, id uuid.UUID, ifMatchVersion int) (domain.ScheduleVersion, error) {
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if ifMatchVersion <= 0 {
		return domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return domain.ScheduleVersion{}, ErrConflict
	}
	if ver.Status != domain.VersionStatusReview {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: can only send back to draft from REVIEW", ErrInvalidState)
	}

	if err := s.repos.ScheduleVersions.UpdateStatus(ctx, id, domain.VersionStatusDraft, ifMatchVersion); err != nil {
		if err == repositories.ErrOptimisticLock {
			return domain.ScheduleVersion{}, ErrConflict
		}
		return domain.ScheduleVersion{}, err
	}

	_ = s.repos.AuditEvents.Create(ctx, domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		Action:        "version.send_back",
		ResourceType:  "schedule_version",
		ResourceID:    id,
		Details:       json.RawMessage(`{"from":"REVIEW","to":"DRAFT"}`),
		CreatedAt:     time.Now(),
	})

	return s.repos.ScheduleVersions.GetByID(ctx, id)
}

func (s *VersionService) Archive(ctx context.Context, id uuid.UUID, ifMatchVersion int) (domain.ScheduleVersion, error) {
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if ifMatchVersion <= 0 {
		return domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return domain.ScheduleVersion{}, ErrConflict
	}

	// State machine transition rule: REVIEW -> ARCHIVED is invalid (must transition through DRAFT first)
	if ver.Status == domain.VersionStatusReview {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: cannot archive version in REVIEW status (must send back to DRAFT first)", ErrInvalidState)
	}
	if ver.Status != domain.VersionStatusDraft && ver.Status != domain.VersionStatusPublished {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: can only archive DRAFT or PUBLISHED versions", ErrInvalidState)
	}

	if err := s.repos.ScheduleVersions.UpdateStatus(ctx, id, domain.VersionStatusArchived, ifMatchVersion); err != nil {
		if err == repositories.ErrOptimisticLock {
			return domain.ScheduleVersion{}, ErrConflict
		}
		return domain.ScheduleVersion{}, err
	}

	_ = s.repos.AuditEvents.Create(ctx, domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		Action:        "version.archive",
		ResourceType:  "schedule_version",
		ResourceID:    id,
		Details:       json.RawMessage(fmt.Sprintf(`{"from":"%s","to":"ARCHIVED"}`, ver.Status)),
		CreatedAt:     time.Now(),
	})

	return s.repos.ScheduleVersions.GetByID(ctx, id)
}
