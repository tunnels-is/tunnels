import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { Plus, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import { apiPost } from '../api';

const PAGE_SIZE = 100;

function Modal({ title, onClose, children }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-[#ffffff] border border-[#e7e3d7] rounded-lg w-full max-w-md p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-[14px] font-semibold text-[#0a0a0a]">{title}</h3>
          <button onClick={onClose} className="text-[#a3a3a3] hover:text-[#262626] text-lg leading-none">×</button>
        </div>
        {children}
      </div>
    </div>
  );
}

const inputClass = "w-full bg-[#fdfcf8] border border-[#e7e3d7] rounded px-3 py-1.5 text-[13px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a]";

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
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  const load = async (next = offset) => {
    setLoading(true);
    setError('');
    try {
      const [devResp, srvResp] = await Promise.all([
        apiPost('/ui/device/list', { Limit: PAGE_SIZE, Offset: next }),
        apiPost('/ui/servers', { StartIndex: 0 }),
      ]);
      if (devResp.status === 200) {
        const data = await devResp.json();
        const list = Array.isArray(data) ? data : [];
        setDevices(list);
        setHasMore(list.length === PAGE_SIZE);
        setOffset(next);
      } else if (devResp.status === 204) {
        setDevices([]);
        setHasMore(false);
        setOffset(next);
      } else {
        const data = await devResp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load devices');
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

  useEffect(() => { load(0); }, []);

  const serverTag = (id) => servers.find((s) => s._id === id)?.Tag || '—';

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const device = { Tag: createForm.Tag };
      if (createForm.ServerID) device.ServerID = createForm.ServerID;
      if (createForm.WireGuardKey) device.WireGuardKey = createForm.WireGuardKey;
      const resp = await apiPost('/ui/device/create', { Device: device });
      if (resp.status === 200) {
        setShowCreate(false);
        setCreateForm({ Tag: '', ServerID: '', WireGuardKey: '' });
        load(offset);
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
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">Devices</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{devices.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => load(offset)} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New Device
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-x-auto card-shadow">
        <div className="grid grid-cols-[1fr_130px_120px_160px] min-w-[680px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Tag', 'WireGuard IP', 'Server', 'Created'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {devices.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">No devices found</div>
        )}

        {devices.map((d) => (
          <div
            key={d._id}
            onClick={() => navigate(`/devices/${d._id}`, { state: { device: d } })}
            className="grid grid-cols-[1fr_130px_120px_160px] min-w-[680px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a] truncate">{d.Tag}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{d.WireGuardIP || '—'}</span>
            <span className="text-[12px] text-[#525252] truncate">{serverTag(d.ServerID)}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono">
              {d.CreatedAt ? dayjs(d.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between mt-3 px-1">
        <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">
          {devices.length === 0 ? '—' : `${offset + 1}–${offset + devices.length}`}
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={() => load(Math.max(0, offset - PAGE_SIZE))}
            disabled={loading || offset === 0}
            className="flex items-center gap-1 px-2 py-1 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[#525252]"
          >
            <ChevronLeft className="w-3.5 h-3.5" /> Prev
          </button>
          <button
            onClick={() => load(offset + PAGE_SIZE)}
            disabled={loading || !hasMore}
            className="flex items-center gap-1 px-2 py-1 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[#525252]"
          >
            Next <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {showCreate && (
        <Modal title="New Device" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={createForm.Tag} onChange={(e) => setCreateForm({ ...createForm, Tag: e.target.value })} required />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Server</label>
              <select className={inputClass} value={createForm.ServerID} onChange={(e) => setCreateForm({ ...createForm, ServerID: e.target.value })}>
                <option value="">— No server —</option>
                {servers.map((s) => <option key={s._id} value={s._id}>{s.Tag} ({s.IP})</option>)}
              </select>
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Key (optional)</label>
              <input type="text" className={inputClass} placeholder="base64 public key" value={createForm.WireGuardKey} onChange={(e) => setCreateForm({ ...createForm, WireGuardKey: e.target.value })} />
            </div>
            {createError && <p className="text-[12px] text-[#dc2626]">{createError}</p>}
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-[12px] text-[#525252] hover:text-[#0a0a0a]">Cancel</button>
              <button type="submit" disabled={creating} className="px-4 py-1.5 text-[12px] bg-[#0a0a0a] hover:bg-[#262626] text-white rounded disabled:opacity-50">
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
