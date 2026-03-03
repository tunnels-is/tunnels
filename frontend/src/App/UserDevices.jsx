import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { Monitor } from "lucide-react";

const UserDevices = () => {
  const state = GLOBAL_STATE("user-devices");
  const [devices, setDevices] = useState([]);

  const loadDevices = async () => {
    const resp = await state.callController(null, "POST", "/client/device/list/user", {}, false, false);
    if (resp?.status === 200 && Array.isArray(resp.data)) {
      setDevices(resp.data);
    }
  };

  useEffect(() => {
    loadDevices();
  }, []);

  // Build a set of WireGuard IPs from local tunnel configs to identify this machine's device.
  const localIPs = new Set(
    (state.Tunnels || []).map((t) => t.IPv4Address).filter(Boolean)
  );

  return (
    <div>
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
