import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { ActivityBar } from "./ActivityBar";
import { useWebSocket } from "../hooks/useWebSocket";

export function Layout() {
  const ws = useWebSocket();

  return (
    <div className="flex min-h-screen pb-10">
      <Sidebar runnerOnline={ws.runnerOnline} runnerPlatform={ws.runnerPlatform} />
      <main className="flex-1 p-6 overflow-y-auto">
        <Outlet context={ws} />
      </main>
      <ActivityBar lastEvent={ws.lastEvent} />
    </div>
  );
}
