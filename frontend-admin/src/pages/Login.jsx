import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { setAuth, isLoggedIn } from '../auth';
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
      const resp = await apiPostRaw('/v3/user/login', {
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

      if (!user.IsAdmin) {
        setError('Admin access required');
        return;
      }

      setAuth({
        _id: user._id,
        DeviceToken: user.DeviceToken?.DT || '',
        Email: user.Email,
        IsAdmin: user.IsAdmin,
      });

      navigate('/users', { replace: true });
    } catch (err) {
      setError(err.message || 'Network error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#060810] flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-xl font-semibold text-white">Tunnels Admin</h1>
          <p className="text-[13px] text-white/40 mt-1">Sign in with an admin account</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <input
              type="text"
              placeholder="Email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full bg-[#0a0d14] border border-[#1e2433] rounded-md px-3 py-2 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/60"
              autoComplete="username"
              required
            />
          </div>
          <div>
            <input
              type="password"
              placeholder="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-[#0a0d14] border border-[#1e2433] rounded-md px-3 py-2 text-[13px] text-white placeholder-white/30 focus:outline-none focus:border-[#4B7BF5]/60"
              autoComplete="current-password"
              required
            />
          </div>

          {error && (
            <p className="text-[12px] text-red-400">{error}</p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-[#4B7BF5] hover:bg-[#3d6de0] disabled:opacity-50 text-white text-[13px] font-medium rounded-md py-2 transition-colors"
          >
            {loading ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
