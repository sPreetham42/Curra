package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type userRepo struct {
	pool *pgxpool.Pool
}

func (r *userRepo) Create(ctx context.Context, user domain.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, name, avatar_url, google_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET name = $3, avatar_url = $4`,
		user.ID, user.Email, user.Name, user.AvatarURL, user.GoogleID)
	return err
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, google_id, created_at, updated_at
		 FROM users WHERE id = $1`, id).Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.GoogleID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, google_id, created_at, updated_at
		 FROM users WHERE email = $1`, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.GoogleID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *userRepo) GetByGoogleID(ctx context.Context, googleID string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, google_id, created_at, updated_at
		 FROM users WHERE google_id = $1`, googleID).Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.GoogleID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
