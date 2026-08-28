package database

const createInstitutions = `
CREATE TABLE IF NOT EXISTS institutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    settings JSONB DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const createUsers = `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    avatar_url TEXT,
    google_id TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const createUserRoles = `
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('INSTITUTION_ADMIN', 'SCHEDULER', 'PROFESSOR', 'VIEWER')),
    faculty_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, institution_id)
);`

const createDepartments = `
CREATE TABLE IF NOT EXISTS departments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_departments_institution ON departments(institution_id);`

const createPrograms = `
CREATE TABLE IF NOT EXISTS programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_programs_institution ON programs(institution_id);
CREATE INDEX IF NOT EXISTS idx_programs_department ON programs(department_id);`

const createClasses = `
CREATE TABLE IF NOT EXISTS classes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_classes_institution ON classes(institution_id);
CREATE INDEX IF NOT EXISTS idx_classes_program ON classes(program_id);`

const createStudentGroups = `
CREATE TABLE IF NOT EXISTS student_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    size INT NOT NULL CHECK (size >= 0),
    is_whole_group BOOLEAN NOT NULL DEFAULT false,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_student_groups_institution ON student_groups(institution_id);
CREATE INDEX IF NOT EXISTS idx_student_groups_class ON student_groups(class_id);`

const createSubjects = `
CREATE TABLE IF NOT EXISTS subjects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subjects_institution ON subjects(institution_id);`

const createFaculty = `
CREATE TABLE IF NOT EXISTS faculty (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_faculty_institution ON faculty(institution_id);`

const createRooms = `
CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    capacity INT NOT NULL CHECK (capacity >= 0),
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rooms_institution ON rooms(institution_id);`

const createRoomFeatures = `
CREATE TABLE IF NOT EXISTS room_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_room_features_institution ON room_features(institution_id);`

const createRoomFeatureAssignments = `
CREATE TABLE IF NOT EXISTS room_feature_assignments (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (room_id, room_feature_id)
);`

const createTimeSlots = `
CREATE TABLE IF NOT EXISTS time_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    day TEXT NOT NULL,
    period INT NOT NULL CHECK (period > 0),
    label TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (institution_id, day, period)
);
CREATE INDEX IF NOT EXISTS idx_time_slots_institution ON time_slots(institution_id);`

const createAcademicYears = `
CREATE TABLE IF NOT EXISTS academic_years (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const createTerms = `
CREATE TABLE IF NOT EXISTS terms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const createCourseOfferings = `
CREATE TABLE IF NOT EXISTS course_offerings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    term_id UUID NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    student_group_id UUID NOT NULL REFERENCES student_groups(id) ON DELETE CASCADE,
    faculty_id UUID NOT NULL REFERENCES faculty(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_course_offerings_institution ON course_offerings(institution_id);
CREATE INDEX IF NOT EXISTS idx_course_offerings_term ON course_offerings(term_id);`

const createCourseOfferingFeatures = `
CREATE TABLE IF NOT EXISTS course_offering_features (
    course_offering_id UUID NOT NULL REFERENCES course_offerings(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (course_offering_id, room_feature_id)
);`

const createSessionRequirements = `
CREATE TABLE IF NOT EXISTS session_requirements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    course_offering_id UUID NOT NULL REFERENCES course_offerings(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('THEORY', 'LAB')),
    sessions_per_week INT NOT NULL CHECK (sessions_per_week > 0),
    duration INT NOT NULL CHECK (duration > 0),
    consecutive BOOLEAN NOT NULL DEFAULT false,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_session_requirements_institution ON session_requirements(institution_id);
CREATE INDEX IF NOT EXISTS idx_session_requirements_offering ON session_requirements(course_offering_id);`

const createSessionRequirementFeatures = `
CREATE TABLE IF NOT EXISTS session_requirement_features (
    session_requirement_id UUID NOT NULL REFERENCES session_requirements(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (session_requirement_id, room_feature_id)
);`

const createFacultyAvailability = `
CREATE TABLE IF NOT EXISTS faculty_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    faculty_id UUID NOT NULL REFERENCES faculty(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (faculty_id, time_slot_id)
);
CREATE INDEX IF NOT EXISTS idx_faculty_availability_faculty ON faculty_availability(faculty_id);`

const createFacultyPreferences = `
CREATE TABLE IF NOT EXISTS faculty_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    faculty_id UUID NOT NULL REFERENCES faculty(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    weight INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (faculty_id, time_slot_id)
);`

const createRoomAvailability = `
CREATE TABLE IF NOT EXISTS room_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, time_slot_id)
);
CREATE INDEX IF NOT EXISTS idx_room_availability_room ON room_availability(room_id);`

const createTimetables = `
CREATE TABLE IF NOT EXISTS timetables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    current_published_version_id UUID,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_timetables_institution ON timetables(institution_id);`

const createProblemSnapshots = `
CREATE TABLE IF NOT EXISTS problem_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    schema_version INT NOT NULL DEFAULT 1,
    problem JSONB NOT NULL,
    constraint_instances JSONB DEFAULT '[]',
    solver_config JSONB DEFAULT '{}',
    objective_config JSONB DEFAULT '{}',
    input_hash TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_snapshots_timetable ON problem_snapshots(timetable_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_hash ON problem_snapshots(input_hash);`

const createConstraintRulesets = `
CREATE TABLE IF NOT EXISTS constraint_rulesets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES problem_snapshots(id) ON DELETE CASCADE,
    rule_set_hash TEXT NOT NULL,
    instances JSONB NOT NULL,
    compiled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const createScheduleRuns = `
CREATE TABLE IF NOT EXISTS schedule_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES problem_snapshots(id),
    status TEXT NOT NULL DEFAULT 'QUEUED'
        CHECK (status IN ('QUEUED','RUNNING','SOLVED','INFEASIBLE','INVALID_PROBLEM',
                          'INVALID_RESULT','CANCELLED','DEADLINE_EXCEEDED','NODE_LIMIT','FAILED')),
    solver_config JSONB DEFAULT '{}',
    objective_config JSONB DEFAULT '{}',
    seed BIGINT,
    rule_set_hash TEXT,
    curra_version TEXT,
    curra_commit TEXT,
    result JSONB,
    diagnostics JSONB,
    score JSONB,
    violations JSONB,
    worker_id TEXT,
    lease_expires_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    heartbeat_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration_ms BIGINT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_runs_timetable ON schedule_runs(timetable_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON schedule_runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_worker ON schedule_runs(worker_id) WHERE status = 'RUNNING';
CREATE INDEX IF NOT EXISTS idx_runs_claim ON schedule_runs(status, created_at) WHERE status = 'QUEUED' OR status = 'RUNNING';`

const createSnapshotImmutabilityTrigger = `
CREATE OR REPLACE FUNCTION prevent_snapshot_problem_update()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.problem IS DISTINCT FROM OLD.problem THEN
        RAISE EXCEPTION 'problem_snapshots.problem is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_snapshot_problem_update ON problem_snapshots;
CREATE TRIGGER trg_prevent_snapshot_problem_update
BEFORE UPDATE ON problem_snapshots
FOR EACH ROW
EXECUTE FUNCTION prevent_snapshot_problem_update();`

const createIdempotencyKeys = `
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS'
        CHECK (status IN ('IN_PROGRESS', 'COMPLETED', 'FAILED')),
    resource_type TEXT NOT NULL,
    resource_id UUID,
    response_code INT,
    response_body JSONB,
    lock_token UUID,
    locked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (institution_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_idempotency_lookup ON idempotency_keys(institution_id, idempotency_key);`

const createScheduleVersions = `
CREATE TABLE IF NOT EXISTS schedule_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    source_run_id UUID REFERENCES schedule_runs(id),
    snapshot_id UUID NOT NULL REFERENCES problem_snapshots(id),
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT','REVIEW','PUBLISHED','ARCHIVED')),
    name TEXT NOT NULL,
    score JSONB,
    version INT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_versions_timetable ON schedule_versions(timetable_id);
CREATE INDEX IF NOT EXISTS idx_versions_status ON schedule_versions(timetable_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_one_published ON schedule_versions(timetable_id) WHERE status = 'PUBLISHED';`

const createScheduleAssignments = `
CREATE TABLE IF NOT EXISTS schedule_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id UUID NOT NULL REFERENCES schedule_versions(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL,
    course_offering_id TEXT NOT NULL,
    session_requirement_id TEXT NOT NULL,
    student_group_id TEXT NOT NULL,
    faculty_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    time_slot_id TEXT NOT NULL,
    instance INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_assignments_version ON schedule_assignments(version_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_assignments_unique_in_version ON schedule_assignments(version_id, assignment_id);`

const createAssignmentPins = `
CREATE TABLE IF NOT EXISTS assignment_pins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id UUID NOT NULL REFERENCES schedule_versions(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL,
    pinned_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (version_id, assignment_id)
);`

const createImportBatches = `
CREATE TABLE IF NOT EXISTS import_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_filename TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING','PARSING','STAGED','VALIDATING','READY','COMMITTED','FAILED','CANCELLED')),
    total_rows INT DEFAULT 0,
    valid_rows INT DEFAULT 0,
    error_rows INT DEFAULT 0,
    error_summary JSONB,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INT NOT NULL DEFAULT 1
);`

const createImportRows = `
CREATE TABLE IF NOT EXISTS import_rows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    row_number INT NOT NULL,
    raw_data JSONB NOT NULL,
    parsed_data JSONB,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING','VALID','ERROR')),
    errors JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_import_rows_batch ON import_rows(batch_id);`

const createAuditEvents = `
CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID NOT NULL,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_institution ON audit_events(institution_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_events(resource_type, resource_id);`
