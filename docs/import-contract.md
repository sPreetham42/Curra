# Import Contract

## 1. Pipeline Overview

```
Upload → Staging → Parsing → Validation → Preview → Commit
```

Every step is idempotent and reversible until COMMIT.

---

## 2. Pipeline Steps

### Step 1: Upload
- User uploads a file (CSV, Excel) via `POST /timetables/:id/import`.
- File is stored in temporary object storage.
- An `ImportBatch` is created with status `PENDING`.
- Returns `import_batch_id`.

### Step 2: Parsing
- Worker picks up the batch, transitions to `PARSING`.
- File is parsed row-by-row into `ImportRow` records with `raw_data` (original) and `parsed_data` (structured).
- Parsing errors are recorded in the row's `errors` field.
- Batch transitions to `STAGED`.

### Step 3: Validation
- Worker transitions batch to `VALIDATING`.
- Each row is validated against:
  - Referential integrity (do referenced entities exist?)
  - Business rules (e.g., capacity > 0, valid day names)
  - CURRA compatibility (would this data produce a valid CURRA Problem?)
- Rows are marked `VALID` or `ERROR`.
- Batch transitions to `READY` (if any valid rows) or `FAILED` (if all error).

### Step 4: Preview
- User views parsed and validated rows via `GET /import-batches/:id/rows`.
- Shows valid rows, error rows, and error messages.
- User decides to commit or cancel.

### Step 5: Commit
- User confirms via `POST /import-batches/:id/commit`.
- Only VALID rows are merged into the live academic data tables.
- Batch transitions to `COMMITTED`.
- Staging data is cleaned up asynchronously.

---

## 3. Import Types

### Faculty Availability Import
```
columns: faculty_name, day, period
→ Maps to: faculty_availability (after resolving faculty_id and time_slot_id)
```

### Room Availability Import
```
columns: room_name, day, period
→ Maps to: room_availability
```

### Course Offering Import
```
columns: subject_code, subject_name, class_name, faculty_name, student_group_name,
         theory_sessions_per_week, theory_duration,
         lab_sessions_per_week, lab_duration
→ Maps to: subjects, course_offerings, session_requirements
```

### Student Group Import
```
columns: program_name, class_name, group_name, size, is_whole_group
→ Maps to: programs, classes, student_groups
```

---

## 4. Duplicate Handling

| Entity | Duplicate detection | Action |
|---|---|---|
| Subject | Same `code` within institution | Skip (update name if different) |
| Faculty | Same `name` within institution | Skip |
| Room | Same `name` within institution | Skip |
| StudentGroup | Same `name` within class | Skip (update size if different) |
| CourseOffering | Same subject + class + faculty + term | Skip |
| FacultyAvailability | Same faculty + time_slot | Skip |
| RoomAvailability | Same room + time_slot | Skip |

---

## 5. Partial Import

- If some rows are valid and some are error, the batch status is `READY`.
- Only valid rows are committed.
- Error rows remain in the import_rows table for inspection.
- The user can see exactly which rows will be imported before confirming.

---

## 6. Rollback

- **Before COMMIT:** User can cancel the batch. Staging data is cleaned up.
- **After COMMIT:** There is no automatic rollback. The user must manually delete or modify the created entities. This is intentional — commit is a significant action.

---

## 7. Import Templates (Deferred)

The system may support saved import mapping templates (e.g., "column 3 = faculty name"). This is deferred to a later phase. The MVP requires the user to map columns during each import.

---

## 8. Validation Ownership

| Validation layer | Responsibility |
|---|---|
| Import parsing | Format validation (dates, numbers, required columns) |
| Import validation | Referential integrity and business rules |
| CURRA adapter | CURRA-level compatibility (optional pre-check) |
| Schedule Engine | Full CURRA validation at snapshot creation time |

Import validation is application-level. CURRA is NOT invoked during import — it is invoked later when a snapshot is created from the imported data.
