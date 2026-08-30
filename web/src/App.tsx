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

  // 1. Initial Load: Auth & Timetable Initialization
  useEffect(() => {
    initConsole();
  }, []);

  async function initConsole() {
    setErrorBanner(null);
    setLoadingText('Authenticating & loading environment...');
    try {
      const authData = await api.getMe();
      setAuth(authData);

      // Load Catalog Metadata
      const [roomsData, slotsData] = await Promise.all([
        api.listRooms().catch(() => []),
        api.listTimeSlots().catch(() => []),
      ]);
      setRooms(roomsData);
      setTimeSlots(slotsData);

      // Load or Create Timetable
      const timetablesData = await api.listTimetables();
      let tt = timetablesData[0];
      if (!tt) {
        tt = await api.createTimetable('Default Timetable Console');
      }
      setTimetable(tt);

      // Load Versions for Timetable
      await loadVersions(tt.id);
    } catch (err: any) {
      console.error('Initialization error:', err);
      setAuthError(err.message || 'Failed to connect to CURRA backend.');
    } finally {
      setLoadingText(null);
    }
  }

  async function loadVersions(timetableId: string, selectVersionId?: string) {
    try {
      const verList = await api.listVersions(timetableId);
      setVersions(verList);

      let targetVer = verList.find((v) => v.id === selectVersionId);
      if (!targetVer && verList.length > 0) {
        // Prefer latest published or draft
        targetVer = verList[0];
      }

      if (targetVer) {
        await selectVersion(targetVer.id);
      } else {
        setCurrentVersion(null);
        setAssignments([]);
      }
    } catch (err: any) {
      console.error('Load versions error:', err);
    }
  }

  async function selectVersion(versionId: string) {
    setLoadingText('Loading schedule version assignments...');
    try {
      const data = await api.getVersion(versionId);
      setCurrentVersion(data.version);
      setAssignments(data.assignments);
      setSelectedAssignment(null);
      setSwapAssignment(null);
    } catch (err: any) {
      setErrorBanner({ title: 'Load Version Error', message: err.message });
    } finally {
      setLoadingText(null);
    }
  }

  // 2. Run CURRA Solver Flow
  async function handleRunCurra() {
    if (!timetable) return;
    setErrorBanner(null);
    setInfoBanner(null);
    setIsSolving(true);
    setLoadingText('Creating problem snapshot from live database catalog...');

    try {
      // Step A: Create Snapshot
      const snap = await api.createSnapshot(timetable.id);
      
      // Step B: Create Solver Run
      setLoadingText('Enqueuing CURRA solver job...');
      const run = await api.createRun(timetable.id, snap.id);
      setCurrentRun(run);

      // Step C: Poll Run Status
      setLoadingText(`CURRA Solver running (Status: ${run.status})...`);
      let pollRun = run;

      while (pollRun.status === 'QUEUED' || pollRun.status === 'RUNNING') {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        pollRun = await api.getRun(run.id);
        setCurrentRun(pollRun);
        setLoadingText(`CURRA Solver optimizing (Status: ${pollRun.status})...`);
      }

      if (pollRun.status === 'SOLVED') {
        setLoadingText('Solver completed successfully! Generating draft schedule version...');
        // Step D: Create Draft Version
        const newVer = await api.createVersion(
          timetable.id,
          snap.id,
          pollRun.id,
          `Draft v${versions.length + 1} (Score: ${pollRun.score?.softPenalty ?? 'N/A'})`
        );

        setInfoBanner(`Run SOLVED! Created new version: ${newVer.name}`);
        await loadVersions(timetable.id, newVer.id);
      } else {
        setErrorBanner({
          title: `Solver Finished with Status: ${pollRun.status}`,
          message: pollRun.diagnostics?.message || `Solver failed with status ${pollRun.status}`,
          details: pollRun.violations,
        });
      }
    } catch (err: any) {
      console.error('Run CURRA error:', err);
      setErrorBanner({ title: 'Run CURRA Execution Error', message: err.message });
    } finally {
      setIsSolving(false);
      setLoadingText(null);
    }
  }

  // 3. Move Assignment Flow
  async function handleExecuteMove() {
    if (!currentVersion || !selectedAssignment || !targetRoomId || !targetTimeSlotId) return;
    setErrorBanner(null);
    setInfoBanner(null);

    const movePayload = {
      assignmentId: selectedAssignment.assignmentId,
      from: { roomId: selectedAssignment.roomId, timeSlotId: selectedAssignment.timeSlotId },
      to: { roomId: targetRoomId, timeSlotId: targetTimeSlotId },
    };

    try {
      const resp = await api.moveAssignment(currentVersion.id, movePayload, currentVersion.version);
      setInfoBanner(`Assignment move succeeded! Updated score: ${resp.version.score?.softPenalty ?? 0}`);
      
      // Refresh Version & Assignments
      await selectVersion(currentVersion.id);
    } catch (err: any) {
      handleMutationError(err, 'Move Assignment Failed');
    }
  }

  // 4. Swap Assignments Flow
  async function handleExecuteSwap() {
    if (!currentVersion || !selectedAssignment || !swapAssignment) return;
    setErrorBanner(null);
    setInfoBanner(null);

    const swapPayload = {
      assignment1Id: selectedAssignment.assignmentId,
      assignment2Id: swapAssignment.assignmentId,
      placement1: { roomId: selectedAssignment.roomId, timeSlotId: selectedAssignment.timeSlotId },
      placement2: { roomId: swapAssignment.roomId, timeSlotId: swapAssignment.timeSlotId },
    };

    try {
      const resp = await api.swapAssignments(currentVersion.id, swapPayload, currentVersion.version);
      setInfoBanner(`Assignments swapped successfully! Updated score: ${resp.version.score?.softPenalty ?? 0}`);

      await selectVersion(currentVersion.id);
    } catch (err: any) {
      handleMutationError(err, 'Swap Assignments Failed');
    }
  }

  // 5. Version State Machine Actions
  async function handleSubmitReview() {
    if (!currentVersion) return;
    try {
      const updated = await api.submitReview(currentVersion.id, currentVersion.version);
      setInfoBanner(`Version submitted for review (Status: REVIEW)`);
      await selectVersion(updated.id);
      await loadVersions(timetable!.id, updated.id);
    } catch (err: any) {
      handleMutationError(err, 'Submit Review Failed');
    }
  }

  async function handleSendBack() {
    if (!currentVersion) return;
    try {
      const updated = await api.sendBack(currentVersion.id, currentVersion.version);
      setInfoBanner(`Version sent back to draft (Status: DRAFT)`);
      await selectVersion(updated.id);
      await loadVersions(timetable!.id, updated.id);
    } catch (err: any) {
      handleMutationError(err, 'Send Back Failed');
    }
  }

  async function handlePublish() {
    if (!currentVersion) return;
    try {
      const updated = await api.publishVersion(currentVersion.id, currentVersion.version);
      setInfoBanner(`Version published successfully! (Status: PUBLISHED)`);
      await selectVersion(updated.id);
      await loadVersions(timetable!.id, updated.id);
    } catch (err: any) {
      handleMutationError(err, 'Publish Version Failed');
    }
  }

  function handleMutationError(err: any, title: string) {
    if (err instanceof HTTPError) {
      if (err.status === 409) {
        setErrorBanner({
          title: `${title}: 409 Conflict (Stale Version)`,
          message: 'The timetable version was modified concurrently by another request. Reloading latest state...',
        });
        if (currentVersion) selectVersion(currentVersion.id);
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

  // Helpers for mapping names
  const getRoomName = (id: string) => rooms.find((r) => r.id === id)?.name || id;
  const getSlotLabel = (id: string) => timeSlots.find((s) => s.id === id)?.label || id;

  const days = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'];
  const periods = [1, 2, 3, 4, 5, 6];

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

        {auth && (
          <div className="flex items-center gap-4 text-sm">
            <span className="text-slate-400">
              Institution: <strong className="text-slate-200">{auth.institution.name}</strong>
            </span>
            <span className="text-slate-400">
              User: <strong className="text-slate-200">{auth.user.name}</strong> ({auth.role})
            </span>
          </div>
        )}

        <button
          className="btn-primary"
          onClick={handleRunCurra}
          disabled={isSolving || !timetable}
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
        {timetable && (
          <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-4 grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Timetable Project</label>
              <div className="text-base font-bold text-slate-100 mt-1">{timetable.name}</div>
              <div className="text-xs text-slate-400 mt-0.5 font-mono">ID: {timetable.id}</div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Schedule Version</label>
              <div className="flex items-center gap-2 mt-1">
                <select
                  className="bg-slate-900 border border-slate-700 text-slate-200 text-sm rounded px-2 py-1 focus:outline-none focus:border-sky-500"
                  value={currentVersion?.id || ''}
                  onChange={(e) => selectVersion(e.target.value)}
                >
                  {versions.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.name} ({v.status}) - v{v.version}
                    </option>
                  ))}
                </select>
                {currentVersion && (
                  <span className={`badge badge-${currentVersion.status.toLowerCase()}`}>
                    {currentVersion.status}
                  </span>
                )}
              </div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Solver Status</label>
              <div className="text-base font-bold text-slate-100 mt-1 flex items-center gap-2">
                {currentRun ? (
                  <span className="text-sky-400">{currentRun.status}</span>
                ) : (
                  <span className="text-slate-400">IDLE</span>
                )}
              </div>
            </div>

            <div>
              <label className="text-xs text-slate-400 uppercase font-semibold">Backend Score</label>
              {currentVersion?.score ? (
                <div className="mt-1 space-y-0.5 text-xs">
                  <div className="text-sm font-bold text-emerald-400">
                    Hard Violations: {currentVersion.score.hardViolations} | Soft Penalty: {currentVersion.score.softPenalty}
                  </div>
                  {currentVersion.score.studentGapPenalty !== undefined && (
                    <div className="text-slate-400">
                      Gap: {currentVersion.score.studentGapPenalty} | Pref: {currentVersion.score.facultyPreferencePenalty} | RoomChange: {currentVersion.score.roomChangePenalty}
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-sm text-slate-500 mt-1">No score available</div>
              )}
            </div>
          </div>
        )}

        {/* Version Workflow State Machine Bar */}
        {currentVersion && (
          <div className="bg-slate-800/50 border border-slate-700/60 rounded-lg p-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm text-slate-300">
              <Layers className="w-4 h-4 text-sky-400" />
              <span>Version Workflow State:</span>
              <strong className="text-slate-100">{currentVersion.status}</strong>
              <span className="text-slate-500">(Version Counter: {currentVersion.version})</span>
            </div>

            <div className="flex items-center gap-3">
              {currentVersion.status === 'DRAFT' && (
                <button className="btn-secondary text-xs" onClick={handleSubmitReview}>
                  <Send className="w-3.5 h-3.5 text-amber-400" />
                  Submit Review
                </button>
              )}

              {currentVersion.status === 'REVIEW' && (
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

              {currentVersion.status === 'PUBLISHED' && (
                <span className="text-xs text-emerald-400 flex items-center gap-1 font-semibold">
                  <CheckCircle2 className="w-4 h-4" /> Published & Active Timetable
                </span>
              )}
            </div>
          </div>
        )}

        {/* Main Grid & Inspector Section */}
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* Timetable Grid (3 cols) */}
          <div className="lg:col-span-3 bg-slate-800/80 border border-slate-700 rounded-lg p-4 overflow-x-auto">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-slate-200">Weekly Schedule Grid</h2>
              <span className="text-xs text-slate-400">{assignments.length} Total Assignments</span>
            </div>

            {assignments.length === 0 ? (
              <div className="text-center py-12 text-slate-500 space-y-3">
                <p>No timetable generated for this version.</p>
                <button className="btn-primary mx-auto text-xs" onClick={handleRunCurra} disabled={isSolving}>
                  Run CURRA Solver
                </button>
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
                        // Find matching assignments for this Day & Period
                        const matched = assignments.filter((a) => {
                          const slot = timeSlots.find((s) => s.id === a.timeSlotId);
                          return slot && slot.day.toLowerCase() === day.toLowerCase() && slot.period === p;
                        });

                        return (
                          <td key={day}>
                            {matched.map((asgn) => {
                              const isSelected = selectedAssignment?.id === asgn.id;
                              const isSwapTarget = swapAssignment?.id === asgn.id;

                              return (
                                <div
                                  key={asgn.id}
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
                                    <span>{asgn.courseOfferingId}</span>
                                    <span className="text-slate-400 font-mono text-[10px]">#{asgn.instance}</span>
                                  </div>
                                  <div className="text-xs text-slate-300 font-medium mt-0.5">
                                    {getRoomName(asgn.roomId)}
                                  </div>
                                  <div className="text-[11px] text-slate-400 mt-0.5 truncate">
                                    Fac: {asgn.facultyId}
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
                    <div className="text-sky-400 font-bold text-sm">{selectedAssignment.assignmentId}</div>
                    <div>Course Offering: <span className="text-slate-200">{selectedAssignment.courseOfferingId}</span></div>
                    <div>Faculty ID: <span className="text-slate-200">{selectedAssignment.facultyId}</span></div>
                    <div>Student Group: <span className="text-slate-200">{selectedAssignment.studentGroupId}</span></div>
                    <div>Current Room: <span className="text-emerald-400 font-semibold">{getRoomName(selectedAssignment.roomId)}</span></div>
                    <div>Current Slot: <span className="text-amber-400 font-semibold">{getSlotLabel(selectedAssignment.timeSlotId)}</span></div>
                  </div>

                  {/* Move Section */}
                  {currentVersion?.status === 'DRAFT' && (
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
                            <option key={r.id} value={r.id}>
                              {r.name} (Cap: {r.capacity})
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
                            <option key={s.id} value={s.id}>
                              {s.label} ({s.day})
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
                  {currentVersion?.status === 'DRAFT' && (
                    <div className="space-y-2 border-t border-slate-700 pt-3">
                      <div className="font-semibold text-slate-200 flex items-center gap-1">
                        <ArrowRightLeft className="w-3.5 h-3.5 text-purple-400" /> Swap Placements
                      </div>

                      {swapAssignment ? (
                        <div className="bg-purple-950/40 border border-purple-800 p-2 rounded text-purple-200 space-y-1">
                          <div>Selected Swap Target:</div>
                          <div className="font-bold text-xs">{swapAssignment.assignmentId}</div>
                          <div>Room: {getRoomName(swapAssignment.roomId)}</div>
                          <div>Slot: {getSlotLabel(swapAssignment.timeSlotId)}</div>
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
      </main>
    </div>
  );
}
