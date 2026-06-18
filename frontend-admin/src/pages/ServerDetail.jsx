import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X, Copy, Shield, Trash2, RefreshCw } from 'lucide-react';
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

export default function ServerDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [server, setServer] = useState(location.state?.server || null);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmingRegen, setConfirmingRegen] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const [keyError, setKeyError] = useState('');

  const load = async () => {
    const resp = await apiPost('/ui/server', { ServerID: id });
    if (resp.status === 200) {
      setServer(await resp.json());
    }
  };

  // location.state.server is only an instant-paint placeholder from the list,
  // which may be stale (e.g. after an APIKey rotation). Always refetch the
  // authoritative record from the backend on mount.
  useEffect(() => {
    load();
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
      EnableFirewall: !!server.EnableFirewall,
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

  const copy = (text) => {
    navigator.clipboard.writeText(text).catch(() => {});
  };

  // Rotate the APIKey: re-save the server with the key cleared; the backend
  // mints a fresh one when it sees an empty APIKey.
  const handleRegenerateKey = async () => {
    setRegenerating(true);
    setKeyError('');
    try {
      const resp = await apiPost('/ui/server/update', {
        Server: { ...server, APIKey: '' },
      });
      if (resp.status === 200) {
        setConfirmingRegen(false);
        await load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setKeyError(data.Error || 'Failed to generate key');
      }
    } catch (err) {
      setKeyError(err.message);
    } finally {
      setRegenerating(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    setError('');
    try {
      const resp = await apiPost('/ui/server/delete', { ServerID: id });
      if (resp.status === 200) {
        navigate('/servers');
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to delete');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
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
          ) : confirmingDelete ? (
            <>
              {error && <span className="text-[12px] text-[#dc2626] self-center">{error}</span>}
              <span className="text-[12px] text-[#525252] self-center">Delete this server?</span>
              <button onClick={() => setConfirmingDelete(false)} disabled={deleting} className="px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a]">
                Cancel
              </button>
              <button onClick={handleDelete} disabled={deleting} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#dc2626] hover:bg-[#b91c1c] text-white disabled:opacity-50">
                <Trash2 className="w-3.5 h-3.5" /> {deleting ? 'Deleting...' : 'Confirm Delete'}
              </button>
            </>
          ) : (
            <>
              <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]">
                <Pencil className="w-3.5 h-3.5" /> Edit
              </button>
              <button onClick={() => { setError(''); setConfirmingDelete(true); }} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#dc2626] hover:bg-[#dc2626]/10">
                <Trash2 className="w-3.5 h-3.5" /> Delete
              </button>
            </>
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
      </div>

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
          <Row label="API Key (--key)">
            <div className="flex items-center gap-2 min-w-0">
              <span className="font-mono text-[12px] text-[#262626] truncate flex-1">{server.APIKey}</span>
              <button onClick={() => copy(server.APIKey)} className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-[#a3a3a3] hover:text-[#262626] hover:bg-black/[0.05] shrink-0">
                <Copy className="w-3 h-3" /> Copy
              </button>
              {confirmingRegen ? (
                <div className="flex items-center gap-1 shrink-0">
                  <button onClick={() => setConfirmingRegen(false)} disabled={regenerating} className="px-2 py-0.5 rounded text-[11px] text-[#525252] hover:text-[#0a0a0a]">
                    Cancel
                  </button>
                  <button onClick={handleRegenerateKey} disabled={regenerating} className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] bg-[#0a0a0a] hover:bg-[#262626] text-white disabled:opacity-50">
                    <RefreshCw className={`w-3 h-3 ${regenerating ? 'animate-spin' : ''}`} /> {regenerating ? 'Generating…' : 'Confirm'}
                  </button>
                </div>
              ) : (
                <button onClick={() => { setKeyError(''); setConfirmingRegen(true); }} className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] text-[#a3a3a3] hover:text-[#262626] hover:bg-black/[0.05] shrink-0">
                  <RefreshCw className="w-3 h-3" /> New key
                </button>
              )}
            </div>
          </Row>
          {keyError && (
            <div className="px-4 py-2 text-[12px] text-[#dc2626] border-b border-[#e7e3d7]/50">{keyError}</div>
          )}
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
          <Row label="Firewall">
            {editing ? (
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={!!form.EnableFirewall} onChange={set('EnableFirewall')} className="accent-[#1d4ed8]" />
                <span className="text-[12px] text-[#525252]">{form.EnableFirewall ? 'Yes' : 'No'}</span>
              </label>
            ) : (
              <span className={server.EnableFirewall ? 'text-[#15803d]' : 'text-[#a3a3a3]'}>
                {server.EnableFirewall ? 'Yes' : 'No'}
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
    </div>
  );
}
