import { useState, useEffect } from 'react';
import { Plus, RefreshCw, Pencil, Trash2 } from 'lucide-react';
import { apiPost } from '../api';

function Modal({ title, onClose, children }) {
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-6">
      <div className="bg-[#ffffff] border border-[#e7e3d7] rounded-lg w-full max-w-md p-5 max-h-full overflow-y-auto">
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
const labelClass = "block text-[11px] text-[#737373] uppercase tracking-[0.12em] mb-1";

const emptyForm = () => ({ Tag: '', CIDR: '', Description: '' });

export default function WANs() {
  const [wans, setWans] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState(emptyForm());
  const [editingID, setEditingID] = useState(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState('');

  const [deleting, setDeleting] = useState(null);

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await apiPost('/ui/wan/list', {});
      if (resp.status === 200) {
        const data = await resp.json();
        setWans(Array.isArray(data) ? data : []);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to load WANs');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const openCreate = () => {
    setForm(emptyForm());
    setEditingID(null);
    setFormError('');
    setShowForm(true);
  };

  const openEdit = (wan) => {
    setForm({ Tag: wan.Tag || '', CIDR: wan.CIDR || '', Description: wan.Description || '' });
    setEditingID(wan.ID);
    setFormError('');
    setShowForm(true);
  };

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  const handleSave = async (e) => {
    e.preventDefault();
    setFormError('');
    setSaving(true);
    try {
      const isEdit = !!editingID;
      const path = isEdit ? '/ui/wan/update' : '/ui/wan/create';
      const wan = isEdit ? { ...form, ID: editingID } : { ...form };
      const resp = await apiPost(path, { WAN: wan });
      if (resp.status === 200) {
        setShowForm(false);
        setForm(emptyForm());
        setEditingID(null);
        load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setFormError(data.Error || 'Failed to save WAN');
      }
    } catch (err) {
      setFormError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (wan) => {
    setDeleting(wan.ID);
    setError('');
    try {
      const resp = await apiPost('/ui/wan/delete', { WANID: wan.ID });
      if (resp.status === 200) {
        load();
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to delete WAN');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(null);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between gap-4 mb-5">
        <div className="flex items-baseline gap-2.5">
          <h1 className="text-[16px] font-semibold tracking-tight text-[#0a0a0a]">WANs</h1>
          <span className="text-[11px] font-mono tabular-nums text-[#a3a3a3]">{wans.length}</span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={load} disabled={loading} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04] transition-colors">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button onClick={openCreate} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] transition-colors">
            <Plus className="w-3.5 h-3.5" />
            New WAN
          </button>
        </div>
      </div>

      <p className="text-[12px] text-[#737373] mb-4 max-w-2xl">
        A WAN is an over-arching network (e.g. <span className="font-mono">10.0.0.0/8</span>) that groups
        multiple WireGuard server subnets. Assign a WAN to a server so clients can route the whole WAN
        through the tunnel and reach peers on sibling servers.
      </p>

      {error && <p className="text-[12px] text-[#dc2626] mb-3">{error}</p>}

      <div className="border border-[#e7e3d7] rounded-lg overflow-hidden bg-white">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="border-b border-[#e7e3d7] text-[#737373]">
              <th className="text-left font-medium px-4 py-2.5">Tag</th>
              <th className="text-left font-medium px-4 py-2.5">CIDR</th>
              <th className="text-left font-medium px-4 py-2.5">Description</th>
              <th className="px-4 py-2.5 w-[90px]"></th>
            </tr>
          </thead>
          <tbody>
            {wans.length === 0 && !loading && (
              <tr><td colSpan={4} className="px-4 py-6 text-center text-[12px] text-[#a3a3a3]">No WANs configured</td></tr>
            )}
            {wans.map((wan) => (
              <tr key={wan.ID} className="border-b border-[#f0ede4] last:border-0 hover:bg-black/[0.015]">
                <td className="px-4 py-2.5 text-[#0a0a0a]">{wan.Tag}</td>
                <td className="px-4 py-2.5 font-mono text-[#262626]">{wan.CIDR}</td>
                <td className="px-4 py-2.5 text-[#525252]">{wan.Description || <span className="text-[#a3a3a3]">—</span>}</td>
                <td className="px-4 py-2.5">
                  <div className="flex items-center justify-end gap-1">
                    <button onClick={() => openEdit(wan)} title="Edit" className="p-1.5 rounded text-[#737373] hover:text-[#0a0a0a] hover:bg-black/[0.05]">
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button onClick={() => handleDelete(wan)} disabled={deleting === wan.ID} title="Delete" className="p-1.5 rounded text-[#737373] hover:text-[#dc2626] hover:bg-black/[0.05] disabled:opacity-40">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showForm && (
        <Modal title={editingID ? 'Edit WAN' : 'New WAN'} onClose={() => setShowForm(false)}>
          <form onSubmit={handleSave} className="space-y-3">
            <div>
              <label className={labelClass}>Tag</label>
              <input className={inputClass} value={form.Tag} onChange={set('Tag')} placeholder="fleet" autoFocus />
            </div>
            <div>
              <label className={labelClass}>CIDR</label>
              <input className={inputClass} value={form.CIDR} onChange={set('CIDR')} placeholder="10.0.0.0/8" />
            </div>
            <div>
              <label className={labelClass}>Description</label>
              <input className={inputClass} value={form.Description} onChange={set('Description')} placeholder="Optional" />
            </div>
            {formError && <p className="text-[12px] text-[#dc2626]">{formError}</p>}
            <div className="flex items-center justify-end gap-2 pt-1">
              <button type="button" onClick={() => setShowForm(false)} className="px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.04]">
                Cancel
              </button>
              <button type="submit" disabled={saving} className="px-3 py-1.5 rounded text-[12px] bg-[#0a0a0a] text-white hover:bg-[#262626] disabled:opacity-50">
                {saving ? 'Saving…' : editingID ? 'Save changes' : 'Create WAN'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
