package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ScheduleRunStatus string

const (
	StatusQueued           ScheduleRunStatus = "QUEUED"
	StatusRunning          ScheduleRunStatus = "RUNNING"
	StatusSolved           ScheduleRunStatus = "SOLVED"
	StatusInfeasible       ScheduleRunStatus = "INFEASIBLE"
	StatusInvalidProblem   ScheduleRunStatus = "INVALID_PROBLEM"
	StatusInvalidResult    ScheduleRunStatus = "INVALID_RESULT"
	StatusCancelled        ScheduleRunStatus = "CANCELLED"
	StatusDeadlineExceeded ScheduleRunStatus = "DEADLINE_EXCEEDED"
	StatusNodeLimit        ScheduleRunStatus = "NODE_LIMIT"
	StatusFailed           ScheduleRunStatus = "FAILED"
)

type ScheduleVersionStatus string

const (
	VersionStatusDraft     ScheduleVersionStatus = "DRAFT"
	VersionStatusReview    ScheduleVersionStatus = "REVIEW"
	VersionStatusPublished ScheduleVersionStatus = "PUBLISHED"
	VersionStatusArchived  ScheduleVersionStatus = "ARCHIVED"
)

type Timetable struct {
	ID                        uuid.UUID
	InstitutionID             uuid.UUID
	Name                      string
	CurrentPublishedVersionID *uuid.UUID
	Version                   int
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type ProblemSnapshot struct {
	ID                  uuid.UUID
	TimetableID         uuid.UUID
	InstitutionID       uuid.UUID
	SchemaVersion       int
	ProblemJSON         json.RawMessage
	ConstraintInstances json.RawMessage
	SolverConfig        json.RawMessage
	ObjectiveConfig     json.RawMessage
	InputHash           string
	CreatedBy           uuid.UUID
	CreatedAt           time.Time
}

type ScheduleRun struct {
	ID              uuid.UUID
	TimetableID     uuid.UUID
	InstitutionID   uuid.UUID
	SnapshotID      uuid.UUID
	Status          ScheduleRunStatus
	SolverConfig    json.RawMessage
	ObjectiveConfig json.RawMessage
	Seed            *int64
	RuleSetHash     *string
	CurrAVersion    *string
	CurrACommit     *string
	Result          json.RawMessage
	Diagnostics     json.RawMessage
	Score           json.RawMessage
	Violations      json.RawMessage
	WorkerID        *string
	LeaseExpiresAt  *time.Time
	RetryCount      int
	HeartbeatAt     *time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	DurationMs      *int64
	CreatedBy       uuid.UUID
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ScheduleVersion struct {
	ID            uuid.UUID
	TimetableID   uuid.UUID
	InstitutionID uuid.UUID
	SourceRunID   *uuid.UUID
	SnapshotID    uuid.UUID
	Status        ScheduleVersionStatus
	Name          string
	Score         json.RawMessage
	Version       int
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ScheduleAssignment struct {
	ID                   uuid.UUID
	VersionID            uuid.UUID
	AssignmentID         string
	CourseOfferingID     string
	SessionRequirementID string
	StudentGroupID       string
	FacultyID            string
	RoomID               string
	TimeSlotID           string
	Instance             int
	CreatedAt            time.Time
}

type AssignmentPin struct {
	ID           uuid.UUID
	VersionID    uuid.UUID
	AssignmentID string
	PinnedBy     uuid.UUID
	CreatedAt    time.Time
}

type ImportBatchStatus string

const (
	ImportStatusPending    ImportBatchStatus = "PENDING"
	ImportStatusParsing    ImportBatchStatus = "PARSING"
	ImportStatusStaged     ImportBatchStatus = "STAGED"
	ImportStatusValidating ImportBatchStatus = "VALIDATING"
	ImportStatusReady      ImportBatchStatus = "READY"
	ImportStatusCommitted  ImportBatchStatus = "COMMITTED"
	ImportStatusFailed     ImportBatchStatus = "FAILED"
	ImportStatusCancelled  ImportBatchStatus = "CANCELLED"
)

type ImportBatch struct {
	ID             uuid.UUID
	TimetableID    uuid.UUID
	InstitutionID  uuid.UUID
	SourceType     string
	SourceFilename *string
	Status         ImportBatchStatus
	TotalRows      int
	ValidRows      int
	ErrorRows      int
	ErrorSummary   json.RawMessage
	CreatedBy      uuid.UUID
	Version        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ImportRowStatus string

const (
	ImportRowPending ImportRowStatus = "PENDING"
	ImportRowValid   ImportRowStatus = "VALID"
	ImportRowError   ImportRowStatus = "ERROR"
)

type ImportRow struct {
	ID         uuid.UUID
	BatchID    uuid.UUID
	RowNumber  int
	RawData    json.RawMessage
	ParsedData json.RawMessage
	Status     ImportRowStatus
	Errors     json.RawMessage
	CreatedAt  time.Time
}

type AuditEvent struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	UserID        *uuid.UUID
	Action        string
	ResourceType  string
	ResourceID    uuid.UUID
	Details       json.RawMessage
	CreatedAt     time.Time
}

type IdempotencyKeyStatus string

const (
	IdempotencyStatusInProgress IdempotencyKeyStatus = "IN_PROGRESS"
	IdempotencyStatusCompleted  IdempotencyKeyStatus = "COMPLETED"
	IdempotencyStatusFailed     IdempotencyKeyStatus = "FAILED"
)

type IdempotencyKey struct {
	ID             uuid.UUID
	InstitutionID  uuid.UUID
	IdempotencyKey string
	Status         IdempotencyKeyStatus
	ResourceType   string
	ResourceID     *uuid.UUID
	ResponseCode   *int
	ResponseBody   json.RawMessage
	LockToken      *uuid.UUID
	LockedAt       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PlacementDTO struct {
	RoomID     string `json:"roomId"`
	TimeSlotID string `json:"timeSlotId"`
}

type MoveDTO struct {
	AssignmentID string       `json:"assignmentId"`
	From         PlacementDTO `json:"from"`
	To           PlacementDTO `json:"to"`
}

type SwapDTO struct {
	Assignment1ID string       `json:"assignment1Id"`
	Assignment2ID string       `json:"assignment2Id"`
	Placement1    PlacementDTO `json:"placement1"`
	Placement2    PlacementDTO `json:"placement2"`
}
