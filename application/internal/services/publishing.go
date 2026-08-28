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

type PublishingService struct {
	repos   *repositories.Repos
	adapter curra.CurraAdapter
}

func NewPublishingService(repos *repositories.Repos, adapter curra.CurraAdapter) *PublishingService {
	return &PublishingService{repos: repos, adapter: adapter}
}

func (s *PublishingService) Publish(
	ctx context.Context,
	versionID uuid.UUID,
	ifMatchVersion int,
	publishedBy uuid.UUID,
) (domain.ScheduleVersion, error) {
	// 1. Load version
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	if err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return domain.ScheduleVersion{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin); err != nil {
		return domain.ScheduleVersion{}, err
	}

	// 2. Validate state & precondition
	if ver.Status != domain.VersionStatusReview {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: can only publish version in REVIEW status", ErrInvalidState)
	}
	if ifMatchVersion <= 0 {
		return domain.ScheduleVersion{}, ErrPreconditionRequired
	}
	if ver.Version != ifMatchVersion {
		return domain.ScheduleVersion{}, ErrConflict
	}

	// 3. Load snapshot & assignments to perform authoritative verification before publishing
	snap, err := s.repos.Snapshots.GetByID(ctx, ver.SnapshotID)
	if err != nil {
		return domain.ScheduleVersion{}, fmt.Errorf("load snapshot: %w", err)
	}

	assignments, err := s.repos.ScheduleAssignments.ListByVersion(ctx, versionID)
	if err != nil {
		return domain.ScheduleVersion{}, fmt.Errorf("load assignments: %w", err)
	}

	solutionJSON, err := buildSolutionJSON(assignments, ver.Score)
	if err != nil {
		return domain.ScheduleVersion{}, fmt.Errorf("build solution JSON: %w", err)
	}

	verifyResp, err := s.adapter.Verify(ctx, curra.VerifyRequest{
		ProblemJSON:     snap.ProblemJSON,
		SolutionJSON:    solutionJSON,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	if err != nil {
		return domain.ScheduleVersion{}, fmt.Errorf("verification error: %w", err)
	}
	if !verifyResp.Valid || verifyResp.Score.HardViolations > 0 {
		return domain.ScheduleVersion{}, fmt.Errorf("%w: cannot publish invalid solution with %d hard violations", ErrValidation, verifyResp.Score.HardViolations)
	}

	// 4. Execute atomic CAS status update, timetable pointer update, and audit log in a single transaction
	audit := domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: ver.InstitutionID,
		UserID:        &publishedBy,
		Action:        "version.publish",
		ResourceType:  "schedule_version",
		ResourceID:    versionID,
		Details:       json.RawMessage(fmt.Sprintf(`{"timetableId":"%s","snapshotId":"%s"}`, ver.TimetableID, ver.SnapshotID)),
		CreatedAt:     time.Now(),
	}

	if err := s.repos.ScheduleVersions.PublishTx(ctx, versionID, ifMatchVersion, ver.TimetableID, audit); err != nil {
		if errors.Is(err, repositories.ErrOptimisticLock) {
			return domain.ScheduleVersion{}, ErrConflict
		}
		return domain.ScheduleVersion{}, fmt.Errorf("publish tx: %w", err)
	}

	return s.repos.ScheduleVersions.GetByID(ctx, versionID)
}
