import { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { ArrowLeft, Plus, Copy, Check } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#fdfcf8] border border-[#e7e3d7] rounded px-3 py-1.5 text-[13px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a]";

function Field({ label, hint, children }) {
  return (
    <div>
      <label className="block text-[11px] text-[#a3a3a3] uppercase tracking-wider mb-1">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-[#a3a3a3] mt-1">{hint}</p>}
    </div>
  );
}

export default function WGConfigCreate() {
  const navigate = useNavigate();
  const location = useLocation();

  const [networks, setNetworks] = useState(location.state?.networks || []);
  const [form, setForm] = useState({
    Tag: '',
    NetworkID: '',
    WireGuardPort: 51820,
    WireGuardIface: 'wg0',
    InternetIface: 'eth0',
    AdminAPIKey: '',
    PacketInspection: false,
    InsecureSkipVerify: false,
  });
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (networks.length === 0) {
      apiPost('/ui/network/list', { Limit: 50000, Offset: 0 }).then(async (r) => {
        if (r.status === 200) {
          const data = await r.json();
          const list = Array.isArray(data.Networks) ? data.Networks : Array.isArray(data) ? data : [];
          setNetworks(list);
        }
      }).catch(() => {});
    }
  }, []);

  const set = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked
      : e.target.type === 'number' ? Number(e.target.value)
      : e.target.value;
    setForm((f) => ({ ...f, [k]: val }));
  };

  const handleCreate = async (e) => {
    e.preventDefault();
    if (!form.NetworkID) {
      setError('Please select a network');
      return;
    }
    setError('');
    setCreating(true);
    try {
      const resp = await apiPost('/ui/wg/server-config', {
        ...form,
        NetworkID: form.NetworkID,
      });
      if (resp.status === 200) {
        const data = await resp.json();
        setResult(data);
      } else {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Failed to create config');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const copyAPIKey = () => {
    navigator.clipboard.writeText(result.APIKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  // Show success result
  if (result) {
    return (
      <div className="max-w-2xl">
        <div className="flex items-center justify-between mb-6">
          <button onClick={() => navigate('/wgconfig')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
            <ArrowLeft className="w-3.5 h-3.5" /> Back to WG Configs
          </button>
        </div>

        <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-1">Config Created</h1>
        <p className="text-[12px] text-[#b45309] mb-4">Save the APIKey now — it will not be shown again.</p>

        <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow mb-5">
          {Object.entries(result).map(([k, v]) => (
            <div key={k} className="flex items-start gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50">
              <span className="text-[11px] text-[#a3a3a3] uppercase tracking-wider w-36 shrink-0 pt-0.5">{k}</span>
              <span className={`flex-1 font-mono text-[12px] break-all ${k === 'APIKey' ? 'text-[#b45309]' : 'text-[#262626]'}`}>
                {String(v)}
              </span>
              {k === 'APIKey' && (
                <button
                  onClick={copyAPIKey}
                  className="flex items-center gap-1 px-2 py-1 text-[11px] rounded bg-black/[0.04] text-[#525252] hover:text-[#0a0a0a] shrink-0"
                >
                  {copied ? <Check className="w-3 h-3 text-[#15803d]" /> : <Copy className="w-3 h-3" />}
                  {copied ? 'Copied' : 'Copy'}
                </button>
              )}
            </div>
          ))}
        </div>

        <button
          onClick={() => navigate('/wgconfig')}
          className="px-4 py-2 text-[13px] bg-black/[0.05] text-[#0a0a0a] hover:bg-black/[0.08] rounded"
        >
          Done
        </button>
      </div>
    );
  }

  const unassignedNetworks = networks.filter(
    (n) => !n.WGConfigID || n.WGConfigID === '00000000-0000-0000-0000-000000000000'
  );
  const assignedNetworks = networks.filter(
    (n) => n.WGConfigID && n.WGConfigID !== '00000000-0000-0000-0000-000000000000'
  );

  return (
    <div className="max-w-2xl">
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/wgconfig')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to WG Configs
        </button>
      </div>

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-5">New WG Config</h1>

      <form onSubmit={handleCreate} className="space-y-4">
        <div className="border border-[#e7e3d7] rounded-lg p-5 space-y-4">
          <div className="grid grid-cols-2 gap-x-5 gap-y-4">
            <Field label="Tag">
              <input type="text" className={inputClass} value={form.Tag} onChange={set('Tag')} required />
            </Field>
            <Field label="Admin API Key" hint="Optional — used by wg-server for admin ops">
              <input type="text" className={inputClass} value={form.AdminAPIKey} onChange={set('AdminAPIKey')} placeholder="leave blank to skip" />
            </Field>
            <Field label="WireGuard Port">
              <input type="number" className={inputClass} value={form.WireGuardPort} onChange={set('WireGuardPort')} />
            </Field>
            <Field label="WireGuard Interface">
              <input type="text" className={inputClass} value={form.WireGuardIface} onChange={set('WireGuardIface')} />
            </Field>
            <Field label="Internet Interface">
              <input type="text" className={inputClass} value={form.InternetIface} onChange={set('InternetIface')} />
            </Field>
          </div>

          <Field label="Network (subnet)" hint="The /22 network that defines this config's WireGuard subnet">
            <select className={inputClass} value={form.NetworkID} onChange={set('NetworkID')} required>
              <option value="">— Select a network —</option>
              {unassignedNetworks.length > 0 && (
                <optgroup label="Available">
                  {unassignedNetworks.map((n) => (
                    <option key={n._id} value={n._id}>
                      {n.CIDR}{n.Tag ? ` — ${n.Tag}` : ''}
                    </option>
                  ))}
                </optgroup>
              )}
              {assignedNetworks.length > 0 && (
                <optgroup label="Already assigned">
                  {assignedNetworks.map((n) => (
                    <option key={n._id} value={n._id}>
                      {n.CIDR}{n.Tag ? ` — ${n.Tag}` : ''} (assigned)
                    </option>
                  ))}
                </optgroup>
              )}
            </select>
          </Field>

          <div className="flex gap-6 pt-1">
            <label className="flex items-center gap-2 text-[12px] text-[#525252] cursor-pointer">
              <input type="checkbox" checked={form.PacketInspection} onChange={set('PacketInspection')} className="accent-[#1d4ed8]" />
              Packet Inspection
            </label>
            <label className="flex items-center gap-2 text-[12px] text-[#525252] cursor-pointer">
              <input type="checkbox" checked={form.InsecureSkipVerify} onChange={set('InsecureSkipVerify')} className="accent-[#1d4ed8]" />
              Insecure Skip Verify
            </label>
          </div>
        </div>

        {error && <p className="text-[12px] text-[#dc2626]">{error}</p>}

        <div className="flex gap-2">
          <button type="button" onClick={() => navigate('/wgconfig')} className="px-3 py-1.5 text-[12px] text-[#525252] hover:text-[#0a0a0a]">
            Cancel
          </button>
          <button type="submit" disabled={creating} className="flex items-center gap-2 px-4 py-1.5 text-[12px] bg-[#0a0a0a] hover:bg-[#262626] text-white rounded disabled:opacity-50">
            <Plus className="w-3.5 h-3.5" />
            {creating ? 'Creating…' : 'Create Config'}
          </button>
        </div>
      </form>
    </div>
  );
}
