import { useEffect, useRef, useState, useCallback } from "react";

interface WSState {
  connected: boolean;
  runnerOnline: boolean;
  runnerPlatform: string;
  lastEvent: unknown;
}

export function useWebSocket() {
  const [state, setState] = useState<WSState>({
    connected: false, runnerOnline: false, runnerPlatform: "", lastEvent: null,
  });
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws/ui`);
    wsRef.current = ws;

    ws.onopen = () => setState((s) => ({ ...s, connected: true }));
    ws.onclose = () => {
      setState((s) => ({ ...s, connected: false }));
      setTimeout(connect, 3000);
    };
    ws.onmessage = (e) => {
      const data = JSON.parse(e.data);
      setState((s) => {
        const next = { ...s, lastEvent: data };
        if (data.type === "connected") {
          next.runnerOnline = true;
          next.runnerPlatform = data.platform;
        }
        if (data.type === "runner_offline") {
          next.runnerOnline = false;
        }
        return next;
      });
    };
  }, []);

  useEffect(() => { connect(); return () => wsRef.current?.close(); }, [connect]);

  return state;
}
