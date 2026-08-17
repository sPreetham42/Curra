package localsearch

// TabuList maintains move signatures and their iteration expirations.
type TabuList struct {
	tenure  int
	entries map[string]int
}

// NewTabuList initializes a TabuList with the given tenure.
func NewTabuList(tenure int) *TabuList {
	if tenure < 0 {
		tenure = 0
	}
	return &TabuList{
		tenure:  tenure,
		entries: make(map[string]int),
	}
}

// Tenure returns the configured tabu tenure.
func (t *TabuList) Tenure() int {
	return t.tenure
}

// Record records a move signature into the tabu list until currentIteration + tenure.
func (t *TabuList) Record(signature string, currentIteration int) {
	if t.tenure <= 0 || signature == "" {
		return
	}
	t.entries[signature] = currentIteration + t.tenure
}

// IsTabu returns true if the move signature is currently tabu.
func (t *TabuList) IsTabu(signature string, currentIteration int) bool {
	if t.tenure <= 0 || signature == "" {
		return false
	}
	expiration, exists := t.entries[signature]
	if !exists {
		return false
	}
	if currentIteration <= expiration {
		return true
	}
	delete(t.entries, signature)
	return false
}

// Purge removes expired tabu entries.
func (t *TabuList) Purge(currentIteration int) {
	for sig, exp := range t.entries {
		if currentIteration > exp {
			delete(t.entries, sig)
		}
	}
}
