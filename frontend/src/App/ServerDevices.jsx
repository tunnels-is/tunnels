import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { useParams } from "react-router-dom";
import { Search, ChevronLeft, ChevronRight } from "lucide-react";

const ServerDevices = () => {
  const state = GLOBAL_STATE("devices");
  const [connectedDevices, setConnectedDevices] = useState([]);
  const { id } = useParams();
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;

  const getConnectedDevices = async () => {
    let server = undefined;
    state.PrivateServers.forEach((s, i) => {
      if (s._id === id) server = state.PrivateServers[i];
    });
    if (!server) return;
    let s = { ...state.User?.ControlServer };
    s.Host = server.IP;
    let resp = await state.callController(s, "POST", "/v3/devices", {}, false, false);
    if (resp.status === 200) {
      setConnectedDevices(resp.data?.Devices || []);
      state.renderPage("devices");
    }
  };

  useEffect(() => {
    getConnectedDevices();
  }, []);

  const filtered = filter
    ? connectedDevices.filter((d) =>
      d.DHCP?.Hostname?.toLowerCase().includes(filter.toLowerCase()) ||
      d.DHCP?.Token?.includes(filter) ||
      d.DHCP?.IP?.join(".").includes(filter))
    : connectedDevices;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  return (
    <div>
      {/* ── Header bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <span className="text-[10px] text-white/45 uppercase tracking-wider">Connected Devices</span>
        {filtered.length > 0 && (
          <>
            <div className="w-px h-4 bg-white/[0.06]" />
            <span className="text-[10px] text-white/40 tabular-nums">{filtered.length} total</span>
          </>
        )}
        <div className="flex items-center gap-1.5 ml-auto">
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/40" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/30 outline-none focus:border-white/25 transition-colors"
              placeholder="Filter devices..."
              value={filter}
              onChange={(e) => { setFilter(e.target.value); setPage(0); }}
            />
          </div>
          {filtered.length > PAGE_SIZE && (
            <div className="flex items-center gap-1">
              <button
                className="p-0.5 text-white/40 hover:text-white/60 disabled:opacity-30 disabled:cursor-default transition-colors"
                disabled={safePage === 0}
                onClick={() => setPage(safePage - 1)}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <span className="text-[10px] text-white/40 tabular-nums">{safePage + 1}/{totalPages}</span>
              <button
                className="p-0.5 text-white/40 hover:text-white/60 disabled:opacity-30 disabled:cursor-default transition-colors"
                disabled={safePage >= totalPages - 1}
                onClick={() => setPage(safePage + 1)}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* ── Column headers ── */}
      <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
        <span className="text-[10px] text-white/40 uppercase tracking-wider flex-1 min-w-0">Device</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-28 text-right hidden md:block">IP</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-20 text-right hidden md:block">Ports</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-12 text-right">CPU</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-12 text-right">RAM</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-32 text-right">Connected</span>
      </div>

      {/* ── Rows ── */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((d, i) => (
          <div key={i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-cyan-500/20 hover:border-cyan-500/50 transition-colors">
            <div className="flex-1 min-w-0">
              <span className="text-[13px] text-white/80 font-medium truncate block">{d.DHCP?.Hostname || d.DHCP?.Token || "Unknown"}</span>
              <span className="text-[11px] text-white/40 font-mono truncate block">{d.DHCP?.Token}</span>
            </div>
            <span className="text-[11px] text-white/40 font-mono tabular-nums shrink-0 w-28 text-right hidden md:block">
              {d.DHCP?.IP ? d.DHCP.IP.join(".") : "—"}
            </span>
            <span className="text-[11px] text-white/45 tabular-nums shrink-0 w-20 text-right hidden md:block">
              {d.StartPort}-{d.EndPort}
            </span>
            <span className="text-[11px] text-white/45 tabular-nums shrink-0 w-12 text-right">{d.CPU ?? "—"}%</span>
            <span className="text-[11px] text-white/45 tabular-nums shrink-0 w-12 text-right">{d.RAM ?? "—"}%</span>
            <span className="text-[11px] text-white/40 tabular-nums shrink-0 w-32 text-right">
              {d.Created ? dayjs(d.Created).format("HH:mm:ss DD-MM-YYYY") : "—"}
            </span>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">
            {filter ? "No matching devices" : "No connected devices"}
          </div>
        )}
      </div>
    </div>
  );
};

export default ServerDevices;
