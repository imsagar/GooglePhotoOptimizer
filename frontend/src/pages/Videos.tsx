import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Video, Paginated } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";

const PAGE_SIZE = 50;

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m ${sec}s`;
}

type VideoRow = Video & { status?: string };

export function Videos() {
  const [videos, setVideos] = useState<VideoRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sort, setSort] = useState("size");
  const [order, setOrder] = useState("desc");
  const [minSize, setMinSize] = useState<number | null>(null);
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const fetchVideos = () => {
    const params = new URLSearchParams();
    params.set("page", String(page));
    params.set("page_size", String(PAGE_SIZE));
    params.set("sort", sort);
    params.set("order", order);
    if (minSize) params.set("min_size", String(minSize));
    if (dateFrom) params.set("from", dateFrom);
    if (dateTo) params.set("to", dateTo);

    api.get<Paginated<VideoRow>>(`/videos?${params}`).then((r) => {
      setVideos(r.data);
      setTotal(r.total);
    }).catch(() => {});
  };

  useEffect(fetchVideos, [page, sort, order, minSize, dateFrom, dateTo]);

  const handleOptimize = async () => {
    await api.post("/jobs", {
      video_ids: Array.from(selected),
      codec: "libx265",
      crf: 28,
      preset: "medium",
    });
    setSelected(new Set());
    fetchVideos();
  };

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleAll = () => {
    if (videos.every((v) => selected.has(v.id))) {
      setSelected((prev) => {
        const next = new Set(prev);
        videos.forEach((v) => next.delete(v.id));
        return next;
      });
    } else {
      setSelected((prev) => {
        const next = new Set(prev);
        videos.forEach((v) => next.add(v.id));
        return next;
      });
    }
  };

  const allSelected = videos.length > 0 && videos.every((v) => selected.has(v.id));
  const start = (page - 1) * PAGE_SIZE + 1;
  const end = Math.min(page * PAGE_SIZE, total);
  const totalPages = Math.ceil(total / PAGE_SIZE);

  const inputClass = "bg-bg-secondary border border-border rounded-lg px-3 py-2 text-text-primary text-sm";

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text-primary">Videos</h1>
        <button
          disabled={selected.size === 0}
          onClick={handleOptimize}
          className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-2 text-white font-medium disabled:opacity-40 disabled:cursor-not-allowed"
        >
          Optimize Selected ({selected.size})
        </button>
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-3">
        <select value={sort} onChange={(e) => { setSort(e.target.value); setPage(1); }} className={inputClass}>
          <option value="size">Sort: Size</option>
          <option value="date">Sort: Date</option>
          <option value="duration">Sort: Duration</option>
          <option value="name">Sort: Name</option>
        </select>
        <button
          onClick={() => { setOrder(order === "desc" ? "asc" : "desc"); setPage(1); }}
          className={inputClass + " cursor-pointer"}
        >
          {order === "desc" ? "↓ Desc" : "↑ Asc"}
        </button>
        <select
          value={minSize ?? ""}
          onChange={(e) => { setMinSize(e.target.value ? Number(e.target.value) : null); setPage(1); }}
          className={inputClass}
        >
          <option value="">Min: Any size</option>
          <option value="104857600">Min: 100 MB</option>
          <option value="524288000">Min: 500 MB</option>
          <option value="1073741824">Min: 1 GB</option>
        </select>
        <input
          type="date"
          value={dateFrom}
          onChange={(e) => { setDateFrom(e.target.value); setPage(1); }}
          placeholder="From"
          className={inputClass}
        />
        <input
          type="date"
          value={dateTo}
          onChange={(e) => { setDateTo(e.target.value); setPage(1); }}
          placeholder="To"
          className={inputClass}
        />
      </div>

      {/* Table */}
      <div className="bg-bg-card rounded-[10px] border border-border overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="p-3 w-10">
                  <input type="checkbox" checked={allSelected} onChange={toggleAll} className="accent-accent" />
                </th>
                <th className="p-3 w-12"></th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Filename</th>
                <th className="p-3 text-right text-text-muted text-xs uppercase font-medium">Size</th>
                <th className="p-3 text-right text-text-muted text-xs uppercase font-medium">Duration</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Resolution</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Album</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Date</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {videos.map((v) => (
                <tr
                  key={v.id}
                  onClick={() => toggleSelect(v.id)}
                  className={`border-b border-border cursor-pointer hover:bg-bg-hover ${selected.has(v.id) ? "bg-bg-hover" : ""}`}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selected.has(v.id)}
                      onChange={() => toggleSelect(v.id)}
                      onClick={(e) => e.stopPropagation()}
                      className="accent-accent"
                    />
                  </td>
                  <td className="p-3">
                    <div className="w-8 h-8 rounded bg-bg-secondary flex items-center justify-center text-text-muted text-xs">▶</div>
                  </td>
                  <td className="p-3 text-text-primary truncate max-w-[200px]">{v.filename}</td>
                  <td className="p-3 text-right text-text-secondary">{formatSize(v.size_bytes)}</td>
                  <td className="p-3 text-right text-text-secondary">{formatDuration(v.duration_ms)}</td>
                  <td className="p-3 text-text-secondary">{v.width}×{v.height}</td>
                  <td className="p-3 text-text-secondary">{v.album_title ?? "—"}</td>
                  <td className="p-3 text-text-secondary">{v.creation_time ? new Date(v.creation_time).toLocaleDateString() : "—"}</td>
                  <td className="p-3">{v.status ? <StatusBadge status={v.status} /> : <span className="text-text-muted text-xs">—</span>}</td>
                </tr>
              ))}
              {videos.length === 0 && (
                <tr>
                  <td colSpan={9} className="p-8 text-center text-text-muted">No videos found</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between text-sm text-text-secondary">
          <span>Showing {start}–{end} of {total} videos</span>
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
