import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X } from 'lucide-react';
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
      WireGuardPort: server.WireGuardPort || '51820',
    });
    setError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const resp = await apiPost('/ui/server/update', {
        Server: { ...server, ...form },
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

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

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
          ) : (
            <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-4">{server.Tag}</h1>

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
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
        <Row label="WG Config">
          {server.WGConfigID ? (
            <button
              onClick={() => navigate(`/wgconfig/${server.WGConfigID}`)}
              className="font-mono text-[12px] text-[#1d4ed8] hover:underline"
            >
              {server.WGConfigID}
            </button>
          ) : (
            <span className="text-[12px] text-[#a3a3a3]">—</span>
          )}
        </Row>
        <Row label="WG Subnet">
          {editing ? (
            <input className={inputClass} value={form.WireGuardSubnet} onChange={set('WireGuardSubnet')} placeholder="10.1.0.0/16" />
          ) : (
            <span className="font-mono text-[12px]">{server.WireGuardSubnet || '—'}</span>
          )}
        </Row>
        <Row label="WG Port">
          {editing ? (
            <input className={inputClass} value={form.WireGuardPort} onChange={set('WireGuardPort')} />
          ) : (
            <span className="text-[12px]">{server.WireGuardPort || '—'}</span>
          )}
        </Row>
        <Row label="WG Pub Key">
          <span className="font-mono text-[12px] text-[#525252] break-all">{server.WireGuardPubKey || '—'}</span>
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-[#525252]">
            {server.CreatedAt ? dayjs(server.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>
    </div>
  );
}
