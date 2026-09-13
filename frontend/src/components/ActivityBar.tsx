import { useEffect, useRef, useState } from "react";

interface LogEntry {
  id: number;
  time: string;
  type: string;
  stage?: string;
  percent?: number;
  jobId?: number;
  message?: string;
}

function formatEvent(event: Record<string, unknown>): LogEntry | null {
  const now = new Date().toLocaleTimeString();
  const base = { time: now, jobId: event.job_id as number | undefined };

  switch (event.type) {
    case "progress":
      return { ...base, id: Date.now(), type: "progress", stage: event.stage as string, percent: event.percent as number };
    case "job_complete":
      return { ...base, id: Date.now(), type: "job_complete", message: `Job complete — ${fmtSize(event.original_size as number)} → ${fmtSize(event.optimized_size as number)}` };
    case "upload_complete":
      return { ...base, id: Date.now(), type: "upload_complete", message: `Upload complete${event.size_verified ? " (verified)" : ""}` };
    case "error":
      return { ...base, id: Date.now(), type: "error", message: event.message as string };
    case "google_auth_status":
      return { ...base, id: Date.now(), type: "info", message: "Google Photos connected" };
    case "delete_local_complete":
      return { ...base, id: Date.now(), type: "info", message: `Cleaned up ${event.count} local file(s)` };
    default:
      return null;
  }
}

function fmtSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

const stageLabels: Record<string, string> = {
  downloading: "Downloading",
  encoding: "Encoding",
  uploading: "Uploading",
};

const stageColors: Record<string, string> = {
  downloading: "bg-status-progress",
  encoding: "bg-status-encoding",
  uploading: "bg-status-progress",
};

export function ActivityBar({ lastEvent }: { lastEvent: unknown }) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [activeJob, setActiveJob] = useState<{ stage: string; percent: number; jobId?: number } | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!lastEvent || typeof lastEvent !== "object") return;
    const event = lastEvent as Record<string, unknown>;
    const entry = formatEvent(event);
    if (!entry) return;

    if (entry.type === "progress" && entry.stage && entry.percent !== undefined) {
      setActiveJob({ stage: entry.stage, percent: entry.percent, jobId: entry.jobId });
      // Only log stage transitions, not every percent tick
      setLogs((prev) => {
        const last = prev[prev.length - 1];
        if (last?.type === "progress" && last?.stage === entry.stage && last?.jobId === entry.jobId) {
          return [...prev.slice(0, -1), entry];
        }
        return [...prev.slice(-49), entry];
      });
    } else {
      if (entry.type === "job_complete" || entry.type === "error") {
        setActiveJob(null);
      }
      setLogs((prev) => [...prev.slice(-49), entry]);
    }
  }, [lastEvent]);

  useEffect(() => {
    if (expanded && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, expanded]);

  if (logs.length === 0 && !activeJob) return null;

  const lastLog = logs[logs.length - 1];
  const summaryText = activeJob
    ? `${stageLabels[activeJob.stage] ?? activeJob.stage} ${activeJob.percent}%`
    : lastLog?.type === "error"
      ? `Error: ${lastLog.message}`
      : lastLog?.message ?? "Idle";

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50">
      {/* Expanded log panel */}
      {expanded && (
        <div className="bg-bg-card border-t border-x border-border mx-4 rounded-t-lg max-h-64 overflow-hidden flex flex-col">
          <div ref={scrollRef} className="overflow-y-auto p-3 space-y-1 text-xs font-mono">
            {logs.map((entry) => (
              <div key={entry.id} className="flex gap-2">
                <span className="text-text-muted shrink-0">{entry.time}</span>
                {entry.type === "error" ? (
                  <span className="text-status-failed">{entry.message}</span>
                ) : entry.type === "progress" ? (
                  <span className="text-text-secondary">
                    {stageLabels[entry.stage!] ?? entry.stage} {entry.percent}%
                    {entry.jobId ? ` (job ${entry.jobId})` : ""}
                  </span>
                ) : entry.type === "job_complete" ? (
                  <span className="text-status-uploaded">{entry.message}</span>
                ) : entry.type === "upload_complete" ? (
                  <span className="text-status-uploaded">{entry.message}</span>
                ) : (
                  <span className="text-text-secondary">{entry.message}</span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Collapsed summary bar */}
      <div
        onClick={() => setExpanded(!expanded)}
        className="bg-bg-secondary border-t border-border px-4 py-2 flex items-center gap-3 cursor-pointer hover:bg-bg-hover select-none"
      >
        <span className="text-text-muted text-xs shrink-0">{expanded ? "▼" : "▲"}</span>

        {activeJob && (
          <div className="flex-1 max-w-xs flex items-center gap-2">
            <div className="flex-1 h-1.5 bg-border rounded-full overflow-hidden">
              <div
                className={`h-full ${stageColors[activeJob.stage] ?? "bg-status-progress"} rounded-full transition-all`}
                style={{ width: `${activeJob.percent}%` }}
              />
            </div>
          </div>
        )}

        <span className={`text-xs font-medium truncate ${lastLog?.type === "error" ? "text-status-failed" : "text-text-secondary"}`}>
          {summaryText}
        </span>

        <button
          onClick={(e) => { e.stopPropagation(); setLogs([]); setActiveJob(null); }}
          className="text-text-muted hover:text-text-primary text-xs ml-auto"
        >
          Clear
        </button>
      </div>
    </div>
  );
}
