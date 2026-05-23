import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, RefreshCw } from 'lucide-react';
import { apiPost } from '../api';

export default function WGConfig() {
  const navigate = useNavigate();
  const [configs, setConfigs] = useState([]);
  const [networks, setNetworks] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const [cfgResp, netResp] = await Promise.all([
        apiPost('/ui/wg/server-config/list', {}),
        apiPost('/ui/network/list', { Limit: 50000, Offset: 0 }),
      ]);
      if (cfgResp.status === 200) {
        const data = await cfgResp.json();
        setConfigs(Array.isArray(data) ? data : []);
      } else if (cfgResp.status !== 204) {
        const data = await cfgResp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load configs');
      }
      if (netResp.status === 200) {
        const data = await netResp.json();
        const list = Array.isArray(data.Networks) ? data.Networks : Array.isArray(data) ? data : [];
        setNetworks(list);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const networkCIDR = (nid) => {
    if (!nid || nid === '00000000-0000-0000-0000-000000000000') return '—';
    const n = networks.find((n) => n._id === nid);
    return n ? n.CIDR : nid.slice(0, 8) + '…';
  };

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">WireGuard Configs</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{configs.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
            onClick={() => navigate('/wgconfig/create', { state: { networks } })}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Add Config
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
        <div className="grid grid-cols-[1fr_160px_80px_160px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Tag', 'Network (CIDR)', 'WG Port', 'Interfaces'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {configs.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">No configs found</div>
        )}

        {configs.map((c) => (
          <div
            key={c._id}
            onClick={() => navigate(`/wgconfig/${c._id}`, { state: { config: c, networks } })}
            className="grid grid-cols-[1fr_160px_80px_160px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a] truncate">{c.Tag}</span>
            <span className="text-[12px] text-[#525252] font-mono truncate">{networkCIDR(c.NetworkID)}</span>
            <span className="text-[12px] text-[#525252]">{c.WireGuardPort || '—'}</span>
            <span className="text-[11px] text-[#a3a3a3] truncate">{c.WireGuardIface || '—'} / {c.InternetIface || '—'}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
