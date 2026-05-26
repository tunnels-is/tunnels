import { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import dayjs from 'dayjs';
import { ArrowLeft, Trash2 } from 'lucide-react';
import { apiPost } from '../api';

function Row({ label, children }) {
  return (
    <div className="flex items-start gap-4 px-4 py-2.5 border-b border-[#e7e3d7]/50">
      <span className="text-[11px] text-[#a3a3a3] uppercase tracking-wider w-36 shrink-0 pt-0.5">{label}</span>
      <div className="flex-1 text-[13px] text-[#0a0a0a] min-w-0">{children}</div>
    </div>
  );
}

export default function DeviceDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const [device, setDevice] = useState(location.state?.device || null);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState('');

  const load = async () => {
    const resp = await apiPost('/ui/device', { DeviceID: id });
    if (resp.status === 200) {
      setDevice(await resp.json());
    }
  };

  useEffect(() => {
    if (!device) load();
  }, [id]);

  const handleDelete = async () => {
    setDeleting(true);
    setError('');
    try {
      const resp = await apiPost('/ui/device/delete', { DID: id });
      if (resp.status === 200) {
        navigate('/devices');
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

  if (!device) {
    return (
      <div>
        <button onClick={() => navigate('/devices')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626] mb-5">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Devices
        </button>
        <p className="text-[13px] text-[#a3a3a3]">Loading...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl">
      <div className="flex items-center justify-between mb-6">
        <button onClick={() => navigate('/devices')} className="flex items-center gap-2 text-[12px] text-[#a3a3a3] hover:text-[#262626]">
          <ArrowLeft className="w-3.5 h-3.5" /> Back to Devices
        </button>
        <div className="flex items-center gap-2">
          {error && <span className="text-[12px] text-[#dc2626] self-center">{error}</span>}
          {confirmingDelete ? (
            <>
              <span className="text-[12px] text-[#525252]">Delete this device?</span>
              <button onClick={() => setConfirmingDelete(false)} disabled={deleting} className="px-3 py-1.5 rounded text-[12px] text-[#525252] hover:text-[#0a0a0a]">
                Cancel
              </button>
              <button onClick={handleDelete} disabled={deleting} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] bg-[#dc2626] hover:bg-[#b91c1c] text-white disabled:opacity-50">
                <Trash2 className="w-3.5 h-3.5" /> {deleting ? 'Deleting...' : 'Confirm Delete'}
              </button>
            </>
          ) : (
            <button onClick={() => setConfirmingDelete(true)} className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] text-[#dc2626] hover:bg-[#dc2626]/10">
              <Trash2 className="w-3.5 h-3.5" /> Delete
            </button>
          )}
        </div>
      </div>

      <h1 className="text-[16px] font-semibold text-[#0a0a0a] mb-4">{device.Tag}</h1>

      <div className="bg-white border border-[#e7e3d7] rounded-lg overflow-hidden card-shadow">
        <Row label="ID">
          <span className="font-mono text-[12px] text-[#525252]">{device._id}</span>
        </Row>
        <Row label="Tag">
          <span>{device.Tag}</span>
        </Row>
        <Row label="WireGuard IP">
          <span className="font-mono text-[12px]">{device.WireGuardIP || '—'}</span>
        </Row>
        <Row label="WireGuard Key">
          <span className="font-mono text-[12px] break-all text-[#525252]">{device.WireGuardKey || '—'}</span>
        </Row>
        <Row label="Server ID">
          <span className="font-mono text-[12px] text-[#525252]">{device.ServerID || '—'}</span>
        </Row>
        <Row label="User ID">
          <span className="font-mono text-[12px] text-[#525252]">{device.UserID || '—'}</span>
        </Row>
        <Row label="Groups">
          <span className="text-[12px] text-[#525252]">
            {device.Groups?.length ? device.Groups.join(', ') : '—'}
          </span>
        </Row>
        <Row label="Created">
          <span className="font-mono text-[12px] text-[#525252]">
            {device.CreatedAt ? dayjs(device.CreatedAt).format('DD-MM-YYYY HH:mm') : '—'}
          </span>
        </Row>
      </div>
    </div>
  );
}
