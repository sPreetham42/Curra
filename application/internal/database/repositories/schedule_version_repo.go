package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type scheduleVersionRepo struct {
	pool *pgxpool.Pool
}

func (r *scheduleVersionRepo) Create(ctx context.Context, ver domain.ScheduleVersion) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO schedule_versions
		 (id, timetable_id, institution_id, source_run_id, snapshot_id, status, name, score, version, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		ver.ID, ver.TimetableID, ver.InstitutionID, ver.SourceRunID, ver.SnapshotID,
		ver.Status, ver.Name, ver.Score, ver.Version, ver.CreatedBy)
	return err
}

func (r *scheduleVersionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, error) {
	var ver domain.ScheduleVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, timetable_id, institution_id, source_run_id, snapshot_id,
		        status, name, score, version, created_by, created_at, updated_at
		 FROM schedule_versions WHERE id = $1`, id).Scan(
		&ver.ID, &ver.TimetableID, &ver.InstitutionID, &ver.SourceRunID, &ver.SnapshotID,
		&ver.Status, &ver.Name, &ver.Score, &ver.Version, &ver.CreatedBy,
		&ver.CreatedAt, &ver.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ver, ErrNotFound
		}
		return ver, err
	}
	return ver, nil
}

func (r *scheduleVersionRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, timetable_id, institution_id, source_run_id, snapshot_id,
		        status, name, score, version, created_by, created_at, updated_at
		 FROM schedule_versions WHERE timetable_id = $1 ORDER BY created_at DESC`, timetableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ScheduleVersion
	for rows.Next() {
		var ver domain.ScheduleVersion
		if err := rows.Scan(&ver.ID, &ver.TimetableID, &ver.InstitutionID, &ver.SourceRunID, &ver.SnapshotID,
			&ver.Status, &ver.Name, &ver.Score, &ver.Version, &ver.CreatedBy,
			&ver.CreatedAt, &ver.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, ver)
	}
	return result, nil
}

func (r *scheduleVersionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleVersionStatus, expectedVersion int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE schedule_versions SET status = $1, version = version + 1, updated_at = now()
		 WHERE id = $2 AND version = $3`,
		status, id, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

func (r *scheduleVersionRepo) Update(ctx context.Context, ver domain.ScheduleVersion) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE schedule_versions SET name = $1, score = $2, version = version + 1, updated_at = now()
		 WHERE id = $3 AND version = $4`,
		ver.Name, ver.Score, ver.ID, ver.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

func (r *scheduleVersionRepo) ApplyAssignmentUpdateTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	scoreJSON json.RawMessage,
	assignments []domain.ScheduleAssignment,
	audit domain.AuditEvent,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Delete old assignments
	if _, err := tx.Exec(ctx, `DELETE FROM schedule_assignments WHERE version_id = $1`, versionID); err != nil {
		return fmt.Errorf("delete assignments in tx: %w", err)
	}

	// 2. Insert new assignments in batch
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
				return fmt.Errorf("batch insert assignments in tx: %w", err)
			}
		}
		br.Close()
	}

	// 3. CAS update version and score
	tag, err := tx.Exec(ctx,
		`UPDATE schedule_versions SET score = $1, version = version + 1, updated_at = now()
		 WHERE id = $2 AND version = $3`,
		scoreJSON, versionID, expectedVersion)
	if err != nil {
		return fmt.Errorf("cas update version in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}

	// 4. Insert audit event
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

func (r *scheduleVersionRepo) PublishTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	timetableID uuid.UUID,
	audit domain.AuditEvent,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Transition any currently PUBLISHED version for this timetable to ARCHIVED
	// This satisfies the idx_versions_one_published partial unique index
	_, err = tx.Exec(ctx,
		`UPDATE schedule_versions
		 SET status = 'ARCHIVED', updated_at = now()
		 WHERE timetable_id = $1 AND status = 'PUBLISHED' AND id != $2`,
		timetableID, versionID)
	if err != nil {
		return fmt.Errorf("archive previous published version in tx: %w", err)
	}

	// 2. CAS transition target version to PUBLISHED
	tag, err := tx.Exec(ctx,
		`UPDATE schedule_versions SET status = 'PUBLISHED', version = version + 1, updated_at = now()
		 WHERE id = $1 AND version = $2`,
		versionID, expectedVersion)
	if err != nil {
		return fmt.Errorf("cas publish version in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}

	// 3. Set timetable current published version pointer
	_, err = tx.Exec(ctx,
		`UPDATE timetables SET current_published_version_id = $1, updated_at = now()
		 WHERE id = $2`,
		versionID, timetableID)
	if err != nil {
		return fmt.Errorf("update timetable published pointer in tx: %w", err)
	}

	// 4. Insert audit event
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_events
		 (id, institution_id, user_id, action, resource_type, resource_id, details, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		audit.ID, audit.InstitutionID, audit.UserID, audit.Action, audit.ResourceType, audit.ResourceID, audit.Details, audit.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert publish audit event in tx: %w", err)
	}

	return tx.Commit(ctx)
}
