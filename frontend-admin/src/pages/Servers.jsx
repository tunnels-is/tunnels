import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, RefreshCw } from 'lucide-react';
import { apiPost } from '../api';

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

const emptyForm = () => ({ Tag: '', IP: '', Port: '443', Country: '', WireGuardSubnet: '', WireGuardPort: '51820' });

export default function Servers() {
  const navigate = useNavigate();
  const [servers, setServers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState(emptyForm());
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/ui/servers', { StartIndex: 0 });
      if (resp.status === 200) {
        const data = await resp.json();
        setServers(Array.isArray(data) ? data : []);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load servers');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const resp = await apiPost('/ui/server/create', { Server: createForm });
      if (resp.status === 200) {
        setShowCreate(false);
        setCreateForm(emptyForm());
        load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setCreateError(data.Error || 'Failed to create server');
      }
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const set = (k) => (e) => setCreateForm((f) => ({ ...f, [k]: e.target.value }));

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">Servers</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{servers.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New Server
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
        <div className="grid grid-cols-[1fr_120px_60px_120px_160px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Tag', 'IP', 'Port', 'WG Subnet', 'WG Pub Key'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {servers.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">No servers found</div>
        )}

        {servers.map((s) => (
          <div
            key={s._id}
            onClick={() => navigate(`/servers/${s._id}`, { state: { server: s } })}
            className="grid grid-cols-[1fr_120px_60px_120px_160px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a] truncate">{s.Tag}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{s.IP}</span>
            <span className="text-[12px] text-[#525252]">{s.Port}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono truncate">{s.WireGuardSubnet || '—'}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono truncate">
              {s.WireGuardPubKey ? s.WireGuardPubKey.slice(0, 16) + '…' : '—'}
            </span>
          </div>
        ))}
      </div>

      {showCreate && (
        <Modal title="New Server" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={createForm.Tag} onChange={set('Tag')} required />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">IP</label>
              <input type="text" className={inputClass} value={createForm.IP} onChange={set('IP')} placeholder="1.2.3.4" required />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Port</label>
              <input type="text" className={inputClass} value={createForm.Port} onChange={set('Port')} />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Country</label>
              <input type="text" className={inputClass} value={createForm.Country} onChange={set('Country')} />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Subnet</label>
              <input type="text" className={inputClass} value={createForm.WireGuardSubnet} onChange={set('WireGuardSubnet')} placeholder="10.1.0.0/16" />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Port</label>
              <input type="text" className={inputClass} value={createForm.WireGuardPort} onChange={set('WireGuardPort')} />
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
