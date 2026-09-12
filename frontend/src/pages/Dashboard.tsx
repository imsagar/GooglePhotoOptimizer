import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Job, Paginated } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";
import { ProgressBar } from "../components/ProgressBar";
import { useWebSocket } from "../hooks/useWebSocket";

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

interface WSEvent {
  type: string;
  job_id?: number;
  percent?: number;
  stage?: string;
}

function isWSEvent(e: unknown): e is WSEvent {
  return typeof e === "object" && e !== null && "type" in e;
}

export function Dashboard() {
  const { runnerOnline, lastEvent } = useWebSocket();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [videoCount, setVideoCount] = useState(0);
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    api.get<Paginated<Job>>("/jobs?page_size=5").then((r) => setJobs(r.data ?? [])).catch(() => {});
    api.get<Paginated<unknown>>("/videos?page_size=1").then((r) => setVideoCount(r.total ?? 0)).catch(() => {});
  }, []);

  useEffect(() => {
    if (!lastEvent || !isWSEvent(lastEvent)) return;
    if (lastEvent.type === "progress" || lastEvent.type === "job_complete") {
      setJobs((prev) =>
        prev.map((j) =>
          j.id === lastEvent.job_id
            ? { ...j, progress: lastEvent.percent ?? j.progress, status: (lastEvent.stage as Job["status"]) ?? j.status }
            : j
        )
      );
    }
  }, [lastEvent]);

  const totalSaved = jobs.reduce((sum, j) => sum + (j.original_size - j.optimized_size), 0);
  const avgSavings = jobs.length > 0
    ? jobs.reduce((sum, j) => sum + j.savings_pct, 0) / jobs.length
    : 0;

  const handleSync = async () => {
    setSyncing(true);
    try { await api.post("/videos/sync"); } catch { /* ignore */ } finally { setSyncing(false); }
  };

  const isActive = (s: string) => s === "downloading" || s === "encoding";

  const stats = [
    { label: "Videos Synced", value: String(videoCount) },
    { label: "Space Saved", value: formatSize(totalSaved) },
    { label: "Average Savings", value: avgSavings.toFixed(1) + "%" },
    { label: "Runner Status", value: runnerOnline ? "Online" : "Offline" },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-text-primary">Dashboard</h1>
        <button
          onClick={handleSync}
          disabled={syncing}
          className="bg-accent hover:bg-accent-hover disabled:opacity-50 text-white font-medium px-4 py-2 rounded-lg transition-colors"
        >
          {syncing ? "Syncing…" : "Sync Videos"}
        </button>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((s) => (
          <div key={s.label} className="bg-bg-card rounded-[10px] border border-border p-5">
            <p className="text-text-muted text-sm mb-1">{s.label}</p>
            <p className="text-xl font-semibold text-text-primary">{s.value}</p>
          </div>
        ))}
      </div>

      <div className="bg-bg-card rounded-[10px] border border-border">
        <div className="p-5 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Recent Jobs</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-text-muted text-left border-b border-border">
                <th className="px-5 py-3 font-medium">Video</th>
                <th className="px-5 py-3 font-medium">Status</th>
                <th className="px-5 py-3 font-medium">Progress</th>
                <th className="px-5 py-3 font-medium text-right">Savings</th>
              </tr>
            </thead>
            <tbody>
              {jobs.length === 0 && (
                <tr><td colSpan={4} className="px-5 py-8 text-center text-text-muted">No jobs yet</td></tr>
              )}
              {jobs.map((j) => (
                <tr key={j.id} className="border-b border-border last:border-0 hover:bg-bg-hover transition-colors">
                  <td className="px-5 py-3 text-text-primary truncate max-w-[200px]">{j.video_id}</td>
                  <td className="px-5 py-3"><StatusBadge status={j.status} /></td>
                  <td className="px-5 py-3 w-40">
                    {isActive(j.status)
                      ? <ProgressBar percent={j.progress} stage={j.status} />
                      : <span className="text-text-muted text-xs">—</span>}
                  </td>
                  <td className="px-5 py-3 text-right text-text-secondary">
                    {j.savings_pct > 0 ? j.savings_pct.toFixed(1) + "%" : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
