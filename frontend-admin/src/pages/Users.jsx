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

export default function Users() {
  const navigate = useNavigate();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ Email: '', Password: '' });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/ui/user/list', { Limit: 200, Offset: 0 });
      if (resp.status === 200) {
        const data = await resp.json();
        setUsers(Array.isArray(data) ? data : []);
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

  useEffect(() => { load(); }, []);

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
        load();
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

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[16px] font-semibold text-white">Users</h1>
        <div className="flex gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/60 hover:text-white/80 hover:bg-white/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New User
          </button>
        </div>
      </div>

      {error && <p className="text-[12px] text-red-400 mb-3">{error}</p>}

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <div className="grid grid-cols-[1fr_80px_80px_80px_160px] gap-4 px-4 py-2 border-b border-[#1e2433] bg-[#0a0d14]">
          {['Email', 'Admin', 'Manager', 'Disabled', 'Sub Expiry'].map((h) => (
            <span key={h} className="text-[10px] text-white/40 uppercase tracking-wider">{h}</span>
          ))}
        </div>

        {users.length === 0 && !loading && (
          <div className="px-4 py-6 text-[12px] text-white/40">No users found</div>
        )}

        {users.map((u) => (
          <div
            key={u._id}
            onClick={() => navigate(`/users/${u._id}`, { state: { user: u } })}
            className="grid grid-cols-[1fr_80px_80px_80px_160px] gap-4 px-4 py-2.5 border-b border-[#1e2433]/50 hover:bg-white/[0.03] cursor-pointer items-center"
          >
            <span className="text-[13px] text-white/80 truncate">{u.Email}</span>
            <span className={`text-[11px] ${u.IsAdmin ? 'text-emerald-400' : 'text-white/30'}`}>{u.IsAdmin ? 'Yes' : 'No'}</span>
            <span className={`text-[11px] ${u.IsManager ? 'text-blue-400' : 'text-white/30'}`}>{u.IsManager ? 'Yes' : 'No'}</span>
            <span className={`text-[11px] ${u.Disabled ? 'text-red-400' : 'text-white/30'}`}>{u.Disabled ? 'Yes' : 'No'}</span>
            <span className="text-[11px] text-white/40 font-mono">
              {u.SubExpiration ? dayjs(u.SubExpiration).format('DD-MM-YYYY') : '—'}
            </span>
          </div>
        ))}
      </div>

      {showCreate && (
        <Modal title="New User" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Email</label>
              <input type="text" className={inputClass} value={createForm.Email} onChange={(e) => setCreateForm({ ...createForm, Email: e.target.value })} required />
            </div>
            <div>
              <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">Password</label>
              <input type="password" className={inputClass} value={createForm.Password} onChange={(e) => setCreateForm({ ...createForm, Password: e.target.value })} minLength={10} required />
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
