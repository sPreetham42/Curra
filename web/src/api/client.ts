import type {
  AuthMeResponse,
  Timetable,
  ProblemSnapshot,
  ScheduleRun,
  ScheduleVersion,
  ScheduleAssignment,
  MoveDTO,
  SwapDTO,
  MoveResponse,
  RoomEntity,
  TimeSlotEntity,
  APIError,
} from './types';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const DEV_TOKEN =
  import.meta.env.VITE_DEV_TOKEN ||
  '11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:INSTITUTION_ADMIN';

export class HTTPError extends Error {
  status: number;
  code: string;
  payload?: any;

  constructor(status: number, code: string, message: string, payload?: any) {
    super(message);
    this.status = status;
    this.code = code;
    this.payload = payload;
  }
}

async function request<T>(
  path: string,
  options: RequestInit & { ifMatch?: number } = {}
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${DEV_TOKEN}`,
  };

  if (options.ifMatch !== undefined) {
    headers['If-Match'] = String(options.ifMatch);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      ...headers,
      ...options.headers,
    },
  });

  if (!response.ok) {
    let errorJson: { error?: APIError; validation?: any } = {};
    try {
      errorJson = await response.json();
    } catch {
      // Body not JSON
    }

    const code = errorJson.error?.code || `HTTP_${response.status}`;
    const message = errorJson.error?.message || response.statusText || 'Request failed';
    throw new HTTPError(response.status, code, message, errorJson);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}

export const api = {
  // Auth
  async getMe(): Promise<AuthMeResponse> {
    return request<AuthMeResponse>('/api/v1/auth/me');
  },

  // Timetables
  async listTimetables(): Promise<Timetable[]> {
    return request<Timetable[]>('/api/v1/timetables');
  },

  async createTimetable(name: string): Promise<Timetable> {
    return request<Timetable>('/api/v1/timetables', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  },

  async getTimetable(id: string): Promise<Timetable> {
    return request<Timetable>(`/api/v1/timetables/${id}`);
  },

  // Snapshots
  async createSnapshot(timetableId: string): Promise<ProblemSnapshot> {
    return request<ProblemSnapshot>(`/api/v1/timetables/${timetableId}/snapshots`, {
      method: 'POST',
    });
  },

  async listSnapshots(timetableId: string): Promise<ProblemSnapshot[]> {
    return request<ProblemSnapshot[]>(`/api/v1/timetables/${timetableId}/snapshots`);
  },

  // Runs
  async createRun(timetableId: string, snapshotId: string): Promise<ScheduleRun> {
    return request<ScheduleRun>(`/api/v1/timetables/${timetableId}/runs`, {
      method: 'POST',
      body: JSON.stringify({ snapshotId }),
    });
  },

  async getRun(runId: string): Promise<ScheduleRun> {
    return request<ScheduleRun>(`/api/v1/runs/${runId}`);
  },

  // Versions
  async createVersion(
    timetableId: string,
    snapshotId: string,
    sourceRunId?: string,
    name?: string
  ): Promise<ScheduleVersion> {
    return request<ScheduleVersion>(`/api/v1/timetables/${timetableId}/versions`, {
      method: 'POST',
      body: JSON.stringify({ snapshotId, sourceRunId, name }),
    });
  },

  async listVersions(timetableId: string): Promise<ScheduleVersion[]> {
    return request<ScheduleVersion[]>(`/api/v1/timetables/${timetableId}/versions`);
  },

  async getVersion(versionId: string): Promise<{ version: ScheduleVersion; assignments: ScheduleAssignment[] }> {
    return request<{ version: ScheduleVersion; assignments: ScheduleAssignment[] }>(`/api/v1/versions/${versionId}`);
  },

  async listAssignments(versionId: string): Promise<ScheduleAssignment[]> {
    return request<ScheduleAssignment[]>(`/api/v1/versions/${versionId}/assignments`);
  },

  async moveAssignment(versionId: string, move: MoveDTO, ifMatchVersion: number): Promise<MoveResponse> {
    return request<MoveResponse>(`/api/v1/versions/${versionId}/assignments/move`, {
      method: 'POST',
      ifMatch: ifMatchVersion,
      body: JSON.stringify(move),
    });
  },

  async swapAssignments(versionId: string, swap: SwapDTO, ifMatchVersion: number): Promise<MoveResponse> {
    return request<MoveResponse>(`/api/v1/versions/${versionId}/assignments/swap`, {
      method: 'POST',
      ifMatch: ifMatchVersion,
      body: JSON.stringify(swap),
    });
  },

  async submitReview(versionId: string, ifMatchVersion: number): Promise<ScheduleVersion> {
    return request<ScheduleVersion>(`/api/v1/versions/${versionId}/submit-review`, {
      method: 'POST',
      ifMatch: ifMatchVersion,
    });
  },

  async sendBack(versionId: string, ifMatchVersion: number): Promise<ScheduleVersion> {
    return request<ScheduleVersion>(`/api/v1/versions/${versionId}/send-back`, {
      method: 'POST',
      ifMatch: ifMatchVersion,
    });
  },

  async publishVersion(versionId: string, ifMatchVersion: number): Promise<ScheduleVersion> {
    return request<ScheduleVersion>(`/api/v1/versions/${versionId}/publish`, {
      method: 'POST',
      ifMatch: ifMatchVersion,
    });
  },

  // Catalog
  async listRooms(): Promise<RoomEntity[]> {
    return request<RoomEntity[]>('/api/v1/rooms');
  },

  async listTimeSlots(): Promise<TimeSlotEntity[]> {
    return request<TimeSlotEntity[]>('/api/v1/time-slots');
  },
};
