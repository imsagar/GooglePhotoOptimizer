export function ProgressBar({ percent, stage }: { percent: number; stage: string }) {
  const color = stage === "encoding" ? "bg-status-encoding" : "bg-status-progress";
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 bg-border rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${percent}%` }} />
      </div>
      <span className="text-xs text-text-secondary font-medium">{percent}%</span>
    </div>
  );
}
