import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { Plus, RefreshCw } from 'lucide-react';
import { apiPost } from '../api';

function Modal({ title, onClose, children }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-[#0a0d14] border border-[#1e2433] rounded-lg w-full max-w-md p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-[14px] font-semibold text-white">{title}</h3>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 text-lg leading-none">×</button>
        </div>
        {children}
      </div>
    </div>
  );
}

const inputClass = "w-full bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50";

export default function Devices() {
  const navigate = useNavigate();
  const [devices, setDevices] = useState([]);
  const [servers, setServers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ Tag: '', ServerID: '', WireGuardKey: '' });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const [devResp, srvResp] = await Promise.all([
        apiPost('/v3/device/list', { Limit: 500, Offset: 0 }),
        apiPost('/v3/servers', { StartIndex: 0 }),
      ]);
      if (devResp.status === 200) {
        const data = await devResp.json();
        setDevices(Array.isArray(data) ? data : []);
      }
      if (srvResp.status === 200) {
        const data = await srvResp.json();
        setServers(Array.isArray(data) ? data : []);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const serverTag = (id) => servers.find((s) => s._id === id)?.Tag || '—';

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const device = { Tag: createForm.Tag };
      if (createForm.ServerID) device.ServerID = createForm.ServerID;
      if (createForm.WireGuardKey) device.WireGuardKey = createForm.WireGuardKey;
      const resp = await apiPost('/v3/device/create', { Device: device });
      if (resp.status === 200) {
        setShowCreate(false);
        setCreateForm({ Tag: '', ServerID: '', WireGuardKey: '' });
        load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setCreateError(data.Error || 'Failed to create device');
      }
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setCreating(false);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-white">Devices</h1>
        <div className="flex gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/60 hover:text-white/80 hover:bg-white/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New Device
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-red-400 mb-3">{error}</p>}

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[1fr_130px_120px_160px] gap-4 px-4 py-2 border-b border-[#1e2433] bg-[#0a0d14]">
          {['Tag', 'WireGuard IP', 'Server', 'Created'].map((h) => (
            <span key={h} className="text-[10px] text-white/40 uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {devices.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-white/40">No devices found</div>
        )}

        {devices.map((d) => (
          <div
            key={d._id}
            onClick={() => navigate(`/devices/${d._id}`, { state: { device: d } })}
            className="grid grid-cols-[1fr_130px_120px_160px] gap-4 px-4 py-2.5 border-b border-[#1e2433]/50 hover:bg-white/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-white/80 truncate">{d.Tag}</span>
            <span className="text-[12px] text-white/50 font-mono truncate">{d.WireGuardIP || '—'}</span>
            <span className="text-[12px] text-white/50 truncate">{serverTag(d.ServerID)}</span>
            <span className="text-[11px] text-white/40 font-mono">
              {d.CreatedAt ? dayjs(d.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          </div>
        ))}
      </div>

      {showCreate && (
        <Modal title="New Device" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={createForm.Tag} onChange={(e) => setCreateForm({ ...createForm, Tag: e.target.value })} required />
            </div>
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Server</label>
              <select className={inputClass} value={createForm.ServerID} onChange={(e) => setCreateForm({ ...createForm, ServerID: e.target.value })}>
                <option value="">— No server —</option>
                {servers.map((s) => <option key={s._id} value={s._id}>{s.Tag} ({s.IP})</option>)}
              </select>
            </div>
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">WireGuard Key (optional)</label>
              <input type="text" className={inputClass} placeholder="base64 public key" value={createForm.WireGuardKey} onChange={(e) => setCreateForm({ ...createForm, WireGuardKey: e.target.value })} />
            </div>
            {createError && <p className="text-[12px] text-red-400">{createError}</p>}
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-[12px] text-white/50 hover:text-white/80">Cancel</button>
              <button type="submit" disabled={creating} className="px-4 py-1.5 text-[12px] bg-[#4B7BF5] hover:bg-[#3d6de0] text-white rounded disabled:opacity-50">
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
