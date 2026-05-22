import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import { apiPost } from '../api';

const PAGE_SIZE = 200;
const EMPTY_ID = '00000000-0000-0000-0000-000000000000';

export default function Networks() {
  const navigate = useNavigate();
  const [networks, setNetworks] = useState([]);
  const [wgConfigs, setWgConfigs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);

  const load = useCallback(async (newOffset = offset) => {
    setLoading(true);
    setError('');
    try {
      const [netResp, cfgResp] = await Promise.all([
        apiPost('/ui/network/list', { Limit: PAGE_SIZE, Offset: newOffset }),
        apiPost('/ui/wg/server-config/list', {}),
      ]);
      if (netResp.status === 200) {
        const data = await netResp.json();
        setNetworks(Array.isArray(data.Networks) ? data.Networks : []);
        setTotal(data.Total || 0);
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
  }, [offset]);

  useEffect(() => { load(0); }, []);

  const goPage = (newOffset) => {
    setOffset(newOffset);
    load(newOffset);
  };

  const configTag = (id) => {
    if (!id || id === EMPTY_ID) return null;
    return wgConfigs.find((c) => c._id === id)?.Tag || id.slice(0, 8) + '…';
  };

  const filtered = search
    ? networks.filter((n) =>
        n.CIDR.includes(search) ||
        (n.Tag || '').toLowerCase().includes(search.toLowerCase()) ||
        (n.Description || '').toLowerCase().includes(search.toLowerCase())
      )
    : networks;

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-[#0a0a0a]">Networks</h1>
        <div className="flex gap-2">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search CIDR, tag…"
            className="bg-[#fdfcf8] border border-[#e7e3d7] rounded px-3 py-1.5 text-[12px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a] w-48"
          />
          <button onClick={() => load(offset)} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="border border-[#e7e3d7] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[140px_1fr_1fr_160px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['CIDR', 'Tag', 'Description', 'WG Config'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {filtered.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">
            {search ? 'No networks match your search' : 'No networks found'}
          </div>
        )}

        {filtered.map((n) => {
          const tag = configTag(n.WGConfigID);
          return (
            <div
              key={n._id}
              onClick={() => navigate(`/networks/${n._id}`, { state: { network: n, wgConfigs } })}
              className="grid grid-cols-[140px_1fr_1fr_160px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
            >
              <span className="text-[12px] text-[#0a0a0a] font-mono">{n.CIDR}</span>
              <span className="text-[12px] text-[#525252] truncate">{n.Tag || '—'}</span>
              <span className="text-[12px] text-[#a3a3a3] truncate">{n.Description || '—'}</span>
              <span className={`text-[11px] font-mono truncate ${tag ? 'text-[#1d4ed8]' : 'text-[#c4c4c4]'}`}>
                {tag || 'Unassigned'}
              </span>
            </div>
          );
        })}
      </div>

      <div className="flex items-center justify-between mt-3">
        <p className="text-[11px] text-[#c4c4c4]">
          {total > 0
            ? `Showing ${offset + 1}–${Math.min(offset + filtered.length, total)} of ${total} networks`
            : 'No networks'}
        </p>
        {totalPages > 1 && (
          <div className="flex items-center gap-2">
            <button
              onClick={() => goPage(offset - PAGE_SIZE)}
              disabled={offset === 0 || loading}
              className="flex items-center gap-1 px-2 py-1 rounded text-[11px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="w-3.5 h-3.5" /> Prev
            </button>
            <span className="text-[11px] text-[#a3a3a3]">
              Page {currentPage} of {totalPages}
            </span>
            <button
              onClick={() => goPage(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total || loading}
              className="flex items-center gap-1 px-2 py-1 rounded text-[11px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              Next <ChevronRight className="w-3.5 h-3.5" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
