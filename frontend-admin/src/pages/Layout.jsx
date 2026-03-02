import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Users, Monitor, Layers, Server, Shield, Network, LogOut } from 'lucide-react';
import { getAuth, clearAuth } from '../auth';
import { apiPost } from '../api';

const navItems = [
  { icon: Users, label: 'Users', route: 'users' },
  { icon: Monitor, label: 'Devices', route: 'devices' },
  { icon: Layers, label: 'Groups', route: 'groups' },
  { icon: Server, label: 'Servers', route: 'servers' },
  { icon: Shield, label: 'WG Config', route: 'wgconfig' },
  { icon: Network, label: 'Networks', route: 'networks' },
];

export default function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const auth = getAuth();

  const handleLogout = async () => {
    try {
      await apiPost('/v3/user/logout', { LogoutToken: auth?.DeviceToken, All: false });
    } catch {
      // ignore
    }
    clearAuth();
    navigate('/login', { replace: true });
  };

  const hash = location.hash.replace('#', '') || '/';
  const currentRoute = hash.split('/')[1] || '';

  return (
    <div className="flex min-h-screen bg-[#060810]">
      {/* Sidebar */}
      <div className="w-[200px] shrink-0 bg-[#0a0d14] border-r border-[#1e2433] flex flex-col">
        <div className="h-12 flex items-center px-4 border-b border-[#1e2433] shrink-0">
          <span className="text-[13px] font-semibold text-white">Tunnels Admin</span>
        </div>

        <nav className="flex-1 py-3 space-y-0.5 px-2">
          {navItems.map((item) => {
            const isActive = currentRoute === item.route;
            return (
              <button
                key={item.route}
                onClick={() => navigate('/' + item.route)}
                className={`flex items-center w-full gap-3 px-3 py-1.5 rounded-md text-[13px] font-medium transition-colors ${
                  isActive
                    ? 'bg-[#4B7BF5]/10 text-[#4B7BF5]'
                    : 'text-white/50 hover:text-white/80 hover:bg-white/[0.03]'
                }`}
              >
                <item.icon
                  className={`w-4 h-4 shrink-0 ${isActive ? 'text-[#4B7BF5]' : 'text-white/40'}`}
                />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div className="border-t border-[#1e2433] p-2">
          {auth && (
            <div className="px-3 py-1 mb-1">
              <div className="text-[11px] text-white/40 truncate">{auth.Email}</div>
            </div>
          )}
          <button
            onClick={handleLogout}
            className="flex items-center w-full gap-3 px-3 py-1.5 rounded-md text-[13px] text-white/50 hover:text-white/80 hover:bg-white/[0.03] transition-colors"
          >
            <LogOut className="w-4 h-4 text-white/40" />
            Logout
          </button>
        </div>
      </div>

      {/* Main content */}
      <main className="flex-1 overflow-auto px-6 py-5">
        <Outlet />
      </main>
    </div>
  );
}
