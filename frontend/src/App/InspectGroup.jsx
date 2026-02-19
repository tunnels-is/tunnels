import React, { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import GLOBAL_STATE from "../state";
import { Plus, Trash2, Search, ChevronLeft, ChevronRight, Save } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const InspectGroup = () => {
  const { id } = useParams();
  const [users, setUsers] = useState([]);
  const [servers, setServers] = useState([]);
  const [devices, setDevices] = useState([]);
  const [dialog, setDialog] = useState(false);
  const [addId, setAddId] = useState("");
  const [group, setGroup] = useState();
  const [tab, setTab] = useState("user");
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;
  const state = GLOBAL_STATE("groups");

  const addToGroup = async () => {
    let e = await state.callController(null, "POST", "/v3/group/add",
      { GroupID: id, TypeID: addId, Type: tab, TypeTag: "" },
      false, false,
    );
    if (e.status === 200) {
      if (tab === "user") setUsers((prev) => [...prev, e.data]);
      else if (tab === "server") setServers((prev) => [...prev, e.data]);
      else if (tab === "device") setDevices((prev) => [...prev, e.data]);
      setDialog(false);
      setAddId("");
    }
  };

  const getEntities = async (type) => {
    let resp = await state.callController(null, "POST", "/v3/group/entities",
      { GID: id, Type: type, Limit: 1000, Offset: 0 },
      false, false,
    );
    if (type === "user") setUsers(resp.data);
    else if (type === "server") setServers(resp.data);
    else if (type === "device") setDevices(resp.data);
  };

  const removeEntity = async (gid, typeid, type) => {
    let e = await state.callController(null, "POST", "/v3/group/remove",
      { GroupID: gid, TypeID: typeid, Type: type },
      false, true,
    );
    if (e === true) {
      if (type === "user") setUsers((prev) => prev.filter((u) => u._id !== typeid));
      else if (type === "server") setServers((prev) => prev.filter((s) => s._id !== typeid));
      else if (type === "device") setDevices((prev) => prev.filter((d) => d._id !== typeid));
    }
  };

  const switchTab = async (t) => {
    setDialog(false);
    setTab(t);
    setFilter("");
    setPage(0);
    await getEntities(t);
  };

  const getGroup = async () => {
    let resp = await state.callController(null, "POST", "/v3/group", { GID: id }, false, false);
    if (resp.status === 200) setGroup(resp.data);
  };

  useEffect(() => {
    getGroup();
    getEntities("user");
  }, []);

  if (!group) return <div />;

  const dataMap = { user: users, server: servers, device: devices };
  const items = dataMap[tab] || [];
  const labelKey = tab === "user" ? "Email" : "Tag";

  const filtered = filter
    ? items.filter((item) => (item[labelKey]?.toLowerCase().includes(filter.toLowerCase())) || item._id?.includes(filter))
    : items;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  const accentColors = {
    user: "border-blue-500/20 hover:border-blue-500/50",
    server: "border-amber-500/20 hover:border-amber-500/50",
    device: "border-cyan-500/20 hover:border-cyan-500/50",
  };

  const tabs = [
    { key: "server", label: "Servers" },
    { key: "device", label: "Devices" },
    { key: "user", label: "Users" },
  ];

  return (
    <div>
      {/* ── Group banner ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-6">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-white/25 uppercase tracking-wider">Group</span>
          <code className="text-[13px] text-cyan-400/70 font-mono">{group.Tag}</code>
        </div>
        {group.Description && (
          <>
            <div className="w-px h-4 bg-white/[0.06]" />
            <span className="text-[12px] text-white/30">{group.Description}</span>
          </>
        )}
        <div className="w-px h-4 bg-white/[0.06]" />
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-white/25 uppercase tracking-wider">ID</span>
          <code className="text-[11px] text-white/30 font-mono">{group._id}</code>
        </div>
      </div>

      {/* ── Tabs + search bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${tab === t.key ? "bg-white/[0.07] text-white/70" : "text-white/20 hover:text-white/40"}`}
              onClick={() => switchTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </div>
        <div className="w-px h-4 bg-white/[0.06]" />
        <button
          className="text-white/20 hover:text-white/50 transition-colors"
          onClick={() => { setAddId(""); setDialog(true); }}
        >
          <Plus className="h-3.5 w-3.5" />
        </button>
        <div className="flex items-center gap-1.5 ml-auto">
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/15" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/15 outline-none focus:border-white/15 transition-colors"
              placeholder={`Filter ${tab}s...`}
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

      {/* ── Column headers ── */}
      <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
        <span className="text-[10px] text-white/15 uppercase tracking-wider flex-1 min-w-0">
          {tab === "user" ? "Email" : "Tag"}
        </span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-48 text-right hidden md:block">ID</span>
        <span className="shrink-0 w-6" />
      </div>

      {/* ── Rows ── */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((item, i) => (
          <div key={item._id || i} className={`group flex items-center gap-4 py-1.5 pl-3 border-l-2 ${accentColors[tab]} transition-colors`}>
            <span className="text-[13px] text-white/80 font-medium flex-1 min-w-0 truncate">
              {item[labelKey] || item._id || "—"}
            </span>
            <span className="text-[11px] text-white/20 font-mono tabular-nums shrink-0 w-48 text-right hidden md:block truncate">
              {item._id}
            </span>
            <div className="shrink-0 w-6 flex justify-end opacity-0 group-hover:opacity-100 transition-opacity">
              <button className="p-1 text-red-500/25 hover:text-red-400" onClick={() => removeEntity(id, item._id, tab)}>
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/15">
            {filter ? `No matching ${tab}s` : `No ${tab}s in this group`}
          </div>
        )}
      </div>

      {/* ── Add to group dialog ── */}
      <Dialog open={dialog} onOpenChange={setDialog}>
        <DialogContent className="sm:max-w-[400px] text-white bg-[#0a0d14] border-[#1e2433]">
          <DialogHeader>
            <DialogTitle className="text-lg font-bold text-white">Add {tab}</DialogTitle>
          </DialogHeader>

          <div className="pt-2">
            <label className="text-[10px] text-white/30 uppercase block mb-1">{tab} ID</label>
            <Input
              className="h-7 text-[12px] border-[#1e2433] bg-transparent"
              placeholder={`Enter ${tab} ID...`}
              value={addId}
              onChange={(e) => setAddId(e.target.value)}
            />
          </div>

          <DialogFooter className="flex gap-2 mt-2">
            <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={addToGroup}>
              <Save className="h-3 w-3 mr-1" /> Add
            </Button>
            <button className="text-[11px] text-white/30 hover:text-white/50 px-2" onClick={() => setDialog(false)}>
              Cancel
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default InspectGroup;
