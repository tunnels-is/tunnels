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

export default function Groups() {
  const navigate = useNavigate();
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [tag, setTag] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  const load = async (next = offset) => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/ui/group/list', { Limit: PAGE_SIZE, Offset: next });
      if (resp.status === 200) {
        const data = await resp.json();
        const list = Array.isArray(data) ? data : [];
        setGroups(list);
        setHasMore(list.length === PAGE_SIZE);
        setOffset(next);
      } else if (resp.status === 204) {
        setGroups([]);
        setHasMore(false);
        setOffset(next);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load groups');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(0); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const resp = await apiPost('/ui/group/create', { Group: { Tag: tag } });
      if (resp.status === 200) {
        setShowCreate(false);
        setTag('');
        load(offset);
      } else {
        const data = await resp.json().catch(() => ({}));
        setCreateError(data.Error || 'Failed to create group');
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
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">Groups</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{groups.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => load(offset)} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New Group
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
        <div className="grid grid-cols-[1fr_200px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Tag', 'Created'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {groups.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">No groups found</div>
        )}

        {groups.map((g) => (
          <div
            key={g._id}
            onClick={() => navigate(`/groups/${g._id}`, { state: { group: g } })}
            className="grid grid-cols-[1fr_200px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a]">{g.Tag}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono">
              {g.CreatedAt ? dayjs(g.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between mt-3 px-1">
        <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">
          {groups.length === 0 ? '—' : `${offset + 1}–${offset + groups.length}`}
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
        <Modal title="New Group" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={tag} onChange={(e) => setTag(e.target.value)} required />
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
