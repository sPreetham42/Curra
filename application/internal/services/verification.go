package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
)

type VerificationService struct {
	repos   *repositories.Repos
	adapter curra.CurraAdapter
}

func NewVerificationService(repos *repositories.Repos, adapter curra.CurraAdapter) *VerificationService {
	return &VerificationService{repos: repos, adapter: adapter}
}

func (s *VerificationService) VerifyVersion(ctx context.Context, snapshotID, versionID uuid.UUID) (curra.VerifyResponse, error) {
	// Load snapshot
	snap, err := s.repos.Snapshots.GetByID(ctx, snapshotID)
	if err != nil {
		return curra.VerifyResponse{}, err
	}
	if err := RequireTenantMatch(ctx, snap.InstitutionID); err != nil {
		return curra.VerifyResponse{}, err
	}

	// Load version
	ver, err := s.repos.ScheduleVersions.GetByID(ctx, versionID)
	if err != nil {
		return curra.VerifyResponse{}, err
	}
	if err := RequireTenantMatch(ctx, ver.InstitutionID); err != nil {
		return curra.VerifyResponse{}, err
	}

	// Load assignments
	assignments, err := s.repos.ScheduleAssignments.ListByVersion(ctx, versionID)
	if err != nil {
		return curra.VerifyResponse{}, fmt.Errorf("load assignments: %w", err)
	}

	solutionJSON, err := buildSolutionJSON(assignments, ver.Score)
	if err != nil {
		return curra.VerifyResponse{}, fmt.Errorf("build solution JSON: %w", err)
	}

	return s.adapter.Verify(ctx, curra.VerifyRequest{
		ProblemJSON:     snap.ProblemJSON,
		SolutionJSON:    solutionJSON,
		ConstraintsJSON: snap.ConstraintInstances,
	})
}
