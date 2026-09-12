import { useEffect, useState } from "react";
import { api } from "../api/client";

interface LocalFile {
  path: string;
  filename: string;
  type: "original" | "optimized";
  size_bytes: number;
  upload_status: string;
  run_date: string;
}

type Filter = "all" | "original" | "optimized" | "safe";

function formatSize(bytes: number): string {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(0) + " MB";
  return (bytes / 1e3).toFixed(0) + " KB";
}

export function LocalFiles() {
  const [files, setFiles] = useState<LocalFile[]>([]);
  const [filter, setFilter] = useState<Filter>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirming, setConfirming] = useState(false);

  const fetchFiles = () => {
    api.get<LocalFile[]>("/cleanup/local-files").then(setFiles).catch(() => {});
  };

  useEffect(fetchFiles, []);

  const filtered = files.filter((f) => {
    if (filter === "original") return f.type === "original";
    if (filter === "optimized") return f.type === "optimized";
    if (filter === "safe") return f.upload_status === "uploaded";
    return true;
  });

  // Storage summary
  const totalSize = files.reduce((s, f) => s + f.size_bytes, 0);
  const originalsSize = files.filter((f) => f.type === "original").reduce((s, f) => s + f.size_bytes, 0);
  const optimizedSize = files.filter((f) => f.type === "optimized").reduce((s, f) => s + f.size_bytes, 0);

  const toggleSelect = (path: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path); else next.add(path);
      return next;
    });
  };

  const toggleAll = () => {
    if (filtered.every((f) => selected.has(f.path))) {
      setSelected((prev) => {
        const next = new Set(prev);
        filtered.forEach((f) => next.delete(f.path));
        return next;
      });
    } else {
      setSelected((prev) => {
        const next = new Set(prev);
        filtered.forEach((f) => next.add(f.path));
        return next;
      });
    }
  };

  const handleDelete = async () => {
    await api.post("/cleanup/local-files/delete", { paths: Array.from(selected) });
    setSelected(new Set());
    setConfirming(false);
    fetchFiles();
  };

  const allSelected = filtered.length > 0 && filtered.every((f) => selected.has(f.path));

  const tabs: { key: Filter; label: string }[] = [
    { key: "all", label: "All Files" },
    { key: "original", label: "Originals" },
    { key: "optimized", label: "Optimized" },
    { key: "safe", label: "Safe to Delete" },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold text-text-primary">Local Files</h1>

      {/* Storage summary */}
      <div className="grid grid-cols-3 gap-3">
        {[
          { label: "Total Local Storage", value: totalSize },
          { label: "Originals", value: originalsSize },
          { label: "Optimized", value: optimizedSize },
        ].map((card) => (
          <div key={card.label} className="bg-bg-card rounded-[10px] border border-border p-6">
            <p className="text-text-secondary text-sm">{card.label}</p>
            <p className="text-text-primary text-2xl font-semibold mt-1">{formatSize(card.value)}</p>
          </div>
        ))}
      </div>

      {/* Filter tabs + delete action */}
      <div className="flex items-center justify-between">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setFilter(t.key)}
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
        {selected.size > 0 && !confirming && (
          <button onClick={() => setConfirming(true)} className="bg-status-failed hover:bg-red-600 rounded-lg px-4 py-2 text-white font-medium text-sm">
            Delete Selected ({selected.size})
          </button>
        )}
        {confirming && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-status-failed">Delete {selected.size} files?</span>
            <button onClick={handleDelete} className="bg-status-failed hover:bg-red-600 rounded-lg px-4 py-2 text-white font-medium text-sm">
              Confirm
            </button>
            <button onClick={() => setConfirming(false)} className="bg-bg-secondary border border-border rounded-lg px-4 py-2 text-text-primary text-sm">
              Cancel
            </button>
          </div>
        )}
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
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Filename</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Type</th>
                <th className="p-3 text-right text-text-muted text-xs uppercase font-medium">Size</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Upload Status</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Run Date</th>
                <th className="p-3 text-left text-text-muted text-xs uppercase font-medium">Path</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((f) => (
                <tr
                  key={f.path}
                  onClick={() => toggleSelect(f.path)}
                  className={`border-b border-border cursor-pointer hover:bg-bg-hover ${selected.has(f.path) ? "bg-bg-hover" : ""}`}
                >
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selected.has(f.path)}
                      onChange={() => toggleSelect(f.path)}
                      onClick={(e) => e.stopPropagation()}
                      className="accent-accent"
                    />
                  </td>
                  <td className="p-3 text-text-primary truncate max-w-[200px]">{f.filename}</td>
                  <td className="p-3">
                    <span className={`text-xs px-2.5 py-1 rounded-full font-medium ${
                      f.type === "original"
                        ? "bg-amber-900/30 text-status-encoding"
                        : "bg-blue-900/30 text-status-progress"
                    }`}>
                      {f.type === "original" ? "Original" : "Optimized"}
                    </span>
                  </td>
                  <td className="p-3 text-right text-text-secondary">{formatSize(f.size_bytes)}</td>
                  <td className="p-3 text-text-secondary">{f.upload_status}</td>
                  <td className="p-3 text-text-secondary">{f.run_date ? new Date(f.run_date).toLocaleDateString() : "—"}</td>
                  <td className="p-3 text-text-muted text-xs truncate max-w-[200px]">{f.path}</td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={7} className="p-8 text-center text-text-muted">No files found</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
