import React, { useEffect, useState } from "react";
import { Search, ChevronLeft, ChevronRight, RefreshCw } from "lucide-react";
import dayjs from "dayjs";
import GLOBAL_STATE from "../state";

const DNSSort = (a, b) => {
  if (dayjs(a.LastSeen).unix() < dayjs(b.LastSeen).unix()) return 1;
  if (dayjs(a.LastSeen).unix() > dayjs(b.LastSeen).unix()) return -1;
  return 0;
};

const DNSStats = () => {
  const state = GLOBAL_STATE("dnsstats");
  const [tab, setTab] = useState("blocked");
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;

  useEffect(() => {
    state.GetDNSStats();
  }, []);

  const getBlockedDomains = () => {
    let dnsBlocks = state.DNSStats;
    if (!dnsBlocks || Object.keys(dnsBlocks).length === 0) return [];
    let stats = [];
    Object.entries(dnsBlocks).forEach(([key, value]) => {
      if (dayjs(value.LastSeen).diff(dayjs(value.LastBlocked), "s") > 0) return;
      stats.push({ ...value, tag: key });
    });
    return stats.sort(DNSSort);
  };

  const getResolvedDomains = () => {
    let dnsResolves = state.DNSStats;
    if (!dnsResolves || Object.keys(dnsResolves).length === 0) return [];
    let stats = [];
    Object.entries(dnsResolves).forEach(([key, value]) => {
      if (dayjs(value.LastSeen).diff(dayjs(value.LastBlocked), "s") > 0)
        stats.push({ ...value, tag: key });
    });
    return stats.sort(DNSSort);
  };

  const allItems = tab === "blocked" ? getBlockedDomains() : getResolvedDomains();
  const filtered = filter
    ? allItems.filter((d) => d.tag.toLowerCase().includes(filter.toLowerCase()))
    : allItems;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);
  const borderClass = tab === "blocked"
    ? "border-red-500/20 hover:border-red-500/50"
    : "border-violet-500/20 hover:border-violet-500/50";

  return (
    <div>
      {/* Header bar */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <div className="flex gap-1">
          {[{ key: "blocked", label: "Blocked" }, { key: "resolved", label: "Resolved" }].map((t) => (
            <button
              key={t.key}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                tab === t.key ? "bg-white/[0.07] text-white/70" : "text-white/20 hover:text-white/40"
              }`}
              onClick={() => { setTab(t.key); setPage(0); setFilter(""); }}
            >
              {t.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-1.5 ml-auto">
          <button
            className="p-1 text-white/20 hover:text-white/50 transition-colors"
            onClick={() => state.GetDNSStats()}
          >
            <RefreshCw className="h-3 w-3" />
          </button>
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/15" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/15 outline-none focus:border-white/15 transition-colors"
              placeholder="Filter domains..."
              value={filter}
              onChange={(e) => { setFilter(e.target.value); setPage(0); }}
            />
          </div>
          {filtered.length > PAGE_SIZE && (
            <div className="flex items-center gap-1">
              <button
                className="p-0.5 text-white/20 hover:text-white/50 disabled:opacity-30 disabled:cursor-default transition-colors"
                disabled={safePage === 0}
                onClick={() => setPage(safePage - 1)}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <span className="text-[10px] text-white/20 tabular-nums">{safePage + 1}/{totalPages}</span>
              <button
                className="p-0.5 text-white/20 hover:text-white/50 disabled:opacity-30 disabled:cursor-default transition-colors"
                disabled={safePage >= totalPages - 1}
                onClick={() => setPage(safePage + 1)}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          )}
          {filtered.length > 0 && (
            <span className="text-[10px] text-white/15 tabular-nums">{filtered.length}</span>
          )}
        </div>
      </div>

      {/* Column headers */}
      <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
        <span className="text-[10px] text-white/15 uppercase tracking-wider flex-1">Domain</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-12 text-right">Count</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-28 text-right hidden md:block">First seen</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-28 text-right">Last seen</span>
      </div>

      {/* Rows */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((d, i) => (
          <div key={i} className={`group flex items-center gap-4 py-1.5 pl-3 border-l-2 ${borderClass} transition-colors`}>
            <span className="text-[13px] text-white/75 font-mono flex-1 min-w-0 truncate">{d.tag}</span>
            <span className="text-[11px] text-white/30 tabular-nums shrink-0 w-12 text-right">{d.Count}</span>
            <span className="text-[11px] text-white/15 tabular-nums shrink-0 w-28 text-right hidden md:block">{dayjs(d.FirstSeen).format(state.DNSListDateFormat)}</span>
            <span className="text-[11px] text-white/20 tabular-nums shrink-0 w-28 text-right">{dayjs(d.LastSeen).format(state.DNSListDateFormat)}</span>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/15">
            {filter ? "No matching domains" : "No data"}
          </div>
        )}
      </div>
    </div>
  );
};

export default DNSStats;
