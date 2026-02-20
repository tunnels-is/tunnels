import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Save, Settings as SettingsIcon } from "lucide-react";

const Settings = () => {
  const state = GLOBAL_STATE("settings");
  const [editing, setEditing] = useState(false);
  const [cfg, setCfg] = useState({ ...state.Config });
  const [mod, setMod] = useState(false);

  const updatecfg = (key, value) => {
    if (key === "APICertDomains" || key === "APICertIPs") {
      value = value.split(",");
    }
    setCfg((prev) => ({ ...prev, [key]: value }));
    setMod(true);
  };

  useEffect(() => {
    state.GetBackendState();
  }, []);

  let basePath = state.State?.BasePath;
  let logPath = "";
  let logFileName = state.State?.LogFileName?.replace(state.State?.LogPath, "");
  let configPath = state.State?.ConfigFileName;
  if (state.State?.LogPath !== basePath) {
    logPath = state.State?.LogPath;
  }
  let version = state.Version ? state.Version : "unknown";
  let apiversion = state.APIVersion ? state.APIVersion : "unknown";

  const loggingOptions = [
    { key: "InfoLogging", label: "Info", checked: state?.Config?.InfoLogging },
    { key: "ErrorLogging", label: "Errors", checked: state?.Config?.ErrorLogging },
    { key: "ConsoleLogging", label: "Console", checked: state?.Config?.ConsoleLogging },
    { key: "DebugLogging", label: "Debug", checked: state?.Config?.DebugLogging },
  ];

  return (
    <div>

      {/* ── API config banner ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-6">
        {!editing ? (
          <>
            <div>
              <span className="text-[9px] text-white/35 uppercase tracking-widest block mb-0.5">API</span>
              <code className="text-[13px] text-white/80 font-mono">
                {cfg.APIIP || "0.0.0.0"}:{cfg.APIPort || "—"}
              </code>
            </div>
            {(cfg.APICert || cfg.APIKey) && (
              <>
                <div className="w-px h-8 bg-white/[0.06]" />
                <div>
                  <span className="text-[9px] text-white/35 uppercase tracking-widest block mb-0.5">TLS Cert</span>
                  <code className="text-[13px] text-white/80 font-mono truncate block max-w-[200px]">{cfg.APICert || "none"}</code>
                </div>
              </>
            )}
            <button
              className="ml-auto p-1.5 rounded text-white/40 hover:text-white/60 hover:bg-white/[0.04] transition-colors"
              onClick={() => setEditing(true)}
            >
              <SettingsIcon className="h-3.5 w-3.5" />
            </button>
          </>
        ) : (
          <div className="flex-1">
            <div className="grid grid-cols-4 gap-3">
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">IP</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APIIP || ""} onChange={(e) => updatecfg("APIIP", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Port</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APIPort || ""} onChange={(e) => updatecfg("APIPort", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Cert Domains</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APICertDomains || ""} onChange={(e) => updatecfg("APICertDomains", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Cert IPs</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APICertIPs || ""} onChange={(e) => updatecfg("APICertIPs", e.target.value)} />
              </div>
            </div>
            <div className="grid grid-cols-4 gap-3 mt-2">
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Cert Path</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APICert || ""} onChange={(e) => updatecfg("APICert", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Key Path</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.APIKey || ""} onChange={(e) => updatecfg("APIKey", e.target.value)} />
              </div>
            </div>
            <div className="flex gap-2 mt-2">
              {mod && (
                <Button
                  className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5"
                  onClick={async () => {
                    state.Config = cfg;
                    let ok = await state.v2_ConfigSave();
                    if (ok) { setMod(false); setEditing(false); }
                  }}
                >
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
              )}
              <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => { setCfg({ ...state.Config }); setMod(false); setEditing(false); }}>
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>

      {/* ── Logging ── */}
      <div className="mb-8">
        <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider block mb-3">Logging</span>
        <div className="flex items-center gap-2 flex-wrap">
          {loggingOptions.map((opt) => (
            <button
              key={opt.key}
              className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                opt.checked
                  ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                  : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
              }`}
              onClick={() => { state.toggleConfigKeyAndSave("Config", opt.key); state.renderPage("settings"); }}
            >
              {opt.label}
            </button>
          ))}
          <button
            className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
              state?.Config?.ConsoleLogOnly
                ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
            }`}
            onClick={() => { state.toggleConfigKeyAndSave("Config", "ConsoleLogOnly"); state.renderPage("settings"); }}
          >
            Console Only
          </button>
          <button
            className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
              state?.Config?.DeepDebugLoggin
                ? "border-amber-500/40 bg-amber-500/15 text-amber-400 shadow-[0_0_12px_rgba(245,158,11,0.12)]"
                : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
            }`}
            onClick={() => { state.toggleConfigKeyAndSave("Config", "DeepDebugLoggin"); state.renderPage("settings"); }}
          >
            Deep Debug
          </button>
          <button
            className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
              state?.debug
                ? "border-amber-500/40 bg-amber-500/15 text-amber-400 shadow-[0_0_12px_rgba(245,158,11,0.12)]"
                : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
            }`}
            onClick={() => { state.toggleDebug(); state.renderPage("settings"); }}
          >
            Debug Mode
          </button>
        </div>
      </div>

      {/* ── Updates ── */}
      <div className="mb-8">
        <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider block mb-3">Updates</span>
        <div className="flex items-center gap-2 flex-wrap mb-3">
          {[
            { key: "DisableUpdates", label: "Disable Updates", checked: state?.Config?.DisableUpdates, amber: true },
            { key: "AutoDownloadUpdate", label: "Auto Download", checked: state?.Config?.AutoDownloadUpdate },
            { key: "UpdateWhileConnected", label: "While Connected", checked: state?.Config?.UpdateWhileConnected },
            { key: "RestartPostUpdate", label: "Restart After", checked: state?.Config?.RestartPostUpdate },
            { key: "ExitPostUpdate", label: "Exit After", checked: state?.Config?.ExitPostUpdate, amber: true },
          ].map((opt) => (
            <button
              key={opt.key}
              className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                opt.checked
                  ? opt.amber
                    ? "border-amber-500/40 bg-amber-500/15 text-amber-400 shadow-[0_0_12px_rgba(245,158,11,0.12)]"
                    : "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                  : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
              }`}
              onClick={() => { state.toggleConfigKeyAndSave("Config", opt.key); state.renderPage("settings"); }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {/* ── DNS ── */}
      <div className="mb-8">
        <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider block mb-3">DNS</span>
        <div className="flex items-center gap-2 flex-wrap">
          <button
            className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
              state?.Config?.DisableDNS
                ? "border-amber-500/40 bg-amber-500/15 text-amber-400 shadow-[0_0_12px_rgba(245,158,11,0.12)]"
                : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
            }`}
            onClick={() => { state.toggleConfigKeyAndSave("Config", "DisableDNS"); state.renderPage("settings"); }}
          >
            Disable DNS
          </button>
        </div>
      </div>

      {/* ── Network + System ── */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">

        {/* Network */}
        <div>
          <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider block mb-3">Network</span>
          <div className="space-y-1">
            {[
              { label: "Interface", value: state.Network?.DefaultInterfaceName },
              { label: "IP Address", value: state.Network?.DefaultInterface },
              { label: "Interface ID", value: state.Network?.DefaultInterfaceID },
              { label: "Gateway", value: state.Network?.DefaultGateway },
            ].map((row, i) => (
              <div key={i} className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-blue-500/20">
                <span className="text-[11px] text-white/45 shrink-0 w-[90px]">{row.label}</span>
                <code className="text-[13px] text-white/60 font-mono truncate">{row.value ?? "unknown"}</code>
              </div>
            ))}
          </div>
        </div>

        {/* System */}
        <div>
          <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider block mb-3">System</span>
          <div className="space-y-1">
            {[
              { label: "API Version", value: apiversion },
              { label: "App Version", value: version },
              { label: "Base Path", value: basePath },
              { label: "Config", value: configPath },
              { label: "Log Path", value: logPath || "Default" },
              { label: "Log File", value: logFileName },
              { label: "Admin", value: state.State?.IsAdmin ? "Yes" : "No" },
            ].map((row, i) => (
              <div key={i} className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-violet-500/20">
                <span className="text-[11px] text-white/45 shrink-0 w-[90px]">{row.label}</span>
                <code className="text-[13px] text-white/60 font-mono truncate">{row.value ?? "unknown"}</code>
              </div>
            ))}
          </div>
        </div>

      </div>
    </div>
  );
};

export default Settings;
