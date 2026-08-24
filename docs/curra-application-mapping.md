# CURRA Application Domain Mapping

This document establishes the precise domain model mapping between the high-level Application concepts (PostgreSQL schemas, REST APIs, Frontend UI) and the frozen CURRA Solver Core concepts.

## 1. Domain Concept Mapping Summary

| Application Concept | CURRA Core Concept | Key Identifiers / Fields | Structural Rationale |
| :--- | :--- | :--- | :--- |
| **Tenant / Institution** | `model.TenantID` | `TenantID string` | Isolates academic institutions in multi-tenant environments. |
| **Academic Term / Semester** | `model.Term` | `TermID`, `Name` | Scope for timetable generation (e.g. Fall 2026). |
| **Department / Faculty Division** | `model.Department` | `DepartmentID`, `Name` | Organizational grouping of programs and faculty. |
| **Academic Program / Major** | `model.Program` | `ProgramID`, `DepartmentID`, `Name` | Degree program (e.g. B.Tech Computer Science). |
| **Class / Student Cohort** | `model.Class` | `ClassID`, `WholeGroupID`, `StudentGroupIDs` | Academic year cohort (e.g. CS Year 2). Defines subgroup hierarchy. |
| **Student Group / Section / Lab Batch** | `model.StudentGroup` | `StudentGroupID`, `ClassID`, `Size` | Atomic student group for conflict checking & gap scoring. |
| **Subject / Course Catalog Item** | `model.Subject` | `SubjectID`, `Code`, `Name` | Academic subject catalog entry (e.g., CS101). |
| **Course Offering / Section Instance** | `model.CourseOffering` | `CourseOfferingID`, `SubjectID`, `FacultyID`, `StudentGroupID` | Concrete offering linking subject, professor, and student group. |
| **Session Requirement / Delivery Format** | `model.SessionRequirement` | `SessionRequirementID`, `Type`, `SessionsPerWeek`, `Duration` | Session structure (e.g. 2 x 1hr Theory or 1 x 2hr Lab per week). |
| **Professor / Instructor** | `model.Faculty` | `FacultyID`, `Name` | Instructor assigned to teach course offerings. |
| **Professor Unavailability** | `model.FacultyAvailability` | `FacultyID`, `TimeSlotID` | Allow-list of slots where faculty is available to teach. |
| **Professor Preferred Slot** | `model.FacultyPreference` | `FacultyID`, `TimeSlotID`, `Weight` | Soft preference weights for preferred teaching slots. |
| **Classroom / Laboratory** | `model.Room` | `RoomID`, `Capacity`, `FeatureIDs` | Room space with enrollment capacity and room feature tags. |
| **Room Unavailability** | `model.RoomAvailability` | `RoomID`, `TimeSlotID` | Allow-list of slots where room is available for scheduling. |
| **Special Room Feature / Tag** | `model.RoomFeature` | `RoomFeatureID`, `Name` | Required equipment or room attributes (e.g. Projector, High-End GPU Lab). |
| **Bell Schedule / Period Grid** | `model.TimeSlot` & `PeriodsPerDay` | `TimeSlotID`, `Day`, `Period`, `Label` | Recurring weekly grid definition. `PeriodsPerDay` defines daily bounds. |
| **Pinned / Fixed Schedule Item** | `problem.Assignment` in `LockedAssignments` | `AssignmentID`, `RoomID`, `TimeSlotID` | Fixed assignment that solver must preserve without mutating. |
| **Saved Timetable / Draft** | `problem.Solution` | `Assignments []Assignment`, `Score` | Scheduled solution payload. |

---

## 2. Structural & Mapping Nuances

### 2.1 Student Cohort vs. Student Subgroup Hierarchy
* **Application Concept**: A class (e.g., "CS-Section-A") consists of 40 students who take Theory classes together, but split into two 20-student lab batches ("CS-A1" and "CS-A2").
* **CURRA Representation**:
  - `model.Class` represents the parent cohort.
  - `WholeGroupID` points to the 40-student group ("CS-A-Whole").
  - `StudentGroupIDs` slice includes `WholeGroupID`, "CS-A1", and "CS-A2".
  - `BuildStudentGroupOverlaps()` automatically derives overlap: "CS-A-Whole" conflicts with "CS-A1" and "CS-A2", while "CS-A1" and "CS-A2" are treated as disjoint.

### 2.2 Course Offering vs. Session Requirement Split
* **Application Concept**: Course "CS101" requires 2 single-period Theory lectures per week AND 1 double-period Lab per week.
* **CURRA Representation**:
  - One `model.CourseOffering` (e.g., `offering-cs101`).
  - Two `model.SessionRequirement` records:
    1. `req-cs101-theory`: `Type: THEORY`, `SessionsPerWeek: 2`, `Duration: 1`.
    2. `req-cs101-lab`: `Type: LAB`, `SessionsPerWeek: 1`, `Duration: 2`.
  - The solver creates 2 `Assignment` instances for theory (`req-cs101-theory#0`, `req-cs101-theory#1`) and 1 for lab (`req-cs101-lab#0`).

### 2.3 Time Slots & Consecutive Duration Expansion
* **Application Concept**: A 2-period lab starting at Monday Period 1 occupies Monday Period 1 and Monday Period 2.
* **CURRA Representation**:
  - `model.TimeSlot` defines individual period units (`Day: Monday`, `Period: 1`).
  - `Problem.OccupiedSlotIDs(startSlot, duration)` resolves the sequential period IDs using `SlotsByDayPeriod[SlotKey{Day, Period + i}]`.
  - Durations must fit within the daily grid (`Period + duration - 1 <= PeriodsPerDay`).

### 2.4 Availability Models
* **Application Concept**: Professor or Room is unavailable at specific times (block-list model).
* **CURRA Representation**:
  - `FacultyAvailabilities` and `RoomAvailabilities` operate as **explicit allow-lists**.
  - If availability records are specified, a resource is available ONLY at the listed `TimeSlotID` values.
  - The Application layer must convert block-list unavailabilities into allow-list availability records before passing `Problem` to CURRA.
