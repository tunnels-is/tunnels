import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import { ArrowLeft, Pencil, Save, X, Link } from 'lucide-react';
import { apiPost, apiGet } from '../api';
import { Copy } from 'lucide-react';

const inputClass = "w-full bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50";

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#1e2433]/50">
      <span className="text-[11px] text-white/40 uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-white/80 min-w-0">{children}</div>
    </div>
  );
}

const EMPTY_ID = '00000000-0000-0000-0000-000000000000';

export default function WGConfigDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [config, setConfig] = useState(location.state?.config || null);
  const [networks, setNetworks] = useState(location.state?.networks || []);
  const [servers, setServers] = useState([]);

  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  const [assignForm, setAssignForm] = useState({ ServerID: '' });
  const [assigning, setAssigning] = useState(false);
  const [assignResult, setAssignResult] = useState(null);
  const [assignError, setAssignError] = useState('');

  const load = async () => {
    const [cfgResp, srvResp, netResp] = await Promise.all([
      apiGet(`/ui/wg/server-config/get?id=${id}`),
      apiPost('/ui/servers', { StartIndex: 0 }),
      networks.length === 0 ? apiPost('/ui/network/list', { Limit: 50000, Offset: 0 }) : Promise.resolve(null),
    ]);
    if (cfgResp.status === 200) {
      const data = await cfgResp.json();
      // GET returns ID (not _id); normalize so the rest of the component works
      if (data.ID) data._id = data.ID;
      setConfig(data);
    }
    if (srvResp.status === 200) {
      const data = await srvResp.json();
      setServers(Array.isArray(data) ? data : []);
    }
    if (netResp && netResp.status === 200) {
      const data = await netResp.json();
      const list = Array.isArray(data.Networks) ? data.Networks : Array.isArray(data) ? data : [];
      setNetworks(list);
    }
  };

  useEffect(() => { load(); }, [id]);

  const networkForID = (nid) => {
    if (!nid || nid === EMPTY_ID) return null;
    return networks.find((n) => n._id === nid) || null;
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text).catch(() => {});
  };

  const startEdit = () => {
    setForm({
      Tag: config.Tag || '',
      WireGuardPort: config.WireGuardPort || 51820,
      NetworkID: config.NetworkID && config.NetworkID !== EMPTY_ID ? config.NetworkID : '',
      WireGuardIface: config.WireGuardIface || '',
      InternetIface: config.InternetIface || '',
      PacketInspection: config.PacketInspection || false,
      InsecureSkipVerify: config.InsecureSkipVerify || false,
    });
    setSaveError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setSaveError('');
    try {
      const resp = await apiPost('/ui/wg/server-config/update', {
        ID: id,
        Tag: form.Tag,
        WireGuardPort: Number(form.WireGuardPort),
        NetworkID: form.NetworkID || EMPTY_ID,
        WireGuardIface: form.WireGuardIface,
        InternetIface: form.InternetIface,
        PacketInspection: form.PacketInspection,
        InsecureSkipVerify: form.InsecureSkipVerify,
      });
      if (resp.status === 200) {
        setEditing(false);
        await load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setSaveError(data.Error || 'Failed to save');
      }
    } catch (err) {
      setSaveError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const set = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked
      : e.target.type === 'number' ? Number(e.target.value)
      : e.target.value;
    setForm((f) => ({ ...f, [k]: val }));
  };

  const handleAssign = async (e) => {
    e.preventDefault();
    setAssignError('');
    setAssignResult(null);
    setAssigning(true);
    try {
      const resp = await apiPost('/ui/wg/server-config/assign', {
        ServerID: assignForm.ServerID,
        ConfigID: id,
      });
      if (resp.status === 200) {
        const data = await resp.json();
        setAssignResult(data);
      } else {
        const data = await resp.json().catch(() => ({}));
        setAssignError(data.Error || 'Failed to assign');
      }
    } catch (err) {
      setAssignError(err.message);
    } finally {
      setAssigning(false);
    }
  };

  if (!config) {
    return (
      <div>
        <button onClick={() => navigate('/wgconfig')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70 mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to WG Configs
        </button>
        <p className="text-[13px] text-white/40">Loading…</p>
      </div>
    );
  }

  const net = networkForID(config.NetworkID);

  // Split networks for the dropdown
  const unassignedNetworks = networks.filter(
    (n) => !n.WGConfigID || n.WGConfigID === EMPTY_ID || n._id === config.NetworkID
  );
  const assignedNetworks = networks.filter(
    (n) => n.WGConfigID && n.WGConfigID !== EMPTY_ID && n._id !== config.NetworkID
  );

  return (
    <div className="max-w-2xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/wgconfig')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to WG Configs
        </button>
        <div className="flex gap-2">
          {editing ? (
            <>
              {saveError && <span className="text-[12px] text-red-400 self-center">{saveError}</span>}
              <button onClick={() => setEditing(false)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80">
                <X className="w-3.5 h-3.5" /> Cancel
              </button>
              <button onClick={handleSave} disabled={saving} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 disabled:opacity-50">
                <Save className="w-3.5 h-3.5" /> {saving ? 'Saving…' : 'Save'}
              </button>
            </>
          ) : (
            <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80 hover:bg-white/[0.04]">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-white mb-4">{config.Tag}</h1>

      {/* Config fields */}
      <div className="border border-[#1e2433] rounded-lg overflow-hidden mb-6">
        <Row label="ID">
          <span className="font-mono text-[12px] text-white/50">{config._id}</span>
        </Row>
        <Row label="Tag">
          {editing ? (
            <input className={inputClass} value={form.Tag} onChange={set('Tag')} required />
          ) : (
            <span>{config.Tag}</span>
          )}
        </Row>
        <Row label="API Key (--key)">
          <div className="flex items-center gap-2 min-w-0">
            <span className="font-mono text-[12px] text-white/70 truncate flex-1">
              {config.APIKey || '—'}
            </span>
            {config.APIKey && (
              <button
                onClick={() => copyToClipboard(config.APIKey)}
                className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-white/40 hover:text-white/70 hover:bg-white/[0.05] shrink-0"
                title="Copy API key"
              >
                <Copy className="w-3 h-3" /> Copy
              </button>
            )}
          </div>
        </Row>
        <Row label="WG Pub Key">
          <div className="flex items-center gap-2 min-w-0">
            <span className="font-mono text-[12px] text-white/50 truncate flex-1">
              {config.WireGuardPubKey || '—'}
            </span>
            {config.WireGuardPubKey && (
              <button
                onClick={() => copyToClipboard(config.WireGuardPubKey)}
                className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-white/40 hover:text-white/70 hover:bg-white/[0.05] shrink-0"
                title="Copy public key"
              >
                <Copy className="w-3 h-3" /> Copy
              </button>
            )}
          </div>
        </Row>
        <Row label="Network">
          {editing ? (
            <select className={inputClass} value={form.NetworkID} onChange={set('NetworkID')}>
              <option value="">— None —</option>
              {unassignedNetworks.length > 0 && (
                <optgroup label="Available">
                  {unassignedNetworks.map((n) => (
                    <option key={n._id} value={n._id}>
                      {n.CIDR}{n.Tag ? ` — ${n.Tag}` : ''}
                    </option>
                  ))}
                </optgroup>
              )}
              {assignedNetworks.length > 0 && (
                <optgroup label="Already assigned">
                  {assignedNetworks.map((n) => (
                    <option key={n._id} value={n._id}>
                      {n.CIDR}{n.Tag ? ` — ${n.Tag}` : ''} (assigned)
                    </option>
                  ))}
                </optgroup>
              )}
            </select>
          ) : net ? (
            <span className="font-mono text-[12px]">
              {net.CIDR}
              {net.Tag && <span className="text-white/40 ml-2">— {net.Tag}</span>}
            </span>
          ) : (
            <span className="text-white/30">—</span>
          )}
        </Row>
        <Row label="WG Port">
          {editing ? (
            <input type="number" className={inputClass} value={form.WireGuardPort} onChange={set('WireGuardPort')} />
          ) : (
            <span className="text-[12px]">{config.WireGuardPort || '—'}</span>
          )}
        </Row>
        <Row label="WG Interface">
          {editing ? (
            <input className={inputClass} value={form.WireGuardIface} onChange={set('WireGuardIface')} />
          ) : (
            <span className="font-mono text-[12px]">{config.WireGuardIface || '—'}</span>
          )}
        </Row>
        <Row label="Internet Iface">
          {editing ? (
            <input className={inputClass} value={form.InternetIface} onChange={set('InternetIface')} />
          ) : (
            <span className="font-mono text-[12px]">{config.InternetIface || '—'}</span>
          )}
        </Row>
        <Row label="Packet Inspect">
          {editing ? (
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.PacketInspection} onChange={set('PacketInspection')} className="accent-[#4B7BF5]" />
              <span className="text-[12px] text-white/60">{form.PacketInspection ? 'Yes' : 'No'}</span>
            </label>
          ) : (
            <span className={config.PacketInspection ? 'text-emerald-400' : 'text-white/30'}>
              {config.PacketInspection ? 'Yes' : 'No'}
            </span>
          )}
        </Row>
        <Row label="Skip Verify">
          {editing ? (
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.InsecureSkipVerify} onChange={set('InsecureSkipVerify')} className="accent-[#4B7BF5]" />
              <span className="text-[12px] text-white/60">{form.InsecureSkipVerify ? 'Yes' : 'No'}</span>
            </label>
          ) : (
            <span className={config.InsecureSkipVerify ? 'text-amber-400' : 'text-white/30'}>
              {config.InsecureSkipVerify ? 'Yes' : 'No'}
            </span>
          )}
        </Row>
      </div>

      {/* Assign to Server */}
      <div className="border border-[#1e2433] rounded-lg p-5">
        <h2 className="text-[14px] font-semibold text-white mb-1">Assign to Server</h2>
        <p className="text-[12px] text-white/40 mb-4">
          Links this config to a server so it uses the correct WireGuard public key and subnet.
        </p>
        <form onSubmit={handleAssign} className="space-y-3">
          <div>
            <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Server</label>
            <select
              className={inputClass}
              value={assignForm.ServerID}
              onChange={(e) => setAssignForm({ ServerID: e.target.value })}
              required
            >
              <option value="">— Select server —</option>
              {servers.map((s) => (
                <option key={s._id} value={s._id}>{s.Tag} ({s.IP})</option>
              ))}
            </select>
          </div>
          {assignError && <p className="text-[12px] text-red-400">{assignError}</p>}
          <button
            type="submit"
            disabled={assigning}
            className="flex items-center gap-2 px-4 py-1.5 text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 rounded disabled:opacity-50"
          >
            <Link className="w-3.5 h-3.5" />
            {assigning ? 'Assigning…' : 'Assign'}
          </button>
        </form>

        {assignResult && (
          <div className="mt-4 p-4 bg-emerald-500/5 border border-emerald-500/20 rounded space-y-1.5">
            <p className="text-[12px] text-emerald-400 font-medium mb-2">Assigned successfully</p>
            {Object.entries(assignResult).map(([k, v]) => (
              <div key={k} className="flex gap-3">
                <span className="text-[11px] text-white/40 w-36 shrink-0">{k}</span>
                <span className="text-[11px] text-white/70 font-mono break-all">{String(v)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
