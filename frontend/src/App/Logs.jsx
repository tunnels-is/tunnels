import STORE from "@/store";
import GLOBAL_STATE from "../state";
import { useState, useMemo } from "react";
import { Search, ChevronLeft, ChevronRight, Trash2 } from "lucide-react";

const Logs = () => {
  const state = GLOBAL_STATE("logs");
  const [page, setPage] = useState(0);
  const [filter, setFilter] = useState("");
  const [tagFilter, setTagFilter] = useState("");
  const PAGE_SIZE = 100;

  let logs = STORE.Cache.GetObject("logs");

  const filteredLogs = useMemo(() => {
    if (!logs) return [];
    let filtered = logs;
    if (filter) {
      filtered = filtered.filter((line) =>
        line.toLowerCase().includes(filter.toLowerCase())
      );
    }
    if (tagFilter) {
      filtered = filtered.filter((line) => {
        const parts = line.split(" || ");
        return parts[1]?.trim() === tagFilter;
      });
    } else {
      filtered = filtered.filter((line) => {
        const parts = line.split(" || ");
        return parts[1]?.trim() !== "ROUTINE";
      });
    }
    return filtered.toReversed();
  }, [logs, filter, tagFilter]);

  const totalPages = Math.max(1, Math.ceil(filteredLogs.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filteredLogs.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  const tags = [
    { key: "",        label: "All",     tone: "neutral" },
    { key: "INFO",    label: "Info",    tone: "info" },
    { key: "ERROR",   label: "Error",   tone: "danger" },
    { key: "DEBUG",   label: "Debug",   tone: "warning" },
    { key: "ROUTINE", label: "Routine", tone: "info" },
  ];

  const activeTagClass = (tone) => ({
    neutral: "bg-black/[0.05] text-[#0a0a0a]",
    info:    "bg-[#1d4ed8]/10 text-[#1d4ed8] ring-1 ring-inset ring-[#1d4ed8]/25",
    danger:  "bg-[#dc2626]/10 text-[#dc2626] ring-1 ring-inset ring-[#dc2626]/25",
    warning: "bg-[#b45309]/10 text-[#b45309] ring-1 ring-inset ring-[#b45309]/25",
    success: "bg-[#15803d]/10 text-[#15803d] ring-1 ring-inset ring-[#15803d]/25",
  }[tone] || "bg-black/[0.05] text-[#0a0a0a]");

  const getBorderClass = (line) => {
    if (line.includes("| ERROR |")) return "border-[#dc2626]/30";
    if (line.includes("| DEBUG |")) return "border-[#b45309]/30";
    if (line.includes("| INFO  |")) return "border-[#1d4ed8]/20";
    if (line.includes("| ROUTINE |")) return "border-[#1d4ed8]/20";
    return "border-black/[0.06]";
  };

  const getTagClass = (line) => {
    if (line.includes("| ERROR |")) return "text-[#dc2626]";
    if (line.includes("| DEBUG |")) return "text-[#b45309]/60";
    if (line.includes("| INFO  |")) return "text-[#1d4ed8]/60";
    if (line.includes("| ROUTINE |")) return "text-[#1d4ed8]/70";
    return "text-[#525252]";
  };

  return (
    <div className="flex flex-col h-[calc(100vh-60px)]">
      {/* Header bar */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#ffffff]/80 border border-[#e7e3d7] mb-4 shrink-0 card-shadow">
        <div className="flex items-center gap-1.5">
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-[#a3a3a3]" />
            <input
              className="h-6 w-48 pl-7 pr-2 text-[11px] rounded bg-black/[0.03] border border-black/[0.06] text-[#525252] placeholder:text-[#a3a3a3] outline-none focus:border-black/25 transition-colors"
              placeholder="Filter logs..."
              value={filter}
              onChange={(e) => { setFilter(e.target.value); setPage(0); }}
            />
          </div>
          <button
            className="btn-icon btn-icon-danger"
            onClick={() => { STORE.Cache.SetObject("logs", []); state.renderPage("logs"); }}
            title="Clear logs"
          >
            <Trash2 className="h-3 w-3" />
          </button>
          {filteredLogs.length > PAGE_SIZE && (
            <div className="flex items-center gap-1">
              <button
                className="btn-icon btn-icon-xs"
                disabled={safePage === 0}
                onClick={() => setPage(safePage - 1)}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
              <span className="text-[10px] text-[#a3a3a3] tabular-nums">
                {safePage + 1}/{totalPages}
              </span>
              <button
                className="btn-icon btn-icon-xs"
                disabled={safePage >= totalPages - 1}
                onClick={() => setPage(safePage + 1)}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          )}
          {filteredLogs.length > 0 && (
            <span className="text-[10px] text-[#a3a3a3] tabular-nums">{filteredLogs.length}</span>
          )}
        </div>

        <div className="flex gap-1 ml-auto">
          {tags.map((t) => (
            <button
              key={t.key}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                tagFilter === t.key
                  ? activeTagClass(t.tone)
                  : "text-[#a3a3a3] hover:text-[#525252]"
              }`}
              onClick={() => { setTagFilter(t.key); setPage(0); }}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Log rows */}
      <div className="flex-1 overflow-y-auto">
        {paged.length > 0 ? paged.map((line, i) => {
          const parts = line.split(" || ");
          const timestamp = parts[0];
          const tag = parts[1];
          const func = parts[2];
          const message = parts.slice(3).join(" || ");

          return (
            <div
              key={i}
              className={`flex items-baseline gap-1.5 py-0 pl-3 border-l-2 ${getBorderClass(line)} hover:bg-black/[0.01] transition-colors`}
            >
              <span className="text-[10px] text-[#a3a3a3] font-mono tabular-nums shrink-0">
                {timestamp}
              </span>
              <span className={`text-[10px] font-medium uppercase shrink-0 w-[40px] ${getTagClass(line)}`}>
                {tag?.trim()}
              </span>
              <span className="text-[11px] text-[#15803d]/40 font-mono shrink-0 max-w-[180px] truncate hidden lg:block">
                {func}
              </span>
              <span className="text-[11px] text-[#0a0a0a] font-mono min-w-0 truncate">
                {message}
              </span>
            </div>
          );
        }) : (
          <div className="py-6 pl-3 border-l-2 border-black/[0.04] text-[12px] text-[#a3a3a3]">
            {filter || tagFilter ? "No matching logs" : "No logs"}
          </div>
        )}
      </div>
    </div>
  );
};

export default Logs;
