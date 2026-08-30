// CURRA Frontend API Types matching OpenAPI and Backend DTO Contracts

export interface User {
  id: string;
  ID?: string;
  email: string;
  Email?: string;
  name: string;
  Name?: string;
}

export interface Institution {
  id: string;
  ID?: string;
  name: string;
  Name?: string;
  slug?: string;
  Slug?: string;
}

export interface AuthMeResponse {
  user?: User;
  User?: User;
  institution?: Institution;
  Institution?: Institution;
  role?: string;
  Role?: string;
}

export interface Timetable {
  id: string;
  ID?: string;
  institutionId?: string;
  InstitutionID?: string;
  name: string;
  Name?: string;
  currentPublishedVersionId?: string;
  CurrentPublishedVersionID?: string;
  version?: number;
  Version?: number;
  createdAt?: string;
  CreatedAt?: string;
  updatedAt?: string;
  UpdatedAt?: string;
}

export interface ProblemSnapshot {
  id: string;
  ID?: string;
  timetableId?: string;
  TimetableID?: string;
  institutionId?: string;
  InstitutionID?: string;
  schemaVersion?: number;
  SchemaVersion?: number;
  inputHash?: string;
  InputHash?: string;
  createdBy?: string;
  CreatedBy?: string;
  createdAt?: string;
  CreatedAt?: string;
}

export interface ObjectiveComponentScore {
  id: string;
  ID?: string;
  rawScore?: number;
  RawScore?: number;
  weight?: number;
  Weight?: number;
  weightedScore?: number;
  WeightedScore?: number;
}

export interface ScoreBreakdown {
  hardViolations: number;
  HardViolations?: number;
  softPenalty: number;
  SoftPenalty?: number;
  studentGapPenalty?: number;
  StudentGapPenalty?: number;
  facultyPreferencePenalty?: number;
  FacultyPreferencePenalty?: number;
  roomChangePenalty?: number;
  RoomChangePenalty?: number;
  components?: ObjectiveComponentScore[];
  Components?: ObjectiveComponentScore[];
}

export interface Violation {
  constraintName: string;
  ConstraintName?: string;
  severity: 'HARD' | 'SOFT';
  Severity?: 'HARD' | 'SOFT';
  message: string;
  Message?: string;
  assignmentId?: string;
  AssignmentID?: string;
  relatedIds?: Record<string, string>;
  RelatedIDs?: Record<string, string>;
  metadata?: Record<string, string>;
  Metadata?: Record<string, string>;
}

export interface ScheduleRun {
  id: string;
  ID?: string;
  timetableId?: string;
  TimetableID?: string;
  institutionId?: string;
  InstitutionID?: string;
  snapshotId?: string;
  SnapshotID?: string;
  status: 'QUEUED' | 'RUNNING' | 'SOLVED' | 'INFEASIBLE' | 'INVALID_PROBLEM' | 'INVALID_RESULT' | 'CANCELLED' | 'DEADLINE_EXCEEDED' | 'NODE_LIMIT' | 'FAILED';
  Status?: 'QUEUED' | 'RUNNING' | 'SOLVED' | 'INFEASIBLE' | 'INVALID_PROBLEM' | 'INVALID_RESULT' | 'CANCELLED' | 'DEADLINE_EXCEEDED' | 'NODE_LIMIT' | 'FAILED';
  seed?: number;
  Seed?: number;
  curraVersion?: string;
  CurrAVersion?: string;
  diagnostics?: {
    status?: string;
    Status?: string;
    nodesExplored?: number;
    NodesExplored?: number;
    candidates?: number;
    Candidates?: number;
    backtracks?: number;
    Backtracks?: number;
    message?: string;
    Message?: string;
  };
  Diagnostics?: {
    status?: string;
    Status?: string;
    nodesExplored?: number;
    NodesExplored?: number;
    candidates?: number;
    Candidates?: number;
    backtracks?: number;
    Backtracks?: number;
    message?: string;
    Message?: string;
  };
  score?: ScoreBreakdown;
  Score?: ScoreBreakdown;
  violations?: Violation[];
  Violations?: Violation[];
  startedAt?: string;
  StartedAt?: string;
  finishedAt?: string;
  FinishedAt?: string;
  durationMs?: number;
  DurationMs?: number;
  createdAt?: string;
  CreatedAt?: string;
  version?: number;
  Version?: number;
}

export type VersionStatus = 'DRAFT' | 'REVIEW' | 'PUBLISHED' | 'ARCHIVED';

export interface ScheduleVersion {
  id: string;
  ID?: string;
  timetableId?: string;
  TimetableID?: string;
  institutionId?: string;
  InstitutionID?: string;
  sourceRunId?: string;
  SourceRunID?: string;
  snapshotId?: string;
  SnapshotID?: string;
  status: VersionStatus;
  Status?: VersionStatus;
  name: string;
  Name?: string;
  score?: ScoreBreakdown;
  Score?: ScoreBreakdown;
  version: number;
  Version?: number;
  createdBy?: string;
  CreatedBy?: string;
  createdAt?: string;
  CreatedAt?: string;
  updatedAt?: string;
  UpdatedAt?: string;
}

export interface ScheduleAssignment {
  id?: string;
  ID?: string;
  versionId?: string;
  VersionID?: string;
  assignmentId?: string;
  AssignmentID?: string;
  courseOfferingId?: string;
  CourseOfferingID?: string;
  sessionRequirementId?: string;
  SessionRequirementID?: string;
  studentGroupId?: string;
  StudentGroupID?: string;
  facultyId?: string;
  FacultyID?: string;
  roomId?: string;
  RoomID?: string;
  timeSlotId?: string;
  TimeSlotID?: string;
  instance?: number;
  Instance?: number;
  createdAt?: string;
  CreatedAt?: string;
}

export interface PlacementDTO {
  roomId: string;
  timeSlotId: string;
}

export interface MoveDTO {
  assignmentId: string;
  from: PlacementDTO;
  to: PlacementDTO;
}

export interface SwapDTO {
  assignment1Id: string;
  assignment2Id: string;
  placement1: PlacementDTO;
  placement2: PlacementDTO;
}

export interface ValidateMoveResponse {
  valid: boolean;
  status: string;
  violations?: Violation[];
  score?: ScoreBreakdown;
  solution?: any;
}

export interface MoveResponse {
  validation: ValidateMoveResponse;
  version: ScheduleVersion;
}

export interface RoomEntity {
  id: string;
  ID?: string;
  name: string;
  Name?: string;
  capacity: number;
  Capacity?: number;
}

export interface TimeSlotEntity {
  id: string;
  ID?: string;
  day: string;
  Day?: string;
  period: number;
  Period?: number;
  label: string;
  Label?: string;
}

export interface APIError {
  code: string;
  message: string;
  details?: any;
}
