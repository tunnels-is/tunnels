import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Save } from "lucide-react";
import GLOBAL_STATE from "../state";

const ServerFormDialog = ({ open, onOpenChange, server, onSave }) => {
  const state = GLOBAL_STATE("server-form");
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) {
      setForm(null);
      return;
    }
    if (server) {
      setForm({ ...server });
    } else {
      setForm({ Tag: "", Country: "", IP: "", Port: "", DataPort: "", PubKey: "" });
    }
  }, [open, server]);

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);

    let ok = false;
    if (form._id) {
      const resp = await state.callController(null, "POST", "/v3/server/update", { Server: form }, false, false);
      if (resp?.status === 200) {
        state.PrivateServers?.forEach((s, i) => {
          if (s._id === form._id) {
            state.PrivateServers[i] = form;
          }
        });
        state.updatePrivateServers();
        ok = true;
      }
    } else {
      const resp = await state.callController(null, "POST", "/v3/server/create", { Server: form }, false, false);
      if (resp?.status === 200) {
        if (!state.PrivateServers) state.PrivateServers = [];
        state.PrivateServers.push(resp.data);
        state.updatePrivateServers();
        ok = true;
      }
    }

    setSaving(false);
    if (ok) {
      onSave?.();
      onOpenChange(false);
    }
  };

  if (!form) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
        <DialogHeader>
          <DialogTitle className="text-lg font-bold text-white">
            {server ? `Edit Server: ${server.Tag}` : "New Server"}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-1">
          {/* Read-only ID */}
          {form._id && (
            <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
              <span className="text-[11px] text-white/45 shrink-0 w-[50px]">ID</span>
              <code className="text-[13px] text-white/50 font-mono truncate">{form._id}</code>
            </div>
          )}

          {/* Editable fields */}
          <div className="pt-3 grid grid-cols-2 gap-x-3 gap-y-3">
            <div>
              <label className="text-[10px] text-white/50 uppercase block mb-1">Tag</label>
              <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
            </div>
            <div>
              <label className="text-[10px] text-white/50 uppercase block mb-1">Country</label>
              <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Country || ""} onChange={(e) => set("Country", e.target.value)} />
            </div>
            <div>
              <label className="text-[10px] text-white/50 uppercase block mb-1">IP</label>
              <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.IP || ""} onChange={(e) => set("IP", e.target.value)} />
            </div>
            <div>
              <label className="text-[10px] text-white/50 uppercase block mb-1">Port</label>
              <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Port || ""} onChange={(e) => set("Port", e.target.value)} />
            </div>
            <div>
              <label className="text-[10px] text-white/50 uppercase block mb-1">Data Port</label>
              <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.DataPort || ""} onChange={(e) => set("DataPort", e.target.value)} />
            </div>
          </div>

          <div className="pt-3">
            <label className="text-[10px] text-white/50 uppercase block mb-1">Public Key</label>
            <Textarea
              className="text-[12px] border-[#1e2433] bg-transparent min-h-[60px] font-mono"
              value={form.PubKey || ""}
              onChange={(e) => set("PubKey", e.target.value)}
            />
          </div>
        </div>

        <DialogFooter className="flex gap-2 mt-2">
          <Button
            className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5"
            onClick={handleSave}
            disabled={saving}
          >
            <Save className="h-3 w-3 mr-1" />
            {saving ? "Saving..." : "Save"}
          </Button>
          <button
            className="text-[11px] text-white/50 hover:text-white/70 px-2"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default ServerFormDialog;
