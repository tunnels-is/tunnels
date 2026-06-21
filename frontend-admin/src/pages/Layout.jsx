import { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Users, Monitor, Layers, Server, Globe, LogOut, Sun, Moon } from 'lucide-react';
import { getUserMeta, clearUserMeta } from '../auth';
import { apiPost } from '../api';
import { getTheme, setTheme as applyAppTheme } from '../theme';

const navItems = [
  { icon: Users, label: 'Users', route: 'users' },
  { icon: Monitor, label: 'Devices', route: 'devices' },
  { icon: Layers, label: 'Groups', route: 'groups' },
  { icon: Server, label: 'Servers', route: 'servers' },
  { icon: Globe, label: 'WANs', route: 'wans' },
];

export default function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const meta = getUserMeta();
  const [theme, setTheme] = useState(() => getTheme());

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    applyAppTheme(next);
  };

  const handleLogout = async () => {
    try {
      await apiPost('/ui/user/logout', {});
    } catch {
      // ignore
    }
    clearUserMeta();
    navigate('/login', { replace: true });
  };

  const hash = location.hash.replace('#', '') || '/';
  const currentRoute = hash.split('/')[1] || '';

  return (
    <div className="flex min-h-screen bg-[#fdfcf8]">
      {/* Sidebar */}
      <div className="w-[220px] shrink-0 bg-white border-r border-[#e7e3d7] flex flex-col">
        <div className="h-14 flex items-center px-5 border-b border-[#e7e3d7] shrink-0">
          <div className="flex items-center gap-1">
            <span className="text-[14px] font-semibold tracking-tight text-[#0a0a0a]">Tunnels Admin</span>
            <span className="w-1 h-1 rounded-full bg-[#0a0a0a]" />
          </div>
        </div>

        <nav className="flex-1 py-3 space-y-0.5 px-2">
          {navItems.map((item) => {
            const isActive = currentRoute === item.route;
            return (
              <button
                key={item.route}
                onClick={() => navigate('/' + item.route)}
                className={`flex items-center w-full gap-3 px-3 py-2 rounded-md text-[13px] font-medium transition-colors ${
                  isActive
                    ? 'bg-black/[0.05] text-[#0a0a0a]'
                    : 'text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.03]'
                }`}
              >
                <item.icon
                  className={`w-4 h-4 shrink-0 ${isActive ? 'text-[#0a0a0a]' : 'text-[#a3a3a3]'}`}
                />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div className="border-t border-[#e7e3d7] p-2">
          {meta && (
            <div className="px-3 py-1.5 mb-1">
              <div className="text-[10px] text-[#737373] uppercase tracking-[0.12em]">Signed in</div>
              <div className="text-[12px] text-[#0a0a0a] truncate mt-0.5">{meta.Email}</div>
            </div>
          )}
          <button
            onClick={handleLogout}
            className="flex items-center w-full gap-3 px-3 py-2 rounded-md text-[13px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.03] transition-colors"
          >
            <LogOut className="w-4 h-4 text-[#a3a3a3]" />
            Logout
          </button>
          <button
            onClick={toggleTheme}
            className="flex items-center w-full gap-3 px-3 py-2 rounded-md text-[13px] text-[#525252] hover:text-[#0a0a0a] hover:bg-black/[0.03] transition-colors"
            title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
          >
            {theme === 'dark' ? (
              <Sun className="w-4 h-4 text-[#a3a3a3]" />
            ) : (
              <Moon className="w-4 h-4 text-[#a3a3a3]" />
            )}
            {theme === 'dark' ? 'Light theme' : 'Dark theme'}
          </button>
        </div>
      </div>

      {/* Main content */}
      <main className="flex-1 overflow-auto px-8 py-6">
        <Outlet />
      </main>
    </div>
  );
}
