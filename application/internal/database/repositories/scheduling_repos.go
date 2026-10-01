package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type schedulingConfigRepo struct{ pool *pgxpool.Pool }

func (r *schedulingConfigRepo) Create(ctx context.Context, config domain.SchedulingConfig) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO scheduling_configs (id, institution_id, name, config, version) VALUES ($1,$2,$3,$4,$5)`,
		config.ID, config.InstitutionID, config.Name, config.Config, config.Version)
	return err
}

func (r *schedulingConfigRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.SchedulingConfig, error) {
	var config domain.SchedulingConfig
	err := r.pool.QueryRow(ctx, `SELECT id, institution_id, name, config, version, created_at, updated_at FROM scheduling_configs WHERE id=$1`, id).Scan(&config.ID, &config.InstitutionID, &config.Name, &config.Config, &config.Version, &config.CreatedAt, &config.UpdatedAt)
	return config, err
}

func (r *schedulingConfigRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.SchedulingConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, institution_id, name, config, version, created_at, updated_at FROM scheduling_configs WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SchedulingConfig
	for rows.Next() {
		var config domain.SchedulingConfig
		if err := rows.Scan(&config.ID, &config.InstitutionID, &config.Name, &config.Config, &config.Version, &config.CreatedAt, &config.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	return out, nil
}

func (r *schedulingConfigRepo) Update(ctx context.Context, config domain.SchedulingConfig) error {
	tag, err := r.pool.Exec(ctx, `UPDATE scheduling_configs SET name=$1, config=$2, version=version+1, updated_at=now() WHERE id=$3 AND version=$4`, config.Name, config.Config, config.ID, config.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

func (r *schedulingConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM scheduling_configs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}