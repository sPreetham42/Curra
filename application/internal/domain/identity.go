package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleInstitutionAdmin Role = "INSTITUTION_ADMIN"
	RoleScheduler        Role = "SCHEDULER"
	RoleProfessor        Role = "PROFESSOR"
	RoleViewer           Role = "VIEWER"
)

type Institution struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Settings  map[string]any
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	AvatarURL string
	GoogleID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserRole struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	InstitutionID  uuid.UUID
	Role           Role
	FacultyID      string // optional: links professor role to a Faculty entity
	CreatedAt      time.Time
}
