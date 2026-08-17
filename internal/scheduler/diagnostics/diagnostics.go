package diagnostics

type Severity string

const (
	SeverityHard Severity = "HARD"
	SeveritySoft Severity = "SOFT"
	SeverityInfo Severity = "INFO"
)

type SolveStatus string

const (
	SolveStatusSolved           SolveStatus = "SOLVED"
	SolveStatusInfeasible       SolveStatus = "INFEASIBLE"
	SolveStatusInvalidProblem   SolveStatus = "INVALID_PROBLEM"
	SolveStatusCancelled        SolveStatus = "CANCELLED"
	SolveStatusDeadlineExceeded SolveStatus = "DEADLINE_EXCEEDED"
	SolveStatusNodeLimit        SolveStatus = "NODE_LIMIT"
)

// Violation explains why a candidate assignment or whole problem is invalid.
type Violation struct {
	ConstraintName string            `json:"constraintName"`
	Severity       Severity          `json:"severity"`
	Message        string            `json:"message"`
	AssignmentID   string            `json:"assignmentId,omitempty"`
	RelatedIDs     map[string]string `json:"relatedIds,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Diagnostics captures search behavior and explainability breadcrumbs.
type Diagnostics struct {
	Status        SolveStatus `json:"status"`
	NodesExplored int         `json:"nodesExplored"`
	Candidates    int         `json:"candidates"`
	Backtracks    int         `json:"backtracks"`
	Violations    []Violation `json:"violations,omitempty"`
	Message       string      `json:"message,omitempty"`
}

func (d *Diagnostics) AddViolation(limit int, violation Violation) {
	if limit <= 0 || len(d.Violations) < limit {
		d.Violations = append(d.Violations, violation)
	}
}
