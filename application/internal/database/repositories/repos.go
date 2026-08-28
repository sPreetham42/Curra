package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type InstitutionRepo interface {
	Create(ctx context.Context, inst domain.Institution) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Institution, error)
}

type UserRepo interface {
	Create(ctx context.Context, user domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByGoogleID(ctx context.Context, googleID string) (domain.User, error)
}

type UserRoleRepo interface {
	Create(ctx context.Context, role domain.UserRole) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserRole, error)
	GetByUserAndInstitution(ctx context.Context, userID, instID uuid.UUID) (domain.UserRole, error)
}

type DepartmentRepo interface {
	Create(ctx context.Context, dept domain.Department) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Department, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Department, error)
	Update(ctx context.Context, dept domain.Department) error
}

type ProgramRepo interface {
	Create(ctx context.Context, prog domain.Program) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Program, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Program, error)
	Update(ctx context.Context, prog domain.Program) error
}

type ClassRepo interface {
	Create(ctx context.Context, class domain.Class) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Class, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Class, error)
	Update(ctx context.Context, class domain.Class) error
}

type StudentGroupRepo interface {
	Create(ctx context.Context, sg domain.StudentGroup) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.StudentGroup, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.StudentGroup, error)
	Update(ctx context.Context, sg domain.StudentGroup) error
}

type SubjectRepo interface {
	Create(ctx context.Context, subj domain.Subject) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Subject, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Subject, error)
	Update(ctx context.Context, subj domain.Subject) error
}

type FacultyRepo interface {
	Create(ctx context.Context, fac domain.Faculty) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Faculty, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Faculty, error)
	Update(ctx context.Context, fac domain.Faculty) error
}

type RoomRepo interface {
	Create(ctx context.Context, room domain.Room) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Room, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Room, error)
	Update(ctx context.Context, room domain.Room) error
}

type RoomFeatureRepo interface {
	Create(ctx context.Context, rf domain.RoomFeature) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.RoomFeature, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.RoomFeature, error)
	Update(ctx context.Context, rf domain.RoomFeature) error
}

type TimeSlotRepo interface {
	Create(ctx context.Context, ts domain.TimeSlot) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.TimeSlot, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.TimeSlot, error)
	Update(ctx context.Context, ts domain.TimeSlot) error
}

type AcademicYearRepo interface {
	Create(ctx context.Context, ay domain.AcademicYear) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.AcademicYear, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.AcademicYear, error)
	Update(ctx context.Context, ay domain.AcademicYear) error
}

type TermRepo interface {
	Create(ctx context.Context, term domain.Term) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Term, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Term, error)
	Update(ctx context.Context, term domain.Term) error
}

type CourseOfferingRepo interface {
	Create(ctx context.Context, co domain.CourseOffering) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.CourseOffering, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.CourseOffering, error)
	Update(ctx context.Context, co domain.CourseOffering) error
	SetFeatures(ctx context.Context, offeringID uuid.UUID, featureIDs []uuid.UUID) error
	GetFeatures(ctx context.Context, offeringID uuid.UUID) ([]uuid.UUID, error)
}

type SessionRequirementRepo interface {
	Create(ctx context.Context, sr domain.SessionRequirement) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.SessionRequirement, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.SessionRequirement, error)
	ListByOffering(ctx context.Context, offeringID uuid.UUID) ([]domain.SessionRequirement, error)
	Update(ctx context.Context, sr domain.SessionRequirement) error
	SetFeatures(ctx context.Context, reqID uuid.UUID, featureIDs []uuid.UUID) error
	GetFeatures(ctx context.Context, reqID uuid.UUID) ([]uuid.UUID, error)
}

type FacultyAvailabilityRepo interface {
	Create(ctx context.Context, fa domain.FacultyAvailability) error
	ListByFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.FacultyAvailability, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.FacultyAvailability, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type RoomAvailabilityRepo interface {
	Create(ctx context.Context, ra domain.RoomAvailability) error
	ListByRoom(ctx context.Context, roomID uuid.UUID) ([]domain.RoomAvailability, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.RoomAvailability, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type FacultyPreferenceRepo interface {
	Create(ctx context.Context, fp domain.FacultyPreference) error
	ListByFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.FacultyPreference, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.FacultyPreference, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type TimetableRepo interface {
	Create(ctx context.Context, tt domain.Timetable) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error)
	ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Timetable, error)
	Update(ctx context.Context, tt domain.Timetable) error
	SetCurrentPublishedVersion(ctx context.Context, timetableID, versionID uuid.UUID) error
}

type ProblemSnapshotRepo interface {
	Create(ctx context.Context, snap domain.ProblemSnapshot) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error)
	ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error)
}

type ScheduleRunRepo interface {
	Create(ctx context.Context, run domain.ScheduleRun) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error)
	ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleRun, error)
	Update(ctx context.Context, run domain.ScheduleRun) error
	ClaimQueued(ctx context.Context, workerID string, leaseDuration time.Duration) (*domain.ScheduleRun, bool, error)
	UpdateTerminalResult(ctx context.Context, id uuid.UUID, workerID string, status domain.ScheduleRunStatus, result, score, diagnostics, violations json.RawMessage, durationMs int64, curraVer, curraCommit, ruleSetHash *string) error
	CommitTerminalResultTx(ctx context.Context, runID uuid.UUID, workerID string, status domain.ScheduleRunStatus, result, score, diagnostics, violations json.RawMessage, durationMs int64, curraVer, curraCommit, ruleSetHash *string, draftVersion *domain.ScheduleVersion, assignments []domain.ScheduleAssignment, audit domain.AuditEvent) error
	UpdateHeartbeat(ctx context.Context, runID uuid.UUID, workerID string) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, updates map[string]any) error
	Cancel(ctx context.Context, id uuid.UUID) (bool, error)
	RecoverExpired(ctx context.Context, maxRetries int) (int, error)
}

type ScheduleVersionRepo interface {
	Create(ctx context.Context, ver domain.ScheduleVersion) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, error)
	ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleVersionStatus, expectedVersion int) error
	Update(ctx context.Context, ver domain.ScheduleVersion) error
	ApplyAssignmentUpdateTx(ctx context.Context, versionID uuid.UUID, expectedVersion int, scoreJSON json.RawMessage, assignments []domain.ScheduleAssignment, audit domain.AuditEvent) error
	PublishTx(ctx context.Context, versionID uuid.UUID, expectedVersion int, timetableID uuid.UUID, audit domain.AuditEvent) error
}

type ScheduleAssignmentRepo interface {
	Create(ctx context.Context, a domain.ScheduleAssignment) error
	CreateBatch(ctx context.Context, assignments []domain.ScheduleAssignment) error
	ReplaceAllForVersion(ctx context.Context, versionID uuid.UUID, assignments []domain.ScheduleAssignment) error
	ListByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.ScheduleAssignment, error)
	DeleteByVersion(ctx context.Context, versionID uuid.UUID) error
}

type AuditEventRepo interface {
	Create(ctx context.Context, event domain.AuditEvent) error
	ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error)
}

// Repos aggregates all repositories.
type Repos struct {
	Institutions        InstitutionRepo
	Users               UserRepo
	UserRoles           UserRoleRepo
	Departments         DepartmentRepo
	Programs            ProgramRepo
	Classes             ClassRepo
	StudentGroups       StudentGroupRepo
	Subjects            SubjectRepo
	Faculty             FacultyRepo
	Rooms               RoomRepo
	RoomFeatures        RoomFeatureRepo
	TimeSlots           TimeSlotRepo
	AcademicYears       AcademicYearRepo
	Terms               TermRepo
	CourseOfferings     CourseOfferingRepo
	SessionRequirements SessionRequirementRepo
	FacultyAvailability FacultyAvailabilityRepo
	RoomAvailability    RoomAvailabilityRepo
	FacultyPreferences  FacultyPreferenceRepo
	Timetables          TimetableRepo
	Snapshots           ProblemSnapshotRepo
	ScheduleRuns        ScheduleRunRepo
	ScheduleVersions    ScheduleVersionRepo
	ScheduleAssignments ScheduleAssignmentRepo
	AuditEvents         AuditEventRepo
	Idempotency         IdempotencyRepo
}

// NewRepos creates all repository implementations backed by PostgreSQL.
func NewRepos(pool *pgxpool.Pool) *Repos {
	return &Repos{
		Institutions:        &institutionRepo{pool: pool},
		Users:               &userRepo{pool: pool},
		UserRoles:           &userRoleRepo{pool: pool},
		Departments:         &departmentRepo{pool: pool},
		Programs:            &programRepo{pool: pool},
		Classes:             &classRepo{pool: pool},
		StudentGroups:       &studentGroupRepo{pool: pool},
		Subjects:            &subjectRepo{pool: pool},
		Faculty:             &facultyRepo{pool: pool},
		Rooms:               &roomRepo{pool: pool},
		RoomFeatures:        &roomFeatureRepo{pool: pool},
		TimeSlots:           &timeSlotRepo{pool: pool},
		AcademicYears:       &academicYearRepo{pool: pool},
		Terms:               &termRepo{pool: pool},
		CourseOfferings:     &courseOfferingRepo{pool: pool},
		SessionRequirements: &sessionRequirementRepo{pool: pool},
		FacultyAvailability: &facultyAvailabilityRepo{pool: pool},
		RoomAvailability:    &roomAvailabilityRepo{pool: pool},
		FacultyPreferences:  &facultyPreferenceRepo{pool: pool},
		Timetables:          &timetableRepo{pool: pool},
		Snapshots:           &snapshotRepo{pool: pool},
		ScheduleRuns:        &scheduleRunRepo{pool: pool},
		ScheduleVersions:    &scheduleVersionRepo{pool: pool},
		ScheduleAssignments: &scheduleAssignmentRepo{pool: pool},
		AuditEvents:         &auditEventRepo{pool: pool},
		Idempotency:         &idempotencyRepo{pool: pool},
	}
}
