const colors: Record<string, string> = {
  queued: "bg-bg-card text-text-secondary",
  downloading: "bg-blue-900/30 text-status-progress",
  encoding: "bg-amber-900/30 text-status-encoding",
  ready: "bg-blue-900/30 text-blue-400",
  uploading: "bg-blue-900/30 text-status-progress",
  uploaded: "bg-green-900/30 text-status-uploaded",
  failed: "bg-red-900/30 text-status-failed",
  cancelled: "bg-bg-card text-text-muted",
};

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`text-xs px-2.5 py-1 rounded-full font-medium ${colors[status] ?? colors.queued}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}
