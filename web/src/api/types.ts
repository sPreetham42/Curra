// CURRA Frontend API Types matching OpenAPI and Backend DTO Contracts

export interface User {
  id: string;
  email: string;
  name: string;
}

export interface Institution {
  id: string;
  name: string;
  slug: string;
}

export interface AuthMeResponse {
  user: User;
  institution: Institution;
  role: string;
}

export interface Timetable {
  id: string;
  institutionId: string;
  name: string;
  currentPublishedVersionId?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface ProblemSnapshot {
  id: string;
  timetableId: string;
  institutionId: string;
  schemaVersion: number;
  inputHash: string;
  createdBy: string;
  createdAt: string;
}

export interface ObjectiveComponentScore {
  id: string;
  rawScore: number;
  weight: number;
  weightedScore: number;
}

export interface ScoreBreakdown {
  hardViolations: number;
  softPenalty: number;
  studentGapPenalty?: number;
  facultyPreferencePenalty?: number;
  roomChangePenalty?: number;
  components?: ObjectiveComponentScore[];
}

export interface Violation {
  constraintName: string;
  severity: 'HARD' | 'SOFT';
  message: string;
  assignmentId?: string;
  relatedIds?: Record<string, string>;
  metadata?: Record<string, string>;
}

export interface ScheduleRun {
  id: string;
  timetableId: string;
  institutionId: string;
  snapshotId: string;
  status: 'QUEUED' | 'RUNNING' | 'SOLVED' | 'INFEASIBLE' | 'INVALID_PROBLEM' | 'INVALID_RESULT' | 'CANCELLED' | 'DEADLINE_EXCEEDED' | 'NODE_LIMIT' | 'FAILED';
  seed?: number;
  curraVersion?: string;
  diagnostics?: {
    status: string;
    nodesExplored: number;
    candidates: number;
    backtracks: number;
    message?: string;
  };
  score?: ScoreBreakdown;
  violations?: Violation[];
  startedAt?: string;
  finishedAt?: string;
  durationMs?: number;
  createdAt: string;
  version: number;
}

export type VersionStatus = 'DRAFT' | 'REVIEW' | 'PUBLISHED' | 'ARCHIVED';

export interface ScheduleVersion {
  id: string;
  timetableId: string;
  institutionId: string;
  sourceRunId?: string;
  snapshotId: string;
  status: VersionStatus;
  name: string;
  score?: ScoreBreakdown;
  version: number;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface ScheduleAssignment {
  id: string;
  versionId: string;
  assignmentId: string;
  courseOfferingId: string;
  sessionRequirementId: string;
  studentGroupId: string;
  facultyId: string;
  roomId: string;
  timeSlotId: string;
  instance: number;
  createdAt: string;
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
  name: string;
  capacity: number;
}

export interface TimeSlotEntity {
  id: string;
  day: string;
  period: number;
  label: string;
}

export interface APIError {
  code: string;
  message: string;
  details?: any;
}
