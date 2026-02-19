import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { Search, ChevronLeft, ChevronRight, Pencil, Trash2, Plus, Save, Minus } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const Devices = () => {
  const [devices, setDevices] = useState([]);
  const state = GLOBAL_STATE("devices");
  const [form, setForm] = useState(null);
  const [isNew, setIsNew] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;

  const getDevices = async (offset, limit) => {
    let resp = await state.callController(null, "POST", "/v3/device/list", { Offset: offset, Limit: limit }, false, false);
    if (resp.status === 200) {
      setDevices(resp.data);
      state.renderPage("devices");
    }
  };

  const deleteDevice = async (id) => {
    let ok = await state.callController(null, "POST", "/v3/device/delete", { DID: id }, false, true);
    if (ok === true) {
      let d = devices.filter((d) => d._id !== id);
      setDevices([...d]);
      state.renderPage("devices");
    }
  };

  const saveDevice = async () => {
    let ok = false;
    if (!isNew && form._id !== undefined) {
      let resp = await state.callController(null, "POST", "/v3/device/update", { Device: form }, false, false);
      if (resp.status === 200) ok = true;
    } else {
      let resp = await state.callController(null, "POST", "/v3/device/create", { Device: form }, false, false);
      if (resp.status === 200) {
        ok = true;
        setDevices((prev) => [...prev, resp.data]);
      }
    }
    if (ok) {
      setEditModalOpen(false);
      state.renderPage("devices");
    }
  };

  const openEdit = (device) => {
    setForm({ ...device, Groups: [...(device.Groups || [])] });
    setIsNew(false);
    setEditModalOpen(true);
  };

  const openNew = () => {
    setForm({ Tag: "", Groups: [] });
    setIsNew(true);
    setEditModalOpen(true);
  };

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const updateGroup = (i, val) => {
    setForm((f) => {
      const groups = [...(f.Groups || [])];
      groups[i] = val;
      return { ...f, Groups: groups };
    });
  };
  const removeGroup = (i) => {
    setForm((f) => ({ ...f, Groups: (f.Groups || []).filter((_, idx) => idx !== i) }));
  };
  const addGroup = () => {
    setForm((f) => ({ ...f, Groups: [...(f.Groups || []), ""] }));
  };

  useEffect(() => {
    getDevices(0, 100);
  }, []);

  const filtered = filter
    ? devices.filter((d) => d.Tag?.toLowerCase().includes(filter.toLowerCase()) || d._id?.includes(filter))
    : devices;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  return (
    <div>
      {/* ── Header bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <span className="text-[10px] text-white/25 uppercase tracking-wider">Devices</span>
        {filtered.length > 0 && (
          <>
            <div className="w-px h-4 bg-white/[0.06]" />
            <span className="text-[10px] text-white/15 tabular-nums">{filtered.length} total</span>
          </>
        )}
        <div className="flex items-center gap-1.5 ml-auto">
          <button className="text-white/20 hover:text-white/50 transition-colors mr-1" onClick={openNew}>
            <Plus className="h-3.5 w-3.5" />
          </button>
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/15" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/15 outline-none focus:border-white/15 transition-colors"
              placeholder="Filter devices..."
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
        </div>
      </div>

      {/* ── Column headers ── */}
      <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
        <span className="text-[10px] text-white/15 uppercase tracking-wider flex-1 min-w-0">Device</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-36 text-right">Created</span>
        <span className="shrink-0 w-12" />
      </div>

      {/* ── Rows ── */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((d, i) => (
          <div key={d._id || i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-cyan-500/20 hover:border-cyan-500/50 transition-colors">
            <div className="flex-1 min-w-0">
              <span className="text-[13px] text-white/80 font-medium truncate block">{d.Tag || "Unnamed"}</span>
              <span className="text-[11px] text-white/20 font-mono truncate block">{d._id}</span>
            </div>
            <span className="text-[11px] text-white/20 tabular-nums shrink-0 w-36 text-right">
              {d.CreatedAt ? dayjs(d.CreatedAt).format("HH:mm DD-MM-YYYY") : "—"}
            </span>
            <div className="shrink-0 w-12 flex justify-end gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button className="p-1 text-white/20 hover:text-white/50" onClick={() => openEdit(d)}>
                <Pencil className="h-3 w-3" />
              </button>
              <button className="p-1 text-red-500/25 hover:text-red-400" onClick={() => deleteDevice(d._id)}>
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/15">
            {filter ? "No matching devices" : "No devices"}
          </div>
        )}
      </div>

      {/* ── Device form dialog ── */}
      <Dialog open={editModalOpen} onOpenChange={setEditModalOpen}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {form && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">
                  {isNew ? "New Device" : "Edit Device"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-1">
                {/* Read-only ID */}
                {form._id && (
                  <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                    <span className="text-[11px] text-white/25 shrink-0 w-[50px]">ID</span>
                    <code className="text-[13px] text-white/50 font-mono truncate">{form._id}</code>
                  </div>
                )}

                <div className="pt-3 space-y-3">
                  <div>
                    <label className="text-[10px] text-white/30 uppercase block mb-1">Tag</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
                  </div>

                  {/* Groups array */}
                  <div>
                    <label className="text-[10px] text-white/30 uppercase block mb-1">Groups</label>
                    <div className="space-y-1">
                      {(form.Groups || []).map((g, i) => (
                        <div key={i} className="flex items-center gap-1">
                          <Input
                            className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent"
                            value={g}
                            onChange={(e) => updateGroup(i, e.target.value)}
                          />
                          <button className="p-1 text-red-400/60 hover:text-red-400" onClick={() => removeGroup(i)}>
                            <Minus className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      ))}
                      <button
                        className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-1"
                        onClick={addGroup}
                      >
                        <Plus className="w-3 h-3" /> Add Group
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={saveDevice}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/30 hover:text-white/50 px-2" onClick={() => setEditModalOpen(false)}>
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

export default Devices;
