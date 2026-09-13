import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Job, Paginated } from "../api/types";
import { useWebSocket } from "../hooks/useWebSocket";
import { StatusBadge } from "../components/StatusBadge";
import { ProgressBar } from "../components/ProgressBar";

type Filter = "all" | "active" | "ready" | "uploaded" | "failed";

const FILTER_STATUSES: Record<Filter, string[]> = {
  all: [],
  active: ["downloading", "encoding"],
  ready: ["ready"],
  uploaded: ["uploaded"],
  failed: ["failed"],
};

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

export function Jobs() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState<Filter>("all");
  const [page, setPage] = useState(1);
  const [deleteOriginals, setDeleteOriginals] = useState<Record<number, boolean>>({});
  const [previewJob, setPreviewJob] = useState<number | null>(null);
  const { lastEvent } = useWebSocket();

  const fetchJobs = () => {
    const statuses = FILTER_STATUSES[filter];
    if (statuses.length <= 1) {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", "20");
      if (statuses.length === 1) params.set("status", statuses[0]);
      api.get<Paginated<Job>>(`/jobs?${params}`).then((r) => {
        setJobs(r.data ?? []);
        setTotal(r.total ?? 0);
      }).catch(() => {});
    } else {
      Promise.all(
        statuses.map((s) => {
          const params = new URLSearchParams();
          params.set("page", String(page));
          params.set("page_size", "20");
          params.set("status", s);
          return api.get<Paginated<Job>>(`/jobs?${params}`);
        })
      ).then((results) => {
        const merged = results.flatMap((r) => r.data ?? []);
        const totalCount = results.reduce((sum, r) => sum + (r.total ?? 0), 0);
        setJobs(merged);
        setTotal(totalCount);
      }).catch(() => {});
    }
  };

  useEffect(fetchJobs, [filter, page]);

  // Real-time updates via WebSocket
  useEffect(() => {
    if (!lastEvent || typeof lastEvent !== "object" || lastEvent === null) return;
    const event = lastEvent as Record<string, unknown>;
    if (event.type === "progress" || event.type === "job_complete") {
      setJobs((prev) =>
        prev.map((j) =>
          j.id === event.job_id
            ? {
                ...j,
                progress: typeof event.percent === "number" ? event.percent : j.progress,
                status: typeof event.stage === "string" ? (event.stage as Job["status"]) : j.status,
                optimized_size: typeof event.optimized_size === "number" ? event.optimized_size : j.optimized_size,
                original_size: typeof event.original_size === "number" ? event.original_size : j.original_size,
              }
            : j
        )
      );
    } else if (event.type === "error") {
      setJobs((prev) =>
        prev.map((j) =>
          j.id === event.job_id
            ? { ...j, status: "failed" as Job["status"], progress: 0, error: (event.message as string) ?? "" }
            : j
        )
      );
    } else if (event.type === "upload_complete") {
      setJobs((prev) =>
        prev.map((j) =>
          j.id === event.job_id ? { ...j, status: "uploaded" as Job["status"], progress: 100 } : j
        )
      );
    }
  }, [lastEvent]);

  const handleUpload = async (job: Job) => {
    await api.post(`/jobs/${job.id}/upload`, { delete_original: deleteOriginals[job.id] ?? false });
    fetchJobs();
  };

  const handleCancel = async (job: Job) => {
    await api.post(`/jobs/${job.id}/cancel`);
    fetchJobs();
  };

  const handleUploadAll = async () => {
    await api.post("/jobs/upload-all");
    fetchJobs();
  };

  const tabs: { key: Filter; label: string }[] = [
    { key: "all", label: "All" },
    { key: "active", label: "In Progress" },
    { key: "ready", label: "Ready" },
    { key: "uploaded", label: "Uploaded" },
    { key: "failed", label: "Failed" },
  ];

  const readyCount = jobs.filter((j) => j.status === "ready").length;
  const totalPages = Math.ceil(total / 20);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text-primary">Jobs</h1>
        {readyCount > 0 && (
          <button onClick={handleUploadAll} className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-2 text-white font-medium">
            Upload All Ready ({readyCount})
          </button>
        )}
      </div>

      {/* Filter tabs */}
      <div className="flex gap-1">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => { setFilter(t.key); setPage(1); }}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              filter === t.key
                ? "bg-bg-card text-accent"
                : "text-text-secondary hover:text-text-primary"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Job cards */}
      <div className="grid gap-3">
        {jobs.map((job) => (
          <div key={job.id} className="bg-bg-card rounded-[10px] border border-border p-6">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-3">
                <span className="text-text-primary font-medium truncate max-w-[300px]">{job.video_id}</span>
                <StatusBadge status={job.status} />
              </div>
              <span className="text-text-muted text-xs">{job.codec} / CRF {job.crf} / {job.preset}</span>
            </div>

            {/* Progress for active jobs */}
            {(job.status === "downloading" || job.status === "encoding") && (
              <ProgressBar percent={job.progress} stage={job.status} />
            )}

            {/* Size info for completed jobs */}
            {(job.status === "ready" || job.status === "uploaded") && job.original_size > 0 && (
              <div className="flex items-center gap-4 text-sm text-text-secondary">
                <span>{formatSize(job.original_size)} → {formatSize(job.optimized_size)}</span>
                <span className="text-accent font-medium">
                  {job.savings_pct > 0 ? `${Math.round(job.savings_pct)}% saved` : `${Math.round((1 - job.optimized_size / job.original_size) * 100)}% saved`}
                </span>
              </div>
            )}

            {/* Error message */}
            {job.status === "failed" && job.error && (
              <p className="text-sm text-status-failed mt-1">{job.error}</p>
            )}

            {/* Video preview */}
            {job.status === "ready" && previewJob === job.id && (
              <div className="mt-3">
                <video
                  controls
                  className="w-full max-h-[400px] rounded-lg bg-black"
                  src={`http://localhost:9090/files/${job.run_date}/optimized/${job.video_id}_opt.mp4`}
                />
              </div>
            )}

            {/* Actions */}
            {job.status === "ready" && (
              <div className="flex items-center gap-3 mt-3">
                <button
                  onClick={() => setPreviewJob(previewJob === job.id ? null : job.id)}
                  className="bg-bg-secondary border border-border rounded-lg px-4 py-2 text-text-primary text-sm hover:bg-bg-hover"
                >
                  {previewJob === job.id ? "Hide Preview" : "Preview"}
                </button>
                <button onClick={() => handleUpload(job)} className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-2 text-white font-medium text-sm">
                  Upload to Google Photos
                </button>
                <label className="flex items-center gap-2 text-sm text-text-secondary cursor-pointer">
                  <input
                    type="checkbox"
                    checked={deleteOriginals[job.id] ?? false}
                    onChange={(e) => setDeleteOriginals((prev) => ({ ...prev, [job.id]: e.target.checked }))}
                    className="accent-accent"
                  />
                  Delete original after upload
                </label>
              </div>
            )}

            {job.status === "queued" && (
              <div className="mt-3">
                <button onClick={() => handleCancel(job)} className="bg-bg-secondary border border-border rounded-lg px-4 py-2 text-text-primary text-sm hover:bg-bg-hover">
                  Cancel
                </button>
              </div>
            )}
          </div>
        ))}

        {jobs.length === 0 && (
          <div className="bg-bg-card rounded-[10px] border border-border p-8 text-center text-text-muted">
            No jobs found
          </div>
        )}
      </div>

      {/* Pagination */}
      {total > 20 && (
        <div className="flex items-center justify-between text-sm text-text-secondary">
          <span>Page {page} of {totalPages} ({total} jobs)</span>
          <div className="flex gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="bg-bg-secondary border border-border rounded-lg px-3 py-1.5 text-text-primary disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <button
              disabled={page >= totalPages}
              onClick={() => setPage(page + 1)}
              className="bg-bg-secondary border border-border rounded-lg px-3 py-1.5 text-text-primary disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
