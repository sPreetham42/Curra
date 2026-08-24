# Database Schema

PostgreSQL 16+. All tables use `institution_id` for multi-tenant isolation. UUIDs for primary keys. Optimistic locking via `version` integer column. Timestamps via `created_at` / `updated_at` with timezone.

---

## 1. Core Identity

### institutions

```sql
CREATE TABLE institutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    settings JSONB DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    avatar_url TEXT,
    google_id TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### user_roles

```sql
CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('INSTITUTION_ADMIN', 'SCHEDULER', 'PROFESSOR', 'VIEWER')),
    faculty_id TEXT, -- optional: links professor role to a Faculty entity
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, institution_id)
);
```

---

## 2. Academic Data

All academic data tables have `institution_id UUID NOT NULL REFERENCES institutions(id)` and `version INT NOT NULL DEFAULT 1`.

### departments

```sql
CREATE TABLE departments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_departments_institution ON departments(institution_id);
```

### programs

```sql
CREATE TABLE programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    department_id UUID NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_programs_institution ON programs(institution_id);
CREATE INDEX idx_programs_department ON programs(department_id);
```

### classes

```sql
CREATE TABLE classes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_classes_institution ON classes(institution_id);
CREATE INDEX idx_classes_program ON classes(program_id);
```

### student_groups

```sql
CREATE TABLE student_groups (
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
CREATE INDEX idx_student_groups_institution ON student_groups(institution_id);
CREATE INDEX idx_student_groups_class ON student_groups(class_id);
```

### subjects

```sql
CREATE TABLE subjects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subjects_institution ON subjects(institution_id);
```

### faculty

```sql
CREATE TABLE faculty (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_faculty_institution ON faculty(institution_id);
```

### rooms

```sql
CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    capacity INT NOT NULL CHECK (capacity >= 0),
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rooms_institution ON rooms(institution_id);
```

### room_features

```sql
CREATE TABLE room_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_room_features_institution ON room_features(institution_id);
```

### room_feature_assignments (many-to-many)

```sql
CREATE TABLE room_feature_assignments (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (room_id, room_feature_id)
);
-- Tenant isolation: both FKs reference institution-scoped tables.
-- Application must validate that room_id and room_feature_id belong to the same institution.
```

### time_slots

```sql
CREATE TABLE time_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    day TEXT NOT NULL, -- 'Monday', 'Tuesday', etc.
    period INT NOT NULL CHECK (period > 0),
    label TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (institution_id, day, period)
);
CREATE INDEX idx_time_slots_institution ON time_slots(institution_id);
```

---

## 3. Scheduling Data

### academic_years

```sql
CREATE TABLE academic_years (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### terms

```sql
CREATE TABLE terms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### course_offerings

```sql
CREATE TABLE course_offerings (
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
CREATE INDEX idx_course_offerings_institution ON course_offerings(institution_id);
CREATE INDEX idx_course_offerings_term ON course_offerings(term_id);
```

### course_offering_features (many-to-many)

```sql
CREATE TABLE course_offering_features (
    course_offering_id UUID NOT NULL REFERENCES course_offerings(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (course_offering_id, room_feature_id)
);
-- Tenant isolation: both FKs reference institution-scoped tables.
-- Application must validate that course_offering_id and room_feature_id belong to the same institution.
```

### session_requirements

```sql
CREATE TABLE session_requirements (
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
CREATE INDEX idx_session_requirements_institution ON session_requirements(institution_id);
CREATE INDEX idx_session_requirements_offering ON session_requirements(course_offering_id);
```

### session_requirement_features (many-to-many)

```sql
CREATE TABLE session_requirement_features (
    session_requirement_id UUID NOT NULL REFERENCES session_requirements(id) ON DELETE CASCADE,
    room_feature_id UUID NOT NULL REFERENCES room_features(id) ON DELETE CASCADE,
    PRIMARY KEY (session_requirement_id, room_feature_id)
);
-- Tenant isolation: both FKs reference institution-scoped tables.
-- Application must validate that session_requirement_id and room_feature_id belong to the same institution.
```

### faculty_availability

```sql
CREATE TABLE faculty_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    faculty_id UUID NOT NULL REFERENCES faculty(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (faculty_id, time_slot_id)
);
CREATE INDEX idx_faculty_availability_faculty ON faculty_availability(faculty_id);
```

### faculty_preferences

```sql
CREATE TABLE faculty_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    faculty_id UUID NOT NULL REFERENCES faculty(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    weight INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (faculty_id, time_slot_id)
);
```

### room_availability

```sql
CREATE TABLE room_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    time_slot_id UUID NOT NULL REFERENCES time_slots(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, time_slot_id)
);
CREATE INDEX idx_room_availability_room ON room_availability(room_id);
```

---

## 4. Timetables & Scheduling

### timetables

```sql
CREATE TABLE timetables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    current_published_version_id UUID, -- forward ref, set after CREATE
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_timetables_institution ON timetables(institution_id);
```

### problem_snapshots

```sql
CREATE TABLE problem_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    schema_version INT NOT NULL DEFAULT 1,
    problem JSONB NOT NULL,              -- serialized problem.Problem
    constraint_instances JSONB DEFAULT '[]',
    solver_config JSONB DEFAULT '{}',
    objective_config JSONB DEFAULT '{}',
    input_hash TEXT NOT NULL,            -- SHA-256 for deduplication
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_snapshots_timetable ON problem_snapshots(timetable_id);
CREATE INDEX idx_snapshots_hash ON problem_snapshots(input_hash);
-- Immutability: no UPDATE trigger needed — application layer enforces read-only.
```

### constraint_rulesets

```sql
CREATE TABLE constraint_rulesets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    snapshot_id UUID NOT NULL REFERENCES problem_snapshots(id) ON DELETE CASCADE,
    rule_set_hash TEXT NOT NULL,
    instances JSONB NOT NULL,
    compiled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### schedule_runs

```sql
CREATE TABLE schedule_runs (
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
    result JSONB,                        -- serialized problem.Solution (when SOLVED)
    diagnostics JSONB,                   -- serialized diagnostics.Diagnostics
    score JSONB,                         -- serialized scorer.Score
    violations JSONB,                    -- serialized []diagnostics.Violation
    worker_id TEXT,
    heartbeat_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration_ms BIGINT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INT NOT NULL DEFAULT 1       -- optimistic locking
);
CREATE INDEX idx_runs_timetable ON schedule_runs(timetable_id);
CREATE INDEX idx_runs_status ON schedule_runs(status);
CREATE INDEX idx_runs_worker ON schedule_runs(worker_id) WHERE status = 'RUNNING';
```

### schedule_versions

```sql
CREATE TABLE schedule_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    source_run_id UUID REFERENCES schedule_runs(id),
    snapshot_id UUID NOT NULL REFERENCES problem_snapshots(id),
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT','REVIEW','PUBLISHED','ARCHIVED')),
    name TEXT NOT NULL,
    score JSONB,
    version INT NOT NULL DEFAULT 1,      -- optimistic locking
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_versions_timetable ON schedule_versions(timetable_id);
CREATE INDEX idx_versions_status ON schedule_versions(timetable_id, status);
-- Enforce at most one PUBLISHED version per timetable (state machine invariant)
CREATE UNIQUE INDEX idx_versions_one_published ON schedule_versions(timetable_id) WHERE status = 'PUBLISHED';
```

### schedule_assignments

```sql
CREATE TABLE schedule_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id UUID NOT NULL REFERENCES schedule_versions(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL,          -- CURRA AssignmentID (e.g. "req-001#0")
    course_offering_id TEXT NOT NULL,
    session_requirement_id TEXT NOT NULL,
    student_group_id TEXT NOT NULL,
    faculty_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    time_slot_id TEXT NOT NULL,
    instance INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assignments_version ON schedule_assignments(version_id);
-- Unique per version: no two assignments with same CURRA assignment_id in one version
CREATE UNIQUE INDEX idx_assignments_unique_in_version ON schedule_assignments(version_id, assignment_id);
```

### assignment_pins

```sql
CREATE TABLE assignment_pins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id UUID NOT NULL REFERENCES schedule_versions(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL,
    pinned_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (version_id, assignment_id)
);
```

---

## 5. Import

### import_batches

```sql
CREATE TABLE import_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timetable_id UUID NOT NULL REFERENCES timetables(id) ON DELETE CASCADE,
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,           -- 'CSV', 'EXCEL', etc.
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
);
```

### import_rows

```sql
CREATE TABLE import_rows (
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
CREATE INDEX idx_import_rows_batch ON import_rows(batch_id);
```

---

## 6. Audit

### audit_events

```sql
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,                -- 'CREATE', 'UPDATE', 'DELETE', 'SOLVE', 'PUBLISH', etc.
    resource_type TEXT NOT NULL,         -- 'timetable', 'schedule_run', etc.
    resource_id UUID NOT NULL,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_institution ON audit_events(institution_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_events(resource_type, resource_id);
-- Immutability: no UPDATE/DELETE allowed. Application enforces append-only.
```

---

## 7. Foreign Key Relationships Diagram

```
institutions ──── departments ──── programs ──── classes ──── student_groups
     │                                                │            │
     │                                                │            │
     ├──── faculty ────── faculty_availability        │            │
     │              ────── faculty_preferences        │            │
     │                    │                           │            │
     ├──── rooms ──────── room_availability           │            │
     │       │                                       │            │
     │       └── room_feature_assignments ── room_features         │
     │                                                            │
     ├──── subjects                                                 │
     │                                                             │
     ├──── time_slots                                               │
     │                                                             │
     └──── terms ──── course_offerings ───── session_requirements  │
                         │    │                    │               │
                         │    │                    │               │
                         │    ├── course_offering_features         │
                         │    └── session_requirement_features     │
                         │                                         │
                         └── student_group_id ────────────────────┘
                         └── class_id ────────────────────────────┘
                         └── faculty_id ──────────────────────────┘
                         └── subject_id ──────────────────────────┘

timetables ──── problem_snapshots ──── constraint_rulesets
     │
     ├──── schedule_runs ──── (result JSONB → Solution)
     │
     └──── schedule_versions ──── schedule_assignments
                            └─── assignment_pins
```

---

## 8. Table Classification

| Table | Mutable? | Notes |
|---|---|---|
| institutions | Yes (optimistic lock) | |
| users | Yes (optimistic lock) | |
| user_roles | Yes (admin manage) | |
| departments | Yes (optimistic lock) | |
| programs | Yes (optimistic lock) | |
| classes | Rarely | |
| student_groups | Yes (optimistic lock) | Size changes are safe |
| subjects | Yes (optimistic lock) | |
| faculty | Yes (optimistic lock) | |
| rooms | Yes (optimistic lock) | |
| room_features | Rarely | |
| time_slots | Rarely | |
| terms | Rarely | |
| course_offerings | Yes (optimistic lock) | Faculty reassignment |
| session_requirements | Rarely | |
| faculty_availability | Yes | Allow-list management |
| room_availability | Yes | Allow-list management |
| faculty_preferences | Yes | |
| timetables | Yes (optimistic lock) | Name/settings only |
| problem_snapshots | **No** | Immutable record |
| constraint_rulesets | **No** | Immutable record |
| schedule_runs | Append-only state | Status transitions only |
| schedule_versions | Status transitions | Assignments immutable per version |
| schedule_assignments | **No** | Part of version — edit creates new version |
| assignment_pins | Add/remove | Within a version |
| import_batches | Status transitions | |
| import_rows | Status transitions | |
| audit_events | **No** | Append-only |
