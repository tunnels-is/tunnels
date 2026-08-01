import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import { apiPost } from '../api';

const PAGE_SIZE = 100;

function Modal({ title, onClose, children }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-6">
      <div className="bg-[#ffffff] border border-[#e7e3d7] rounded-lg w-full max-w-md p-5 max-h-full overflow-y-auto">
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

const emptyForm = () => ({ Tag: '', InfraTag: '', IP: '', Port: '443', Country: '', WireGuardSubnet: '', WireGuardSubnet6: '', WireGuardPort: 51820, WireGuardIface: 'wg0', InternetIface: '', EnableFirewall: true, InsecureSkipVerify: false, WANID: '' });

export default function Servers() {
  const navigate = useNavigate();
  const [servers, setServers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState(emptyForm());
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [startIndex, setStartIndex] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [wans, setWans] = useState([]);

  const loadWans = async () => {
    try {
      const resp = await apiPost('/ui/wan/list', {});
      if (resp.status === 200) {
        const data = await resp.json();
        setWans(Array.isArray(data) ? data : []);
      }
    } catch {

    }
  };

  const load = async (index = startIndex) => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/ui/servers', { StartIndex: index });
      if (resp.status === 200) {
        const data = await resp.json();
        const list = Array.isArray(data) ? data : [];
        setServers(list);
        setHasMore(list.length === PAGE_SIZE);
        setStartIndex(index);
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

  useEffect(() => { load(0); loadWans(); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const resp = await apiPost('/ui/server/create', {
        Server: { ...createForm, WireGuardPort: Number(createForm.WireGuardPort) || 0 },
      });
      if (resp.status === 200) {
        setShowCreate(false);
        setCreateForm(emptyForm());
        load(startIndex);
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

  const set = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked : e.target.value;
    setCreateForm((f) => ({ ...f, [k]: val }));
  };

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">Servers</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{servers.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => load(startIndex)} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
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

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-x-auto card-shadow">
        <div className="grid grid-cols-[1fr_110px_50px_80px_80px_110px_100px_60px_150px] min-w-[1040px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Tag', 'IP', 'Port', 'WG Iface', 'Net Iface', 'WG Subnet', 'WAN', 'Firewall', 'WG Pub Key'].map((h) => (
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
            className="grid grid-cols-[1fr_110px_50px_80px_80px_110px_100px_60px_150px] min-w-[1040px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a] truncate">{s.Tag}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{s.IP}</span>
            <span className="text-[12px] text-[#525252]">{s.Port}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{s.WireGuardIface || '—'}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{s.InternetIface || '—'}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono truncate">{s.WireGuardSubnet || '—'}</span>
            <span className="text-[11px] text-[#525252] truncate" title={s.WAN ? `${s.WAN.Tag} (${s.WAN.CIDR})` : (s.WANID || '')}>
              {s.WAN ? s.WAN.Tag : <span className="text-[#a3a3a3]">—</span>}
            </span>
            <span className={`text-[12px] ${s.EnableFirewall ? 'text-[#15803d]' : 'text-[#a3a3a3]'}`}>
              {s.EnableFirewall ? 'On' : 'Off'}
            </span>
            <span className="text-[11px] text-[#a3a3a3] font-mono truncate">
              {s.WireGuardPubKey ? s.WireGuardPubKey.slice(0, 16) + '…' : '—'}
            </span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between mt-3 px-1">
        <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">
          {servers.length === 0
            ? '—'
            : `${startIndex + 1}–${startIndex + servers.length}`}
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={() => load(Math.max(0, startIndex - PAGE_SIZE))}
            disabled={loading || startIndex === 0}
            className="flex items-center gap-1 px-2 py-1 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[#525252]"
          >
            <ChevronLeft className="w-3.5 h-3.5" /> Prev
          </button>
          <button
            onClick={() => load(startIndex + PAGE_SIZE)}
            disabled={loading || !hasMore}
            className="flex items-center gap-1 px-2 py-1 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-[#525252]"
          >
            Next <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {showCreate && (
        <Modal title="New Server" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={createForm.Tag} onChange={set('Tag')} required />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Infra Tag</label>
              <input type="text" className={inputClass} value={createForm.InfraTag} onChange={set('InfraTag')} />
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
              <input type="text" className={inputClass} value={createForm.WireGuardSubnet} onChange={set('WireGuardSubnet')} placeholder="10.42.0.0/22" />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Subnet (IPv6)</label>
              <input type="text" className={inputClass} value={createForm.WireGuardSubnet6} onChange={set('WireGuardSubnet6')} placeholder="fd00::/64" />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Port</label>
              <input type="number" className={inputClass} value={createForm.WireGuardPort} onChange={set('WireGuardPort')} />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WireGuard Interface</label>
              <input type="text" className={inputClass} value={createForm.WireGuardIface} onChange={set('WireGuardIface')} placeholder="wg0" />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Internet Interface</label>
              <input type="text" className={inputClass} value={createForm.InternetIface} onChange={set('InternetIface')} placeholder="eth0" />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">WAN</label>
              <select className={inputClass} value={createForm.WANID} onChange={set('WANID')}>
                <option value="">None</option>
                {wans.map((w) => (
                  <option key={w.ID} value={w.ID}>{w.Tag} ({w.CIDR})</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Firewall</label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={!!createForm.EnableFirewall} onChange={set('EnableFirewall')} className="accent-[#1d4ed8]" />
                <span className="text-[12px] text-[#525252]">{createForm.EnableFirewall ? 'Enabled' : 'Disabled'}</span>
              </label>
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Skip Verify</label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={!!createForm.InsecureSkipVerify} onChange={set('InsecureSkipVerify')} className="accent-[#1d4ed8]" />
                <span className="text-[12px] text-[#525252]">{createForm.InsecureSkipVerify ? 'Yes' : 'No'}</span>
              </label>
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
