package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type CatalogService struct {
	repos *repositories.Repos
}

func NewCatalogService(repos *repositories.Repos) *CatalogService {
	return &CatalogService{repos: repos}
}

// Departments
func (s *CatalogService) ListDepartments(ctx context.Context, instID uuid.UUID) ([]domain.Department, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Departments.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateDepartment(ctx context.Context, instID uuid.UUID, name string) (domain.Department, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Department{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Department{}, err
	}
	dept := domain.Department{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.Departments.Create(ctx, dept); err != nil {
		return domain.Department{}, err
	}
	return dept, nil
}

// Programs
func (s *CatalogService) ListPrograms(ctx context.Context, instID uuid.UUID) ([]domain.Program, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Programs.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateProgram(ctx context.Context, instID, deptID uuid.UUID, name string) (domain.Program, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Program{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Program{}, err
	}
	prog := domain.Program{
		ID:            uuid.New(),
		InstitutionID: instID,
		DepartmentID:  deptID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.Programs.Create(ctx, prog); err != nil {
		return domain.Program{}, err
	}
	return prog, nil
}

// Classes
func (s *CatalogService) ListClasses(ctx context.Context, instID uuid.UUID) ([]domain.Class, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Classes.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateClass(ctx context.Context, instID, progID uuid.UUID, name string) (domain.Class, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Class{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Class{}, err
	}
	cls := domain.Class{
		ID:            uuid.New(),
		InstitutionID: instID,
		ProgramID:     progID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.Classes.Create(ctx, cls); err != nil {
		return domain.Class{}, err
	}
	return cls, nil
}

// Student Groups
func (s *CatalogService) ListStudentGroups(ctx context.Context, instID uuid.UUID) ([]domain.StudentGroup, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.StudentGroups.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateStudentGroup(ctx context.Context, instID, classID uuid.UUID, name string, size int, isWhole bool) (domain.StudentGroup, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.StudentGroup{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.StudentGroup{}, err
	}
	sg := domain.StudentGroup{
		ID:            uuid.New(),
		InstitutionID: instID,
		ClassID:       classID,
		Name:          name,
		Size:          size,
		IsWholeGroup:  isWhole,
		Version:       1,
	}
	if err := s.repos.StudentGroups.Create(ctx, sg); err != nil {
		return domain.StudentGroup{}, err
	}
	return sg, nil
}

// Subjects
func (s *CatalogService) ListSubjects(ctx context.Context, instID uuid.UUID) ([]domain.Subject, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Subjects.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateSubject(ctx context.Context, instID uuid.UUID, code, name string) (domain.Subject, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Subject{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Subject{}, err
	}
	subj := domain.Subject{
		ID:            uuid.New(),
		InstitutionID: instID,
		Code:          code,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.Subjects.Create(ctx, subj); err != nil {
		return domain.Subject{}, err
	}
	return subj, nil
}

// Faculty
func (s *CatalogService) ListFaculty(ctx context.Context, instID uuid.UUID) ([]domain.Faculty, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Faculty.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateFaculty(ctx context.Context, instID uuid.UUID, name string) (domain.Faculty, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Faculty{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Faculty{}, err
	}
	fac := domain.Faculty{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.Faculty.Create(ctx, fac); err != nil {
		return domain.Faculty{}, err
	}
	return fac, nil
}

// Rooms
func (s *CatalogService) ListRooms(ctx context.Context, instID uuid.UUID) ([]domain.Room, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Rooms.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateRoom(ctx context.Context, instID uuid.UUID, name string, capacity int) (domain.Room, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Room{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Room{}, err
	}
	room := domain.Room{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Capacity:      capacity,
		Version:       1,
	}
	if err := s.repos.Rooms.Create(ctx, room); err != nil {
		return domain.Room{}, err
	}
	return room, nil
}

// Room Features
func (s *CatalogService) ListRoomFeatures(ctx context.Context, instID uuid.UUID) ([]domain.RoomFeature, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.RoomFeatures.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateRoomFeature(ctx context.Context, instID uuid.UUID, name string) (domain.RoomFeature, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.RoomFeature{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.RoomFeature{}, err
	}
	rf := domain.RoomFeature{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.RoomFeatures.Create(ctx, rf); err != nil {
		return domain.RoomFeature{}, err
	}
	return rf, nil
}

// Time Slots
func (s *CatalogService) ListTimeSlots(ctx context.Context, instID uuid.UUID) ([]domain.TimeSlot, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.TimeSlots.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateTimeSlot(ctx context.Context, instID uuid.UUID, day string, period int, label string) (domain.TimeSlot, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.TimeSlot{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.TimeSlot{}, err
	}
	ts := domain.TimeSlot{
		ID:            uuid.New(),
		InstitutionID: instID,
		Day:           day,
		Period:        period,
		Label:         label,
		Version:       1,
	}
	if err := s.repos.TimeSlots.Create(ctx, ts); err != nil {
		return domain.TimeSlot{}, err
	}
	return ts, nil
}

// Course Offerings
func (s *CatalogService) ListCourseOfferings(ctx context.Context, instID uuid.UUID) ([]domain.CourseOffering, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.CourseOfferings.ListByInstitution(ctx, instID)
}

// Session Requirements
func (s *CatalogService) ListSessionRequirements(ctx context.Context, instID uuid.UUID) ([]domain.SessionRequirement, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.SessionRequirements.ListByInstitution(ctx, instID)
}
