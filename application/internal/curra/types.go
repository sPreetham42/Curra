package curra

import (
	"context"
	"encoding/json"
)

// CurraAdapter is the ONLY boundary between application and CURRA.
// It is stateless. Each call is independent. No solver state persists.
type CurraAdapter interface {
	// Solve runs the complete pipeline: validate -> presolve -> CSP -> Tabu -> verify.
	Solve(ctx context.Context, req SolveRequest) (SolveResponse, error)

	// Verify independently checks a stored solution against a snapshot.
	Verify(ctx context.Context, req VerifyRequest) (VerifyResponse, error)

	// ValidateMove tests a manual edit without mutating the original solution.
	ValidateMove(ctx context.Context, req ValidateMoveRequest) (ValidateMoveResponse, error)

	// ValidateSwap tests a manual swap without mutating the original solution.
	ValidateSwap(ctx context.Context, req ValidateSwapRequest) (ValidateMoveResponse, error)

	// CompileConstraints validates and compiles constraint instances.
	CompileConstraints(ctx context.Context, req CompileRequest) (CompileResponse, error)
}

// SolveRequest is the application-facing request for solving a timetable.
type SolveRequest struct {
	ProblemJSON      json.RawMessage `json:"problem"`
	ConstraintsJSON  json.RawMessage `json:"constraints,omitempty"`
	Seed             int64           `json:"seed"`
	ObjectiveWeights map[string]int  `json:"objectiveWeights,omitempty"`
	DisableOptimize  bool            `json:"disableOptimize,omitempty"`
	MaxNodes         int             `json:"maxNodes,omitempty"`
	SearchMode       string          `json:"searchMode,omitempty"`
}

// SolveResponse is the application-facing response from solving.
type SolveResponse struct {
	Status      string          `json:"status"`
	Solution    json.RawMessage `json:"solution,omitempty"`
	Score       ScoreDTO        `json:"score"`
	Diagnostics DiagnosticsDTO `json:"diagnostics"`
	Violations  []ViolationDTO  `json:"violations,omitempty"`
	RuleSetHash string          `json:"ruleSetHash,omitempty"`
}

// VerifyRequest is the application-facing request for verifying a solution.
type VerifyRequest struct {
	ProblemJSON      json.RawMessage `json:"problem"`
	SolutionJSON     json.RawMessage `json:"solution"`
	ConstraintsJSON  json.RawMessage `json:"constraints,omitempty"`
	ObjectiveWeights map[string]int  `json:"objectiveWeights,omitempty"`
}

// VerifyResponse is the application-facing response from verification.
type VerifyResponse struct {
	Valid      bool           `json:"valid"`
	Status     string         `json:"status"`
	Violations []ViolationDTO `json:"violations,omitempty"`
	Score      ScoreDTO       `json:"score"`
}

// PlacementDTO represents a room and time slot placement.
type PlacementDTO struct {
	RoomID     string `json:"roomId"`
	TimeSlotID string `json:"timeSlotId"`
}

// MoveDTO represents a single assignment move.
type MoveDTO struct {
	AssignmentID string       `json:"assignmentId"`
	From         PlacementDTO `json:"from"`
	To           PlacementDTO `json:"to"`
}

// SwapDTO represents swapping two assignments.
type SwapDTO struct {
	Assignment1ID string       `json:"assignment1Id"`
	Assignment2ID string       `json:"assignment2Id"`
	Placement1    PlacementDTO `json:"placement1"`
	Placement2    PlacementDTO `json:"placement2"`
}

// ValidateMoveRequest is the request to validate a single assignment move.
type ValidateMoveRequest struct {
	ProblemJSON     json.RawMessage `json:"problem"`
	SolutionJSON    json.RawMessage `json:"solution"`
	Move            MoveDTO         `json:"move"`
	ConstraintsJSON json.RawMessage `json:"constraints,omitempty"`
}

// ValidateSwapRequest is the request to validate a swap of two assignments.
type ValidateSwapRequest struct {
	ProblemJSON     json.RawMessage `json:"problem"`
	SolutionJSON    json.RawMessage `json:"solution"`
	Swap            SwapDTO         `json:"swap"`
	ConstraintsJSON json.RawMessage `json:"constraints,omitempty"`
}

// ValidateMoveResponse is the response from validating a move or swap.
type ValidateMoveResponse struct {
	Valid      bool            `json:"valid"`
	Status     string          `json:"status"`
	Violations []ViolationDTO  `json:"violations,omitempty"`
	Score      ScoreDTO        `json:"score"`
	Solution   json.RawMessage `json:"solution,omitempty"`
}

// CompileRequest is the request to compile constraint instances.
type CompileRequest struct {
	ProblemJSON     json.RawMessage `json:"problem"`
	ConstraintsJSON json.RawMessage `json:"constraints"`
}

// CompileResponse is the application-facing response from constraint compilation.
type CompileResponse struct {
	RuleSetHash string         `json:"ruleSetHash,omitempty"`
	Errors      []CompileError `json:"errors,omitempty"`
}

// CompileError represents a constraint compilation error.
type CompileError struct {
	TemplateID string `json:"templateId"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

// ScoreDTO is the application-facing score representation.
type ScoreDTO struct {
	HardViolations int            `json:"hardViolations"`
	SoftPenalty    int            `json:"softPenalty"`
	Breakdown      map[string]any `json:"breakdown,omitempty"`
}

// DiagnosticsDTO is the application-facing diagnostics representation.
type DiagnosticsDTO struct {
	Status        string `json:"status"`
	NodesExplored int    `json:"nodesExplored"`
	Candidates    int    `json:"candidates"`
	Backtracks    int    `json:"backtracks"`
	Message       string `json:"message,omitempty"`
}

// ViolationDTO is the application-facing violation representation.
type ViolationDTO struct {
	ConstraintName string            `json:"constraintName"`
	Severity       string            `json:"severity"`
	Message        string            `json:"message"`
	AssignmentID   string            `json:"assignmentId,omitempty"`
	RelatedIDs     map[string]string `json:"relatedIds,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}
