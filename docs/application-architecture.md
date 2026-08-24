# Application Architecture

## 1. System Overview

```
┌──────────────────────────────────────────────────────┐
│                   BROWSER / UI                        │
│          (Manus-implemented React app)                │
└──────────────────────┬───────────────────────────────┘
                       │ REST API (JSON)
                       ▼
┌──────────────────────────────────────────────────────┐
│                   API LAYER                           │
│         (OpenAPI-generated types, auth middleware)     │
└──────────────────────┬───────────────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │ Academic │ │ Schedule │ │    AI    │
    │  Data    │ │  Engine  │ │  Chat    │
    │ Service  │ │ Service  │ │ Service  │
    └────┬─────┘ └────┬─────┘ └────┬─────┘
         │            │            │
         ▼            ▼            ▼
    ┌──────────────────────────────────────┐
    │          PostgreSQL Database          │
    │  (Academic data, snapshots, runs,    │
    │   versions, audit events)            │
    └──────────────────────────────────────┘
         │
         ▼
    ┌──────────────────────────────────────┐
    │         CURRA Adapter Boundary       │
    │    (Pure Go, stateless, no DB)       │
    └──────────────────┬───────────────────┘
                       │
                       ▼
    ┌──────────────────────────────────────┐
    │          CURRA Solver Core           │
    │  (Frozen — no application code here) │
    └──────────────────────────────────────┘
```

## 2. Layer Responsibilities

### API Layer
- Authentication (Google OAuth → JWT)
- Request validation
- OpenAPI-generated request/response types
- Rate limiting
- CORS

### Academic Data Service
- CRUD for Departments, Programs, Classes, StudentGroups, Subjects, Faculty, Rooms, RoomFeatures, TimeSlots, CourseOfferings, SessionRequirements, FacultyAvailability, FacultyAvailability, RoomAvailability
- Import pipeline (staging → validation → commit)
- Data integrity enforcement

### Schedule Engine Service
- ProblemSnapshot creation from current academic data
- ScheduleRun lifecycle (QUEUED → RUNNING → SOLVED/FAILED)
- ScheduleVersion lifecycle (DRAFT → REVIEW → PUBLISHED → ARCHIVED)
- Assignment management within versions
- Manual edit validation via CURRA adapter
- Optimistic concurrency control

### AI Chat Service
- Natural language understanding
- Read-only queries (ANSWER)
- Structured edit proposals (PROPOSAL)
- Never writes directly to canonical data

## 3. Integration Boundaries

### CURRA Adapter (Go)
The ONLY way the application talks to CURRA:

```
CurraAdapter.Solve(ctx, Request) → (Response, error)
CurraAdapter.Verify(ctx, Problem, Solution, VerifyOptions) → (Report, error)
CurraAdapter.ValidateMove(ctx, Problem, Solution, Move, VerifyOptions) → (Report, Score, error)
```

The adapter is implemented as a Go package imported by the Schedule Engine Service. It has NO database access. It is pure and stateless.

### API Contract (OpenAPI)
Frontend communicates with backend exclusively through the OpenAPI-defined REST API. Types are generated from the spec. No hand-written API types.

### Database Contract
PostgreSQL with optimistic locking. No ORMs — direct SQL with a thin query layer. Migrations are version-controlled and require human approval.

## 4. Key Design Decisions

### Snapshot-First Architecture
Every solve operates on an immutable ProblemSnapshot, not live data. This ensures:
- Historical solves are reproducible
- Concurrent data edits don't corrupt running solves
- Audit trail is self-contained

### Version-Based Editing
Timetable versions are immutable. Editing creates a new version from the current one (copy-on-write). Published versions are never mutated.

### Stateless CURRA
CURRA adapter calls are stateless. The adapter creates Problem objects from snapshots, calls CURRA, and returns results. No solver state persists between calls.

### Separation of Concerns
- Academic data changes do not affect saved schedules
- Schedule solving does not block data editing
- AI proposals follow the same validation path as manual edits
