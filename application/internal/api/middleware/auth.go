package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
)

// AuthMiddleware extracts user from JWT or test headers and sets domain context.
type AuthMiddleware struct{}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Authorization header or test headers
		authHeader := r.Header.Get("Authorization")
		testInstHeader := r.Header.Get("X-Institution-ID")
		testUserHeader := r.Header.Get("X-User-ID")
		testRoleHeader := r.Header.Get("X-Role")

		var user domain.User
		var inst domain.Institution
		var role domain.UserRole

		if testInstHeader != "" && testUserHeader != "" {
			instID, err1 := uuid.Parse(testInstHeader)
			userID, err2 := uuid.Parse(testUserHeader)
			if err1 == nil && err2 == nil {
				userRole := domain.RoleScheduler
				if testRoleHeader != "" {
					userRole = domain.Role(testRoleHeader)
				}
				user = domain.User{ID: userID, Name: "Test User", Email: "test@institution.edu"}
				inst = domain.Institution{ID: instID, Name: "Test Institution"}
				role = domain.UserRole{ID: uuid.New(), UserID: userID, InstitutionID: instID, Role: userRole}

				ctx := services.ContextWithUser(r.Context(), user)
				ctx = services.ContextWithInstitution(ctx, inst)
				ctx = services.ContextWithRole(ctx, role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		if authHeader == "" {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing authorization header"}}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing bearer token"}}`, http.StatusUnauthorized)
			return
		}

		// Token format for dev / test: "user_id:institution_id:role"
		parts := strings.Split(token, ":")
		if len(parts) >= 2 {
			userID, err1 := uuid.Parse(parts[0])
			instID, err2 := uuid.Parse(parts[1])
			if err1 == nil && err2 == nil {
				userRole := domain.RoleScheduler
				if len(parts) >= 3 {
					userRole = domain.Role(parts[2])
				}
				user = domain.User{ID: userID, Name: "API User", Email: "user@institution.edu"}
				inst = domain.Institution{ID: instID, Name: "API Institution"}
				role = domain.UserRole{ID: uuid.New(), UserID: userID, InstitutionID: instID, Role: userRole}

				ctx := services.ContextWithUser(r.Context(), user)
				ctx = services.ContextWithInstitution(ctx, inst)
				ctx = services.ContextWithRole(ctx, role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`, http.StatusUnauthorized)
	})
}

// AuthenticatedUser extracts the authenticated user from context.
func AuthenticatedUser(ctx context.Context) (domain.User, bool) {
	return services.UserFromContext(ctx)
}
