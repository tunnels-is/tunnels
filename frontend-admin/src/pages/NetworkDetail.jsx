import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Pencil, Save, X } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50";

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#1e2433]/50">
      <span className="text-[11px] text-white/40 uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-white/80 min-w-0">{children}</div>
    </div>
  );
}

const EMPTY_ID = '00000000-0000-0000-0000-000000000000';

export default function NetworkDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [network, setNetwork] = useState(location.state?.network || null);
  const [wgConfigs, setWgConfigs] = useState(location.state?.wgConfigs || []);
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const load = async () => {
    const [netResp, cfgResp] = await Promise.all([
      apiPost('/ui/network/list', { Limit: 50000, Offset: 0 }),
      wgConfigs.length === 0 ? apiPost('/ui/wg/server-config/list', {}) : Promise.resolve(null),
    ]);
    if (netResp.status === 200) {
      const data = await netResp.json();
      const list = Array.isArray(data.Networks) ? data.Networks : Array.isArray(data) ? data : [];
      const found = list.find((n) => n._id === id);
      if (found) setNetwork(found);
    }
    if (cfgResp && cfgResp.status === 200) {
      const data = await cfgResp.json();
      setWgConfigs(Array.isArray(data) ? data : []);
    }
  };

  useEffect(() => {
    if (!network) load();
    else if (wgConfigs.length === 0) {
      apiPost('/ui/wg/server-config/list', {}).then(async (r) => {
        if (r.status === 200) {
          const data = await r.json();
          setWgConfigs(Array.isArray(data) ? data : []);
        }
      }).catch(() => {});
    }
  }, [id]);

  const configTag = (cid) => {
    if (!cid || cid === EMPTY_ID) return null;
    return wgConfigs.find((c) => c._id === cid)?.Tag || null;
  };

  const startEdit = () => {
    setForm({
      Tag: network.Tag || '',
      Description: network.Description || '',
      WGConfigID: network.WGConfigID && network.WGConfigID !== EMPTY_ID ? network.WGConfigID : '',
    });
    setError('');
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const updated = {
        ...network,
        Tag: form.Tag,
        Description: form.Description,
        WGConfigID: form.WGConfigID || EMPTY_ID,
      };
      const resp = await apiPost('/ui/network/update', { Network: updated });
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

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  if (!network) {
    return (
      <div>
        <button onClick={() => navigate('/networks')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70 mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Networks
        </button>
        <p className="text-[13px] text-white/40">Loading...</p>
      </div>
    );
  }

  const assignedTag = configTag(network.WGConfigID);

  return (
    <div className="max-w-2xl">
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/networks')} className="flex items-center gap-2 text-[12px] text-white/40 hover:text-white/70">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Networks
        </button>
        <div className="flex gap-2">
          {editing ? (
            <>
              {error && <span className="text-[12px] text-red-400 self-center">{error}</span>}
              <button onClick={() => setEditing(false)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80">
                <X className="w-3.5 h-3.5" /> Cancel
              </button>
              <button onClick={handleSave} disabled={saving} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 disabled:opacity-50">
                <Save className="w-3.5 h-3.5" /> {saving ? 'Saving…' : 'Save'}
              </button>
            </>
          ) : (
            <button onClick={startEdit} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-white/50 hover:text-white/80 hover:bg-white/[0.04]">
              <Pencil className="w-3.5 h-3.5" /> Edit
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-white font-mono mb-4">{network.CIDR}</h1>

      <div className="border border-[#1e2433] rounded-lg overflow-hidden">
        <Row label="ID">
          <span className="font-mono text-[12px] text-white/50">{network._id}</span>
        </Row>
        <Row label="CIDR">
          <span className="font-mono text-[12px]">{network.CIDR}</span>
        </Row>
        <Row label="Tag">
          {editing ? (
            <input className={inputClass} value={form.Tag} onChange={set('Tag')} placeholder="optional label" />
          ) : (
            <span className={network.Tag ? '' : 'text-white/30'}>{network.Tag || '—'}</span>
          )}
        </Row>
        <Row label="Description">
          {editing ? (
            <input className={inputClass} value={form.Description} onChange={set('Description')} placeholder="optional description" />
          ) : (
            <span className={network.Description ? '' : 'text-white/30'}>{network.Description || '—'}</span>
          )}
        </Row>
        <Row label="WG Config">
          {editing ? (
            <select className={inputClass} value={form.WGConfigID} onChange={set('WGConfigID')}>
              <option value="">— Unassigned —</option>
              {wgConfigs.map((c) => (
                <option key={c._id} value={c._id}>{c.Tag}</option>
              ))}
            </select>
          ) : (
            <span className={assignedTag ? 'text-[#4B7BF5]' : 'text-white/30'}>
              {assignedTag || 'Unassigned'}
            </span>
          )}
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-white/50">
            {network.CreatedAt ? dayjs(network.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>
    </div>
  );
}
