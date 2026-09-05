package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/api"
	"github.com/sPreetham42/timetable-platform/application/internal/api/handlers"
	"github.com/sPreetham42/timetable-platform/application/internal/api/middleware"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
)

type mockRepos struct {
	timetables   map[uuid.UUID]domain.Timetable
	snapshots    map[uuid.UUID]domain.ProblemSnapshot
	runs         map[uuid.UUID]domain.ScheduleRun
	versions     map[uuid.UUID]domain.ScheduleVersion
	assignments  map[uuid.UUID][]domain.ScheduleAssignment
	auditEvents  []domain.AuditEvent
	idempotency  map[string]domain.IdempotencyKey
	departments  map[uuid.UUID]domain.Department
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

// Repos stubs
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
	return nil, false, nil
}
func (r *mockRunRepo) UpdateTerminalResult(ctx context.Context, id uuid.UUID, workerID string, status domain.ScheduleRunStatus, result, score, diagnostics, violations json.RawMessage, durationMs int64, curraVer, curraCommit, ruleSetHash *string) error {
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
	return nil
}
func (r *mockRunRepo) UpdateHeartbeat(ctx context.Context, runID uuid.UUID, workerID string) error {
	return nil
}
func (r *mockRunRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, updates map[string]any) error {
	return nil
}
func (r *mockRunRepo) Cancel(ctx context.Context, id uuid.UUID) (bool, error) {
	return true, nil
}
func (r *mockRunRepo) RecoverExpired(ctx context.Context, maxRetries int) (int, error) {
	return 0, nil
}

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

type mockAuditRepo struct{ m *mockRepos }
func (r *mockAuditRepo) Create(ctx context.Context, event domain.AuditEvent) error {
	r.m.auditEvents = append(r.m.auditEvents, event)
	return nil
}
func (r *mockAuditRepo) ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	return nil, nil
}

type mockIdempotencyRepo struct{ m *mockRepos }
func (r *mockIdempotencyRepo) Acquire(ctx context.Context, instID uuid.UUID, key, resourceType string) (*domain.IdempotencyKey, bool, error) {
	token := uuid.New()
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
	return &newKey, false, nil
}
func (r *mockIdempotencyRepo) Complete(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID, resourceID uuid.UUID, responseCode int, responseBody json.RawMessage) error {
	return nil
}
func (r *mockIdempotencyRepo) Release(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID) error {
	return nil
}
func (r *mockIdempotencyRepo) Get(ctx context.Context, instID uuid.UUID, key string) (*domain.IdempotencyKey, error) {
	return nil, nil
}

type mockCurraAdapter struct{}
func (a *mockCurraAdapter) Solve(ctx context.Context, req curra.SolveRequest) (curra.SolveResponse, error) {
	return curra.SolveResponse{Status: "SOLVED"}, nil
}
func (a *mockCurraAdapter) Verify(ctx context.Context, req curra.VerifyRequest) (curra.VerifyResponse, error) {
	return curra.VerifyResponse{Valid: true, Status: "SOLVED"}, nil
}
func (a *mockCurraAdapter) ValidateMove(ctx context.Context, req curra.ValidateMoveRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{Valid: true, Status: "SOLVED"}, nil
}
func (a *mockCurraAdapter) ValidateSwap(ctx context.Context, req curra.ValidateSwapRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{Valid: true, Status: "SOLVED"}, nil
}
func (a *mockCurraAdapter) CompileConstraints(ctx context.Context, req curra.CompileRequest) (curra.CompileResponse, error) {
	return curra.CompileResponse{RuleSetHash: "mock-hash"}, nil
}

func setupTestRouter(m *mockRepos) http.Handler {
	repos := &repositories.Repos{
		Timetables:          &mockTimetableRepo{m: m},
		Snapshots:           &mockSnapshotRepo{m: m},
		ScheduleRuns:        &mockRunRepo{m: m},
		ScheduleVersions:    &mockVersionRepo{m: m},
		ScheduleAssignments: &mockAssignmentRepo{m: m},
		AuditEvents:         &mockAuditRepo{m: m},
		Idempotency:         &mockIdempotencyRepo{m: m},
	}
	adapter := &mockCurraAdapter{}

	ttSvc := services.NewTimetableService(repos)
	snapSvc := services.NewSnapshotService(repos)
	runSvc := services.NewRunService(repos, adapter)
	verSvc := services.NewVersionService(repos)
	pubSvc := services.NewPublishingService(repos, adapter)
	msSvc := services.NewMoveSwapService(repos, adapter)
	vSvc := services.NewVerificationService(repos, adapter)
	catSvc := services.NewCatalogService(repos)
	sliceSvc := services.NewSliceService(repos, adapter)

	h := handlers.New(ttSvc, snapSvc, runSvc, verSvc, pubSvc, msSvc, vSvc, catSvc, sliceSvc)
	auth := middleware.NewAuthMiddleware()

	return api.NewRouter(h, auth)
}

func TestHealthEndpoint(t *testing.T) {
	m := newMockRepos()
	router := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestPreconditionRequired_MissingIfMatch(t *testing.T) {
	m := newMockRepos()
	router := setupTestRouter(m)

	instID := uuid.New()
	userID := uuid.New()
	verID := uuid.New()

	m.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/versions/"+verID.String(), bytes.NewBufferString(`{"name":"New Name"}`))
	req.Header.Set("X-Institution-ID", instID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Role", "SCHEDULER")
	// Notice: If-Match header is intentionally omitted

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected HTTP 428 Precondition Required when If-Match is missing, got: %d", rec.Code)
	}
}

func TestConflict_VersionMismatch(t *testing.T) {
	m := newMockRepos()
	router := setupTestRouter(m)

	instID := uuid.New()
	userID := uuid.New()
	verID := uuid.New()

	m.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		Status:        domain.VersionStatusDraft,
		Version:       2, // Current version is 2
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/versions/"+verID.String(), bytes.NewBufferString(`{"name":"New Name"}`))
	req.Header.Set("X-Institution-ID", instID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Role", "SCHEDULER")
	req.Header.Set("If-Match", "1") // Stale version 1

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected HTTP 409 Conflict when If-Match version mismatches, got: %d", rec.Code)
	}
}

func TestTenantIsolation_CrossTenantReturns404(t *testing.T) {
	m := newMockRepos()
	router := setupTestRouter(m)

	instA := uuid.New()
	instB := uuid.New()
	userB := uuid.New()
	ttA := uuid.New()

	m.timetables[ttA] = domain.Timetable{
		ID:            ttA,
		InstitutionID: instA,
		Name:          "Institution A Timetable",
		Version:       1,
	}

	// User from Institution B requests Timetable from Institution A
	req := httptest.NewRequest(http.MethodGet, "/api/v1/timetables/"+ttA.String(), nil)
	req.Header.Set("X-Institution-ID", instB.String())
	req.Header.Set("X-User-ID", userB.String())
	req.Header.Set("X-Role", "SCHEDULER")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 Not Found on cross-tenant access, got: %d", rec.Code)
	}
}
