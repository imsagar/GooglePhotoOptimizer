import { useEffect, useRef, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { api } from "../api/client";
import type { Job, Paginated } from "../api/types";
import { ProgressBar } from "../components/ProgressBar";

type Filter = "all" | "local" | "uploaded" | "active";

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

function formatDate(iso?: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

interface VideoGroup {
  videoId: string;
  filename: string;
  jobs: Job[];
}

function groupByVideo(jobs: Job[]): VideoGroup[] {
  const map = new Map<string, VideoGroup>();
  for (const j of jobs) {
    let g = map.get(j.video_id);
    if (!g) {
      g = { videoId: j.video_id, filename: j.filename || j.video_id, jobs: [] };
      map.set(j.video_id, g);
    }
    if (j.filename) g.filename = j.filename;
    g.jobs.push(j);
  }
  return Array.from(map.values());
}

function isActive(j: Job) {
  if (j.status === "encoding") return true;
  if (j.status === "downloading" && j.progress < 100) return true;
  return false;
}

function bestStatus(jobs: Job[]): string {
  if (jobs.some(isActive)) return "active";
  if (jobs.some((j) => j.status === "uploaded")) return "uploaded";
  if (jobs.some((j) => j.status === "ready")) return "ready";
  if (jobs.some((j) => j.status === "downloading" && j.progress >= 100)) return "ready";
  if (jobs.some((j) => j.status === "failed")) return "failed";
  return "queued";
}

function statusChips(jobs: Job[]) {
  const downloaded = jobs.some((j) => j.downloaded_at || ["encoding", "ready", "uploaded"].includes(j.status) || (j.status === "downloading" && j.progress >= 100));
  const optimized = jobs.filter((j) => j.status === "ready" || j.status === "uploaded").length;
  const uploaded = jobs.filter((j) => j.status === "uploaded").length;
  const activeJob = jobs.find(isActive);
  const failed = jobs.some((j) => j.status === "failed");

  return (
    <div className="flex flex-wrap gap-1.5">
      {downloaded && (
        <span className="text-[11px] px-2 py-0.5 rounded-full bg-blue-900/30 text-blue-400 font-medium">Downloaded</span>
      )}
      {optimized > 0 && (
        <span className="text-[11px] px-2 py-0.5 rounded-full bg-green-900/30 text-green-400 font-medium">
          Optimized{optimized > 1 ? ` ×${optimized}` : ""}
        </span>
      )}
      {uploaded > 0 && (
        <span className="text-[11px] px-2 py-0.5 rounded-full bg-purple-900/30 text-purple-400 font-medium">
          Uploaded{uploaded > 1 ? ` ×${uploaded}` : ""}
        </span>
      )}
      {activeJob && (
        <span className="text-[11px] px-2 py-0.5 rounded-full bg-amber-900/30 text-amber-400 font-medium animate-pulse">
          {activeJob.status === "encoding" ? "Encoding" : "Downloading"} {activeJob.progress}%
        </span>
      )}
      {failed && (
        <span className="text-[11px] px-2 py-0.5 rounded-full bg-red-900/30 text-red-400 font-medium">Failed</span>
      )}
    </div>
  );
}

function bestSavings(jobs: Job[]): { original: number; optimized: number; pct: number } | null {
  const ready = jobs.filter((j) => (j.status === "ready" || j.status === "uploaded") && j.original_size > 0);
  if (ready.length === 0) return null;
  const best = ready.reduce((a, b) => (a.savings_pct > b.savings_pct ? a : b));
  return { original: best.original_size, optimized: best.optimized_size, pct: best.savings_pct };
}

export function Library() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [filter, setFilter] = useState<Filter>("all");
  const [expanded, setExpanded] = useState<string | null>(null);
  const [reEncodeVideo, setReEncodeVideo] = useState<string | null>(null);
  const [reEncodeOpts, setReEncodeOpts] = useState({ codec: "libx265", crf: 28, preset: "slow" });
  const [previewJob, setPreviewJob] = useState<number | null>(null);
  const { lastEvent } = useOutletContext<{ lastEvent: unknown }>();
  const fetchingRef = useRef(false);

  const fetchJobs = () => {
    if (fetchingRef.current) return;
    fetchingRef.current = true;
    api.get<Paginated<Job>>("/jobs?page_size=200").then((r) => setJobs(r.data ?? [])).catch(() => {}).finally(() => { fetchingRef.current = false; });
  };

  useEffect(() => {
    fetchJobs();
    const interval = setInterval(fetchJobs, 30000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (!lastEvent || typeof lastEvent !== "object" || lastEvent === null) return;
    const event = lastEvent as Record<string, unknown>;
    if (event.type === "progress") {
      setJobs((prev) => {
        const exists = prev.some((j) => j.id === event.job_id);
        if (!exists) {
          fetchJobs();
          return prev;
        }
        return prev.map((j) =>
          j.id === event.job_id
            ? {
                ...j,
                progress: typeof event.percent === "number" ? event.percent : j.progress,
                status: typeof event.stage === "string" ? (event.stage as Job["status"]) : j.status,
              }
            : j
        );
      });
    } else if (event.type === "job_complete") {
      setJobs((prev) =>
        prev.map((j) =>
          j.id === event.job_id
            ? {
                ...j,
                status: "ready" as Job["status"],
                progress: 100,
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
          j.id === event.job_id
            ? { ...j, status: "uploaded" as Job["status"], progress: 100, drive_file_id: (event.drive_file_id as string) ?? j.drive_file_id }
            : j
        )
      );
    }
  }, [lastEvent]);

  const groups = groupByVideo(jobs);

  const filtered = groups.filter((g) => {
    if (filter === "local") return g.jobs.some((j) => j.status === "ready");
    if (filter === "uploaded") return g.jobs.some((j) => j.status === "uploaded");
    if (filter === "active") return g.jobs.some((j) => j.status === "downloading" || j.status === "encoding");
    return true;
  });

  const totalSaved = jobs.reduce((s, j) => s + Math.max(0, j.original_size - j.optimized_size), 0);
  const readyCount = jobs.filter((j) => j.status === "ready").length;

  const handleUpload = async (job: Job) => {
    await api.post(`/jobs/${job.id}/upload`, { delete_original: false });
  };

  const handleUploadAll = async () => {
    await api.post("/jobs/upload-all");
  };

  const handleReEncode = async (job: Job) => {
    await api.post(`/jobs/${job.id}/re-encode`, reEncodeOpts);
    setReEncodeVideo(null);
    setJobs((prev) =>
      prev.map((j) =>
        j.id === job.id
          ? { ...j, status: "encoding" as Job["status"], progress: 0, codec: reEncodeOpts.codec, crf: reEncodeOpts.crf, preset: reEncodeOpts.preset }
          : j
      )
    );
  };

  const handleClearStuck = async () => {
    if (!confirm("Clear failed and stuck jobs?")) return;
    await Promise.all([
      api.del("/jobs?status=failed"),
      api.del("/jobs?status=downloading"),
      api.del("/jobs?status=encoding"),
    ]);
    fetchJobs();
  };

  const tabs: { key: Filter; label: string }[] = [
    { key: "all", label: `All (${groups.length})` },
    { key: "active", label: "In Progress" },
    { key: "local", label: "Ready to Upload" },
    { key: "uploaded", label: "Uploaded" },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text-primary">Library</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={handleClearStuck}
            className="bg-bg-secondary border border-border rounded-lg px-3 py-1.5 text-text-secondary text-sm hover:bg-bg-hover"
          >
            Clear Stuck
          </button>
          {readyCount > 0 && (
            <button onClick={handleUploadAll} className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-2 text-white font-medium text-sm">
              Upload All ({readyCount})
            </button>
          )}
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <div className="bg-bg-card rounded-[10px] border border-border p-4">
          <p className="text-text-muted text-xs">Videos</p>
          <p className="text-xl font-semibold text-text-primary">{groups.length}</p>
        </div>
        <div className="bg-bg-card rounded-[10px] border border-border p-4">
          <p className="text-text-muted text-xs">Total Saved</p>
          <p className="text-xl font-semibold text-accent">{formatSize(totalSaved)}</p>
        </div>
        <div className="bg-bg-card rounded-[10px] border border-border p-4">
          <p className="text-text-muted text-xs">Ready to Upload</p>
          <p className="text-xl font-semibold text-text-primary">{readyCount}</p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-1">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setFilter(t.key)}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              filter === t.key ? "bg-bg-card text-accent" : "text-text-secondary hover:text-text-primary"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Video cards */}
      <div className="grid gap-3">
        {filtered.map((g) => {
          const isExpanded = expanded === g.videoId;
          const savings = bestSavings(g.jobs);
          const latestJob = g.jobs[0];
          const status = bestStatus(g.jobs);

          return (
            <div key={g.videoId} className="bg-bg-card rounded-[10px] border border-border overflow-hidden">
              {/* Card header — always visible */}
              <button
                onClick={() => setExpanded(isExpanded ? null : g.videoId)}
                className="w-full flex items-center gap-4 p-4 text-left hover:bg-bg-hover transition-colors"
              >
                {/* Thumbnail */}
                <div className="w-24 h-16 rounded-lg bg-bg-secondary overflow-hidden shrink-0 flex items-center justify-center">
                  <video
                    src={`http://localhost:9090/files/${latestJob.run_date}/originals/${encodeURIComponent(latestJob.filename || latestJob.video_id + '.mp4')}`}
                    className="w-full h-full object-cover"
                    muted
                    preload="metadata"
                  />
                </div>

                <div className="flex-1 min-w-0">
                  <p className="text-text-primary font-medium truncate">{g.filename}</p>
                  <div className="mt-1">{statusChips(g.jobs)}</div>
                </div>

                {/* Savings */}
                <div className="text-right shrink-0">
                  {savings && (
                    <>
                      <p className="text-sm text-text-secondary">
                        {formatSize(savings.original)} → {formatSize(savings.optimized)}
                      </p>
                      <p className="text-sm font-semibold text-accent">{Math.round(savings.pct)}% saved</p>
                    </>
                  )}
                  {status === "active" && (
                    <div className="w-32">
                      <ProgressBar
                        percent={latestJob.progress}
                        stage={latestJob.status === "encoding" ? "encoding" : "downloading"}
                      />
                    </div>
                  )}
                </div>

                {/* Expand chevron */}
                <svg
                  className={`w-5 h-5 text-text-muted transition-transform ${isExpanded ? "rotate-180" : ""}`}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              {/* Expanded detail */}
              {isExpanded && (
                <div className="border-t border-border p-4 space-y-4">
                  {/* Timeline */}
                  <div className="grid grid-cols-3 gap-3 text-xs">
                    <div>
                      <span className="text-text-muted">Downloaded</span>
                      <p className="text-text-secondary mt-0.5">
                        {formatDate(latestJob.downloaded_at || (latestJob.status !== "queued" ? latestJob.created_at : undefined))}
                      </p>
                    </div>
                    <div>
                      <span className="text-text-muted">Optimized</span>
                      <p className="text-text-secondary mt-0.5">{formatDate(latestJob.optimized_at)}</p>
                    </div>
                    <div>
                      <span className="text-text-muted">Uploaded</span>
                      <p className="text-text-secondary mt-0.5">
                        {formatDate(g.jobs.find((j) => j.uploaded_at)?.uploaded_at)}
                      </p>
                    </div>
                  </div>

                  {/* Job history */}
                  <div>
                    <h3 className="text-xs text-text-muted uppercase font-medium mb-2">Optimizations ({g.jobs.length})</h3>
                    <div className="space-y-2">
                      {g.jobs.map((job) => (
                        <div key={job.id} className="bg-bg-secondary rounded-lg p-3 border border-border">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                              <span className="text-xs text-text-muted">#{job.id}</span>
                              <span className="text-xs text-text-secondary">{job.codec} / CRF {job.crf} / {job.preset}</span>
                              <StatusPill status={job.status} progress={job.progress} />
                            </div>
                            <div className="flex items-center gap-2">
                              {job.status === "ready" && (
                                <>
                                  <button
                                    onClick={() => setPreviewJob(previewJob === job.id ? null : job.id)}
                                    className="text-xs px-2.5 py-1 rounded-md border border-border text-text-secondary hover:bg-bg-hover"
                                  >
                                    {previewJob === job.id ? "Hide" : "Preview"}
                                  </button>
                                  <button
                                    onClick={() => handleUpload(job)}
                                    className="text-xs px-2.5 py-1 rounded-md bg-accent hover:bg-accent-hover text-white font-medium"
                                  >
                                    Upload
                                  </button>
                                </>
                              )}
                              {job.status === "uploaded" && (
                                <>
                                  {job.drive_file_id && (
                                    <a
                                      href={`https://drive.google.com/file/d/${job.drive_file_id}/view`}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      className="text-xs text-accent hover:underline"
                                    >
                                      View in Drive
                                    </a>
                                  )}
                                  <button
                                    onClick={() => handleUpload(job)}
                                    className="text-xs px-2.5 py-1 rounded-md border border-border text-text-secondary hover:bg-bg-hover"
                                  >
                                    Re-upload
                                  </button>
                                </>
                              )}
                            </div>
                          </div>

                          {/* Progress bar for active jobs */}
                          {isActive(job) && (
                            <div className="mt-2">
                              <ProgressBar percent={job.progress} stage={job.status} />
                            </div>
                          )}

                          {/* Size info */}
                          {(job.status === "ready" || job.status === "uploaded") && job.original_size > 0 && (
                            <div className="mt-2 flex items-center gap-3 text-xs text-text-secondary">
                              <span>{formatSize(job.original_size)} → {formatSize(job.optimized_size)}</span>
                              <span className="text-accent font-medium">{Math.round(job.savings_pct)}% saved</span>
                            </div>
                          )}

                          {/* Error */}
                          {job.status === "failed" && job.error && (
                            <p className="mt-2 text-xs text-red-400">{job.error}</p>
                          )}

                          {/* Preview */}
                          {previewJob === job.id && job.status === "ready" && (
                            <div className="mt-3 grid grid-cols-2 gap-3">
                              <div>
                                <p className="text-xs text-text-muted mb-1">Original</p>
                                <video
                                  controls
                                  className="w-full max-h-[240px] rounded-lg bg-black"
                                  src={`http://localhost:9090/files/${job.run_date}/originals/${encodeURIComponent(job.filename || job.video_id + '.mp4')}`}
                                />
                              </div>
                              <div>
                                <p className="text-xs text-text-muted mb-1">Optimized</p>
                                <video
                                  controls
                                  className="w-full max-h-[240px] rounded-lg bg-black"
                                  src={`http://localhost:9090/files/${job.run_date}/optimized/${encodeURIComponent(((job.filename || job.video_id + '.mp4').replace(/\.[^.]+$/, '')) + '-o.mp4')}`}
                                />
                              </div>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Re-optimize */}
                  <div>
                    {reEncodeVideo === g.videoId ? (
                      <div className="flex items-center gap-3 bg-bg-secondary rounded-lg p-3 border border-border">
                        <label className="text-xs text-text-secondary">
                          Codec
                          <select
                            value={reEncodeOpts.codec}
                            onChange={(e) => setReEncodeOpts((o) => ({ ...o, codec: e.target.value }))}
                            className="ml-1 bg-bg-card border border-border rounded px-2 py-1 text-text-primary text-xs"
                          >
                            <option value="libx265">H.265</option>
                            <option value="libx264">H.264</option>
                          </select>
                        </label>
                        <label className="text-xs text-text-secondary">
                          CRF
                          <input
                            type="number"
                            min={18}
                            max={51}
                            value={reEncodeOpts.crf}
                            onChange={(e) => setReEncodeOpts((o) => ({ ...o, crf: Number(e.target.value) }))}
                            className="ml-1 w-14 bg-bg-card border border-border rounded px-2 py-1 text-text-primary text-xs"
                          />
                        </label>
                        <label className="text-xs text-text-secondary">
                          Preset
                          <select
                            value={reEncodeOpts.preset}
                            onChange={(e) => setReEncodeOpts((o) => ({ ...o, preset: e.target.value }))}
                            className="ml-1 bg-bg-card border border-border rounded px-2 py-1 text-text-primary text-xs"
                          >
                            <option value="ultrafast">Ultrafast</option>
                            <option value="fast">Fast</option>
                            <option value="medium">Medium</option>
                            <option value="slow">Slow</option>
                            <option value="veryslow">Very Slow</option>
                          </select>
                        </label>
                        <button
                          onClick={() => {
                            const readyJob = g.jobs.find((j) => j.status === "ready" || j.status === "uploaded");
                            if (readyJob) handleReEncode(readyJob);
                          }}
                          className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-1.5 text-white font-medium text-xs"
                        >
                          Start
                        </button>
                        <button
                          onClick={() => setReEncodeVideo(null)}
                          className="text-xs text-text-muted hover:text-text-primary"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setReEncodeVideo(g.videoId)}
                        className="text-xs px-3 py-1.5 rounded-md border border-border text-text-secondary hover:bg-bg-hover"
                      >
                        + Re-optimize
                      </button>
                    )}
                  </div>

                  {/* File paths */}
                  <div className="text-[11px] text-text-muted font-mono space-y-0.5">
                    <p>Original: ~/GooglePhotosOptimized/{latestJob.run_date}/originals/{g.videoId}.mp4</p>
                    <p>Optimized: ~/GooglePhotosOptimized/{latestJob.run_date}/optimized/{g.videoId}-o.mp4</p>
                  </div>
                </div>
              )}
            </div>
          );
        })}

        {filtered.length === 0 && (
          <div className="bg-bg-card rounded-[10px] border border-border p-12 text-center text-text-muted">
            {filter === "all" ? "No videos processed yet. Pick videos from the Videos page to get started." : "No videos match this filter."}
          </div>
        )}
      </div>
    </div>
  );
}

function StatusPill({ status, progress }: { status: string; progress?: number }) {
  const display = status === "downloading" && (progress ?? 0) >= 100 ? "downloaded" : status;
  const styles: Record<string, string> = {
    queued: "bg-gray-800 text-gray-400",
    downloading: "bg-blue-900/30 text-blue-400",
    downloaded: "bg-blue-900/30 text-blue-400",
    encoding: "bg-amber-900/30 text-amber-400",
    ready: "bg-green-900/30 text-green-400",
    uploading: "bg-purple-900/30 text-purple-400",
    uploaded: "bg-purple-900/30 text-purple-400",
    failed: "bg-red-900/30 text-red-400",
    cancelled: "bg-gray-800 text-gray-400",
  };
  return (
    <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${styles[display] || styles.queued}`}>
      {display}
    </span>
  );
}
