import { useEffect, useState } from "react";
import { api } from "../api/client";

interface OptSettings {
  codec: string;
  crf: number;
  preset: string;
}

const CODECS = ["libx265", "libx264", "libsvtav1"];
const PRESETS = ["ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"];

export function Settings() {
  const [pairingCode, setPairingCode] = useState("");
  const [settings, setSettings] = useState<OptSettings>({ codec: "libx265", crf: 28, preset: "medium" });
  const [saved, setSaved] = useState(false);
  const [googleConnected, setGoogleConnected] = useState(false);

  useEffect(() => {
    api.get<OptSettings>("/settings").then(setSettings).catch(() => {});
    api.get<{ connected: boolean }>("/auth/google/photos/status").then((r) => setGoogleConnected(r.connected)).catch(() => {});
  }, []);

  const handleDisconnect = async () => {
    await api.post("/auth/google/photos/disconnect");
    setGoogleConnected(false);
  };

  const handlePair = async () => {
    const res = await api.post<{ code: string }>("/runner/pair");
    setPairingCode(res.code);
  };

  const handleRotate = async () => {
    await api.post("/runner/rotate");
  };

  const handleSave = async () => {
    await api.put("/settings", settings);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  const inputClass = "bg-bg-secondary border border-border rounded-lg px-3 py-2 text-text-primary w-full focus:outline-none focus:border-accent";
  const btnClass = "bg-accent hover:bg-accent-hover text-white font-medium px-4 py-2 rounded-lg transition-colors";
  const cardClass = "bg-bg-card rounded-[10px] border border-border p-6";

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-text-primary">Settings</h1>

      {/* Runner Setup */}
      <div className={cardClass}>
        <h2 className="text-lg font-semibold text-text-primary mb-4">Runner Setup</h2>
        <p className="text-text-secondary text-sm mb-4">
          Generate a pairing code and run the command on your machine to connect the runner.
        </p>
        <button onClick={handlePair} className={btnClass}>Generate Pairing Code</button>
        {pairingCode && (
          <div className="mt-4 space-y-2">
            <p className="text-text-muted text-sm">Run this command on your machine:</p>
            <code className="block bg-bg-secondary rounded-lg px-4 py-3 text-accent font-mono text-lg">
              gpoptimizer-runner --pair {pairingCode}
            </code>
          </div>
        )}
      </div>

      {/* Runner Token */}
      <div className={cardClass}>
        <h2 className="text-lg font-semibold text-text-primary mb-4">Runner Token</h2>
        <p className="text-text-secondary text-sm mb-4">
          Rotate the runner authentication token if you suspect it has been compromised.
        </p>
        <button onClick={handleRotate} className={btnClass}>Rotate Token</button>
      </div>

      {/* Google Photos Connection */}
      <div className={cardClass}>
        <h2 className="text-lg font-semibold text-text-primary mb-4">Google Photos Connection</h2>
        {googleConnected ? (
          <>
            <div className="flex items-center gap-2 mb-3">
              <span className="w-2.5 h-2.5 rounded-full bg-green-500" />
              <span className="text-green-400 text-sm font-medium">Connected</span>
            </div>
            <button onClick={handleDisconnect} className="bg-red-600 hover:bg-red-700 text-white font-medium px-4 py-2 rounded-lg transition-colors">
              Disconnect Google Photos
            </button>
          </>
        ) : (
          <>
            <p className="text-text-secondary text-sm mb-3">
              Connect your Google Photos account to sync and optimize videos.
            </p>
            <a href="/api/auth/google/photos" className={btnClass + " inline-block text-center no-underline"}>
              Connect Google Photos
            </a>
          </>
        )}
      </div>

      {/* Optimization Defaults */}
      <div className={cardClass}>
        <h2 className="text-lg font-semibold text-text-primary mb-4">Optimization Defaults</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
          <div>
            <label className="block text-text-secondary text-sm mb-1">Codec</label>
            <select
              value={settings.codec}
              onChange={(e) => setSettings({ ...settings, codec: e.target.value })}
              className={inputClass + " appearance-none"}
            >
              {CODECS.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-text-secondary text-sm mb-1">CRF (0–51)</label>
            <input
              type="number"
              min={0}
              max={51}
              value={settings.crf}
              onChange={(e) => setSettings({ ...settings, crf: Number(e.target.value) })}
              className={inputClass}
            />
          </div>
          <div>
            <label className="block text-text-secondary text-sm mb-1">Preset</label>
            <select
              value={settings.preset}
              onChange={(e) => setSettings({ ...settings, preset: e.target.value })}
              className={inputClass + " appearance-none"}
            >
              {PRESETS.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
        </div>
        <button onClick={handleSave} className={btnClass}>
          {saved ? "Saved!" : "Save Defaults"}
        </button>
      </div>
    </div>
  );
}
