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

export default function Groups() {
  const navigate = useNavigate();
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [tag, setTag] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/v3/group/list', {});
      if (resp.status === 200) {
        const data = await resp.json();
        setGroups(Array.isArray(data) ? data : []);
      } else if (resp.status === 204) {
        setGroups([]);
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

  useEffect(() => { load(); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const resp = await apiPost('/v3/group/create', { Group: { Tag: tag } });
      if (resp.status === 200) {
        setShowCreate(false);
        setTag('');
        load();
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
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-white">Groups</h1>
        <div className="flex gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/60 hover:text-white/80 hover:bg-white/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New Group
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-red-400 mb-3">{error}</p>}

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[1fr_200px] gap-4 px-4 py-2 border-b border-[#1e2433] bg-[#0a0d14]">
          {['Tag', 'Created'].map((h) => (
            <span key={h} className="text-[10px] text-white/40 uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {groups.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-white/40">No groups found</div>
        )}

        {groups.map((g) => (
          <div
            key={g._id}
            onClick={() => navigate(`/groups/${g._id}`, { state: { group: g } })}
            className="grid grid-cols-[1fr_200px] gap-4 px-4 py-2.5 border-b border-[#1e2433]/50 hover:bg-white/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-white/80">{g.Tag}</span>
            <span className="text-[11px] text-white/40 font-mono">
              {g.CreatedAt ? dayjs(g.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          </div>
        ))}
      </div>

      {showCreate && (
        <Modal title="New Group" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Tag</label>
              <input type="text" className={inputClass} value={tag} onChange={(e) => setTag(e.target.value)} required />
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
