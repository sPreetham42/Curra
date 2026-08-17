package scorer

// Score is reserved for future soft-constraint optimization. Phase 1 only
// records feasibility, so the default zero score is meaningful.
type Score struct {
	HardViolations int `json:"hardViolations"`
	SoftPenalty    int `json:"softPenalty"`
}
