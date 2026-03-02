import { useState, useEffect } from 'react';
import { Plus, Link } from 'lucide-react';
import { apiPost } from '../api';

const inputClass = "w-full bg-[#060810] border border-[#1e2433] rounded px-3 py-1.5 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/50";

function Section({ title, children }) {
  return (
    <div className="border border-[#1e2433] rounded-lg p-5 mb-5">
      <h2 className="text-[14px] font-semibold text-white mb-4">{title}</h2>
      {children}
    </div>
  );
}

function Field({ label, children, hint }) {
  return (
    <div className="mb-3">
      <label className="block text-[11px] text-white/40 uppercase tracking-wider mb-1">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-white/30 mt-1">{hint}</p>}
    </div>
  );
}

export default function WGConfig() {
  const [servers, setServers] = useState([]);
  const [createForm, setCreateForm] = useState({
    Tag: '',
    WireGuardPort: 51820,
    WireGuardSubnet: '10.1.0.0/16',
    WireGuardIface: 'wg0',
    InternetIface: 'eth0',
    AdminAPIKey: '',
    PacketInspection: false,
    InsecureSkipVerify: false,
  });
  const [createResult, setCreateResult] = useState(null);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const [assignForm, setAssignForm] = useState({ ServerID: '', ConfigID: '' });
  const [assigning, setAssigning] = useState(false);
  const [assignResult, setAssignResult] = useState(null);
  const [assignError, setAssignError] = useState('');

  useEffect(() => {
    apiPost('/v3/servers', { StartIndex: 0 }).then(async (resp) => {
      if (resp.status === 200) {
        const data = await resp.json();
        setServers(Array.isArray(data) ? data : []);
      }
    }).catch(() => {});
  }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreateError('');
    setCreateResult(null);
    setCreating(true);
    try {
      const resp = await apiPost('/v3/wg/server-config', createForm);
      if (resp.status === 200) {
        const data = await resp.json();
        setCreateResult(data);
      } else {
        const data = await resp.json().catch(() => ({}));
        setCreateError(data.Error || 'Failed to create WG config');
      }
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleAssign = async (e) => {
    e.preventDefault();
    setAssignError('');
    setAssignResult(null);
    setAssigning(true);
    try {
      const resp = await apiPost('/v3/wg/server-config/assign', {
        ServerID: assignForm.ServerID,
        ConfigID: assignForm.ConfigID,
      });
      if (resp.status === 200) {
        const data = await resp.json();
        setAssignResult(data);
      } else {
        const data = await resp.json().catch(() => ({}));
        setAssignError(data.Error || 'Failed to assign config');
      }
    } catch (err) {
      setAssignError(err.message);
    } finally {
      setAssigning(false);
    }
  };

  const setC = (k) => (e) => {
    const val = e.target.type === 'checkbox' ? e.target.checked : (e.target.type === 'number' ? Number(e.target.value) : e.target.value);
    setCreateForm((f) => ({ ...f, [k]: val }));
  };

  return (
    <div>
      <h1 className="text-[16px] font-semibold text-white mb-5">WireGuard Config</h1>

      <Section title="Create WG Server Config">
        <p className="text-[12px] text-white/40 mb-4">
          Creates a new WireGuard server config with an auto-generated key pair and API key.
          The generated APIKey is used by the wg-server to authenticate and fetch its config.
        </p>
        <form onSubmit={handleCreate}>
          <div className="grid grid-cols-2 gap-x-4 gap-y-3">
            <Field label="Tag"><input type="text" className={inputClass} value={createForm.Tag} onChange={setC('Tag')} required /></Field>
            <Field label="Admin API Key" hint="Optional: used by wg-server for admin ops">
              <input type="text" className={inputClass} value={createForm.AdminAPIKey} onChange={setC('AdminAPIKey')} placeholder="leave blank to auto-skip" />
            </Field>
            <Field label="WireGuard Port">
              <input type="number" className={inputClass} value={createForm.WireGuardPort} onChange={setC('WireGuardPort')} />
            </Field>
            <Field label="WireGuard Subnet">
              <input type="text" className={inputClass} value={createForm.WireGuardSubnet} onChange={setC('WireGuardSubnet')} />
            </Field>
            <Field label="WireGuard Interface">
              <input type="text" className={inputClass} value={createForm.WireGuardIface} onChange={setC('WireGuardIface')} />
            </Field>
            <Field label="Internet Interface">
              <input type="text" className={inputClass} value={createForm.InternetIface} onChange={setC('InternetIface')} />
            </Field>
          </div>
          <div className="flex gap-6 mb-4">
            <label className="flex items-center gap-2 text-[12px] text-white/60 cursor-pointer">
              <input type="checkbox" checked={createForm.PacketInspection} onChange={setC('PacketInspection')} className="accent-[#4B7BF5]" />
              Packet Inspection
            </label>
            <label className="flex items-center gap-2 text-[12px] text-white/60 cursor-pointer">
              <input type="checkbox" checked={createForm.InsecureSkipVerify} onChange={setC('InsecureSkipVerify')} className="accent-[#4B7BF5]" />
              Insecure Skip Verify
            </label>
          </div>
          {createError && <p className="text-[12px] text-red-400 mb-3">{createError}</p>}
          <button type="submit" disabled={creating} className="flex items-center gap-2 px-4 py-2 bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 rounded text-[13px] disabled:opacity-50">
            <Plus className="w-4 h-4" />
            {creating ? 'Creating...' : 'Create Config'}
          </button>
        </form>

        {createResult && (
          <div className="mt-4 p-4 bg-emerald-500/5 border border-emerald-500/20 rounded">
            <p className="text-[12px] text-emerald-400 mb-2 font-medium">Config created — save this APIKey, it won't be shown again:</p>
            <div className="space-y-1.5">
              {Object.entries(createResult).map(([k, v]) => (
                <div key={k} className="flex gap-3">
                  <span className="text-[11px] text-white/40 w-36 shrink-0">{k}</span>
                  <span className="text-[11px] text-white/80 font-mono break-all">{String(v)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </Section>

      <Section title="Assign Config to Server">
        <p className="text-[12px] text-white/40 mb-4">
          Links a WGServerConfig to a Server entry so the server shows the correct WireGuard public key and port.
        </p>
        <form onSubmit={handleAssign} className="space-y-3">
          <Field label="Server">
            <select
              className={inputClass}
              value={assignForm.ServerID}
              onChange={(e) => setAssignForm((f) => ({ ...f, ServerID: e.target.value }))}
              required
            >
              <option value="">— Select server —</option>
              {servers.map((s) => (
                <option key={s._id} value={s._id}>{s.Tag} ({s.IP})</option>
              ))}
            </select>
          </Field>
          <Field label="Config ID" hint="The ID returned when creating the config">
            <input
              type="text"
              className={inputClass}
              placeholder="hex object ID"
              value={assignForm.ConfigID}
              onChange={(e) => setAssignForm((f) => ({ ...f, ConfigID: e.target.value }))}
              required
            />
          </Field>
          {assignError && <p className="text-[12px] text-red-400">{assignError}</p>}
          <button type="submit" disabled={assigning} className="flex items-center gap-2 px-4 py-2 bg-[#4B7BF5]/10 text-[#4B7BF5] hover:bg-[#4B7BF5]/20 rounded text-[13px] disabled:opacity-50">
            <Link className="w-4 h-4" />
            {assigning ? 'Assigning...' : 'Assign'}
          </button>
        </form>

        {assignResult && (
          <div className="mt-4 p-4 bg-emerald-500/5 border border-emerald-500/20 rounded">
            <p className="text-[12px] text-emerald-400 mb-2 font-medium">Config assigned successfully:</p>
            <div className="space-y-1.5">
              {Object.entries(assignResult).map(([k, v]) => (
                <div key={k} className="flex gap-3">
                  <span className="text-[11px] text-white/40 w-36 shrink-0">{k}</span>
                  <span className="text-[11px] text-white/80 font-mono">{String(v)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </Section>
    </div>
  );
}
