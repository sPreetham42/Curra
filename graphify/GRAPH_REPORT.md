# Graph Report - timetable-platform  (2026-09-13)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 2178 nodes · 8037 edges · 110 communities (71 shown, 32 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 242 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7d9fec94`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- github.com/google/uuid.UUID
- Handlers
- services/slice_test.go
- GenerateSyntheticProblem
- NewSolution
- ScheduleAssignment
- testing.T
- types.go
- time.Time
- Timetable
- ProblemSnapshot
- New
- Violation
- constraints_test.go
- IncrementalScoreEvaluator
- NewSearchCtx
- Assignment
- setupTestRouter
- backtracking.go
- Repos
- EvaluateMove
- Worker
- Problem
- ConstraintInstance
- RoomID
- FeasibleProblem
- ScoreBreakdown
- types.ts
- User
- phase2_test.go
- .Check
- Validate
- invariants_test.go
- LocalSearchTestProblem
- FacultyID
- .Search
- TimeSlotID
- compilerOptions
- EngineSnapshot
- encoding/json.RawMessage
- services_test.go
- context.Context
- CheckAll
- compilerOptions
- concurrency_test.go
- AuditEvent
- .CreateSnapshot
- Solution
- RoomFeature
- ScheduleRun
- package.json
- ScheduleVersion
- resource_test.go
- CourseOfferingID
- RoomFeatureCompatibilityConstraint
- SearchCtx
- setupSliceRouter
- MemberRef
- hardening_invariants_test.go
- Room
- buildRoomChangeTestProblem
- devDependencies
- react
- New
- .Solve
- StudentGroupID
- App
- scheduleRunRepo
- indexAssignment
- SolveJobView.tsx
- SessionRequirement
- txRunRepo
- hRun
- mockRunRepo
- PreSolve
- sliceRunRepo
- CourseOffering
- scheduleVersionRepo
- TabuList
- mockVersionRepo
- FacultyAvailability
- FacultyPreference
- Faculty
- academic.go
- TimeSlot
- github.com/jackc/pgx/v5/pgxpool.Pool
- ScheduleVersionStatus
- txVersionRepo
- mockVersionRepo
- mockVersionRepo
- p2Ver
- sliceVersionRepo
- AcademicYear
- Class
- Program
- StudentGroup
- Subject
- Term
- sampleProblem
- academic_repos.go
- tsconfig.json
- solver.go
- github.com/sPreetham42/timetable-platform

## God Nodes (most connected - your core abstractions)
1. `Problem` - 191 edges
2. `Solution` - 122 edges
3. `RequireTenantMatch()` - 100 edges
4. `Violation` - 94 edges
5. `Compile()` - 91 edges
6. `GenerateSyntheticProblem()` - 90 edges
7. `Handlers` - 81 edges
8. `NewSolution()` - 76 edges
9. `writeError()` - 75 edges
10. `handleError()` - 74 edges

## Surprising Connections (you probably didn't know these)
- `TestITC2007_BenchmarkExecutionAndQuality()` --calls--> `New()`  [EXTRACTED]
  tests/benchmark_itc2007_test.go → internal/scheduler/solver/backtracking/backtracking.go
- `TestHardening_CSP_FailurePaths()` --calls--> `New()`  [EXTRACTED]
  tests/hardening_invariants_test.go → internal/scheduler/solver/backtracking/backtracking.go
- `TestHardening_DeterministicReplay()` --calls--> `New()`  [EXTRACTED]
  tests/hardening_invariants_test.go → internal/scheduler/solver/backtracking/backtracking.go
- `TestHardening_FinalFeasibilityGuarantee()` --calls--> `New()`  [EXTRACTED]
  tests/hardening_invariants_test.go → internal/scheduler/solver/backtracking/backtracking.go
- `BenchmarkCSPSolve_Medium_Basic()` --calls--> `New()`  [EXTRACTED]
  tests/performance_baseline_test.go → internal/scheduler/solver/backtracking/backtracking.go

## Import Cycles
- None detected.

## Communities (110 total, 32 thin omitted)

### Community 0 - "github.com/google/uuid.UUID"
Cohesion: 0.07
Nodes (5): CatalogService, RequireRole(), RequireTenantMatch(), github.com/google/uuid.UUID, SchedulingService

### Community 1 - "Handlers"
Cohesion: 0.14
Nodes (11): Handlers, handleError(), parseIfMatch(), writeError(), writeJSON(), Handlers, InstitutionFromContext(), InstitutionIDFromContext() (+3 more)

### Community 2 - "services/slice_test.go"
Cohesion: 0.06
Nodes (57): buildEngineSnapshot(), canonicalDiagnosticsFromAdapter(), ComputeInputHash(), engineVersionOr(), Execute(), mapToCanonicalResult(), parseAssignmentsFromEngine(), runIndependentVerification() (+49 more)

### Community 3 - "GenerateSyntheticProblem"
Cohesion: 0.10
Nodes (51): CompileErrors, TestCLI_EngineIntegration(), TestEngine_ConstraintLifecycle_SingleSourceOfTruth(), TestEngine_ContextDeadline_Contract(), TestEngine_ContextDeadline_DuringCSP(), TestEngine_DeterminismAcrossSeeds(), TestEngine_NoDuplicateDerivedIndexState(), TestEngine_RepeatedPrepareSafety() (+43 more)

### Community 4 - "NewSolution"
Cohesion: 0.10
Nodes (49): NewAssignmentID(), NewSolution(), createBasicTestProblem(), TestFacultyPreference_AuthoritativeVerifierParity(), TestFacultyPreference_Determinism(), TestFacultyPreference_HardConstraintIsolation(), TestFacultyPreference_IncrementalDeltaEquivalence(), TestFacultyPreference_MultiPeriodSession() (+41 more)

### Community 5 - "ScheduleAssignment"
Cohesion: 0.06
Nodes (9): scheduleAssignmentRepo, ScheduleAssignment, txAssignmentRepo, hAsn, mockAssignmentRepo, mockAssignmentRepo, p2Asn, sliceAssignmentRepo (+1 more)

### Community 6 - "testing.T"
Cohesion: 0.13
Nodes (45): TestCurraImportBoundary(), TestAdapterImplementsEngineV1(), TestCapabilities(), TestCapabilitiesMatchesEnginePackage(), TestCompatibilityShim(), testing.T, getSolvedSmallInstance(), TestEngine_Verifier_DuplicateAssignmentID() (+37 more)

### Community 7 - "types.go"
Cohesion: 0.09
Nodes (22): SolverCapabilities, CompileRequest, CompileResponse, SolveRequest, SolveResponse, ValidateMoveRequest, ValidateMoveResponse, ValidateSwapRequest (+14 more)

### Community 8 - "time.Time"
Cohesion: 0.06
Nodes (22): departmentRepo, Department, IdempotencyKey, MoveDTO, SchedulingConfig, SwapDTO, buildSolutionJSON(), MoveSwapService (+14 more)

### Community 9 - "Timetable"
Cohesion: 0.07
Nodes (8): timetableRepo, Timetable, txTimetableRepo, hTbl, mockTimetableRepo, mockTimetableRepo, p2Tbl, sliceTimetableRepo

### Community 10 - "ProblemSnapshot"
Cohesion: 0.07
Nodes (12): ProblemSnapshot, newMockWorkerRepos(), TestWorker_ClaimAndExecute(), TestWorker_RetryCeilingReachesFailed(), TestWorker_StaleWorkerRejected(), txSnapshotRepo, mockSnapshotRepo, snapshotRepo (+4 more)

### Community 11 - "New"
Cohesion: 0.13
Nodes (40): lockedBaseProblem(), TestLockedAssignments_ContributeToStudentGapPenalty(), TestLockedAssignments_ExcludedFromSearchVariables(), TestLockedAssignments_FullyLockedRequirement(), TestLockedAssignments_LockedAvailabilityViolation(), TestLockedAssignments_LockedCapacityViolation(), TestLockedAssignments_LockedRoomFeatureViolation(), TestLockedAssignments_LockedSessionsExceedingRequirementCount() (+32 more)

### Community 12 - "Violation"
Cohesion: 0.11
Nodes (10): FacultyAvailabilityConstraint, RoomAvailabilityConstraint, Severity, joinTimeSlotIDs(), NewFacultyAvailabilityConstraint(), NewRoomAvailabilityConstraint(), SolveStatus, Violation (+2 more)

### Community 13 - "constraints_test.go"
Cohesion: 0.12
Nodes (39): compareStudentGroupViolationSets(), compareViolationSets(), containsField(), normalizeStudentGroupViolations(), normalizeViolations(), TestCompile_Atomicity(), TestCompile_SoftConstraintsRejected(), TestCompile_StrictValidation() (+31 more)

### Community 14 - "IncrementalScoreEvaluator"
Cohesion: 0.11
Nodes (18): math/rand.Rand, Placement, ApplyCandidateMove(), EvaluateCandidateMove(), SingleMove(), SwapMove(), UndoCandidateMove(), calcDayRoomChanges() (+10 more)

### Community 15 - "NewSearchCtx"
Cohesion: 0.10
Nodes (35): testing.B, BenchmarkFacultyAvailability_CompiledFullSolutionEvaluate(), BenchmarkFacultyAvailability_CompiledIsConsistent(), BenchmarkFacultyAvailability_CompiledViolatedByMove(), BenchmarkRoomAvailability_CompiledFullSolutionEvaluate(), BenchmarkRoomAvailability_CompiledIsConsistent(), BenchmarkRoomAvailability_CompiledViolatedByMove(), BenchmarkRoomCapacity_CompiledFullSolutionEvaluate() (+27 more)

### Community 16 - "Assignment"
Cohesion: 0.13
Nodes (5): RoomConflictConstraint, StudentGroupConflictConstraint, NewRoomConflictConstraint(), Assignment, AssignmentID

### Community 17 - "setupTestRouter"
Cohesion: 0.12
Nodes (24): main(), New(), newMockRepos(), setupTestRouter(), TestConflict_VersionMismatch(), TestHealthEndpoint(), TestPreconditionRequired_MissingIfMatch(), TestTenantIsolation_CrossTenantReturns404() (+16 more)

### Community 18 - "backtracking.go"
Cohesion: 0.17
Nodes (24): candidate, decision, Solver, Request, Constraint, CompiledConstraintSet, SolveOptions, buildAssignment() (+16 more)

### Community 19 - "Repos"
Cohesion: 0.12
Nodes (30): AcademicYearRepo, AuditEventRepo, ClassRepo, CourseOfferingRepo, DepartmentRepo, EngineSnapshotRepo, FacultyAvailabilityRepo, FacultyPreferenceRepo (+22 more)

### Community 20 - "EvaluateMove"
Cohesion: 0.19
Nodes (28): EvaluateMove(), BenchmarkEvaluateMove(), copyReqCountMap(), copyResourceMap(), K, V, localSearchTestProblem(), TestApplyUndo_ExactStateEquality() (+20 more)

### Community 21 - "Worker"
Cohesion: 0.11
Nodes (11): main(), getEnv(), LoadConfig(), loadDotEnv(), New(), calculateLeaseDuration(), New(), Config (+3 more)

### Community 22 - "Problem"
Cohesion: 0.10
Nodes (7): legacyFacultyAvailabilityOracle, legacyRoomAvailabilityOracle, legacyRoomCapacityOracle, legacyRoomConflictOracle, legacyRoomFeatureCompatibilityOracle, joinTimeSlotIDs(), Problem

### Community 23 - "ConstraintInstance"
Cohesion: 0.13
Nodes (12): ConstraintDef, ConstraintKind, RoomCapacityConstraint, NewFacultyConflictConstraint(), compileInstanceDef(), CompileError, ConstraintInstance, validateInstance() (+4 more)

### Community 24 - "RoomID"
Cohesion: 0.13
Nodes (20): joinRoomFeatureIDs(), FacultyPreference, Room, RoomFeature, SessionRequirement, SessionType, StudentGroup, ClassID (+12 more)

### Community 25 - "FeasibleProblem"
Cohesion: 0.13
Nodes (25): FacultyAvailability, RoomAvailability, captureDerivedState(), TestPrepare_AvailabilityAddedAfterPrepare_IsIndexed(), TestPrepare_AvailabilityRemovedAfterPrepare_IsDropped(), TestPrepare_RepeatedPrepare_EquivalentToFreshPrepare(), TestPrepare_SlotIndexDeterministicUnderDuplicateGridKeys(), TestPrepare_SlotMovedAfterPrepare_DropsStaleGridEntry() (+17 more)

### Community 26 - "ScoreBreakdown"
Cohesion: 0.20
Nodes (16): CalculateFacultyPreferencePenaltyWithConfig(), CalculateRoomChangePenaltyWithConfig(), CalculateStudentGapPenalty(), CalculateStudentGapPenaltyWithConfig(), DefaultObjectiveConfig(), ObjectiveConfig, ScoreBreakdown, calculateIndependentScore() (+8 more)

### Community 27 - "types.ts"
Cohesion: 0.13
Nodes (24): HTTPError, request(), APIError, AuthMeResponse, Institution, MoveDTO, MoveResponse, ObjectiveComponentScore (+16 more)

### Community 28 - "User"
Cohesion: 0.11
Nodes (12): institutionRepo, userRepo, userRoleRepo, Institution, Role, User, UserRole, ContextWithInstitution() (+4 more)

### Community 29 - "phase2_test.go"
Cohesion: 0.20
Nodes (24): ctxWithInstitutionP2(), makePhase2Snapshot(), newPhase2Mem(), ptr(), setupPhase2Fixture(), TestPhase2_CancelSolveJob_InvalidState(), TestPhase2_CancelSolveJob_QueuedToCancelled(), TestPhase2_ConcurrentClaim_AtomicViaSKIPLOCKED() (+16 more)

### Community 30 - ".Check"
Cohesion: 0.12
Nodes (23): legacyStudentGroupConflictOracle, BenchmarkFacultyAvailability_LegacyFullSolutionCheck(), BenchmarkRoomAvailability_LegacyFullSolutionCheck(), BenchmarkRoomCapacity_LegacyFullSolutionCheck(), BenchmarkRoomConflict_LegacyFullSolutionCheck(), BenchmarkRoomFeatureCompatibility_LegacyFullSolutionCheck(), BenchmarkStudentGroupConflict_LegacyFullSolutionCheck(), compareCapacityViolationSets() (+15 more)

### Community 31 - "Validate"
Cohesion: 0.19
Nodes (25): Class, catalogKeyMismatchViolation(), classContainsGroup(), classContainsListedGroup(), Problem, hasClass(), hasCourseOffering(), hasDepartment() (+17 more)

### Community 32 - "invariants_test.go"
Cohesion: 0.17
Nodes (24): testing.TB, buildInvariantTestSolution(), cloneAssignments(), copyReqCountMap(), copyResourceMap(), K, V, requirementCountEqual() (+16 more)

### Community 33 - "LocalSearchTestProblem"
Cohesion: 0.17
Nodes (23): TestFacultyAvailability_InTabu(), TestRoomAvailability_InTabu(), TestRoomCapacity_InTabu(), TestRoomConflict_InTabu(), TestRoomFeatureCompatibility_InTabu(), TestStudentGroupConflict_InTabu(), BenchmarkTabuSearch_MediumProblem(), TabuSearch() (+15 more)

### Community 34 - "FacultyID"
Cohesion: 0.15
Nodes (21): Department, Faculty, Program, Term, DepartmentID, FacultyID, ProgramID, TenantID (+13 more)

### Community 35 - ".Search"
Cohesion: 0.13
Nodes (18): baseViolation(), DefaultHardConstraints(), DefaultScopedValidators(), ScopedValidator, invalidDurationViolation(), joinRoomFeatureIDs(), assertNoHardConstraintViolations(), NewNeighborhoodGenerator() (+10 more)

### Community 36 - "TimeSlotID"
Cohesion: 0.21
Nodes (10): TimeSlotID, Problem, BuildIndexFromAssignments(), facultyKey(), Problem, groupKey(), NewSolutionIndex(), roomKey() (+2 more)

### Community 37 - "compilerOptions"
Cohesion: 0.08
Nodes (23): DOM, src, vite/client, compilerOptions, allowArbitraryExtensions, allowImportingTsExtensions, erasableSyntaxOnly, jsx (+15 more)

### Community 38 - "EngineSnapshot"
Cohesion: 0.11
Nodes (5): engineSnapshotRepo, EngineSnapshot, mockRepos, mockEngineSnapshotRepo, mockEngineSnapshotRepo

### Community 39 - "encoding/json.RawMessage"
Cohesion: 0.18
Nodes (3): ScheduleRunStatus, calculateDeadline(), encoding/json.RawMessage

### Community 40 - "services_test.go"
Cohesion: 0.33
Nodes (17): buildTestContext(), buildTestRepos(), newMockRepos(), TestIdempotency_AtomicDeduplication(), TestOptimisticLocking_VersionConflict(), TestPhase3_EngineSnapshot_GetByID(), TestPhase3_EngineSnapshot_GetByID_NotFound(), TestPhase3_SolveJobIdempotency_ConcurrentSameKey() (+9 more)

### Community 41 - "context.Context"
Cohesion: 0.17
Nodes (3): context.Context, mockRunRepo, p2Run

### Community 42 - "CheckAll"
Cohesion: 0.41
Nodes (13): CheckAll(), Diagnostics, buildCSPAssignment(), buildCSPDecisions(), buildCSPInitialDomains(), orderValuesByLCV(), pruneCSPDomains(), selectMRVVar() (+5 more)

### Community 43 - "compilerOptions"
Cohesion: 0.10
Nodes (19): node, vite.config.ts, compilerOptions, allowImportingTsExtensions, erasableSyntaxOnly, lib, module, moduleDetection (+11 more)

### Community 44 - "concurrency_test.go"
Cohesion: 0.32
Nodes (17): buildContext(), buildTxTestRepos(), newTxMemoryDB(), TestConcurrency_IdempotencyRace(), TestConcurrency_MoveRace(), TestConcurrency_PublishRace(), TestConcurrency_StaleWorkerRejection(), TestConcurrency_SwapRace() (+9 more)

### Community 45 - "AuditEvent"
Cohesion: 0.14
Nodes (7): AuditEvent, transactionalMemoryDB, txAuditRepo, hAud, mockAuditRepo, mockAuditRepo, mockRepos

### Community 46 - ".CreateSnapshot"
Cohesion: 0.20
Nodes (17): SnapshotService, mapFromClasses(), mapFromCOs(), mapFromDepts(), mapFromFA(), mapFromFac(), mapFromFP(), mapFromProgs() (+9 more)

### Community 47 - "Solution"
Cohesion: 0.20
Nodes (6): FacultyConflictConstraint, Response, Solution, Score, mockScoreEvaluator, ScoreBreakdown

### Community 48 - "RoomFeature"
Cohesion: 0.15
Nodes (4): roomFeatureAssignmentRepo, roomFeatureRepo, RoomFeature, mockRoomFeatureRepo

### Community 49 - "ScheduleRun"
Cohesion: 0.18
Nodes (3): ScheduleRun, time.Duration, mockRunRepo

### Community 50 - "package.json"
Cohesion: 0.12
Nodes (16): lucide-react, react, react-dom, dependencies, lucide-react, react, react-dom, name (+8 more)

### Community 51 - "ScheduleVersion"
Cohesion: 0.20
Nodes (3): ScheduleVersion, VersionService, hVer

### Community 52 - "resource_test.go"
Cohesion: 0.42
Nodes (13): NewCatalogService(), buildResourceRepos(), newResourceMockRepos(), resourceCtx(), TestPhase4B_RBAC_RoomManagement(), TestPhase4B_Room_CRUD(), TestPhase4B_Room_VersionConflict(), TestPhase4B_RoomFeature_CRUD() (+5 more)

### Community 53 - "CourseOfferingID"
Cohesion: 0.23
Nodes (13): CourseOffering, Subject, CourseOfferingID, SubjectID, sortedCourseOfferingIDs(), sortedSubjectIDs(), sortedOfferings(), stressBaseProblem() (+5 more)

### Community 56 - "setupSliceRouter"
Cohesion: 0.36
Nodes (12): authedRequest(), getFirstSnapshot(), newSliceHTTPMem(), setupSliceRouter(), TestHTTP_SolveJob_Deterministic(), TestHTTP_SolveJob_EndToEnd_WithRealEngineV1(), testLogger(), NewSliceService() (+4 more)

### Community 57 - "MemberRef"
Cohesion: 0.22
Nodes (7): hierarchyMemberSet, HierarchyMembershipIndex, MemberRef, MemberSet, MembershipIndex, NewHierarchyMembershipIndex(), TestMembershipIndex_HierarchyBackedBehavior()

### Community 58 - "hardening_invariants_test.go"
Cohesion: 0.24
Nodes (13): assertSolutionIndexConsistent(), containsSubstring(), hasViolationMessage(), searchSubstr(), TestHardening_AdversarialInputValidation(), TestHardening_CSP_FailurePaths(), TestHardening_DeterministicReplay(), TestHardening_FinalFeasibilityGuarantee() (+5 more)

### Community 59 - "Room"
Cohesion: 0.23
Nodes (3): roomRepo, Room, mockRoomRepo

### Community 60 - "buildRoomChangeTestProblem"
Cohesion: 0.29
Nodes (12): buildRoomChangeTestProblem(), TestRoomChange_A_NoSessions(), TestRoomChange_B_OneSession(), TestRoomChange_C_TwoConsecutive_SameRoom(), TestRoomChange_D_TwoConsecutive_DifferentRooms(), TestRoomChange_E_GapBetweenSessions_SameDay(), TestRoomChange_F_DayBoundaryIsolation(), TestRoomChange_G_MultipleSessionsCumulative() (+4 more)

### Community 61 - "devDependencies"
Cohesion: 0.15
Nodes (13): oxlint, @types/node, @types/react, @types/react-dom, vite, @vitejs/plugin-react, devDependencies, oxlint (+5 more)

### Community 62 - "react"
Cohesion: 0.17
Nodes (10): oxc, react, warn, typescript, plugins, rules, react/only-export-components, react/rules-of-hooks (+2 more)

### Community 63 - "New"
Cohesion: 0.31
Nodes (8): New(), TestAdapter_CompileConstraints(), TestAdapter_Solve_SimpleProblem(), TestAdapter_Verify(), assignmentJSON(), conflictProblemJSON(), TestAdapter_Verify_AcceptsConflictFreeJSONSolution(), TestAdapter_Verify_RejectsJSONDeserializedConflicts()

### Community 64 - ".Solve"
Cohesion: 0.35
Nodes (6): mapDiagnostics(), mapScore(), mapViolations(), Adapter, SolveRequest, ValidateMoveResponse

### Community 65 - "StudentGroupID"
Cohesion: 0.25
Nodes (7): time.Weekday, StudentGroupID, SlotKey, TimeSlot, groupDayKey, GroupDayGap, OccupiedSession

### Community 66 - "App"
Cohesion: 0.49
Nodes (11): App(), handleExecuteMove(), handleExecuteSwap(), handleMutationError(), handlePublish(), handleRunCurra(), handleSendBack(), handleSubmitReview() (+3 more)

### Community 68 - "indexAssignment"
Cohesion: 0.51
Nodes (4): Problem, Solution, indexAssignment(), unindexAssignment()

### Community 69 - "SolveJobView.tsx"
Cohesion: 0.24
Nodes (8): api, SolveJob, SolveJobResult, Props, SolveJobView(), pollUntilTerminal(), runJob(), TERMINAL_STATUSES

### Community 74 - "PreSolve"
Cohesion: 0.53
Nodes (8): checkFacultyOverload(), checkRoomFeatureBottleneck(), checkZeroDomain(), featureSetKey(), Problem, joinFeatureIDs(), PreSolve(), presolveViolation()

### Community 78 - "TabuList"
Cohesion: 0.29
Nodes (3): NewTabuList(), TestTabuSearch_TabuTenurePreventsReversal(), TabuList

### Community 85 - "github.com/jackc/pgx/v5/pgxpool.Pool"
Cohesion: 0.38
Nodes (3): NewIdempotencyRepo(), github.com/jackc/pgx/v5/pgxpool.Pool, IdempotencyRepo

### Community 98 - "sampleProblem"
Cohesion: 0.70
Nodes (4): loadProblem(), main(), sampleProblem(), writeOutput()

## Knowledge Gaps
- **74 isolated node(s):** `Institution`, `ObjectiveComponentScore`, `PlacementDTO`, `ScoreBreakdown`, `SolveJobAssignment` (+69 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 148 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Problem` connect `Problem` to `GenerateSyntheticProblem`, `NewSolution`, `testing.T`, `New`, `Violation`, `constraints_test.go`, `IncrementalScoreEvaluator`, `NewSearchCtx`, `Assignment`, `backtracking.go`, `EvaluateMove`, `ConstraintInstance`, `RoomID`, `FeasibleProblem`, `ScoreBreakdown`, `.Check`, `Validate`, `invariants_test.go`, `LocalSearchTestProblem`, `FacultyID`, `.Search`, `TimeSlotID`, `CheckAll`, `Solution`, `CourseOfferingID`, `RoomFeatureCompatibilityConstraint`, `SearchCtx`, `MemberRef`, `hardening_invariants_test.go`, `buildRoomChangeTestProblem`, `StudentGroupID`, `sampleProblem`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Why does `Compile()` connect `constraints_test.go` to `.Solve`, `LocalSearchTestProblem`, `GenerateSyntheticProblem`, `testing.T`, `types.go`, `NewSearchCtx`, `backtracking.go`, `Problem`, `ConstraintInstance`, `hardening_invariants_test.go`, `.Check`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **Why does `TabuSearch()` connect `LocalSearchTestProblem` to `FacultyID`, `.Search`, `GenerateSyntheticProblem`, `NewSolution`, `testing.T`, `context.Context`, `constraints_test.go`, `Solution`, `EvaluateMove`, `Problem`, `hardening_invariants_test.go`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **What connects `Institution`, `ObjectiveComponentScore`, `PlacementDTO` to the rest of the system?**
  _74 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `github.com/google/uuid.UUID` be split into smaller, more focused modules?**
  _Cohesion score 0.06975633062589584 - nodes in this community are weakly interconnected._
- **Should `Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.13707250341997265 - nodes in this community are weakly interconnected._
- **Should `services/slice_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.060718252499074414 - nodes in this community are weakly interconnected._