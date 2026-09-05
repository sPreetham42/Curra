package worker_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/worker"
)

type mockWorkerRepos struct {
	runs          map[uuid.UUID]domain.ScheduleRun
	snapshots     map[uuid.UUID]domain.ProblemSnapshot
	versions      map[uuid.UUID]domain.ScheduleVersion
	assignments   map[uuid.UUID][]domain.ScheduleAssignment
	auditEvents   []domain.AuditEvent
	engineSnaps   map[uuid.UUID]domain.EngineSnapshot
}

func newMockWorkerRepos() *mockWorkerRepos {
	return &mockWorkerRepos{
		runs:        make(map[uuid.UUID]domain.ScheduleRun),
		snapshots:   make(map[uuid.UUID]domain.ProblemSnapshot),
		versions:    make(map[uuid.UUID]domain.ScheduleVersion),
		assignments: make(map[uuid.UUID][]domain.ScheduleAssignment),
		engineSnaps: make(map[uuid.UUID]domain.EngineSnapshot),
	}
}

// Mock implementations
type mockRunRepo struct{ m *mockWorkerRepos }

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
	return nil, nil
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
			exp := time.Now().Add(leaseDuration)
			run.LeaseExpiresAt = &exp
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
	return nil
}
func (r *mockRunRepo) Cancel(ctx context.Context, id uuid.UUID) (bool, error) {
	return true, nil
}
func (r *mockRunRepo) RecoverExpired(ctx context.Context, maxRetries int) (int, error) {
	count := 0
	for id, run := range r.m.runs {
		if run.Status == domain.StatusRunning && run.LeaseExpiresAt != nil && run.LeaseExpiresAt.Before(time.Now()) {
			if run.RetryCount < maxRetries {
				run.Status = domain.StatusQueued
				run.RetryCount++
				run.WorkerID = nil
				run.LeaseExpiresAt = nil
			} else {
				run.Status = domain.StatusFailed
			}
			r.m.runs[id] = run
			count++
		}
	}
	return count, nil
}

type mockSnapshotRepo struct{ m *mockWorkerRepos }

func (r *mockSnapshotRepo) Create(ctx context.Context, snap domain.ProblemSnapshot) error {
	r.m.snapshots[snap.ID] = snap
	return nil
}
func (r *mockSnapshotRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	snap, ok := r.m.snapshots[id]
	if !ok {
		return domain.ProblemSnapshot{}, repositories.ErrNotFound
	}
	return snap, nil
}
func (r *mockSnapshotRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	return nil, nil
}

type mockVersionRepo struct{ m *mockWorkerRepos }

func (r *mockVersionRepo) Create(ctx context.Context, ver domain.ScheduleVersion) error {
	r.m.versions[ver.ID] = ver
	return nil
}
func (r *mockVersionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, error) {
	return r.m.versions[id], nil
}
func (r *mockVersionRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error) {
	return nil, nil
}
func (r *mockVersionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleVersionStatus, expectedVersion int) error {
	return nil
}
func (r *mockVersionRepo) Update(ctx context.Context, ver domain.ScheduleVersion) error {
	return nil
}
func (r *mockVersionRepo) ApplyAssignmentUpdateTx(ctx context.Context, versionID uuid.UUID, expectedVersion int, scoreJSON json.RawMessage, assignments []domain.ScheduleAssignment, audit domain.AuditEvent) error {
	return nil
}
func (r *mockVersionRepo) PublishTx(ctx context.Context, versionID uuid.UUID, expectedVersion int, timetableID uuid.UUID, audit domain.AuditEvent) error {
	return nil
}

type mockAssignmentRepo struct{ m *mockWorkerRepos }

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

type mockAuditRepo struct{ m *mockWorkerRepos }

func (r *mockAuditRepo) Create(ctx context.Context, event domain.AuditEvent) error {
	r.m.auditEvents = append(r.m.auditEvents, event)
	return nil
}
func (r *mockAuditRepo) ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	return nil, nil
}

type mockEngineSnapshotRepo struct{ m *mockWorkerRepos }

func (r *mockEngineSnapshotRepo) Create(ctx context.Context, snap domain.EngineSnapshot) error {
	r.m.engineSnaps[snap.ScheduleRunID] = snap
	return nil
}
func (r *mockEngineSnapshotRepo) GetByRunID(ctx context.Context, runID uuid.UUID) (domain.EngineSnapshot, error) {
	snap, ok := r.m.engineSnaps[runID]
	if !ok {
		return domain.EngineSnapshot{}, repositories.ErrNotFound
	}
	return snap, nil
}

type mockCurraAdapter struct{}

func (a *mockCurraAdapter) Solve(ctx context.Context, req curra.SolveRequest) (curra.SolveResponse, error) {
	return curra.SolveResponse{
		Status:   "SOLVED",
		Solution: json.RawMessage(`{"assignments":[{"id":"sr-1#0","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"room-1","timeSlotId":"ts-1","instance":0}]}`),
		Score:    curra.ScoreDTO{HardViolations: 0, SoftPenalty: 0},
	}, nil
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
	return curra.CompileResponse{RuleSetHash: "mock-rule-hash"}, nil
}

func TestWorker_ClaimAndExecute(t *testing.T) {
	m := newMockWorkerRepos()
	repos := &repositories.Repos{
		ScheduleRuns:        &mockRunRepo{m: m},
		Snapshots:           &mockSnapshotRepo{m: m},
		ScheduleVersions:    &mockVersionRepo{m: m},
		ScheduleAssignments: &mockAssignmentRepo{m: m},
		AuditEvents:         &mockAuditRepo{m: m},
		EngineSnapshots:     &mockEngineSnapshotRepo{m: m},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	w := worker.New("worker-test-1", repos, &mockCurraAdapter{}, logger)

	instID := uuid.New()
	ttID := uuid.New()
	snapID := uuid.New()
	runID := uuid.New()
	seed := int64(42)

	m.snapshots[snapID] = domain.ProblemSnapshot{
		ID:            snapID,
		TimetableID:   ttID,
		InstitutionID: instID,
		ProblemJSON:   []byte(`{}`),
		ConstraintInstances: []byte(`[]`),
	}

	m.runs[runID] = domain.ScheduleRun{
		ID:            runID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.StatusQueued,
		Seed:          &seed,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go w.Start(ctx)

	time.Sleep(1500 * time.Millisecond)

	run := m.runs[runID]
	if run.Status != domain.StatusSolved {
		t.Fatalf("expected run status SOLVED, got %s", run.Status)
	}

	if len(m.versions) == 0 {
		t.Fatalf("expected auto-generated version to be created upon SOLVED status")
	}
}

func TestWorker_StaleWorkerRejected(t *testing.T) {
	m := newMockWorkerRepos()
	repos := &repositories.Repos{
		ScheduleRuns: &mockRunRepo{m: m},
		Snapshots:    &mockSnapshotRepo{m: m},
	}

	runID := uuid.New()
	otherWorkerID := "worker-other"
	m.runs[runID] = domain.ScheduleRun{
		ID:       runID,
		Status:   domain.StatusRunning,
		WorkerID: &otherWorkerID,
	}

	staleWorkerID := "worker-stale"
	err := repos.ScheduleRuns.UpdateTerminalResult(
		context.Background(),
		runID,
		staleWorkerID,
		domain.StatusSolved,
		nil, nil, nil, nil, 100, nil, nil, nil,
	)

	if err != repositories.ErrStaleWorker {
		t.Fatalf("expected ErrStaleWorker when stale worker attempts to commit, got: %v", err)
	}
}

func TestWorker_RetryCeilingReachesFailed(t *testing.T) {
	m := newMockWorkerRepos()
	runRepo := &mockRunRepo{m: m}

	runID := uuid.New()
	workerID := "worker-crashed"
	past := time.Now().Add(-10 * time.Minute)

	m.runs[runID] = domain.ScheduleRun{
		ID:             runID,
		Status:         domain.StatusRunning,
		WorkerID:       &workerID,
		LeaseExpiresAt: &past,
		RetryCount:     0,
	}

	const maxRetries = 2

	// Expiration 1: retry_count 0 -> 1, status -> QUEUED
	reaped, err := runRepo.RecoverExpired(context.Background(), maxRetries)
	if err != nil || reaped != 1 {
		t.Fatalf("expected 1 run reaped on first expiration, got %d, err: %v", reaped, err)
	}
	if m.runs[runID].Status != domain.StatusQueued || m.runs[runID].RetryCount != 1 {
		t.Fatalf("expected status QUEUED with retry_count 1, got %s, count %d", m.runs[runID].Status, m.runs[runID].RetryCount)
	}

	// Worker claims again and expires
	run := m.runs[runID]
	run.Status = domain.StatusRunning
	run.LeaseExpiresAt = &past
	m.runs[runID] = run

	// Expiration 2: retry_count 1 -> 2, status -> QUEUED
	reaped, err = runRepo.RecoverExpired(context.Background(), maxRetries)
	if err != nil || reaped != 1 {
		t.Fatalf("expected 1 run reaped on second expiration, got %d, err: %v", reaped, err)
	}
	if m.runs[runID].Status != domain.StatusQueued || m.runs[runID].RetryCount != 2 {
		t.Fatalf("expected status QUEUED with retry_count 2, got %s, count %d", m.runs[runID].Status, m.runs[runID].RetryCount)
	}

	// Worker claims again and expires for the 3rd time (exceeds maxRetries = 2)
	run = m.runs[runID]
	run.Status = domain.StatusRunning
	run.LeaseExpiresAt = &past
	m.runs[runID] = run

	// Expiration 3: retry_count 2 >= maxRetries -> terminal status FAILED
	reaped, err = runRepo.RecoverExpired(context.Background(), maxRetries)
	if err != nil || reaped != 1 {
		t.Fatalf("expected 1 run reaped on third expiration, got %d, err: %v", reaped, err)
	}
	if m.runs[runID].Status != domain.StatusFailed {
		t.Fatalf("expected terminal status FAILED after exceeding retry ceiling, got %s", m.runs[runID].Status)
	}
}
