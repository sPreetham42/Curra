# AI Contract

## 1. Core Principle

The AI system operates within a timetable workspace. It can read academic data and timetable state, but it can ONLY affect canonical data through the normal application path (create → validate → persist).

**AI never writes directly to canonical timetable data.**

---

## 2. Two Categories of AI Output

### ANSWER

Read-only responses to user questions. No side effects.

Examples:
- "How many sessions does CS101 have per week?"
- "Which rooms have projectors?"
- "Is Professor Smith available on Monday periods 1-3?"
- "Show me the current schedule for Class CS-Year2-A."
- "Why was the last solve infeasible?"

**Implementation:** The AI queries the application's read APIs and formats a natural language response. No data mutation occurs.

### PROPOSAL

Structured edit suggestions that require user confirmation before taking effect.

Examples:
- "Move CS101 Theory from Monday P1 to Wednesday P3."
- "Swap the rooms for the CS101 Lab and the Math101 Theory."
- "Add a session requirement for CS101: 2x1hr Theory per week."
- "Change Professor Smith's availability to include Friday afternoons."

**Implementation:** The AI returns a structured diff that the frontend renders for user review. The user must explicitly accept or reject. Acceptance follows the normal CURRA validation path.

---

## 3. PROPOSAL Structure

```json
{
  "type": "PROPOSAL",
  "description": "Move CS101 Theory from Monday Period 1 to Wednesday Period 3",
  "actions": [
    {
      "action": "MOVE_ASSIGNMENT",
      "target": {
        "assignmentId": "req-cs101-theory#0",
        "currentRoomId": "room-301",
        "currentTimeSlotId": "ts-mon-p1"
      },
      "proposed": {
        "roomId": "room-301",
        "timeSlotId": "ts-wed-p3"
      }
    }
  ],
  "explanation": "This moves the lecture to avoid a gap for CS-Year2-A on Wednesdays."
}
```

### Action Types

| Action | Description | Validation |
|---|---|---|
| `MOVE_ASSIGNMENT` | Move an assignment to new room/slot | CURRA `ValidateMove` |
| `SWAP_ASSIGNMENTS` | Swap placements of two assignments | CURRA `ValidateMove` (both directions) |
| `ADD_SESSION_REQUIREMENT` | Add a new session requirement | `problem.Validate` |
| `REMOVE_SESSION_REQUIREMENT` | Remove a session requirement | Check impact on existing assignments |
| `UPDATE_AVAILABILITY` | Change faculty/room availability | Application validation |
| `PIN_ASSIGNMENT` | Pin an assignment | Application validation |

---

## 4. Acceptance Flow

```
User accepts proposal
  → Backend receives acceptance
  → For each action:
      1. Load current state from database
      2. Apply the action to a clone
      3. Validate through CURRA adapter (ValidateMove or full Verify)
      4. If valid: persist as new ScheduleVersion draft
      5. If invalid: reject with explanation
  → Return result to frontend
```

The acceptance path is identical to a manual user edit. There is no special "AI path."

---

## 5. Backend API Shapes

### Send Chat Message
```
POST /timetables/:id/chat
{
  "message": "Move the CS101 lab to Thursday",
  "context": {
    "currentVersionId": "ver_abc",
    "selectedAssignmentId": "req-cs101-lab#0"  // optional
  }
}
```

### Response
```
{
  "data": {
    "type": "ANSWER" | "PROPOSAL",
    "message": "Natural language response",
    "proposal": { ... }  // only for PROPOSAL type
  }
}
```

### Accept Proposal
```
POST /chat/proposals/:id/accept
{
  "versionId": "ver_abc"  // target version to edit
}
```

### Reject Proposal
```
POST /chat/proposals/:id/reject
```

---

## 6. AI Context Window

The AI receives:
- The current timetable version's assignments
- The problem snapshot's academic data (departments, programs, classes, faculty, rooms, subjects)
- The chat history for the current session
- Any currently selected assignment in the UI

The AI does NOT receive:
- Internal solver state
- CURRA search diagnostics
- Other users' data
- Database internals

---

## 7. Safety Rules

1. **No direct writes:** AI proposals go through normal validation and persistence.
2. **User confirmation required:** Every proposal must be explicitly accepted.
3. **Audit trail:** AI proposals and acceptances are logged in audit_events.
4. **No autocomplete without consent:** The AI should not auto-apply proposals.
5. **Scoped to workspace:** AI only sees data for the current timetable workspace.
6. **Rate limiting:** AI chat requests are rate-limited per user.
7. **Cost control:** AI context is bounded to prevent excessive token usage.
