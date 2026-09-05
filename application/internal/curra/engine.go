package curra

import (
	"context"

	"github.com/sPreetham42/timetable-platform/internal/scheduler/engine"
)

type SolverCapabilities struct {
	Version    string   `json:"version"`
	Commit     string   `json:"commit"`
	BuildAt    string   `json:"buildAt"`
	Stages     []string `json:"stages"`
	Algorithms []string `json:"algorithms"`
}

type EngineV1 interface {
	Solve(ctx context.Context, req SolveRequest) (SolveResponse, error)
	Verify(ctx context.Context, req VerifyRequest) (VerifyResponse, error)
	ValidateMove(ctx context.Context, req ValidateMoveRequest) (ValidateMoveResponse, error)
	ValidateSwap(ctx context.Context, req ValidateSwapRequest) (ValidateMoveResponse, error)
	CompileConstraints(ctx context.Context, req CompileRequest) (CompileResponse, error)
	Capabilities() SolverCapabilities
}

// EngineVersion returns the current Engine V1 version string. It is the
// single application-side entry point for engine version metadata; all
// other application code must read this through the adapter rather than
// importing internal/scheduler.
func EngineVersion() string { return engine.Version }

// EngineCommit returns the current Engine V1 commit string.
func EngineCommit() string { return engine.Commit }

// EngineBuildAt returns the current Engine V1 build time.
func EngineBuildAt() string { return engine.BuildAt }
