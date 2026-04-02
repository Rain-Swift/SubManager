import { Link, useLocation } from 'react-router-dom';
import { Home, Rss, BookOpen, Layers, Key, LogOut } from 'lucide-react';
import { SystemAlerts } from './components/SystemAlerts';
import { logout } from './api';

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();

  const links = [
    { name: 'Overview', path: '/', icon: Home },
    { name: 'Subscriptions', path: '/subscriptions', icon: Rss },
    { name: 'Rules', path: '/rules', icon: BookOpen },
    { name: 'Build Profiles', path: '/profiles', icon: Layers },
    { name: 'Tokens', path: '/tokens', icon: Key },
  ];

  return (
    <div className="app-layout">
      <div className="app-sidebar">
        <div style={{ padding: '0 24px', marginBottom: 40 }}>
          <h2 style={{ fontSize: 20, fontWeight: 700, color: 'white', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Layers color="var(--primary)" />
            SubManager
          </h2>
        </div>

        <nav className="flex-col gap-2" style={{ padding: '0 16px', flex: 1 }}>
          {links.map(link => {
            const active = location.pathname === link.path;
            const Icon = link.icon;
            return (
              <Link 
                key={link.path}
                to={link.path} 
                className="flex-row gap-3"
                style={{ 
                  color: active ? 'white' : 'var(--text-secondary)',
                  padding: '12px 16px',
                  borderRadius: 'var(--radius-md)',
                  background: active ? 'var(--bg-surface-active)' : 'transparent',
                  fontWeight: active ? 600 : 500,
                  transition: 'all 0.2s',
                  borderLeft: active ? '3px solid var(--primary)' : '3px solid transparent'
                }}
              >
                <Icon size={20} color={active ? 'var(--primary)' : 'currentColor'} />
                {link.name}
              </Link>
            )
          })}
        </nav>

        <div style={{ padding: '24px 16px' }}>
          <button className="btn btn-ghost" style={{ width: '100%', justifyContent: 'flex-start' }} onClick={logout}>
            <LogOut size={18} />
            Logout
          </button>
        </div>
      </div>

      <div className="app-main" style={{ position: 'relative' }}>
        <SystemAlerts />
        {children}
      </div>
    </div>
  );
}
