# CI / Agent Boundaries

## 1. Repository Structure

```
repository/
├── curra/                  # CURRA solver core (SEPARATE repo/dependency)
├── application/            # MIMO implements backend here
│   ├── cmd/
│   ├── internal/
│   │   ├── api/           # REST handlers, middleware
│   │   ├── services/      # Business logic
│   │   ├── curra/         # CURRA adapter (imports curra)
│   │   └── database/      # Queries, migrations
│   └── go.mod
├── frontend/               # Manus implements frontend here
│   ├── src/
│   ├── api/               # Generated from OpenAPI
│   └── package.json
├── contracts/              # Shared contracts (read-only for agents)
│   ├── curra-adapter.md
│   └── api-contract.md
├── api/
│   ├── openapi.yaml        # Single source of truth for API
│   └── fixtures/           # JSON fixtures for mocking
├── docs/                   # Architecture documents
├── database/
│   └── schema.md           # Database design (human-approved migrations)
└── .agent-boundaries.yaml  # Mechanical enforcement rules
```

---

## 2. Agent Rules

### MIMO (Backend Agent)

| Allowed | Forbidden |
|---|---|
| Implement backend services in `application/` | Modify `frontend/` |
| Write Go code | Modify `curra/` |
| Create database migrations (draft) | Auto-apply migrations without approval |
| Implement CURRA adapter | Modify CURRA algorithms |
| Write API handlers conforming to OpenAPI | Modify `openapi.yaml` without approval |
| Write tests | Modify existing CURRA tests |
| Read contract documents | Modify contract documents without approval |

### Manus (Frontend Agent)

| Allowed | Forbidden |
|---|---|
| Implement frontend in `frontend/` | Modify `application/` |
| Use generated API types from `openapi.yaml` | Hand-write API schemas |
| Implement UI components | Modify `curra/` |
| Write frontend tests | Modify backend code |
| Read contract documents | Modify contract documents without approval |
| Mock API responses from fixtures | Create backend API endpoints |

---

## 3. Mechanical Enforcement

Prose rules are necessary but not sufficient. The following CI mechanisms must be implemented:

### 3.1 Path-Based Access Control (CI Pipeline)

Every PR must be validated against path restrictions:

| Agent | Blocked Paths |
|---|---|
| MIMO PRs | `frontend/**`, `curra/**` |
| Manus PRs | `application/**`, `curra/**`, `database/**` |

**Implementation:** GitHub Actions workflow that checks `git diff --name-only` against blocked paths. Fail CI if any blocked path is modified.

### 3.2 OpenAPI Drift Detection (CI Pipeline)

After any PR touching `api/openapi.yaml` or `frontend/src/api/**`:
1. Regenerate frontend API types from `openapi.yaml`.
2. If generated types differ from committed types, CI fails.
3. This catches manual hand-editing of generated API schemas.

**Implementation:** `npm run api:check` script that regenerates and diffs.

### 3.3 CURRA Import Boundary (CI Pipeline)

Verify that no application code outside `application/internal/curra/` imports CURRA packages:

```bash
# Fail if any file outside the adapter imports curra packages
grep -r 'github.com/sPreetham42/timetable-platform/internal/scheduler' application/internal/ \
  --include='*.go' | grep -v 'application/internal/curra/' | grep -v '_test.go'
```

**Implementation:** Shell script in CI that greps for CURRA imports outside the adapter.

### 3.4 CURRA Dependency Pinning

The `application/go.mod` must pin CURRA to a specific commit hash (not a branch or tag that can move):

```
require github.com/sPreetham42/timetable-platform v0.0.0-20260824-abc123f
```

Any CURRA version bump requires human approval.

### 3.5 Migration Review Gate

Any PR touching `database/` or `*.sql` files requires:
1. Human approval (GitHub CODEOWNERS).
2. Migration must be reversible (down migration required).
3. No destructive operations without explicit approval.

### 3.6 Contract Change Gate

Any PR touching `contracts/`, `docs/permissions.md`, or `api/openapi.yaml` requires:
1. Human approval (GitHub CODEOWNERS).
2. Both MIMO and Manus must acknowledge before merging.

### 3.7 `.agent-boundaries.yaml`

```yaml
agents:
  mimo:
    allowed_paths:
      - "application/**"
      - "contracts/**"
      - "docs/**"
      - "api/openapi.yaml"
      - "api/fixtures/**"
      - "database/**"
    blocked_paths:
      - "frontend/**"
      - "curra/**"
    
  manus:
    allowed_paths:
      - "frontend/**"
      - "contracts/**"
      - "docs/**"
      - "api/openapi.yaml"
      - "api/fixtures/**"
    blocked_paths:
      - "application/**"
      - "curra/**"
      - "database/**"

shared:
  require_approval:
    - "api/openapi.yaml"        # API changes need human review
    - "database/*.sql"          # Migration changes need human review
    - "curra/**"                # Any CURRA change needs human review
    - "contracts/**"            # Contract changes need human review
    - "docs/permissions.md"     # Auth changes need human review
```

This file is the declarative source of truth. CI workflows enforce it mechanically.

---

## 4. Workflow Rules

### API Changes
1. MIMO or Manus proposes an API change.
2. The change is described in a PR or issue.
3. Human approves the change.
4. `openapi.yaml` is updated.
5. Frontend re-generates API types.
6. Backend updates handlers.

### Database Migrations
1. MIMO drafts the migration SQL.
2. Human reviews and approves.
3. Migration is applied in a controlled deployment.
4. Schema.md is updated to reflect the current state.

### CURRA Changes
1. Any change to CURRA (solver, constraints, model) requires:
   - Human approval
   - Full test suite pass
   - Benchmark comparison
   - Contract review (does the adapter need updating?)

### Auth Changes
1. Any change to authentication, authorization, or permissions requires:
   - Human approval
   - Security review
   - Permissions.md update

### Contract Changes
1. Any change to contract documents requires:
   - Human approval
   - Both MIMO and Manus must acknowledge the change
   - API and schema updates if needed

---

## 5. Testing Boundaries

| Test type | Location | Owner |
|---|---|---|
| CURRA unit/integration | `curra/tests/` | CURRA team (frozen) |
| Backend unit tests | `application/**/*_test.go` | MIMO |
| Backend integration tests | `application/**/*_test.go` | MIMO |
| Frontend unit tests | `frontend/src/**/*.test.*` | Manus |
| E2E tests | `e2e/` | Human + both agents |
| Contract tests | `contracts/` fixtures | Both agents validate against |

### Contract Testing
- Frontend generates API types from `openapi.yaml`.
- Backend implements API conforming to `openapi.yaml`.
- Contract tests verify that backend responses match the OpenAPI schema.
- Both agents are responsible for keeping their implementations in sync with the contract.

---

## 6. Conflict Resolution

When MIMO and Manus need to change the same file (e.g., `openapi.yaml`):
1. One agent proposes the change.
2. Human reviews and applies.
3. Both agents are notified and update their implementations.

The contract documents (`docs/`, `contracts/`) are the source of truth for shared understanding. Neither agent should invent new semantics — they should reference the contract.
