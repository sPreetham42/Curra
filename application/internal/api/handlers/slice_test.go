package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
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

// sliceTestRepos is a minimal in-memory store sufficient to exercise the
// slice HTTP path. It implements every interface the slice service uses.
type sliceHTTPMem struct {
	mu           sync.Mutex
	timetables   map[uuid.UUID]domain.Timetable
	snapshots    map[uuid.UUID]domain.ProblemSnapshot
	runs         map[uuid.UUID]domain.ScheduleRun
	versions     map[uuid.UUID]domain.ScheduleVersion
	assignments  map[uuid.UUID][]domain.ScheduleAssignment
	idempotency  map[string]domain.IdempotencyKey
	engineSnaps  map[uuid.UUID]domain.EngineSnapshot
}

func newSliceHTTPMem() *sliceHTTPMem {
	return &sliceHTTPMem{
		timetables:  map[uuid.UUID]domain.Timetable{},
		snapshots:   map[uuid.UUID]domain.ProblemSnapshot{},
		runs:        map[uuid.UUID]domain.ScheduleRun{},
		versions:    map[uuid.UUID]domain.ScheduleVersion{},
		assignments: map[uuid.UUID][]domain.ScheduleAssignment{},
		idempotency: map[string]domain.IdempotencyKey{},
		engineSnaps: map[uuid.UUID]domain.EngineSnapshot{},
	}
}

// timetable
type hTbl struct{ m *sliceHTTPMem }
func (r *hTbl) Create(ctx context.Context, tt domain.Timetable) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock(); r.m.timetables[tt.ID] = tt; return nil
}
func (r *hTbl) GetByID(ctx context.Context, id uuid.UUID) (domain.Timetable, error) {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	tt, ok := r.m.timetables[id]
	if !ok {
		return domain.Timetable{}, repositories.ErrNotFound
	}
	return tt, nil
}
func (r *hTbl) ListByInstitution(ctx context.Context, instID uuid.UUID) ([]domain.Timetable, error) {
	return nil, nil
}
func (r *hTbl) Update(ctx context.Context, tt domain.Timetable) error { return nil }
func (r *hTbl) SetCurrentPublishedVersion(ctx context.Context, _, _ uuid.UUID) error { return nil }

// snapshot
type hSnap struct{ m *sliceHTTPMem }
func (r *hSnap) Create(ctx context.Context, snap domain.ProblemSnapshot) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock(); r.m.snapshots[snap.ID] = snap; return nil
}
func (r *hSnap) GetByID(ctx context.Context, id uuid.UUID) (domain.ProblemSnapshot, error) {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	s, ok := r.m.snapshots[id]
	if !ok {
		return domain.ProblemSnapshot{}, repositories.ErrNotFound
	}
	return s, nil
}
func (r *hSnap) ListByTimetable(ctx context.Context, ttID uuid.UUID) ([]domain.ProblemSnapshot, error) {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	var out []domain.ProblemSnapshot
	for _, s := range r.m.snapshots {
		if s.TimetableID == ttID {
			out = append(out, s)
		}
	}
	return out, nil
}

// run
type hRun struct{ m *sliceHTTPMem }
func (r *hRun) Create(ctx context.Context, run domain.ScheduleRun) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock(); r.m.runs[run.ID] = run; return nil
}
func (r *hRun) GetByID(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	run, ok := r.m.runs[id]
	if !ok {
		return domain.ScheduleRun{}, repositories.ErrNotFound
	}
	return run, nil
}
func (r *hRun) ListByTimetable(ctx context.Context, _ uuid.UUID) ([]domain.ScheduleRun, error) {
	return nil, nil
}
func (r *hRun) Update(ctx context.Context, run domain.ScheduleRun) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock(); r.m.runs[run.ID] = run; return nil
}
func (r *hRun) ClaimQueued(ctx context.Context, _ string, _ time.Duration) (*domain.ScheduleRun, bool, error) {
	return nil, false, nil
}
func (r *hRun) UpdateTerminalResult(ctx context.Context, _ uuid.UUID, _ string, _ domain.ScheduleRunStatus, _, _, _, _ json.RawMessage, _ int64, _, _, _ *string) error {
	return nil
}
func (r *hRun) CommitTerminalResultTx(ctx context.Context, _ uuid.UUID, _ string, _ domain.ScheduleRunStatus, _, _, _, _ json.RawMessage, _ int64, _, _, _ *string, _ *domain.ScheduleVersion, _ []domain.ScheduleAssignment, _ domain.AuditEvent) error {
	return nil
}
func (r *hRun) UpdateHeartbeat(ctx context.Context, _ uuid.UUID, _ string) error { return nil }
func (r *hRun) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScheduleRunStatus, _ map[string]any) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	run, ok := r.m.runs[id]
	if !ok {
		return repositories.ErrNotFound
	}
	run.Status = status
	r.m.runs[id] = run
	return nil
}
func (r *hRun) Cancel(ctx context.Context, _ uuid.UUID) (bool, error) { return false, nil }
func (r *hRun) RecoverExpired(ctx context.Context, _ int) (int, error) { return 0, nil }

// version
type hVer struct{ m *sliceHTTPMem }
func (r *hVer) Create(ctx context.Context, ver domain.ScheduleVersion) error { return nil }
func (r *hVer) GetByID(ctx context.Context, _ uuid.UUID) (domain.ScheduleVersion, error) { return domain.ScheduleVersion{}, repositories.ErrNotFound }
func (r *hVer) ListByTimetable(ctx context.Context, _ uuid.UUID) ([]domain.ScheduleVersion, error) { return nil, nil }
func (r *hVer) UpdateStatus(ctx context.Context, _ uuid.UUID, _ domain.ScheduleVersionStatus, _ int) error { return nil }
func (r *hVer) Update(ctx context.Context, _ domain.ScheduleVersion) error { return nil }
func (r *hVer) ApplyAssignmentUpdateTx(ctx context.Context, _ uuid.UUID, _ int, _ json.RawMessage, _ []domain.ScheduleAssignment, _ domain.AuditEvent) error { return nil }
func (r *hVer) PublishTx(ctx context.Context, _ uuid.UUID, _ int, _ uuid.UUID, _ domain.AuditEvent) error { return nil }

// assignment
type hAsn struct{ m *sliceHTTPMem }
func (r *hAsn) Create(ctx context.Context, a domain.ScheduleAssignment) error { return nil }
func (r *hAsn) CreateBatch(ctx context.Context, _ []domain.ScheduleAssignment) error { return nil }
func (r *hAsn) ReplaceAllForVersion(ctx context.Context, _ uuid.UUID, _ []domain.ScheduleAssignment) error { return nil }
func (r *hAsn) ListByVersion(ctx context.Context, _ uuid.UUID) ([]domain.ScheduleAssignment, error) { return nil, nil }
func (r *hAsn) DeleteByVersion(ctx context.Context, _ uuid.UUID) error { return nil }

// audit
type hAud struct{}
func (r *hAud) Create(ctx context.Context, _ domain.AuditEvent) error { return nil }
func (r *hAud) ListByInstitution(ctx context.Context, _ uuid.UUID, _ int) ([]domain.AuditEvent, error) { return nil, nil }

// engine snapshot
type hESnap struct{ m *sliceHTTPMem }
func (r *hESnap) Create(ctx context.Context, snap domain.EngineSnapshot) error {
	r.m.mu.Lock(); defer r.m.mu.Unlock(); r.m.engineSnaps[snap.ScheduleRunID] = snap; return nil
}
func (r *hESnap) GetByRunID(ctx context.Context, runID uuid.UUID) (domain.EngineSnapshot, error) {
	r.m.mu.Lock(); defer r.m.mu.Unlock()
	s, ok := r.m.engineSnaps[runID]
	if !ok {
		return domain.EngineSnapshot{}, repositories.ErrNotFound
	}
	return s, nil
}

func (m *sliceHTTPMem) buildRepos() *repositories.Repos {
	return &repositories.Repos{
		Timetables:          &hTbl{m: m},
		Snapshots:           &hSnap{m: m},
		ScheduleRuns:        &hRun{m: m},
		ScheduleVersions:    &hVer{m: m},
		ScheduleAssignments: &hAsn{m: m},
		AuditEvents:         &hAud{},
		EngineSnapshots:     &hESnap{m: m},
	}
}

func setupSliceRouter(t *testing.T) (http.Handler, *sliceHTTPMem, uuid.UUID, uuid.UUID) {
	t.Helper()
	mem := newSliceHTTPMem()
	repos := mem.buildRepos()
	instID := uuid.New()
	ttID := uuid.New()
	now := time.Now()
	mem.timetables[ttID] = domain.Timetable{
		ID:            ttID,
		InstitutionID: instID,
		Name:          "Test",
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	problemData := map[string]any{
		"TenantID":      instID.String(),
		"PeriodsPerDay": 2,
		"Term":          map[string]any{"ID": "term-1", "TenantID": instID.String(), "Name": "Term 1"},
		"Departments":   map[string]any{"dept": map[string]any{"ID": "dept", "TenantID": instID.String(), "Name": "CS"}},
		"Programs":      map[string]any{"prog": map[string]any{"ID": "prog", "DepartmentID": "dept", "Name": "CS"}},
		"Classes": map[string]any{
			"class": map[string]any{"ID": "class", "ProgramID": "prog", "Name": "A", "WholeGroupID": "sg", "StudentGroupIDs": []string{"sg"}},
		},
		"StudentGroups": map[string]any{"sg": map[string]any{"ID": "sg", "ClassID": "class", "Name": "Whole", "Size": 30}},
		"Subjects":      map[string]any{"subj": map[string]any{"ID": "subj", "Code": "CS101", "Name": "CS101"}},
		"CourseOfferings": map[string]any{
			"co": map[string]any{
				"ID": "co", "TermID": "term-1", "ClassID": "class", "SubjectID": "subj", "StudentGroupID": "sg",
				"FacultyID": "fac", "RequiredRoomFeatureIDs": []string{}, "SessionRequirementIDs": []string{"req"},
			},
		},
		"SessionRequirements": map[string]any{
			"req": map[string]any{"ID": "req", "CourseOfferingID": "co", "Type": "THEORY", "SessionsPerWeek": 2, "Duration": 1, "Consecutive": true, "RequiredRoomFeatureIDs": []string{}},
		},
		"Faculty":   map[string]any{"fac": map[string]any{"ID": "fac", "TenantID": instID.String(), "Name": "F"}},
		"Rooms":     map[string]any{"room": map[string]any{"ID": "room", "TenantID": instID.String(), "Name": "R", "Capacity": 50, "FeatureIDs": []string{}}},
		"RoomFeatures": map[string]any{},
		"TimeSlots": map[string]any{
			"t1": map[string]any{"ID": "t1", "Day": 1, "Period": 1, "Label": "Mon P1"},
			"t2": map[string]any{"ID": "t2", "Day": 1, "Period": 2, "Label": "Mon P2"},
			"t3": map[string]any{"ID": "t3", "Day": 2, "Period": 1, "Label": "Tue P1"},
		},
		"FacultyAvailabilities": []any{
			map[string]any{"FacultyID": "fac", "TimeSlotID": "t1"},
			map[string]any{"FacultyID": "fac", "TimeSlotID": "t2"},
			map[string]any{"FacultyID": "fac", "TimeSlotID": "t3"},
		},
		"RoomAvailabilities": []any{
			map[string]any{"RoomID": "room", "TimeSlotID": "t1"},
			map[string]any{"RoomID": "room", "TimeSlotID": "t2"},
			map[string]any{"RoomID": "room", "TimeSlotID": "t3"},
		},
		"FacultyPreferences": []any{},
		"LockedAssignments":  []any{},
	}
	problemJSON, _ := json.Marshal(problemData)
	h := sha256.New()
	h.Write(problemJSON)
	h.Write([]byte(`[]`))
	h.Write([]byte(`{}`))
	h.Write([]byte(`{}`))
	inputHash := hex.EncodeToString(h.Sum(nil))

	snap := domain.ProblemSnapshot{
		ID:                  uuid.New(),
		TimetableID:         ttID,
		InstitutionID:       instID,
		SchemaVersion:       1,
		ProblemJSON:         problemJSON,
		ConstraintInstances: []byte(`[]`),
		SolverConfig:        []byte(`{"searchMode":"HEURISTIC_LCV","maxNodes":100000}`),
		ObjectiveConfig:     []byte(`{"components":[{"id":"StudentGapPenalty","weight":1}]}`),
		InputHash:           inputHash,
		CreatedBy:           uuid.New(),
		CreatedAt:           now,
	}
	mem.snapshots[snap.ID] = snap

	adapter := curra.New(testLogger(t))
	ttSvc := services.NewTimetableService(repos)
	snapSvc := services.NewSnapshotService(repos)
	runSvc := services.NewRunService(repos, adapter)
	verSvc := services.NewVersionService(repos)
	pubSvc := services.NewPublishingService(repos, adapter)
	msSvc := services.NewMoveSwapService(repos, adapter)
	vSvc := services.NewVerificationService(repos, adapter)
	catSvc := services.NewCatalogService(repos)
	sliceSvc := services.NewSliceService(repos, adapter)

	h2 := handlers.New(ttSvc, snapSvc, runSvc, verSvc, pubSvc, msSvc, vSvc, catSvc, sliceSvc)
	auth := middleware.NewAuthMiddleware()
	return api.NewRouter(h2, auth), mem, instID, ttID
}

func authedRequest(t *testing.T, method, path string, body []byte, instID, userID uuid.UUID) *http.Request {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Institution-ID", instID.String())
	req.Header.Set("X-Role", "INSTITUTION_ADMIN")
	return req
}

func TestHTTP_SolveJob_EndToEnd_WithRealEngineV1(t *testing.T) {
	router, mem, instID, ttID := setupSliceRouter(t)
	userID := uuid.New()

	snap := getFirstSnapshot(mem)

	body, _ := json.Marshal(map[string]any{
		"timetableId": ttID,
		"snapshotId":  snap.ID,
		"seed":        1234,
		"useSeed":     true,
	})
	req := authedRequest(t, http.MethodPost, "/api/v1/solve-jobs", body, instID, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			RunID  string          `json:"runId"`
			Result json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	resp := envelope.Data
	if resp.RunID == "" {
		t.Fatalf("expected runId in response; body=%s", rec.Body.String())
	}
	if len(resp.Result) > 0 {
		t.Fatalf("async API must not return synchronous result; body=%s", rec.Body.String())
	}

	runID, _ := uuid.Parse(resp.RunID)

	run, ok := mem.runs[runID]
	if !ok {
		t.Fatalf("run not created")
	}
	if run.Status != domain.StatusQueued {
		t.Errorf("expected job status QUEUED immediately after POST, got %s", run.Status)
	}

	// Simulate the worker picking up the job
	workerAdapter := curra.New(testLogger(t))
	workerSlice := services.NewSliceService(mem.buildRepos(), workerAdapter)
	commitHook := func(canonical domain.CanonicalResult, solutionJSON json.RawMessage) error {
		now := time.Now()
		run.Status = domain.ScheduleRunStatus(canonical.Status)
		run.FinishedAt = &now
		run.StartedAt = &run.CreatedAt
		ruleSetHash := canonical.Metadata.RuleSetHash
		run.RuleSetHash = &ruleSetHash
		run.CurrAVersion = &canonical.Metadata.EngineVersion
		run.CurrACommit = &canonical.Metadata.EngineCommit
		run.Result = solutionJSON
		run.Diagnostics = json.RawMessage(`{}`)
		mem.runs[run.ID] = run
		return nil
	}
	if err := workerSlice.ExecuteQueuedRun(context.Background(), run, snap, commitHook); err != nil {
		t.Fatalf("execute queued run: %v", err)
	}

	// GET /solve-jobs/{id}
	getReq := authedRequest(t, http.MethodGet, "/api/v1/solve-jobs/"+runID.String(), nil, instID, userID)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", getRec.Code, getRec.Body.String())
	}

	// GET /solve-jobs/{id}/result
	resReq := authedRequest(t, http.MethodGet, "/api/v1/solve-jobs/"+runID.String()+"/result", nil, instID, userID)
	resRec := httptest.NewRecorder()
	router.ServeHTTP(resRec, resReq)
	if resRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /result, got %d, body=%s", resRec.Code, resRec.Body.String())
	}
	var resultEnvelope struct {
		Data struct {
			Status      string `json:"status"`
			Verified    bool   `json:"verified"`
			VerifierOK  bool   `json:"verifierOk"`
			Assignments []struct {
				AssignmentID         string `json:"assignmentId"`
				SessionRequirementID string `json:"sessionRequirementId"`
			} `json:"assignments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resRec.Body.Bytes(), &resultEnvelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	result := resultEnvelope.Data
	if result.Status != "SOLVED" {
		t.Fatalf("expected status SOLVED, got %s", result.Status)
	}
	if !result.Verified {
		t.Fatal("expected verified=true")
	}
	if !result.VerifierOK {
		t.Fatal("expected verifierOk=true")
	}
	if len(result.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(result.Assignments))
	}
}

func TestHTTP_SolveJob_Deterministic(t *testing.T) {
	routerA, memA, instIDA, ttIDA := setupSliceRouter(t)
	userID := uuid.New()

	snapA := getFirstSnapshot(memA)

	// Create a second run on the SAME snapshot+tenant to compare deterministic
	// behavior across two independent solve invocations.
	bodyA1, _ := json.Marshal(map[string]any{
		"timetableId": ttIDA,
		"snapshotId":  snapA.ID,
		"seed":        9999,
		"useSeed":     true,
	})
	reqA1 := authedRequest(t, http.MethodPost, "/api/v1/solve-jobs", bodyA1, instIDA, userID)
	recA1 := httptest.NewRecorder()
	routerA.ServeHTTP(recA1, reqA1)
	if recA1.Code != http.StatusAccepted {
		t.Fatalf("A1: %d %s", recA1.Code, recA1.Body.String())
	}

	bodyA2, _ := json.Marshal(map[string]any{
		"timetableId": ttIDA,
		"snapshotId":  snapA.ID,
		"seed":        9999,
		"useSeed":     true,
	})
	reqA2 := authedRequest(t, http.MethodPost, "/api/v1/solve-jobs", bodyA2, instIDA, userID)
	recA2 := httptest.NewRecorder()
	routerA.ServeHTTP(recA2, reqA2)
	if recA2.Code != http.StatusAccepted {
		t.Fatalf("A2: %d %s", recA2.Code, recA2.Body.String())
	}

	var envA1, envA2 struct {
		Data struct {
			RunID string `json:"runId"`
		} `json:"data"`
	}
	_ = json.Unmarshal(recA1.Body.Bytes(), &envA1)
	_ = json.Unmarshal(recA2.Body.Bytes(), &envA2)
	runID1, _ := uuid.Parse(envA1.Data.RunID)
	runID2, _ := uuid.Parse(envA2.Data.RunID)

	run1 := memA.runs[runID1]
	run2 := memA.runs[runID2]

	processJob := func(run domain.ScheduleRun) domain.CanonicalResult {
		workerAdapter := curra.New(testLogger(t))
		workerSlice := services.NewSliceService(memA.buildRepos(), workerAdapter)
		var result domain.CanonicalResult
		commitHook := func(canonical domain.CanonicalResult, solutionJSON json.RawMessage) error {
			now := time.Now()
			run.Status = domain.ScheduleRunStatus(canonical.Status)
			run.FinishedAt = &now
			run.StartedAt = &run.CreatedAt
			ruleSetHash := canonical.Metadata.RuleSetHash
			run.RuleSetHash = &ruleSetHash
			run.CurrAVersion = &canonical.Metadata.EngineVersion
			run.CurrACommit = &canonical.Metadata.EngineCommit
			run.Result = solutionJSON
			run.Diagnostics = json.RawMessage(`{}`)
			memA.runs[run.ID] = run
			result = canonical
			return nil
		}
		if err := workerSlice.ExecuteQueuedRun(context.Background(), run, snapA, commitHook); err != nil {
			t.Fatalf("execute: %v", err)
		}
		return result
	}

	result1 := processJob(run1)
	result2 := processJob(run2)

	if result1.Metadata.InputHash != result2.Metadata.InputHash {
		t.Errorf("input hash differs across deterministic runs: %s vs %s",
			result1.Metadata.InputHash, result2.Metadata.InputHash)
	}
	if result1.Metadata.RuleSetHash != result2.Metadata.RuleSetHash {
		t.Errorf("rule set hash differs across deterministic runs")
	}
}

func getFirstSnapshot(m *sliceHTTPMem) domain.ProblemSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.snapshots {
		return s
	}
	return domain.ProblemSnapshot{}
}

func testLogger(_ *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
