package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type Worker struct {
	id          string
	repos       *repositories.Repos
	adapter     curra.CurraAdapter
	logger      *slog.Logger
	maxRetries  int
	pollPeriod  time.Duration
	reapPeriod  time.Duration
}

func New(id string, repos *repositories.Repos, adapter curra.CurraAdapter, logger *slog.Logger) *Worker {
	if id == "" {
		id = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}
	return &Worker{
		id:         id,
		repos:      repos,
		adapter:    adapter,
		logger:     logger,
		maxRetries: 3,
		pollPeriod: 1 * time.Second,
		reapPeriod: 30 * time.Second,
	}
}

func (w *Worker) ID() string {
	return w.id
}

func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("starting solver worker", "worker_id", w.id)

	pollTicker := time.NewTicker(w.pollPeriod)
	defer pollTicker.Stop()

	reapTicker := time.NewTicker(w.reapPeriod)
	defer reapTicker.Stop()

	// Initial poll immediately upon start
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping solver worker", "worker_id", w.id)
			return nil

		case <-reapTicker.C:
			reaped, err := w.repos.ScheduleRuns.RecoverExpired(ctx, w.maxRetries)
			if err != nil {
				w.logger.Error("failed to recover expired leases", "error", err)
			} else if reaped > 0 {
				w.logger.Info("reaped expired runs", "count", reaped)
			}

		case <-pollTicker.C:
			w.poll(ctx)
		}
	}
}

// calculateLeaseDuration computes lease = maximum solver execution window + fixed safety margin.
func calculateLeaseDuration(solverConfigJSON json.RawMessage) time.Duration {
	const defaultSafetyMargin = 2 * time.Minute
	const fallbackTimeout = 5 * time.Minute

	var cfg struct {
		TimeoutSeconds     int `json:"timeoutSeconds"`
		MaxDurationSeconds int `json:"maxDurationSeconds"`
		MaxNodes           int `json:"maxNodes"`
	}

	if len(solverConfigJSON) > 0 {
		_ = json.Unmarshal(solverConfigJSON, &cfg)
	}

	timeout := fallbackTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	} else if cfg.MaxDurationSeconds > 0 {
		timeout = time.Duration(cfg.MaxDurationSeconds) * time.Second
	} else if cfg.MaxNodes > 0 {
		// Reasonable heuristic: 1000 nodes ~ 1 second
		estimatedSec := cfg.MaxNodes / 1000
		if estimatedSec > 300 {
			timeout = time.Duration(estimatedSec) * time.Second
		}
	}

	return timeout + defaultSafetyMargin
}

func (w *Worker) poll(ctx context.Context) {
	// 1. Initial default claim lease (will be extended if configured for longer)
	leaseDuration := 5 * time.Minute
	run, claimed, err := w.repos.ScheduleRuns.ClaimQueued(ctx, w.id, leaseDuration)
	if err != nil {
		w.logger.Error("failed to claim run", "error", err)
		return
	}
	if !claimed || run == nil {
		return
	}

	dynamicLease := calculateLeaseDuration(run.SolverConfig)
	w.logger.Info("claimed run", "run_id", run.ID, "worker_id", w.id, "lease_duration", dynamicLease)

	// Heartbeat context
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go w.heartbeat(heartbeatCtx, run.ID)

	// Execute solve
	w.execute(ctx, run)
}

func (w *Worker) heartbeat(ctx context.Context, runID uuid.UUID) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.repos.ScheduleRuns.UpdateHeartbeat(ctx, runID, w.id); err != nil {
				w.logger.Error("heartbeat failed", "run_id", runID, "worker_id", w.id, "error", err)
			}
		}
	}
}

func (w *Worker) execute(ctx context.Context, run *domain.ScheduleRun) {
	start := time.Now()

	// 1. Load immutable snapshot
	snap, err := w.repos.Snapshots.GetByID(ctx, run.SnapshotID)
	if err != nil {
		w.logger.Error("failed to load snapshot for run", "run_id", run.ID, "error", err)
		w.failRun(ctx, run, fmt.Sprintf("failed to load snapshot: %v", err))
		return
	}

	// 2. Parse solver config from snapshot/run
	var seed int64
	if run.Seed != nil {
		seed = *run.Seed
	} else {
		seed = time.Now().UnixNano()
	}

	var solverConfig struct {
		SearchMode string `json:"searchMode"`
		MaxNodes   int    `json:"maxNodes"`
	}
	if len(run.SolverConfig) > 0 {
		_ = json.Unmarshal(run.SolverConfig, &solverConfig)
	} else {
		_ = json.Unmarshal(snap.SolverConfig, &solverConfig)
	}

	solveReq := curra.SolveRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
		Seed:            seed,
		MaxNodes:        solverConfig.MaxNodes,
		SearchMode:      solverConfig.SearchMode,
	}

	// 3. Execute CURRA solver strictly OUTSIDE database transaction
	w.logger.Info("executing curra solver outside db transaction", "run_id", run.ID, "seed", seed)
	solveResp, err := w.adapter.Solve(ctx, solveReq)
	durationMs := int64(time.Since(start).Milliseconds())

	if err != nil && solveResp.Status == "" {
		w.logger.Error("curra solver execution error", "run_id", run.ID, "error", err)
		w.failRun(ctx, run, err.Error())
		return
	}

	// 4. Prepare terminal payload
	scoreJSON, _ := json.Marshal(solveResp.Score)
	diagJSON, _ := json.Marshal(solveResp.Diagnostics)
	var violationsJSON json.RawMessage
	if len(solveResp.Violations) > 0 {
		violationsJSON, _ = json.Marshal(solveResp.Violations)
	}

	curraVer := curra.CurrAVersion
	curraCommit := curra.CurrACommit
	ruleSetHash := solveResp.RuleSetHash

	terminalStatus := domain.ScheduleRunStatus(solveResp.Status)

	var draftVer *domain.ScheduleVersion
	var newAssignments []domain.ScheduleAssignment

	if terminalStatus == domain.StatusSolved && len(solveResp.Solution) > 0 {
		v := domain.ScheduleVersion{
			ID:            uuid.New(),
			TimetableID:   run.TimetableID,
			InstitutionID: run.InstitutionID,
			SourceRunID:   &run.ID,
			SnapshotID:    run.SnapshotID,
			Status:        domain.VersionStatusDraft,
			Name:          fmt.Sprintf("Auto-generated from run %s", run.ID.String()[:8]),
			Score:         scoreJSON,
			Version:       1,
			CreatedBy:     run.CreatedBy,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		draftVer = &v
		assignments, parseErr := parseAssignmentsFromSolution(v.ID, solveResp.Solution)
		if parseErr == nil && len(assignments) > 0 {
			newAssignments = assignments
		}
	}

	audit := domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: run.InstitutionID,
		UserID:        &run.CreatedBy,
		Action:        "schedule_run.complete",
		ResourceType:  "schedule_run",
		ResourceID:    run.ID,
		Details:       json.RawMessage(fmt.Sprintf(`{"status":"%s","durationMs":%d}`, terminalStatus, durationMs)),
		CreatedAt:     time.Now(),
	}

	// 5. Commit all terminal result writes in one atomic database transaction
	err = w.repos.ScheduleRuns.CommitTerminalResultTx(
		ctx,
		run.ID,
		w.id,
		terminalStatus,
		solveResp.Solution,
		scoreJSON,
		diagJSON,
		violationsJSON,
		durationMs,
		&curraVer,
		&curraCommit,
		&ruleSetHash,
		draftVer,
		newAssignments,
		audit,
	)

	if err != nil {
		if errors.Is(err, repositories.ErrStaleWorker) {
			w.logger.Warn("worker lease was revoked, stolen, or cancelled; ignoring late result", "run_id", run.ID, "worker_id", w.id)
			return
		}
		w.logger.Error("failed to atomically commit terminal result", "run_id", run.ID, "error", err)
		return
	}

	w.logger.Info("run completed and atomically persisted", "run_id", run.ID, "status", terminalStatus, "duration_ms", durationMs)
}

func (w *Worker) failRun(ctx context.Context, run *domain.ScheduleRun, msg string) {
	diagJSON, _ := json.Marshal(map[string]any{"message": msg})
	curraVer := curra.CurrAVersion
	curraCommit := curra.CurrACommit

	audit := domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: run.InstitutionID,
		UserID:        &run.CreatedBy,
		Action:        "schedule_run.fail",
		ResourceType:  "schedule_run",
		ResourceID:    run.ID,
		Details:       diagJSON,
		CreatedAt:     time.Now(),
	}

	err := w.repos.ScheduleRuns.CommitTerminalResultTx(
		ctx,
		run.ID,
		w.id,
		domain.StatusFailed,
		nil,
		nil,
		diagJSON,
		nil,
		0,
		&curraVer,
		&curraCommit,
		nil,
		nil,
		nil,
		audit,
	)

	if err != nil && errors.Is(err, repositories.ErrStaleWorker) {
		w.logger.Warn("worker lease expired while failing run", "run_id", run.ID)
	}
}

func parseAssignmentsFromSolution(versionID uuid.UUID, solutionJSON json.RawMessage) ([]domain.ScheduleAssignment, error) {
	var sol struct {
		Assignments []struct {
			ID                   string `json:"id"`
			CourseOfferingID     string `json:"courseOfferingId"`
			SessionRequirementID string `json:"sessionRequirementId"`
			StudentGroupID       string `json:"studentGroupId"`
			FacultyID            string `json:"facultyId"`
			RoomID               string `json:"roomId"`
			TimeSlotID           string `json:"timeSlotId"`
			Instance             int    `json:"instance"`
		} `json:"assignments"`
	}

	if err := json.Unmarshal(solutionJSON, &sol); err != nil {
		return nil, err
	}

	res := make([]domain.ScheduleAssignment, len(sol.Assignments))
	for i, a := range sol.Assignments {
		res[i] = domain.ScheduleAssignment{
			ID:                   uuid.New(),
			VersionID:            versionID,
			AssignmentID:         a.ID,
			CourseOfferingID:     a.CourseOfferingID,
			SessionRequirementID: a.SessionRequirementID,
			StudentGroupID:       a.StudentGroupID,
			FacultyID:            a.FacultyID,
			RoomID:               a.RoomID,
			TimeSlotID:           a.TimeSlotID,
			Instance:             a.Instance,
			CreatedAt:            time.Now(),
		}
	}

	return res, nil
}
