import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { Monitor, Plus, X } from "lucide-react";
import QRCode from "react-qr-code";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const UserDevices = () => {
  const state = GLOBAL_STATE("user-devices");
  const [devices, setDevices] = useState([]);
  const [showCreate, setShowCreate] = useState(false);
  const [tag, setTag] = useState("");
  const [selectedServerID, setSelectedServerID] = useState("");
  const [servers, setServers] = useState([]);
  const [wgConfig, setWgConfig] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  const loadDevices = async () => {
    const resp = await state.callController(null, "POST", "/client/device/list/user", {}, false, false);
    if (resp?.status === 200 && Array.isArray(resp.data)) {
      setDevices(resp.data);
    }
  };

  const loadServers = async () => {
    const resp = await state.callController(null, "POST", "/client/servers", { StartIndex: 0 }, false, false);
    if (resp?.status === 200 && Array.isArray(resp.data)) {
      setServers(resp.data);
      if (resp.data.length > 0) {
        setSelectedServerID(resp.data[0]._id);
      }
    }
  };

  useEffect(() => {
    loadDevices();
  }, []);

  const handleOpenCreate = async () => {
    setTag("");
    setWgConfig(null);
    setShowCreate(true);
    await loadServers();
  };

  const handleSubmit = async () => {
    if (!tag.trim()) {
      state.toggleError("Please enter a device tag");
      return;
    }
    if (!selectedServerID) {
      state.toggleError("Please select a server");
      return;
    }

    setSubmitting(true);
    try {
      const resp = await state.API.method("createDeviceWithKeys", {
        Server: state.User?.ControlServer,
        Tag: tag.trim(),
        ServerID: selectedServerID,
        DeviceToken: state.User?.DeviceToken?.DT || "",
        UID: state.User?._id || "",
      });

      if (resp?.status === 200 && resp.data?.WGConfig) {
        setWgConfig(resp.data.WGConfig);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleDownload = () => {
    const blob = new Blob([wgConfig], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${tag.trim() || "device"}.conf`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleDone = () => {
    setShowCreate(false);
    setWgConfig(null);
    setTag("");
    setSelectedServerID("");
    loadDevices();
  };

  // Build a set of WireGuard IPs from local tunnel configs to identify this machine's device.
  const localIPs = new Set(
    (state.Tunnels || []).map((t) => t.IPv4Address).filter(Boolean)
  );

  return (
    <div>
      <div className="flex justify-end mb-3">
        {!showCreate && (
          <Button
            className="h-7 text-[11px] text-white bg-[#4B7BF5] hover:bg-[#5d8af7] flex items-center gap-1.5"
            onClick={handleOpenCreate}
          >
            <Plus className="w-3.5 h-3.5" />
            New Device
          </Button>
        )}
      </div>

      {showCreate && (
        <div className="mb-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] p-4">
          {!wgConfig ? (
            <>
              <div className="flex items-center justify-between mb-3">
                <span className="text-[11px] text-white/50 uppercase tracking-wider">New Device</span>
                <button onClick={() => setShowCreate(false)} className="text-white/30 hover:text-white/60">
                  <X className="w-4 h-4" />
                </button>
              </div>
              <div className="space-y-3">
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">Tag</label>
                  <Input
                    className="h-7 text-[12px] border-[#1e2433] bg-transparent"
                    type="text"
                    placeholder="e.g. my-laptop"
                    value={tag}
                    onChange={(e) => setTag(e.target.value)}
                  />
                </div>
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">Server</label>
                  <select
                    className="w-full h-7 text-[12px] bg-[#0a0d14] border border-[#1e2433] rounded-md px-2 text-white/80"
                    value={selectedServerID}
                    onChange={(e) => setSelectedServerID(e.target.value)}
                  >
                    {servers.length === 0 && <option value="">No servers available</option>}
                    {servers.map((s) => (
                      <option key={s._id} value={s._id}>{s.Tag} ({s.Country})</option>
                    ))}
                  </select>
                </div>
                <Button
                  className="w-full h-7 text-[11px] text-white bg-emerald-600 hover:bg-emerald-500"
                  onClick={handleSubmit}
                  disabled={submitting}
                >
                  {submitting ? "Creating..." : "Create Device"}
                </Button>
              </div>
            </>
          ) : (
            <>
              <div className="py-2 px-3 rounded bg-amber-500/5 border border-amber-500/20 mb-3">
                <p className="text-[11px] text-amber-400/80">Save this config — it cannot be shown again</p>
              </div>
              <div className="flex justify-center mb-3">
                <div className="p-4 bg-white rounded w-[220px]">
                  <QRCode
                    style={{ height: "auto", maxWidth: "188px", width: "188px" }}
                    value={wgConfig}
                    viewBox="0 0 256 256"
                  />
                </div>
              </div>
              <div className="flex gap-2">
                <Button
                  className="flex-1 h-7 text-[11px] text-white bg-[#4B7BF5] hover:bg-[#5d8af7]"
                  onClick={handleDownload}
                >
                  Download .conf
                </Button>
                <Button
                  className="flex-1 h-7 text-[11px] text-white bg-emerald-600 hover:bg-emerald-500"
                  onClick={handleDone}
                >
                  Done
                </Button>
              </div>
            </>
          )}
        </div>
      )}

      <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
        <span className="text-[10px] text-white/40 uppercase tracking-wider w-40 shrink-0">Tag</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider flex-1 min-w-0">WireGuard IP</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-40 text-right">Created</span>
      </div>

      <div className="space-y-px">
        {devices.length > 0 ? devices.map((d) => {
          const isCurrent = d.WireGuardIP && localIPs.has(d.WireGuardIP);
          return (
            <div
              key={d._id}
              className={`flex items-center gap-4 py-1.5 pl-3 border-l-2 transition-colors ${
                isCurrent
                  ? "border-emerald-500/50 hover:border-emerald-500/80"
                  : "border-[#4B7BF5]/20 hover:border-[#4B7BF5]/50"
              }`}
            >
              <div className="flex items-center gap-2 w-40 shrink-0">
                <Monitor className={`w-3.5 h-3.5 shrink-0 ${isCurrent ? "text-emerald-500/70" : "text-[#4B7BF5]/60"}`} />
                <span className="text-[13px] text-white/80 font-medium truncate">{d.Tag}</span>
                {isCurrent && (
                  <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded shrink-0">this device</span>
                )}
              </div>
              <span className="text-[12px] text-white/50 font-mono flex-1 min-w-0 truncate">
                {d.WireGuardIP || "—"}
              </span>
              <span className="text-[11px] text-white/40 tabular-nums shrink-0 w-40 text-right">
                {d.CreatedAt ? dayjs(d.CreatedAt).format("HH:mm:ss DD-MM-YYYY") : "—"}
              </span>
            </div>
          );
        }) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">
            No devices found
          </div>
        )}
      </div>
    </div>
  );
};

export default UserDevices;
