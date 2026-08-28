package curra

import (
	"github.com/sPreetham42/timetable-platform/internal/scheduler/diagnostics"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/scorer"
)

func mapScore(s scorer.Score) ScoreDTO {
	return ScoreDTO{
		HardViolations: s.HardViolations,
		SoftPenalty:    s.SoftPenalty,
	}
}

func mapDiagnostics(d diagnostics.Diagnostics) DiagnosticsDTO {
	return DiagnosticsDTO{
		Status:        string(d.Status),
		NodesExplored: d.NodesExplored,
		Candidates:    d.Candidates,
		Backtracks:    d.Backtracks,
		Message:       d.Message,
	}
}

func mapViolations(v []diagnostics.Violation) []ViolationDTO {
	if len(v) == 0 {
		return nil
	}
	result := make([]ViolationDTO, len(v))
	for i, viol := range v {
		result[i] = ViolationDTO{
			ConstraintName: viol.ConstraintName,
			Severity:       string(viol.Severity),
			Message:        viol.Message,
			AssignmentID:   viol.AssignmentID,
			RelatedIDs:     viol.RelatedIDs,
			Metadata:       viol.Metadata,
		}
	}
	return result
}
