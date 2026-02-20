import React, { useEffect, useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Save, Plus, Pencil, Trash2, Settings, Minus } from "lucide-react";
import GLOBAL_STATE from "../state";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

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

  const options = [
    { key: "DNSOverHTTPS", label: "Secure DNS", checked: state?.Config?.DNSOverHTTPS },
    { key: "LogBlockedDomains", label: "Log Blocked", checked: state?.Config?.LogBlockedDomains },
    { key: "LogAllDomains", label: "Log All", checked: state?.Config?.LogAllDomains },
    { key: "DNSstats", label: "Stats", checked: state?.Config?.DNSstats },
  ];

  return (
    <div>

      {/* ── Config banner ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-6">
        {!editing ? (
          <>
            <div>
              <span className="text-[9px] text-white/35 uppercase tracking-widest block mb-0.5">Server</span>
              <code className="text-[13px] text-white/80 font-mono">
                {cfg.DNSServerIP || "0.0.0.0"}:{cfg.DNSServerPort || "53"}
              </code>
            </div>
            <div className="w-px h-8 bg-white/[0.06]" />
            <div>
              <span className="text-[9px] text-white/35 uppercase tracking-widest block mb-0.5">Resolvers</span>
              <div className="flex items-center gap-2">
                <code className="text-[13px] text-white/80 font-mono">{cfg.DNS1Default || "—"}</code>
                <span className="text-[11px] text-white/30">|</span>
                <code className="text-[13px] text-white/80 font-mono">{cfg.DNS2Default || "—"}</code>
              </div>
            </div>
            <button
              className="ml-auto p-1.5 rounded text-white/40 hover:text-white/60 hover:bg-white/[0.04] transition-colors"
              onClick={() => setEditing(true)}
            >
              <Settings className="h-3.5 w-3.5" />
            </button>
          </>
        ) : (
          <div className="flex-1">
            <div className="grid grid-cols-4 gap-3">
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Server IP</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.DNSServerIP || ""} onChange={(e) => updatecfg("DNSServerIP", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Port</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.DNSServerPort || ""} onChange={(e) => updatecfg("DNSServerPort", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Primary DNS</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.DNS1Default || ""} onChange={(e) => updatecfg("DNS1Default", e.target.value)} />
              </div>
              <div>
                <label className="text-[10px] text-white/50 uppercase block mb-1">Backup DNS</label>
                <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={cfg.DNS2Default || ""} onChange={(e) => updatecfg("DNS2Default", e.target.value)} />
              </div>
            </div>
            <div className="flex gap-2 mt-2">
              {mod && (
                <Button
                  className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5"
                  onClick={async () => {
                    state.Config = cfg;
                    let ok = await state.v2_ConfigSave();
                    if (ok) { setMod(false); setEditing(false); }
                  }}
                >
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
              )}
              <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => { setCfg({ ...state.Config }); setMod(false); setEditing(false); }}>
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>

      {/* ── Option pills ── */}
      <div className="flex items-center gap-2 mb-8">
        {options.map((opt) => (
          <button
            key={opt.key}
            className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
              opt.checked
                ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
            }`}
            onClick={() => { state.toggleConfigKeyAndSave("Config", opt.key); state.rerender(); }}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* ── Sections ── */}
      <div className="flex flex-wrap gap-8 mb-8">
        {/* Records */}
        <div className="min-w-[280px] flex-1">
          <div className="flex items-center justify-between mb-3">
            <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider">Records</span>
            <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors" onClick={() => openRecord(null, false)}>
              <Plus className="h-3 w-3" /> New
            </button>
          </div>
          <div className="space-y-1">
            {records.length > 0 ? records.map((r, i) => (
              <div key={i} className="group flex items-center gap-3 py-1.5 pl-3 border-l-2 border-cyan-500/20 hover:border-cyan-500/50 transition-colors">
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] text-white/80 font-medium truncate">{r.Domain}</div>
                  <div className="text-[11px] text-white/45 font-mono truncate">
                    {r.IP?.join(", ")}{r.Wildcard ? " *" : ""}
                  </div>
                </div>
                <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button className="p-1 text-white/40 hover:text-white/60" onClick={() => openRecord(r, true)}><Pencil className="h-3 w-3" /></button>
                  <button className="p-1 text-red-500/25 hover:text-red-400" onClick={() => deleteRecord(r)}><Trash2 className="h-3 w-3" /></button>
                </div>
              </div>
            )) : (
              <div className="py-4 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">No records</div>
            )}
          </div>
        </div>

        {/* Block Lists */}
        <div className="min-w-[280px] flex-1">
          <div className="flex items-center justify-between mb-3">
            <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider">Block Lists</span>
            <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors" onClick={() => openBlocklist(null, false)}>
              <Plus className="h-3 w-3" /> New
            </button>
          </div>
          <div className="space-y-1">
            {blockLists.length > 0 ? blockLists.map((bl, i) => (
              <div key={i} className="group flex items-center gap-3 py-1.5 pl-3 border-l-2 border-amber-500/20 hover:border-amber-500/50 transition-colors">
                <button
                  className={`text-[10px] px-2 py-0.5 rounded-full border transition-all cursor-pointer shrink-0 ${
                    bl.Enabled
                      ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                      : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
                  }`}
                  onClick={(e) => { e.stopPropagation(); state.toggleBlocklist(bl); state.v2_ConfigSave(); }}
                >{bl.Enabled ? "On" : "Off"}</button>
                <div className="flex-1 min-w-0">
                  <span className="text-[13px] text-white/80 font-medium">{bl.Tag}</span>
                  <span className="text-[11px] text-white/40 ml-2">{bl.Count}</span>
                </div>
                <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button className="p-1 text-white/40 hover:text-white/60" onClick={() => openBlocklist(bl, true)}><Pencil className="h-3 w-3" /></button>
                  <button className="p-1 text-red-500/25 hover:text-red-400" onClick={() => state.deleteBlocklist(bl)}><Trash2 className="h-3 w-3" /></button>
                </div>
              </div>
            )) : (
              <div className="py-4 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">No block lists</div>
            )}
          </div>
        </div>

        {/* White Lists */}
        <div className="min-w-[280px] flex-1">
          <div className="flex items-center justify-between mb-3">
            <span className="text-[11px] text-white/50 font-medium uppercase tracking-wider">White Lists</span>
            <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors" onClick={() => openWhitelist(null, false)}>
              <Plus className="h-3 w-3" /> New
            </button>
          </div>
          <div className="space-y-1">
            {whiteLists.length > 0 ? whiteLists.map((wl, i) => (
              <div key={i} className="group flex items-center gap-3 py-1.5 pl-3 border-l-2 border-emerald-500/20 hover:border-emerald-500/50 transition-colors">
                <button
                  className={`text-[10px] px-2 py-0.5 rounded-full border transition-all cursor-pointer shrink-0 ${
                    wl.Enabled
                      ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                      : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
                  }`}
                  onClick={(e) => { e.stopPropagation(); state.toggleWhitelist(wl); state.v2_ConfigSave(); }}
                >{wl.Enabled ? "On" : "Off"}</button>
                <div className="flex-1 min-w-0">
                  <span className="text-[13px] text-white/80 font-medium">{wl.Tag}</span>
                  <span className="text-[11px] text-white/40 ml-2">{wl.Count}</span>
                </div>
                <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button className="p-1 text-white/40 hover:text-white/60" onClick={() => openWhitelist(wl, true)}><Pencil className="h-3 w-3" /></button>
                  <button className="p-1 text-red-500/25 hover:text-red-400" onClick={() => state.deleteWhitelist(wl)}><Trash2 className="h-3 w-3" /></button>
                </div>
              </div>
            )) : (
              <div className="py-4 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">No white lists</div>
            )}
          </div>
        </div>
      </div>

      {/* ── DNS Record dialog ── */}
      <Dialog open={recordModal} onOpenChange={setRecordModal}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {record && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">
                  {isRecordEdit ? "Edit DNS Record" : "New DNS Record"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div className="flex items-center gap-3">
                  <div className="flex-1">
                    <label className="text-[10px] text-white/50 uppercase block mb-1">Domain</label>
                    <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={record.Domain || ""} onChange={(e) => { record.Domain = e.target.value; setRecord({ ...record }); }} />
                  </div>
                  <div className="pt-4">
                    <button
                      className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                        record.Wildcard
                          ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                          : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
                      }`}
                      onClick={() => { record.Wildcard = !record.Wildcard; setRecord({ ...record }); }}
                    >
                      Wildcard
                    </button>
                  </div>
                </div>

                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">IP Addresses</label>
                  <div className="space-y-1">
                    {(record.IP || []).map((ip, i) => (
                      <div key={i} className="flex items-center gap-1">
                        <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" value={ip} onChange={(e) => { record.IP[i] = e.target.value; setRecord({ ...record }); }} />
                        <button className="p-1 text-red-400/60 hover:text-red-400" onClick={() => { record.IP.splice(i, 1); setRecord({ ...record }); }}>
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    ))}
                    <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-1" onClick={() => { record.IP = [...(record.IP || []), ""]; setRecord({ ...record }); }}>
                      <Plus className="w-3 h-3" /> Add IP
                    </button>
                  </div>
                </div>

                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">TXT Records</label>
                  <div className="space-y-1">
                    {(record.TXT || []).map((txt, i) => (
                      <div key={i} className="flex items-center gap-1">
                        <Input className="flex-1 h-7 text-[12px] border-[#1e2433] bg-transparent" value={txt} onChange={(e) => { record.TXT[i] = e.target.value; setRecord({ ...record }); }} />
                        <button className="p-1 text-red-400/60 hover:text-red-400" onClick={() => { record.TXT.splice(i, 1); setRecord({ ...record }); }}>
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    ))}
                    <button className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 mt-1" onClick={() => { record.TXT = [...(record.TXT || []), ""]; setRecord({ ...record }); }}>
                      <Plus className="w-3 h-3" /> Add TXT
                    </button>
                  </div>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={async () => {
                  if (!isRecordEdit) { if (!state.Config?.DNSRecords) state.Config.DNSRecords = []; state.Config.DNSRecords.push(record); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setRecordModal(false); setIsRecordEdit(false); }
                }}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => setRecordModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Blocklist dialog ── */}
      <Dialog open={blocklistModal} onOpenChange={setBlocklistModal}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {blocklist && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">
                  {isBlocklistEdit ? "Edit Block List" : "New Block List"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">Tag</label>
                  <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={blocklist.Tag || ""} onChange={(e) => { blocklist.Tag = e.target.value; setBlocklist({ ...blocklist }); }} />
                </div>
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">URL</label>
                  <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={blocklist.URL || ""} onChange={(e) => { blocklist.URL = e.target.value; setBlocklist({ ...blocklist }); }} />
                </div>
                <div className="flex items-center gap-2">
                  <button
                    className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                      blocklist.Enabled
                        ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                        : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
                    }`}
                    onClick={() => { blocklist.Enabled = !blocklist.Enabled; setBlocklist({ ...blocklist }); }}
                  >
                    Enabled
                  </button>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={async () => {
                  if (!isBlocklistEdit) { if (!state.Config?.DNSBlockLists) state.Config.DNSBlockLists = []; state.Config.DNSBlockLists.push(blocklist); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setBlocklistModal(false); setIsBlocklistEdit(false); }
                }}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => setBlocklistModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Whitelist dialog ── */}
      <Dialog open={whitelistModal} onOpenChange={setWhitelistModal}>
        <DialogContent className="sm:max-w-[480px] text-white bg-[#0a0d14] border-[#1e2433]">
          {whitelist && (
            <>
              <DialogHeader>
                <DialogTitle className="text-lg font-bold text-white">
                  {isWhitelistEdit ? "Edit White List" : "New White List"}
                </DialogTitle>
              </DialogHeader>

              <div className="space-y-3">
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">Tag</label>
                  <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={whitelist.Tag || ""} onChange={(e) => { whitelist.Tag = e.target.value; setWhitelist({ ...whitelist }); }} />
                </div>
                <div>
                  <label className="text-[10px] text-white/50 uppercase block mb-1">URL</label>
                  <Input className="h-7 text-[12px] border-[#1e2433] bg-transparent" value={whitelist.URL || ""} onChange={(e) => { whitelist.URL = e.target.value; setWhitelist({ ...whitelist }); }} />
                </div>
                <div className="flex items-center gap-2">
                  <button
                    className={`text-[11px] px-3 py-1 rounded-full border transition-all cursor-pointer ${
                      whitelist.Enabled
                        ? "border-emerald-500/40 bg-emerald-500/15 text-emerald-400 shadow-[0_0_12px_rgba(16,185,129,0.12)]"
                        : "border-white/[0.06] bg-white/[0.02] text-white/50 hover:text-white/70 hover:border-white/25 hover:bg-white/[0.04]"
                    }`}
                    onClick={() => { whitelist.Enabled = !whitelist.Enabled; setWhitelist({ ...whitelist }); }}
                  >
                    Enabled
                  </button>
                </div>
              </div>

              <DialogFooter className="flex gap-2 mt-2">
                <Button className="text-white bg-emerald-600 hover:bg-emerald-500 h-6 text-[11px] px-2.5" onClick={async () => {
                  if (!isWhitelistEdit) { if (!state.Config?.DNSWhiteLists) state.Config.DNSWhiteLists = []; state.Config.DNSWhiteLists.push(whitelist); }
                  let ok = await state.v2_ConfigSave();
                  if (ok) { setWhitelistModal(false); setIsWhitelistEdit(false); }
                }}>
                  <Save className="h-3 w-3 mr-1" /> Save
                </Button>
                <button className="text-[11px] text-white/50 hover:text-white/70 px-2" onClick={() => setWhitelistModal(false)}>Cancel</button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default DNS;
