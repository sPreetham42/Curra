package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type institutionRepo struct {
	pool *pgxpool.Pool
}

func (r *institutionRepo) Create(ctx context.Context, inst domain.Institution) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO institutions (id, name, slug, settings, version)
		 VALUES ($1, $2, $3, $4, $5)`,
		inst.ID, inst.Name, inst.Slug, inst.Settings, inst.Version)
	return err
}

func (r *institutionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Institution, error) {
	var inst domain.Institution
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, version, created_at, updated_at
		 FROM institutions WHERE id = $1`, id).Scan(
		&inst.ID, &inst.Name, &inst.Slug, &inst.Settings,
		&inst.Version, &inst.CreatedAt, &inst.UpdatedAt)
	return inst, err
}
