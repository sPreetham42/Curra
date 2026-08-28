package domain

import (
	"time"

	"github.com/google/uuid"
)

type Department struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Program struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	DepartmentID  uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Class struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	ProgramID     uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type StudentGroup struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	ClassID       uuid.UUID
	Name          string
	Size          int
	IsWholeGroup  bool
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Subject struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Code          string
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Faculty struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Room struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Name          string
	Capacity      int
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RoomFeature struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TimeSlot struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Day           string
	Period        int
	Label         string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AcademicYear struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	Name          string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Term struct {
	ID              uuid.UUID
	InstitutionID   uuid.UUID
	AcademicYearID  uuid.UUID
	Name            string
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CourseOffering struct {
	ID                     uuid.UUID
	InstitutionID          uuid.UUID
	TermID                 uuid.UUID
	ClassID                uuid.UUID
	SubjectID              uuid.UUID
	StudentGroupID         uuid.UUID
	FacultyID              uuid.UUID
	RequiredRoomFeatureIDs []uuid.UUID
	Version                int
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type SessionRequirement struct {
	ID                     uuid.UUID
	InstitutionID          uuid.UUID
	CourseOfferingID       uuid.UUID
	Type                   string // THEORY or LAB
	SessionsPerWeek        int
	Duration               int
	Consecutive            bool
	RequiredRoomFeatureIDs []uuid.UUID
	Version                int
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type FacultyAvailability struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	FacultyID     uuid.UUID
	TimeSlotID    uuid.UUID
	CreatedAt     time.Time
}

type RoomAvailability struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	RoomID        uuid.UUID
	TimeSlotID    uuid.UUID
	CreatedAt     time.Time
}

type FacultyPreference struct {
	ID            uuid.UUID
	InstitutionID uuid.UUID
	FacultyID     uuid.UUID
	TimeSlotID    uuid.UUID
	Weight        int
	CreatedAt     time.Time
}
