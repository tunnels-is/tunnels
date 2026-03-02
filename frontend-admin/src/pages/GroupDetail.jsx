import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X, Trash2, Plus } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[13px] text-white focus:outline-none focus:border-[#4B7BF5]/50";

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#1e2433]/50">
      <span className="text-[11px] text-white/40 uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-white/80">{children}</div>
    </div>
  );
}

export default function GroupDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [group, setGroup] = useState(location.state?.group || null);
  const [members, setMembers] = useState({ users: [], devices: [], servers: [] });
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [addForm, setAddForm] = useState({ type: 'user', value: '' });
  const [addError, setAddError] = useState('');

  const loadGroup = async () => {
    const resp = await apiPost('/v3/group', { GID: id });
    if (resp.status === 200) {
      setGroup(await resp.json());
    }
  };

  const loadMembers = async () => {
    const results = { users: [], devices: [], servers: [] };
    for (const type of ['user', 'device', 'server']) {
      const resp = await apiPost('/v3/group/entities', { GID: id, Type: type, Limit: 500, Offset: 0 });
      if (resp.status === 200) {
        const data = await resp.json();
        results[`${type}s`] = Array.isArray(data) ? data : [];
      }
    }
    setMembers(results);
  };

  useEffect(() => {
    loadGroup();
    loadMembers();
  }, [id]);

  const startEdit = () => {
    setForm({ Tag: group.Tag || '', Description: group.Description || '' });
    setError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const resp = await apiPost('/v3/group/update', {
        Group: { ...group, Tag: form.Tag, Description: form.Description },
      });
      if (resp.status === 200) {
        setEditing(false);
        await loadGroup();
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to save');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleAdd = async (e) => {
    e.preventDefault();
    setAddError('');
    if (!addForm.value) return;
    try {
      const resp = await apiPost('/v3/group/add', {
        GroupID: id,
        Type: addForm.type,
        TypeID: addForm.value,
      });
      if (resp.status === 200) {
        setAddForm((f) => ({ ...f, value: '' }));
        loadMembers();
      } else {
        const data = await resp.json().catch(() => ({}));
        setAddError(data.Error || 'Failed to add member');
      }
    } catch (err) {
      setAddError(err.message);
    }
  };

  const handleRemove = async (type, typeID) => {
    try {
      const resp = await apiPost('/v3/group/remove', { GroupID: id, Type: type, TypeID: typeID });
      if (resp.status === 200) loadMembers();
    } catch {
      // ignore
    }
  };

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  if (!group) {
    return (
      <div>
        <button onClick={() => navigate('/groups')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70 mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Groups
        </button>
        <p className="text-[13px] text-white/40">Loading...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/groups')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Groups
        </button>
        <div className="flex gap-2">
          {editing ? (
            <>
              {error && <span className="text-[12px] text-red-400 self-center">{error}</span>}
              <button onClick={() => setEditing(false)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80">
                <X className="w-3.5 h-3.5" /> Cancel
              </button>
              <button onClick={handleSave} disabled={saving} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 disabled:opacity-50">
                <Save className="w-3.5 h-3.5" /> {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          ) : (
            <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80 hover:bg-white/[0.04]">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-white mb-4">{group.Tag}</h1>

      {/* Info */}
      <div className="border border-[#1e2433] rounded-lg overflow-hidden mb-6">
        <Row label="ID">
          <span className="font-mono text-[12px] text-white/50">{group._id}</span>
        </Row>
        <Row label="Tag">
          {editing ? (
            <input className={inputClass} value={form.Tag} onChange={set('Tag')} />
          ) : (
            <span>{group.Tag}</span>
          )}
        </Row>
        <Row label="Description">
          {editing ? (
            <input className={inputClass} value={form.Description} onChange={set('Description')} />
          ) : (
            <span className="text-white/50">{group.Description || '—'}</span>
          )}
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-white/50">
            {group.CreatedAt ? dayjs(group.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>

      {/* Members */}
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-[14px] font-semibold text-white">Members</h2>
      </div>

      {/* Add member form */}
      <form onSubmit={handleAdd} className="flex gap-2 mb-4">
        <select
          className="bg-[#0a0d14] border border-[#1e2433] rounded px-2 py-1.5 text-[12px] text-white/70 focus:outline-none"
          value={addForm.type}
          onChange={(e) => setAddForm((f) => ({ ...f, type: e.target.value }))}
        >
          <option value="user">User</option>
          <option value="device">Device</option>
          <option value="server">Server</option>
        </select>
        <input
          className="flex-1 bg-[#0a0d14] border border-[#1e2433] rounded px-3 py-1.5 text-[12px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50"
          placeholder="hex ID"
          value={addForm.value}
          onChange={(e) => setAddForm((f) => ({ ...f, value: e.target.value }))}
        />
        <button type="submit" className="flex items-center gap-1.5 px-3 py-1.5 bg-[#4B7BF5]/10 text-[#4B7BF5] rounded text-[12px] hover:bg-[#4B7BF5]/20">
          <Plus className="w-3.5 h-3.5" /> Add
        </button>
      </form>
      {addError && <p className="text-[12px] text-red-400 mb-3">{addError}</p>}

      {/* Member lists */}
      {[
        { key: 'users', label: 'Users', type: 'user', nameKey: 'Email' },
        { key: 'devices', label: 'Devices', type: 'device', nameKey: 'Tag' },
        { key: 'servers', label: 'Servers', type: 'server', nameKey: 'Tag' },
      ].map(({ key, label, type, nameKey }) => (
        <div key={key} className="mb-4">
          <div className="text-[11px] text-white/40 uppercase tracking-wider mb-1.5">
            {label} <span className="text-white/20">({members[key].length})</span>
          </div>
          {members[key].length === 0 ? (
            <p className="text-[12px] text-white/30 pl-1">None</p>
          ) : (
            <div className="border border-[#1e2433] rounded-lg overflow-hidden">
              {members[key].map((m) => (
                <div key={m._id} className="flex items-center justify-between px-4 py-2 border-b border-[#1e2433]/50 hover:bg-white/[0.02]">
                  <div>
                    <span className="text-[13px] text-white/80">{m[nameKey] || m._id}</span>
                    <span className="ml-3 font-mono text-[11px] text-white/30">{m._id}</span>
                  </div>
                  <button
                    onClick={() => handleRemove(type, m._id)}
                    className="p-1 rounded text-white/30 hover:text-red-400 hover:bg-red-400/10 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
