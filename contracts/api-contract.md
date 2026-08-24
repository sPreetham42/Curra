# API Contract

This document defines the principal REST resources and operations. The full OpenAPI spec (`api/openapi.yaml`) is the machine-readable source of truth.

## 1. Resource Model

### Authentication

| Resource | Methods | Notes |
|---|---|---|
| `POST /auth/google` | Exchange Google OAuth code for JWT | Returns access + refresh tokens |
| `POST /auth/refresh` | Refresh JWT | |
| `GET /auth/me` | Current user profile | |

### Institutions

| Resource | Methods | Notes |
|---|---|---|
| `GET /institutions/:id` | Read institution | |
| `PATCH /institutions/:id` | Update institution | Admin only |
| `GET /institutions/:id/users` | List users | Admin only |
| `POST /institutions/:id/users/invite` | Invite user | Admin only |
| `PATCH /institutions/:id/users/:userId/role` | Change role | Admin only |

### Academic Data

All under `/institutions/:instId/`:

| Resource | Methods | Notes |
|---|---|---|
| `/departments` | CRUD | |
| `/programs` | CRUD | FK to department |
| `/classes` | CRUD | FK to program |
| `/student-groups` | CRUD | FK to class |
| `/subjects` | CRUD | |
| `/faculty` | CRUD | |
| `/rooms` | CRUD | |
| `/room-features` | CRUD | |
| `/time-slots` | CRUD | |
| `/course-offerings` | CRUD | FK to term, class, subject, faculty, student-group |
| `/session-requirements` | CRUD | FK to course-offering |
| `/faculty-availability` | CRUD | FK to faculty |
| `/faculty-preferences` | CRUD | FK to faculty |
| `/room-availability` | CRUD | FK to room |

### Timetables

| Resource | Methods | Notes |
|---|---|---|
| `GET /timetables` | List | Filtered by institution |
| `POST /timetables` | Create | |
| `GET /timetables/:id` | Read | |
| `PATCH /timetables/:id` | Update name/settings | |
| `DELETE /timetables/:id` | Delete | Admin only, cascades |

### Snapshots

| Resource | Methods | Notes |
|---|---|---|
| `POST /timetables/:id/snapshots` | Create from current data | Freezes current academic data |
| `GET /timetables/:id/snapshots` | List snapshots | |
| `GET /snapshots/:id` | Read snapshot | Includes canonical problem JSON |
| `GET /snapshots/:id/problem` | Download problem JSON | For debugging / external tools |

### Schedule Runs

| Resource | Methods | Notes |
|---|---|---|
| `POST /timetables/:id/runs` | Create run | Requires snapshot_id. Enqueues solve. |
| `GET /timetables/:id/runs` | List runs | |
| `GET /runs/:id` | Read run | Includes status, diagnostics, result |
| `POST /runs/:id/cancel` | Cancel run | If QUEUED or RUNNING |
| `GET /runs/:id/verify` | Re-verify result | Independent verification |

### Schedule Versions

| Resource | Methods | Notes |
|---|---|---|
| `POST /timetables/:id/versions` | Create draft | From run result or blank |
| `GET /timetables/:id/versions` | List versions | |
| `GET /versions/:id` | Read version | Includes assignments and score |
| `PATCH /versions/:id` | Update draft | Name only |
| `POST /versions/:id/submit-review` | DRAFT → REVIEW | |
| `POST /versions/:id/publish` | REVIEW → PUBLISHED | Admin only |
| `POST /versions/:id/send-back` | REVIEW → DRAFT | Admin only |
| `POST /versions/:id/archive` | → ARCHIVED | |
| `GET /versions/:id/assignments` | List assignments in version | |
| `GET /versions/:id/timeline` | View as timetable grid | Structured for UI |

### Assignments (within a version)

| Resource | Methods | Notes |
|---|---|---|
| `POST /versions/:id/assignments/move` | Move assignment | Validated by CURRA |
| `POST /versions/:id/assignments/swap` | Swap two assignments | Validated by CURRA |
| `POST /versions/:id/assignments/pin` | Pin assignment | Creates AssignmentPin |
| `DELETE /versions/:id/assignments/pins/:pinId` | Unpin | |

### Import

| Resource | Methods | Notes |
|---|---|---|
| `POST /timetables/:id/import` | Upload file | Returns import_batch_id |
| `GET /import-batches/:id` | Read batch status | |
| `GET /import-batches/:id/rows` | Preview rows | Before commit |
| `POST /import-batches/:id/commit` | Commit valid rows | Merges into academic data |
| `POST /import-batches/:id/cancel` | Cancel import | |

### AI Chat

| Resource | Methods | Notes |
|---|---|---|
| `POST /timetables/:id/chat` | Send message | Returns ANSWER or PROPOSAL |
| `POST /chat/proposals/:id/accept` | Accept proposal | Creates draft version |
| `POST /chat/proposals/:id/reject` | Reject proposal | Discards |

### Verification

| Resource | Methods | Notes |
|---|---|---|
| `POST /verify` | Verify a version against a snapshot | Uses snapshotId + versionId |

---

## 2. Common Response Shapes

### Success
```json
{
  "data": { ... }
}
```

### Error
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human-readable message",
    "details": [
      { "field": "name", "message": "Required" }
    ]
  }
}
```

### Pagination
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "total": 142,
    "totalPages": 8
  }
}
```

### Optimistic Lock Conflict
HTTP 409:
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "Resource was modified by another request. Please refresh and retry.",
    "currentVersion": 5,
    "yourVersion": 3
  }
}
```

---

## 3. Error Codes

| HTTP | Code | Meaning |
|---|---|---|
| 400 | VALIDATION_ERROR | Request body validation failed |
| 401 | UNAUTHORIZED | Not authenticated |
| 403 | FORBIDDEN | Authenticated but insufficient permissions |
| 404 | NOT_FOUND | Resource does not exist |
| 409 | CONFLICT | Optimistic lock conflict |
| 422 | UNPROCESSABLE | Business logic error (e.g., invalid state transition) |
| 429 | RATE_LIMITED | Too many requests |
| 500 | INTERNAL_ERROR | Unexpected server error |

---

## 4. Versioning

The API is versioned via URL prefix: `/api/v1/...`.

Breaking changes require a new version (v2). Additive changes (new fields, new endpoints) are made within the current version.
