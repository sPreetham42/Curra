package curra_test

import (
	"testing"

	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
)

func TestCapabilities(t *testing.T) {
	adapter := curra.New(nil)
	caps := adapter.Capabilities()

	if caps.Version == "" {
		t.Error("Capabilities().Version should not be empty")
	}
	if caps.Commit == "" {
		t.Error("Capabilities().Commit should not be empty")
	}
	if len(caps.Stages) == 0 {
		t.Error("Capabilities().Stages should not be empty")
	}
	if len(caps.Algorithms) == 0 {
		t.Error("Capabilities().Algorithms should not be empty")
	}

	expectedStages := []string{"CSP Backtracking", "Tabu Search", "Independent Verification"}
	for i, s := range expectedStages {
		if caps.Stages[i] != s {
			t.Errorf("Stages[%d]: got %q, want %q", i, caps.Stages[i], s)
		}
	}

	expectedAlgs := []string{"MRV", "Degree Heuristic", "LCV", "Forward Checking", "Tabu Search"}
	for i, a := range expectedAlgs {
		if caps.Algorithms[i] != a {
			t.Errorf("Algorithms[%d]: got %q, want %q", i, caps.Algorithms[i], a)
		}
	}
}

func TestCapabilitiesMatchesEnginePackage(t *testing.T) {
	adapter := curra.New(nil)
	caps := adapter.Capabilities()

	if caps.Version != engine.Version {
		t.Errorf("Capabilities().Version = %q, want engine.Version %q", caps.Version, engine.Version)
	}
	if caps.Commit != engine.Commit {
		t.Errorf("Capabilities().Commit = %q, want engine.Commit %q", caps.Commit, engine.Commit)
	}
	if caps.BuildAt != engine.BuildAt {
		t.Errorf("Capabilities().BuildAt = %q, want engine.BuildAt %q", caps.BuildAt, engine.BuildAt)
	}
}

func TestCompatibilityShim(t *testing.T) {
	if curra.CurrAVersion == "" {
		t.Error("compatibility shim CurrAVersion should not be empty")
	}
	if curra.CurrACommit == "" {
		t.Error("compatibility shim CurrACommit should not be empty")
	}
	if curra.CurrAVersion != engine.Version {
		t.Errorf("CurrAVersion = %q, want engine.Version %q", curra.CurrAVersion, engine.Version)
	}
	if curra.CurrACommit != engine.Commit {
		t.Errorf("CurrACommit = %q, want engine.Commit %q", curra.CurrACommit, engine.Commit)
	}
}

func TestAdapterImplementsEngineV1(t *testing.T) {
	var _ curra.EngineV1 = (*curra.Adapter)(nil)
}
