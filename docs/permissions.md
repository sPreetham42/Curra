# RBAC Permissions Contract

## 1. Role Definitions

| Role | Description | Typical User |
|---|---|---|
| `INSTITUTION_ADMIN` | Full control over the institution's data, users, and schedules. | Department head, scheduler lead |
| `SCHEDULER` | Can create/edit timetables, run solves, manage versions. | Dedicated scheduling staff |
| `PROFESSOR` | Can view and pin their own assignments, propose edits. | Teaching faculty |
| `VIEWER` | Read-only access to published and draft timetables. | Students, external reviewers |

A user has exactly one role per institution. Global superadmin role is outside this contract (infrastructure concern).

---

## 2. Permission Matrix

### Academic Data

| Resource | INSTITUTION_ADMIN | SCHEDULER | PROFESSOR | VIEWER |
|---|---|---|---|---|
| Departments | CRUD | Read | Read | Read |
| Programs | CRUD | Read | Read | Read |
| Classes | CRUD | Read | Read | Read |
| StudentGroups | CRUD | Read | Read | Read |
| Subjects | CRUD | Read | Read | Read |
| Faculty | CRUD | Read | Read | Read |
| Rooms | CRUD | Read | Read | Read |
| RoomFeatures | CRUD | Read | Read | Read |
| TimeSlots | CRUD | Read | Read | Read |
| CourseOfferings | CRUD | Read | Read (own) | Read |
| SessionRequirements | CRUD | Read | Read | Read |
| FacultyAvailability | CRUD (own faculty) | CRUD | Read (own) | Read |
| RoomAvailability | CRUD | CRUD | Read | Read |

### Timetables

| Action | INSTITUTION_ADMIN | SCHEDULER | PROFESSOR | VIEWER |
|---|---|---|---|---|
| Create timetable | ✓ | ✓ | ✗ | ✗ |
| Edit timetable name/settings | ✓ | ✓ | ✗ | ✗ |
| Create snapshot | ✓ | ✓ | ✗ | ✗ |
| Run solver | ✓ | ✓ | ✗ | ✗ |
| Cancel run | ✓ | ✓ (own) | ✗ | ✗ |
| View runs | ✓ | ✓ | Read (related) | Read (published) |
| Create draft version | ✓ | ✓ | ✗ | ✗ |
| Edit draft version | ✓ | ✓ | ✗ | ✗ |
| Submit for review | ✓ | ✓ | ✗ | ✗ |
| Review version | ✓ | ✗ | ✗ | ✗ |
| Publish version | ✓ | ✗ | ✗ | ✗ |
| Archive version | ✓ | ✗ | ✗ | ✗ |
| View published | ✓ | ✓ | ✓ | ✓ |
| View draft | ✓ | ✓ | ✗ | ✗ |
| Pin assignment | ✓ | ✓ | Own offerings | ✗ |
| Manual move/swap | ✓ | ✓ | ✗ | ✗ |
| Import data | ✓ | ✓ | ✗ | ✗ |
| Delete timetable | ✓ | ✗ | ✗ | ✗ |

### Users

| Action | INSTITUTION_ADMIN | SCHEDULER | PROFESSOR | VIEWER |
|---|---|---|---|---|
| List users in institution | ✓ | ✗ | ✗ | ✗ |
| Invite user | ✓ | ✗ | ✗ | ✗ |
| Change user role | ✓ | ✗ | ✗ | ✗ |
| Remove user | ✓ | ✗ | ✗ | ✗ |

### AI

| Action | INSTITUTION_ADMIN | SCHEDULER | PROFESSOR | VIEWER |
|---|---|---|---|---|
| Ask AI question | ✓ | ✓ | ✓ | ✗ |
| Accept AI proposal | ✓ | ✓ | ✗ | ✗ |

---

## 3. Ownership Rules

- **Professor sees own offerings only:** A Professor can view CourseOfferings where `FacultyID` matches their own Faculty record.
- **Professor can pin own assignments:** A Professor can pin/unpin assignments for offerings they teach.
- **Scheduler manages own runs:** A Scheduler can cancel runs they created.
- **Admin overrides:** INSTITUTION_ADMIN has access to everything in their institution.

---

## 4. Tenant Isolation

Every query is scoped by `institution_id`. No cross-institution data access is possible through the API layer. The JWT contains `institution_id` and `role`.

---

## 5. Implementation Notes

- Permissions are checked at the service layer, not in the database.
- The API middleware extracts user and institution from JWT.
- Service methods accept the authenticated user context and enforce permissions before any data access.
- Database has `institution_id` on every table for defense-in-depth.
