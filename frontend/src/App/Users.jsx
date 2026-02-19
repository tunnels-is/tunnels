import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { Search, ChevronLeft, ChevronRight, Pencil, Save } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

import { Button } from "@/components/ui/button";

const Users = () => {
  const [users, setUsers] = useState([]);
  const [selectedUser, setSelectedUser] = useState(undefined);
  const [form, setForm] = useState(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const PAGE_SIZE = 50;
  const state = GLOBAL_STATE("groups");

  const getUsers = async (offset, limit) => {
    let resp = await state.callController(null, "POST", "/v3/user/list", { Offset: offset, Limit: limit }, false, false);
    if (resp.status === 200) {
      if (resp.data?.length === 0) {
        state.successNotification("no more users");
      } else {
        setUsers(resp.data);
      }
    }
  };

  useEffect(() => {
    getUsers(0, PAGE_SIZE);
  }, []);

  const openEdit = (user) => {
    setSelectedUser(user);
    setForm({
      Email: user.Email || "",
      Disabled: !!user.Disabled,
      IsManager: !!user.IsManager,
      Trial: !!user.Trial,
      SubExpiration: user.SubExpiration || "",
    });
    setModalOpen(true);
  };

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const saveUser = async () => {
    let resp = await state.callController(
      null, "POST", "/v3/user/adminupdate",
      {
        TargetUserID: selectedUser._id,
        Email: form.Email,
        Disabled: form.Disabled,
        IsManager: form.IsManager,
        Trial: form.Trial,
        SubExpiration: form.SubExpiration,
      },
      false, true,
    );
    if (resp === true) {
      state.successNotification("User updated successfully");
      setModalOpen(false);
      await getUsers(0, PAGE_SIZE);
    }
  };

  const filtered = filter
    ? users.filter((u) => u.Email?.toLowerCase().includes(filter.toLowerCase()) || u._id?.includes(filter))
    : users;
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages - 1);
  const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  return (
    <div>
      {/* ── Header bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-4">
        <span className="text-[10px] text-white/25 uppercase tracking-wider">Users</span>
        {filtered.length > 0 && (
          <>
            <div className="w-px h-4 bg-white/[0.06]" />
            <span className="text-[10px] text-white/15 tabular-nums">{filtered.length} total</span>
          </>
        )}
        <div className="flex items-center gap-1.5 ml-auto">
          <div className="relative">
            <Search className="h-3 w-3 absolute left-2 top-1/2 -translate-y-1/2 text-white/15" />
            <input
              className="h-6 w-40 pl-7 pr-2 text-[11px] rounded bg-white/[0.03] border border-white/[0.06] text-white/60 placeholder:text-white/15 outline-none focus:border-white/15 transition-colors"
              placeholder="Filter users..."
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
        <span className="text-[10px] text-white/15 uppercase tracking-wider flex-1 min-w-0">Email</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-14 text-center">Trial</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-36 text-right hidden md:block">Subscription</span>
        <span className="text-[10px] text-white/15 uppercase tracking-wider shrink-0 w-36 text-right">Updated</span>
        <span className="shrink-0 w-6" />
      </div>

      {/* ── Rows ── */}
      <div className="space-y-px">
        {paged.length > 0 ? paged.map((u, i) => (
          <div key={u._id || i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-blue-500/20 hover:border-blue-500/50 transition-colors">
            <div className="flex-1 min-w-0">
              <span className="text-[13px] text-white/80 font-medium truncate block">{u.Email}</span>
              <span className="text-[11px] text-white/20 font-mono truncate block">{u._id}</span>
            </div>
            <span className={`text-[11px] shrink-0 w-14 text-center ${u.Trial ? "text-emerald-400/70" : "text-white/20"}`}>
              {u.Trial ? "yes" : "no"}
            </span>
            <span className="text-[11px] text-white/25 tabular-nums shrink-0 w-36 text-right hidden md:block">
              {u.SubExpiration ? dayjs(u.SubExpiration).format("HH:mm DD-MM-YYYY") : "—"}
            </span>
            <span className="text-[11px] text-white/20 tabular-nums shrink-0 w-36 text-right">
              {u.Updated ? dayjs(u.Updated).format("HH:mm DD-MM-YYYY") : "—"}
            </span>
            <div className="shrink-0 w-6 flex justify-end opacity-0 group-hover:opacity-100 transition-opacity">
              <button className="p-1 text-white/20 hover:text-white/50" onClick={() => openEdit(u)}>
                <Pencil className="h-3 w-3" />
              </button>
            </div>
          </div>
        )) : (
          <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/15">
            {filter ? "No matching users" : "No users"}
          </div>
        )}
      </div>

      {/* ── Edit User dialog ── */}
      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {form && selectedUser && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">Edit User</DialogTitle>
              </DialogHeader>

              <div className="space-y-1">
                {/* Read-only info */}
                <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                  <span className="text-[11px] text-white/25 shrink-0 w-[90px]">ID</span>
                  <code className="text-[13px] text-white/50 font-mono truncate">{selectedUser._id}</code>
                </div>
                <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                  <span className="text-[11px] text-white/25 shrink-0 w-[90px]">Updated</span>
                  <code className="text-[13px] text-white/50 font-mono">{selectedUser.Updated ? dayjs(selectedUser.Updated).format("HH:mm:ss DD-MM-YYYY") : "—"}</code>
                </div>

                {/* Editable fields */}
                <div className="pt-3 space-y-3">
                  <div>
                    <label className="text-[10px] text-white/30 uppercase block mb-1">Email</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Email} onChange={(e) => set("Email", e.target.value)} />
                  </div>
                  <div>
                    <label className="text-[10px] text-white/30 uppercase block mb-1">Subscription Expiration</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.SubExpiration} onChange={(e) => set("SubExpiration", e.target.value)} />
                  </div>
                </div>

                {/* Boolean toggles */}
                <div className="flex items-center gap-2 pt-3">
                  {[
                    { key: "Trial", label: "Trial" },
                    { key: "IsManager", label: "Manager" },
                    { key: "Disabled", label: "Disabled" },
                  ].map((opt) => (
                    <button
                      key={opt.key}
                      className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                        form[opt.key]
                          ? opt.key === "Disabled"
                            ? "border-red-500/40 bg-red-500/15 text-red-400 shadow-[0_0_12px_rgba(239,68,68,0.12)]"
                            : "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                          : "border-white/[0.06] bg-white/[0.02] text-white/30 hover:text-white/50 hover:border-white/15 hover:bg-white/[0.04]"
                      }`}
                      onClick={() => set(opt.key, !form[opt.key])}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={saveUser}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/30 hover:text-white/50 px-2" onClick={() => setModalOpen(false)}>
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

export default Users;
