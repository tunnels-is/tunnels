import React, { useEffect, useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Save, Plus, Pencil, Trash2, Minus, X } from "lucide-react";
import GLOBAL_STATE from "../state";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

/* ─── Reusable building blocks ─────────────────────────────────────── */

const SettingsCard = ({ title, description, actions, children, className = "" }) => (
  <section
    className={
      "rounded-lg bg-white border border-[#e7e3d7] card-shadow p-5 " + className
    }
  >
    <header className="flex items-start justify-between gap-3 mb-4">
      <div>
        <h3 className="text-[13px] font-semibold tracking-tight text-[#0a0a0a]">{title}</h3>
        {description && (
          <p className="mt-1 text-[11px] text-[#737373] leading-relaxed">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </header>
    {children}
  </section>
);

const TogglePill = ({ checked, label, onClick, tone = "active" }) => {
  const activeClass = {
    active: "pill pill-active",
    warning: "pill pill-active-warning",
    danger: "pill pill-active-danger",
    success: "pill pill-active-success",
  }[tone] || "pill pill-active";
  return (
    <button onClick={onClick} className={checked ? activeClass : "pill"}>
      {label}
    </button>
  );
};

const ListRow = ({ enabled, onToggle, name, meta, onEdit, onDelete, last }) => (
  <div
    className={
      "group flex items-center gap-3 px-3 py-2 " +
      (last ? "" : "border-b border-[#e7e3d7]/70")
    }
  >
    <button
      onClick={(e) => { e.stopPropagation(); onToggle?.(); }}
      className={(enabled ? "pill pill-active-success" : "pill") + " pill-sm shrink-0 min-w-[44px]"}
      title={enabled ? "Click to disable" : "Click to enable"}
    >
      {enabled ? "ON" : "OFF"}
    </button>
    <div className="flex-1 min-w-0">
      <div className="text-[13px] font-medium tracking-tight text-[#0a0a0a] truncate">{name}</div>
      {meta && <div className="text-[11px] text-[#a3a3a3] font-mono truncate">{meta}</div>}
    </div>
    <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
      {onEdit && <button className="btn-icon" onClick={onEdit}><Pencil className="h-3 w-3" /></button>}
      {onDelete && <button className="btn-icon btn-icon-danger" onClick={onDelete}><Trash2 className="h-3 w-3" /></button>}
    </div>
  </div>
);

const EmptyRow = ({ children }) => (
  <div className="px-3 py-5 text-center text-[11px] italic text-[#a3a3a3]">{children}</div>
);

/* ─── DNS page ─────────────────────────────────────────────────────── */

const DNS = () => {
  const state = GLOBAL_STATE("dns");
  const [editing, setEditing] = useState(false);
  const [record, setRecord] = useState(undefined);
  const [recordModal, setRecordModal] = useState(false);
  const [isRecordEdit, setIsRecordEdit] = useState(false);
  const [blocklist, setBlocklist] = useState(undefined);
  const [blocklistModal, setBlocklistModal] = useState(false);
  const [isBlocklistEdit, setIsBlocklistEdit] = useState(false);
  const [whitelist, setWhitelist] = useState(undefined);
  const [whitelistModal, setWhitelistModal] = useState(false);
  const [isWhitelistEdit, setIsWhitelistEdit] = useState(false);
  const [cfg, setCfg] = useState({ ...state.Config });
  const [mod, setMod] = useState(false);

  const updatecfg = (key, value) => {
    setCfg((prev) => ({ ...prev, [key]: value }));
    setMod(true);
  };

  useEffect(() => {
    state.GetBackendState();
  }, []);

  const saveServer = async () => {
    state.Config = cfg;
    let ok = await state.v2_ConfigSave();
    if (ok) { setMod(false); setEditing(false); }
  };
  const cancelServer = () => {
    setCfg({ ...state.Config });
    setMod(false);
    setEditing(false);
  };

  let blockLists = state.Config?.DNSBlockLists;
  state.modifiedLists?.forEach((l) => {
    blockLists?.forEach((ll, i) => {
      if (ll.Tag === l.Tag) blockLists[i] = l;
    });
  });
  if (!blockLists) blockLists = [];

  let whiteLists = state.Config?.DNSWhiteLists;
  state.modifiedLists?.forEach((l) => {
    whiteLists?.forEach((ll, i) => {
      if (ll.Tag === l.Tag) whiteLists[i] = l;
    });
  });
  if (!whiteLists) whiteLists = [];

  const records = state.Config?.DNSRecords || [];

  const openRecord = (obj, edit) => {
    setIsRecordEdit(edit);
    setRecord(edit ? obj : { Domain: "yourdomain.com", IP: ["127.0.0.1"], TXT: ["yourdomain.com text record"], Wildcard: true });
    setRecordModal(true);
  };
  const deleteRecord = (obj) => {
    state.Config.DNSRecords = state.Config.DNSRecords.filter((r) => r.Domain !== obj.Domain);
    state.v2_ConfigSave();
  };
  const openBlocklist = (obj, edit) => {
    setIsBlocklistEdit(edit);
    setBlocklist(edit ? obj : { Tag: "new-blocklist", URL: "https://example.com/blocklist.txt", Enabled: true, Count: 0 });
    setBlocklistModal(true);
  };
  const openWhitelist = (obj, edit) => {
    setIsWhitelistEdit(edit);
    setWhitelist(edit ? obj : { Tag: "new-whitelist", URL: "https://example.com/whitelist.txt", Enabled: true, Count: 0 });
    setWhitelistModal(true);
  };

  const toggleOpt = (key) => {
    state.toggleConfigKeyAndSave("Config", key);
    state.rerender();
  };

  const options = [
    { key: "DNSOverHTTPS",       label: "Secure DNS",         tone: "active" },
    { key: "LogBlockedDomains",  label: "Log Blocked",        tone: "active" },
    { key: "LogAllDomains",      label: "Log All",            tone: "warning" },
    { key: "DNSstats",           label: "Stats",              tone: "active" },
    { key: "DNSHTTPSAutomatic",  label: "Dynamic Encryption", tone: "active" },
  ];

  return (
    <div className="max-w-5xl">

      {/* Page header */}
      <header className="mb-6 flex items-baseline justify-between gap-4">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight text-[#0a0a0a]">DNS</h1>
          <p className="mt-1 text-[12px] text-[#737373]">
            Local DNS resolver — records, block lists and upstream servers.
          </p>
        </div>
        <div className="flex items-center gap-2 text-[11px]">
          <span className={"tag tag-sm " + (records.length > 0 ? "tag-success" : "tag-muted")}>
            <span className="uppercase tracking-[0.08em] text-[9px] font-semibold">Records</span>
            <span className="font-mono tabular-nums">{records.length}</span>
          </span>
          <span className={"tag tag-sm " + (blockLists.length > 0 ? "tag-danger" : "tag-muted")}>
            <span className="uppercase tracking-[0.08em] text-[9px] font-semibold">Block</span>
            <span className="font-mono tabular-nums">{blockLists.length}</span>
          </span>
          <span className={"tag tag-sm " + (whiteLists.length > 0 ? "tag-success" : "tag-muted")}>
            <span className="uppercase tracking-[0.08em] text-[9px] font-semibold">White</span>
            <span className="font-mono tabular-nums">{whiteLists.length}</span>
          </span>
        </div>
      </header>

      {/* Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">

        {/* ── DNS server ── */}
        <SettingsCard
          className="lg:col-span-2"
          tone="info"
          title="DNS server"
          description="Address the resolver listens on and upstream fallback resolvers."
          actions={
            editing ? (
              <>
                {mod && (
                  <Button onClick={saveServer} className="btn btn-primary btn-xs">
                    <Save className="h-3 w-3" /> Save
                  </Button>
                )}
                <button onClick={cancelServer} className="btn btn-ghost btn-xs">
                  <X className="h-3 w-3" /> Cancel
                </button>
              </>
            ) : (
              <button onClick={() => setEditing(true)} className="btn btn-outline btn-xs">
                <Pencil className="h-3 w-3" /> Edit
              </button>
            )
          }
        >
          {!editing ? (
            <div className="flex flex-wrap items-start gap-x-8 gap-y-3">
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  Listening on
                </span>
                <code className="text-[13px] text-[#0a0a0a] font-mono">
                  {cfg.DNSServerIP || "0.0.0.0"}<span className="text-[#a3a3a3]">:</span>{cfg.DNSServerPort || "53"}
                </code>
              </div>
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  Primary resolver
                </span>
                <code className="text-[13px] text-[#0a0a0a] font-mono">
                  {cfg.DNS1Default || <span className="font-sans italic text-[#a3a3a3]">none</span>}
                </code>
              </div>
              <div>
                <span className="block text-[10px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] mb-1">
                  Backup resolver
                </span>
                <code className="text-[13px] text-[#0a0a0a] font-mono">
                  {cfg.DNS2Default || <span className="font-sans italic text-[#a3a3a3]">none</span>}
                </code>
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
              <div>
                <label className="label">Server IP</label>
                <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.DNSServerIP || ""} onChange={(e) => updatecfg("DNSServerIP", e.target.value)} />
              </div>
              <div>
                <label className="label">Port</label>
                <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.DNSServerPort || ""} onChange={(e) => updatecfg("DNSServerPort", e.target.value)} />
              </div>
              <div>
                <label className="label">Primary DNS</label>
                <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.DNS1Default || ""} onChange={(e) => updatecfg("DNS1Default", e.target.value)} />
              </div>
              <div>
                <label className="label">Backup DNS</label>
                <Input className="h-8 text-[12px] border-[#e7e3d7] bg-white" value={cfg.DNS2Default || ""} onChange={(e) => updatecfg("DNS2Default", e.target.value)} />
              </div>
            </div>
          )}
        </SettingsCard>

        {/* ── Behaviour ── */}
        <SettingsCard
          className="lg:col-span-2"
          tone="warning"
          title="Behaviour"
          description="Encryption, logging and statistics for the resolver."
        >
          <div className="flex flex-wrap items-center gap-2">
            {options.map((opt) => (
              <TogglePill
                key={opt.key}
                checked={!!state?.Config?.[opt.key]}
                tone={opt.tone}
                label={opt.label}
                onClick={() => toggleOpt(opt.key)}
              />
            ))}
          </div>
        </SettingsCard>

        {/* ── Records ── */}
        <SettingsCard
          tone="success"
          title="Records"
          description="Locally resolved A and TXT records."
          actions={
            <button className="btn btn-primary btn-xs" onClick={() => openRecord(null, false)}>
              Create
            </button>
          }
        >
          <div className="-mx-2">
            {records.length > 0 ? records.map((r, i) => (
              <ListRow
                key={i}
                enabled
                name={r.Domain + (r.Wildcard ? " *" : "")}
                meta={r.IP?.join(", ")}
                onEdit={() => openRecord(r, true)}
                onDelete={() => deleteRecord(r)}
                last={i === records.length - 1}
              />
            )) : <EmptyRow>No records configured</EmptyRow>}
          </div>
        </SettingsCard>

        {/* ── Block Lists ── */}
        <SettingsCard
          tone="danger"
          title="Block lists"
          description="External lists of domains that will be blocked."
          actions={
            <button className="btn btn-primary btn-xs" onClick={() => openBlocklist(null, false)}>
              Create
            </button>
          }
        >
          <div className="-mx-2">
            {blockLists.length > 0 ? blockLists.map((bl, i) => (
              <ListRow
                key={i}
                enabled={bl.Enabled}
                onToggle={() => { state.toggleBlocklist(bl); state.v2_ConfigSave(); }}
                name={bl.Tag}
                meta={`${bl.Count?.toLocaleString?.() ?? bl.Count} domains`}
                onEdit={() => openBlocklist(bl, true)}
                onDelete={() => state.deleteBlocklist(bl)}
                last={i === blockLists.length - 1}
              />
            )) : <EmptyRow>No block lists configured</EmptyRow>}
          </div>
        </SettingsCard>

        {/* ── White Lists ── */}
        <SettingsCard
          className="lg:col-span-2"
          tone="success"
          title="White lists"
          description="Domains here always resolve, even if they appear on a block list."
          actions={
            <button className="btn btn-primary btn-xs" onClick={() => openWhitelist(null, false)}>
              Create
            </button>
          }
        >
          <div className="-mx-2">
            {whiteLists.length > 0 ? whiteLists.map((wl, i) => (
              <ListRow
                key={i}
                enabled={wl.Enabled}
                onToggle={() => { state.toggleWhitelist(wl); state.v2_ConfigSave(); }}
                name={wl.Tag}
                meta={`${wl.Count?.toLocaleString?.() ?? wl.Count} domains`}
                onEdit={() => openWhitelist(wl, true)}
                onDelete={() => state.deleteWhitelist(wl)}
                last={i === whiteLists.length - 1}
              />
            )) : <EmptyRow>No white lists configured</EmptyRow>}
          </div>
        </SettingsCard>

      </div>

      {/* ── DNS Record dialog ── */}
      <Dialog open={recordModal} onOpenChange={setRecordModal}>
        <DialogContent className="sm:max-w-[480px] text-[#0a0a0a] bg-white border-[#e7e3d7]">
          {record && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-semibold tracking-tight text-[#0a0a0a]">
                  {isRecordEdit ? "Edit DNS record" : "New DNS record"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div className="flex items-end gap-3">
                  <div className="flex-1">
                    <label className="label">Domain</label>
                    <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={record.Domain || ""} onChange={(e) => { record.Domain = e.target.value; setRecord({ ...record }); }} />
                  </div>
                  <TogglePill
                    checked={record.Wildcard}
                    label="Wildcard"
                    onClick={() => { record.Wildcard = !record.Wildcard; setRecord({ ...record }); }}
                  />
                </div>

                <div>
                  <label className="label">IP addresses</label>
                  <div className="space-y-1">
                    {(record.IP || []).map((ip, i) => (
                      <div key={i} className="flex items-center gap-1">
                        <Input className="flex-1 h-8 text-[12px] border-[#e7e3d7] bg-white" value={ip} onChange={(e) => { record.IP[i] = e.target.value; setRecord({ ...record }); }} />
                        <button className="btn-icon btn-icon-danger" onClick={() => { record.IP.splice(i, 1); setRecord({ ...record }); }}>
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    ))}
                    <button className="btn btn-ghost-success btn-xs" onClick={() => { record.IP = [...(record.IP || []), ""]; setRecord({ ...record }); }}>
                      <Plus className="w-3 h-3" /> Add IP
                    </button>
                  </div>
                </div>

                <div>
                  <label className="label">TXT records</label>
                  <div className="space-y-1">
                    {(record.TXT || []).map((txt, i) => (
                      <div key={i} className="flex items-center gap-1">
                        <Input className="flex-1 h-8 text-[12px] border-[#e7e3d7] bg-white" value={txt} onChange={(e) => { record.TXT[i] = e.target.value; setRecord({ ...record }); }} />
                        <button className="btn-icon btn-icon-danger" onClick={() => { record.TXT.splice(i, 1); setRecord({ ...record }); }}>
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    ))}
                    <button className="btn btn-ghost-success btn-xs" onClick={() => { record.TXT = [...(record.TXT || []), ""]; setRecord({ ...record }); }}>
                      <Plus className="w-3 h-3" /> Add TXT
                    </button>
                  </div>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="btn btn-primary btn-sm" onClick={async () => {
                  if (!isRecordEdit) { if (!state.Config?.DNSRecords) state.Config.DNSRecords = []; state.Config.DNSRecords.push(record); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setRecordModal(false); setIsRecordEdit(false); }
                }}>
                  <Save className="h-3 w-3" /> Save
                </Button>
                <button className="btn btn-ghost btn-sm" onClick={() => setRecordModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Blocklist dialog ── */}
      <Dialog open={blocklistModal} onOpenChange={setBlocklistModal}>
        <DialogContent className="sm:max-w-[480px] text-[#0a0a0a] bg-white border-[#e7e3d7]">
          {blocklist && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-semibold tracking-tight text-[#0a0a0a]">
                  {isBlocklistEdit ? "Edit block list" : "New block list"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div>
                  <label className="label">Tag</label>
                  <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={blocklist.Tag || ""} onChange={(e) => { blocklist.Tag = e.target.value; setBlocklist({ ...blocklist }); }} />
                </div>
                <div>
                  <label className="label">URL</label>
                  <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={blocklist.URL || ""} onChange={(e) => { blocklist.URL = e.target.value; setBlocklist({ ...blocklist }); }} />
                </div>
                <div>
                  <TogglePill
                    checked={blocklist.Enabled}
                    label="Enabled"
                    tone="success"
                    onClick={() => { blocklist.Enabled = !blocklist.Enabled; setBlocklist({ ...blocklist }); }}
                  />
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="btn btn-primary btn-sm" onClick={async () => {
                  if (!isBlocklistEdit) { if (!state.Config?.DNSBlockLists) state.Config.DNSBlockLists = []; state.Config.DNSBlockLists.push(blocklist); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setBlocklistModal(false); setIsBlocklistEdit(false); }
                }}>
                  <Save className="h-3 w-3" /> Save
                </Button>
                <button className="btn btn-ghost btn-sm" onClick={() => setBlocklistModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Whitelist dialog ── */}
      <Dialog open={whitelistModal} onOpenChange={setWhitelistModal}>
        <DialogContent className="sm:max-w-[480px] text-[#0a0a0a] bg-white border-[#e7e3d7]">
          {whitelist && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-semibold tracking-tight text-[#0a0a0a]">
                  {isWhitelistEdit ? "Edit white list" : "New white list"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div>
                  <label className="label">Tag</label>
                  <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={whitelist.Tag || ""} onChange={(e) => { whitelist.Tag = e.target.value; setWhitelist({ ...whitelist }); }} />
                </div>
                <div>
                  <label className="label">URL</label>
                  <Input className="h-9 text-[13px] border-[#e7e3d7] bg-white" value={whitelist.URL || ""} onChange={(e) => { whitelist.URL = e.target.value; setWhitelist({ ...whitelist }); }} />
                </div>
                <div>
                  <TogglePill
                    checked={whitelist.Enabled}
                    label="Enabled"
                    tone="success"
                    onClick={() => { whitelist.Enabled = !whitelist.Enabled; setWhitelist({ ...whitelist }); }}
                  />
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="btn btn-primary btn-sm" onClick={async () => {
                  if (!isWhitelistEdit) { if (!state.Config?.DNSWhiteLists) state.Config.DNSWhiteLists = []; state.Config.DNSWhiteLists.push(whitelist); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setWhitelistModal(false); setIsWhitelistEdit(false); }
                }}>
                  <Save className="h-3 w-3" /> Save
                </Button>
                <button className="btn btn-ghost btn-sm" onClick={() => setWhitelistModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

    </div>
  );
};

export default DNS;
