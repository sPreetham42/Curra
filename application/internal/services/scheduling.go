package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type SchedulingService struct {
	repos *repositories.Repos
}

func NewSchedulingService(repos *repositories.Repos) *SchedulingService {
	return &SchedulingService{repos: repos}
}

func (s *SchedulingService) CreateSchedulingConfig(ctx context.Context, instID uuid.UUID, name string, config domain.SchedulingConfig) (domain.SchedulingConfig, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.SchedulingConfig{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.SchedulingConfig{}, err
	}

	cfg := domain.SchedulingConfig{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Config:        config.Config,
		Version:       1,
	}

	if err := s.repos.SchedulingConfigs.Create(ctx, cfg); err != nil {
		return domain.SchedulingConfig{}, err
	}
	return cfg, nil
}

func (s *SchedulingService) GetSchedulingConfig(ctx context.Context, instID, configID uuid.UUID) (domain.SchedulingConfig, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.SchedulingConfig{}, err
	}
	cfg, err := s.repos.SchedulingConfigs.GetByID(ctx, configID)
	if err != nil {
		return domain.SchedulingConfig{}, err
	}
	if cfg.InstitutionID != instID {
		return domain.SchedulingConfig{}, ErrNotFound
	}
	return cfg, nil
}

func (s *SchedulingService) ListSchedulingConfigs(ctx context.Context, instID uuid.UUID) ([]domain.SchedulingConfig, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.SchedulingConfigs.ListByInstitution(ctx, instID)
}

func (s *SchedulingService) UpdateSchedulingConfig(ctx context.Context, instID uuid.UUID, configID uuid.UUID, name string, configJSON json.RawMessage, version int) (domain.SchedulingConfig, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.SchedulingConfig{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.SchedulingConfig{}, err
	}

	cfg, err := s.repos.SchedulingConfigs.GetByID(ctx, configID)
	if err != nil {
		return domain.SchedulingConfig{}, err
	}
	if cfg.InstitutionID != instID {
		return domain.SchedulingConfig{}, ErrNotFound
	}
	if cfg.Version != version {
		return domain.SchedulingConfig{}, ErrConflict
	}

	cfg.Name = name
	cfg.Config = configJSON
	if err := s.repos.SchedulingConfigs.Update(ctx, cfg); err != nil {
		return domain.SchedulingConfig{}, err
	}
	return s.repos.SchedulingConfigs.GetByID(ctx, configID)
}

func (s *SchedulingService) DeleteSchedulingConfig(ctx context.Context, instID, configID uuid.UUID, ifMatch int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}

	cfg, err := s.repos.SchedulingConfigs.GetByID(ctx, configID)
	if err != nil {
		return err
	}
	if cfg.InstitutionID != instID {
		return ErrNotFound
	}
	if cfg.Version != ifMatch {
		return ErrConflict
	}

	return s.repos.SchedulingConfigs.Delete(ctx, configID)
}