package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type snapshotRepo struct {
	pool *pgxpool.Pool
}

func (r *snapshotRepo) Create(ctx context.Context, snap domain.ProblemSnapshot) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO problem_snapshots
		 (id, timetable_id, institution_id, schema_version, problem, constraint_instances,
		  solver_config, objective_config, input_hash, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		snap.ID, snap.TimetableID, snap.InstitutionID, snap.SchemaVersion,
		snap.ProblemJSON, snap.ConstraintInstances, snap.SolverConfig,
		snap.ObjectiveConfig, snap.InputHash, snap.CreatedBy)
	return err
}

func (r *snapshotRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	var snap domain.ProblemSnapshot
	err := r.pool.QueryRow(ctx,
		`SELECT id, timetable_id, institution_id, schema_version, problem,
		        constraint_instances, solver_config, objective_config,
		        input_hash, created_by, created_at
		 FROM problem_snapshots WHERE id = $1`, id).Scan(
		&snap.ID, &snap.TimetableID, &snap.InstitutionID, &snap.SchemaVersion,
		&snap.ProblemJSON, &snap.ConstraintInstances, &snap.SolverConfig,
		&snap.ObjectiveConfig, &snap.InputHash, &snap.CreatedBy, &snap.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return snap, ErrNotFound
		}
		return snap, err
	}
	return snap, nil
}

func (r *snapshotRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, timetable_id, institution_id, schema_version, problem,
		        constraint_instances, solver_config, objective_config,
		        input_hash, created_by, created_at
		 FROM problem_snapshots WHERE timetable_id = $1 ORDER BY created_at DESC`, timetableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ProblemSnapshot
	for rows.Next() {
		var snap domain.ProblemSnapshot
		if err := rows.Scan(&snap.ID, &snap.TimetableID, &snap.InstitutionID, &snap.SchemaVersion,
			&snap.ProblemJSON, &snap.ConstraintInstances, &snap.SolverConfig,
			&snap.ObjectiveConfig, &snap.InputHash, &snap.CreatedBy, &snap.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, snap)
	}
	return result, nil
}
