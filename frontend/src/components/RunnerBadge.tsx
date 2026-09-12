export function RunnerBadge({ online, platform }: { online: boolean; platform: string }) {
  return (
    <div className="flex items-center gap-2 px-4 py-3">
      <div className={`w-2 h-2 rounded-full ${online ? "bg-accent" : "bg-status-failed"}`} />
      <div>
        <div className="text-sm text-text-primary">{online ? "Runner Online" : "Runner Offline"}</div>
        {platform && <div className="text-xs text-text-muted">{platform}</div>}
      </div>
    </div>
  );
}
