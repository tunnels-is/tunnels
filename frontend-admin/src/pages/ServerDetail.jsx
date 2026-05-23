import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X, Copy, Shield } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#fdfcf8] border border-[#e7e3d7] rounded px-3 py-1.5 text-[13px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a]";

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50">
      <span className="text-[11px] text-[#a3a3a3] uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-[#0a0a0a] min-w-0">{children}</div>
    </div>
  );
}

// RFC 4122 v4 UUID generator — used to mint a per-server WG APIKey client-side.
function uuidv4() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID();
  const buf = new Uint8Array(16);
  crypto.getRandomValues(buf);
  buf[6] = (buf[6] & 0x0f) | 0x40;
  buf[8] = (buf[8] & 0x3f) | 0x80;
  const h = [...buf].map((b) => b.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export default function ServerDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [server, setServer] = useState(location.state?.server || null);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [enabling, setEnabling] = useState(false);
  const [enableError, setEnableError] = useState('');
  const [newAPIKey, setNewAPIKey] = useState('');

  const load = async () => {
    const resp = await apiPost('/ui/servers', { StartIndex: 0 });
    if (resp.status === 200) {
      const list = await resp.json();
      const found = (Array.isArray(list) ? list : []).find((s) => s._id === id);
      if (found) setServer(found);
    }
  };

  useEffect(() => {
    if (!server) load();
  }, [id]);

  const startEdit = () => {
    setForm({
      Tag: server.Tag || '',
      IP: server.IP || '',
      Port: server.Port || '443',
      Country: server.Country || '',
      WireGuardSubnet: server.WireGuardSubnet || '',
      WireGuardSubnet6: server.WireGuardSubnet6 || '',
      WireGuardPort: server.WireGuardPort || 51820,
      WireGuardIface: server.WireGuardIface || 'wg0',
      InternetIface: server.InternetIface || '',
      PacketInspection: !!server.PacketInspection,
      InsecureSkipVerify: !!server.InsecureSkipVerify,
    });
    setError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const resp = await apiPost('/ui/server/update', {
        Server: { ...server, ...form, WireGuardPort: Number(form.WireGuardPort) || 0 },
      });
      if (resp.status === 200) {
        setEditing(false);
        await load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to save');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleEnableWG = async () => {
    setEnabling(true);
    setEnableError('');
    const apiKey = uuidv4();
    try {
      const resp = await apiPost('/ui/server/update', {
        Server: {
          ...server,
          APIKey: apiKey,
          WireGuardPort: server.WireGuardPort || 51820,
          WireGuardIface: server.WireGuardIface || 'wg0',
        },
      });
      if (resp.status === 200) {
        setNewAPIKey(apiKey);
        await load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setEnableError(data.Error || 'Failed to enable WG');
      }
    } catch (err) {
      setEnableError(err.message);
    } finally {
      setEnabling(false);
    }
  };

  const copy = (text) => {
    navigator.clipboard.writeText(text).catch(() => {});
  };

  const set = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked
      : e.target.type === 'number' ? Number(e.target.value)
      : e.target.value;
    setForm((f) => ({ ...f, [k]: val }));
  };

  if (!server) {
    return (
      <div>
        <button onClick={() => navigate('/servers')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626] mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Servers
        </button>
        <p className="text-[13px] text-[#a3a3a3]">Loading...</p>
      </div>
    );
  }

  const wgEnabled = !!server.APIKey;

  return (
    <div className="max-w-2xl">
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/servers')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Servers
        </button>
        <div className="flex gap-2">
          {editing ? (
            <>
              {error && <span className="text-[12px] text-[#dc2626] self-center">{error}</span>}
              <button onClick={() => setEditing(false)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a]">
                <X className="w-3.5 h-3.5" /> Cancel
              </button>
              <button onClick={handleSave} disabled={saving} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] disabled:opacity-50">
                <Save className="w-3.5 h-3.5" /> {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          ) : (
            <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-4">{server.Tag}</h1>

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow mb-6">
        <Row label="ID">
          <span className="font-mono text-[12px] text-[#525252]">{server._id}</span>
        </Row>
        <Row label="Tag">
          {editing ? (
            <input className={inputClass} value={form.Tag} onChange={set('Tag')} required />
          ) : (
            <span>{server.Tag}</span>
          )}
        </Row>
        <Row label="IP">
          {editing ? (
            <input className={inputClass} value={form.IP} onChange={set('IP')} placeholder="1.2.3.4" />
          ) : (
            <span className="font-mono text-[12px]">{server.IP || '—'}</span>
          )}
        </Row>
        <Row label="Port">
          {editing ? (
            <input className={inputClass} value={form.Port} onChange={set('Port')} />
          ) : (
            <span className="text-[12px]">{server.Port || '—'}</span>
          )}
        </Row>
        <Row label="Country">
          {editing ? (
            <input className={inputClass} value={form.Country} onChange={set('Country')} />
          ) : (
            <span className="text-[12px]">{server.Country || '—'}</span>
          )}
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-[#525252]">
            {server.CreatedAt ? dayjs(server.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>

      <div className="flex items-baseline justify-between mb-2">
        <h2 className="text-[14px] font-semibold text-[#0a0a0a] flex items-center gap-2">
          <Shield className="w-3.5 h-3.5 text-[#a3a3a3]" />
          WireGuard
        </h2>
        {!wgEnabled && !editing && (
          <button
            onClick={handleEnableWG}
            disabled={enabling}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#0a0a0a] hover:bg-[#262626] text-white disabled:opacity-50"
          >
            {enabling ? 'Enabling…' : 'Enable WireGuard'}
          </button>
        )}
      </div>
      {enableError && <p className="text-[12px] text-[#dc2626] mb-2">{enableError}</p>}
      {newAPIKey && (
        <div className="mb-3 p-3 rounded border border-[#b45309]/40 bg-[#fef3c7]/40 text-[12px]">
          <div className="text-[#b45309] font-medium mb-1">Save this APIKey now — it will not be shown again.</div>
          <div className="flex items-center gap-2">
            <span className="font-mono text-[#262626] break-all flex-1">{newAPIKey}</span>
            <button onClick={() => copy(newAPIKey)} className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.05] shrink-0">
              <Copy className="w-3 h-3" /> Copy
            </button>
          </div>
        </div>
      )}

      {wgEnabled && (
        <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
          <Row label="API Key (--key)">
            <div className="flex items-center gap-2 min-w-0">
              <span className="font-mono text-[12px] text-[#262626] truncate flex-1">{server.APIKey}</span>
              <button onClick={() => copy(server.APIKey)} className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-[#a3a3a3] hover:text-[#262626] hover:bg-black/[0.05] shrink-0">
                <Copy className="w-3 h-3" /> Copy
              </button>
            </div>
          </Row>
          <Row label="WG Pub Key">
            <span className="font-mono text-[12px] text-[#525252] break-all">{server.WireGuardPubKey || '—'}</span>
          </Row>
          <Row label="WG Subnet">
            {editing ? (
              <input className={inputClass} value={form.WireGuardSubnet} onChange={set('WireGuardSubnet')} placeholder="10.42.0.0/22" />
            ) : (
              <span className="font-mono text-[12px]">{server.WireGuardSubnet || '—'}</span>
            )}
          </Row>
          <Row label="WG Subnet6">
            {editing ? (
              <input className={inputClass} value={form.WireGuardSubnet6} onChange={set('WireGuardSubnet6')} placeholder="fd00::/64" />
            ) : (
              <span className="font-mono text-[12px]">{server.WireGuardSubnet6 || '—'}</span>
            )}
          </Row>
          <Row label="WG Port">
            {editing ? (
              <input type="number" className={inputClass} value={form.WireGuardPort} onChange={set('WireGuardPort')} />
            ) : (
              <span className="text-[12px]">{server.WireGuardPort || '—'}</span>
            )}
          </Row>
          <Row label="WG Interface">
            {editing ? (
              <input className={inputClass} value={form.WireGuardIface} onChange={set('WireGuardIface')} />
            ) : (
              <span className="font-mono text-[12px]">{server.WireGuardIface || '—'}</span>
            )}
          </Row>
          <Row label="Internet Iface">
            {editing ? (
              <input className={inputClass} value={form.InternetIface} onChange={set('InternetIface')} />
            ) : (
              <span className="font-mono text-[12px]">{server.InternetIface || '—'}</span>
            )}
          </Row>
          <Row label="Packet Inspect">
            {editing ? (
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={!!form.PacketInspection} onChange={set('PacketInspection')} className="accent-[#1d4ed8]" />
                <span className="text-[12px] text-[#525252]">{form.PacketInspection ? 'Yes' : 'No'}</span>
              </label>
            ) : (
              <span className={server.PacketInspection ? 'text-[#15803d]' : 'text-[#a3a3a3]'}>
                {server.PacketInspection ? 'Yes' : 'No'}
              </span>
            )}
          </Row>
          <Row label="Skip Verify">
            {editing ? (
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={!!form.InsecureSkipVerify} onChange={set('InsecureSkipVerify')} className="accent-[#1d4ed8]" />
                <span className="text-[12px] text-[#525252]">{form.InsecureSkipVerify ? 'Yes' : 'No'}</span>
              </label>
            ) : (
              <span className={server.InsecureSkipVerify ? 'text-[#b45309]' : 'text-[#a3a3a3]'}>
                {server.InsecureSkipVerify ? 'Yes' : 'No'}
              </span>
            )}
          </Row>
        </div>
      )}
    </div>
  );
}
