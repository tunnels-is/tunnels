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

const ServerFormDialog = ({ open, onOpenChange, onSave }) => {
  const state = GLOBAL_STATE("server-form");
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) {
      setForm(null);
      return;
    }
    setForm({ Tag: "", Country: "", IP: "", Port: "", DataPort: "", PubKey: "" });
  }, [open]);

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);

    const resp = await state.callController(null, "POST", "/client/server/create", { Server: form }, false, false);
    if (resp?.status === 200) {
      if (!state.PrivateServers) state.PrivateServers = [];
      state.PrivateServers.push(resp.data);
      state.updatePrivateServers();
      onSave?.();
      onOpenChange(false);
    }

    setSaving(false);
  };

  if (!form) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px] text-[#0a0a0a] bg-[#ffffff] border-[#e7e3d7]">
        <DialogHeader>
          <DialogTitle className="text-lg font-bold text-[#0a0a0a]">
            New Server
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-1">
          {/* Editable fields */}
          <div className="pt-3 grid grid-cols-2 gap-x-3 gap-y-3">
            <div>
              <label className="label">Tag</label>
              <Input className="h-7 text-[12px] border-[#e7e3d7] bg-transparent" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
            </div>
            <div>
              <label className="label">Country</label>
              <Input className="h-7 text-[12px] border-[#e7e3d7] bg-transparent" value={form.Country || ""} onChange={(e) => set("Country", e.target.value)} />
            </div>
            <div>
              <label className="label">IP</label>
              <Input className="h-7 text-[12px] border-[#e7e3d7] bg-transparent" value={form.IP || ""} onChange={(e) => set("IP", e.target.value)} />
            </div>
            <div>
              <label className="label">Port</label>
              <Input className="h-7 text-[12px] border-[#e7e3d7] bg-transparent" value={form.Port || ""} onChange={(e) => set("Port", e.target.value)} />
            </div>
            <div>
              <label className="label">Data Port</label>
              <Input className="h-7 text-[12px] border-[#e7e3d7] bg-transparent" value={form.DataPort || ""} onChange={(e) => set("DataPort", e.target.value)} />
            </div>
          </div>

          <div className="pt-3">
            <label className="label">Public Key</label>
            <Textarea
              className="text-[12px] border-[#e7e3d7] bg-transparent min-h-[60px] font-mono"
              value={form.PubKey || ""}
              onChange={(e) => set("PubKey", e.target.value)}
            />
          </div>
        </div>

        <DialogFooter className="flex gap-2 mt-2">
          <Button
            className="btn btn-primary btn-xs"
            onClick={handleSave}
            disabled={saving}
          >
            <Save className="h-3 w-3 mr-1" />
            {saving ? "Saving..." : "Save"}
          </Button>
          <button
            className="text-[11px] text-[#525252] hover:text-[#0a0a0a] px-2"
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
