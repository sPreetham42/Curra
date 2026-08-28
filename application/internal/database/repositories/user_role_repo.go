package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type userRoleRepo struct {
	pool *pgxpool.Pool
}

func (r *userRoleRepo) Create(ctx context.Context, role domain.UserRole) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_roles (id, user_id, institution_id, role, faculty_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, institution_id) DO UPDATE SET role = $4`,
		role.ID, role.UserID, role.InstitutionID, role.Role, role.FacultyID)
	return err
}

func (r *userRoleRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, institution_id, role, faculty_id, created_at
		 FROM user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserRole
	for rows.Next() {
		var role domain.UserRole
		if err := rows.Scan(&role.ID, &role.UserID, &role.InstitutionID,
			&role.Role, &role.FacultyID, &role.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, nil
}

func (r *userRoleRepo) GetByUserAndInstitution(ctx context.Context, userID, instID uuid.UUID) (domain.UserRole, error) {
	var role domain.UserRole
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, institution_id, role, faculty_id, created_at
		 FROM user_roles WHERE user_id = $1 AND institution_id = $2`,
		userID, instID).Scan(
		&role.ID, &role.UserID, &role.InstitutionID,
		&role.Role, &role.FacultyID, &role.CreatedAt)
	return role, err
}
