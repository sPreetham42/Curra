package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/domain"
)

// mapToCanonical converts a raw adapter SolveResponse into the
// application-owned CanonicalResult. It does NOT import any Engine V1
// types directly — it only deserializes the adapter's JSON solution,
// which is the documented Engine V1 wire format.
func mapToCanonical(run domain.ScheduleRun, snap domain.ProblemSnapshot, resp curra.SolveResponse, ruleSetHash string, seed int64) domain.CanonicalResult {
	assignments := parseAssignments(resp.Solution)

	diags := canonicalDiagnosticsFromAdapter(resp.Diagnostics)

	return domain.CanonicalResult{
		RunID:         run.ID,
		SnapshotID:    snap.ID,
		Status:        resp.Status,
		HardViolations: resp.Score.HardViolations,
		SoftPenalty:    resp.Score.SoftPenalty,
		Assignments:   assignments,
		Diagnostics:   diags,
		Metadata: domain.ResultMetadata{
			EngineVersion:  engineVersionOr(resp.Metadata.Version, curra.EngineVersion()),
			EngineCommit:   engineVersionOr(resp.Metadata.Commit, curra.EngineCommit()),
			AdapterVersion: curra.CurrAVersion,
			BuildAt:        engineVersionOr(resp.Metadata.BuildAt, curra.EngineBuildAt()),
			RuleSetHash:    engineVersionOr(resp.Metadata.RuleSetHash, ruleSetHash),
			InputHash:      ComputeInputHash(snap, nil),
			Seed:           seed,
		},
	}
}

func engineVersionOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// assignmentFromEngine is the wire-format shape of one Engine V1
// assignment inside the JSON solution body. The application treats this
// as opaque; the only fields it depends on are the IDs that map back to
// canonical domain identifiers.
type assignmentFromEngine struct {
	ID                   string `json:"id"`
	CourseOfferingID     string `json:"courseOfferingId"`
	SessionRequirementID string `json:"sessionRequirementId"`
	StudentGroupID       string `json:"studentGroupId"`
	FacultyID            string `json:"facultyId"`
	RoomID               string `json:"roomId"`
	TimeSlotID           string `json:"timeSlotId"`
	Instance             int    `json:"instance"`
}

type solutionEnvelope struct {
	Assignments []assignmentFromEngine `json:"assignments"`
}

func parseAssignments(solutionJSON json.RawMessage) []domain.CanonicalAssignment {
	if len(solutionJSON) == 0 {
		return nil
	}
	var env solutionEnvelope
	if err := json.Unmarshal(solutionJSON, &env); err != nil {
		return nil
	}
	out := make([]domain.CanonicalAssignment, 0, len(env.Assignments))
	for _, a := range env.Assignments {
		out = append(out, domain.CanonicalAssignment{
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

func canonicalDiagnosticsFromAdapter(d curra.DiagnosticsDTO) domain.CanonicalDiagnostics {
	return domain.CanonicalDiagnostics{
		NodesExplored: d.NodesExplored,
		Backtracks:    d.Backtracks,
		Message:       d.Message,
	}
}

// buildEngineSnapshot constructs the immutable engine-version-tagged
// record of the exact input/output captured for a run.
func buildEngineSnapshot(runID, snapID, instID uuid.UUID, seed int64, ruleSetHash, adapterVersion string, request any, response any, diagnostics any) (domain.EngineSnapshot, error) {
	reqJSON, err := json.Marshal(request)
	if err != nil {
		return domain.EngineSnapshot{}, fmt.Errorf("marshal engine request: %w", err)
	}
	respJSON, err := json.Marshal(response)
	if err != nil {
		return domain.EngineSnapshot{}, fmt.Errorf("marshal engine response: %w", err)
	}
	diagJSON, err := json.Marshal(diagnostics)
	if err != nil {
		return domain.EngineSnapshot{}, fmt.Errorf("marshal engine diagnostics: %w", err)
	}

	return domain.EngineSnapshot{
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
	}, nil
}
