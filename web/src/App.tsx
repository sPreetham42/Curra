import { useEffect, useState } from 'react';
import { api, HTTPError } from './api/client';
import type {
  AuthMeResponse,
  Timetable,
  ScheduleVersion,
  ScheduleAssignment,
  ScheduleRun,
  RoomEntity,
  TimeSlotEntity,
} from './api/types';
import {
  Play,
  RotateCw,
  CheckCircle2,
  AlertTriangle,
  Send,
  CheckCheck,
  ArrowRightLeft,
  MoveRight,
  RotateCcw,
  Sparkles,
  Layers,
} from 'lucide-react';

export default function App() {
  const [auth, setAuth] = useState<AuthMeResponse | null>(null);
  const [authError, setAuthError] = useState<string | null>(null);
  const [timetable, setTimetable] = useState<Timetable | null>(null);
  const [versions, setVersions] = useState<ScheduleVersion[]>([]);
  const [currentVersion, setCurrentVersion] = useState<ScheduleVersion | null>(null);
  const [assignments, setAssignments] = useState<ScheduleAssignment[]>([]);

  const [rooms, setRooms] = useState<RoomEntity[]>([]);
  const [timeSlots, setTimeSlots] = useState<TimeSlotEntity[]>([]);

  const [currentRun, setCurrentRun] = useState<ScheduleRun | null>(null);
  const [isSolving, setIsSolving] = useState(false);
  const [selectedAssignment, setSelectedAssignment] = useState<ScheduleAssignment | null>(null);
  const [swapAssignment, setSwapAssignment] = useState<ScheduleAssignment | null>(null);

  const [targetRoomId, setTargetRoomId] = useState<string>('');
  const [targetTimeSlotId, setTargetTimeSlotId] = useState<string>('');

  const [errorBanner, setErrorBanner] = useState<{ title: string; message: string; details?: any } | null>(null);
  const [infoBanner, setInfoBanner] = useState<string | null>(null);
  const [loadingText, setLoadingText] = useState<string | null>('Initializing CURRA Console...');

  // Safe Property Accessors (supporting both camelCase and PascalCase DTOs)
  const getTTId = (t: Timetable | null) => (t ? t.id || t.ID || '' : '');
  const getTTName = (t: Timetable | null) => (t ? t.name || t.Name || '' : '');
  const getTTPublishedVer = (t: Timetable | null) => (t ? t.currentPublishedVersionId || t.CurrentPublishedVersionID : undefined);

  const getVerId = (v: ScheduleVersion | null) => (v ? v.id || v.ID || '' : '');
  const getVerName = (v: ScheduleVersion | null) => (v ? v.name || v.Name || '' : '');
  const getVerStatus = (v: ScheduleVersion | null) => (v ? v.status || v.Status || 'DRAFT' : 'DRAFT');
  const getVerCounter = (v: ScheduleVersion | null) => (v ? v.version ?? v.Version ?? 1 : 1);
  const getVerScore = (v: ScheduleVersion | null) => (v ? v.score || v.Score : undefined);

  const getAsgnKey = (a: ScheduleAssignment) => a.id || a.ID || a.assignmentId || a.AssignmentID || '';
  const getAsgnAssignmentId = (a: ScheduleAssignment) => a.assignmentId || a.AssignmentID || a.id || a.ID || '';
  const getAsgnCourseOfferingId = (a: ScheduleAssignment) => a.courseOfferingId || a.CourseOfferingID || 'Unknown Course';
  const getAsgnFacultyId = (a: ScheduleAssignment) => a.facultyId || a.FacultyID || 'Unknown Faculty';
  const getAsgnStudentGroupId = (a: ScheduleAssignment) => a.studentGroupId || a.StudentGroupID || 'Unknown Group';
  const getAsgnRoomId = (a: ScheduleAssignment) => a.roomId || a.RoomID || '';
  const getAsgnSlotId = (a: ScheduleAssignment) => a.timeSlotId || a.TimeSlotID || '';
  const getAsgnInstance = (a: ScheduleAssignment) => a.instance ?? a.Instance ?? 0;

  const getRoomId = (r: RoomEntity) => r.id || r.ID || '';
  const getRoomName = (id: string) => {
    if (!id) return 'Unknown Room';
    const found = rooms.find((r) => getRoomId(r) === id);
    return found ? found.name || found.Name || 'Unknown Room' : 'Unknown Room';
  };

  const getSlotId = (s: TimeSlotEntity) => s.id || s.ID || '';
  const getSlotLabel = (id: string) => {
    if (!id) return 'Unknown Slot';
    const found = timeSlots.find((s) => getSlotId(s) === id);
    return found ? found.label || found.Label || id : id || 'Unknown Slot';
  };

  // 1. Initial Load: Auth -> Timetable -> Versions -> Version Selection -> Assignments
  useEffect(() => {
    initConsole();
  }, []);

  async function initConsole() {
    setErrorBanner(null);
    setLoadingText('Authenticating & loading environment...');
    try {
      // Step A: Auth
      const authData = await api.getMe();
      setAuth(authData);

      // Load Catalog Metadata
      const [roomsData, slotsData] = await Promise.all([
        api.listRooms().catch(() => []),
        api.listTimeSlots().catch(() => []),
      ]);
      setRooms(Array.isArray(roomsData) ? roomsData : []);
      setTimeSlots(Array.isArray(slotsData) ? slotsData : []);

      // Step B: Timetable Resolution
      setLoadingText('Loading timetable...');
      let targetTimetable: Timetable | null = null;

      const envTimetableId = import.meta.env.VITE_TIMETABLE_ID;
      if (envTimetableId) {
        try {
          targetTimetable = await api.getTimetable(envTimetableId);
        } catch (e) {
          console.warn('Failed to fetch VITE_TIMETABLE_ID, falling back to listTimetables:', e);
        }
      }

      if (!targetTimetable) {
        const timetablesData = await api.listTimetables();
        if (Array.isArray(timetablesData) && timetablesData.length > 0) {
          targetTimetable = timetablesData[0];
        }
      }

      if (!targetTimetable) {
        setLoadingText(null);
        setTimetable(null);
        return;
      }

      setTimetable(targetTimetable);

      // Step C: Versions (Only execute if timetableId is confirmed valid!)
      const ttId = getTTId(targetTimetable);
      if (ttId) {
        await loadVersions(ttId, getTTPublishedVer(targetTimetable));
      }
    } catch (err: any) {
      console.error('Initialization error:', err);
      setAuthError(err.message || 'Failed to connect to CURRA backend.');
    } finally {
      setLoadingText(null);
    }
  }

  async function loadVersions(timetableId: string, selectVersionId?: string) {
    if (!timetableId) return;
    setLoadingText('Loading versions...');
    try {
      const verList = await api.listVersions(timetableId);
      const safeVerList = Array.isArray(verList) ? verList : [];
      setVersions(safeVerList);

      if (safeVerList.length === 0) {
        setCurrentVersion(null);
        setAssignments([]);
        return;
      }

      // Version selection priority:
      // 1. Explicit selectVersionId / currentPublishedVersionId if valid
      // 2. Published version
      // 3. Draft or Review version
      // 4. Newest version
      let targetVer = safeVerList.find((v) => getVerId(v) === selectVersionId);
      if (!targetVer) {
        targetVer = safeVerList.find((v) => getVerStatus(v) === 'PUBLISHED') ||
                    safeVerList.find((v) => getVerStatus(v) === 'DRAFT' || getVerStatus(v) === 'REVIEW') ||
                    safeVerList[safeVerList.length - 1] ||
                    safeVerList[0];
      }

      if (targetVer && getVerId(targetVer)) {
        await selectVersion(getVerId(targetVer));
      } else {
        setCurrentVersion(null);
        setAssignments([]);
      }
    } catch (err: any) {
      console.error('Load versions error:', err);
      setErrorBanner({ title: 'Load Versions Error', message: err.message || 'Failed to load versions' });
    } finally {
      setLoadingText(null);
    }
  }

  async function selectVersion(versionId: string) {
    if (!versionId) return;
    setLoadingText('Loading assignments...');
    try {
      const data = await api.getVersion(versionId);
      if (data && data.version) {
        setCurrentVersion(data.version);
        setAssignments(Array.isArray(data.assignments) ? data.assignments : []);
      } else {
        const asgns = await api.listAssignments(versionId);
        setAssignments(Array.isArray(asgns) ? asgns : []);
      }
      setSelectedAssignment(null);
      setSwapAssignment(null);
    } catch (err: any) {
      setErrorBanner({ title: 'Load Version Error', message: err.message || 'Failed to load version' });
    } finally {
      setLoadingText(null);
    }
  }

  // 2. Run CURRA Solver Flow (Triggered ONLY on explicit user click)
  async function handleRunCurra() {
    const ttId = getTTId(timetable);
    if (!ttId) return;
    setErrorBanner(null);
    setInfoBanner(null);
    setIsSolving(true);
    setLoadingText('Creating problem snapshot from live database catalog...');

    try {
      // Step A: Create Snapshot
      const snap = await api.createSnapshot(ttId);
      const snapId = snap.id || (snap as any).ID;

      // Step B: Create Solver Run
      setLoadingText('Enqueuing CURRA solver job...');
      const run = await api.createRun(ttId, snapId);
      const runId = run.id || (run as any).ID;
      setCurrentRun(run);

      // Step C: Poll Run Status
      setLoadingText(`CURRA Solver running (Status: ${run.status || (run as any).Status})...`);
      let pollRun = run;
      let status = pollRun.status || (pollRun as any).Status;

      while (status === 'QUEUED' || status === 'RUNNING') {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        pollRun = await api.getRun(runId);
        status = pollRun.status || (pollRun as any).Status;
        setCurrentRun(pollRun);
        setLoadingText(`CURRA Solver optimizing (Status: ${status})...`);
      }

      if (status === 'SOLVED') {
        setLoadingText('Solver completed successfully! Generating draft schedule version...');
        // Step D: Create Draft Version
        const softPenalty = pollRun.score?.softPenalty ?? (pollRun as any).Score?.softPenalty ?? 0;
        const newVer = await api.createVersion(
          ttId,
          snapId,
          runId,
          `Draft v${versions.length + 1} (Score: ${softPenalty})`
        );
        const newVerId = getVerId(newVer);
        const newVerName = getVerName(newVer);

        setInfoBanner(`Run SOLVED! Created new version: ${newVerName}`);
        await loadVersions(ttId, newVerId);
      } else {
        const diagMsg = pollRun.diagnostics?.message || (pollRun as any).Diagnostics?.message || `Solver failed with status ${status}`;
        const violations = pollRun.violations || (pollRun as any).Violations;
        setErrorBanner({
          title: `Solver Finished with Status: ${status}`,
          message: diagMsg,
          details: violations,
        });
      }
    } catch (err: any) {
      console.error('Run CURRA error:', err);
      setErrorBanner({ title: 'Run CURRA Execution Error', message: err.message || 'Solver execution failed' });
    } finally {
      setIsSolving(false);
      setLoadingText(null);
    }
  }

  // 3. Move Assignment Flow
  async function handleExecuteMove() {
    const verId = getVerId(currentVersion);
    const verCounter = getVerCounter(currentVersion);
    if (!verId || !selectedAssignment || !targetRoomId || !targetTimeSlotId) return;
    setErrorBanner(null);
    setInfoBanner(null);

    const movePayload = {
      assignmentId: getAsgnAssignmentId(selectedAssignment),
      from: { roomId: getAsgnRoomId(selectedAssignment), timeSlotId: getAsgnSlotId(selectedAssignment) },
      to: { roomId: targetRoomId, timeSlotId: targetTimeSlotId },
    };

    try {
      const resp = await api.moveAssignment(verId, movePayload, verCounter);
      const updatedVer = resp.version || (resp as any).Version;
      const score = getVerScore(updatedVer);
      setInfoBanner(`Assignment move succeeded! Updated score: ${score?.softPenalty ?? 0}`);
      await selectVersion(verId);
    } catch (err: any) {
      handleMutationError(err, 'Move Assignment Failed');
    }
  }

  // 4. Swap Assignments Flow
  async function handleExecuteSwap() {
    const verId = getVerId(currentVersion);
    const verCounter = getVerCounter(currentVersion);
    if (!verId || !selectedAssignment || !swapAssignment) return;
    setErrorBanner(null);
    setInfoBanner(null);

    const swapPayload = {
      assignment1Id: getAsgnAssignmentId(selectedAssignment),
      assignment2Id: getAsgnAssignmentId(swapAssignment),
      placement1: { roomId: getAsgnRoomId(selectedAssignment), timeSlotId: getAsgnSlotId(selectedAssignment) },
      placement2: { roomId: getAsgnRoomId(swapAssignment), timeSlotId: getAsgnSlotId(swapAssignment) },
    };

    try {
      const resp = await api.swapAssignments(verId, swapPayload, verCounter);
      const updatedVer = resp.version || (resp as any).Version;
      const score = getVerScore(updatedVer);
      setInfoBanner(`Assignments swapped successfully! Updated score: ${score?.softPenalty ?? 0}`);
      await selectVersion(verId);
    } catch (err: any) {
      handleMutationError(err, 'Swap Assignments Failed');
    }
  }

  // 5. Version State Machine Actions
  async function handleSubmitReview() {
    const verId = getVerId(currentVersion);
    const verCounter = getVerCounter(currentVersion);
    const ttId = getTTId(timetable);
    if (!verId || !ttId) return;
    try {
      const updated = await api.submitReview(verId, verCounter);
      const updatedId = getVerId(updated);
      setInfoBanner(`Version submitted for review (Status: REVIEW)`);
      await selectVersion(updatedId);
      await loadVersions(ttId, updatedId);
    } catch (err: any) {
      handleMutationError(err, 'Submit Review Failed');
    }
  }

  async function handleSendBack() {
    const verId = getVerId(currentVersion);
    const verCounter = getVerCounter(currentVersion);
    const ttId = getTTId(timetable);
    if (!verId || !ttId) return;
    try {
      const updated = await api.sendBack(verId, verCounter);
      const updatedId = getVerId(updated);
      setInfoBanner(`Version sent back to draft (Status: DRAFT)`);
      await selectVersion(updatedId);
      await loadVersions(ttId, updatedId);
    } catch (err: any) {
      handleMutationError(err, 'Send Back Failed');
    }
  }

  async function handlePublish() {
    const verId = getVerId(currentVersion);
    const verCounter = getVerCounter(currentVersion);
    const ttId = getTTId(timetable);
    if (!verId || !ttId) return;
    try {
      const updated = await api.publishVersion(verId, verCounter);
      const updatedId = getVerId(updated);
      setInfoBanner(`Version published successfully! (Status: PUBLISHED)`);
      await selectVersion(updatedId);
      await loadVersions(ttId, updatedId);
    } catch (err: any) {
      handleMutationError(err, 'Publish Version Failed');
    }
  }

  function handleMutationError(err: any, title: string) {
    if (err instanceof HTTPError) {
      if (err.status === 409) {
        setInfoBanner('This timetable changed. Reloading the latest version...');
        const ttId = getTTId(timetable);
        if (ttId) {
          loadVersions(ttId);
        }
        return;
      }
      if (err.status === 422) {
        setErrorBanner({
          title: `${title}: 422 Validation Failed`,
          message: err.message,
          details: err.payload?.validation?.violations || err.payload,
        });
        return;
      }
    }
    setErrorBanner({ title, message: err.message || 'Unknown error' });
  }

  const days = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'];
  const periods = [1, 2, 3, 4, 5, 6];

  const authUser = auth?.user || (auth as any)?.User;
  const authInst = auth?.institution || (auth as any)?.Institution;
  const authRole = auth?.role || (auth as any)?.Role;

  const currentVerStatus = getVerStatus(currentVersion);
  const currentVerScore = getVerScore(currentVersion);

  return (
    <div className="app-container">
      {/* Header */}
      <header>
        <div className="logo-group">
          <Sparkles className="w-6 h-6 text-sky-400" />
          <h1 className="logo-title">CURRA Timetable Platform</h1>
          <span className="text-xs bg-slate-800 text-sky-400 px-2 py-0.5 rounded border border-slate-700 font-mono">
            v1.0.0 Console
          </span>
        </div>

        {auth && authUser && authInst && (
          <div className="flex items-center gap-4 text-sm">
            <span className="text-slate-400">
              Institution: <strong className="text-slate-200">{authInst.name || authInst.Name || 'Default Institution'}</strong>
            </span>
            <span className="text-slate-400">
              User: <strong className="text-slate-200">{authUser.name || authUser.Name || 'Dev Admin'}</strong> ({authRole || 'ADMIN'})
            </span>
          </div>
        )}

        <button
          className="btn-primary"
          onClick={handleRunCurra}
          disabled={isSolving || !timetable || !getTTId(timetable)}
        >
          {isSolving ? <RotateCw className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4 fill-current" />}
          Run CURRA
        </button>
      </header>

      {/* Main Body */}
      <main className="p-6 space-y-6 flex-1 max-w-7xl mx-auto w-full">
        {authError && (
          <div className="alert-panel alert-error flex items-center justify-between">
            <div>
              <strong>Auth / Connection Error:</strong> {authError}
            </div>
            <button className="btn-secondary text-xs" onClick={initConsole}>
              Retry Connection
            </button>
          </div>
        )}

        {loadingText && (
          <div className="alert-panel alert-info flex items-center gap-2">
            <RotateCw className="w-4 h-4 animate-spin text-sky-400" />
            <span>{loadingText}</span>
          </div>
        )}

        {infoBanner && (
          <div className="alert-panel alert-info flex items-center justify-between">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 text-sky-400" />
              <span>{infoBanner}</span>
            </div>
            <button className="text-xs text-sky-400 underline" onClick={() => setInfoBanner(null)}>
              Dismiss
            </button>
          </div>
        )}

        {errorBanner && (
          <div className="alert-panel alert-error space-y-2">
            <div className="flex items-center gap-2 font-semibold">
              <AlertTriangle className="w-4 h-4 text-rose-400" />
              <span>{errorBanner.title}</span>
            </div>
            <p className="text-sm">{errorBanner.message}</p>
            {errorBanner.details && (
              <pre className="text-xs bg-slate-900/80 p-2 rounded overflow-x-auto text-rose-300 font-mono">
                {JSON.stringify(errorBanner.details, null, 2)}
              </pre>
            )}
          </div>
        )}

        {/* Top Control Bar & Metrics */}
        {timetable ? (
          <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-4 grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Timetable Project</label>
              <div className="text-base font-bold text-slate-100 mt-1">{getTTName(timetable)}</div>
              <div className="text-xs text-slate-400 mt-0.5 font-mono">ID: {getTTId(timetable)}</div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Schedule Version</label>
              <div className="flex items-center gap-2 mt-1">
                {versions.length > 0 ? (
                  <select
                    className="bg-slate-900 border border-slate-700 text-slate-200 text-sm rounded px-2 py-1 focus:outline-none focus:border-sky-500"
                    value={getVerId(currentVersion)}
                    onChange={(e) => selectVersion(e.target.value)}
                  >
                    {versions.map((v) => (
                      <option key={getVerId(v)} value={getVerId(v)}>
                        {getVerName(v)} ({getVerStatus(v)}) - v{getVerCounter(v)}
                      </option>
                    ))}
                  </select>
                ) : (
                  <span className="text-sm text-slate-400 italic">No versions yet</span>
                )}
                {currentVersion && (
                  <span className={`badge badge-${currentVerStatus.toLowerCase()}`}>
                    {currentVerStatus}
                  </span>
                )}
              </div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Solver Status</label>
              <div className="text-base font-bold text-slate-100 mt-1 flex items-center gap-2">
                {currentRun ? (
                  <span className="text-sky-400">{currentRun.status || (currentRun as any).Status}</span>
                ) : (
                  <span className="text-slate-400">IDLE</span>
                )}
              </div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Backend Score</label>
              {currentVerScore ? (
                <div className="mt-1 space-y-0.5 text-xs">
                  <div className="text-sm font-bold text-emerald-400">
                    Hard Violations: {currentVerScore.hardViolations ?? 0} | Soft Penalty: {currentVerScore.softPenalty ?? 0}
                  </div>
                  {currentVerScore.studentGapPenalty !== undefined && (
                    <div className="text-slate-400">
                      Gap: {currentVerScore.studentGapPenalty} | Pref: {currentVerScore.facultyPreferencePenalty ?? 0} | RoomChange: {currentVerScore.roomChangePenalty ?? 0}
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-sm text-slate-500 mt-1">No score available</div>
              )}
            </div>
          </div>
        ) : (
          !loadingText && (
            <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-8 text-center text-slate-400">
              <p className="text-base font-semibold">No timetable available.</p>
            </div>
          )
        )}

        {/* Version Workflow State Machine Bar */}
        {currentVersion && (
          <div className="bg-slate-800/50 border border-slate-700/60 rounded-lg p-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm text-slate-300">
              <Layers className="w-4 h-4 text-sky-400" />
              <span>Version Workflow State:</span>
              <strong className="text-slate-100">{currentVerStatus}</strong>
              <span className="text-slate-500">(Version Counter: {getVerCounter(currentVersion)})</span>
            </div>

            <div className="flex items-center gap-3">
              {currentVerStatus === 'DRAFT' && (
                <button className="btn-secondary text-xs" onClick={handleSubmitReview}>
                  <Send className="w-3.5 h-3.5 text-amber-400" />
                  Submit Review
                </button>
              )}

              {currentVerStatus === 'REVIEW' && (
                <>
                  <button className="btn-secondary text-xs" onClick={handleSendBack}>
                    <RotateCcw className="w-3.5 h-3.5 text-slate-400" />
                    Send Back to Draft
                  </button>
                  <button className="btn-primary text-xs" onClick={handlePublish}>
                    <CheckCheck className="w-3.5 h-3.5" />
                    Publish Version
                  </button>
                </>
              )}

              {currentVerStatus === 'PUBLISHED' && (
                <span className="text-xs text-emerald-400 flex items-center gap-1 font-semibold">
                  <CheckCircle2 className="w-4 h-4" /> Published & Active Timetable
                </span>
              )}
            </div>
          </div>
        )}

        {/* Main Grid & Inspector Section */}
        {timetable && (
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Timetable Grid (3 cols) */}
            <div className="lg:col-span-3 bg-slate-800/80 border border-slate-700 rounded-lg p-4 overflow-x-auto">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-bold text-slate-200">Weekly Schedule Grid</h2>
                <span className="text-xs text-slate-400">{assignments.length} Total Assignments</span>
              </div>

              {versions.length === 0 ? (
                <div className="text-center py-12 text-slate-500 space-y-3">
                  <p className="text-base font-medium text-slate-400">No timetable version yet.</p>
                  <p className="text-xs text-slate-500">Run CURRA to generate one.</p>
                  <button className="btn-primary mx-auto text-xs" onClick={handleRunCurra} disabled={isSolving}>
                    <Play className="w-4 h-4 fill-current inline mr-1" />
                    Run CURRA
                  </button>
                </div>
              ) : assignments.length === 0 ? (
                <div className="text-center py-12 text-slate-500 space-y-3">
                  <p className="text-base font-medium text-slate-400">No assignments in this version.</p>
                </div>
              ) : (
                <table className="grid-table">
                  <thead>
                    <tr>
                      <th style={{ width: '80px' }}>Period</th>
                      {days.map((d) => (
                        <th key={d}>{d}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {periods.map((p) => (
                      <tr key={p}>
                        <td className="font-semibold text-slate-400">Period {p}</td>
                        {days.map((day) => {
                          const matched = assignments.filter((a) => {
                            const slotId = getAsgnSlotId(a);
                            const slot = timeSlots.find((s) => getSlotId(s) === slotId);
                            const slotDay = slot ? (slot.day || slot.Day || '') : '';
                            const slotPeriod = slot ? (slot.period ?? slot.Period) : 0;
                            return slotDay.toLowerCase() === day.toLowerCase() && slotPeriod === p;
                          });

                          return (
                            <td key={day}>
                              {matched.map((asgn) => {
                                const asgnKey = getAsgnKey(asgn);
                                const isSelected = selectedAssignment && getAsgnKey(selectedAssignment) === asgnKey;
                                const isSwapTarget = swapAssignment && getAsgnKey(swapAssignment) === asgnKey;

                                return (
                                  <div
                                    key={asgnKey}
                                    className={`assignment-card ${isSelected ? 'selected' : ''} ${
                                      isSwapTarget ? 'border-purple-400 bg-purple-950/40' : ''
                                    }`}
                                    onClick={() => {
                                      if (isSelected) {
                                        setSelectedAssignment(null);
                                      } else if (selectedAssignment && !isSwapTarget) {
                                        setSwapAssignment(asgn);
                                      } else {
                                        setSelectedAssignment(asgn);
                                        setSwapAssignment(null);
                                      }
                                    }}
                                  >
                                    <div className="flex justify-between items-center text-xs font-bold text-sky-300">
                                      <span>{getAsgnCourseOfferingId(asgn)}</span>
                                      <span className="text-slate-400 font-mono text-[10px]">#{getAsgnInstance(asgn)}</span>
                                    </div>
                                    <div className="text-xs text-slate-300 font-medium mt-0.5">
                                      {getRoomName(getAsgnRoomId(asgn))}
                                    </div>
                                    <div className="text-[11px] text-slate-400 mt-0.5 truncate">
                                      Fac: {getAsgnFacultyId(asgn)}
                                    </div>
                                  </div>
                                );
                              })}
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            {/* Selected Assignment & Move/Swap Inspector Panel (1 col) */}
            <div className="lg:col-span-1 space-y-4">
              <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-4 space-y-4">
                <h3 className="text-sm font-bold text-slate-200 uppercase border-b border-slate-700 pb-2">
                  Assignment Inspector
                </h3>

                {selectedAssignment ? (
                  <div className="space-y-4 text-xs">
                    <div className="space-y-1 bg-slate-900/60 p-3 rounded border border-slate-700/80">
                      <div className="text-sky-400 font-bold text-sm">{getAsgnAssignmentId(selectedAssignment)}</div>
                      <div>Course Offering: <span className="text-slate-200">{getAsgnCourseOfferingId(selectedAssignment)}</span></div>
                      <div>Faculty ID: <span className="text-slate-200">{getAsgnFacultyId(selectedAssignment)}</span></div>
                      <div>Student Group: <span className="text-slate-200">{getAsgnStudentGroupId(selectedAssignment)}</span></div>
                      <div>Current Room: <span className="text-emerald-400 font-semibold">{getRoomName(getAsgnRoomId(selectedAssignment))}</span></div>
                      <div>Current Slot: <span className="text-amber-400 font-semibold">{getSlotLabel(getAsgnSlotId(selectedAssignment))}</span></div>
                    </div>

                    {/* Move Section */}
                    {currentVerStatus === 'DRAFT' && (
                      <div className="space-y-2 border-t border-slate-700 pt-3">
                        <div className="font-semibold text-slate-200 flex items-center gap-1">
                          <MoveRight className="w-3.5 h-3.5 text-sky-400" /> Move Placement
                        </div>

                        <div>
                          <label className="text-[11px] text-slate-400">Target Room</label>
                          <select
                            className="w-full bg-slate-900 border border-slate-700 text-slate-200 text-xs rounded px-2 py-1 mt-1"
                            value={targetRoomId}
                            onChange={(e) => setTargetRoomId(e.target.value)}
                          >
                            <option value="">Select Target Room...</option>
                            {rooms.map((r) => (
                              <option key={getRoomId(r)} value={getRoomId(r)}>
                                {r.name || r.Name} (Cap: {r.capacity ?? r.Capacity ?? 0})
                              </option>
                            ))}
                          </select>
                        </div>

                        <div>
                          <label className="text-[11px] text-slate-400">Target Time Slot</label>
                          <select
                            className="w-full bg-slate-900 border border-slate-700 text-slate-200 text-xs rounded px-2 py-1 mt-1"
                            value={targetTimeSlotId}
                            onChange={(e) => setTargetTimeSlotId(e.target.value)}
                          >
                            <option value="">Select Target Slot...</option>
                            {timeSlots.map((s) => (
                              <option key={getSlotId(s)} value={getSlotId(s)}>
                                {s.label || s.Label} ({s.day || s.Day})
                              </option>
                            ))}
                          </select>
                        </div>

                        <button
                          className="btn-primary w-full text-xs justify-center mt-2"
                          onClick={handleExecuteMove}
                          disabled={!targetRoomId || !targetTimeSlotId}
                        >
                          Execute Move
                        </button>
                      </div>
                    )}

                    {/* Swap Section */}
                    {currentVerStatus === 'DRAFT' && (
                      <div className="space-y-2 border-t border-slate-700 pt-3">
                        <div className="font-semibold text-slate-200 flex items-center gap-1">
                          <ArrowRightLeft className="w-3.5 h-3.5 text-purple-400" /> Swap Placements
                        </div>

                        {swapAssignment ? (
                          <div className="bg-purple-950/40 border border-purple-800 p-2 rounded text-purple-200 space-y-1">
                            <div>Selected Swap Target:</div>
                            <div className="font-bold text-xs">{getAsgnAssignmentId(swapAssignment)}</div>
                            <div>Room: {getRoomName(getAsgnRoomId(swapAssignment))}</div>
                            <div>Slot: {getSlotLabel(getAsgnSlotId(swapAssignment))}</div>
                          </div>
                        ) : (
                          <p className="text-[11px] text-slate-400">
                            Click another assignment in the timetable grid to select it as the swap target.
                          </p>
                        )}

                        <button
                          className="btn-secondary w-full text-xs justify-center mt-2"
                          onClick={handleExecuteSwap}
                          disabled={!swapAssignment}
                        >
                          Execute Swap
                        </button>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="text-slate-500 text-xs py-6 text-center">
                    Select any assignment in the timetable grid to view details or perform a move/swap.
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
