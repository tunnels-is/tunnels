import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { Clock, Plus, RefreshCw, ChevronLeft, ChevronRight, Search } from 'lucide-react';
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

const StatPill = ({ label, value }) => (
  <div className="flex items-baseline gap-1.5 rounded-md border border-[#e7e3d7] bg-[#fdfcf8] px-2.5 py-1.5">
    <span className="text-[10px] uppercase tracking-wider text-[#a3a3a3]">{label}</span>
    <span className="text-[13px] font-mono tabular-nums font-semibold text-[#0a0a0a]">{value ?? '—'}</span>
  </div>
);

export default function Users() {
  const navigate = useNavigate();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ Email: '', Password: '' });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [mode, setMode] = useState('browse'); // 'browse' | 'latest' | 'search'
  const [stats, setStats] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');

  const load = async (next = offset) => {
    setLoading(true);
    setError('');
    setMode('browse');
    setStats(null);
    try {
      const resp = await apiPost('/ui/user/list', { Limit: PAGE_SIZE, Offset: next });
      if (resp.status === 200) {
        const data = await resp.json();
        const list = Array.isArray(data) ? data : [];
        setUsers(list);
        setHasMore(list.length === PAGE_SIZE);
        setOffset(next);
      } else if (resp.status === 204) {
        setUsers([]);
        setHasMore(false);
        setOffset(next);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load users');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const loadLatest = async () => {
    setLoading(true);
    setError('');
    setMode('latest');
    setStats(null);
    try {
      const resp = await apiPost('/ui/user/latest', {});
      if (resp.status === 200) {
        const data = await resp.json();
        setUsers(Array.isArray(data.Users) ? data.Users : []);
        setStats({
          Total: data.Total ?? 0,
          Trial: data.Trial ?? 0,
          ActiveSubscribers: data.ActiveSubscribers ?? 0,
        });
        setHasMore(false);
        setOffset(0);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load latest users');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = async (e) => {
    e?.preventDefault?.();
    const email = searchQuery.trim();
    if (!email) {
      setError('Enter a username / email to search');
      return;
    }
    setLoading(true);
    setError('');
    setMode('search');
    setStats(null);
    setHasMore(false);
    setOffset(0);
    try {
      const resp = await apiPost('/ui/user/search', { Email: email });
      if (resp.status === 200) {
        const data = await resp.json();
        setUsers(Array.isArray(data) ? data : data ? [data] : []);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Search failed');
        setUsers([]);
      }
    } catch (err) {
      setError(err.message);
      setUsers([]);
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
      const resp = await fetch('/client/user/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ Email: createForm.Email, Password: createForm.Password }),
      });
      if (resp.status === 200) {
        setShowCreate(false);
        setCreateForm({ Email: '', Password: '' });
        if (mode === 'latest') loadLatest();
        else if (mode === 'search') handleSearch();
        else load(offset);
      } else {
        const data = await resp.json().catch(() => ({}));
        setCreateError(data.Error || 'Failed to create user');
      }
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const refresh = () => {
    if (mode === 'latest') loadLatest();
    else if (mode === 'search') handleSearch();
    else load(offset);
  };

  const countLabel =
    mode === 'latest' ? `latest ${users.length}` :
    mode === 'search' ? `search ${users.length}` :
    users.length;

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">Users</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{countLabel}</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={loadLatest}
            disabled={loading}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] transition-colors ${
              mode === 'latest'
                ? 'bg-[#0a0a0a] text-white'
                : 'text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]'
            }`}
          >
            <Clock className="w-3.5 h-3.5" />
            Latest
          </button>
          <button
            onClick={refresh}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New User
          </button>
        </div>
      </div>

      <form onSubmit={handleSearch} className="flex items-center gap-2 mb-4">
        <div className="relative flex-1 max-w-md">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[#a3a3a3]" />
          <input
            type="text"
            className={`${inputClass} pl-8`}
            placeholder="Search by username / email"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            autoComplete="off"
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#0a0a0a] text-white hover:bg-[#262626] disabled:opacity-50 transition-colors"
        >
          <Search className="w-3.5 h-3.5" />
          Search
        </button>
        {mode === 'search' && (
          <button
            type="button"
            onClick={() => { setSearchQuery(''); load(0); }}
            className="px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]"
          >
            Clear
          </button>
        )}
      </form>

      {stats && (
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <StatPill label="Total" value={stats.Total} />
          <StatPill label="Trial" value={stats.Trial} />
          <StatPill label="Active subs" value={stats.ActiveSubscribers} />
          <span className="text-[11px] text-[#a3a3a3]">Showing 100 most recently updated</span>
        </div>
      )}

      {mode === 'search' && !loading && !error && (
        <p className="text-[11px] text-[#a3a3a3] mb-3">
          Search results for <span className="font-mono text-[#525252]">{searchQuery.trim()}</span>
        </p>
      )}

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-x-auto card-shadow">
        <div className="grid grid-cols-[1fr_80px_80px_80px_160px_160px] min-w-[840px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
          {['Email', 'Admin', 'Manager', 'Disabled', 'Sub Expiry', 'Updated'].map((h) => (
            <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {users.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-[#a3a3a3]">No users found</div>
        )}

        {users.map((u) => (
          <div
            key={u._id}
            onClick={() => navigate(`/users/${u._id}`)}
            className="grid grid-cols-[1fr_80px_80px_80px_160px_160px] min-w-[840px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 hover:bg-black/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-[#0a0a0a] truncate">{u.Email}</span>
            <span className={`text-[11px] ${u.IsAdmin ? 'text-[#15803d]' : 'text-[#a3a3a3]'}`}>{u.IsAdmin ? 'Yes' : 'No'}</span>
            <span className={`text-[11px] ${u.IsManager ? 'text-[#1d4ed8]' : 'text-[#a3a3a3]'}`}>{u.IsManager ? 'Yes' : 'No'}</span>
            <span className={`text-[11px] ${u.Disabled ? 'text-[#dc2626]' : 'text-[#a3a3a3]'}`}>{u.Disabled ? 'Yes' : 'No'}</span>
            <span className="text-[11px] text-[#a3a3a3] font-mono">
              {u.SubExpiration ? dayjs(u.SubExpiration).format('DD-MM-YYYY') : '—'}
            </span>
            <span className="text-[11px] text-[#a3a3a3] font-mono">
              {u.Updated ? dayjs(u.Updated).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          </div>
        ))}
      </div>

      {mode === 'browse' && (
        <div className="flex items-center justify-between mt-3 px-1">
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">
            {users.length === 0 ? '—' : `${offset + 1}–${offset + users.length}`}
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
      )}

      {showCreate && (
        <Modal title="New User" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Email</label>
              <input type="text" className={inputClass} value={createForm.Email} onChange={(e) => setCreateForm({ ...createForm, Email: e.target.value })} required />
            </div>
            <div>
              <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">Password</label>
              <input type="password" className={inputClass} value={createForm.Password} onChange={(e) => setCreateForm({ ...createForm, Password: e.target.value })} minLength={10} required />
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
