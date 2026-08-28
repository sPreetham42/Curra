package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type RunService struct {
	repos   *repositories.Repos
	adapter curra.CurraAdapter
}

func NewRunService(repos *repositories.Repos, adapter curra.CurraAdapter) *RunService {
	return &RunService{repos: repos, adapter: adapter}
}

func (s *RunService) CreateRun(
	ctx context.Context,
	timetableID, snapshotID uuid.UUID,
	solverConfigJSON json.RawMessage,
	requestedSeed *int64,
	createdBy uuid.UUID,
	idempotencyKey string,
) (domain.ScheduleRun, error) {
	// 1. Verify Timetable ownership
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return domain.ScheduleRun{}, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return domain.ScheduleRun{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ScheduleRun{}, err
	}

	// 2. Atomic Idempotency Lock
	var lockToken *uuid.UUID
	if idempotencyKey != "" {
		keyRecord, isCompleted, err := s.repos.Idempotency.Acquire(ctx, tt.InstitutionID, idempotencyKey, "schedule_run")
		if err != nil {
			if errors.Is(err, repositories.ErrIdempotencyConflict) {
				// Await completion of in-flight concurrent request
				for i := 0; i < 5; i++ {
					time.Sleep(50 * time.Millisecond)
					if existing, getErr := s.repos.Idempotency.Get(ctx, tt.InstitutionID, idempotencyKey); getErr == nil && existing != nil && existing.Status == domain.IdempotencyStatusCompleted {
						var r domain.ScheduleRun
						if unmarshalErr := json.Unmarshal(existing.ResponseBody, &r); unmarshalErr == nil {
							return r, nil
						}
					}
				}
				return domain.ScheduleRun{}, ErrConflict
			}
			return domain.ScheduleRun{}, fmt.Errorf("acquire idempotency key: %w", err)
		}
		if keyRecord != nil {
			lockToken = keyRecord.LockToken
		}
		if isCompleted && keyRecord != nil && len(keyRecord.ResponseBody) > 0 {
			var r domain.ScheduleRun
			if err := json.Unmarshal(keyRecord.ResponseBody, &r); err == nil {
				return r, nil
			}
		}
	}

	// 3. Verify Snapshot ownership & relation to timetable
	snap, err := s.repos.Snapshots.GetByID(ctx, snapshotID)
	if err != nil {
		if idempotencyKey != "" {
			_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
		}
		return domain.ScheduleRun{}, err
	}
	if snap.TimetableID != timetableID || snap.InstitutionID != tt.InstitutionID {
		if idempotencyKey != "" {
			_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
		}
		return domain.ScheduleRun{}, ErrNotFound
	}

	// 4. Resolve solver options & seed
	var finalSolverConfig json.RawMessage
	if len(solverConfigJSON) > 0 && string(solverConfigJSON) != "null" && string(solverConfigJSON) != "{}" {
		finalSolverConfig = solverConfigJSON
	} else {
		finalSolverConfig = snap.SolverConfig
	}

	var finalSeed int64
	if requestedSeed != nil {
		finalSeed = *requestedSeed
	} else {
		finalSeed = rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	}

	// 5. Compile constraints to extract canonical RuleSetHash
	compileResp, _ := s.adapter.CompileConstraints(ctx, curra.CompileRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	ruleSetHash := compileResp.RuleSetHash

	curraVer := curra.CurrAVersion
	curraCommit := curra.CurrACommit

	run := domain.ScheduleRun{
		ID:              uuid.New(),
		TimetableID:     timetableID,
		InstitutionID:   tt.InstitutionID,
		SnapshotID:      snapshotID,
		Status:          domain.StatusQueued,
		SolverConfig:    finalSolverConfig,
		ObjectiveConfig: snap.ObjectiveConfig,
		Seed:            &finalSeed,
		RuleSetHash:     &ruleSetHash,
		CurrAVersion:    &curraVer,
		CurrACommit:     &curraCommit,
		CreatedBy:       createdBy,
		Version:         1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.repos.ScheduleRuns.Create(ctx, run); err != nil {
		if idempotencyKey != "" {
			_ = s.repos.Idempotency.Release(ctx, tt.InstitutionID, idempotencyKey, lockToken)
		}
		return domain.ScheduleRun{}, fmt.Errorf("create schedule run: %w", err)
	}

	// Persist Audit Event
	_ = s.repos.AuditEvents.Create(ctx, domain.AuditEvent{
		ID:            uuid.New(),
		InstitutionID: tt.InstitutionID,
		UserID:        &createdBy,
		Action:        "schedule_run.create",
		ResourceType:  "schedule_run",
		ResourceID:    run.ID,
		Details:       json.RawMessage(fmt.Sprintf(`{"timetableId":"%s","snapshotId":"%s","seed":%d}`, timetableID, snapshotID, finalSeed)),
		CreatedAt:     time.Now(),
	})

	// Mark Idempotency Key Completed
	if idempotencyKey != "" {
		body, _ := json.Marshal(run)
		_ = s.repos.Idempotency.Complete(ctx, tt.InstitutionID, idempotencyKey, lockToken, run.ID, 201, body)
	}

	return run, nil
}

func (s *RunService) GetRun(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	run, err := s.repos.ScheduleRuns.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleRun{}, err
	}
	if err := RequireTenantMatch(ctx, run.InstitutionID); err != nil {
		return domain.ScheduleRun{}, err
	}
	return run, nil
}

func (s *RunService) ListRuns(ctx context.Context, timetableID uuid.UUID) ([]domain.ScheduleRun, error) {
	tt, err := s.repos.Timetables.GetByID(ctx, timetableID)
	if err != nil {
		return nil, err
	}
	if err := RequireTenantMatch(ctx, tt.InstitutionID); err != nil {
		return nil, err
	}
	return s.repos.ScheduleRuns.ListByTimetable(ctx, timetableID)
}

func (s *RunService) CancelRun(ctx context.Context, id uuid.UUID) (domain.ScheduleRun, error) {
	run, err := s.repos.ScheduleRuns.GetByID(ctx, id)
	if err != nil {
		return domain.ScheduleRun{}, err
	}
	if err := RequireTenantMatch(ctx, run.InstitutionID); err != nil {
		return domain.ScheduleRun{}, err
	}
	if err := RequireRole(ctx, domain.RoleInstitutionAdmin, domain.RoleScheduler); err != nil {
		return domain.ScheduleRun{}, err
	}

	// Cancel is race-safe; if already finished/solved/failed, cancellation returns updated run or no-op
	if run.Status == domain.StatusQueued || run.Status == domain.StatusRunning {
		_, err = s.repos.ScheduleRuns.Cancel(ctx, id)
		if err != nil {
			return domain.ScheduleRun{}, fmt.Errorf("cancel run: %w", err)
		}
	}

	return s.repos.ScheduleRuns.GetByID(ctx, id)
}
