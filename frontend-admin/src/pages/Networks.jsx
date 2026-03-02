import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { apiPost } from '../api';

const PAGE_SIZE = 200;

export default function Networks() {
  const navigate = useNavigate();
  const [networks, setNetworks] = useState([]);
  const [wgConfigs, setWgConfigs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const [netResp, cfgResp] = await Promise.all([
        apiPost('/v3/network/list', { Limit: PAGE_SIZE, Offset: 0 }),
        apiPost('/v3/wg/server-config/list', {}),
      ]);
      if (netResp.status === 200) {
        const data = await netResp.json();
        setNetworks(Array.isArray(data) ? data : []);
      } else if (netResp.status !== 204) {
        const data = await netResp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load networks');
      }
      if (cfgResp.status === 200) {
        const data = await cfgResp.json();
        setWgConfigs(Array.isArray(data) ? data : []);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const configTag = (id) => {
    if (!id || id === '000000000000000000000000') return null;
    return wgConfigs.find((c) => c._id === id)?.Tag || id.slice(0, 8) + '…';
  };

  const filtered = search
    ? networks.filter((n) =>
        n.CIDR.includes(search) ||
        (n.Tag || '').toLowerCase().includes(search.toLowerCase()) ||
        (n.Description || '').toLowerCase().includes(search.toLowerCase())
      )
    : networks;

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-white">Networks</h1>
        <div className="flex gap-2">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search CIDR, tag…"
            className="bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[12px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50 w-48"
          />
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/60 hover:text-white/80 hover:bg-white/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-red-400 mb-3">{error}</p>}

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[140px_1fr_1fr_160px] gap-4 px-4 py-2 border-b border-[#1e2433] bg-[#0a0d14]">
          {['CIDR', 'Tag', 'Description', 'WG Config'].map((h) => (
            <span key={h} className="text-[10px] text-white/40 uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {filtered.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-white/40">
            {search ? 'No networks match your search' : 'No networks found'}
          </div>
        )}

        {filtered.map((n) => {
          const tag = configTag(n.WGConfigID);
          return (
            <div
              key={n._id}
              onClick={() => navigate(`/networks/${n._id}`, { state: { network: n, wgConfigs } })}
              className="grid grid-cols-[140px_1fr_1fr_160px] gap-4 px-4 py-2.5 border-b border-[#1e2433]/50 hover:bg-white/[0.03] cursor-pointer items-center"
            >
              <span className="text-[12px] text-white/80 font-mono">{n.CIDR}</span>
              <span className="text-[12px] text-white/60 truncate">{n.Tag || '—'}</span>
              <span className="text-[12px] text-white/40 truncate">{n.Description || '—'}</span>
              <span className={`text-[11px] font-mono truncate ${tag ? 'text-[#4B7BF5]' : 'text-white/25'}`}>
                {tag || 'Unassigned'}
              </span>
            </div>
          );
        })}
      </div>

      {networks.length > 0 && (
        <p className="text-[11px] text-white/25 mt-2">
          Showing {filtered.length} of {networks.length} networks
        </p>
      )}
    </div>
  );
}
