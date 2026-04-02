import { useState, useEffect } from 'react';
import { Bell, Trash2, AlertTriangle, AlertCircle } from 'lucide-react';
import { fetchApi } from '../api';
import { Drawer } from './Modal';

export function SystemAlerts() {
  const [alerts, setAlerts] = useState<any[]>([]);
  const [open, setOpen] = useState(false);

  const loadAlerts = async () => {
    try {
      const data: any = await fetchApi('/system-alerts');
      setAlerts(data.items || []);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    loadAlerts();
    const interval = setInterval(loadAlerts, 10000); // Pool every 10s
    return () => clearInterval(interval);
  }, []);

  const clearAlerts = async () => {
    try {
      await fetchApi('/system-alerts', { method: 'DELETE' });
      setAlerts([]);
      setOpen(false);
    } catch (err) {
      alert(err);
    }
  };

  if (alerts.length === 0) return null;

  return (
    <>
      <div style={{ position: 'absolute', top: 24, right: 32, zIndex: 10 }}>
        <button 
          className="btn btn-ghost" 
          onClick={() => setOpen(true)}
          style={{ position: 'relative', background: 'var(--bg-surface)', border: '1px solid var(--border-color)', borderRadius: '50%', width: 48, height: 48, padding: 0, justifyContent: 'center' }}
        >
          <Bell size={20} color="var(--primary)" />
          <div style={{ position: 'absolute', top: -4, right: -4, background: 'var(--danger)', color: 'white', fontSize: 11, fontWeight: 'bold', width: 22, height: 22, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 4px 8px rgba(255,59,48,0.3)' }}>
            {alerts.length > 99 ? '99+' : alerts.length}
          </div>
        </button>
      </div>

      <Drawer isOpen={open} onClose={() => setOpen(false)} title="System Alerts" width={450}>
        <div className="flex-col gap-3">
          <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', paddingBottom: 16, borderBottom: '1px solid var(--border-color)' }}>
            <p style={{ opacity: 0.7, fontSize: 14 }}>{alerts.length} unread alerts</p>
            <button className="btn btn-ghost text-danger" onClick={clearAlerts} style={{ padding: '6px 12px' }}>
              <Trash2 size={16} /> Clear All
            </button>
          </div>

          <div className="flex-col gap-3" style={{ overflowY: 'auto', maxHeight: 'calc(100vh - 150px)' }}>
            {alerts.map((alert: any) => (
              <div key={alert.id} style={{ padding: 16, borderRadius: 'var(--radius-md)', background: alert.level === 'error' ? 'rgba(255, 59, 48, 0.1)' : 'rgba(255, 204, 0, 0.1)', border: `1px solid ${alert.level === 'error' ? 'var(--danger)' : 'var(--warning)'}` }}>
                <div className="flex-row gap-2" style={{ alignItems: 'flex-start' }}>
                  {alert.level === 'error' ? <AlertCircle size={18} color="var(--danger)" style={{ marginTop: 2 }} /> : <AlertTriangle size={18} color="var(--warning)" style={{ marginTop: 2 }} />}
                  <div className="flex-col gap-1">
                    <p style={{ fontSize: 14, lineHeight: 1.4, color: alert.level === 'error' ? 'var(--danger)' : 'var(--warning)', fontWeight: 500 }}>{alert.message}</p>
                    <span style={{ fontSize: 12, opacity: 0.6, fontFamily: 'monospace' }}>{new Date(alert.created_at).toLocaleString()}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </Drawer>
    </>
  );
}
