package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/asyncrun"
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

func calculateLeaseDuration(solverConfigJSON json.RawMessage) time.Duration {
	const defaultSafetyMargin = 2 * time.Minute
	const fallbackTimeout = 5 * time.Minute

	var cfg struct {
		DeadlineSeconds    int `json:"deadlineSeconds"`
		TimeoutSeconds     int `json:"timeoutSeconds"`
		MaxDurationSeconds int `json:"maxDurationSeconds"`
		MaxNodes           int `json:"maxNodes"`
	}

	if len(solverConfigJSON) > 0 {
		_ = json.Unmarshal(solverConfigJSON, &cfg)
	}

	timeout := fallbackTimeout
	if cfg.DeadlineSeconds > 0 {
		timeout = time.Duration(cfg.DeadlineSeconds) * time.Second
	} else if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	} else if cfg.MaxDurationSeconds > 0 {
		timeout = time.Duration(cfg.MaxDurationSeconds) * time.Second
	} else if cfg.MaxNodes > 0 {
		estimatedSec := cfg.MaxNodes / 1000
		if estimatedSec > 300 {
			timeout = time.Duration(estimatedSec) * time.Second
		}
	}

	return timeout + defaultSafetyMargin
}

func (w *Worker) poll(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("worker recovered from panic in poll", "panic", r)
		}
	}()

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

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go w.heartbeat(heartbeatCtx, run.ID)

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
			func() {
				defer func() {
					if r := recover(); r != nil {
						w.logger.Error("heartbeat recovered from panic", "run_id", runID, "panic", r)
					}
				}()
				if err := w.repos.ScheduleRuns.UpdateHeartbeat(ctx, runID, w.id); err != nil {
					w.logger.Error("heartbeat failed", "run_id", runID, "worker_id", w.id, "error", err)
				}
			}()
		}
	}
}

func (w *Worker) execute(ctx context.Context, run *domain.ScheduleRun) {
	start := time.Now()

	snap, err := w.repos.Snapshots.GetByID(ctx, run.SnapshotID)
	if err != nil {
		w.logger.Error("failed to load snapshot for run", "run_id", run.ID, "error", err)
		w.failRun(ctx, run, fmt.Sprintf("failed to load snapshot: %v", err))
		return
	}

	var terminalStatus domain.ScheduleRunStatus
	var resultJSON json.RawMessage
	var scoreJSON, diagJSON, violationsJSON json.RawMessage
	var durationMs int64
	var curraVer, curraCommit, ruleSetHash string

	commitHook := func(canonical asyncrun.CanonicalResult, solutionJSON json.RawMessage) error {
		terminalStatus = domain.ScheduleRunStatus(canonical.Status)

		if len(canonical.Assignments) > 0 {
			resultJSON, _ = json.Marshal(map[string]any{"assignments": canonical.Assignments})
		}

		diagJSON, _ = json.Marshal(canonical.Diagnostics)

		if terminalStatus == domain.StatusSolved {
			ruleSetHash = canonical.Metadata.RuleSetHash
			curraVer = canonical.Metadata.EngineVersion
			curraCommit = canonical.Metadata.EngineCommit
		}

		durationMs = int64(time.Since(start).Milliseconds())

		var draftVer *domain.ScheduleVersion
		var newAssignments []domain.ScheduleAssignment

		if terminalStatus == domain.StatusSolved && len(canonical.Assignments) > 0 {
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
			assignments := make([]domain.ScheduleAssignment, len(canonical.Assignments))
			for i, a := range canonical.Assignments {
				assignments[i] = domain.ScheduleAssignment{
					ID:                   uuid.New(),
					VersionID:            v.ID,
					AssignmentID:         a.AssignmentID,
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
			newAssignments = assignments
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

		err := w.repos.ScheduleRuns.CommitTerminalResultTx(
			ctx,
			run.ID,
			w.id,
			terminalStatus,
			resultJSON,
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
		return err
	}

	err = func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during solve: %v", r)
				w.logger.Error("worker recovered from panic", "run_id", run.ID, "panic", r)
			}
		}()
		return asyncrun.Execute(ctx, *run, snap, w.repos, w.adapter, asyncrun.ComputeInputHash, commitHook)
	}()

	if err != nil {
		w.logger.Error("execute failed", "run_id", run.ID, "error", err)
		w.failRun(ctx, run, fmt.Sprintf("execute failed: %v", err))
		return
	}

	if terminalStatus == "" {
		w.logger.Error("execute returned nil error but terminal status not set", "run_id", run.ID)
		w.failRun(ctx, run, "internal error: terminal status not set")
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
