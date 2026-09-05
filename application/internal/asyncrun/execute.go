package asyncrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

type CanonicalResult = domain.CanonicalResult
type CanonicalAssignment = domain.CanonicalAssignment
type CanonicalDiagnostics = domain.CanonicalDiagnostics
type ResultMetadata = domain.ResultMetadata
type EngineSnapshot = domain.EngineSnapshot

type ComputeInputHashFunc func(snap domain.ProblemSnapshot, extra []byte) string

func Execute(
	ctx context.Context,
	run domain.ScheduleRun,
	snap domain.ProblemSnapshot,
	repos *repositories.Repos,
	adapter curra.CurraAdapter,
	computeHash ComputeInputHashFunc,
	commitHook func(CanonicalResult, json.RawMessage) error,
) error {
	seed := int64(0)
	if run.Seed != nil {
		seed = *run.Seed
	}

	compileResp, compileErr := adapter.CompileConstraints(ctx, curra.CompileRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	if compileErr != nil {
		canonical := CanonicalResult{
			Status:     string(domain.StatusInvalidProblem),
			SnapshotID: snap.ID,
			Diagnostics: CanonicalDiagnostics{
				Message: compileErr.Error(),
			},
			Metadata: ResultMetadata{
				InputHash:   computeHash(snap, nil),
				RuleSetHash: "",
				Seed:        seed,
			},
		}
		_ = commitHook(canonical, nil)
		return nil
	}
	ruleSetHash := compileResp.RuleSetHash

	solveReq := curra.SolveRequest{
		ProblemJSON:     snap.ProblemJSON,
		ConstraintsJSON: snap.ConstraintInstances,
		Seed:           seed,
	}
	solveResp, solveErr := adapter.Solve(ctx, solveReq)
	if solveErr != nil && solveResp.Status == "" {
		canonical := CanonicalResult{
			Status:     string(domain.StatusFailed),
			SnapshotID: snap.ID,
			Diagnostics: CanonicalDiagnostics{
				Message: solveErr.Error(),
			},
			Metadata: ResultMetadata{
				InputHash:   computeHash(snap, nil),
				RuleSetHash: ruleSetHash,
				Seed:        seed,
			},
		}
		_ = commitHook(canonical, nil)
		return nil
	}

	canonical := mapToCanonicalResult(run, snap, solveResp, ruleSetHash, seed, computeHash)

	verified, _ := runIndependentVerification(ctx, snap, solveResp, adapter)
	canonical.VerifierOK = verified
	canonical.Verified = verified && canonical.Status == "SOLVED"

	if !verified && canonical.Status == "SOLVED" {
		canonical.Status = string(domain.StatusInvalidResult)
	}

	if canonical.Status == "SOLVED" {
		engineSnap := buildEngineSnapshot(
			run.ID, snap.ID, run.InstitutionID, seed,
			ruleSetHash, curra.CurrAVersion,
			solveReq, solveResp, canonical.Diagnostics,
		)
		engineSnap.InputHash = canonical.Metadata.InputHash
		if err := repos.EngineSnapshots.Create(ctx, engineSnap); err != nil {
			return fmt.Errorf("persist engine snapshot: %w", err)
		}
	}

	if commitHook != nil {
		if err := commitHook(canonical, solveResp.Solution); err != nil {
			return err
		}
	}

	return nil
}

func mapToCanonicalResult(run domain.ScheduleRun, snap domain.ProblemSnapshot, resp curra.SolveResponse, ruleSetHash string, seed int64, computeHash ComputeInputHashFunc) CanonicalResult {
	assignments := parseAssignmentsFromEngine(resp.Solution)
	diags := canonicalDiagnosticsFromAdapter(resp.Diagnostics)

	return CanonicalResult{
		RunID:         run.ID,
		SnapshotID:    snap.ID,
		Status:        resp.Status,
		HardViolations: resp.Score.HardViolations,
		SoftPenalty:    resp.Score.SoftPenalty,
		Assignments:   assignments,
		Diagnostics:   diags,
		Metadata: ResultMetadata{
			EngineVersion:  engineVersionOr(resp.Metadata.Version, curra.EngineVersion()),
			EngineCommit:   engineVersionOr(resp.Metadata.Commit, curra.EngineCommit()),
			AdapterVersion: curra.CurrAVersion,
			BuildAt:        engineVersionOr(resp.Metadata.BuildAt, curra.EngineBuildAt()),
			RuleSetHash:    engineVersionOr(resp.Metadata.RuleSetHash, ruleSetHash),
			InputHash:      computeHash(snap, nil),
			Seed:           seed,
		},
	}
}

func runIndependentVerification(ctx context.Context, snap domain.ProblemSnapshot, solveResp curra.SolveResponse, adapter curra.CurraAdapter) (bool, error) {
	if solveResp.Status != "SOLVED" || len(solveResp.Solution) == 0 {
		return false, nil
	}
	v, err := adapter.Verify(ctx, curra.VerifyRequest{
		ProblemJSON:     snap.ProblemJSON,
		SolutionJSON:    solveResp.Solution,
		ConstraintsJSON: snap.ConstraintInstances,
	})
	if err != nil {
		return false, err
	}
	return v.Valid, nil
}

func engineVersionOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type engineAssignment struct {
	ID                   string `json:"id"`
	CourseOfferingID     string `json:"courseOfferingId"`
	SessionRequirementID string `json:"sessionRequirementId"`
	StudentGroupID       string `json:"studentGroupId"`
	FacultyID            string `json:"facultyId"`
	RoomID               string `json:"roomId"`
	TimeSlotID           string `json:"timeSlotId"`
	Instance             int    `json:"instance"`
}

type engineSolution struct {
	Assignments []engineAssignment `json:"assignments"`
}

func parseAssignmentsFromEngine(solutionJSON json.RawMessage) []CanonicalAssignment {
	if len(solutionJSON) == 0 {
		return nil
	}
	var env engineSolution
	if err := json.Unmarshal(solutionJSON, &env); err != nil {
		return nil
	}
	out := make([]CanonicalAssignment, 0, len(env.Assignments))
	for _, a := range env.Assignments {
		out = append(out, CanonicalAssignment{
			AssignmentID:         a.ID,
			CourseOfferingID:     a.CourseOfferingID,
			SessionRequirementID: a.SessionRequirementID,
			StudentGroupID:       a.StudentGroupID,
			FacultyID:            a.FacultyID,
			RoomID:               a.RoomID,
			TimeSlotID:           a.TimeSlotID,
			Instance:             a.Instance,
		})
	}
	return out
}

func canonicalDiagnosticsFromAdapter(d curra.DiagnosticsDTO) CanonicalDiagnostics {
	return CanonicalDiagnostics{
		NodesExplored: d.NodesExplored,
		Backtracks:    d.Backtracks,
		Message:       d.Message,
	}
}

func buildEngineSnapshot(runID, snapID, instID uuid.UUID, seed int64, ruleSetHash, adapterVersion string, request any, response any, diagnostics any) EngineSnapshot {
	reqJSON, _ := json.Marshal(request)
	respJSON, _ := json.Marshal(response)
	diagJSON, _ := json.Marshal(diagnostics)

	return EngineSnapshot{
		ID:             uuid.New(),
		ScheduleRunID:  runID,
		SnapshotID:     snapID,
		InstitutionID:  instID,
		EngineVersion:  curra.EngineVersion(),
		EngineCommit:   curra.EngineCommit(),
		AdapterVersion: adapterVersion,
		RuleSetHash:    ruleSetHash,
		InputHash:      "",
		Request:        reqJSON,
		Response:       respJSON,
		Diagnostics:    diagJSON,
		CreatedAt:      time.Now().UTC(),
	}
}

func ComputeInputHash(snap domain.ProblemSnapshot, extra []byte) string {
	h := sha256.New()
	h.Write([]byte("problem:"))
	h.Write(snap.ProblemJSON)
	h.Write([]byte("|constraints:"))
	h.Write(snap.ConstraintInstances)
	h.Write([]byte("|solver:"))
	h.Write(snap.SolverConfig)
	h.Write([]byte("|objective:"))
	h.Write(snap.ObjectiveConfig)
	if len(extra) > 0 {
		h.Write([]byte("|extra:"))
		h.Write(extra)
	}
	return hex.EncodeToString(h.Sum(nil))
}
