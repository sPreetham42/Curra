package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
)

// Transactional memory database simulating PostgreSQL transaction isolation,
// row-level locking, and rollback semantics for concurrency & rollback testing.
type transactionalMemoryDB struct {
	mu          sync.Mutex
	timetables  map[uuid.UUID]domain.Timetable
	snapshots   map[uuid.UUID]domain.ProblemSnapshot
	runs        map[uuid.UUID]domain.ScheduleRun
	versions    map[uuid.UUID]domain.ScheduleVersion
	assignments map[uuid.UUID][]domain.ScheduleAssignment
	auditEvents []domain.AuditEvent
	idempotency map[string]domain.IdempotencyKey

	// Fail injection flags
	failCASInjection bool
}

func newTxMemoryDB() *transactionalMemoryDB {
	return &transactionalMemoryDB{
		timetables:  make(map[uuid.UUID]domain.Timetable),
		snapshots:   make(map[uuid.UUID]domain.ProblemSnapshot),
		runs:        make(map[uuid.UUID]domain.ScheduleRun),
		versions:    make(map[uuid.UUID]domain.ScheduleVersion),
		assignments: make(map[uuid.UUID][]domain.ScheduleAssignment),
		idempotency: make(map[string]domain.IdempotencyKey),
	}
}

type txTimetableRepo struct{ db *transactionalMemoryDB }

func (r *txTimetableRepo) Create(ctx context.Context, tt domain.Timetable) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.timetables[tt.ID] = tt
	return nil
}
func (r *txTimetableRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	tt, ok := r.db.timetables[id]
	if !ok {
		return domain.Timetable{}, repositories.ErrNotFound
	}
	return tt, nil
}
func (r *txTimetableRepo) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Timetable, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var list []domain.Timetable
	for _, tt := range r.db.timetables {
		if tt.InstitutionID == instID {
			list = append(list, tt)
		}
	}
	return list, nil
}
func (r *txTimetableRepo) Update(ctx context.Context, tt domain.Timetable) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.timetables[tt.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != tt.Version {
		return repositories.ErrOptimisticLock
	}
	tt.Version++
	r.db.timetables[tt.ID] = tt
	return nil
}
func (r *txTimetableRepo) SetCurrentPublishedVersion(ctx context.Context, timetableID, versionID uuid.UUID) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	tt, ok := r.db.timetables[timetableID]
	if !ok {
		return repositories.ErrNotFound
	}
	tt.CurrentPublishedVersionID = &versionID
	r.db.timetables[timetableID] = tt
	return nil
}

type txSnapshotRepo struct{ db *transactionalMemoryDB }

func (r *txSnapshotRepo) Create(ctx context.Context, snap domain.ProblemSnapshot) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.snapshots[snap.ID] = snap
	return nil
}
func (r *txSnapshotRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	s, ok := r.db.snapshots[id]
	if !ok {
		return domain.ProblemSnapshot{}, repositories.ErrNotFound
	}
	return s, nil
}
func (r *txSnapshotRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var list []domain.ProblemSnapshot
	for _, s := range r.db.snapshots {
		if s.TimetableID == timetableID {
			list = append(list, s)
		}
	}
	return list, nil
}

type txVersionRepo struct{ db *transactionalMemoryDB }

func (r *txVersionRepo) Create(ctx context.Context, ver domain.ScheduleVersion) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.versions[ver.ID] = ver
	return nil
}
func (r *txVersionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleVersion, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	v, ok := r.db.versions[id]
	if !ok {
		return domain.ScheduleVersion{}, repositories.ErrNotFound
	}
	return v, nil
}
func (r *txVersionRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleVersion, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var list []domain.ScheduleVersion
	for _, v := range r.db.versions {
		if v.TimetableID == timetableID {
			list = append(list, v)
		}
	}
	return list, nil
}
func (r *txVersionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleVersionStatus, expectedVersion int) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	v, ok := r.db.versions[id]
	if !ok {
		return repositories.ErrNotFound
	}
	if v.Version != expectedVersion {
		return repositories.ErrOptimisticLock
	}
	v.Status = status
	v.Version++
	r.db.versions[id] = v
	return nil
}
func (r *txVersionRepo) Update(ctx context.Context, ver domain.ScheduleVersion) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.versions[ver.ID]
	if !ok {
		return repositories.ErrNotFound
	}
	if existing.Version != ver.Version {
		return repositories.ErrOptimisticLock
	}
	ver.Version++
	r.db.versions[ver.ID] = ver
	return nil
}
func (r *txVersionRepo) ApplyAssignmentUpdateTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	scoreJSON json.RawMessage,
	assignments []domain.ScheduleAssignment,
	audit domain.AuditEvent,
) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	oldVer, ok := r.db.versions[versionID]
	if !ok {
		return repositories.ErrNotFound
	}

	// CAS check
	if oldVer.Version != expectedVersion || r.db.failCASInjection {
		return repositories.ErrOptimisticLock
	}

	// Atomic commit
	oldVer.Score = scoreJSON
	oldVer.Version++
	r.db.versions[versionID] = oldVer
	r.db.assignments[versionID] = assignments
	r.db.auditEvents = append(r.db.auditEvents, audit)
	return nil
}
func (r *txVersionRepo) PublishTx(
	ctx context.Context,
	versionID uuid.UUID,
	expectedVersion int,
	timetableID uuid.UUID,
	audit domain.AuditEvent,
) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	oldVer, ok := r.db.versions[versionID]
	if !ok {
		return repositories.ErrNotFound
	}
	if oldVer.Version != expectedVersion || r.db.failCASInjection {
		return repositories.ErrOptimisticLock
	}

	// 1. Atomically archive any currently PUBLISHED version for this timetable
	for id, v := range r.db.versions {
		if v.TimetableID == timetableID && v.Status == domain.VersionStatusPublished && id != versionID {
			v.Status = domain.VersionStatusArchived
			v.UpdatedAt = time.Now()
			r.db.versions[id] = v
		}
	}

	// 2. Transition target version to PUBLISHED
	oldVer.Status = domain.VersionStatusPublished
	oldVer.Version++
	oldVer.UpdatedAt = time.Now()
	r.db.versions[versionID] = oldVer

	// 3. Update timetable pointer
	tt, ok := r.db.timetables[timetableID]
	if ok {
		tt.CurrentPublishedVersionID = &versionID
		tt.UpdatedAt = time.Now()
		r.db.timetables[timetableID] = tt
	}
	r.db.auditEvents = append(r.db.auditEvents, audit)
	return nil
}

type txAssignmentRepo struct{ db *transactionalMemoryDB }

func (r *txAssignmentRepo) Create(ctx context.Context, a domain.ScheduleAssignment) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.assignments[a.VersionID] = append(r.db.assignments[a.VersionID], a)
	return nil
}
func (r *txAssignmentRepo) CreateBatch(ctx context.Context, assignments []domain.ScheduleAssignment) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	for _, a := range assignments {
		r.db.assignments[a.VersionID] = append(r.db.assignments[a.VersionID], a)
	}
	return nil
}
func (r *txAssignmentRepo) ReplaceAllForVersion(ctx context.Context, versionID uuid.UUID, assignments []domain.ScheduleAssignment) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.assignments[versionID] = assignments
	return nil
}
func (r *txAssignmentRepo) ListByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.ScheduleAssignment, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	return r.db.assignments[versionID], nil
}
func (r *txAssignmentRepo) DeleteByVersion(ctx context.Context, versionID uuid.UUID) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	delete(r.db.assignments, versionID)
	return nil
}

type txRunRepo struct{ db *transactionalMemoryDB }

func (r *txRunRepo) Create(ctx context.Context, run domain.ScheduleRun) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.runs[run.ID] = run
	return nil
}
func (r *txRunRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	run, ok := r.db.runs[id]
	if !ok {
		return domain.ScheduleRun{}, repositories.ErrNotFound
	}
	return run, nil
}
func (r *txRunRepo) ListByTimetable(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleRun, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var list []domain.ScheduleRun
	for _, run := range r.db.runs {
		if run.TimetableID == timetableID {
			list = append(list, run)
		}
	}
	return list, nil
}
func (r *txRunRepo) Update(ctx context.Context, run domain.ScheduleRun) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.runs[run.ID] = run
	return nil
}
func (r *txRunRepo) ClaimQueued(ctx context.Context, workerID string, leaseDuration time.Duration) (*domain.ScheduleRun, bool, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	for _, run := range r.db.runs {
		if run.Status == domain.StatusQueued {
			run.Status = domain.StatusRunning
			run.WorkerID = &workerID

			// Dynamic lease calculation matching SQL implementation
			leaseSec := 300
			var cfg struct {
				TimeoutSeconds     int `json:"timeoutSeconds"`
				MaxDurationSeconds int `json:"maxDurationSeconds"`
				MaxNodes           int `json:"maxNodes"`
			}
			if len(run.SolverConfig) > 0 {
				_ = json.Unmarshal(run.SolverConfig, &cfg)
				if cfg.TimeoutSeconds > 0 {
					leaseSec = cfg.TimeoutSeconds
				} else if cfg.MaxDurationSeconds > 0 {
					leaseSec = cfg.MaxDurationSeconds
				} else if cfg.MaxNodes > 0 {
					est := cfg.MaxNodes / 1000
					if est > 300 {
						leaseSec = est
					}
				}
			}
			exp := time.Now().Add(time.Duration(leaseSec+120) * time.Second)
			run.LeaseExpiresAt = &exp
			r.db.runs[run.ID] = run
			return &run, true, nil
		}
	}
	return nil, false, nil
}
func (r *txRunRepo) UpdateTerminalResult(ctx context.Context, id uuid.UUID, workerID string, status domain.ScheduleRunStatus, result, score, diagnostics, violations json.RawMessage, durationMs int64, curraVer, curraCommit, ruleSetHash *string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	run, ok := r.db.runs[id]
	if !ok || run.WorkerID == nil || *run.WorkerID != workerID || run.Status != domain.StatusRunning {
		return repositories.ErrStaleWorker
	}
	run.Status = status
	run.Result = result
	run.Score = score
	r.db.runs[id] = run
	return nil
}
func (r *txRunRepo) CommitTerminalResultTx(
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
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	run, ok := r.db.runs[runID]
	if !ok || run.WorkerID == nil || *run.WorkerID != workerID || run.Status != domain.StatusRunning {
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
	r.db.runs[runID] = run

	if draftVersion != nil {
		r.db.versions[draftVersion.ID] = *draftVersion
		if len(assignments) > 0 {
			r.db.assignments[draftVersion.ID] = assignments
		}
	}
	r.db.auditEvents = append(r.db.auditEvents, audit)
	return nil
}
func (r *txRunRepo) UpdateHeartbeat(ctx context.Context, runID uuid.UUID, workerID string) error {
	return nil
}
func (r *txRunRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, updates map[string]any) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	run, ok := r.db.runs[id]
	if !ok {
		return repositories.ErrNotFound
	}
	run.Status = status
	r.db.runs[id] = run
	return nil
}
func (r *txRunRepo) Cancel(ctx context.Context, id uuid.UUID) (bool, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	run, ok := r.db.runs[id]
	if !ok {
		return false, nil
	}
	if run.Status == domain.StatusQueued || run.Status == domain.StatusRunning {
		run.Status = domain.StatusCancelled
		r.db.runs[id] = run
		return true, nil
	}
	return false, nil
}
func (r *txRunRepo) RecoverExpired(ctx context.Context, maxRetries int) (int, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	count := 0
	for id, run := range r.db.runs {
		if run.Status == domain.StatusRunning && run.LeaseExpiresAt != nil && run.LeaseExpiresAt.Before(time.Now()) {
			if run.RetryCount < maxRetries {
				run.Status = domain.StatusQueued
				run.RetryCount++
				run.WorkerID = nil
				run.LeaseExpiresAt = nil
			} else {
				run.Status = domain.StatusFailed
			}
			r.db.runs[id] = run
			count++
		}
	}
	return count, nil
}

type txAuditRepo struct{ db *transactionalMemoryDB }

func (r *txAuditRepo) Create(ctx context.Context, event domain.AuditEvent) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	r.db.auditEvents = append(r.db.auditEvents, event)
	return nil
}
func (r *txAuditRepo) ListByInstitution(ctx context.Context, instID uuid.UUID, limit int) ([]domain.AuditEvent, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var list []domain.AuditEvent
	for _, e := range r.db.auditEvents {
		if e.InstitutionID == instID {
			list = append(list, e)
		}
	}
	return list, nil
}

type txIdempotencyRepo struct{ db *transactionalMemoryDB }

func (r *txIdempotencyRepo) Acquire(ctx context.Context, instID uuid.UUID, key, resourceType string) (*domain.IdempotencyKey, bool, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	k := instID.String() + ":" + key
	myToken := uuid.New()

	existing, ok := r.db.idempotency[k]
	if ok {
		if existing.Status == domain.IdempotencyStatusCompleted {
			return &existing, true, nil
		}
		if existing.Status == domain.IdempotencyStatusInProgress {
			// Stale check (> 1 minute)
			if existing.LockedAt.Before(time.Now().Add(-1 * time.Minute)) {
				// Reclaim with new token
				existing.LockToken = &myToken
				existing.LockedAt = time.Now()
				existing.UpdatedAt = time.Now()
				r.db.idempotency[k] = existing
				return &existing, false, nil
			}
			// Active lock held by another token
			return nil, false, repositories.ErrIdempotencyConflict
		}
	}

	newKey := domain.IdempotencyKey{
		ID:             uuid.New(),
		InstitutionID:  instID,
		IdempotencyKey: key,
		Status:         domain.IdempotencyStatusInProgress,
		ResourceType:   resourceType,
		LockToken:      &myToken,
		LockedAt:       time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	r.db.idempotency[k] = newKey
	return &newKey, false, nil
}

func (r *txIdempotencyRepo) Complete(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID, resourceID uuid.UUID, responseCode int, responseBody json.RawMessage) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	k := instID.String() + ":" + key
	ik, ok := r.db.idempotency[k]
	if !ok {
		ik = domain.IdempotencyKey{ID: uuid.New(), InstitutionID: instID, IdempotencyKey: key}
	}
	if lockToken != nil && ik.LockToken != nil && *ik.LockToken != *lockToken {
		return repositories.ErrIdempotencyConflict
	}
	ik.Status = domain.IdempotencyStatusCompleted
	ik.ResourceID = &resourceID
	ik.ResponseCode = &responseCode
	ik.ResponseBody = responseBody
	ik.UpdatedAt = time.Now()
	r.db.idempotency[k] = ik
	return nil
}

func (r *txIdempotencyRepo) Release(ctx context.Context, instID uuid.UUID, key string, lockToken *uuid.UUID) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	k := instID.String() + ":" + key
	existing, ok := r.db.idempotency[k]
	if ok && existing.Status == domain.IdempotencyStatusInProgress {
		if lockToken == nil || existing.LockToken == nil || *existing.LockToken == *lockToken {
			delete(r.db.idempotency, k)
		}
	}
	return nil
}

func (r *txIdempotencyRepo) Get(ctx context.Context, instID uuid.UUID, key string) (*domain.IdempotencyKey, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	k := instID.String() + ":" + key
	ik, ok := r.db.idempotency[k]
	if !ok {
		return nil, nil
	}
	return &ik, nil
}

type mockAdapter struct{}

func (a *mockAdapter) Solve(ctx context.Context, req curra.SolveRequest) (curra.SolveResponse, error) {
	return curra.SolveResponse{Status: "SOLVED", RuleSetHash: "h1"}, nil
}
func (a *mockAdapter) Verify(ctx context.Context, req curra.VerifyRequest) (curra.VerifyResponse, error) {
	return curra.VerifyResponse{Valid: true, Status: "SOLVED", Score: curra.ScoreDTO{HardViolations: 0}}, nil
}
func (a *mockAdapter) ValidateMove(ctx context.Context, req curra.ValidateMoveRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{
		Valid:    true,
		Status:   "SOLVED",
		Score:    curra.ScoreDTO{HardViolations: 0},
		Solution: json.RawMessage(fmt.Sprintf(`{"assignments":[{"id":"%s","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"%s","timeSlotId":"%s","instance":0}]}`, req.Move.AssignmentID, req.Move.To.RoomID, req.Move.To.TimeSlotID)),
	}, nil
}
func (a *mockAdapter) ValidateSwap(ctx context.Context, req curra.ValidateSwapRequest) (curra.ValidateMoveResponse, error) {
	return curra.ValidateMoveResponse{
		Valid:    true,
		Status:   "SOLVED",
		Score:    curra.ScoreDTO{HardViolations: 0},
		Solution: json.RawMessage(fmt.Sprintf(`{"assignments":[{"id":"%s","courseOfferingId":"co-1","sessionRequirementId":"sr-1","studentGroupId":"sg-1","facultyId":"fac-1","roomId":"%s","timeSlotId":"%s","instance":0}]}`, req.Swap.Assignment1ID, req.Swap.Placement1.RoomID, req.Swap.Placement1.TimeSlotID)),
	}, nil
}
func (a *mockAdapter) CompileConstraints(ctx context.Context, req curra.CompileRequest) (curra.CompileResponse, error) {
	return curra.CompileResponse{RuleSetHash: "h1"}, nil
}

func buildTxTestRepos(db *transactionalMemoryDB) *repositories.Repos {
	return &repositories.Repos{
		Timetables:          &txTimetableRepo{db: db},
		Snapshots:           &txSnapshotRepo{db: db},
		ScheduleRuns:        &txRunRepo{db: db},
		ScheduleVersions:    &txVersionRepo{db: db},
		ScheduleAssignments: &txAssignmentRepo{db: db},
		AuditEvents:         &txAuditRepo{db: db},
		Idempotency:         &txIdempotencyRepo{db: db},
	}
}

func buildContext(instID, userID uuid.UUID, role domain.Role) context.Context {
	ctx := context.Background()
	ctx = services.ContextWithInstitution(ctx, domain.Institution{ID: instID, Name: "Test Univ"})
	ctx = services.ContextWithUser(ctx, domain.User{ID: userID, Email: "u@test.edu", Name: "U"})
	ctx = services.ContextWithRole(ctx, domain.UserRole{UserID: userID, InstitutionID: instID, Role: role})
	return ctx
}

// ---------------------------------------------------------------------------
// TEST A — Subsequent Publish Transitions Old Published Version to Archived
// ---------------------------------------------------------------------------
func TestPublish_SubsequentPublishArchivesOldVersion(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	pubSvc := services.NewPublishingService(repos, adapter)

	instID := uuid.New()
	admin := uuid.New()
	ctx := buildContext(instID, admin, domain.RoleInstitutionAdmin)

	ttID := uuid.New()
	snapID := uuid.New()
	ver1ID := uuid.New()
	ver2ID := uuid.New()

	db.timetables[ttID] = domain.Timetable{
		ID:                        ttID,
		InstitutionID:             instID,
		CurrentPublishedVersionID: &ver1ID,
		Version:                   1,
	}
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	// Version 1 is currently PUBLISHED
	db.versions[ver1ID] = domain.ScheduleVersion{
		ID:            ver1ID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusPublished,
		Version:       5,
	}
	// Version 2 is in REVIEW
	db.versions[ver2ID] = domain.ScheduleVersion{
		ID:            ver2ID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusReview,
		Version:       1,
	}

	// Publish Version 2
	publishedVer, err := pubSvc.Publish(ctx, ver2ID, 1, admin)
	if err != nil {
		t.Fatalf("expected successful publication of version 2, got: %v", err)
	}

	if publishedVer.Status != domain.VersionStatusPublished {
		t.Fatalf("expected version 2 to be PUBLISHED, got %s", publishedVer.Status)
	}

	// Verify Version 1 was transitioned to ARCHIVED
	oldVer := db.versions[ver1ID]
	if oldVer.Status != domain.VersionStatusArchived {
		t.Fatalf("expected version 1 to be transitioned to ARCHIVED, got %s", oldVer.Status)
	}

	// Verify timetable published pointer updated to Version 2
	tt := db.timetables[ttID]
	if tt.CurrentPublishedVersionID == nil || *tt.CurrentPublishedVersionID != ver2ID {
		t.Fatalf("expected timetable published version ID to point to version 2")
	}
}

// ---------------------------------------------------------------------------
// TEST B — Publish Rollback Preserves Old Published Version
// ---------------------------------------------------------------------------
func TestPublish_RollbackPreservesOldPublished(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	pubSvc := services.NewPublishingService(repos, adapter)

	instID := uuid.New()
	admin := uuid.New()
	ctx := buildContext(instID, admin, domain.RoleInstitutionAdmin)

	ttID := uuid.New()
	snapID := uuid.New()
	ver1ID := uuid.New()
	ver2ID := uuid.New()

	db.timetables[ttID] = domain.Timetable{
		ID:                        ttID,
		InstitutionID:             instID,
		CurrentPublishedVersionID: &ver1ID,
		Version:                   1,
	}
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	db.versions[ver1ID] = domain.ScheduleVersion{
		ID:            ver1ID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusPublished,
		Version:       5,
	}
	db.versions[ver2ID] = domain.ScheduleVersion{
		ID:            ver2ID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusReview,
		Version:       1,
	}

	// Inject CAS failure during transaction
	db.failCASInjection = true

	_, err := pubSvc.Publish(ctx, ver2ID, 1, admin)
	if err == nil {
		t.Fatalf("expected publish to fail on injected CAS error, got nil")
	}

	// Verify rollback invariants:
	// 1. Version 1 remains PUBLISHED
	if db.versions[ver1ID].Status != domain.VersionStatusPublished {
		t.Fatalf("expected version 1 to remain PUBLISHED on rollback, got %s", db.versions[ver1ID].Status)
	}
	// 2. Version 2 remains REVIEW
	if db.versions[ver2ID].Status != domain.VersionStatusReview {
		t.Fatalf("expected version 2 to remain REVIEW on rollback, got %s", db.versions[ver2ID].Status)
	}
	// 3. Timetable pointer unchanged
	if *db.timetables[ttID].CurrentPublishedVersionID != ver1ID {
		t.Fatalf("expected timetable pointer to still point to version 1")
	}
}

// ---------------------------------------------------------------------------
// TEST C — Concurrent Publish Race
// ---------------------------------------------------------------------------
func TestConcurrency_PublishRace(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	pubSvc := services.NewPublishingService(repos, adapter)

	instID := uuid.New()
	adminA := uuid.New()
	adminB := uuid.New()
	ctxA := buildContext(instID, adminA, domain.RoleInstitutionAdmin)
	ctxB := buildContext(instID, adminB, domain.RoleInstitutionAdmin)

	verID := uuid.New()
	snapID := uuid.New()
	ttID := uuid.New()
	db.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Version: 1}
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}
	db.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		TimetableID:   ttID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusReview,
		Version:       3,
	}

	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, errA = pubSvc.Publish(ctxA, verID, 3, adminA)
	}()
	go func() {
		defer wg.Done()
		_, errB = pubSvc.Publish(ctxB, verID, 3, adminB)
	}()
	wg.Wait()

	successCount := 0
	conflictCount := 0
	if errA == nil {
		successCount++
	} else if errors.Is(errA, services.ErrConflict) || errors.Is(errA, services.ErrInvalidState) {
		conflictCount++
	}
	if errB == nil {
		successCount++
	} else if errors.Is(errB, services.ErrConflict) || errors.Is(errB, services.ErrInvalidState) {
		conflictCount++
	}

	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 publish success and 1 conflict, got %d and %d", successCount, conflictCount)
	}

	tt := db.timetables[ttID]
	if tt.CurrentPublishedVersionID == nil || *tt.CurrentPublishedVersionID != verID {
		t.Fatalf("timetable published version pointer missing or incorrect")
	}
}

// ---------------------------------------------------------------------------
// TEST D — Idempotency First-Writer Race (5 concurrent requests)
// ---------------------------------------------------------------------------
func TestConcurrency_IdempotencyRace(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	runSvc := services.NewRunService(repos, adapter)

	instID := uuid.New()
	user := uuid.New()
	ctx := buildContext(instID, user, domain.RoleScheduler)

	ttID := uuid.New()
	snapID := uuid.New()
	db.timetables[ttID] = domain.Timetable{ID: ttID, InstitutionID: instID, Version: 1}
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, TimetableID: ttID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}

	const sharedKey = "concurrent-idempotency-key-999"

	var wg sync.WaitGroup
	const numConcurrent = 5
	results := make([]domain.ScheduleRun, numConcurrent)
	errs := make([]error, numConcurrent)
	wg.Add(numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx], errs[idx] = runSvc.CreateRun(ctx, ttID, snapID, nil, nil, user, sharedKey)
		}()
	}
	wg.Wait()

	var firstRunID uuid.UUID
	for i := 0; i < numConcurrent; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d returned error: %v", i, errs[i])
		}
		if firstRunID == uuid.Nil {
			firstRunID = results[i].ID
		} else if results[i].ID != firstRunID {
			t.Fatalf("idempotency violated: got different run IDs %s vs %s", firstRunID, results[i].ID)
		}
	}

	if len(db.runs) != 1 {
		t.Fatalf("expected exactly 1 schedule run created, got %d", len(db.runs))
	}
}

// ---------------------------------------------------------------------------
// TEST E — Active Lock Ownership (Caller B rejected while A is active)
// ---------------------------------------------------------------------------
func TestIdempotency_ActiveLockOwnershipRejected(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)

	instID := uuid.New()
	key := "test-active-lock-key"

	// Caller A acquires key
	recordA, isCompA, errA := repos.Idempotency.Acquire(context.Background(), instID, key, "schedule_run")
	if errA != nil || isCompA || recordA == nil || recordA.LockToken == nil {
		t.Fatalf("caller A failed to acquire key: %v", errA)
	}

	// Caller B arrives immediately while A is IN_PROGRESS (fresh lock)
	recordB, isCompB, errB := repos.Idempotency.Acquire(context.Background(), instID, key, "schedule_run")
	if !errors.Is(errB, repositories.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict for caller B, got record: %v, comp: %v, err: %v", recordB, isCompB, errB)
	}
}

// ---------------------------------------------------------------------------
// TEST F — Stale Lock Reclamation
// ---------------------------------------------------------------------------
func TestIdempotency_StaleLockReclamation(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)

	instID := uuid.New()
	key := "test-stale-lock-key"

	tokenA := uuid.New()
	past := time.Now().Add(-2 * time.Minute)

	// Simulate crashed process A leaving a stale lock (> 1 minute old)
	k := instID.String() + ":" + key
	db.idempotency[k] = domain.IdempotencyKey{
		ID:             uuid.New(),
		InstitutionID:  instID,
		IdempotencyKey: key,
		Status:         domain.IdempotencyStatusInProgress,
		ResourceType:   "schedule_run",
		LockToken:      &tokenA,
		LockedAt:       past,
		CreatedAt:      past,
		UpdatedAt:      past,
	}

	// Caller B attempts acquisition -> should reclaim stale lock with a new token
	recordB, isCompB, errB := repos.Idempotency.Acquire(context.Background(), instID, key, "schedule_run")
	if errB != nil || isCompB || recordB == nil {
		t.Fatalf("expected caller B to reclaim stale lock, got: %v", errB)
	}

	if recordB.LockToken == nil || *recordB.LockToken == tokenA {
		t.Fatalf("expected new lock token for caller B distinct from tokenA")
	}
}

// ---------------------------------------------------------------------------
// TEST G — Idempotency Completion (Completed key returns stored result)
// ---------------------------------------------------------------------------
func TestIdempotency_CompletedReturnsStoredResult(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)

	instID := uuid.New()
	key := "test-completed-key"
	resID := uuid.New()
	resBody := json.RawMessage(`{"id":"` + resID.String() + `","status":"QUEUED"}`)

	// A acquires and completes
	recA, _, err := repos.Idempotency.Acquire(context.Background(), instID, key, "schedule_run")
	if err != nil {
		t.Fatalf("acquire error: %v", err)
	}

	err = repos.Idempotency.Complete(context.Background(), instID, key, recA.LockToken, resID, 201, resBody)
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}

	// B calls acquire -> should return stored result with isCompleted=true
	recB, isCompB, errB := repos.Idempotency.Acquire(context.Background(), instID, key, "schedule_run")
	if errB != nil || !isCompB || recB == nil {
		t.Fatalf("expected isCompleted=true with stored result, got: %v, comp: %v", errB, isCompB)
	}

	if string(recB.ResponseBody) != string(resBody) {
		t.Fatalf("expected response body %s, got %s", string(resBody), string(recB.ResponseBody))
	}
}

// ---------------------------------------------------------------------------
// TEST H — Dynamic Worker Lease Persistence
// ---------------------------------------------------------------------------
func TestWorker_DynamicLeasePersistence(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)

	instID := uuid.New()
	runID := uuid.New()

	// Solver config with 10-minute timeout (600s)
	solverCfg := json.RawMessage(`{"timeoutSeconds": 600}`)
	db.runs[runID] = domain.ScheduleRun{
		ID:            runID,
		InstitutionID: instID,
		Status:        domain.StatusQueued,
		SolverConfig:  solverCfg,
	}

	claimedRun, claimed, err := repos.ScheduleRuns.ClaimQueued(context.Background(), "worker-1", 5*time.Minute)
	if err != nil || !claimed || claimedRun == nil {
		t.Fatalf("claim failed: %v", err)
	}

	if claimedRun.LeaseExpiresAt == nil {
		t.Fatalf("expected lease_expires_at to be set")
	}

	// Expected lease duration = 600s + 120s safety margin = 720s (12 minutes)
	expectedMinExpiry := time.Now().Add(700 * time.Second)
	if claimedRun.LeaseExpiresAt.Before(expectedMinExpiry) {
		t.Fatalf("expected lease_expires_at to be > 700s from now (dynamic 12min), got %v", claimedRun.LeaseExpiresAt.Sub(time.Now()))
	}
}

// ---------------------------------------------------------------------------
// TEST I — Worker Lease Safety Margin
// ---------------------------------------------------------------------------
func TestWorker_LeaseSafetyMargin(t *testing.T) {
	testCases := []struct {
		name       string
		solverCfg  string
		minLeaseSec int
	}{
		{name: "short timeout (60s)", solverCfg: `{"timeoutSeconds":60}`, minLeaseSec: 180},
		{name: "long timeout (1800s)", solverCfg: `{"timeoutSeconds":1800}`, minLeaseSec: 1920},
		{name: "maxDuration (900s)", solverCfg: `{"maxDurationSeconds":900}`, minLeaseSec: 1020},
		{name: "fallback (empty)", solverCfg: `{}`, minLeaseSec: 420},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTxMemoryDB()
			repos := buildTxTestRepos(db)
			runID := uuid.New()

			db.runs[runID] = domain.ScheduleRun{
				ID:           runID,
				Status:       domain.StatusQueued,
				SolverConfig: json.RawMessage(tc.solverCfg),
			}

			claimedRun, _, err := repos.ScheduleRuns.ClaimQueued(context.Background(), "w-1", 0)
			if err != nil {
				t.Fatalf("claim failed: %v", err)
			}

			leaseDuration := claimedRun.LeaseExpiresAt.Sub(time.Now())
			if leaseDuration < time.Duration(tc.minLeaseSec-5)*time.Second {
				t.Fatalf("lease duration %v is smaller than expected minimum %d seconds", leaseDuration, tc.minLeaseSec)
			}
		})
	}
}

// Move Race Test
func TestConcurrency_MoveRace(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	msSvc := services.NewMoveSwapService(repos, adapter)

	instID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	ctxA := buildContext(instID, userA, domain.RoleScheduler)
	ctxB := buildContext(instID, userB, domain.RoleScheduler)

	verID := uuid.New()
	snapID := uuid.New()
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}
	db.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusDraft,
		Version:       7,
	}
	db.assignments[verID] = []domain.ScheduleAssignment{
		{ID: uuid.New(), VersionID: verID, AssignmentID: "sr-1#0", RoomID: "r-orig", TimeSlotID: "ts-orig"},
	}

	moveA := domain.MoveDTO{AssignmentID: "sr-1#0", From: domain.PlacementDTO{RoomID: "r-orig", TimeSlotID: "ts-orig"}, To: domain.PlacementDTO{RoomID: "r-A", TimeSlotID: "ts-A"}}
	moveB := domain.MoveDTO{AssignmentID: "sr-1#0", From: domain.PlacementDTO{RoomID: "r-orig", TimeSlotID: "ts-orig"}, To: domain.PlacementDTO{RoomID: "r-B", TimeSlotID: "ts-B"}}

	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _, errA = msSvc.Move(ctxA, verID, moveA, 7, false)
	}()

	go func() {
		defer wg.Done()
		_, _, errB = msSvc.Move(ctxB, verID, moveB, 7, false)
	}()

	wg.Wait()

	successCount := 0
	conflictCount := 0

	if errA == nil {
		successCount++
	} else if errors.Is(errA, services.ErrConflict) {
		conflictCount++
	}
	if errB == nil {
		successCount++
	} else if errors.Is(errB, services.ErrConflict) {
		conflictCount++
	}

	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 conflict, got success: %d, conflict: %d", successCount, conflictCount)
	}

	finalVer := db.versions[verID]
	if finalVer.Version != 8 {
		t.Fatalf("expected version bumped to 8, got %d", finalVer.Version)
	}

	assignments := db.assignments[verID]
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
}

// Swap Race Test
func TestConcurrency_SwapRace(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	msSvc := services.NewMoveSwapService(repos, adapter)

	instID := uuid.New()
	ctxA := buildContext(instID, uuid.New(), domain.RoleScheduler)
	ctxB := buildContext(instID, uuid.New(), domain.RoleScheduler)

	verID := uuid.New()
	snapID := uuid.New()
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}
	db.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}

	swapA := domain.SwapDTO{Assignment1ID: "a1", Assignment2ID: "a2", Placement1: domain.PlacementDTO{RoomID: "r-A1", TimeSlotID: "ts-A1"}}
	swapB := domain.SwapDTO{Assignment1ID: "a1", Assignment2ID: "a2", Placement1: domain.PlacementDTO{RoomID: "r-B1", TimeSlotID: "ts-B1"}}

	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _, errA = msSvc.Swap(ctxA, verID, swapA, 1, false)
	}()
	go func() {
		defer wg.Done()
		_, _, errB = msSvc.Swap(ctxB, verID, swapB, 1, false)
	}()
	wg.Wait()

	successCount := 0
	conflictCount := 0
	if errA == nil {
		successCount++
	} else if errors.Is(errA, services.ErrConflict) {
		conflictCount++
	}
	if errB == nil {
		successCount++
	} else if errors.Is(errB, services.ErrConflict) {
		conflictCount++
	}

	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 conflict for swap race, got success: %d, conflict: %d", successCount, conflictCount)
	}
}

// Stale Worker Test
func TestConcurrency_StaleWorkerRejection(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)

	runID := uuid.New()
	workerA := "worker-A"
	workerB := "worker-B"

	db.runs[runID] = domain.ScheduleRun{
		ID:       runID,
		Status:   domain.StatusRunning,
		WorkerID: &workerB,
	}

	errA := repos.ScheduleRuns.CommitTerminalResultTx(
		context.Background(),
		runID,
		workerA,
		domain.StatusSolved,
		nil, nil, nil, nil, 100, nil, nil, nil, nil, nil,
		domain.AuditEvent{ID: uuid.New()},
	)

	if !errors.Is(errA, repositories.ErrStaleWorker) {
		t.Fatalf("expected ErrStaleWorker for revoked worker A, got: %v", errA)
	}

	errB := repos.ScheduleRuns.CommitTerminalResultTx(
		context.Background(),
		runID,
		workerB,
		domain.StatusSolved,
		nil, nil, nil, nil, 120, nil, nil, nil, nil, nil,
		domain.AuditEvent{ID: uuid.New()},
	)

	if errB != nil {
		t.Fatalf("expected worker B commit to succeed, got: %v", errB)
	}

	if db.runs[runID].Status != domain.StatusSolved {
		t.Fatalf("expected run status SOLVED, got %s", db.runs[runID].Status)
	}
}

// Transaction Failure Rollback Test
func TestTransaction_FailureRollsBackEverything(t *testing.T) {
	db := newTxMemoryDB()
	repos := buildTxTestRepos(db)
	adapter := &mockAdapter{}
	msSvc := services.NewMoveSwapService(repos, adapter)

	instID := uuid.New()
	ctx := buildContext(instID, uuid.New(), domain.RoleScheduler)

	verID := uuid.New()
	snapID := uuid.New()
	db.snapshots[snapID] = domain.ProblemSnapshot{ID: snapID, InstitutionID: instID, ProblemJSON: []byte(`{}`)}
	db.versions[verID] = domain.ScheduleVersion{
		ID:            verID,
		InstitutionID: instID,
		SnapshotID:    snapID,
		Status:        domain.VersionStatusDraft,
		Version:       1,
	}
	originalAssignments := []domain.ScheduleAssignment{
		{ID: uuid.New(), VersionID: verID, AssignmentID: "orig", RoomID: "r-orig", TimeSlotID: "ts-orig"},
	}
	db.assignments[verID] = originalAssignments

	db.failCASInjection = true

	move := domain.MoveDTO{AssignmentID: "orig", From: domain.PlacementDTO{RoomID: "r-orig", TimeSlotID: "ts-orig"}, To: domain.PlacementDTO{RoomID: "r-new", TimeSlotID: "ts-new"}}
	_, _, err := msSvc.Move(ctx, verID, move, 1, false)

	if err == nil {
		t.Fatalf("expected Move to fail under forced CAS error, got nil")
	}

	if db.versions[verID].Version != 1 {
		t.Fatalf("version was mutated on rollback, expected 1 got %d", db.versions[verID].Version)
	}
	if db.assignments[verID][0].RoomID != "r-orig" {
		t.Fatalf("assignments were partially committed on failure! room: %s", db.assignments[verID][0].RoomID)
	}
	if len(db.auditEvents) != 0 {
		t.Fatalf("audit event was committed despite transaction rollback, count: %d", len(db.auditEvents))
	}
}
