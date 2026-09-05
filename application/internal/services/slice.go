package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/asyncrun"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

// SliceService is the Phase 1 minimal end-to-end vertical slice.
//
// It performs the complete flow synchronously and writes a canonical,
// application-owned result plus an immutable engine-version-tagged
// engine snapshot to the database. It is the single path through which
// the application invokes Engine V1. The HTTP layer, repositories,
// database code, and frontend never import internal/scheduler directly.
type SliceService struct {
	repos   *repositories.Repos
	adapter curra.CurraAdapter
}

// NewSliceService wires a SliceService with the adapter that talks to
// Engine V1 and the repository container.
func NewSliceService(repos *repositories.Repos, adapter curra.CurraAdapter) *SliceService {
	return &SliceService{repos: repos, adapter: adapter}
}

// SolveRequest is the input the application hands to the vertical slice.
type SolveRequest struct {
	TimetableID      uuid.UUID
	SnapshotID       *uuid.UUID
	Seed             int64
	UseSeed          bool
	DeadlineSeconds  int
}

// SolveJobStatus is the application-owned lifecycle state for a solve job.
// A job is either still being processed, or has reached a terminal state.
type SolveJobStatus string

const (
	JobStatusQueued        SolveJobStatus = "QUEUED"
	JobStatusRunning       SolveJobStatus = "RUNNING"
	JobStatusSolved        SolveJobStatus = "SOLVED"
	JobStatusInfeasible    SolveJobStatus = "INFEASIBLE"
	JobStatusInvalidProblem SolveJobStatus = "INVALID_PROBLEM"
	JobStatusInvalidResult  SolveJobStatus = "INVALID_RESULT"
	JobStatusFailed        SolveJobStatus = "FAILED"
)

// SolveJob is the application view of a solve job exposed via the API.
// It is decoupled from Engine V1's SolveStatus so the API surface can
// evolve independently of the engine.
type SolveJob struct {
	RunID            uuid.UUID      `json:"runId"`
	SnapshotID       uuid.UUID      `json:"snapshotId"`
	TimetableID      uuid.UUID      `json:"timetableId"`
	InstitutionID    uuid.UUID      `json:"institutionId"`
	Status           SolveJobStatus `json:"status"`
	VerificationOK   bool           `json:"verificationOk"`
	VerificationStale bool          `json:"verificationStale"`
	StartedAt        time.Time      `json:"startedAt"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	Seed             int64          `json:"seed"`
	EngineVersion    string         `json:"engineVersion"`
	EngineCommit     string         `json:"engineCommit"`
}

// CreateSolveJob creates a durable queued job and returns the run ID.
// The job is processed asynchronously by the background worker.
func (s *SliceService) CreateSolveJob(ctx context.Context, req SolveRequest, createdBy uuid.UUID) (uuid.UUID, error) {
	if req.TimetableID == uuid.Nil {
		return uuid.Nil, errors.New("timetableID is required")
	}

	tt, err := s.repos.Timetables.GetByID(ctx, req.TimetableID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return uuid.Nil, err
	}

	var snap domain.ProblemSnapshot
	if req.SnapshotID != nil {
		snap, err = s.repos.Snapshots.GetByID(ctx, *req.SnapshotID)
		if err != nil {
			return uuid.Nil, err
		}
		if snap.TimetableID != req.TimetableID {
			return uuid.Nil, ErrNotFound
		}
	} else {
		snap, err = s.createSnapshot(ctx, req.TimetableID, createdBy)
		if err != nil {
			return uuid.Nil, err
		}
	}

	seed := req.Seed
	if !req.UseSeed {
		seed = 0
	}

	run := domain.ScheduleRun{
		ID:              uuid.New(),
		TimetableID:     req.TimetableID,
		InstitutionID:   tt.InstitutionID,
		SnapshotID:      snap.ID,
		Status:          domain.StatusQueued,
		SolverConfig:    snap.SolverConfig,
		ObjectiveConfig: snap.ObjectiveConfig,
		Seed:            &seed,
		CreatedBy:       createdBy,
		Version:         1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repos.ScheduleRuns.Create(ctx, run); err != nil {
		return uuid.Nil, fmt.Errorf("create schedule run: %w", err)
	}

	return run.ID, nil
}

// ExecuteQueuedRun processes a queued run through the complete vertical slice.
// It is called by the background worker and implements the full solve+verify+snapshot flow
// with deadline enforcement, panic isolation, and stale-worker protection.
func (s *SliceService) ExecuteQueuedRun(ctx context.Context, run domain.ScheduleRun, snap domain.ProblemSnapshot, commitHook func(domain.CanonicalResult, json.RawMessage) error) error {
	deadline := calculateDeadline(snap.SolverConfig, run.SolverConfig)

	deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(deadline))
	defer cancel()

	runCopy := run
	runCopy.Status = domain.StatusRunning

	solveHook := func(canonical domain.CanonicalResult, solutionJSON json.RawMessage) error {
		return commitHook(canonical, solutionJSON)
	}

	return asyncrun.Execute(deadlineCtx, runCopy, snap, s.repos, s.adapter, asyncrun.ComputeInputHash, solveHook)
}

func calculateDeadline(snapConfig, runConfig json.RawMessage) time.Duration {
	const defaultDeadline = 5 * time.Minute

	type solverCfg struct {
		DeadlineSeconds    int `json:"deadlineSeconds"`
		TimeoutSeconds     int `json:"timeoutSeconds"`
		MaxDurationSeconds int `json:"maxDurationSeconds"`
	}

	for _, cfg := range []json.RawMessage{snapConfig, runConfig} {
		if len(cfg) == 0 {
			continue
		}
		var c solverCfg
		if err := json.Unmarshal(cfg, &c); err != nil {
			continue
		}
		if c.DeadlineSeconds > 0 {
			return time.Duration(c.DeadlineSeconds) * time.Second
		}
		if c.TimeoutSeconds > 0 {
			return time.Duration(c.TimeoutSeconds) * time.Second
		}
		if c.MaxDurationSeconds > 0 {
			return time.Duration(c.MaxDurationSeconds) * time.Second
		}
	}

	return defaultDeadline
}

// CreateAndRunSolveJob is the synchronous Phase 1 compatibility path.
// It creates a QUEUED job then immediately executes it inline.
// For async execution, use CreateSolveJob + worker.
func (s *SliceService) CreateAndRunSolveJob(ctx context.Context, req SolveRequest, createdBy uuid.UUID) (domain.CanonicalResult, error) {
	if req.TimetableID == uuid.Nil {
		return domain.CanonicalResult{}, errors.New("timetableID is required")
	}

	tt, err := s.repos.Timetables.GetByID(ctx, req.TimetableID)
	if err != nil {
		return domain.CanonicalResult{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.CanonicalResult{}, err
	}

	var snap domain.ProblemSnapshot
	if req.SnapshotID != nil {
		snap, err = s.repos.Snapshots.GetByID(ctx, *req.SnapshotID)
		if err != nil {
			return domain.CanonicalResult{}, err
		}
		if snap.TimetableID != req.TimetableID {
			return domain.CanonicalResult{}, ErrNotFound
		}
	} else {
		snap, err = s.createSnapshot(ctx, req.TimetableID, createdBy)
		if err != nil {
			return domain.CanonicalResult{}, err
		}
	}

	seed := req.Seed
	if !req.UseSeed {
		seed = 0
	}

	run := domain.ScheduleRun{
		ID:              uuid.New(),
		TimetableID:     req.TimetableID,
		InstitutionID:   tt.InstitutionID,
		SnapshotID:      snap.ID,
		Status:          domain.StatusRunning,
		SolverConfig:    snap.SolverConfig,
		ObjectiveConfig: snap.ObjectiveConfig,
		Seed:            &seed,
		CreatedBy:       createdBy,
		Version:         1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repos.ScheduleRuns.Create(ctx, run); err != nil {
		return domain.CanonicalResult{}, fmt.Errorf("create schedule run: %w", err)
	}

	commitHook := func(canonical domain.CanonicalResult, solutionJSON json.RawMessage) error {
		run.Status = domain.ScheduleRunStatus(canonical.Status)
		now := time.Now()
		run.FinishedAt = &now
		run.StartedAt = &run.CreatedAt
		ruleSetHash := canonical.Metadata.RuleSetHash
		run.RuleSetHash = &ruleSetHash
		run.CurrAVersion = &canonical.Metadata.EngineVersion
		run.CurrACommit = &canonical.Metadata.EngineCommit
		run.DurationMs = ptrInt64(int64(now.Sub(run.CreatedAt) / time.Millisecond))
		run.Diagnostics = mustMarshal(canonical.Diagnostics)
		if len(solutionJSON) > 0 {
			run.Result = solutionJSON
		}
		return s.repos.ScheduleRuns.Update(ctx, run)
	}

	canonical, err := s.executeSolveAndVerify(ctx, run, snap, seed, commitHook)
	if err != nil {
		return domain.CanonicalResult{}, err
	}

	canonical.RunID = run.ID
	canonical.SnapshotID = snap.ID
	canonical.CreatedAt = time.Now()

	return canonical, nil
}

func (s *SliceService) executeSolveAndVerify(ctx context.Context, run domain.ScheduleRun, snap domain.ProblemSnapshot, seed int64, commitHook func(domain.CanonicalResult, json.RawMessage) error) (domain.CanonicalResult, error) {
	compileResp, compileErr := s.adapter.CompileConstraints(ctx, curra.CompileRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	if compileErr != nil {
		return s.recordInvalidProblem(ctx, run, snap, seed, compileErr)
	}
	ruleSetHash := compileResp.RuleSetHash

	solveReq := curra.SolveRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
		Seed:            seed,
	}
	solveResp, solveErr := s.adapter.Solve(ctx, solveReq)
	if solveErr != nil && solveResp.Status == "" {
		return s.recordFailed(ctx, run, snap, seed, ruleSetHash, solveErr)
	}

	canonical := mapToCanonical(run, snap, solveResp, ruleSetHash, seed)

	verified, _ := s.runIndependentVerification(ctx, snap, solveResp, run.ID, ruleSetHash, seed)
	canonical.VerifierOK = verified
	canonical.Verified = verified && canonical.Status == "SOLVED"

	if !verified && canonical.Status == "SOLVED" {
		canonical.Status = string(domain.StatusInvalidResult)
	}

	if canonical.Status == "SOLVED" {
		engineSnap, err := buildEngineSnapshot(
			run.ID, snap.ID, run.InstitutionID, seed,
			ruleSetHash, curra.CurrAVersion,
			solveReq, solveResp, canonical.Diagnostics,
		)
		if err != nil {
			return domain.CanonicalResult{}, fmt.Errorf("build engine snapshot: %w", err)
		}
		engineSnap.InputHash = canonical.Metadata.InputHash
		if err := s.repos.EngineSnapshots.Create(ctx, engineSnap); err != nil {
			return domain.CanonicalResult{}, fmt.Errorf("persist engine snapshot: %w", err)
		}
	}

	if commitHook != nil {
		if err := commitHook(canonical, solveResp.Solution); err != nil {
			return domain.CanonicalResult{}, fmt.Errorf("commit hook: %w", err)
		}
	}

	return canonical, nil
}

func (s *SliceService) runIndependentVerification(ctx context.Context, snap domain.ProblemSnapshot, solveResp curra.SolveResponse, runID uuid.UUID, ruleSetHash string, seed int64) (bool, error) {
	if solveResp.Status != "SOLVED" || len(solveResp.Solution) == 0 {
		return false, nil
	}
	v, err := s.adapter.Verify(ctx, curra.VerifyRequest{
		ProblemJSON:     snap.ProblemJSON,
		SolutionJSON:    solveResp.Solution,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	if err != nil {
		return false, err
	}
	return v.Valid, nil
}

func (s *SliceService) recordInvalidProblem(ctx context.Context, run domain.ScheduleRun, snap domain.ProblemSnapshot, seed int64, compileErr error) (domain.CanonicalResult, error) {
	return domain.CanonicalResult{
		Status:     string(domain.StatusInvalidProblem),
		SnapshotID: snap.ID,
		Diagnostics: domain.CanonicalDiagnostics{
			Message: compileErr.Error(),
		},
		Metadata: domain.ResultMetadata{
			InputHash:   ComputeInputHash(snap, nil),
			RuleSetHash: "",
			Seed:        seed,
		},
	}, nil
}

func (s *SliceService) recordFailed(ctx context.Context, run domain.ScheduleRun, snap domain.ProblemSnapshot, seed int64, ruleSetHash string, solveErr error) (domain.CanonicalResult, error) {
	return domain.CanonicalResult{
		Status:     string(domain.StatusFailed),
		SnapshotID: snap.ID,
		Diagnostics: domain.CanonicalDiagnostics{
			Message: solveErr.Error(),
		},
		Metadata: domain.ResultMetadata{
			InputHash:   ComputeInputHash(snap, nil),
			RuleSetHash: ruleSetHash,
			Seed:        seed,
		},
	}, nil
}

// createSnapshot pins the live academic catalog for this timetable as an
// immutable problem revision.
func (s *SliceService) createSnapshot(ctx context.Context, timetableID uuid.UUID, createdBy uuid.UUID) (domain.ProblemSnapshot, error) {
	snapshots, err := s.repos.Snapshots.ListByTimetable(ctx, timetableID)
	if err != nil {
		return domain.ProblemSnapshot{}, err
	}
	if len(snapshots) > 0 {
		return snapshots[0], nil
	}
	return domain.ProblemSnapshot{}, errors.New("no snapshot exists for timetable; create a snapshot first")
}

func ptrInt64(v int64) *int64 { return &v }

// mustMarshal is a small helper that panics only on programmer error
// (i.e. trying to marshal a value whose type is unmarshalable). The
// adapter DTOs are all JSON-safe, so a panic indicates a bug, not a
// runtime condition.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshal: %v", err))
	}
	return b
}

// GetResult returns the application-owned canonical result for a given
// run, by reading the immutable engine snapshot and converting the
// engine output back into a CanonicalResult.
func (s *SliceService) GetResult(ctx context.Context, runID uuid.UUID) (domain.CanonicalResult, error) {
	run, err := s.repos.ScheduleRuns.GetByID(ctx, runID)
	if err != nil {
		return domain.CanonicalResult{}, err
	}
	if err := RequireTenantMatch(ctx, run.InstitutionID); err != nil {
		return domain.CanonicalResult{}, err
	}
	snap, err := s.repos.Snapshots.GetByID(ctx, run.SnapshotID)
	if err != nil {
		return domain.CanonicalResult{}, err
	}
	seed := int64(0)
	if run.Seed != nil {
		seed = *run.Seed
	}
	ruleSetHash := ""
	if run.RuleSetHash != nil {
		ruleSetHash = *run.RuleSetHash
	}
	engineVer := ""
	if run.CurrAVersion != nil {
		engineVer = *run.CurrAVersion
	}
	engineCommit := ""
	if run.CurrACommit != nil {
		engineCommit = *run.CurrACommit
	}
	status := string(run.Status)
	verified := run.Status == domain.StatusSolved
	assignments := parseAssignments(run.Result)

	return domain.CanonicalResult{
		RunID:         run.ID,
		SnapshotID:    snap.ID,
		Status:        status,
		Verified:      verified,
		VerifierOK:    verified,
		HardViolations: 0,
		SoftPenalty:    0,
		Assignments:   assignments,
		Metadata: domain.ResultMetadata{
			EngineVersion:  engineVer,
			EngineCommit:   engineCommit,
			AdapterVersion: curra.CurrAVersion,
			RuleSetHash:    ruleSetHash,
			InputHash:      ComputeInputHash(snap, nil),
			Seed:           seed,
		},
		CreatedAt: run.UpdatedAt,
	}, nil
}

// GetJob returns a public SolveJob view for a run row. It does not
// deserialize any Engine V1 types.
func (s *SliceService) GetJob(ctx context.Context, runID uuid.UUID) (SolveJob, error) {
	run, err := s.repos.ScheduleRuns.GetByID(ctx, runID)
	if err != nil {
		return SolveJob{}, err
	}
	if err := RequireTenantMatch(ctx, run.InstitutionID); err != nil {
		return SolveJob{}, err
	}
	seed := int64(0)
	if run.Seed != nil {
		seed = *run.Seed
	}
	engineVer := ""
	if run.CurrAVersion != nil {
		engineVer = *run.CurrAVersion
	}
	engineCommit := ""
	if run.CurrACommit != nil {
		engineCommit = *run.CurrACommit
	}
	return SolveJob{
		RunID:           run.ID,
		SnapshotID:      run.SnapshotID,
		TimetableID:     run.TimetableID,
		InstitutionID:   run.InstitutionID,
		Status:          SolveJobStatus(run.Status),
		VerificationOK:  run.Status == domain.StatusSolved,
		StartedAt:       run.CreatedAt,
		CompletedAt:     run.FinishedAt,
		Seed:            seed,
		EngineVersion:   engineVer,
		EngineCommit:    engineCommit,
	}, nil
}

// CancelSolveJob requests cancellation of a queued or running solve job.
func (s *SliceService) CancelSolveJob(ctx context.Context, runID uuid.UUID) error {
	run, err := s.repos.ScheduleRuns.GetByID(ctx, runID)
	if err != nil {
		return err
	}
	if err := RequireTenantMatch(ctx, run.InstitutionID); err != nil {
		return err
	}
	if run.Status != domain.StatusQueued && run.Status != domain.StatusRunning {
		return fmt.Errorf("cannot cancel job in status %s", run.Status)
	}
	cancelled, err := s.repos.ScheduleRuns.Cancel(ctx, runID)
	if err != nil {
		return err
	}
	if !cancelled {
		return errors.New("cancellation not applied")
	}
	return nil
}
