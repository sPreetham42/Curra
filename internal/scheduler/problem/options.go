package problem

type SearchMode string

const (
	SearchModeBasic     SearchMode = "BASIC"
	SearchModeHeuristic SearchMode = "HEURISTIC"
)

// SolveOptions controls the bounded search performed by a solver.
type SolveOptions struct {
	// MaxNodes limits search nodes. Zero means use the solver default.
	MaxNodes int
	// ViolationLimit caps how many rejected-candidate violations diagnostics retain.
	// Zero means use the solver default.
	ViolationLimit int
	// SearchMode selects basic deterministic search or heuristic search.
	SearchMode SearchMode
}

func (o SolveOptions) normalized() SolveOptions {
	if o.MaxNodes <= 0 {
		o.MaxNodes = 100000
	}
	if o.ViolationLimit <= 0 {
		o.ViolationLimit = 50
	}
	if o.SearchMode == "" {
		o.SearchMode = SearchModeHeuristic
	}
	return o
}

// Normalize returns SolveOptions with defaults filled in.
func (o SolveOptions) Normalize() SolveOptions {
	return o.normalized()
}
