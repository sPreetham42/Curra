package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type contextKey string

const (
	userKey        contextKey = "user"
	institutionKey contextKey = "institution"
	roleKey        contextKey = "role"
)

var (
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict: resource version mismatch")
	ErrPreconditionRequired = errors.New("precondition required: If-Match header missing")
	ErrInvalidState         = errors.New("invalid state transition")
	ErrValidation           = errors.New("validation error")
)

func ContextWithUser(ctx context.Context, user domain.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func ContextWithInstitution(ctx context.Context, inst domain.Institution) context.Context {
	return context.WithValue(ctx, institutionKey, inst)
}

func ContextWithRole(ctx context.Context, role domain.UserRole) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userKey).(domain.User)
	return u, ok
}

func InstitutionFromContext(ctx context.Context) (domain.Institution, bool) {
	i, ok := ctx.Value(institutionKey).(domain.Institution)
	return i, ok
}

func RoleFromContext(ctx context.Context) (domain.UserRole, bool) {
	r, ok := ctx.Value(roleKey).(domain.UserRole)
	return r, ok
}

func InstitutionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	inst, ok := InstitutionFromContext(ctx)
	return inst.ID, ok
}

// RequireTenantMatch verifies that the caller's institution matches the resource's institution.
// If not matching or if institution is missing, it returns ErrNotFound (404) to avoid leaking resource existence.
func RequireTenantMatch(ctx context.Context, resourceInstitutionID uuid.UUID) error {
	callerInstID, ok := InstitutionIDFromContext(ctx)
	if !ok || callerInstID != resourceInstitutionID {
		return ErrNotFound
	}
	return nil
}

func RequireRole(ctx context.Context, roles ...domain.Role) error {
	role, ok := RoleFromContext(ctx)
	if !ok {
		return ErrUnauthorized
	}
	for _, r := range roles {
		if role.Role == r {
			return nil
		}
	}
	return ErrForbidden
}
