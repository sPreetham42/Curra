# Snapshot & Reproducibility Contract

## 1. ProblemSnapshot Contract

A ProblemSnapshot is the immutable, self-contained record of everything CURRA needs to solve a timetable. Historical snapshots must NEVER depend on mutable live academic records.

### 1.1 Snapshot Structure

```json
{
  "snapshotId": "snap_abc123",
  "schemaVersion": 1,
  "createdAt": "2026-08-24T10:30:00Z",
  "createdBy": "user_xyz",
  "timetableId": "tt_001",
  "inputHash": "sha256:a1b2c3...",
  "problem": {
    "tenantId": "inst_001",
    "term": { "id": "term_fall2026", "tenantId": "inst_001", "name": "Fall 2026" },
    "periodsPerDay": 8,
    "departments": { ... },
    "programs": { ... },
    "classes": { ... },
    "studentGroups": { ... },
    "subjects": { ... },
    "courseOfferings": { ... },
    "sessionRequirements": { ... },
    "faculty": { ... },
    "facultyAvailabilities": [ ... ],
    "facultyPreferences": [ ... ],
    "rooms": { ... },
    "roomAvailabilities": [ ... ],
    "roomFeatures": { ... },
    "timeSlots": { ... },
    "lockedAssignments": [ ... ]
  },
  "constraintInstances": [ ... ],
  "solverConfig": {
    "searchMode": "HEURISTIC_LCV",
    "maxNodes": 100000,
    "tabuMaxIterations": 1000,
    "tabuTenure": 10,
    "seed": 42,
    "disableOptimize": false
  },
  "objectiveConfig": {
    "components": [
      { "id": "StudentGapPenalty", "weight": 1 }
    ]
  }
}
```

### 1.2 Snapshot Rules

1. **Self-contained:** The snapshot contains ALL data CURRA needs. No external references.
2. **Immutable:** Once created, a snapshot is never modified. Create a new one if data changes.
3. **Canonical JSON:** The `problem` field serializes directly to `problem.Problem` via Go JSON.
4. **Input hash:** SHA-256 of the canonical JSON of `problem + constraintInstances + solverConfig + objectiveConfig`. Used for deduplication.
5. **Schema version:** Monotonically increasing integer. Required for forward-compatible deserialization.
6. **Snapshot tables (normalized) vs. canonical JSON:** We use **canonical JSON** stored in a single `problem` JSONB column.

### 1.3 Why Canonical JSON

The `problem.Problem` struct already IS the canonical representation. CURRA expects exactly this struct. Storing normalized tables would require a bidirectional mapping layer that adds complexity with no benefit — CURRA cannot consume anything other than its own struct. JSONB in PostgreSQL gives us queryability on specific fields when needed while preserving serialization fidelity.

---

## 2. Reproducibility Contract

### 2.1 What "Reproduce" Means

Given identical inputs, CURRA produces identical outputs. This requires:

| Factor | Required for exact reproducibility | Notes |
|---|---|---|
| Problem (academic data) | ProblemSnapshot — frozen | Deterministic from snapshot |
| Constraint instances | ProblemSnapshot — frozen | Deterministic from snapshot |
| Solver config | ProblemSnapshot — frozen | Includes search mode, limits |
| Objective config | ProblemSnapshot — frozen | Includes weights |
| Seed | ProblemSnapshot — frozen | Controls Tabu Search RNG |
| CURRA version | ScheduleRun — recorded | CURRA is frozen; same version = same behavior |
| CURRA git commit | ScheduleRun — recorded | Exact code provenance |
| RuleSetHash | Computed during solve | SHA-256 of compiled constraints |
| Snapshot schema version | ProblemSnapshot — recorded | For deserialization compatibility |
| Timezone / calendar | ProblemSnapshot — embedded | TimeSlot Day/Period definitions |

### 2.2 Reproduce Operation

```
REPRODUCE(snapshotId, solverConfig?) → ScheduleRun
```

1. Load the ProblemSnapshot by ID.
2. Deserialize `problem.Problem` from the `problem` JSONB field.
3. Use the solver config from the snapshot (or override with provided config).
4. Execute `engine.Solve()` with the same seed.
5. Returns identical solution (same assignments, same score).

**Guarantee:** If CURRA version has not changed, REPRODUCE returns an identical result to the original run.

### 2.3 VERIFY Operation

```
VERIFY(snapshotId, solutionId) → VerificationReport
```

1. Load the ProblemSnapshot.
2. Load the ScheduleVersion's assignments.
3. Deserialize both into `problem.Problem` and `problem.Solution`.
4. Call `verifier.VerifySolution()`.
5. Returns independent verification report without running the solver.

This operation proves that a stored solution is valid against the snapshot it was created from, without re-solving.

### 2.4 AUDIT Operation

```
AUDIT(runId) → AuditReport
```

Returns a complete audit trail for a ScheduleRun:
- Snapshot ID and content hash
- Solver configuration
- Solver status and diagnostics (nodes explored, backtracks, candidates)
- CURRA version and commit
- Duration
- All violations encountered during search
- Final verification result
- Timeline of status changes

### 2.5 Version Compatibility

| Scenario | Supported? | Notes |
|---|---|---|
| Same CURRA version, same snapshot | ✓ Exact reproducibility | Deterministic |
| Same CURRA commit, same snapshot | ✓ Exact reproducibility | Same code |
| Different CURRA version, same snapshot | ⚠ Best-effort | Results may differ if algorithms changed. Record version mismatch in audit. |
| Snapshot schema version mismatch | ✗ Reject | Cannot deserialize incompatible snapshots. |

**Policy:** The application MUST NOT claim exact reproducibility across CURRA versions. It CAN claim reproducibility within the same version. The audit report should always record the CURRA version used.

---

## 3. Snapshot Lifecycle

1. **Creation:** When a user clicks "Generate Timetable" or creates a ScheduleRun, the application creates a ProblemSnapshot from the current state of the academic data.
2. **Storage:** Snapshot is stored as a row with the canonical Problem JSON.
3. **Reference:** ScheduleRuns and ScheduleVersions reference the snapshot by ID.
4. **Retention:** Snapshots are retained indefinitely for audit trail.
5. **Garbage collection:** Snapshots are NEVER garbage collected. They are the historical record.
