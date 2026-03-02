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
        apiPost('/v3/wg/server-config/list', {}),
        apiPost('/v3/network/list', { Limit: 50000, Offset: 0 }),
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
        setNetworks(Array.isArray(data) ? data : []);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const networkCIDR = (nid) => {
    if (!nid || nid === '000000000000000000000000') return '—';
    const n = networks.find((n) => n._id === nid);
    return n ? n.CIDR : nid.slice(0, 8) + '…';
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-white">WireGuard Configs</h1>
        <div className="flex gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/60 hover:text-white/80 hover:bg-white/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
            onClick={() => navigate('/wgconfig/create', { state: { networks } })}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Add Config
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-red-400 mb-3">{error}</p>}

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[1fr_160px_80px_160px] gap-4 px-4 py-2 border-b border-[#1e2433] bg-[#0a0d14]">
          {['Tag', 'Network (CIDR)', 'WG Port', 'Interfaces'].map((h) => (
            <span key={h} className="text-[10px] text-white/40 uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {configs.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-white/40">No configs found</div>
        )}

        {configs.map((c) => (
          <div
            key={c._id}
            onClick={() => navigate(`/wgconfig/${c._id}`, { state: { config: c, networks } })}
            className="grid grid-cols-[1fr_160px_80px_160px] gap-4 px-4 py-2.5 border-b border-[#1e2433]/50 hover:bg-white/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-white/80 truncate">{c.Tag}</span>
            <span className="text-[12px] text-white/60 font-mono truncate">{networkCIDR(c.NetworkID)}</span>
            <span className="text-[12px] text-white/50">{c.WireGuardPort || '—'}</span>
            <span className="text-[11px] text-white/40 truncate">{c.WireGuardIface || '—'} / {c.InternetIface || '—'}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
