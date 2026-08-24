# Application Domain Model

This document is the single source of truth for the application entity taxonomy.

Every entity below is classified as REQUIRED, IMPORTANT, DEFER, or REMOVE with a rationale.

---

## 1. Entity Taxonomy

### REQUIRED — Core entities the system cannot function without

| Entity | Purpose | Maps to CURRA |
|---|---|---|
| **Institution** | Multi-tenant root. Every entity belongs to one institution. | `model.TenantID` |
| **User** | Authenticated person (professor, scheduler, admin). | None — application-only |
| **Role** | User's relationship to an institution. Determines permissions. | None — application-only |
| **AcademicYear** | Logical grouping of terms (e.g., "2026-2027"). | None — application-only |
| **Term** | A scheduling period (e.g., "Fall 2026"). | `model.Term` |
| **Department** | Organizational unit (e.g., "Computer Science"). | `model.Department` |
| **Program** | Degree program within a department (e.g., "B.Tech CSE"). | `model.Program` |
| **Class** | A student cohort within a program (e.g., "CS Year 2 Section A"). | `model.Class` |
| **StudentGroup** | Atomic group for scheduling and conflict detection. | `model.StudentGroup` |
| **Subject** | Catalog entry for a course (e.g., "CS101 Data Structures"). | `model.Subject` |
| **CourseOffering** | Concrete offering of a subject for a term, class, faculty. | `model.CourseOffering` |
| **SessionRequirement** | Delivery format for a course offering (e.g., 2×1hr Theory). | `model.SessionRequirement` |
| **Faculty** | Instructor who teaches course offerings. | `model.Faculty` |
| **Room** | Physical space with capacity and feature tags. | `model.Room` |
| **RoomFeature** | Tag describing room capability (e.g., "Projector", "GPU Lab"). | `model.RoomFeature` |
| **TimeSlot** | Single period in the weekly grid (Day + Period). | `model.TimeSlot` |
| **Timetable** | A workspace container for one scheduling task. Holds the source academic data, snapshots, runs, and versions. | None — application-only |
| **ProblemSnapshot** | Immutable, self-contained capture of all data CURRA needs. | `problem.Problem` (serialized) |
| **ScheduleRun** | A single execution of the solver against a snapshot. | `engine.Request` / `engine.Response` |
| **ScheduleVersion** | A saved, immutable result that a user can review, edit, or publish. | `problem.Solution` (serialized) |
| **ScheduleAssignment** | A single scheduled session in a version or active timetable. | `problem.Assignment` |
| **AuditEvent** | Append-only log of who did what and when. | None — application-only |

### IMPORTANT — Significant but can be phased in after MVP

| Entity | Purpose | Maps to CURRA |
|---|---|---|
| **FacultyAvailability** | Records when a faculty member is available to teach. | `model.FacultyAvailability` |
| **FacultyPreference** | Soft preference for specific slots. | `model.FacultyPreference` |
| **RoomAvailability** | Records when a room is available for scheduling. | `model.RoomAvailability` |
| **AssignmentPin** | Marks a specific assignment as fixed/pinned across solves. | `LockedAssignments` |
| **ImportBatch** | Tracks a data import operation (source, status, row counts). | None — application-only |
| **ImportRow** | Individual row from an import with validation status. | None — application-only |

### DEFER — Real domain concepts that should not be built yet

| Entity | Reason to defer |
|---|---|
| **AcademicCalendar** | Holiday/exam period handling is not yet supported by CURRA. Add when CURRA adds date-range awareness. |
| **TimetablePeriod** | Fine-grained period metadata (breaks, lunch) is not in CURRA. Add when needed. |
| **Cohort** | Redundant with Class + StudentGroup. CURRA already models this hierarchy. Do not add a separate concept. |
| **Section** | Redundant with StudentGroup. A "section" IS a StudentGroup in CURRA's model. |

### REMOVE — Not appropriate for this system

| Entity | Reason to remove |
|---|---|
| **Batch** (as scheduling concept) | Ambiguous. Import batches are handled by ImportBatch. Do not add a separate "batch" concept for student grouping. |
| **CrossEnrolledStudent** | CURRA does not model individual students. Cross-enrollment is modeled through StudentGroup overlaps via Class relationships. |

---

## 2. Entity Relationships

```
Institution
  ├── AcademicYear
  │     └── Term
  ├── Department
  │     └── Program
  │           └── Class
  │                 ├── StudentGroup (WholeGroup)
  │                 └── StudentGroup (subgroup)
  ├── Faculty
  ├── Room
  │     └── RoomFeature (many-to-many via feature IDs)
  ├── Subject
  ├── User
  │     └── Role (per institution)
  └── Timetable
        ├── CourseOffering (per Term)
        │     ├── links to Subject, Faculty, Class, StudentGroup
        │     └── SessionRequirement (one or more)
        ├── ProblemSnapshot
        │     └── canonical CURRA Problem JSON
        ├── ScheduleRun
        │     ├── snapshot_id (FK)
        │     ├── solver_config
        │     └── result (solution + diagnostics)
        └── ScheduleVersion
              ├── source_run_id (FK, nullable)
              ├── ScheduleAssignment[]
              └── AssignmentPin[]
```

---

## 3. Ambiguity Resolution

### Class vs. Section vs. Cohort vs. StudentGroup

**Decision:** CURRA uses a two-level hierarchy: `Class` contains `StudentGroup` records. The application should use these same concepts directly.

- **Class** = "CS Year 2 Section A" — the entire cohort that shares a curriculum.
- **StudentGroup (whole)** = the whole class (40 students who sit together for theory).
- **StudentGroup (subgroup)** = a lab batch or tutorial group (e.g., "CS-A1" = 20 students).

A "Section" in common university parlance IS a Class. A "Cohort" IS a Class. Do not add separate entities.

### Faculty vs. Professor

**Decision:** Use `Faculty` throughout. CURRA uses `Faculty` and changing the name would create confusion between the application and engine layers.

### Room vs. Classroom vs. Laboratory

**Decision:** Use `Room` for everything. Distinguish by `RoomFeature` tags (e.g., "Lab", "Projector"). Do not add separate entity types for different room kinds.

---

## 4. CURRA Mapping Rules

The application maps to CURRA through these rules:

1. **Application IDs are strings.** CURRA model types are string-typed IDs. Use the same IDs.
2. **Application entities map 1:1 to CURRA entities.** No transformation beyond field selection.
3. **Availability is an allow-list.** The application must convert user-entered "unavailable" times into CURRA's allow-list format before creating a snapshot.
4. **TimeSlot is the atomic unit.** CURRA does not understand date ranges, only weekly recurring periods. The application must generate TimeSlot records from its calendar configuration.
5. **ProblemSnapshot is the CURRA Problem.** The snapshot serializes a `problem.Problem` struct. This is the ONLY format CURRA accepts.
6. **Solution is the CURRA Solution.** The schedule version stores a serialized `problem.Solution` (assignments + score).

---

## 5. Immutable vs. Mutable Classification

| Entity | Immutable after creation | Mutable fields | Version strategy |
|---|---|---|---|
| Institution | No | Name, settings | Optimistic lock |
| User | No | Name, email, avatar | Optimistic lock |
| AcademicYear | Partially | Name only after creation | Optimistic lock |
| Term | Partially | Name only after creation | Optimistic lock |
| Department | Partially | Name only | Optimistic lock |
| Program | Partially | Name only | Optimistic lock |
| Class | Yes | — | Create new version |
| StudentGroup | Partially | Size, name | Optimistic lock |
| Subject | Partially | Name, code | Optimistic lock |
| CourseOffering | Partially | FacultyID, features | Optimistic lock |
| SessionRequirement | Yes | — | Create new offering |
| Faculty | Partially | Name | Optimistic lock |
| Room | Partially | Name, capacity, features | Optimistic lock |
| RoomFeature | Yes | — | Create new feature |
| TimeSlot | Yes | — | — |
| Timetable | Partially | Name | Optimistic lock |
| ProblemSnapshot | **Yes** | — | Never — it is an immutable record |
| ScheduleRun | Status changes only | Status, result, diagnostics | Append-only state transitions |
| ScheduleVersion | Status changes only | Status (DRAFT→REVIEW→PUBLISHED→ARCHIVED) | State machine transitions |
| ScheduleAssignment | Yes | — | Part of version; edits create new version |
| AssignmentPin | Yes | — | Add/remove only |
| AuditEvent | **Yes** | — | Append-only |
