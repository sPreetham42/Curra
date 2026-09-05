package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
)

// Mock repos implementation for testing services
type mockRepos struct {
	timetables  map[uuid.UUID]domain.Timetable
	snapshots   map[uuid.UUID]domain.ProblemSnapshot
	runs        map[uuid.UUID]domain.ScheduleRun
	versions    map[uuid.UUID]domain.ScheduleVersion
	assignments map[uuid.UUID][]domain.ScheduleAssignment
	auditEvents []domain.AuditEvent
	idempotency map[string]domain.IdempotencyKey
	departments map[uuid.UUID]domain.Department
}

func newMockRepos() *mockRepos {
	return &mockRepos{
		timetables:  make(map[uuid.UUID]domain.Timetable),
		snapshots:   make(map[uuid.UUID]domain.ProblemSnapshot),
		runs:        make(map[uuid.UUID]domain.ScheduleRun),
		versions:    make(map[uuid.UUID]domain.ScheduleVersion),
		assignments: make(map[uuid.UUID][]domain.ScheduleAssignment),
		idempotency: make(map[string]domain.IdempotencyKey),
		departments: make(map[uuid.UUID]domain.Department),
	}
}

// Mock TimetableRepo
type mockTimetableRepo struct{ m *mockRepos }

func (r *mockTimetableRepo) Create(ctx context.Context, tt domain.Timetable) error {
	r.m.timetables[tt.ID] = tt
	return nil
}
func (r *mockTimetableRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error) {
	tt, ok := r.m.timetables[id]
	if !ok {
		return domain.Timetable{}, repositories.ErrNotFound
	}
	return tt, nil
}
func (r *mockTimetableRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Timetable, error) {
	var list []domain.Timetable
	for _, tt := range r.m.timetables {
		if tt.InstitutionID == instID {
			list = append(list, tt)
		}
	}
	return list, nil
}
func (r *mockTimetableRepo) Update(ctx context.Context, tt domain.Timetable) error {
	existing, ok := r.m.timetables[tt.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != tt.Version {
		return repositories.ErrOptimisticLock
	}
	tt.Version++
	r.m.timetables[tt.ID] = tt
	return nil
}
func (r *mockTimetableRepo) SetCurrentPublishedVersion(ctx context.Context, timetableID, versionID uuid.UUID) error {
	tt, ok := r.m.timetables[timetableID]
	if !ok {
		return repositories.ErrNotFound
	}
	tt.CurrentPublishedVersionID = &versionID
	r.m.timetables[timetableID] = tt
	return nil
}

// Mock SnapshotRepo
type mockSnapshotRepo struct{ m *mockRepos }

func (r *mockSnapshotRepo) Create(ctx context.Context, snap domain.ProblemSnapshot) error {
	r.m.snapshots[snap.ID] = snap
	return nil
}
func (r *mockSnapshotRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	s, ok := r.m.snapshots[id]
	if !ok {
		return domain.ProblemSnapshot{}, repositories.ErrNotFound
	}
	return s, nil
}
func (r *mockSnapshotRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	var list []domain.ProblemSnapshot
	for _, s := range r.m.snapshots {
		if s.TimetableID == timetableID {
			list = append(list, s)
		}
	}
	return list, nil
}

// Mock ScheduleRunRepo
type mockRunRepo struct{ m *mockRepos }

func (r *mockRunRepo) Create(ctx context.Context, run domain.ScheduleRun) error {
	r.m.runs[run.ID] = run
	return nil
}
func (r *mockRunRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	run, ok := r.m.runs[id]
	if !ok {
		return domain.ScheduleRun{}, repositories.ErrNotFound
	}
	return run, nil
}
func (r *mockRunRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleRun, error) {
	var list []domain.ScheduleRun
	for _, run := range r.m.runs {
		if run.TimetableID == timetableID {
			list = append(list, run)
		}
	}
	return list, nil
}
func (r *mockRunRepo) Update(ctx context.Context, run domain.ScheduleRun) error {
	r.m.runs[run.ID] = run
	return nil
}
func (r *mockRunRepo) ClaimQueued(ctx context.Context, workerID string, leaseDuration time.Duration) (*domain.ScheduleRun, bool, error) {
	for _, run := range r.m.runs {
		if run.Status == domain.StatusQueued {
			run.Status = domain.StatusRunning
			run.WorkerID = &workerID
			r.m.runs[run.ID] = run
			return &run, true, nil
		}
	}
	return nil, false, nil
}
func (r *mockRunRepo) UpdateTerminalResult(ctx context.Context, id uuid.UUID, workerID string, status domain.ScheduleRunStatus, result, score, diagnostics, violations json.RawMessage, durationMs int64, curraVer, curraCommit, ruleSetHash *string) error {
	run, ok := r.m.runs[id]
	if !ok || run.WorkerID == nil || *run.WorkerID != workerID {
		return repositories.ErrStaleWorker
	}
	run.Status = status
	run.Result = result
	run.Score = score
	r.m.runs[id] = run
	return nil
}
func (r *mockRunRepo) CommitTerminalResultTx(
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
	run, ok := r.m.runs[runID]
	if !ok || run.WorkerID == nil || *run.WorkerID != workerID {
		return repositories.ErrStaleWorker
	}
	run.Status = status
	run.Result = result
	run.Score = score
	run.Diagnostics = diagnostics
	run.Violations = violations
	run.DurationMs = &durationMs
	run.CurrAVersion = curraVer
	run.CurrACommit = curraCommit
	run.RuleSetHash = ruleSetHash
	r.m.runs[runID] = run

	if draftVersion != nil {
		r.m.versions[draftVersion.ID] = *draftVersion
		if len(assignments) > 0 {
			r.m.assignments[draftVersion.ID] = assignments
		}
	}
	r.m.auditEvents = append(r.m.auditEvents, audit)
	return nil
}
func (r *mockRunRepo) UpdateHeartbeat(ctx context.Context, runID uuid.UUID, workerID string) error {
	return nil
}
func (r *mockRunRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, updates map[string]any) error {
	run, ok := r.m.runs[id]
	if !ok {
		return repositories.ErrNotFound
	}
	run.Status = status
	r.m.runs[id] = run
	return nil
}
func (r *mockRunRepo) Cancel(ctx context.Context, id uuid.UUID) (bool, error) {
	run, ok := r.m.runs[id]
	if !ok {
		return false, nil
	}
	if run.Status == domain.StatusQueued || run.Status == domain.StatusRunning {
		run.Status = domain.StatusCancelled
		r.m.runs[id] = run
		return true, nil
	}
	return false, nil
}
func (r *mockRunRepo) RecoverExpired(ctx context.Context, maxRetries int) (int, error) {
	return 0, nil
}

// Mock ScheduleVersionRepo
type mockVersionRepo struct{ m *mockRepos }

func (r *mockVersionRepo) Create(ctx context.Context, ver domain.ScheduleVersion) error {
	r.m.versions[ver.ID] = ver
	return nil
}
func (r *mockVersionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, error) {
	v, ok := r.m.versions[id]
	if !ok {
		return domain.ScheduleVersion{}, repositories.ErrNotFound
	}
	return v, nil
}
func (r *mockVersionRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error) {
	var list []domain.ScheduleVersion
	for _, v := range r.m.versions {
		if v.TimetableID == timetableID {
			list = append(list, v)
		}
	}
	return list, nil
}
func (r *mockVersionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleVersionStatus, expectedVersion int) error {
	v, ok := r.m.versions[id]
	if !ok {
		return repositories.ErrNotFound
	}
	if v.Version != expectedVersion {
		return repositories.ErrOptimisticLock
	}
	v.Status = status
	v.Version++
	r.m.versions[id] = v
	return nil
}
func (r *mockVersionRepo) Update(ctx context.Context, ver domain.ScheduleVersion) error {
	existing, ok := r.m.versions[ver.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != ver.Version {
		return repositories.ErrOptimisticLock
	}
	ver.Version++
	r.m.versions[ver.ID] = ver
	return nil
}
func (r *mockVersionRepo) ApplyAssignmentUpdateTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	scoreJSON json.RawMessage,
	assignments []domain.ScheduleAssignment,
	audit domain.AuditEvent,
) error {
	v, ok := r.m.versions[versionID]
	if !ok {
		return repositories.ErrNotFound
	}
	if v.Version != expectedVersion {
		return repositories.ErrOptimisticLock
	}
	v.Score = scoreJSON
	v.Version++
	r.m.versions[versionID] = v
	r.m.assignments[versionID] = assignments
	r.m.auditEvents = append(r.m.auditEvents, audit)
	return nil
}
func (r *mockVersionRepo) PublishTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	timetableID uuid.UUID,
	audit domain.AuditEvent,
) error {
	v, ok := r.m.versions[versionID]
	if !ok {
		return repositories.ErrNotFound
	}
	if v.Version != expectedVersion {
		return repositories.ErrOptimisticLock
	}
	v.Status = domain.VersionStatusPublished
	v.Version++
	r.m.versions[versionID] = v

	tt, ok := r.m.timetables[timetableID]
	if ok {
		tt.CurrentPublishedVersionID = &versionID
		r.m.timetables[timetableID] = tt
	}
	r.m.auditEvents = append(r.m.auditEvents, audit)
	return nil
}

// Mock ScheduleAssignmentRepo
type mockAssignmentRepo struct{ m *mockRepos }

func (r *mockAssignmentRepo) Create(ctx context.Context, a domain.ScheduleAssignment) error {
	r.m.assignments[a.VersionID] = append(r.m.assignments[a.VersionID], a)
	return nil
}
func (r *mockAssignmentRepo) CreateBatch(ctx context.Context, assignments []domain.ScheduleAssignment) error {
	for _, a := range assignments {
		r.m.assignments[a.VersionID] = append(r.m.assignments[a.VersionID], a)
	}
	return nil
}
func (r *mockAssignmentRepo) ReplaceAllForVersion(ctx context.Context, versionID uuid.UUID, assignments []domain.ScheduleAssignment) error {
	r.m.assignments[versionID] = assignments
	return nil
}
func (r *mockAssignmentRepo) ListByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.ScheduleAssignment, error) {
	return r.m.assignments[versionID], nil
}
func (r *mockAssignmentRepo) DeleteByVersion(ctx context.Context, versionID uuid.UUID) error {
	delete(r.m.assignments, versionID)
	return nil
}

// Mock AuditEventRepo
type mockAuditRepo struct{ m *mockRepos }

func (r *mockAuditRepo) Create(ctx context.Context, event domain.AuditEvent) error {
	r.m.auditEvents = append(r.m.auditEvents, event)
	return nil
}
func (r *mockAuditRepo) ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	var list []domain.AuditEvent
	for _, e := range r.m.auditEvents {
		if e.InstitutionID == instID {
			list = append(list, e)
		}
	}
	return list, nil
}

// Mock IdempotencyRepo
type mockIdempotencyRepo struct{ m *mockRepos }

func (r *mockIdempotencyRepo) Acquire(ctx context.Context, instID uuid.UUID, key, resourceType string) (*domain.IdempotencyKey, bool, error) {
	k := instID.String() + ":" + key
	token := uuid.New()
	existing, ok := r.m.idempotency[k]
	if ok {
		if existing.Status == domain.IdempotencyStatusCompleted {
			return &existing, true, nil
		}
		if existing.Status == domain.IdempotencyStatusInProgress {
			if existing.LockedAt.Before(time.Now().Add(-1 * time.Minute)) {
				existing.LockToken = &token
				existing.LockedAt = time.Now()
				existing.UpdatedAt = time.Now()
				r.m.idempotency[k] = existing
				return &existing, false, nil
			}
			return nil, false, repositories.ErrIdempotencyConflict
		}
	}
	newKey := domain.IdempotencyKey{
		ID:             uuid.New(),
		InstitutionID:  instID,
		IdempotencyKey: key,
		Status:         domain.IdempotencyStatusInProgress,
		ResourceType:   resourceType,
		LockToken:      &token,
		LockedAt:       time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	r.m.idempotency[k] = newKey
	return &newKey, false, nil
}

func (r *mockIdempotencyRepo) Complete(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID, resourceID uuid.UUID, responseCode int, responseBody json.RawMessage) error {
	k := instID.String() + ":" + key
	ik, ok := r.m.idempotency[k]
	if !ok {
		ik = domain.IdempotencyKey{
			ID:             uuid.New(),
			InstitutionID:  instID,
			IdempotencyKey: key,
		}
	}
	if lockToken != nil && ik.LockToken != nil && *ik.LockToken != *lockToken {
		return repositories.ErrIdempotencyConflict
	}
	ik.Status = domain.IdempotencyStatusCompleted
	ik.ResourceID = &resourceID
	ik.ResponseCode = &responseCode
	ik.ResponseBody = responseBody
	ik.UpdatedAt = time.Now()
	r.m.idempotency[k] = ik
	return nil
}

func (r *mockIdempotencyRepo) Release(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID) error {
	k := instID.String() + ":" + key
	existing, ok := r.m.idempotency[k]
	if ok && existing.Status == domain.IdempotencyStatusInProgress {
		if lockToken == nil || existing.LockToken == nil || *existing.LockToken == *lockToken {
			delete(r.m.idempotency, k)
		}
	}
	return nil
}

func (r *mockIdempotencyRepo) Get(ctx context.Context, instID uuid.UUID, key string) (*domain.IdempotencyKey, error) {
	k := instID.String() + ":" + key
	ik, ok := r.m.idempotency[k]
	if !ok {
		return nil, nil
	}
	return &ik, nil
}

// Mock CurraAdapter
type mockCurraAdapter struct{}

func (a *mockCurraAdapter) Solve(ctx context.Context, req curra.SolveRequest) (curra.SolveResponse, error) {
	return curra.SolveResponse{
		Status:   "SOLVED",
		Solution: json.RawMessage(`{"assignments":[{"id":"sr-1#0","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"room-1","timeSlotId":"ts-1","instance":0}]}`),
		Score:    curra.ScoreDTO{HardViolations: 0, SoftPenalty: 0},
		Metadata: curra.SolveMetadata{RuleSetHash: "mock-hash"},
	}, nil
}
func (a *mockCurraAdapter) Verify(ctx context.Context, req curra.VerifyRequest) (curra.VerifyResponse, error) {
	return curra.VerifyResponse{
		Valid:  true,
		Status: "SOLVED",
		Score:  curra.ScoreDTO{HardViolations: 0, SoftPenalty: 0},
	}, nil
}
func (a *mockCurraAdapter) ValidateMove(ctx context.Context, req curra.ValidateMoveRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{
		Valid:    true,
		Status:   "SOLVED",
		Score:    curra.ScoreDTO{HardViolations: 0, SoftPenalty: 0},
		Solution: json.RawMessage(`{"assignments":[{"id":"sr-1#0","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"room-2","timeSlotId":"ts-2","instance":0}]}`),
	}, nil
}
func (a *mockCurraAdapter) ValidateSwap(ctx context.Context, req curra.ValidateSwapRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{
		Valid:    true,
		Status:   "SOLVED",
		Score:    curra.ScoreDTO{HardViolations: 0, SoftPenalty: 0},
		Solution: json.RawMessage(`{"assignments":[{"id":"sr-1#0","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"room-2","timeSlotId":"ts-2","instance":0}]}`),
	}, nil
}
func (a *mockCurraAdapter) CompileConstraints(ctx context.Context, req curra.CompileRequest) (curra.CompileResponse, error) {
	return curra.CompileResponse{RuleSetHash: "mock-compiled-hash"}, nil
}

func buildTestRepos(m *mockRepos) *repositories.Repos {
	return &repositories.Repos{
		Timetables:          &mockTimetableRepo{m: m},
		Snapshots:           &mockSnapshotRepo{m: m},
		ScheduleRuns:        &mockRunRepo{m: m},
		ScheduleVersions:    &mockVersionRepo{m: m},
		ScheduleAssignments: &mockAssignmentRepo{m: m},
		AuditEvents:         &mockAuditRepo{m: m},
		Idempotency:         &mockIdempotencyRepo{m: m},
	}
}

func buildTestContext(instID, userID uuid.UUID, role domain.Role) context.Context {
	ctx := context.Background()
	ctx = services.ContextWithInstitution(ctx, domain.Institution{ID: instID, Name: "Test University", Slug: "test-univ"})
	ctx = services.ContextWithUser(ctx, domain.User{ID: userID, Email: "admin@test.edu", Name: "Admin User"})
	ctx = services.ContextWithRole(ctx, domain.UserRole{UserID: userID, InstitutionID: instID, Role: role})
	return ctx
}

func TestTenantIsolation_CrossTenantReturns404(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	ttSvc := services.NewTimetableService(repos)

	instA := uuid.New()
	instB := uuid.New()
	userA := uuid.New()

	ttB := uuid.New()
	m.timetables[ttB] = domain.Timetable{
		ID:            ttB,
		InstitutionID: instB,
		Name:          "Institution B Timetable",
		Version:       1,
	}

	ctxA := buildTestContext(instA, userA, domain.RoleScheduler)

	_, err := ttSvc.GetByID(ctxA, ttB)
	if err == nil {
		t.Fatalf("expected error accessing cross-tenant resource, got nil")
	}
	if !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-tenant resource, got: %v", err)
	}
}

func TestOptimisticLocking_VersionConflict(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	ttSvc := services.NewTimetableService(repos)

	instID := uuid.New()
	userID := uuid.New()
	ctx := buildTestContext(instID, userID, domain.RoleScheduler)

	ttID := uuid.New()
	m.timetables[ttID] = domain.Timetable{
		ID:            ttID,
		InstitutionID: instID,
		Name:          "Original Name",
		Version:       5,
	}

	// Stale If-Match version 4
	_, err := ttSvc.Update(ctx, ttID, "New Name", 4)
	if err == nil {
		t.Fatalf("expected conflict on stale version, got nil")
	}
	if !errors.Is(err, services.ErrConflict) {
		t.Fatalf("expected ErrConflict, got: %v", err)
	}
}

func TestPreconditionRequired_MissingIfMatch(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	adapter := &mockCurraAdapter{}
	moveSwapSvc := services.NewMoveSwapService(repos, adapter)

	instID := uuid.New()
	userID := uuid.New()
	ctx := buildTestContext(instID, userID, domain.RoleScheduler)

	verID := uuid.New()
	m.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}

	move := domain.MoveDTO{
		AssignmentID: "sr-1#0",
		From:         domain.PlacementDTO{RoomID: "room-1", TimeSlotID: "ts-1"},
		To:           domain.PlacementDTO{RoomID: "room-2", TimeSlotID: "ts-2"},
	}

	// Missing If-Match (ifMatchVersion = 0)
	_, _, err := moveSwapSvc.Move(ctx, verID, move, 0, false)
	if err == nil {
		t.Fatalf("expected ErrPreconditionRequired, got nil")
	}
	if !errors.Is(err, services.ErrPreconditionRequired) {
		t.Fatalf("expected ErrPreconditionRequired, got: %v", err)
	}
}

func TestVersionLifecycle_StateMachine(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	adapter := &mockCurraAdapter{}
	versionSvc := services.NewVersionService(repos)
	publishingSvc := services.NewPublishingService(repos, adapter)

	instID := uuid.New()
	schedulerID := uuid.New()
	adminID := uuid.New()

	schedulerCtx := buildTestContext(instID, schedulerID, domain.RoleScheduler)
	adminCtx := buildTestContext(instID, adminID, domain.RoleInstitutionAdmin)

	ttID := uuid.New()
	snapID := uuid.New()
	m.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Name: "Fall 2026", Version: 1}
	m.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	// 1. Create Draft Version (status DRAFT, version 1)
	ver, err := versionSvc.CreateVersion(schedulerCtx, ttID, nil, snapID, "Draft 1", schedulerID, "")
	if err != nil {
		t.Fatalf("create version failed: %v", err)
	}
	if ver.Status != domain.VersionStatusDraft {
		t.Fatalf("expected status DRAFT, got %s", ver.Status)
	}

	// 2. Submit for Review (DRAFT -> REVIEW, version 1 -> 2)
	reviewVer, err := versionSvc.SubmitReview(schedulerCtx, ver.ID, 1)
	if err != nil {
		t.Fatalf("submit review failed: %v", err)
	}
	if reviewVer.Status != domain.VersionStatusReview {
		t.Fatalf("expected status REVIEW, got %s", reviewVer.Status)
	}

	// 3. Publish (REVIEW -> PUBLISHED, version 2 -> 3)
	pubVer, err := publishingSvc.Publish(adminCtx, ver.ID, 2, adminID)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if pubVer.Status != domain.VersionStatusPublished {
		t.Fatalf("expected status PUBLISHED, got %s", pubVer.Status)
	}

	// Verify timetable published version was updated
	tt := m.timetables[ttID]
	if tt.CurrentPublishedVersionID == nil || *tt.CurrentPublishedVersionID != ver.ID {
		t.Fatalf("timetable published version ID was not updated")
	}
}

func TestVersionLifecycle_StateMachineInvalidTransitions(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	adapter := &mockCurraAdapter{}
	versionSvc := services.NewVersionService(repos)
	publishingSvc := services.NewPublishingService(repos, adapter)

	instID := uuid.New()
	adminID := uuid.New()
	adminCtx := buildTestContext(instID, adminID, domain.RoleInstitutionAdmin)

	verID := uuid.New()
	snapID := uuid.New()
	ttID := uuid.New()
	m.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Name: "Fall 2026", Version: 1}
	m.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	// Setup version in REVIEW status
	m.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusReview,
		Version:       1,
	}

	// Invalid Transition 1: REVIEW -> ARCHIVED directly (prohibited by state-machines.md)
	_, err := versionSvc.Archive(adminCtx, verID, 1)
	if err == nil {
		t.Fatalf("expected error archiving version in REVIEW status, got nil")
	}
	if !errors.Is(err, services.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}

	// Invalid Transition 2: DRAFT -> PUBLISHED directly
	draftID := uuid.New()
	m.versions[draftID] = domain.ScheduleVersion{
		ID:            draftID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}
	_, err = publishingSvc.Publish(adminCtx, draftID, 1, adminID)
	if err == nil {
		t.Fatalf("expected error publishing directly from DRAFT, got nil")
	}
	if !errors.Is(err, services.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}

	// Valid Transition: DRAFT -> ARCHIVED
	archivedVer, err := versionSvc.Archive(adminCtx, draftID, 1)
	if err != nil {
		t.Fatalf("expected DRAFT -> ARCHIVED to succeed, got error: %v", err)
	}
	if archivedVer.Status != domain.VersionStatusArchived {
		t.Fatalf("expected status ARCHIVED, got %s", archivedVer.Status)
	}
}

func TestRBAC_VersionPermissions(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	versionSvc := services.NewVersionService(repos)

	instID := uuid.New()
	userID := uuid.New()
	verID := uuid.New()
	m.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}

	// 1. Unauthenticated Context (empty ctx)
	_, err := versionSvc.SubmitReview(context.Background(), verID, 1)
	if err == nil {
		t.Fatalf("expected error on unauthenticated context, got nil")
	}

	// 2. Viewer Role (forbidden 403)
	viewerCtx := buildTestContext(instID, userID, domain.RoleViewer)
	_, err = versionSvc.SubmitReview(viewerCtx, verID, 1)
	if err == nil {
		t.Fatalf("expected ErrForbidden for viewer role, got nil")
	}
	if !errors.Is(err, services.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}

	// 3. Scheduler Role (allowed)
	schedulerCtx := buildTestContext(instID, userID, domain.RoleScheduler)
	submittedVer, err := versionSvc.SubmitReview(schedulerCtx, verID, 1)
	if err != nil {
		t.Fatalf("expected SubmitReview to succeed for scheduler, got: %v", err)
	}
	if submittedVer.Status != domain.VersionStatusReview {
		t.Fatalf("expected status REVIEW, got %s", submittedVer.Status)
	}
}

func TestIdempotency_AtomicDeduplication(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	adapter := &mockCurraAdapter{}
	runSvc := services.NewRunService(repos, adapter)

	instID := uuid.New()
	userID := uuid.New()
	ctx := buildTestContext(instID, userID, domain.RoleScheduler)

	ttID := uuid.New()
	snapID := uuid.New()
	m.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Name: "Fall 2026", Version: 1}
	m.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	const idempotencyKey = "client-unique-key-12345"

	// Request 1: Creates the run
	run1, err := runSvc.CreateRun(ctx, ttID, snapID, nil, nil, userID, idempotencyKey)
	if err != nil {
		t.Fatalf("first create run failed: %v", err)
	}

	// Request 2: Replays with same idempotency key
	run2, err := runSvc.CreateRun(ctx, ttID, snapID, nil, nil, userID, idempotencyKey)
	if err != nil {
		t.Fatalf("second create run failed: %v", err)
	}

	// Must be the exact same logical run ID
	if run1.ID != run2.ID {
		t.Fatalf("expected same run ID for duplicate idempotency key, got %s and %s", run1.ID, run2.ID)
	}
	if len(m.runs) != 1 {
		t.Fatalf("expected exactly 1 run created in database, got %d", len(m.runs))
	}
}

func TestRunCancellation_RaceSafe(t *testing.T) {
	m := newMockRepos()
	repos := buildTestRepos(m)
	adapter := &mockCurraAdapter{}
	runSvc := services.NewRunService(repos, adapter)

	instID := uuid.New()
	userID := uuid.New()
	ctx := buildTestContext(instID, userID, domain.RoleScheduler)

	ttID := uuid.New()
	snapID := uuid.New()
	m.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Name: "Fall 2026", Version: 1}
	m.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	// Create Run (status QUEUED)
	run, err := runSvc.CreateRun(ctx, ttID, snapID, nil, nil, userID, "")
	if err != nil {
		t.Fatalf("create run failed: %v", err)
	}

	// Cancel Run -> status CANCELLED
	cancelledRun, err := runSvc.CancelRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("cancel run failed: %v", err)
	}
	if cancelledRun.Status != domain.StatusCancelled {
		t.Fatalf("expected status CANCELLED, got %s", cancelledRun.Status)
	}

	// Cancel again -> successful no-op
	noOpRun, err := runSvc.CancelRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("second cancel failed: %v", err)
	}
	if noOpRun.Status != domain.StatusCancelled {
		t.Fatalf("expected status CANCELLED, got %s", noOpRun.Status)
	}
}
