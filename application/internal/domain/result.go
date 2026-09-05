package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EngineSnapshot is an immutable, engine-version-tagged record of the
// exact engine input and output captured during a run. It is owned by the
// application but its body is opaque to non-adapter code; only the adapter
// package can deserialize it into Engine V1 types.
type EngineSnapshot struct {
	ID             uuid.UUID
	ScheduleRunID  uuid.UUID
	SnapshotID     uuid.UUID
	InstitutionID  uuid.UUID
	EngineVersion  string
	EngineCommit   string
	AdapterVersion string
	RuleSetHash    string
	InputHash      string
	Request        json.RawMessage
	Response       json.RawMessage
	Diagnostics    json.RawMessage
	CreatedAt      time.Time
}

// CanonicalResult is the application-owned, Engine-V1-independent
// representation of a solved timetable. It is what the API returns to
// clients and what the frontend renders.
type CanonicalResult struct {
	RunID       uuid.UUID
	SnapshotID  uuid.UUID
	Status      string
	Verified    bool
	VerifierOK  bool
	HardViolations int
	SoftPenalty    int
	Assignments []CanonicalAssignment
	Diagnostics CanonicalDiagnostics
	Metadata    ResultMetadata
	CreatedAt   time.Time
}

// CanonicalAssignment is the application view of a single scheduled session.
type CanonicalAssignment struct {
	AssignmentID         string `json:"assignmentId"`
	CourseOfferingID     string `json:"courseOfferingId"`
	SessionRequirementID string `json:"sessionRequirementId"`
	StudentGroupID       string `json:"studentGroupId"`
	FacultyID            string `json:"facultyId"`
	RoomID               string `json:"roomId"`
	TimeSlotID           string `json:"timeSlotId"`
	Instance             int    `json:"instance"`
}

// CanonicalDiagnostics is the application view of solver diagnostics.
type CanonicalDiagnostics struct {
	NodesExplored int    `json:"nodesExplored"`
	Backtracks    int    `json:"backtracks"`
	Message       string `json:"message,omitempty"`
}

// ResultMetadata captures the reproducibility metadata for a result.
type ResultMetadata struct {
	EngineVersion  string `json:"engineVersion"`
	EngineCommit   string `json:"engineCommit"`
	AdapterVersion string `json:"adapterVersion"`
	BuildAt        string `json:"buildAt"`
	RuleSetHash    string `json:"ruleSetHash"`
	InputHash      string `json:"inputHash"`
	Seed           int64  `json:"seed"`
}
