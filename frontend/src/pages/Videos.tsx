import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Video, Paginated } from "../api/types";

const PAGE_SIZE = 50;

type VideoRow = Video & { status?: string };

export function Videos() {
  const [videos, setVideos] = useState<VideoRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [picking, setPicking] = useState(false);

  const fetchVideos = () => {
    const params = new URLSearchParams();
    params.set("page", String(page));
    params.set("page_size", String(PAGE_SIZE));
    params.set("sort", "date");
    params.set("order", "desc");

    api.get<Paginated<VideoRow>>(`/videos?${params}`).then((r) => {
      setVideos(r.data ?? []);
      setTotal(r.total ?? 0);
    }).catch(() => {});
  };

  useEffect(fetchVideos, [page]);

  const handlePickVideos = async () => {
    setPicking(true);
    try {
      const { session_id, picker_url } = await api.post<{ session_id: string; picker_url: string }>("/photos/picker/start");
      window.open(picker_url, "gpoptimizer_picker", "width=800,height=600");

      const poll = async (): Promise<void> => {
        const { ready } = await api.get<{ ready: boolean }>(`/photos/picker/poll?session_id=${session_id}`);
        if (ready) {
          fetchVideos();
          return;
        }
        await new Promise((r) => setTimeout(r, 2000));
        return poll();
      };
      await poll();
    } catch {
      // ignore
    } finally {
      setPicking(false);
    }
  };

  const handleClearVideos = async () => {
    if (!confirm("Clear all videos from the list?")) return;
    await api.del("/videos");
    setSelected(new Set());
    fetchVideos();
  };

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

  const [playingVideo, setPlayingVideo] = useState<string | null>(null);

  const allSelected = videos.length > 0 && videos.every((v) => selected.has(v.id));
  const start = (page - 1) * PAGE_SIZE + 1;
  const end = Math.min(page * PAGE_SIZE, total);
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text-primary">Videos</h1>
        <div className="flex gap-3">
          {videos.length > 0 && (
            <button
              onClick={handleClearVideos}
              className="bg-bg-secondary border border-border hover:bg-bg-hover rounded-lg px-4 py-2 text-text-primary font-medium text-sm"
            >
              Clear All
            </button>
          )}
          <button
            disabled={picking}
            onClick={handlePickVideos}
            className="bg-bg-secondary border border-border hover:bg-bg-hover rounded-lg px-4 py-2 text-text-primary font-medium disabled:opacity-40"
          >
            {picking ? "Picking…" : "Pick Videos from Google Photos"}
          </button>
          <button
            disabled={selected.size === 0}
            onClick={handleOptimize}
            className="bg-accent hover:bg-accent-hover rounded-lg px-4 py-2 text-white font-medium disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Optimize Selected ({selected.size})
          </button>
        </div>
      </div>

      {/* Select all */}
      {videos.length > 0 && (
        <label className="flex items-center gap-2 text-sm text-text-secondary cursor-pointer">
          <input type="checkbox" checked={allSelected} onChange={toggleAll} className="accent-accent" />
          Select all
        </label>
      )}

      {/* Video grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
        {videos.map((v) => (
          <div
            key={v.id}
            onClick={() => toggleSelect(v.id)}
            className={`relative bg-bg-card rounded-lg border overflow-hidden cursor-pointer transition-all ${
              selected.has(v.id) ? "border-accent ring-2 ring-accent/30" : "border-border hover:border-text-muted"
            }`}
          >
            {/* Thumbnail / Player */}
            <div className="aspect-video bg-bg-secondary relative">
              {playingVideo === v.id && v.base_url ? (
                <video
                  autoPlay
                  controls
                  className="w-full h-full object-contain bg-black"
                  src={`${v.base_url}=dv`}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <>
                  {v.base_url ? (
                    <img
                      src={`${v.base_url}=w320-h180-c`}
                      alt={v.filename}
                      className="w-full h-full object-cover"
                      loading="lazy"
                      onError={(e) => {
                        (e.target as HTMLImageElement).style.display = "none";
                        (e.target as HTMLImageElement).nextElementSibling?.classList.remove("hidden");
                      }}
                    />
                  ) : null}
                  <button
                    className={`absolute inset-0 flex items-center justify-center ${v.base_url ? "hidden" : ""}`}
                    onClick={(e) => { e.stopPropagation(); if (v.base_url) setPlayingVideo(v.id); }}
                  >
                    <span className="text-text-muted text-3xl">▶</span>
                  </button>
                  {v.base_url && (
                    <button
                      className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 hover:opacity-100 transition-opacity"
                      onClick={(e) => { e.stopPropagation(); setPlayingVideo(v.id); }}
                    >
                      <span className="text-white text-4xl drop-shadow-lg">▶</span>
                    </button>
                  )}
                </>
              )}
            </div>
            {/* Checkbox overlay */}
            <div className="absolute top-2 left-2">
              <input
                type="checkbox"
                checked={selected.has(v.id)}
                onChange={() => toggleSelect(v.id)}
                onClick={(e) => e.stopPropagation()}
                className="accent-accent w-4 h-4"
              />
            </div>
            {/* Filename */}
            <div className="p-2">
              <p className="text-xs text-text-primary truncate" title={v.filename}>{v.filename}</p>
            </div>
          </div>
        ))}
        {videos.length === 0 && (
          <div className="col-span-full bg-bg-card rounded-lg border border-border p-8 text-center text-text-muted">
            No videos found. Pick videos from Google Photos to get started.
          </div>
        )}
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
