# CURRA Backend Implementation State & Gap Analysis

**Date:** 2026-08-28  
**Scope:** `application/` codebase audit against frozen contracts (`contracts/api-contract.md`, `contracts/curra-adapter.md`, `api/openapi.yaml`, `database/schema.md`).

---

## 1. Executive Summary

This document captures the baseline state of the backend application in `application/`, identifies implemented vs incomplete components, catalogs discrepancies with frozen contracts, and defines the roadmap for backend implementation across Phases 1 through 7.

---

## 2. Component-by-Component Audit

### 2.1 Database & Migrations (`application/internal/database/`)
- **What Already Exists:**
  - `migrations.go` defines DDL for 30 tables: `institutions`, `users`, `user_roles`, `departments`, `programs`, `classes`, `student_groups`, `subjects`, `faculty`, `rooms`, `room_features`, `room_feature_assignments`, `time_slots`, `academic_years`, `terms`, `course_offerings`, `course_offering_features`, `session_requirements`, `session_requirement_features`, `faculty_availability`, `faculty_preferences`, `room_availability`, `timetables`, `problem_snapshots`, `constraint_rulesets`, `schedule_runs`, `schedule_versions`, `schedule_assignments`, `assignment_pins`, `import_batches`, `import_rows`, `audit_events`.
  - Multi-tenant column `institution_id` and optimistic lock column `version INT NOT NULL DEFAULT 1` are present on all mutable entities.
  - Connection pooling with `pgxpool` (`db.go`) and config loader (`config.go`).
- **What Is Incomplete / Missing:**
  - **Snapshot Immutability:** `problem_snapshots.problem` does not have a database-level `BEFORE UPDATE` trigger preventing modification after insertion.
  - **Idempotency Persistence:** No table or index for `(institution_id, idempotency_key)` to enforce race-safe idempotency on snapshot/run/version creation.
  - **Worker Leasing Schema:** `schedule_runs` lacks `lease_expires_at` and `retry_count` columns to support robust worker leasing, retry ceiling, and recovery scans.
  - **Database Integration Tests:** Zero database tests exist.

### 2.2 Domain Models (`application/internal/domain/`)
- **What Already Exists:**
  - `academic.go`, `identity.go`, `scheduling.go` define structs for all core domain entities.
  - `ScheduleRunStatus` enums (`QUEUED`, `RUNNING`, `SOLVED`, `INFEASIBLE`, `INVALID_PROBLEM`, `INVALID_RESULT`, `CANCELLED`, `DEADLINE_EXCEEDED`, `NODE_LIMIT`, `FAILED`).
  - `ScheduleVersionStatus` enums (`DRAFT`, `REVIEW`, `PUBLISHED`, `ARCHIVED`).
- **What Is Incomplete / Missing:**
  - Idempotency key tracking types.
  - Move/Swap DTOs for assignment mutations.

### 2.3 Repositories (`application/internal/database/repositories/`)
- **What Already Exists:**
  - Interfaces and pgx implementations for `InstitutionRepo`, `UserRepo`, `UserRoleRepo`, `DepartmentRepo`, `ProgramRepo`, `ClassRepo`, `StudentGroupRepo`, `SubjectRepo`, `FacultyRepo`, `RoomRepo`, `RoomFeatureRepo`, `TimeSlotRepo`, `AcademicYearRepo`, `TermRepo`, `CourseOfferingRepo`, `SessionRequirementRepo`, `FacultyAvailabilityRepo`, `RoomAvailabilityRepo`, `FacultyPreferenceRepo`, `TimetableRepo`, `ProblemSnapshotRepo`, `ScheduleRunRepo`, `ScheduleVersionRepo`, `ScheduleAssignmentRepo`, `AuditEventRepo`.
- **What Is Incomplete / Missing:**
  - **Transactional CAS on `ScheduleVersions`:** Needs `WHERE id = $1 AND version = $2` conditional update returning affected rows for optimistic locking.
  - **Worker Claim Query:** Needs `FOR UPDATE SKIP LOCKED` query selecting `QUEUED` or expired `RUNNING` jobs with lease assignment.
  - **Terminal State Protection:** Needs `WHERE id = $1 AND status = 'RUNNING' AND worker_id = $self` check when saving run terminal status.
  - **Idempotency Repository:** Needs repository for creating/finding idempotency records transactionally.
  - **Transactional Audit Logging:** Needs mechanism to execute audit event creation within the same transaction as state mutations.

### 2.4 CurraAdapter (`application/internal/curra/`)
- **What Already Exists:**
  - `Adapter` struct implementing `Solve` and `Verify`.
  - `mapper.go` converting between solver diagnostics/scores and application DTOs.
  - `adapter_test.go` verifying solve and verify on a simple problem instance.
  - Strict import boundary maintained (only `application/internal/curra` imports `internal/scheduler/...`).
- **What Is Incomplete / Missing:**
  - `CurraAdapter` interface definition missing in Go code (exists only in `contracts/curra-adapter.md`).
  - `ValidateMove` signature in `adapter.go` uses `problem.Move` directly instead of `ValidateMoveRequest` / `ValidateMoveResponse` DTOs.
  - `CompileConstraints` method is not implemented on `Adapter`.

### 2.5 Services (`application/internal/services/`)
- **What Already Exists:**
  - `TimetableService`, `ScheduleRunService`, `context.go` (context user/tenant helpers).
- **What Is Incomplete / Missing:**
  - Service layer is monolithic and lacks clear workflow decomposition.
  - Missing distinct services: `CatalogService`, `SnapshotService`, `RunService`, `VersionService`, `MoveSwapService`, `PublishingService`, `VerificationService`.
  - `requireTenantMatch` helper returning `404 Not Found` (to avoid leaking resource existence across institutions) is not consistently implemented.
  - `If-Match` header validation and CAS error handling (returning `409 Conflict` with `currentVersion`) missing in service layer.

### 2.6 Worker (`application/internal/worker/`)
- **What Already Exists:**
  - Basic worker loop polling `ClaimQueued` and running `adapter.Solve`.
  - Heartbeat ticker.
- **What Is Incomplete / Missing:**
  - Lease expiration calculation based on `solver deadline + safety margin`.
  - Periodic recovery scan for expired worker leases.
  - Retry counter increment and `retry_count >= max_retries -> FAILED` transition.
  - Stale worker terminal protection (`worker_id = $self`).
  - Safe cancellation propagation.

### 2.7 API Handlers & Middleware (`application/internal/api/`)
- **What Already Exists:**
  - `handlers.go` has 6 basic endpoints.
  - `auth.go` has basic token extraction.
- **What Is Incomplete / Missing:**
  - Missing ~25 endpoints specified in `api/openapi.yaml`:
    - Full academic catalog CRUD endpoints.
    - Snapshot creation and download endpoints (`POST /timetables/:id/snapshots`, `GET /snapshots/:id`, `GET /snapshots/:id/problem`).
    - Run cancellation (`POST /runs/:id/cancel`) and independent verification (`GET /runs/:id/verify`, `POST /verify`).
    - Assignment editing (`POST /versions/:id/assignments/move`, `POST /versions/:id/assignments/swap`, `POST /versions/:id/assignments/pin`, `DELETE /versions/:id/assignments/pins/:pinId`).
    - Version lifecycle (`POST /versions/:id/submit-review`, `POST /versions/:id/publish`, `POST /versions/:id/send-back`, `POST /versions/:id/archive`).
  - Standard response envelope and error models conforming to OpenAPI 3.1.
  - Role-based authorization middleware enforcing `INSTITUTION_ADMIN`, `SCHEDULER`, `PROFESSOR`, `VIEWER`.

### 2.8 CI & Import Boundary Enforcement
- **What Already Exists:**
  - `application/scripts/check-curra-imports.sh` verifying that only `application/internal/curra` imports `internal/scheduler/...`.
- **What Is Incomplete / Missing:**
  - `.golangci.yml` with `depguard` configuration for mechanical static analysis.

---

## 3. Contradictions & Ambiguities Resolved

1. **`QUEUED` vs `PENDING` Status:**
   - In `database/schema.md` and `api/openapi.yaml`, the initial status is `QUEUED`.
   - The worker leasing queries will operate on `status = 'QUEUED'`.
2. **Deterministic Replay Requirements:**
   - Run creation must persist fully resolved solver options, seed, rule set hash, and engine metadata *before* starting search.
   - Solver execution must always run outside database transactions to prevent connection starvation.
3. **Cross-Tenant Access Code:**
   - All unauthorized cross-tenant resource lookups must return `404 Not Found` (never 403) to prevent tenant probing.

---

## 4. Phase Implementation Plan

- **Phase 1 — Database Foundation:** Add snapshot immutability trigger, idempotency table, worker lease columns, and database tests.
- **Phase 2 — Domain & Repositories:** Refine domain types, add CAS queries, worker lease queries, idempotency repository, and repository tests.
- **Phase 3 — CurraAdapter:** Implement `CurraAdapter` interface with `CompileConstraints`, `ValidateMove`, and DTO mappings.
- **Phase 4 — Application Services:** Implement `CatalogService`, `SnapshotService`, `RunService`, `VersionService`, `MoveSwapService`, `PublishingService`, `VerificationService` with `requireTenantMatch` and CAS.
- **Phase 5 — Worker Execution & Recovery:** Implement robust worker leasing (`FOR UPDATE SKIP LOCKED`), lease timeouts, recovery scan, retries, and cancellation.
- **Phase 6 — API Layer:** Implement full set of REST handlers matching `api/openapi.yaml`, RBAC middleware, and error mappings.
- **Phase 7 — Verification & CI:** Comprehensive tests (unit, integration, concurrency, tenant isolation, idempotency, snapshot immutability, worker recovery) and CI checks.
