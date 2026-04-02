import { useState, useEffect } from 'react';
import { fetchApi } from '../api';
import { RefreshCw, Clock, Plus, Settings2, Upload, Trash2 } from 'lucide-react';
import { Drawer } from '../components/Modal';
import { InputField, Switch, KeyValueInput } from '../components/Forms';

export default function Subscriptions() {
  const [subs, setSubs] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [isDrawerOpen, setDrawerOpen] = useState(false);
  const [formData, setFormData] = useState<any>({});
  const [saving, setSaving] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const data: any = await fetchApi('/subscriptions');
      setSubs(data.items || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleRefresh = async (id: string, e?: any) => {
    if (e) e.stopPropagation();
    try {
      await fetchApi(`/subscriptions/${id}/refresh`, { method: 'POST' });
      setTimeout(loadData, 1000);
    } catch (err) {
      alert(err);
    }
  };

  const handleDelete = async (id: string, e?: any) => {
    if (e) e.stopPropagation();
    if (!window.confirm("Are you sure you want to delete this subscription?")) return;
    try {
      await fetchApi(`/subscriptions/${id}`, { method: 'DELETE' });
      loadData();
    } catch (err: any) { alert(err.message); }
  };

  const openCreate = () => {
    setFormData({
      name: '', type: 'remote', url: '', payload: '', headers: {}, enabled: true, timeout_sec: 15, user_agent: '',
      retry_attempts: 3, retry_backoff_ms: 1000, min_fetch_interval_sec: 3600,
      cache_ttl_seconds: 3600, refresh_interval_sec: 86400
    });
    setEditingId(null);
    setDrawerOpen(true);
  };

  const openEdit = (sub: any) => {
    setFormData({ ...sub });
    setEditingId(sub.id);
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = {
        name: formData.name, type: formData.type || 'remote', url: formData.url, payload: formData.payload, headers: formData.headers || {},
        enabled: formData.enabled, timeout_sec: Number(formData.timeout_sec),
        user_agent: formData.user_agent, retry_attempts: Number(formData.retry_attempts),
        retry_backoff_ms: Number(formData.retry_backoff_ms),
        min_fetch_interval_sec: Number(formData.min_fetch_interval_sec),
        cache_ttl_seconds: Number(formData.cache_ttl_seconds),
        refresh_interval_sec: Number(formData.refresh_interval_sec)
      };

      if (editingId) {
        await fetchApi(`/subscriptions/${editingId}`, {
          method: 'PUT', body: JSON.stringify(payload)
        });
      } else {
        await fetchApi(`/subscriptions`, {
          method: 'POST', body: JSON.stringify(payload)
        });
      }
      setDrawerOpen(false);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleFileUpload = (e: any) => {
    const file = e.target.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      setFormData({ ...formData, payload: event.target?.result });
    };
    reader.readAsText(file);
  };

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Subscriptions</h1>
          <p>Manage your remote proxy sources.</p>
        </div>
        <div className="flex-row gap-3">
          <button className="btn btn-ghost" onClick={loadData}>
            <RefreshCw size={16} /> Reload
          </button>
          <button className="btn btn-primary" onClick={openCreate}>
            <Plus size={16} /> Add New
          </button>
        </div>
      </div>

      <div className="flex-col gap-4">
        {loading && subs.length === 0 ? (
          <div className="flex-center" style={{ padding: 40 }}><RefreshCw className="lucide-spin text-secondary" /></div>
        ) : subs.map(sub => (
          <div key={sub.id} className="glass glass-card flex-row" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
            <div className="flex-col gap-1">
              <div className="flex-row gap-3" style={{ alignItems: 'center' }}>
                <h3 style={{ fontSize: 16, margin: 0 }}>{sub.name}</h3>
                <span className="badge badge-primary">{sub.type === 'local' ? 'Local' : 'Remote'}</span>
                {sub.enabled ? (
                  <span className="badge badge-success">Enabled</span>
                ) : (
                  <span className="badge badge-warning">Disabled</span>
                )}
                {sub.status === 'running' && <span className="badge badge-primary"><RefreshCw size={12} className="lucide-spin" style={{ marginRight: 4 }}/> Refreshing</span>}
                {sub.status === 'failed' && <span className="badge badge-danger">Failed</span>}
              </div>
              {sub.type === 'local' ? (
                <p style={{ fontSize: 13, fontFamily: 'monospace', opacity: 0.7, marginTop: 4 }}>Local Configuration ({sub.payload?.length || 0} bytes)</p>
              ) : (
                <p style={{ fontSize: 13, fontFamily: 'monospace', opacity: 0.7, marginTop: 4 }}>{sub.url}</p>
              )}
              <div className="flex-row gap-4" style={{ fontSize: 12, color: 'var(--text-tertiary)', marginTop: 8 }}>
                <span className="flex-row gap-1" style={{ alignItems: 'center' }}><Clock size={12} /> Sync: {sub.refresh_interval_sec}s</span>
                {sub.last_fetched_at && <span>Last sync: {new Date(sub.last_fetched_at).toLocaleString()}</span>}
                {sub.last_error && <span style={{ color: 'var(--danger)' }}>Err: {sub.last_error}</span>}
              </div>
            </div>
            
            <div className="flex-row gap-2">
              <button className="btn btn-ghost" onClick={() => openEdit(sub)} style={{ padding: 8 }}>
                <Settings2 size={18} />
              </button>
              <button className="btn btn-ghost text-danger" onClick={(e) => handleDelete(sub.id, e)} style={{ padding: 8 }}>
                <Trash2 size={18} />
              </button>
              <button 
                className="btn btn-ghost" 
                onClick={(e) => handleRefresh(sub.id, e)}
                disabled={sub.status === 'running'}
              >
                <RefreshCw size={16} className={sub.status === 'running' ? 'lucide-spin' : ''} />
                Sync
              </button>
            </div>
          </div>
        ))}
        {!loading && subs.length === 0 && (
          <div className="glass glass-card flex-center flex-col" style={{ padding: '60px 20px', color: 'var(--text-secondary)' }}>
            <p>No subscriptions found.</p>
          </div>
        )}
      </div>

      <Drawer isOpen={isDrawerOpen} onClose={() => setDrawerOpen(false)} title={editingId ? 'Edit Subscription' : 'New Subscription'} width={800}>
        <div className="flex-col gap-2">
          <div className="flex-row gap-2" style={{ marginBottom: 12 }}>
            <button className={`btn ${formData.type !== 'local' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFormData({...formData, type: 'remote'})}>🌐 Remote URL</button>
            <button className={`btn ${formData.type === 'local' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFormData({...formData, type: 'local'})}>📝 Local Text</button>
          </div>
          
          <div className="flex-row gap-4">
            <InputField label="Name" value={formData.name || ''} onChange={(e: any) => setFormData({...formData, name: e.target.value})} required />
            <div style={{ flexShrink: 0, marginTop: 24 }}><Switch label="Enabled" checked={formData.enabled} onChange={(v: boolean) => setFormData({...formData, enabled: v})} /></div>
          </div>
          
          {formData.type === 'local' ? (
            <div className="flex-col gap-2 glass glass-card" style={{ padding: 16, marginTop: 12 }}>
              <div className="flex-row gap-2" style={{ alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontSize: 13, fontWeight: 500 }}>Configuration Content</span>
                <label className="btn btn-ghost" style={{ padding: '6px 12px', fontSize: 12, cursor: 'pointer' }}>
                  <Upload size={14} style={{ marginRight: 6 }} /> Upload YAML/TXT File
                  <input type="file" style={{ display: 'none' }} accept=".yaml,.yml,.txt" onChange={handleFileUpload} />
                </label>
              </div>
              <textarea 
                className="input" 
                style={{ height: 250, fontFamily: 'monospace', fontSize: 12, resize: 'vertical' }}
                placeholder="Paste your configuration content here..."
                value={formData.payload || ''}
                onChange={(e) => setFormData({...formData, payload: e.target.value})}
              />
            </div>
          ) : (
            <>
              <div className="glass glass-card flex-col gap-2" style={{ padding: 16, marginTop: 12 }}>
                <InputField label="URL" value={formData.url || ''} onChange={(e: any) => setFormData({...formData, url: e.target.value})} required />
                <InputField label="User Agent" placeholder="Leave empty for default" value={formData.user_agent || ''} onChange={(e: any) => setFormData({...formData, user_agent: e.target.value})} />
                
                <KeyValueInput label="Custom Headers" value={formData.headers || {}} onChange={(v) => setFormData({...formData, headers: v})} />
                
                <div className="flex-row gap-3">
                  <InputField label="Timeout (s)" type="number" value={formData.timeout_sec || 0} onChange={(e: any) => setFormData({...formData, timeout_sec: e.target.value})} />
                  <InputField label="Retry Attempts" type="number" value={formData.retry_attempts || 0} onChange={(e: any) => setFormData({...formData, retry_attempts: e.target.value})} />
                  <InputField label="Backoff (ms)" type="number" value={formData.retry_backoff_ms || 0} onChange={(e: any) => setFormData({...formData, retry_backoff_ms: e.target.value})} />
                </div>

                <div className="flex-row gap-3">
                  <InputField label="Min Fetch Interval (s)" type="number" value={formData.min_fetch_interval_sec || 0} onChange={(e: any) => setFormData({...formData, min_fetch_interval_sec: e.target.value})} />
                  <InputField label="Cache TTL (s)" type="number" value={formData.cache_ttl_seconds || 0} onChange={(e: any) => setFormData({...formData, cache_ttl_seconds: e.target.value})} />
                </div>
                
                <InputField label="Auto Refresh Interval (s)" type="number" value={formData.refresh_interval_sec || 0} onChange={(e: any) => setFormData({...formData, refresh_interval_sec: e.target.value})} />
              </div>
            </>
          )}

          <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
            <button className="btn btn-ghost" onClick={() => setDrawerOpen(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save Source'}</button>
          </div>
        </div>
      </Drawer>
    </div>
  );
}
