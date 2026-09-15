import { NavLink } from "react-router-dom";
import { RunnerBadge } from "./RunnerBadge";

const links = [
  { to: "/", label: "Dashboard" },
  { to: "/videos", label: "Videos" },
  { to: "/library", label: "Library" },
  { to: "/local-files", label: "Local Files" },
  { to: "/settings", label: "Settings" },
];

export function Sidebar({ runnerOnline, runnerPlatform }: { runnerOnline: boolean; runnerPlatform: string }) {
  const handleLogout = async () => {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" });
    window.location.href = "/";
  };

  return (
    <aside className="w-60 shrink-0 bg-bg-secondary border-r border-border flex flex-col h-screen sticky top-0">
      <div className="px-4 py-5">
        <span className="text-xl font-bold text-accent">GPOptimizer</span>
      </div>
      <nav className="flex-1 flex flex-col gap-0.5 px-2">
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            end={l.to === "/"}
            className={({ isActive }) =>
              `px-3 py-2 rounded-lg text-sm transition-colors ${
                isActive
                  ? "bg-bg-card text-accent"
                  : "text-text-secondary hover:text-text-primary hover:bg-bg-hover"
              }`
            }
          >
            {l.label}
          </NavLink>
        ))}
      </nav>
      <div className="px-2 mb-2">
        <button
          onClick={handleLogout}
          className="w-full px-3 py-2 rounded-lg text-sm text-text-secondary hover:text-text-primary hover:bg-bg-hover transition-colors text-left"
        >
          Logout
        </button>
      </div>
      <RunnerBadge online={runnerOnline} platform={runnerPlatform} />
    </aside>
  );
}
