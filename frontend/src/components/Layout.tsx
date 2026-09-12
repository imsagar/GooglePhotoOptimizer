import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { useWebSocket } from "../hooks/useWebSocket";

export function Layout() {
  const { runnerOnline, runnerPlatform } = useWebSocket();

  return (
    <div className="flex min-h-screen">
      <Sidebar runnerOnline={runnerOnline} runnerPlatform={runnerPlatform} />
      <main className="flex-1 p-6 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}
