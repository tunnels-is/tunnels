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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Save, Plus, Minus, ChevronDown, ChevronRight } from "lucide-react";
import GLOBAL_STATE from "../state";

const encTypes = [
  { value: "0", label: "None" },
  { value: "1", label: "AES-128" },
  { value: "2", label: "AES-256" },
  { value: "3", label: "ChaCha20" },
];

const Section = ({ title, children, defaultOpen = true }) => {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="pt-4">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-white/25 hover:text-white/40 transition-colors mb-2"
      >
        {open ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
        {title}
      </button>
      {open && children}
    </div>
  );
};

const StringArrayField = ({ label, items, onChange }) => {
  const update = (i, val) => { const next = [...items]; next[i] = val; onChange(next); };
  const remove = (i) => onChange(items.filter((_, idx) => idx !== i));
  const add = () => onChange([...items, ""]);

  return (
    <div className="mt-3">
      {label && <label className="text-[10px] text-white/30 uppercase block mb-1">{label}</label>}
      <div className="space-y-1">
        {items.map((item, i) => (
          <div key={i} className="flex items-center gap-1">
            <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" value={item} onChange={(e) => update(i, e.target.value)} />
            <button type="button" onClick={() => remove(i)} className="p-1 text-red-400/60 hover:text-red-400">
              <Minus className="w-3.5 h-3.5" />
            </button>
          </div>
        ))}
        <button type="button" onClick={add} className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-1">
          <Plus className="w-3 h-3" /> Add
        </button>
      </div>
    </div>
  );
};

const DNSRecordEditor = ({ record, onChange, onRemove }) => {
  const set = (key, val) => onChange({ ...record, [key]: val });
  const updateArr = (key, i, val) => { const next = [...(record[key] || [])]; next[i] = val; set(key, next); };
  const removeArr = (key, i) => set(key, (record[key] || []).filter((_, idx) => idx !== i));
  const addArr = (key) => set(key, [...(record[key] || []), ""]);

  return (
    <div className="mt-2 py-2 pl-3 border-l-2 border-cyan-500/20 space-y-2">
      <div className="flex items-center gap-2">
        <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Domain" value={record.Domain || ""} onChange={(e) => set("Domain", e.target.value)} />
        <button
          type="button"
          className={`text-[10px] px-2 py-0.5 rounded-full border transition-all cursor-pointer shrink-0 ${
            record.Wildcard
              ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
              : "border-white/[0.06] bg-white/[0.02] text-white/30 hover:text-white/50 hover:border-white/15 hover:bg-white/[0.04]"
          }`}
          onClick={() => set("Wildcard", !record.Wildcard)}
        >*</button>
        <button type="button" onClick={onRemove} className="p-1 text-red-400/40 hover:text-red-400 shrink-0">
          <Minus className="w-3.5 h-3.5" />
        </button>
      </div>
      {/* IP addresses */}
      <div>
        <span className="text-[10px] text-white/20 uppercase">IPs</span>
        <div className="space-y-1 mt-0.5">
          {(record.IP || []).map((ip, i) => (
            <div key={i} className="flex items-center gap-1">
              <Input className="flex-1 h-6 text-[11px] border-[#1e2433] bg-transparent" value={ip} onChange={(e) => updateArr("IP", i, e.target.value)} />
              <button type="button" onClick={() => removeArr("IP", i)} className="p-0.5 text-red-400/40 hover:text-red-400"><Minus className="w-3 h-3" /></button>
            </div>
          ))}
          <button type="button" onClick={() => addArr("IP")} className="text-[10px] text-emerald-400/50 hover:text-emerald-400"><Plus className="w-3 h-3 inline" /> IP</button>
        </div>
      </div>
      {/* TXT records */}
      <div>
        <span className="text-[10px] text-white/20 uppercase">TXT</span>
        <div className="space-y-1 mt-0.5">
          {(record.TXT || []).map((txt, i) => (
            <div key={i} className="flex items-center gap-1">
              <Input className="flex-1 h-6 text-[11px] border-[#1e2433] bg-transparent" value={txt} onChange={(e) => updateArr("TXT", i, e.target.value)} />
              <button type="button" onClick={() => removeArr("TXT", i)} className="p-0.5 text-red-400/40 hover:text-red-400"><Minus className="w-3 h-3" /></button>
            </div>
          ))}
          <button type="button" onClick={() => addArr("TXT")} className="text-[10px] text-emerald-400/50 hover:text-emerald-400"><Plus className="w-3 h-3 inline" /> TXT</button>
        </div>
      </div>
    </div>
  );
};

const NetworkEditor = ({ net, onChange, onRemove }) => {
  const set = (key, val) => onChange({ ...net, [key]: val });
  return (
    <div className="flex items-center gap-1.5 mt-1.5 pl-3 border-l-2 border-violet-500/15 py-1">
      <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Tag" value={net.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
      <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Network" value={net.Network || ""} onChange={(e) => set("Network", e.target.value)} />
      <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Nat" value={net.Nat || ""} onChange={(e) => set("Nat", e.target.value)} />
      <button type="button" onClick={onRemove} className="p-1 text-red-400/40 hover:text-red-400 shrink-0">
        <Minus className="w-3.5 h-3.5" />
      </button>
    </div>
  );
};

const RouteEditor = ({ route, onChange, onRemove }) => {
  const set = (key, val) => onChange({ ...route, [key]: val });
  return (
    <div className="flex items-center gap-1.5 mt-1.5 pl-3 border-l-2 border-amber-500/15 py-1">
      <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Address" value={route.Address || ""} onChange={(e) => set("Address", e.target.value)} />
      <Input className="w-24 h-7 text-[12px] border-[#1e2433] bg-transparent" placeholder="Metric" value={route.Metric || ""} onChange={(e) => set("Metric", e.target.value)} />
      <button type="button" onClick={onRemove} className="p-1 text-red-400/40 hover:text-red-400 shrink-0">
        <Minus className="w-3.5 h-3.5" />
      </button>
    </div>
  );
};

const TunnelFormDialog = ({ open, onOpenChange, tunnel, servers, onSave }) => {
  const state = GLOBAL_STATE("tunnel-form");
  const [form, setForm] = useState(null);
  const [originalTag, setOriginalTag] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) { setForm(null); return; }
    if (tunnel) {
      const clone = JSON.parse(JSON.stringify(tunnel));
      if (!clone.DNSRecords) clone.DNSRecords = [];
      if (!clone.Networks) clone.Networks = [];
      if (!clone.Routes) clone.Routes = [];
      if (!clone.AllowedHosts) clone.AllowedHosts = [];
      if (!clone.DNSServers) clone.DNSServers = [];
      setForm(clone);
      setOriginalTag(tunnel.Tag);
    } else {
      setForm(null);
      setOriginalTag("");
    }
  }, [open, tunnel]);

  const set = (key, val) => setForm((f) => ({ ...f, [key]: val }));

  const setArrayItem = (key, index, val) => {
    setForm((f) => {
      const next = { ...f };
      next[key] = [...(next[key] || [])];
      next[key][index] = val;
      return next;
    });
  };

  const removeArrayItem = (key, index) => {
    setForm((f) => ({ ...f, [key]: (f[key] || []).filter((_, i) => i !== index) }));
  };

  const addArrayItem = (key, defaultVal) => {
    setForm((f) => ({ ...f, [key]: [...(f[key] || []), defaultVal] }));
  };

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);
    const ok = await state.v2_TunnelSave(form, originalTag || form.Tag);
    setSaving(false);
    if (ok) { onSave?.(); onOpenChange(false); }
  };

  if (!form) return null;

  const featureToggles = [
    { key: "DNSBlocking", label: "DNS Blocking" },
    { key: "LocalhostNat", label: "Localhost NAT" },
    { key: "AutoReconnect", label: "Auto Reconnect" },
    { key: "AutoConnect", label: "Auto Connect" },
    { key: "RequestVPNPorts", label: "VPN Ports" },
    { key: "KillSwitch", label: "Kill Switch" },
    { key: "EnableDefaultRoute", label: "Default Route" },
    { key: "DisableFirewall", label: "Disable FW" },
  ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[640px] text-white bg-[#0a0d14] border-[#1e2433]">
        <DialogHeader>
          <DialogTitle className="text-lg font-bold text-white">
            {tunnel ? `Edit Tunnel: ${originalTag}` : "New Tunnel"}
          </DialogTitle>
        </DialogHeader>

        <div className="max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">

          {/* ── Identity ── */}
          <Section title="Identity" defaultOpen={true}>
            <div className="grid grid-cols-2 gap-x-3 gap-y-3">
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">Tag</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">Interface</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.IFName || ""} onChange={(e) => set("IFName", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">Server</label>
                <Select value={form.ServerID || "_none"} onValueChange={(v) => set("ServerID", v === "_none" ? "" : v)}>
                  <SelectTrigger className="h-7 text-[12px] border-[#1e2433] bg-transparent">
                    <SelectValue placeholder="No server" />
                  </SelectTrigger>
                  <SelectContent className="bg-[#0a0d14] border-[#1e2433]">
                    <SelectItem value="_none" className="text-[12px]">None</SelectItem>
                    {servers?.map((s) => (
                      <SelectItem key={s._id} value={s._id} className="text-[12px]">{s.Tag} ({s.IP})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">Encryption</label>
                <Select value={String(form.EncryptionType ?? 0)} onValueChange={(v) => set("EncryptionType", Number(v))}>
                  <SelectTrigger className="h-7 text-[12px] border-[#1e2433] bg-transparent">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="bg-[#0a0d14] border-[#1e2433]">
                    {encTypes.map((t) => (
                      <SelectItem key={t.value} value={t.value} className="text-[12px]">{t.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </Section>

          {/* ── Network ── */}
          <Section title="Network" defaultOpen={true}>
            <div className="grid grid-cols-2 gap-x-3 gap-y-3">
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">IPv4 Address</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.IPv4Address || ""} onChange={(e) => set("IPv4Address", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">IPv6 Address</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.IPv6Address || ""} onChange={(e) => set("IPv6Address", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">Netmask</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={form.NetMask || ""} onChange={(e) => set("NetMask", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">MTU</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" type="number" value={form.MTU ?? 1420} onChange={(e) => set("MTU", Number(e.target.value))} />
              </div>
              <div>
                <label className="text-[10px] text-white/30 uppercase block mb-1">TX Queue Length</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" type="number" value={form.TxQueueLen ?? 2000} onChange={(e) => set("TxQueueLen", Number(e.target.value))} />
              </div>
            </div>
          </Section>

          {/* ── Features ── */}
          <Section title="Features" defaultOpen={true}>
            <div className="flex flex-wrap gap-1.5">
              {featureToggles.map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                    form[opt.key]
                      ? opt.key === "DisableFirewall" || opt.key === "KillSwitch"
                        ? "border-amber-500/40 bg-amber-500/15 text-amber-400 shadow-[0_0_12px_rgba(245,158,11,0.12)]"
                        : "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                      : "border-white/[0.06] bg-white/[0.02] text-white/30 hover:text-white/50 hover:border-white/15 hover:bg-white/[0.04]"
                  }`}
                  onClick={() => set(opt.key, !form[opt.key])}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </Section>

          {/* ── DNS ── */}
          <Section title={`DNS Servers (${(form.DNSServers || []).length})`} defaultOpen={false}>
            <StringArrayField items={form.DNSServers || []} onChange={(v) => set("DNSServers", v)} />
          </Section>

          {/* ── Firewall: Allowed Hosts ── */}
          <Section title={`Firewall: Allowed Hosts (${(form.AllowedHosts || []).length})`} defaultOpen={false}>
            <StringArrayField items={form.AllowedHosts || []} onChange={(v) => set("AllowedHosts", v)} />
          </Section>

          {/* ── DNS Records ── */}
          <Section title={`DNS Records (${(form.DNSRecords || []).length})`} defaultOpen={false}>
            {(form.DNSRecords || []).map((rec, i) => (
              <DNSRecordEditor
                key={i}
                record={rec}
                onChange={(val) => setArrayItem("DNSRecords", i, val)}
                onRemove={() => removeArrayItem("DNSRecords", i)}
              />
            ))}
            <button
              type="button"
              onClick={() => addArrayItem("DNSRecords", { Domain: "", Wildcard: false, IP: [], TXT: [] })}
              className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-2"
            >
              <Plus className="w-3 h-3" /> Add DNS Record
            </button>
          </Section>

          {/* ── Networks ── */}
          <Section title={`Networks (${(form.Networks || []).length})`} defaultOpen={false}>
            {(form.Networks || []).map((net, i) => (
              <NetworkEditor
                key={i}
                net={net}
                onChange={(val) => setArrayItem("Networks", i, val)}
                onRemove={() => removeArrayItem("Networks", i)}
              />
            ))}
            <button
              type="button"
              onClick={() => addArrayItem("Networks", { Tag: "", Network: "", Nat: "" })}
              className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-2"
            >
              <Plus className="w-3 h-3" /> Add Network
            </button>
          </Section>

          {/* ── Routes ── */}
          <Section title={`Routes (${(form.Routes || []).length})`} defaultOpen={false}>
            {(form.Routes || []).map((route, i) => (
              <RouteEditor
                key={i}
                route={route}
                onChange={(val) => setArrayItem("Routes", i, val)}
                onRemove={() => removeArrayItem("Routes", i)}
              />
            ))}
            <button
              type="button"
              onClick={() => addArrayItem("Routes", { Address: "", Metric: "" })}
              className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-2"
            >
              <Plus className="w-3 h-3" /> Add Route
            </button>
          </Section>

          {/* ── System (read-only) ── */}
          {(form.WindowsGUID || form.ConfigFormat) && (
            <Section title="System" defaultOpen={false}>
              <div className="space-y-px">
                {form.WindowsGUID && (
                  <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                    <span className="text-[11px] text-white/25 shrink-0 w-[100px]">Windows GUID</span>
                    <code className="text-[12px] text-white/50 font-mono truncate">{form.WindowsGUID}</code>
                  </div>
                )}
                {form.ConfigFormat && (
                  <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-white/[0.06]">
                    <span className="text-[11px] text-white/25 shrink-0 w-[100px]">Config Format</span>
                    <code className="text-[12px] text-white/50 font-mono">{form.ConfigFormat}</code>
                  </div>
                )}
              </div>
            </Section>
          )}
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
            className="text-[11px] text-white/30 hover:text-white/50 px-2"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default TunnelFormDialog;
