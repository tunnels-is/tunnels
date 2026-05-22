import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Save, X, Pencil } from "lucide-react";

/* ─── Reusable building blocks ─────────────────────────────────────── */

const SettingsCard = ({ title, description, actions, children, className = "" }) => (
  <section
    className={
      "rounded-lg bg-white border border-[#e7e3d7] card-shadow p-5 " + className
    }
  >
    <header className="flex items-start justify-between gap-3 mb-4">
      <div>
        <h3 className="text-[13px] font-semibold tracking-tight text-[#0a0a0a]">{title}</h3>
        {description && (
          <p className="mt-1 text-[11px] text-[#737373] leading-relaxed">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </header>
    {children}
  </section>
);

const InfoRow = ({ label, value, mono = true, last = false }) => (
  <div
    className={
      "flex items-baseline gap-3 py-2 " +
      (last ? "" : "border-b border-[#e7e3d7]/70")
    }
  >
    <span className="w-[110px] shrink-0 text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3]">
      {label}
    </span>
    <span
      className={
        "min-w-0 flex-1 text-[12px] truncate " +
        (mono ? "font-mono text-[#0a0a0a]" : "text-[#0a0a0a]")
      }
      title={typeof value === "string" ? value : undefined}
    >
      {value || <span className="italic text-[#a3a3a3]">unknown</span>}
    </span>
  </div>
);

const TogglePill = ({ checked, label, onClick, tone = "active" }) => {
  const activeClass = {
    active: "pill pill-active",
    warning: "pill pill-active-warning",
    danger: "pill pill-active-danger",
  }[tone] || "pill pill-active";
  return (
    <button
      onClick={onClick}
      className={checked ? activeClass : "pill"}
    >
      {label}
    </button>
  );
};

/* ─── Settings page ────────────────────────────────────────────────── */

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

  const saveApi = async () => {
    state.Config = cfg;
    const ok = await state.v2_ConfigSave();
    if (ok) { setMod(false); setEditing(false); }
  };

  const cancelApi = () => {
    setCfg({ ...state.Config });
    setMod(false);
    setEditing(false);
  };

  let basePath = state.State?.BasePath;
  let logPath = "";
  let logFileName = state.State?.LogFileName?.replace(state.State?.LogPath, "");
  let configPath = state.State?.ConfigFileName;
  if (state.State?.LogPath !== basePath) {
    logPath = state.State?.LogPath;
  }
  let version = state.Version ? state.Version : "unknown";
  let apiversion = state.APIVersion ? state.APIVersion : "unknown";

  const toggle = (key) => {
    state.toggleConfigKeyAndSave("Config", key);
    state.renderPage("settings");
  };

  const loggingOptions = [
    { key: "InfoLogging",     label: "Info",            tone: "active" },
    { key: "ErrorLogging",    label: "Errors",          tone: "active" },
    { key: "ConsoleLogging",  label: "Console",         tone: "active" },
    { key: "DebugLogging",    label: "Debug",           tone: "active" },
    { key: "BandwidthGraphs", label: "Bandwidth Graphs", tone: "active" },
    { key: "ConsoleLogOnly",  label: "Console Only",    tone: "active" },
    { key: "DeepDebugLoggin", label: "Deep Debug",      tone: "warning" },
  ];

  const updateOptions = [
    { key: "DisableUpdates",       label: "Disable Updates", tone: "warning" },
    { key: "AutoDownloadUpdate",   label: "Auto Download",   tone: "active" },
    { key: "UpdateWhileConnected", label: "While Connected", tone: "active" },
    { key: "RestartPostUpdate",    label: "Restart After",   tone: "active" },
    { key: "ExitPostUpdate",       label: "Exit After",      tone: "warning" },
  ];

  return (
    <div className="max-w-5xl">

      {/* Page header */}
      <header className="mb-6 flex items-baseline justify-between gap-4">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight text-[#0a0a0a]">Settings</h1>
          <p className="mt-1 text-[12px] text-[#737373]">
            Configure your API, logging, updates and network preferences.
          </p>
        </div>
        <div className="flex items-center gap-3 text-[11px] text-[#a3a3a3]">
          <div className="flex items-baseline gap-1.5">
            <span className="uppercase tracking-[0.1em] text-[9px] font-semibold">App</span>
            <code className="font-mono text-[#525252]">{version}</code>
          </div>
          <span className="w-px h-3 bg-[#e7e3d7]" />
          <div className="flex items-baseline gap-1.5">
            <span className="uppercase tracking-[0.1em] text-[9px] font-semibold">API</span>
            <code className="font-mono text-[#525252]">{apiversion}</code>
          </div>
        </div>
      </header>

      {/* Grid layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">

        {/* ── API ── */}
        <SettingsCard
          className="lg:col-span-2"
          tone="info"
          title="API server"
          description="Address the desktop client listens on, plus optional TLS certificate."
          actions={
            editing ? (
              <>
                {mod && (
                  <Button onClick={saveApi} className="btn btn-primary btn-xs">
                    <Save className="h-3 w-3" /> Save
                  </Button>
                )}
                <button onClick={cancelApi} className="btn btn-ghost btn-xs">
                  <X className="h-3 w-3" /> Cancel
                </button>
              </>
            ) : (
              <button onClick={() => setEditing(true)} className="btn btn-outline btn-xs">
                <Pencil className="h-3 w-3" /> Edit
              </button>
            )
          }
        >
          {!editing ? (
            <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  Address
                </span>
                <code className="text-[13px] text-[#0a0a0a] font-mono">
                  {cfg.APIIP || "0.0.0.0"}<span className="text-[#a3a3a3]">:</span>{cfg.APIPort || "—"}
                </code>
              </div>
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  TLS Cert
                </span>
                <code
                  className="block max-w-[260px] truncate text-[13px] text-[#0a0a0a] font-mono"
                  title={cfg.APICert || ""}
                >
                  {cfg.APICert || <span className="font-sans italic text-[#a3a3a3]">none</span>}
                </code>
              </div>
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  TLS Key
                </span>
                <code
                  className="block max-w-[260px] truncate text-[13px] text-[#0a0a0a] font-mono"
                  title={cfg.APIKey || ""}
                >
                  {cfg.APIKey || <span className="font-sans italic text-[#a3a3a3]">none</span>}
                </code>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
                <div>
                  <label className="label">IP</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APIIP || ""} onChange={(e) => updatecfg("APIIP", e.target.value)} />
                </div>
                <div>
                  <label className="label">Port</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APIPort || ""} onChange={(e) => updatecfg("APIPort", e.target.value)} />
                </div>
                <div>
                  <label className="label">Cert Domains</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APICertDomains || ""} onChange={(e) => updatecfg("APICertDomains", e.target.value)} />
                </div>
                <div>
                  <label className="label">Cert IPs</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APICertIPs || ""} onChange={(e) => updatecfg("APICertIPs", e.target.value)} />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label className="label">Cert Path</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APICert || ""} onChange={(e) => updatecfg("APICert", e.target.value)} />
                </div>
                <div>
                  <label className="label">Key Path</label>
                  <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.APIKey || ""} onChange={(e) => updatecfg("APIKey", e.target.value)} />
                </div>
              </div>
            </div>
          )}
        </SettingsCard>

        {/* ── Logging ── */}
        <SettingsCard
          tone="warning"
          title="Logging"
          description="Select which event types are captured to disk and console."
        >
          <div className="flex flex-wrap items-center gap-2">
            {loggingOptions.map((opt) => (
              <TogglePill
                key={opt.key}
                checked={!!state?.Config?.[opt.key]}
                tone={opt.tone}
                label={opt.label}
                onClick={() => toggle(opt.key)}
              />
            ))}
            <TogglePill
              checked={!!state?.debug}
              tone="warning"
              label="Debug Mode"
              onClick={() => { state.toggleDebug(); state.renderPage("settings"); }}
            />
          </div>
        </SettingsCard>

        {/* ── Updates ── */}
        <SettingsCard
          tone="success"
          title="Updates"
          description="Behaviour when a new build of Tunnels is available."
        >
          <div className="flex flex-wrap items-center gap-2">
            {updateOptions.map((opt) => (
              <TogglePill
                key={opt.key}
                checked={!!state?.Config?.[opt.key]}
                tone={opt.tone}
                label={opt.label}
                onClick={() => toggle(opt.key)}
              />
            ))}
          </div>
        </SettingsCard>

        {/* ── DNS ── */}
        <SettingsCard
          tone="danger"
          title="DNS"
          description="The local DNS resolver is enabled by default."
        >
          <div className="flex flex-wrap items-center gap-2">
            <TogglePill
              checked={!!state?.Config?.DisableDNS}
              tone="warning"
              label="Disable DNS"
              onClick={() => toggle("DisableDNS")}
            />
          </div>
        </SettingsCard>

        {/* ── Network ── */}
        <SettingsCard
          tone="info"
          title="Network"
          description="Detected default network interface (read-only)."
        >
          <div>
            {[
              { label: "Interface",    value: state.Network?.DefaultInterfaceName },
              { label: "IP Address",   value: state.Network?.DefaultInterface },
              { label: "Interface ID", value: state.Network?.DefaultInterfaceID },
              { label: "Gateway",      value: state.Network?.DefaultGateway },
            ].map((row, i, arr) => (
              <InfoRow key={i} label={row.label} value={row.value} last={i === arr.length - 1} />
            ))}
          </div>
        </SettingsCard>

        {/* ── System ── */}
        <SettingsCard
          tone="neutral"
          title="System"
          description="Paths, files and privileges this app is running with."
          className="lg:col-span-2"
        >
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8">
            <div>
              {[
                { label: "Base Path", value: basePath },
                { label: "Config",    value: configPath },
                { label: "Log Path",  value: logPath || "Default" },
              ].map((row, i, arr) => (
                <InfoRow key={i} label={row.label} value={row.value} last={i === arr.length - 1} />
              ))}
            </div>
            <div>
              {[
                { label: "Log File",    value: logFileName },
                { label: "Admin",       value: state.State?.IsAdmin ? "Yes" : "No", mono: false },
                { label: "API Version", value: apiversion },
              ].map((row, i, arr) => (
                <InfoRow
                  key={i}
                  label={row.label}
                  value={row.value}
                  mono={row.mono !== false}
                  last={i === arr.length - 1}
                />
              ))}
            </div>
          </div>
        </SettingsCard>

      </div>
    </div>
  );
};

export default Settings;
