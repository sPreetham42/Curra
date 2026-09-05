import { useEffect, useRef, useState } from 'react';
import { api, HTTPError } from '../api/client';
import type { SolveJob, SolveJobResult } from '../api/types';
import { Play, AlertTriangle, CheckCircle2, Hash, XCircle, Loader2 } from 'lucide-react';

interface Props {
  timetableId: string;
  onComplete: (result: SolveJobResult) => void;
}

const TERMINAL_STATUSES = new Set([
  'SOLVED',
  'INFEASIBLE',
  'INVALID_PROBLEM',
  'INVALID_RESULT',
  'FAILED',
  'CANCELLED',
  'DEADLINE_EXCEEDED',
  'NODE_LIMIT',
]);

/**
 * Phase 2 asynchronous solve-job panel.
 * Submits a job to POST /api/v1/solve-jobs (202 Accepted), then polls
 * GET /api/v1/solve-jobs/{id} until a terminal status is reached. On
 * terminal SOLVED status, fetches the canonical result.
 */
export default function SolveJobView({ timetableId, onComplete }: Props) {
  const [job, setJob] = useState<SolveJob | null>(null);
  const [result, setResult] = useState<SolveJobResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cancelledRef = useRef(false);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
    };
  }, []);

  async function runJob() {
    if (!timetableId) return;
    setSubmitting(true);
    setError(null);
    setResult(null);
    setJob(null);
    cancelledRef.current = false;
    try {
      const { runId } = await api.createSolveJob(timetableId);
      const initial = await api.getSolveJob(runId);
      setJob(initial);
      setSubmitting(false);
      pollUntilTerminal(runId);
    } catch (e) {
      setSubmitting(false);
      if (e instanceof HTTPError) {
        setError(`${e.status} ${e.code}: ${e.message}`);
      } else {
        setError((e as Error).message);
      }
    }
  }

  async function pollUntilTerminal(runId: string) {
    if (cancelledRef.current) return;
    try {
      const current = await api.getSolveJob(runId);
      if (cancelledRef.current) return;
      setJob(current);

      if (TERMINAL_STATUSES.has(current.status)) {
        if (current.status === 'SOLVED') {
          try {
            const r = await api.getSolveJobResult(runId);
            setResult(r);
            onComplete(r);
          } catch (e) {
            if (e instanceof HTTPError) {
              setError(`${e.code}: ${e.message}`);
            } else {
              setError((e as Error).message);
            }
          }
        } else {
          setError(`Job ended in status ${current.status}`);
        }
        return;
      }

      pollRef.current = setTimeout(() => pollUntilTerminal(runId), 1000);
    } catch (e) {
      if (e instanceof HTTPError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError((e as Error).message);
      }
    }
  }

  async function cancelJob() {
    if (!job) return;
    cancelledRef.current = true;
    if (pollRef.current) clearTimeout(pollRef.current);
    try {
      await api.cancelSolveJob(job.runId);
    } catch (e) {
      if (e instanceof HTTPError) {
        setError(`${e.code}: ${e.message}`);
      } else {
        setError((e as Error).message);
      }
    }
  }

  const isRunning = job && !TERMINAL_STATUSES.has(job.status);
  const isFailed = job && TERMINAL_STATUSES.has(job.status) && job.status !== 'SOLVED' && job.status !== 'CANCELLED';

  if (error && !job) {
    return (
      <div className="bg-rose-950/30 border border-rose-800 rounded p-4 space-y-2">
        <div className="flex items-center gap-2 text-rose-300 font-semibold">
          <AlertTriangle className="w-4 h-4" />
          Solve job failed to start
        </div>
        <p className="text-xs text-rose-200 font-mono">{error}</p>
        <button className="btn-primary text-xs" onClick={runJob}>
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-bold text-slate-200 uppercase">Async Solve Job</h3>
        <div className="flex items-center gap-2">
          {isRunning && (
            <button
              className="btn-secondary text-xs"
              onClick={cancelJob}
              data-testid="cancel-solve-job"
            >
              <XCircle className="w-3.5 h-3.5" />
              Cancel
            </button>
          )}
          <button
            className="btn-primary text-xs"
            onClick={runJob}
            disabled={submitting || isRunning || !timetableId}
          >
            {submitting || isRunning ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Play className="w-3.5 h-3.5 fill-current" />
            )}
            {submitting ? 'Submitting…' : isRunning ? 'Working…' : 'Run Solve Job'}
          </button>
        </div>
      </div>

      {job && (
        <div className="space-y-1 text-xs">
          <div className="flex items-center gap-2">
            {isRunning && <Loader2 className="w-4 h-4 text-sky-400 animate-spin" />}
            {job.status === 'SOLVED' && (
              <CheckCircle2 className="w-4 h-4 text-emerald-400" />
            )}
            {isFailed && <AlertTriangle className="w-4 h-4 text-rose-400" />}
            {job.status === 'CANCELLED' && <XCircle className="w-4 h-4 text-slate-400" />}
            <span className="font-semibold text-slate-200">
              {job.status}
              {job.verificationOk && job.status === 'SOLVED' ? ' · verified' : ''}
            </span>
          </div>
          <div className="text-[10px] text-slate-400 font-mono">runId: {job.runId}</div>
        </div>
      )}

      {result && (
        <div className="space-y-3 text-xs">
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-slate-900/60 p-2 rounded border border-slate-700/60">
              <div className="text-slate-400 text-[10px] uppercase">Assignments</div>
              <div className="text-slate-200 font-bold">{result.assignments.length}</div>
            </div>
            <div className="bg-slate-900/60 p-2 rounded border border-slate-700/60">
              <div className="text-slate-400 text-[10px] uppercase">Hard Violations</div>
              <div className={`font-bold ${result.hardViolations === 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                {result.hardViolations}
              </div>
            </div>
            <div className="bg-slate-900/60 p-2 rounded border border-slate-700/60">
              <div className="text-slate-400 text-[10px] uppercase">Soft Penalty</div>
              <div className="text-slate-200 font-bold">{result.softPenalty}</div>
            </div>
            <div className="bg-slate-900/60 p-2 rounded border border-slate-700/60">
              <div className="text-slate-400 text-[10px] uppercase">Seed</div>
              <div className="text-slate-200 font-bold font-mono">{result.metadata.seed}</div>
            </div>
          </div>

          <div className="space-y-1">
            <div className="flex items-center gap-1 text-slate-400 text-[10px] uppercase font-semibold">
              <Hash className="w-3 h-3" />
              Engine Metadata
            </div>
            <div className="bg-slate-900/60 p-2 rounded border border-slate-700/60 font-mono text-[10px] space-y-0.5">
              <div>engine: {result.metadata.engineVersion || '(unset)'}</div>
              <div>commit: {result.metadata.engineCommit || '(unset)'}</div>
              <div>adapter: {result.metadata.adapterVersion}</div>
              <div>ruleset: <span className="text-sky-300">{result.metadata.ruleSetHash.slice(0, 16)}…</span></div>
              <div>input: <span className="text-sky-300">{result.metadata.inputHash.slice(0, 16)}…</span></div>
            </div>
          </div>

          {result.assignments.length > 0 && (
            <div className="space-y-1">
              <div className="text-slate-400 text-[10px] uppercase font-semibold">Assignments</div>
              <table className="w-full text-[11px]">
                <thead>
                  <tr className="text-slate-400 text-left">
                    <th className="font-semibold">ID</th>
                    <th className="font-semibold">Room</th>
                    <th className="font-semibold">Slot</th>
                  </tr>
                </thead>
                <tbody>
                  {result.assignments.map((a) => (
                    <tr key={a.assignmentId} className="border-t border-slate-700/40">
                      <td className="py-1 text-sky-300 font-mono">{a.assignmentId}</td>
                      <td className="py-1 text-slate-200">{a.roomId}</td>
                      <td className="py-1 text-slate-200">{a.timeSlotId}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
