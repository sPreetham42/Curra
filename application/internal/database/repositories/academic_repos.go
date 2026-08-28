package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

// Department Repo
type departmentRepo struct{ pool *pgxpool.Pool }

func (r *departmentRepo) Create(ctx context.Context, d domain.Department) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO departments (id, institution_id, name, version) VALUES ($1,$2,$3,$4)`,
		d.ID, d.InstitutionID, d.Name, d.Version)
	return err
}
func (r *departmentRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Department, error) {
	var d domain.Department
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM departments WHERE id=$1`, id).Scan(&d.ID, &d.InstitutionID, &d.Name, &d.Version, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}
func (r *departmentRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Department, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM departments WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Department
	for rows.Next() {
		var d domain.Department
		if err := rows.Scan(&d.ID, &d.InstitutionID, &d.Name, &d.Version, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
func (r *departmentRepo) Update(ctx context.Context, d domain.Department) error {
	tag, err := r.pool.Exec(ctx, `UPDATE departments SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, d.Name, d.ID, d.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Program Repo
type programRepo struct{ pool *pgxpool.Pool }

func (r *programRepo) Create(ctx context.Context, p domain.Program) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO programs (id,institution_id,department_id,name,version) VALUES ($1,$2,$3,$4,$5)`,
		p.ID, p.InstitutionID, p.DepartmentID, p.Name, p.Version)
	return err
}
func (r *programRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Program, error) {
	var p domain.Program
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,department_id,name,version,created_at,updated_at FROM programs WHERE id=$1`, id).Scan(&p.ID, &p.InstitutionID, &p.DepartmentID, &p.Name, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (r *programRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Program, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,department_id,name,version,created_at,updated_at FROM programs WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Program
	for rows.Next() {
		var p domain.Program
		if err := rows.Scan(&p.ID, &p.InstitutionID, &p.DepartmentID, &p.Name, &p.Version, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
func (r *programRepo) Update(ctx context.Context, p domain.Program) error {
	tag, err := r.pool.Exec(ctx, `UPDATE programs SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, p.Name, p.ID, p.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Class Repo
type classRepo struct{ pool *pgxpool.Pool }

func (r *classRepo) Create(ctx context.Context, c domain.Class) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO classes (id,institution_id,program_id,name,version) VALUES ($1,$2,$3,$4,$5)`,
		c.ID, c.InstitutionID, c.ProgramID, c.Name, c.Version)
	return err
}
func (r *classRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Class, error) {
	var c domain.Class
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,program_id,name,version,created_at,updated_at FROM classes WHERE id=$1`, id).Scan(&c.ID, &c.InstitutionID, &c.ProgramID, &c.Name, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (r *classRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Class, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,program_id,name,version,created_at,updated_at FROM classes WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Class
	for rows.Next() {
		var c domain.Class
		if err := rows.Scan(&c.ID, &c.InstitutionID, &c.ProgramID, &c.Name, &c.Version, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
func (r *classRepo) Update(ctx context.Context, c domain.Class) error {
	tag, err := r.pool.Exec(ctx, `UPDATE classes SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, c.Name, c.ID, c.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// StudentGroup Repo
type studentGroupRepo struct{ pool *pgxpool.Pool }

func (r *studentGroupRepo) Create(ctx context.Context, s domain.StudentGroup) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO student_groups (id,institution_id,class_id,name,size,is_whole_group,version) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.InstitutionID, s.ClassID, s.Name, s.Size, s.IsWholeGroup, s.Version)
	return err
}
func (r *studentGroupRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.StudentGroup, error) {
	var s domain.StudentGroup
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,class_id,name,size,is_whole_group,version,created_at,updated_at FROM student_groups WHERE id=$1`, id).Scan(&s.ID, &s.InstitutionID, &s.ClassID, &s.Name, &s.Size, &s.IsWholeGroup, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}
func (r *studentGroupRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.StudentGroup, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,class_id,name,size,is_whole_group,version,created_at,updated_at FROM student_groups WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StudentGroup
	for rows.Next() {
		var s domain.StudentGroup
		if err := rows.Scan(&s.ID, &s.InstitutionID, &s.ClassID, &s.Name, &s.Size, &s.IsWholeGroup, &s.Version, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
func (r *studentGroupRepo) Update(ctx context.Context, s domain.StudentGroup) error {
	tag, err := r.pool.Exec(ctx, `UPDATE student_groups SET name=$1, size=$2, version=version+1, updated_at=now() WHERE id=$3 AND version=$4`, s.Name, s.Size, s.ID, s.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Subject Repo
type subjectRepo struct{ pool *pgxpool.Pool }

func (r *subjectRepo) Create(ctx context.Context, s domain.Subject) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO subjects (id,institution_id,code,name,version) VALUES ($1,$2,$3,$4,$5)`,
		s.ID, s.InstitutionID, s.Code, s.Name, s.Version)
	return err
}
func (r *subjectRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Subject, error) {
	var s domain.Subject
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,code,name,version,created_at,updated_at FROM subjects WHERE id=$1`, id).Scan(&s.ID, &s.InstitutionID, &s.Code, &s.Name, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}
func (r *subjectRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Subject, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,code,name,version,created_at,updated_at FROM subjects WHERE institution_id=$1 ORDER BY code`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Subject
	for rows.Next() {
		var s domain.Subject
		if err := rows.Scan(&s.ID, &s.InstitutionID, &s.Code, &s.Name, &s.Version, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
func (r *subjectRepo) Update(ctx context.Context, s domain.Subject) error {
	tag, err := r.pool.Exec(ctx, `UPDATE subjects SET code=$1, name=$2, version=version+1, updated_at=now() WHERE id=$3 AND version=$4`, s.Code, s.Name, s.ID, s.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Faculty Repo
type facultyRepo struct{ pool *pgxpool.Pool }

func (r *facultyRepo) Create(ctx context.Context, f domain.Faculty) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO faculty (id,institution_id,name,version) VALUES ($1,$2,$3,$4)`,
		f.ID, f.InstitutionID, f.Name, f.Version)
	return err
}
func (r *facultyRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Faculty, error) {
	var f domain.Faculty
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM faculty WHERE id=$1`, id).Scan(&f.ID, &f.InstitutionID, &f.Name, &f.Version, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}
func (r *facultyRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Faculty, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM faculty WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Faculty
	for rows.Next() {
		var f domain.Faculty
		if err := rows.Scan(&f.ID, &f.InstitutionID, &f.Name, &f.Version, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}
func (r *facultyRepo) Update(ctx context.Context, f domain.Faculty) error {
	tag, err := r.pool.Exec(ctx, `UPDATE faculty SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, f.Name, f.ID, f.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Room Repo
type roomRepo struct{ pool *pgxpool.Pool }

func (r *roomRepo) Create(ctx context.Context, rm domain.Room) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO rooms (id,institution_id,name,capacity,version) VALUES ($1,$2,$3,$4,$5)`,
		rm.ID, rm.InstitutionID, rm.Name, rm.Capacity, rm.Version)
	return err
}
func (r *roomRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	var rm domain.Room
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,name,capacity,version,created_at,updated_at FROM rooms WHERE id=$1`, id).Scan(&rm.ID, &rm.InstitutionID, &rm.Name, &rm.Capacity, &rm.Version, &rm.CreatedAt, &rm.UpdatedAt)
	return rm, err
}
func (r *roomRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Room, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,name,capacity,version,created_at,updated_at FROM rooms WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Room
	for rows.Next() {
		var rm domain.Room
		if err := rows.Scan(&rm.ID, &rm.InstitutionID, &rm.Name, &rm.Capacity, &rm.Version, &rm.CreatedAt, &rm.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rm)
	}
	return out, nil
}
func (r *roomRepo) Update(ctx context.Context, rm domain.Room) error {
	tag, err := r.pool.Exec(ctx, `UPDATE rooms SET name=$1, capacity=$2, version=version+1, updated_at=now() WHERE id=$3 AND version=$4`, rm.Name, rm.Capacity, rm.ID, rm.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// RoomFeature Repo
type roomFeatureRepo struct{ pool *pgxpool.Pool }

func (r *roomFeatureRepo) Create(ctx context.Context, rf domain.RoomFeature) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO room_features (id,institution_id,name,version) VALUES ($1,$2,$3,$4)`,
		rf.ID, rf.InstitutionID, rf.Name, rf.Version)
	return err
}
func (r *roomFeatureRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.RoomFeature, error) {
	var rf domain.RoomFeature
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM room_features WHERE id=$1`, id).Scan(&rf.ID, &rf.InstitutionID, &rf.Name, &rf.Version, &rf.CreatedAt, &rf.UpdatedAt)
	return rf, err
}
func (r *roomFeatureRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.RoomFeature, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM room_features WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RoomFeature
	for rows.Next() {
		var rf domain.RoomFeature
		if err := rows.Scan(&rf.ID, &rf.InstitutionID, &rf.Name, &rf.Version, &rf.CreatedAt, &rf.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
}
func (r *roomFeatureRepo) Update(ctx context.Context, rf domain.RoomFeature) error {
	tag, err := r.pool.Exec(ctx, `UPDATE room_features SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, rf.Name, rf.ID, rf.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// TimeSlot Repo
type timeSlotRepo struct{ pool *pgxpool.Pool }

func (r *timeSlotRepo) Create(ctx context.Context, ts domain.TimeSlot) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO time_slots (id,institution_id,day,period,label,version) VALUES ($1,$2,$3,$4,$5,$6)`,
		ts.ID, ts.InstitutionID, ts.Day, ts.Period, ts.Label, ts.Version)
	return err
}
func (r *timeSlotRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.TimeSlot, error) {
	var ts domain.TimeSlot
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,day,period,label,version,created_at,updated_at FROM time_slots WHERE id=$1`, id).Scan(&ts.ID, &ts.InstitutionID, &ts.Day, &ts.Period, &ts.Label, &ts.Version, &ts.CreatedAt, &ts.UpdatedAt)
	return ts, err
}
func (r *timeSlotRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.TimeSlot, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,day,period,label,version,created_at,updated_at FROM time_slots WHERE institution_id=$1 ORDER BY day, period`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TimeSlot
	for rows.Next() {
		var ts domain.TimeSlot
		if err := rows.Scan(&ts.ID, &ts.InstitutionID, &ts.Day, &ts.Period, &ts.Label, &ts.Version, &ts.CreatedAt, &ts.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ts)
	}
	return out, nil
}
func (r *timeSlotRepo) Update(ctx context.Context, ts domain.TimeSlot) error {
	tag, err := r.pool.Exec(ctx, `UPDATE time_slots SET day=$1, period=$2, label=$3, version=version+1, updated_at=now() WHERE id=$4 AND version=$5`, ts.Day, ts.Period, ts.Label, ts.ID, ts.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// AcademicYear Repo
type academicYearRepo struct{ pool *pgxpool.Pool }

func (r *academicYearRepo) Create(ctx context.Context, ay domain.AcademicYear) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO academic_years (id,institution_id,name,version) VALUES ($1,$2,$3,$4)`,
		ay.ID, ay.InstitutionID, ay.Name, ay.Version)
	return err
}
func (r *academicYearRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.AcademicYear, error) {
	var ay domain.AcademicYear
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM academic_years WHERE id=$1`, id).Scan(&ay.ID, &ay.InstitutionID, &ay.Name, &ay.Version, &ay.CreatedAt, &ay.UpdatedAt)
	return ay, err
}
func (r *academicYearRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.AcademicYear, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,name,version,created_at,updated_at FROM academic_years WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AcademicYear
	for rows.Next() {
		var ay domain.AcademicYear
		if err := rows.Scan(&ay.ID, &ay.InstitutionID, &ay.Name, &ay.Version, &ay.CreatedAt, &ay.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ay)
	}
	return out, nil
}
func (r *academicYearRepo) Update(ctx context.Context, ay domain.AcademicYear) error {
	tag, err := r.pool.Exec(ctx, `UPDATE academic_years SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, ay.Name, ay.ID, ay.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// Term Repo
type termRepo struct{ pool *pgxpool.Pool }

func (r *termRepo) Create(ctx context.Context, t domain.Term) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO terms (id,institution_id,academic_year_id,name,version) VALUES ($1,$2,$3,$4,$5)`,
		t.ID, t.InstitutionID, t.AcademicYearID, t.Name, t.Version)
	return err
}
func (r *termRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Term, error) {
	var t domain.Term
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,academic_year_id,name,version,created_at,updated_at FROM terms WHERE id=$1`, id).Scan(&t.ID, &t.InstitutionID, &t.AcademicYearID, &t.Name, &t.Version, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
func (r *termRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Term, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,academic_year_id,name,version,created_at,updated_at FROM terms WHERE institution_id=$1 ORDER BY name`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Term
	for rows.Next() {
		var t domain.Term
		if err := rows.Scan(&t.ID, &t.InstitutionID, &t.AcademicYearID, &t.Name, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
func (r *termRepo) Update(ctx context.Context, t domain.Term) error {
	tag, err := r.pool.Exec(ctx, `UPDATE terms SET name=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, t.Name, t.ID, t.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// CourseOffering Repo
type courseOfferingRepo struct{ pool *pgxpool.Pool }

func (r *courseOfferingRepo) Create(ctx context.Context, co domain.CourseOffering) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO course_offerings (id,institution_id,term_id,class_id,subject_id,student_group_id,faculty_id,version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		co.ID, co.InstitutionID, co.TermID, co.ClassID, co.SubjectID, co.StudentGroupID, co.FacultyID, co.Version)
	return err
}
func (r *courseOfferingRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.CourseOffering, error) {
	var co domain.CourseOffering
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,term_id,class_id,subject_id,student_group_id,faculty_id,version,created_at,updated_at FROM course_offerings WHERE id=$1`, id).Scan(&co.ID, &co.InstitutionID, &co.TermID, &co.ClassID, &co.SubjectID, &co.StudentGroupID, &co.FacultyID, &co.Version, &co.CreatedAt, &co.UpdatedAt)
	return co, err
}
func (r *courseOfferingRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.CourseOffering, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,term_id,class_id,subject_id,student_group_id,faculty_id,version,created_at,updated_at FROM course_offerings WHERE institution_id=$1 ORDER BY created_at`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CourseOffering
	for rows.Next() {
		var co domain.CourseOffering
		if err := rows.Scan(&co.ID, &co.InstitutionID, &co.TermID, &co.ClassID, &co.SubjectID, &co.StudentGroupID, &co.FacultyID, &co.Version, &co.CreatedAt, &co.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, co)
	}
	return out, nil
}
func (r *courseOfferingRepo) Update(ctx context.Context, co domain.CourseOffering) error {
	tag, err := r.pool.Exec(ctx, `UPDATE course_offerings SET faculty_id=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`, co.FacultyID, co.ID, co.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}
func (r *courseOfferingRepo) SetFeatures(ctx context.Context, offeringID uuid.UUID, featureIDs []uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM course_offering_features WHERE course_offering_id=$1`, offeringID)
	if err != nil {
		return err
	}
	for _, fid := range featureIDs {
		_, err := r.pool.Exec(ctx, `INSERT INTO course_offering_features (course_offering_id, room_feature_id) VALUES ($1,$2)`, offeringID, fid)
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *courseOfferingRepo) GetFeatures(ctx context.Context, offeringID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT room_feature_id FROM course_offering_features WHERE course_offering_id=$1`, offeringID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var fid uuid.UUID
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		out = append(out, fid)
	}
	return out, nil
}

// SessionRequirement Repo
type sessionRequirementRepo struct{ pool *pgxpool.Pool }

func (r *sessionRequirementRepo) Create(ctx context.Context, sr domain.SessionRequirement) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO session_requirements (id,institution_id,course_offering_id,type,sessions_per_week,duration,consecutive,version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sr.ID, sr.InstitutionID, sr.CourseOfferingID, sr.Type, sr.SessionsPerWeek, sr.Duration, sr.Consecutive, sr.Version)
	return err
}
func (r *sessionRequirementRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.SessionRequirement, error) {
	var sr domain.SessionRequirement
	err := r.pool.QueryRow(ctx, `SELECT id,institution_id,course_offering_id,type,sessions_per_week,duration,consecutive,version,created_at,updated_at FROM session_requirements WHERE id=$1`, id).Scan(&sr.ID, &sr.InstitutionID, &sr.CourseOfferingID, &sr.Type, &sr.SessionsPerWeek, &sr.Duration, &sr.Consecutive, &sr.Version, &sr.CreatedAt, &sr.UpdatedAt)
	return sr, err
}
func (r *sessionRequirementRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.SessionRequirement, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,course_offering_id,type,sessions_per_week,duration,consecutive,version,created_at,updated_at FROM session_requirements WHERE institution_id=$1 ORDER BY created_at`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SessionRequirement
	for rows.Next() {
		var sr domain.SessionRequirement
		if err := rows.Scan(&sr.ID, &sr.InstitutionID, &sr.CourseOfferingID, &sr.Type, &sr.SessionsPerWeek, &sr.Duration, &sr.Consecutive, &sr.Version, &sr.CreatedAt, &sr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, nil
}
func (r *sessionRequirementRepo) ListByOffering(ctx context.Context, offeringID uuid.UUID) ([]domain.SessionRequirement, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,course_offering_id,type,sessions_per_week,duration,consecutive,version,created_at,updated_at FROM session_requirements WHERE course_offering_id=$1 ORDER BY id`, offeringID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SessionRequirement
	for rows.Next() {
		var sr domain.SessionRequirement
		if err := rows.Scan(&sr.ID, &sr.InstitutionID, &sr.CourseOfferingID, &sr.Type, &sr.SessionsPerWeek, &sr.Duration, &sr.Consecutive, &sr.Version, &sr.CreatedAt, &sr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, nil
}
func (r *sessionRequirementRepo) Update(ctx context.Context, sr domain.SessionRequirement) error {
	tag, err := r.pool.Exec(ctx, `UPDATE session_requirements SET type=$1, sessions_per_week=$2, duration=$3, consecutive=$4, version=version+1, updated_at=now() WHERE id=$5 AND version=$6`,
		sr.Type, sr.SessionsPerWeek, sr.Duration, sr.Consecutive, sr.ID, sr.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOptimisticLock
	}
	return nil
}
func (r *sessionRequirementRepo) SetFeatures(ctx context.Context, reqID uuid.UUID, featureIDs []uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM session_requirement_features WHERE session_requirement_id=$1`, reqID)
	if err != nil {
		return err
	}
	for _, fid := range featureIDs {
		_, err := r.pool.Exec(ctx, `INSERT INTO session_requirement_features (session_requirement_id, room_feature_id) VALUES ($1,$2)`, reqID, fid)
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *sessionRequirementRepo) GetFeatures(ctx context.Context, reqID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT room_feature_id FROM session_requirement_features WHERE session_requirement_id=$1`, reqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var fid uuid.UUID
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		out = append(out, fid)
	}
	return out, nil
}

// FacultyAvailability Repo
type facultyAvailabilityRepo struct{ pool *pgxpool.Pool }

func (r *facultyAvailabilityRepo) Create(ctx context.Context, fa domain.FacultyAvailability) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO faculty_availability (id,institution_id,faculty_id,time_slot_id) VALUES ($1,$2,$3,$4)`,
		fa.ID, fa.InstitutionID, fa.FacultyID, fa.TimeSlotID)
	return err
}
func (r *facultyAvailabilityRepo) ListByFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.FacultyAvailability, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,faculty_id,time_slot_id,created_at FROM faculty_availability WHERE faculty_id=$1`, facultyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FacultyAvailability
	for rows.Next() {
		var fa domain.FacultyAvailability
		if err := rows.Scan(&fa.ID, &fa.InstitutionID, &fa.FacultyID, &fa.TimeSlotID, &fa.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fa)
	}
	return out, nil
}
func (r *facultyAvailabilityRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.FacultyAvailability, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,faculty_id,time_slot_id,created_at FROM faculty_availability WHERE institution_id=$1`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FacultyAvailability
	for rows.Next() {
		var fa domain.FacultyAvailability
		if err := rows.Scan(&fa.ID, &fa.InstitutionID, &fa.FacultyID, &fa.TimeSlotID, &fa.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fa)
	}
	return out, nil
}
func (r *facultyAvailabilityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM faculty_availability WHERE id=$1`, id)
	return err
}

// RoomAvailability Repo
type roomAvailabilityRepo struct{ pool *pgxpool.Pool }

func (r *roomAvailabilityRepo) Create(ctx context.Context, ra domain.RoomAvailability) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO room_availability (id,institution_id,room_id,time_slot_id) VALUES ($1,$2,$3,$4)`,
		ra.ID, ra.InstitutionID, ra.RoomID, ra.TimeSlotID)
	return err
}
func (r *roomAvailabilityRepo) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]domain.RoomAvailability, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,room_id,time_slot_id,created_at FROM room_availability WHERE room_id=$1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RoomAvailability
	for rows.Next() {
		var ra domain.RoomAvailability
		if err := rows.Scan(&ra.ID, &ra.InstitutionID, &ra.RoomID, &ra.TimeSlotID, &ra.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ra)
	}
	return out, nil
}
func (r *roomAvailabilityRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.RoomAvailability, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,room_id,time_slot_id,created_at FROM room_availability WHERE institution_id=$1`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RoomAvailability
	for rows.Next() {
		var ra domain.RoomAvailability
		if err := rows.Scan(&ra.ID, &ra.InstitutionID, &ra.RoomID, &ra.TimeSlotID, &ra.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ra)
	}
	return out, nil
}
func (r *roomAvailabilityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM room_availability WHERE id=$1`, id)
	return err
}

// FacultyPreference Repo
type facultyPreferenceRepo struct{ pool *pgxpool.Pool }

func (r *facultyPreferenceRepo) Create(ctx context.Context, fp domain.FacultyPreference) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO faculty_preferences (id,institution_id,faculty_id,time_slot_id,weight) VALUES ($1,$2,$3,$4,$5)`,
		fp.ID, fp.InstitutionID, fp.FacultyID, fp.TimeSlotID, fp.Weight)
	return err
}
func (r *facultyPreferenceRepo) ListByFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.FacultyPreference, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,faculty_id,time_slot_id,weight,created_at FROM faculty_preferences WHERE faculty_id=$1`, facultyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FacultyPreference
	for rows.Next() {
		var fp domain.FacultyPreference
		if err := rows.Scan(&fp.ID, &fp.InstitutionID, &fp.FacultyID, &fp.TimeSlotID, &fp.Weight, &fp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fp)
	}
	return out, nil
}
func (r *facultyPreferenceRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.FacultyPreference, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,faculty_id,time_slot_id,weight,created_at FROM faculty_preferences WHERE institution_id=$1`, instID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FacultyPreference
	for rows.Next() {
		var fp domain.FacultyPreference
		if err := rows.Scan(&fp.ID, &fp.InstitutionID, &fp.FacultyID, &fp.TimeSlotID, &fp.Weight, &fp.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fp)
	}
	return out, nil
}
func (r *facultyPreferenceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM faculty_preferences WHERE id=$1`, id)
	return err
}

// AuditEvent Repo
type auditEventRepo struct{ pool *pgxpool.Pool }

func (r *auditEventRepo) Create(ctx context.Context, event domain.AuditEvent) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO audit_events (id,institution_id,user_id,action,resource_type,resource_id,details) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		event.ID, event.InstitutionID, event.UserID, event.Action, event.ResourceType, event.ResourceID, event.Details)
	return err
}
func (r *auditEventRepo) ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,institution_id,user_id,action,resource_type,resource_id,details,created_at FROM audit_events WHERE institution_id=$1 ORDER BY created_at DESC LIMIT $2`, instID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.ID, &e.InstitutionID, &e.UserID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
