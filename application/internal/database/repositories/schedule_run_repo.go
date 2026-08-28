package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type scheduleRunRepo struct {
	pool *pgxpool.Pool
}

func (r *scheduleRunRepo) Create(ctx context.Context, run domain.ScheduleRun) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO schedule_runs
		 (id, timetable_id, institution_id, snapshot_id, status, solver_config,
		  objective_config, seed, rule_set_hash, curra_version, curra_commit,
		  created_by, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		run.ID, run.TimetableID, run.InstitutionID, run.SnapshotID, run.Status,
		run.SolverConfig, run.ObjectiveConfig, run.Seed, run.RuleSetHash,
		run.CurrAVersion, run.CurrACommit, run.CreatedBy, run.Version)
	return err
}

func (r *scheduleRunRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	var run domain.ScheduleRun
	err := r.pool.QueryRow(ctx,
		`SELECT id, timetable_id, institution_id, snapshot_id, status, solver_config,
		        objective_config, seed, rule_set_hash, curra_version, curra_commit,
		        result, diagnostics, score, violations, worker_id, lease_expires_at,
		        retry_count, heartbeat_at, started_at, finished_at, duration_ms,
		        created_by, version, created_at, updated_at
		 FROM schedule_runs WHERE id = $1`, id).Scan(
		&run.ID, &run.TimetableID, &run.InstitutionID, &run.SnapshotID, &run.Status,
		&run.SolverConfig, &run.ObjectiveConfig, &run.Seed, &run.RuleSetHash,
		&run.CurrAVersion, &run.CurrACommit, &run.Result, &run.Diagnostics,
		&run.Score, &run.Violations, &run.WorkerID, &run.LeaseExpiresAt,
		&run.RetryCount, &run.HeartbeatAt, &run.StartedAt, &run.FinishedAt,
		&run.DurationMs, &run.CreatedBy, &run.Version, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return run, ErrNotFound
		}
		return run, err
	}
	return run, nil
}

func (r *scheduleRunRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, timetable_id, institution_id, snapshot_id, status, seed, rule_set_hash,
		        curra_version, curra_commit, score, diagnostics, duration_ms, created_by, version, created_at
		 FROM schedule_runs WHERE timetable_id = $1 ORDER BY created_at DESC`, timetableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ScheduleRun
	for rows.Next() {
		var run domain.ScheduleRun
		if err := rows.Scan(&run.ID, &run.TimetableID, &run.InstitutionID,
			&run.SnapshotID, &run.Status, &run.Seed, &run.RuleSetHash,
			&run.CurrAVersion, &run.CurrACommit, &run.Score, &run.Diagnostics,
			&run.DurationMs, &run.CreatedBy, &run.Version, &run.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, nil
}

func (r *scheduleRunRepo) Update(ctx context.Context, run domain.ScheduleRun) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs SET
		 status = $1, solver_config = $2, objective_config = $3, seed = $4,
		 rule_set_hash = $5, curra_version = $6, curra_commit = $7,
		 result = $8, diagnostics = $9, score = $10, violations = $11,
		 worker_id = $12, lease_expires_at = $13, retry_count = $14, heartbeat_at = $15,
		 started_at = $16, finished_at = $17, duration_ms = $18, version = version + 1, updated_at = now()
		 WHERE id = $19 AND version = $20`,
		run.Status, run.SolverConfig, run.ObjectiveConfig, run.Seed,
		run.RuleSetHash, run.CurrAVersion, run.CurrACommit,
		run.Result, run.Diagnostics, run.Score, run.Violations,
		run.WorkerID, run.LeaseExpiresAt, run.RetryCount, run.HeartbeatAt,
		run.StartedAt, run.FinishedAt, run.DurationMs, run.ID, run.Version)
	return err
}

func (r *scheduleRunRepo) ClaimQueued(ctx context.Context, workerID string, leaseDuration time.Duration) (*domain.ScheduleRun, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	var run domain.ScheduleRun
	leaseSeconds := int64(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 300 // default 5 minutes
	}

	err = tx.QueryRow(ctx,
		`UPDATE schedule_runs
		 SET status = 'RUNNING',
		     worker_id = $1,
		     lease_expires_at = now() + (
		       COALESCE(
		         NULLIF((solver_config->>'timeoutSeconds')::int, 0),
		         NULLIF((solver_config->>'maxDurationSeconds')::int, 0),
		         GREATEST((solver_config->>'maxNodes')::int / 1000, 300),
		         $2
		       ) + 120 || ' seconds'
		     )::INTERVAL,
		     started_at = COALESCE(started_at, now()),
		     heartbeat_at = now(),
		     version = version + 1,
		     updated_at = now()
		 WHERE id = (
		   SELECT id
		   FROM schedule_runs
		   WHERE status = 'QUEUED'
		   ORDER BY created_at ASC
		   LIMIT 1
		   FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, timetable_id, institution_id, snapshot_id, status, solver_config,
		           objective_config, seed, rule_set_hash, curra_version, curra_commit,
		           worker_id, lease_expires_at, retry_count, version, created_by`,
		workerID, leaseSeconds).Scan(
		&run.ID, &run.TimetableID, &run.InstitutionID, &run.SnapshotID,
		&run.Status, &run.SolverConfig, &run.ObjectiveConfig, &run.Seed,
		&run.RuleSetHash, &run.CurrAVersion, &run.CurrACommit,
		&run.WorkerID, &run.LeaseExpiresAt, &run.RetryCount, &run.Version, &run.CreatedBy)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil // no available queued runs
		}
		return nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}

	return &run, true, nil
}

func (r *scheduleRunRepo) UpdateTerminalResult(
	ctx context.Context,
	id uuid.UUID,
	workerID string,
	status domain.ScheduleRunStatus,
	result, score, diagnostics, violations json.RawMessage,
	durationMs int64,
	curraVer, curraCommit, ruleSetHash *string,
) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs
		 SET status = $3,
		     result = $4,
		     score = $5,
		     diagnostics = $6,
		     violations = $7,
		     duration_ms = $8,
		     curra_version = $9,
		     curra_commit = $10,
		     rule_set_hash = $11,
		     finished_at = now(),
		     updated_at = now(),
		     version = version + 1
		 WHERE id = $1 AND status = 'RUNNING' AND worker_id = $2`,
		id, workerID, status, result, score, diagnostics, violations,
		durationMs, curraVer, curraCommit, ruleSetHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleWorker
	}
	return nil
}

func (r *scheduleRunRepo) CommitTerminalResultTx(
	ctx context.Context,
	runID uuid.UUID,
	workerID string,
	status domain.ScheduleRunStatus,
	result, score, diagnostics, violations json.RawMessage,
	durationMs int64,
	curraVer, curraCommit, ruleSetHash *string,
	draftVersion *domain.ScheduleVersion,
	assignments []domain.ScheduleAssignment,
	audit domain.AuditEvent,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Update schedule_runs with stale worker check
	tag, err := tx.Exec(ctx,
		`UPDATE schedule_runs
		 SET status = $3,
		     result = $4,
		     score = $5,
		     diagnostics = $6,
		     violations = $7,
		     duration_ms = $8,
		     curra_version = $9,
		     curra_commit = $10,
		     rule_set_hash = $11,
		     finished_at = now(),
		     updated_at = now(),
		     version = version + 1
		 WHERE id = $1 AND status = 'RUNNING' AND worker_id = $2`,
		runID, workerID, status, result, score, diagnostics, violations,
		durationMs, curraVer, curraCommit, ruleSetHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleWorker
	}

	// 2. If solved, create draft version and insert assignments atomically
	if draftVersion != nil {
		_, err = tx.Exec(ctx,
			`INSERT INTO schedule_versions
			 (id, timetable_id, institution_id, source_run_id, snapshot_id, status, name, score, version, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			draftVersion.ID, draftVersion.TimetableID, draftVersion.InstitutionID,
			draftVersion.SourceRunID, draftVersion.SnapshotID, draftVersion.Status,
			draftVersion.Name, draftVersion.Score, draftVersion.Version,
			draftVersion.CreatedBy, draftVersion.CreatedAt, draftVersion.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert draft version in tx: %w", err)
		}

		if len(assignments) > 0 {
			batch := &pgx.Batch{}
			for _, a := range assignments {
				batch.Queue(
					`INSERT INTO schedule_assignments
					 (id, version_id, assignment_id, course_offering_id, session_requirement_id,
					  student_group_id, faculty_id, room_id, time_slot_id, instance)
					 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
					a.ID, a.VersionID, a.AssignmentID, a.CourseOfferingID, a.SessionRequirementID,
					a.StudentGroupID, a.FacultyID, a.RoomID, a.TimeSlotID, a.Instance)
			}
			br := tx.SendBatch(ctx, batch)
			for i := 0; i < len(assignments); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("insert assignment in tx: %w", err)
				}
			}
			br.Close()
		}
	}

	// 3. Insert audit event
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_events
		 (id, institution_id, user_id, action, resource_type, resource_id, details, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		audit.ID, audit.InstitutionID, audit.UserID, audit.Action, audit.ResourceType, audit.ResourceID, audit.Details, audit.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert audit event in tx: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *scheduleRunRepo) UpdateHeartbeat(ctx context.Context, runID uuid.UUID, workerID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs SET heartbeat_at = now()
		 WHERE id = $1 AND worker_id = $2 AND status = 'RUNNING'`,
		runID, workerID)
	return err
}

func (r *scheduleRunRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, updates map[string]any) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs SET status = $1, updated_at = now()
		 WHERE id = $2`,
		status, id)
	return err
}

func (r *scheduleRunRepo) Cancel(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs
		 SET status = 'CANCELLED', finished_at = now(), updated_at = now(), version = version + 1
		 WHERE id = $1 AND status IN ('QUEUED', 'RUNNING')`,
		id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *scheduleRunRepo) RecoverExpired(ctx context.Context, maxRetries int) (int, error) {
	// Re-queue expired runs with retry < maxRetries and increment retry count
	tag1, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs
		 SET status = 'QUEUED', retry_count = retry_count + 1, worker_id = NULL, lease_expires_at = NULL, updated_at = now(), version = version + 1
		 WHERE status = 'RUNNING' AND lease_expires_at < now() AND retry_count < $1`,
		maxRetries)
	if err != nil {
		return 0, err
	}

	// Fail expired runs that exceeded maxRetries
	tag2, err := r.pool.Exec(ctx,
		`UPDATE schedule_runs
		 SET status = 'FAILED', finished_at = now(), updated_at = now(), version = version + 1,
		     diagnostics = '{"message": "job failed: lease expired and exceeded maximum retry limit"}'::JSONB
		 WHERE status = 'RUNNING' AND lease_expires_at < now() AND retry_count >= $1`,
		maxRetries)
	if err != nil {
		return int(tag1.RowsAffected()), err
	}

	return int(tag1.RowsAffected() + tag2.RowsAffected()), nil
}
