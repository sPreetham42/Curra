# ADR-0001: Engine V1 Boundary & Versioning Strategy

**Date:** 2026-09-05
**Status:** Accepted
**Deciders:** Project owner

---

## Context

CURRA Engine V1 is frozen. The application platform must integrate with it through a stable, versioned boundary that:

1. Prevents accidental import of `internal/scheduler/` outside the designated adapter package.
2. Exposes engine capabilities (version, algorithms, stages) to the application layer.
3. Captures reproducibility metadata (version, commit, build time, RuleSetHash) in every solve response.
4. Replaces the previously hardcoded static version strings (`CurrAVersion`, `CurrACommit`).

---

## Decision

### 1. Engine Version Package

Create `internal/scheduler/engine/version.go` with three package-level variables injected at build time via Go's `-ldflags`:

```go
var Version, Commit, BuildAt string
```

At runtime (if not injected), the `init()` function reads values from `debug.BuildInfo`.

Build command:
```bash
go build -ldflags="-X github.com/sPreetham42/timetable-platform/internal/scheduler/engine.Version=1.0.0 \
  -X github.com/sPreetham42/timetable-platform/internal/scheduler/engine.Commit=$(git rev-parse --short HEAD) \
  -X github.com/sPreetham42/timetable-platform/internal/scheduler/engine.BuildAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

### 2. EngineV1 Interface

Define an `EngineV1` interface in `application/internal/curra/engine.go` that mirrors the `CurraAdapter` interface and adds a `Capabilities()` method:

```go
type EngineV1 interface {
    Solve(ctx context.Context, req SolveRequest) (SolveResponse, error)
    Verify(ctx context.Context, req VerifyRequest) (VerifyResponse, error)
    ValidateMove(ctx context.Context, req ValidateMoveRequest) (ValidateMoveResponse, error)
    ValidateSwap(ctx context.Context, req ValidateSwapRequest) (ValidateMoveResponse, error)
    CompileConstraints(ctx context.Context, req CompileRequest) (CompileResponse, error)
    Capabilities() SolverCapabilities
}
```

### 3. SolverCapabilities Manifest

`Capabilities()` returns a struct describing the engine's capabilities:

```go
type SolverCapabilities struct {
    Version    string   `json:"version"`
    Commit     string   `json:"commit"`
    BuildAt    string   `json:"buildAt"`
    Stages     []string `json:"stages"`
    Algorithms []string `json:"algorithms"`
}
```

This enables the application to introspect the engine version without coupling to internal types.

### 4. SolveMetadata in SolveResponse

Every `SolveResponse` includes a `Metadata` field:

```go
type SolveMetadata struct {
    Version     string `json:"version"`
    Commit      string `json:"commit"`
    BuildAt     string `json:"buildAt"`
    RuleSetHash string `json:"ruleSetHash,omitempty"`
}
```

This captures reproducibility metadata for audit, debugging, and reproducibility guarantees.

### 5. Compatibility Shim

A `compatibility.go` file in `application/internal/curra/` re-exports the version variables as `CurrAVersion` and `CurrACommit`:

```go
var CurrAVersion = engine.Version
var CurrACommit  = engine.Commit
```

This maintains backward compatibility with existing code that references these identifiers (worker, run service, repository).

### 6. AST-Based Import Boundary Enforcement

The existing `application/boundary_test.go` performs AST scanning on every `go test` run. It is included in CI. No Go packages outside `application/internal/curra/` may import `github.com/sPreetham42/timetable-platform/internal/scheduler`.

### 7. CI Enforcement

GitHub Actions CI runs:
- `TestCurraImportBoundary` (AST boundary test) on every PR
- Root module test suite
- Application module test suite
- Adapter-specific tests

---

## Consequences

### Positive
- Engine version/commit/build metadata is injected at build time, not hardcoded in source.
- Every solve response carries version + reproducibility metadata.
- The application can introspect engine capabilities at runtime via `Capabilities()`.
- The `EngineV1` interface documents the stable contract the adapter must implement.
- AST boundary enforcement runs on every test invocation.
- CI enforces the boundary in CI pipelines.

### Negative
- Build pipeline must inject version variables via `-ldflags`. If not injected, values fall back to `debug.BuildInfo` (which may report `unknown` or `devel` for local builds).
- Adding the `Capabilities()` method to `CurraAdapter` requires updating all mock implementations.

### Neutral
- `SolveResponse.RuleSetHash` field is removed in favor of `SolveResponse.Metadata.RuleSetHash`. Callers reading `RuleSetHash` directly will need updating (this is a minor API change in the adapter DTOs only; the engine API surface is unchanged).

---

## Alternatives Considered

### No version injection (status quo)
Static strings in `adapter.go`. Rejected because they require manual updates and cannot reflect actual build provenance.

### Separate version package in application
Creating a version package in the application module that mirrors engine values. Rejected in favor of direct import from the engine package via the compatibility shim, reducing duplication and ensuring a single source of truth.

### RuleSetHash in a response header
Storing `RuleSetHash` in an HTTP response header rather than the response body. Rejected because the adapter is not HTTP-specific; it is used by workers and services where headers are not available.
