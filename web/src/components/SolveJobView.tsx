import { useState } from 'react';
import { api, HTTPError } from '../api/client';
import type { SolveJobResult } from '../api/types';
import { Play, AlertTriangle, CheckCircle2, Hash } from 'lucide-react';

interface Props {
  timetableId: string;
  onComplete: (result: SolveJobResult) => void;
}

/**
 * Phase 1 minimal solve-job panel.
 * Submits a synchronous vertical slice job to the application API and
 * renders the canonical result, including verification status and the
 * engine-version-tagged metadata that proves the run reached Engine V1.
 */
export default function SolveJobView({ timetableId, onComplete }: Props) {
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<SolveJobResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function runJob() {
    if (!timetableId) return;
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const resp = await api.createSolveJob(timetableId);
      setResult(resp.result);
      onComplete(resp.result);
    } catch (e) {
      if (e instanceof HTTPError) {
        setError(`${e.status} ${e.code}: ${e.message}`);
      } else {
        setError((e as Error).message);
      }
    } finally {
      setRunning(false);
    }
  }

  if (error) {
    return (
      <div className="bg-rose-950/30 border border-rose-800 rounded p-4 space-y-2">
        <div className="flex items-center gap-2 text-rose-300 font-semibold">
          <AlertTriangle className="w-4 h-4" />
          Solve job failed
        </div>
        <p className="text-xs text-rose-200 font-mono">{error}</p>
      </div>
    );
  }

  return (
    <div className="bg-slate-800/80 border border-slate-700 rounded-lg p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-bold text-slate-200 uppercase">Phase 1 Vertical Slice</h3>
        <button
          className="btn-primary text-xs"
          onClick={runJob}
          disabled={running || !timetableId}
        >
          <Play className="w-3.5 h-3.5 fill-current" />
          {running ? 'Running…' : 'Run Vertical Slice'}
        </button>
      </div>

      {result && (
        <div className="space-y-3 text-xs">
          <div className="flex items-center gap-2">
            <CheckCircle2
              className={`w-4 h-4 ${result.verified ? 'text-emerald-400' : 'text-rose-400'}`}
            />
            <span className="font-semibold text-slate-200">
              {result.status} {result.verified ? '· verified' : '· not verified'}
            </span>
          </div>

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
