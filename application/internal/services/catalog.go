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

func (s *CatalogService) CreateFaculty(ctx context.Context, instID, departmentID uuid.UUID, name string) (domain.Faculty, error) {
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
		DepartmentID:  &departmentID,
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

func (s *CatalogService) GetRoom(ctx context.Context, instID, roomID uuid.UUID) (domain.Room, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Room{}, err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return domain.Room{}, err
	}
	if room.InstitutionID != instID {
		return domain.Room{}, ErrNotFound
	}
	return room, nil
}

func (s *CatalogService) UpdateRoom(ctx context.Context, instID uuid.UUID, roomID uuid.UUID, name string, capacity int, version int) (domain.Room, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Room{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Room{}, err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return domain.Room{}, err
	}
	if room.InstitutionID != instID {
		return domain.Room{}, ErrNotFound
	}
	if room.Version != version {
		return domain.Room{}, ErrConflict
	}
	room.Name = name
	room.Capacity = capacity
	if err := s.repos.Rooms.Update(ctx, room); err != nil {
		return domain.Room{}, err
	}
	return s.repos.Rooms.GetByID(ctx, roomID)
}

func (s *CatalogService) DeleteRoom(ctx context.Context, instID, roomID uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room.InstitutionID != instID {
		return ErrNotFound
	}
	if room.Version != version {
		return ErrConflict
	}
	return s.repos.Rooms.Delete(ctx, roomID)
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

func (s *CatalogService) GetRoomFeature(ctx context.Context, instID, featureID uuid.UUID) (domain.RoomFeature, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.RoomFeature{}, err
	}
	feature, err := s.repos.RoomFeatures.GetByID(ctx, featureID)
	if err != nil {
		return domain.RoomFeature{}, err
	}
	if feature.InstitutionID != instID {
		return domain.RoomFeature{}, ErrNotFound
	}
	return feature, nil
}

func (s *CatalogService) UpdateRoomFeature(ctx context.Context, instID uuid.UUID, featureID uuid.UUID, name string, version int) (domain.RoomFeature, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.RoomFeature{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.RoomFeature{}, err
	}
	feature, err := s.repos.RoomFeatures.GetByID(ctx, featureID)
	if err != nil {
		return domain.RoomFeature{}, err
	}
	if feature.InstitutionID != instID {
		return domain.RoomFeature{}, ErrNotFound
	}
	if feature.Version != version {
		return domain.RoomFeature{}, ErrConflict
	}
	feature.Name = name
	if err := s.repos.RoomFeatures.Update(ctx, feature); err != nil {
		return domain.RoomFeature{}, err
	}
	return s.repos.RoomFeatures.GetByID(ctx, featureID)
}

func (s *CatalogService) DeleteRoomFeature(ctx context.Context, instID, featureID uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	feature, err := s.repos.RoomFeatures.GetByID(ctx, featureID)
	if err != nil {
		return err
	}
	if feature.InstitutionID != instID {
		return ErrNotFound
	}
	if feature.Version != version {
		return ErrConflict
	}
	return s.repos.RoomFeatures.Delete(ctx, featureID)
}

// Room Feature Assignments
func (s *CatalogService) GetRoomFeatures(ctx context.Context, instID, roomID uuid.UUID) ([]domain.RoomFeature, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return nil, err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.InstitutionID != instID {
		return nil, ErrNotFound
	}
	return s.repos.RoomFeatureAssignments.ListByRoom(ctx, roomID)
}

func (s *CatalogService) AssignRoomFeature(ctx context.Context, instID, roomID, featureID uuid.UUID) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room.InstitutionID != instID {
		return ErrNotFound
	}
	feature, err := s.repos.RoomFeatures.GetByID(ctx, featureID)
	if err != nil {
		return err
	}
	if feature.InstitutionID != instID {
		return ErrNotFound
	}
	return s.repos.RoomFeatureAssignments.Create(ctx, roomID, featureID)
}

func (s *CatalogService) UnassignRoomFeature(ctx context.Context, instID, roomID, featureID uuid.UUID) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	room, err := s.repos.Rooms.GetByID(ctx, roomID)
	if err != nil {
		return err
	}
	if room.InstitutionID != instID {
		return ErrNotFound
	}
	return s.repos.RoomFeatureAssignments.Delete(ctx, roomID, featureID)
}

// Time Slots
func (s *CatalogService) ListTimeSlots(ctx context.Context, instID uuid.UUID) ([]domain.TimeSlot, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.TimeSlots.ListByInstitution(ctx, instID)
}

// Departments
func (s *CatalogService) GetDepartment(ctx context.Context, instID, id uuid.UUID) (domain.Department, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Department{}, err
	}
	dept, err := s.repos.Departments.GetByID(ctx, id)
	if err != nil {
		return domain.Department{}, err
	}
	if dept.InstitutionID != instID {
		return domain.Department{}, ErrNotFound
	}
	return dept, nil
}

func (s *CatalogService) UpdateDepartment(ctx context.Context, instID, id uuid.UUID, name string, version int) (domain.Department, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Department{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Department{}, err
	}
	dept, err := s.repos.Departments.GetByID(ctx, id)
	if err != nil {
		return domain.Department{}, err
	}
	if dept.InstitutionID != instID {
		return domain.Department{}, ErrNotFound
	}
	if dept.Version != version {
		return domain.Department{}, ErrConflict
	}
	dept.Name = name
	if err := s.repos.Departments.Update(ctx, dept); err != nil {
		return domain.Department{}, err
	}
	return s.repos.Departments.GetByID(ctx, id)
}

func (s *CatalogService) DeleteDepartment(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	dept, err := s.repos.Departments.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if dept.InstitutionID != instID {
		return ErrNotFound
	}
	if dept.Version != version {
		return ErrConflict
	}
	return s.repos.Departments.Delete(ctx, id)
}

// Programs
func (s *CatalogService) GetProgram(ctx context.Context, instID, id uuid.UUID) (domain.Program, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Program{}, err
	}
	prog, err := s.repos.Programs.GetByID(ctx, id)
	if err != nil {
		return domain.Program{}, err
	}
	if prog.InstitutionID != instID {
		return domain.Program{}, ErrNotFound
	}
	return prog, nil
}

func (s *CatalogService) UpdateProgram(ctx context.Context, instID, id, deptID uuid.UUID, name string, version int) (domain.Program, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Program{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Program{}, err
	}
	prog, err := s.repos.Programs.GetByID(ctx, id)
	if err != nil {
		return domain.Program{}, err
	}
	if prog.InstitutionID != instID {
		return domain.Program{}, ErrNotFound
	}
	if prog.Version != version {
		return domain.Program{}, ErrConflict
	}
	prog.Name = name
	prog.DepartmentID = deptID
	if err := s.repos.Programs.Update(ctx, prog); err != nil {
		return domain.Program{}, err
	}
	return s.repos.Programs.GetByID(ctx, id)
}

func (s *CatalogService) DeleteProgram(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	prog, err := s.repos.Programs.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if prog.InstitutionID != instID {
		return ErrNotFound
	}
	if prog.Version != version {
		return ErrConflict
	}
	return s.repos.Programs.Delete(ctx, id)
}

// Classes
func (s *CatalogService) GetClass(ctx context.Context, instID, id uuid.UUID) (domain.Class, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Class{}, err
	}
	cls, err := s.repos.Classes.GetByID(ctx, id)
	if err != nil {
		return domain.Class{}, err
	}
	if cls.InstitutionID != instID {
		return domain.Class{}, ErrNotFound
	}
	return cls, nil
}

func (s *CatalogService) UpdateClass(ctx context.Context, instID, id, progID uuid.UUID, name string, version int) (domain.Class, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Class{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Class{}, err
	}
	cls, err := s.repos.Classes.GetByID(ctx, id)
	if err != nil {
		return domain.Class{}, err
	}
	if cls.InstitutionID != instID {
		return domain.Class{}, ErrNotFound
	}
	if cls.Version != version {
		return domain.Class{}, ErrConflict
	}
	cls.Name = name
	cls.ProgramID = progID
	if err := s.repos.Classes.Update(ctx, cls); err != nil {
		return domain.Class{}, err
	}
	return s.repos.Classes.GetByID(ctx, id)
}

func (s *CatalogService) DeleteClass(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	cls, err := s.repos.Classes.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cls.InstitutionID != instID {
		return ErrNotFound
	}
	if cls.Version != version {
		return ErrConflict
	}
	return s.repos.Classes.Delete(ctx, id)
}

// Student Groups
func (s *CatalogService) GetStudentGroup(ctx context.Context, instID, id uuid.UUID) (domain.StudentGroup, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.StudentGroup{}, err
	}
	sg, err := s.repos.StudentGroups.GetByID(ctx, id)
	if err != nil {
		return domain.StudentGroup{}, err
	}
	if sg.InstitutionID != instID {
		return domain.StudentGroup{}, ErrNotFound
	}
	return sg, nil
}

func (s *CatalogService) UpdateStudentGroup(ctx context.Context, instID, id, classID uuid.UUID, name string, size int, isWhole bool, version int) (domain.StudentGroup, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.StudentGroup{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.StudentGroup{}, err
	}
	sg, err := s.repos.StudentGroups.GetByID(ctx, id)
	if err != nil {
		return domain.StudentGroup{}, err
	}
	if sg.InstitutionID != instID {
		return domain.StudentGroup{}, ErrNotFound
	}
	if sg.Version != version {
		return domain.StudentGroup{}, ErrConflict
	}
	sg.Name = name
	sg.ClassID = classID
	sg.Size = size
	sg.IsWholeGroup = isWhole
	if err := s.repos.StudentGroups.Update(ctx, sg); err != nil {
		return domain.StudentGroup{}, err
	}
	return s.repos.StudentGroups.GetByID(ctx, id)
}

func (s *CatalogService) DeleteStudentGroup(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	sg, err := s.repos.StudentGroups.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if sg.InstitutionID != instID {
		return ErrNotFound
	}
	if sg.Version != version {
		return ErrConflict
	}
	return s.repos.StudentGroups.Delete(ctx, id)
}

// Subjects
func (s *CatalogService) GetSubject(ctx context.Context, instID, id uuid.UUID) (domain.Subject, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Subject{}, err
	}
	subj, err := s.repos.Subjects.GetByID(ctx, id)
	if err != nil {
		return domain.Subject{}, err
	}
	if subj.InstitutionID != instID {
		return domain.Subject{}, ErrNotFound
	}
	return subj, nil
}

func (s *CatalogService) UpdateSubject(ctx context.Context, instID, id uuid.UUID, code, name string, version int) (domain.Subject, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Subject{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Subject{}, err
	}
	subj, err := s.repos.Subjects.GetByID(ctx, id)
	if err != nil {
		return domain.Subject{}, err
	}
	if subj.InstitutionID != instID {
		return domain.Subject{}, ErrNotFound
	}
	if subj.Version != version {
		return domain.Subject{}, ErrConflict
	}
	subj.Code = code
	subj.Name = name
	if err := s.repos.Subjects.Update(ctx, subj); err != nil {
		return domain.Subject{}, err
	}
	return s.repos.Subjects.GetByID(ctx, id)
}

func (s *CatalogService) DeleteSubject(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	subj, err := s.repos.Subjects.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if subj.InstitutionID != instID {
		return ErrNotFound
	}
	if subj.Version != version {
		return ErrConflict
	}
	return s.repos.Subjects.Delete(ctx, id)
}

// Academic Years
func (s *CatalogService) GetAcademicYear(ctx context.Context, instID, id uuid.UUID) (domain.AcademicYear, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.AcademicYear{}, err
	}
	ay, err := s.repos.AcademicYears.GetByID(ctx, id)
	if err != nil {
		return domain.AcademicYear{}, err
	}
	if ay.InstitutionID != instID {
		return domain.AcademicYear{}, ErrNotFound
	}
	return ay, nil
}

func (s *CatalogService) UpdateAcademicYear(ctx context.Context, instID, id uuid.UUID, name string, version int) (domain.AcademicYear, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.AcademicYear{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.AcademicYear{}, err
	}
	ay, err := s.repos.AcademicYears.GetByID(ctx, id)
	if err != nil {
		return domain.AcademicYear{}, err
	}
	if ay.InstitutionID != instID {
		return domain.AcademicYear{}, ErrNotFound
	}
	if ay.Version != version {
		return domain.AcademicYear{}, ErrConflict
	}
	ay.Name = name
	if err := s.repos.AcademicYears.Update(ctx, ay); err != nil {
		return domain.AcademicYear{}, err
	}
	return s.repos.AcademicYears.GetByID(ctx, id)
}

func (s *CatalogService) DeleteAcademicYear(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	ay, err := s.repos.AcademicYears.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ay.InstitutionID != instID {
		return ErrNotFound
	}
	if ay.Version != version {
		return ErrConflict
	}
	return s.repos.AcademicYears.Delete(ctx, id)
}

// Terms
func (s *CatalogService) GetTerm(ctx context.Context, instID, id uuid.UUID) (domain.Term, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Term{}, err
	}
	term, err := s.repos.Terms.GetByID(ctx, id)
	if err != nil {
		return domain.Term{}, err
	}
	if term.InstitutionID != instID {
		return domain.Term{}, ErrNotFound
	}
	return term, nil
}

func (s *CatalogService) UpdateTerm(ctx context.Context, instID, id, academicYearID uuid.UUID, name string, version int) (domain.Term, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Term{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Term{}, err
	}
	term, err := s.repos.Terms.GetByID(ctx, id)
	if err != nil {
		return domain.Term{}, err
	}
	if term.InstitutionID != instID {
		return domain.Term{}, ErrNotFound
	}
	if term.Version != version {
		return domain.Term{}, ErrConflict
	}
	term.Name = name
	term.AcademicYearID = academicYearID
	if err := s.repos.Terms.Update(ctx, term); err != nil {
		return domain.Term{}, err
	}
	return s.repos.Terms.GetByID(ctx, id)
}

func (s *CatalogService) DeleteTerm(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	term, err := s.repos.Terms.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if term.InstitutionID != instID {
		return ErrNotFound
	}
	if term.Version != version {
		return ErrConflict
	}
	return s.repos.Terms.Delete(ctx, id)
}

// Time Slots
func (s *CatalogService) GetTimeSlot(ctx context.Context, instID, id uuid.UUID) (domain.TimeSlot, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.TimeSlot{}, err
	}
	ts, err := s.repos.TimeSlots.GetByID(ctx, id)
	if err != nil {
		return domain.TimeSlot{}, err
	}
	if ts.InstitutionID != instID {
		return domain.TimeSlot{}, ErrNotFound
	}
	return ts, nil
}

func (s *CatalogService) UpdateTimeSlot(ctx context.Context, instID, id uuid.UUID, day string, period int, label string, version int) (domain.TimeSlot, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.TimeSlot{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.TimeSlot{}, err
	}
	ts, err := s.repos.TimeSlots.GetByID(ctx, id)
	if err != nil {
		return domain.TimeSlot{}, err
	}
	if ts.InstitutionID != instID {
		return domain.TimeSlot{}, ErrNotFound
	}
	if ts.Version != version {
		return domain.TimeSlot{}, ErrConflict
	}
	ts.Day = day
	ts.Period = period
	ts.Label = label
	if err := s.repos.TimeSlots.Update(ctx, ts); err != nil {
		return domain.TimeSlot{}, err
	}
	return s.repos.TimeSlots.GetByID(ctx, id)
}

func (s *CatalogService) DeleteTimeSlot(ctx context.Context, instID, id uuid.UUID, version int) error {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return err
	}
	ts, err := s.repos.TimeSlots.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ts.InstitutionID != instID {
		return ErrNotFound
	}
	if ts.Version != version {
		return ErrConflict
	}
	return s.repos.TimeSlots.Delete(ctx, id)
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

// Session Requirements
func (s *CatalogService) ListSessionRequirements(ctx context.Context, instID uuid.UUID) ([]domain.SessionRequirement, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.SessionRequirements.ListByInstitution(ctx, instID)
}

// Academic Years
func (s *CatalogService) ListAcademicYears(ctx context.Context, instID uuid.UUID) ([]domain.AcademicYear, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.AcademicYears.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateAcademicYear(ctx context.Context, instID uuid.UUID, name string) (domain.AcademicYear, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.AcademicYear{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.AcademicYear{}, err
	}
	ay := domain.AcademicYear{
		ID:            uuid.New(),
		InstitutionID: instID,
		Name:          name,
		Version:       1,
	}
	if err := s.repos.AcademicYears.Create(ctx, ay); err != nil {
		return domain.AcademicYear{}, err
	}
	return ay, nil
}

// Terms
func (s *CatalogService) ListTerms(ctx context.Context, instID uuid.UUID) ([]domain.Term, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.Terms.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateTerm(ctx context.Context, instID, academicYearID uuid.UUID, name string) (domain.Term, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.Term{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.Term{}, err
	}
	term := domain.Term{
		ID:              uuid.New(),
		InstitutionID:   instID,
		AcademicYearID:  academicYearID,
		Name:            name,
		Version:         1,
	}
	if err := s.repos.Terms.Create(ctx, term); err != nil {
		return domain.Term{}, err
	}
	return term, nil
}

// Course Offerings
func (s *CatalogService) ListCourseOfferings(ctx context.Context, instID uuid.UUID) ([]domain.CourseOffering, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.CourseOfferings.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateCourseOffering(ctx context.Context, instID, termID, classID, subjectID, studentGroupID, facultyID, academicYearID uuid.UUID) (domain.CourseOffering, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.CourseOffering{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.CourseOffering{}, err
	}
	co := domain.CourseOffering{
		ID:             uuid.New(),
		InstitutionID:  instID,
		TermID:         termID,
		ClassID:        classID,
		SubjectID:      subjectID,
		StudentGroupID: studentGroupID,
		FacultyID:      facultyID,
		AcademicYearID: academicYearID,
		Version:        1,
	}
	if err := s.repos.CourseOfferings.Create(ctx, co); err != nil {
		return domain.CourseOffering{}, err
	}
	return co, nil
}

// Session Requirements
func (s *CatalogService) CreateSessionRequirement(ctx context.Context, instID, courseOfferingID uuid.UUID, reqType string, sessionsPerWeek, duration int, consecutive bool) (domain.SessionRequirement, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.SessionRequirement{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.SessionRequirement{}, err
	}
	sr := domain.SessionRequirement{
		ID:                 uuid.New(),
		InstitutionID:      instID,
		CourseOfferingID:   courseOfferingID,
		Type:               reqType,
		SessionsPerWeek:    sessionsPerWeek,
		Duration:           duration,
		Consecutive:        consecutive,
		Version:            1,
	}
	if err := s.repos.SessionRequirements.Create(ctx, sr); err != nil {
		return domain.SessionRequirement{}, err
	}
	return sr, nil
}

// Faculty Availability
func (s *CatalogService) ListFacultyAvailability(ctx context.Context, instID uuid.UUID) ([]domain.FacultyAvailability, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.FacultyAvailability.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateFacultyAvailability(ctx context.Context, instID, facultyID, timeSlotID uuid.UUID) (domain.FacultyAvailability, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.FacultyAvailability{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.FacultyAvailability{}, err
	}
	fa := domain.FacultyAvailability{
		ID:            uuid.New(),
		InstitutionID: instID,
		FacultyID:     facultyID,
		TimeSlotID:    timeSlotID,
	}
	if err := s.repos.FacultyAvailability.Create(ctx, fa); err != nil {
		return domain.FacultyAvailability{}, err
	}
	return fa, nil
}

func (s *CatalogService) DeleteFacultyAvailability(ctx context.Context, id uuid.UUID) error {
	return s.repos.FacultyAvailability.Delete(ctx, id)
}

// Room Availability
func (s *CatalogService) ListRoomAvailability(ctx context.Context, instID uuid.UUID) ([]domain.RoomAvailability, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.RoomAvailability.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateRoomAvailability(ctx context.Context, instID, roomID, timeSlotID uuid.UUID) (domain.RoomAvailability, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.RoomAvailability{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.RoomAvailability{}, err
	}
	ra := domain.RoomAvailability{
		ID:            uuid.New(),
		InstitutionID: instID,
		RoomID:        roomID,
		TimeSlotID:    timeSlotID,
	}
	if err := s.repos.RoomAvailability.Create(ctx, ra); err != nil {
		return domain.RoomAvailability{}, err
	}
	return ra, nil
}

func (s *CatalogService) DeleteRoomAvailability(ctx context.Context, id uuid.UUID) error {
	return s.repos.RoomAvailability.Delete(ctx, id)
}

// Faculty Preferences
func (s *CatalogService) ListFacultyPreferences(ctx context.Context, instID uuid.UUID) ([]domain.FacultyPreference, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return nil, err
	}
	return s.repos.FacultyPreferences.ListByInstitution(ctx, instID)
}

func (s *CatalogService) CreateFacultyPreference(ctx context.Context, instID, facultyID, timeSlotID uuid.UUID, weight int) (domain.FacultyPreference, error) {
	if err := RequireTenantMatch(ctx, instID); err != nil {
		return domain.FacultyPreference{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.FacultyPreference{}, err
	}
	fp := domain.FacultyPreference{
		ID:            uuid.New(),
		InstitutionID: instID,
		FacultyID:     facultyID,
		TimeSlotID:    timeSlotID,
		Weight:        weight,
	}
	if err := s.repos.FacultyPreferences.Create(ctx, fp); err != nil {
		return domain.FacultyPreference{}, err
	}
	return fp, nil
}

func (s *CatalogService) DeleteFacultyPreference(ctx context.Context, id uuid.UUID) error {
	return s.repos.FacultyPreferences.Delete(ctx, id)
}

// Room Features

