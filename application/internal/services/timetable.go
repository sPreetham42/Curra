package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type TimetableService struct {
	repos *repositories.Repos
}

func NewTimetableService(repos *repositories.Repos) *TimetableService {
	return &TimetableService{repos: repos}
}

func (s *TimetableService) Create(ctx context.Context, name string) (domain.Timetable, error) {
	instID, ok := InstitutionIDFromContext(ctx)
	if !ok {
		return domain.Timetable{}, ErrUnauthorized
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Timetable{}, err
	}

	tt := domain.Timetable{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Version:       1,
	}

	if err := s.repos.Timetables.Create(ctx, tt); err != nil {
		return domain.Timetable{}, err
	}
	return tt, nil
}

func (s *TimetableService) GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, id)
	if err != nil {
		return domain.Timetable{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.Timetable{}, err
	}
	return tt, nil
}

func (s *TimetableService) ListByInstitution(ctx context.Context) ([]domain.Timetable, error) {
	instID, ok := InstitutionIDFromContext(ctx)
	if !ok {
		return nil, ErrUnauthorized
	}
	return s.repos.Timetables.ListByInstitution(ctx, instID)
}

func (s *TimetableService) Update(ctx context.Context, id uuid.UUID, name string, ifMatchVersion int) (domain.Timetable, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, id)
	if err != nil {
		return domain.Timetable{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.Timetable{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Timetable{}, err
	}
	if ifMatchVersion <= 0 {
		return domain.Timetable{}, ErrPreconditionRequired
	}
	if tt.Version != ifMatchVersion {
		return domain.Timetable{}, ErrConflict
	}
	tt.Name = name
	if err := s.repos.Timetables.Update(ctx, tt); err != nil {
		if err == repositories.ErrOptimisticLock {
			return domain.Timetable{}, ErrConflict
		}
		return domain.Timetable{}, err
	}
	return s.repos.Timetables.GetByID(ctx, id)
}
