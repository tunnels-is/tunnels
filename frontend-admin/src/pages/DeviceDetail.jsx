import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#fdfcf8] border border-[#e7e3d7] rounded px-3 py-1.5 text-[13px] text-[#0a0a0a] focus:outline-none focus:border-[#0a0a0a]";

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50">
      <span className="text-[11px] text-[#a3a3a3] uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-[#0a0a0a] min-w-0">{children}</div>
    </div>
  );
}

export default function DeviceDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [device, setDevice] = useState(location.state?.device || null);
  const [servers, setServers] = useState([]);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const load = async () => {
    const [devResp, srvResp] = await Promise.all([
      apiPost('/ui/device', { DeviceID: id }),
      apiPost('/ui/servers', { StartIndex: 0 }),
    ]);
    if (devResp.status === 200) {
      setDevice(await devResp.json());
    }
    if (srvResp.status === 200) {
      const data = await srvResp.json();
      setServers(Array.isArray(data) ? data : []);
    }
  };

  useEffect(() => {
    load();
  }, [id]);

  const serverTag = (sid) => {
    const s = servers.find((s) => s._id === sid);
    return s ? `${s.Tag} (${s.IP})` : sid || '—';
  };

  const startEdit = () => {
    setForm({
      Tag: device.Tag || '',
      WireGuardKey: device.WireGuardKey || '',
    });
    setError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const resp = await apiPost('/ui/device/update', {
        Device: {
          ...device,
          Tag: form.Tag,
          WireGuardKey: form.WireGuardKey,
        },
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

  if (!device) {
    return (
      <div>
        <button onClick={() => navigate('/devices')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626] mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Devices
        </button>
        <p className="text-[13px] text-[#a3a3a3]">Loading...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/devices')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Devices
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

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-4">{device.Tag}</h1>

      <div className="border border-[#e7e3d7] rounded-lg overflow-hidden">
        <Row label="ID">
          <span className="font-mono text-[12px] text-[#525252]">{device._id}</span>
        </Row>
        <Row label="Tag">
          {editing ? (
            <input className={inputClass} value={form.Tag} onChange={set('Tag')} />
          ) : (
            <span>{device.Tag}</span>
          )}
        </Row>
        <Row label="WireGuard IP">
          <span className="font-mono text-[12px]">{device.WireGuardIP || '—'}</span>
        </Row>
        <Row label="WireGuard Key">
          {editing ? (
            <input className={`${inputClass} w-full font-mono text-[12px]`} value={form.WireGuardKey} onChange={set('WireGuardKey')} placeholder="base64 public key" />
          ) : (
            <span className="font-mono text-[12px] break-all text-[#525252]">{device.WireGuardKey || '—'}</span>
          )}
        </Row>
        <Row label="Server">
          <span className="text-[12px]">{serverTag(device.ServerID)}</span>
        </Row>
        <Row label="User ID">
          <span className="font-mono text-[12px] text-[#525252]">{device.UserID || '—'}</span>
        </Row>
        <Row label="Groups">
          <span className="text-[12px] text-[#525252]">
            {device.Groups?.length ? device.Groups.join(', ') : '—'}
          </span>
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-[#525252]">
            {device.CreatedAt ? dayjs(device.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>
    </div>
  );
}
