import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { setUserMeta, isLoggedIn } from '../auth';
import { apiPostRaw } from '../api';

export default function Login() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  if (isLoggedIn()) {
    navigate('/users', { replace: true });
    return null;
  }

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const resp = await apiPostRaw('/ui/user/login', {
        Email: email,
        Password: password,
        DeviceName: 'admin-ui',
      });

      if (resp.status !== 200) {
        const data = await resp.json().catch(() => ({}));
        setError(data.Error || 'Login failed');
        return;
      }

      const user = await resp.json();

      if (!user.IsAdmin && !user.IsManager) {
        setError('Admin or Manager access required');
        return;
      }

      setUserMeta({
        Email: user.Email,
        IsAdmin: user.IsAdmin,
        IsManager: user.IsManager,
      });

      navigate('/users', { replace: true });
    } catch (err) {
      setError(err.message || 'Network error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#fdfcf8] flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-10 text-center">
          <div className="inline-flex items-center gap-1 mb-2">
            <span className="text-[18px] font-semibold tracking-tight text-[#0a0a0a]">Tunnels Admin</span>
            <span className="w-1 h-1 rounded-full bg-[#0a0a0a]" />
          </div>
          <p className="text-[13px] text-[#525252]">Sign in with an admin account</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-[10px] text-[#737373] uppercase tracking-[0.12em] block mb-1.5">Email</label>
            <input
              type="text"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full bg-white border border-[#e7e3d7] rounded-md px-3 py-2 text-[13px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a] transition-colors"
              autoComplete="username"
              required
            />
          </div>
          <div>
            <label className="text-[10px] text-[#737373] uppercase tracking-[0.12em] block mb-1.5">Password</label>
            <input
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-white border border-[#e7e3d7] rounded-md px-3 py-2 text-[13px] text-[#0a0a0a] placeholder-[#a3a3a3] focus:outline-none focus:border-[#0a0a0a] transition-colors"
              autoComplete="current-password"
              required
            />
          </div>

          {error && (
            <div className="py-2 px-3 rounded-md border border-[#dc2626]/25 bg-[#dc2626]/[0.04] text-[12px] text-[#b91c1c]">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-[#0a0a0a] hover:bg-[#262626] disabled:opacity-50 text-white text-[13px] font-medium rounded-md py-2.5 transition-colors"
          >
            {loading ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
