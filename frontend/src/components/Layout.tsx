import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { ActivityBar } from "./ActivityBar";
import { useWebSocket } from "../hooks/useWebSocket";

export function Layout() {
  const { runnerOnline, runnerPlatform, lastEvent } = useWebSocket();

  return (
    <div className="flex min-h-screen pb-10">
      <Sidebar runnerOnline={runnerOnline} runnerPlatform={runnerPlatform} />
      <main className="flex-1 p-6 overflow-y-auto">
        <Outlet />
      </main>
      <ActivityBar lastEvent={lastEvent} />
    </div>
  );
}
