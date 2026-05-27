import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import DatePicker from 'react-datepicker';
import { ArrowLeft, Pencil, Save, X, Trash2 } from 'lucide-react';
import { apiPost } from '../api';

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50">
      <span className="text-[11px] text-[#a3a3a3] uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-[#0a0a0a]">{children}</div>
    </div>
  );
}

export default function UserDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [user, setUser] = useState(location.state?.user || null);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const load = async () => {
    const resp = await apiPost('/ui/user/list', { Limit: 500, Offset: 0 });
    if (resp.status === 200) {
      const list = await resp.json();
      const found = (list || []).find((u) => u._id === id);
      if (found) setUser(found);
    }
  };

  useEffect(() => {
    if (!user) load();
  }, [id]);

  const startEdit = () => {
    setForm({
      IsManager: user.IsManager,
      Disabled: user.Disabled,
      Trial: user.Trial,
      SubExpiration: user.SubExpiration ? new Date(user.SubExpiration) : null,
    });
    setError('');
    setEditing(true);
  };

  const cancelEdit = () => setEditing(false);

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const resp = await apiPost('/ui/user/adminupdate', {
        TargetUserID: id,
        Disabled: form.Disabled,
        IsManager: form.IsManager,
        Trial: form.Trial,
        SubExpiration: form.SubExpiration ? form.SubExpiration.toISOString() : undefined,
      });
      if (resp.status === 200) {
        setEditing(false);
        await load();
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

  const set = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked : e.target.value;
    setForm((f) => ({ ...f, [k]: val }));
  };

  const handleDelete = async () => {
    setDeleting(true);
    setError('');
    try {
      const resp = await apiPost('/ui/user/delete', { TargetUserID: id });
      if (resp.status === 200) {
        navigate('/users');
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to delete');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
  };

  if (!user) {
    return (
      <div>
        <button onClick={() => navigate('/users')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626] mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Users
        </button>
        <p className="text-[13px] text-[#a3a3a3]">Loading...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/users')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Users
        </button>
        <div className="flex gap-2">
          {editing ? (
            <>
              {error && <span className="text-[12px] text-[#dc2626] self-center">{error}</span>}
              <button onClick={cancelEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a]">
                <X className="w-3.5 h-3.5" /> Cancel
              </button>
              <button onClick={handleSave} disabled={saving} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] disabled:opacity-50">
                <Save className="w-3.5 h-3.5" /> {saving ? 'Saving...' : 'Save'}
              </button>
            </>
          ) : confirmingDelete ? (
            <>
              {error && <span className="text-[12px] text-[#dc2626] self-center">{error}</span>}
              <span className="text-[12px] text-[#525252] self-center">Delete this user?</span>
              <button onClick={() => setConfirmingDelete(false)} disabled={deleting} className="px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a]">
                Cancel
              </button>
              <button onClick={handleDelete} disabled={deleting} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#dc2626] hover:bg-[#b91c1c] text-white disabled:opacity-50">
                <Trash2 className="w-3.5 h-3.5" /> {deleting ? 'Deleting...' : 'Confirm Delete'}
              </button>
            </>
          ) : (
            <>
              <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]">
                <Pencil className="w-3.5 h-3.5" /> Edit
              </button>
              <button onClick={() => { setError(''); setConfirmingDelete(true); }} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#dc2626] hover:bg-[#dc2626]/10">
                <Trash2 className="w-3.5 h-3.5" /> Delete
              </button>
            </>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-4">{user.Email}</h1>

      {/* Info / Edit fields */}
      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow mb-5">
        <Row label="ID">
          <span className="font-mono text-[12px] text-[#525252]">{user._id}</span>
        </Row>
        <Row label="Email">
          <span>{user.Email}</span>
        </Row>
        <Row label="Is Admin">
          <span className={user.IsAdmin ? 'text-[#15803d]' : 'text-[#a3a3a3]'}>{user.IsAdmin ? 'Yes' : 'No'}</span>
        </Row>
        <Row label="Is Manager">
          {editing ? (
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.IsManager} onChange={set('IsManager')} className="accent-[#1d4ed8]" />
              <span className="text-[12px] text-[#525252]">{form.IsManager ? 'Yes' : 'No'}</span>
            </label>
          ) : (
            <span className={user.IsManager ? 'text-[#1d4ed8]' : 'text-[#a3a3a3]'}>{user.IsManager ? 'Yes' : 'No'}</span>
          )}
        </Row>
        <Row label="Disabled">
          {editing ? (
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.Disabled} onChange={set('Disabled')} className="accent-[#1d4ed8]" />
              <span className="text-[12px] text-[#525252]">{form.Disabled ? 'Yes' : 'No'}</span>
            </label>
          ) : (
            <span className={user.Disabled ? 'text-[#dc2626]' : 'text-[#a3a3a3]'}>{user.Disabled ? 'Yes' : 'No'}</span>
          )}
        </Row>
        <Row label="Trial">
          {editing ? (
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={form.Trial} onChange={set('Trial')} className="accent-[#1d4ed8]" />
              <span className="text-[12px] text-[#525252]">{form.Trial ? 'Yes' : 'No'}</span>
            </label>
          ) : (
            <span className={user.Trial ? 'text-[#b45309]' : 'text-[#a3a3a3]'}>{user.Trial ? 'Yes' : 'No'}</span>
          )}
        </Row>
        <Row label="Sub Expiry">
          {editing ? (
            <DatePicker
              selected={form.SubExpiration}
              onChange={(date) => setForm((f) => ({ ...f, SubExpiration: date }))}
              showTimeSelect
              timeFormat="HH:mm"
              timeIntervals={15}
              dateFormat="dd/MM/yyyy HH:mm"
              placeholderText="No expiry"
              isClearable
              popperPlacement="bottom-start"
            />
          ) : (
            <span className="font-mono text-[12px]">
              {user.SubExpiration ? dayjs(user.SubExpiration).format('DD-MM-YYYY HH:mm') : '—'}
            </span>
          )}
        </Row>
        <Row label="Updated">
          <span className="font-mono text-[12px] text-[#525252]">
            {user.Updated ? dayjs(user.Updated).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>

      {/* Active sessions */}
      {user.Tokens && user.Tokens.length > 0 && (
        <div>
          <h2 className="text-[13px] font-semibold text-[#0a0a0a] mb-2">
            Active Sessions <span className="text-[#a3a3a3] font-normal">({user.Tokens.length})</span>
          </h2>
          <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
            <div className="grid grid-cols-[1fr_160px] gap-4 px-4 py-2 border-b border-[#e7e3d7] bg-[#ffffff]">
              {['Device Name', 'Created'].map((h) => (
                <span key={h} className="text-[10px] text-[#a3a3a3] uppercase tracking-wider">{h}</span>
              ))}
            </div>
            {user.Tokens.map((t, i) => (
              <div key={i} className="grid grid-cols-[1fr_160px] gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50 items-center">
                <span className="text-[13px] text-[#262626]">{t.N || '—'}</span>
                <span className="text-[11px] text-[#a3a3a3] font-mono">
                  {t.Created ? dayjs(t.Created).format('DD-MM-YYYY HH:mm') : '—'}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
