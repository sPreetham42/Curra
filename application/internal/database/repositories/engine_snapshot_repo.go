package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type engineSnapshotRepo struct {
	pool *pgxpool.Pool
}

func (r *engineSnapshotRepo) Create(ctx context.Context, snap domain.EngineSnapshot) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO engine_snapshots
		 (id, schedule_run_id, snapshot_id, institution_id, engine_version,
		  engine_commit, adapter_version, rule_set_hash, input_hash, request,
		  response, diagnostics, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		snap.ID, snap.ScheduleRunID, snap.SnapshotID, snap.InstitutionID,
		snap.EngineVersion, snap.EngineCommit, snap.AdapterVersion,
		snap.RuleSetHash, snap.InputHash, snap.Request, snap.Response,
		snap.Diagnostics, snap.CreatedAt)
	return err
}

func (r *engineSnapshotRepo) GetByRunID(ctx context.Context, runID uuid.UUID) (domain.EngineSnapshot, error) {
	var snap domain.EngineSnapshot
	err := r.pool.QueryRow(ctx,
		`SELECT id, schedule_run_id, snapshot_id, institution_id, engine_version,
		        engine_commit, adapter_version, rule_set_hash, input_hash, request,
		        response, diagnostics, created_at
		 FROM engine_snapshots WHERE schedule_run_id = $1`, runID).Scan(
		&snap.ID, &snap.ScheduleRunID, &snap.SnapshotID, &snap.InstitutionID,
		&snap.EngineVersion, &snap.EngineCommit, &snap.AdapterVersion,
		&snap.RuleSetHash, &snap.InputHash, &snap.Request, &snap.Response,
		&snap.Diagnostics, &snap.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return snap, ErrNotFound
		}
		return snap, err
	}
	return snap, nil
}
