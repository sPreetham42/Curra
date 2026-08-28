package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type scheduleAssignmentRepo struct {
	pool *pgxpool.Pool
}

func (r *scheduleAssignmentRepo) Create(ctx context.Context, a domain.ScheduleAssignment) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO schedule_assignments
		 (id, version_id, assignment_id, course_offering_id, session_requirement_id,
		  student_group_id, faculty_id, room_id, time_slot_id, instance)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.VersionID, a.AssignmentID, a.CourseOfferingID, a.SessionRequirementID,
		a.StudentGroupID, a.FacultyID, a.RoomID, a.TimeSlotID, a.Instance)
	return err
}

func (r *scheduleAssignmentRepo) CreateBatch(ctx context.Context, assignments []domain.ScheduleAssignment) error {
	if len(assignments) == 0 {
		return nil
	}
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
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < len(assignments); i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *scheduleAssignmentRepo) ReplaceAllForVersion(ctx context.Context, versionID uuid.UUID, assignments []domain.ScheduleAssignment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM schedule_assignments WHERE version_id = $1`, versionID); err != nil {
		return err
	}

	for _, a := range assignments {
		_, err := tx.Exec(ctx,
			`INSERT INTO schedule_assignments
			 (id, version_id, assignment_id, course_offering_id, session_requirement_id,
			  student_group_id, faculty_id, room_id, time_slot_id, instance)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			a.ID, a.VersionID, a.AssignmentID, a.CourseOfferingID, a.SessionRequirementID,
			a.StudentGroupID, a.FacultyID, a.RoomID, a.TimeSlotID, a.Instance)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *scheduleAssignmentRepo) ListByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.ScheduleAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, version_id, assignment_id, course_offering_id, session_requirement_id,
		        student_group_id, faculty_id, room_id, time_slot_id, instance, created_at
		 FROM schedule_assignments WHERE version_id = $1 ORDER BY assignment_id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ScheduleAssignment
	for rows.Next() {
		var a domain.ScheduleAssignment
		if err := rows.Scan(&a.ID, &a.VersionID, &a.AssignmentID, &a.CourseOfferingID,
			&a.SessionRequirementID, &a.StudentGroupID, &a.FacultyID,
			&a.RoomID, &a.TimeSlotID, &a.Instance, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

func (r *scheduleAssignmentRepo) DeleteByVersion(ctx context.Context, versionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM schedule_assignments WHERE version_id = $1`, versionID)
	return err
}
