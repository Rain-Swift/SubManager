import { useEffect, useState } from 'react';
import { fetchApi } from '../api';
import { Activity } from 'lucide-react';

export default function Dashboard() {
  const [stats, setStats] = useState<any>(null);

  useEffect(() => {
    // We fetch a few resources concurrently to build the dashboard
    Promise.all([
      fetchApi('/subscriptions').catch(() => ({ items: [] })),
      fetchApi('/rules').catch(() => ({ items: [] })),
      fetchApi('/build-profiles').catch(() => ({ items: [] }))
    ]).then(([subs, rules, profiles]: any) => {
      setStats({
        subsCount: (subs.items || []).length,
        rulesCount: (rules.items || []).length,
        profilesCount: (profiles.items || []).length,
      });
    });
  }, []);

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Overview</h1>
          <p>Welcome back. Here is the status of your subscriptions.</p>
        </div>
      </div>

      {!stats ? (
        <div style={{ padding: 40 }} className="flex-center">
          <Activity className="lucide-spin text-secondary" />
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 24 }}>
          
          <div className="glass glass-card">
            <h3 style={{ color: 'var(--text-secondary)', fontSize: 14, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em' }}>Subscriptions</h3>
            <div style={{ fontSize: 48, fontWeight: 700, margin: '16px 0 8px', color: 'var(--primary)' }}>
              {stats.subsCount}
            </div>
            <p style={{ fontSize: 13 }}>Active subscription sources</p>
          </div>

          <div className="glass glass-card">
            <h3 style={{ color: 'var(--text-secondary)', fontSize: 14, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em' }}>Rules</h3>
            <div style={{ fontSize: 48, fontWeight: 700, margin: '16px 0 8px', color: 'var(--secondary)' }}>
              {stats.rulesCount}
            </div>
            <p style={{ fontSize: 13 }}>Configured rule sets</p>
          </div>

          <div className="glass glass-card">
            <h3 style={{ color: 'var(--text-secondary)', fontSize: 14, fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em' }}>Profiles</h3>
            <div style={{ fontSize: 48, fontWeight: 700, margin: '16px 0 8px', color: 'var(--accent)' }}>
              {stats.profilesCount}
            </div>
            <p style={{ fontSize: 13 }}>Active build profiles</p>
          </div>

        </div>
      )}
    </div>
  );
}
