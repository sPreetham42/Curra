package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type timetableRepo struct {
	pool *pgxpool.Pool
}

func (r *timetableRepo) Create(ctx context.Context, tt domain.Timetable) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO timetables (id, institution_id, name, version)
		 VALUES ($1, $2, $3, $4)`,
		tt.ID, tt.InstitutionID, tt.Name, tt.Version)
	return err
}

func (r *timetableRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error) {
	var tt domain.Timetable
	err := r.pool.QueryRow(ctx,
		`SELECT id, institution_id, name, current_published_version_id, version, created_at, updated_at
		 FROM timetables WHERE id = $1`, id).Scan(
		&tt.ID, &tt.InstitutionID, &tt.Name, &tt.CurrentPublishedVersionID,
		&tt.Version, &tt.CreatedAt, &tt.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return tt, ErrNotFound
		}
		return tt, err
	}
	return tt, nil
}

func (r *timetableRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Timetable, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, institution_id, name, current_published_version_id, version, created_at, updated_at
		 FROM timetables WHERE institution_id = $1 ORDER BY created_at DESC`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Timetable
	for rows.Next() {
		var tt domain.Timetable
		if err := rows.Scan(&tt.ID, &tt.InstitutionID, &tt.Name, &tt.CurrentPublishedVersionID,
			&tt.Version, &tt.CreatedAt, &tt.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, tt)
	}
	return result, nil
}

func (r *timetableRepo) Update(ctx context.Context, tt domain.Timetable) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE timetables SET name = $1, version = version + 1, updated_at = now()
		 WHERE id = $2 AND version = $3`,
		tt.Name, tt.ID, tt.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

func (r *timetableRepo) SetCurrentPublishedVersion(ctx context.Context, timetableID, versionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE timetables SET current_published_version_id = $1, updated_at = now()
		 WHERE id = $2`,
		versionID, timetableID)
	return err
}
