import React, { useEffect, useState } from "react";
import dayjs from "dayjs";
import GLOBAL_STATE from "../state";
import { useNavigate } from "react-router-dom";
import { Search, ChevronLeft, ChevronRight, Pencil, Trash2, Plus, ChevronRight as Arrow, Save } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const Groups = () => {
  const state = GLOBAL_STATE("groups");
  const [groups, setGroups] = useState([]);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [form, setForm] = useState(null);
  const [isNew, setIsNew] = useState(false);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;
  const navigate = useNavigate();

  const getGroups = async () => {
    let resp = await state.callController(null, "POST", "/v3/group/list", {}, false, false);
    if (resp.status === 200) {
      setGroups(resp.data);
    }
  };

  useEffect(() => {
    getGroups();
  }, []);

  const saveGroup = async () => {
    let ok = false;
    if (!isNew && form._id !== undefined) {
      let resp = await state.callController(null, "POST", "/v3/group/update", { Group: form }, false, false);
      if (resp.status === 200) ok = true;
    } else {
      let resp = await state.callController(null, "POST", "/v3/group/create", { Group: form }, false, false);
      if (resp.status === 200) {
        ok = true;
        setGroups((prev) => [...prev, resp.data]);
      }
    }
    if (!ok) {
      state.toggleError("unable to save group");
    } else {
      setEditModalOpen(false);
    }
    state.renderPage("groups");
  };

  const openNew = () => {
    setForm({ Tag: "my-new-group", Description: "This is a new group" });
    setIsNew(true);
    setEditModalOpen(true);
  };

  const openEdit = (group) => {
    setForm({ ...group });
    setIsNew(false);
    setEditModalOpen(true);
  };

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const deleteGroup = async (id) => {
    let resp = await state.callController(null, "POST", "/v3/group/delete", { GID: id }, false, false);
    if (resp.status === 200) {
      setGroups((prev) => prev.filter((g) => g._id !== id));
    }
  };

  const filtered = filter
    ? groups.filter((g) => g.Tag?.toLowerCase().includes(filter.toLowerCase()) || g._id?.includes(filter))
    : groups;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  return (
    <div>
      {/* ── Header bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <span className="text-[10px] text-white/45 uppercase tracking-wider">Groups</span>
        {filtered.length > 0 && (
          <>
            <div className="w-px h-4 bg-white/[0.06]" />
            <span className="text-[10px] text-white/40 tabular-nums">{filtered.length} total</span>
          </>
        )}
        <div className="flex items-center gap-1.5 ml-auto">
          <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors mr-1" onClick={openNew}>
            <Plus className="h-3 w-3" /> New
          </button>
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/40" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/50 outline-none focus:border-white/25 transition-colors"
              placeholder="Filter groups..."
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
        <span className="text-[10px] text-white/40 uppercase tracking-wider flex-1 min-w-0">Group</span>
        <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-36 text-right hidden md:block">Created</span>
        <span className="shrink-0 w-16" />
      </div>

      {/* ── Rows ── */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((g, i) => (
          <div key={g._id || i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-violet-500/20 hover:border-violet-500/50 transition-colors">
            <div className="flex-1 min-w-0 cursor-pointer" onClick={() => navigate("/groups/" + g._id)}>
              <span className="text-[13px] text-white/80 font-medium truncate block hover:text-white transition-colors">{g.Tag}</span>
              <span className="text-[11px] text-white/40 font-mono truncate block">{g._id}</span>
              {g.Description && (
                <span className="text-[11px] text-white/40 truncate block">{g.Description}</span>
              )}
            </div>
            <span className="text-[11px] text-white/40 tabular-nums shrink-0 w-36 text-right hidden md:block">
              {g.CreatedAt ? dayjs(g.CreatedAt).format("HH:mm DD-MM-YYYY") : "—"}
            </span>
            <div className="shrink-0 w-16 flex justify-end gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button className="p-1 text-white/40 hover:text-white/60" onClick={(e) => { e.stopPropagation(); navigate("/groups/" + g._id); }}>
                <Arrow className="h-3 w-3" />
              </button>
              <button className="p-1 text-white/40 hover:text-white/60" onClick={(e) => { e.stopPropagation(); openEdit(g); }}>
                <Pencil className="h-3 w-3" />
              </button>
              <button className="p-1 text-red-500/25 hover:text-red-400" onClick={(e) => { e.stopPropagation(); deleteGroup(g._id); }}>
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">
            {filter ? "No matching groups" : "No groups"}
          </div>
        )}
      </div>

      {/* ── Group form dialog ── */}
      <Dialog open={editModalOpen} onOpenChange={setEditModalOpen}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {form && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">
                  {isNew ? "New Group" : "Edit Group"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-1">
                {form._id && (
                  <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                    <span className="text-[11px] text-white/45 shrink-0 w-[50px]">ID</span>
                    <code className="text-[13px] text-white/50 font-mono truncate">{form._id}</code>
                  </div>
                )}

                <div className="pt-3 space-y-3">
                  <div>
                    <label className="text-[10px] text-white/50 uppercase block mb-1">Tag</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
                  </div>
                  <div>
                    <label className="text-[10px] text-white/50 uppercase block mb-1">Description</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Description || ""} onChange={(e) => set("Description", e.target.value)} />
                  </div>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={saveGroup}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => setEditModalOpen(false)}>
                  Cancel
                </button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default Groups;
