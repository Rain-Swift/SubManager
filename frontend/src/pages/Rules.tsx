import { useState, useEffect } from 'react';
import { fetchApi } from '../api';
import { RefreshCw, Plus, Settings2, Trash2 } from 'lucide-react';
import { Drawer } from '../components/Modal';
import { InputField, Switch, KeyValueInput } from '../components/Forms';

export default function Rules() {
  const [items, setItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [isDrawerOpen, setDrawerOpen] = useState(false);
  const [formData, setFormData] = useState<any>({});
  const [saving, setSaving] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const data: any = await fetchApi('/rules');
      setItems(data.items || []);
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
      await fetchApi(`/rules/${id}/refresh`, { method: 'POST' });
      setTimeout(loadData, 1000);
    } catch (err) {
      alert(err);
    }
  };

  const handleDelete = async (id: string, e?: any) => {
    if (e) e.stopPropagation();
    if (!window.confirm("Are you sure you want to delete this rule source?")) return;
    try {
      await fetchApi(`/rules/${id}`, { method: 'DELETE' });
      loadData();
    } catch (err: any) { alert(err.message); }
  };

  const openCreate = () => {
    setFormData({
      name: '', url: '', mode: 'link_only', headers: {}, enabled: true, 
      timeout_sec: 15, user_agent: '', retry_attempts: 3, retry_backoff_ms: 1000, 
      min_fetch_interval_sec: 3600, cache_ttl_seconds: 3600, refresh_interval_sec: 86400
    });
    setEditingId(null);
    setDrawerOpen(true);
  };

  const openEdit = (item: any) => {
    setFormData({ ...item });
    setEditingId(item.id);
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = {
        name: formData.name, url: formData.url, mode: formData.mode, headers: formData.headers || {},
        enabled: formData.enabled, timeout_sec: Number(formData.timeout_sec),
        user_agent: formData.user_agent, retry_attempts: Number(formData.retry_attempts),
        retry_backoff_ms: Number(formData.retry_backoff_ms),
        min_fetch_interval_sec: Number(formData.min_fetch_interval_sec),
        cache_ttl_seconds: Number(formData.cache_ttl_seconds),
        refresh_interval_sec: Number(formData.refresh_interval_sec)
      };

      if (editingId) {
        await fetchApi(`/rules/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      } else {
        await fetchApi(`/rules`, { method: 'POST', body: JSON.stringify(payload) });
      }
      setDrawerOpen(false);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Rule Sources</h1>
          <p>Manage rule sets and routing configurations.</p>
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
        {loading && items.length === 0 ? (
          <div className="flex-center" style={{ padding: 40 }}><RefreshCw className="lucide-spin text-secondary" /></div>
        ) : items.map(item => (
          <div key={item.id} className="glass glass-card flex-row" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
            <div className="flex-col gap-1">
              <div className="flex-row gap-3" style={{ alignItems: 'center' }}>
                <h3 style={{ fontSize: 16, margin: 0 }}>{item.name}</h3>
                <span className="badge badge-primary">{item.mode}</span>
                {item.enabled ? <span className="badge badge-success">Enabled</span> : <span className="badge badge-warning">Disabled</span>}
                {item.status === 'running' && <span className="badge badge-primary"><RefreshCw size={12} className="lucide-spin" style={{ marginRight: 4 }}/> Refreshing</span>}
                {item.status === 'failed' && <span className="badge badge-danger">Failed</span>}
              </div>
              <p style={{ fontSize: 13, fontFamily: 'monospace', opacity: 0.7, marginTop: 4 }}>{item.url}</p>
            </div>
            
            <div className="flex-row gap-2">
              <button className="btn btn-ghost" onClick={() => openEdit(item)} style={{ padding: 8 }}><Settings2 size={18} /></button>
              <button className="btn btn-ghost text-danger" onClick={(e) => handleDelete(item.id, e)} style={{ padding: 8 }}>
                <Trash2 size={18} />
              </button>
              <button 
                className="btn btn-ghost" 
                onClick={(e) => handleRefresh(item.id, e)}
                disabled={item.status === 'running' || item.mode === 'link_only'}
              >
                <RefreshCw size={16} className={item.status === 'running' ? 'lucide-spin' : ''} />
                Sync
              </button>
            </div>
          </div>
        ))}
      </div>

      <Drawer isOpen={isDrawerOpen} onClose={() => setDrawerOpen(false)} title={editingId ? 'Edit Rule Source' : 'New Rule Source'} width={800}>
        <div className="flex-col gap-2">
          <Switch label="Enabled" checked={formData.enabled} onChange={(v: boolean) => setFormData({...formData, enabled: v})} />
          <InputField label="Name" value={formData.name || ''} onChange={(e: any) => setFormData({...formData, name: e.target.value})} required />
          <InputField label="URL" value={formData.url || ''} onChange={(e: any) => setFormData({...formData, url: e.target.value})} required />
          
          <InputField label="Mode" type="select" value={formData.mode || 'link_only'} onChange={(e: any) => setFormData({...formData, mode: e.target.value})}>
            <option value="link_only">Link Only (Injects rule-provider entry)</option>
            <option value="fetch_text">Fetch Text (Downloads and inline embeds rules)</option>
          </InputField>

          <InputField label="User Agent" placeholder="Leave empty for default" value={formData.user_agent || ''} onChange={(e: any) => setFormData({...formData, user_agent: e.target.value})} />
          <KeyValueInput label="Custom Headers" value={formData.headers || {}} onChange={(v) => setFormData({...formData, headers: v})} />
          
          <div className="flex-row gap-3">
            <InputField label="Timeout (s)" type="number" value={formData.timeout_sec || 0} onChange={(e: any) => setFormData({...formData, timeout_sec: e.target.value})} />
            <InputField label="Retry Attempts" type="number" value={formData.retry_attempts || 0} onChange={(e: any) => setFormData({...formData, retry_attempts: e.target.value})} />
          </div>
          
          <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
            <button className="btn btn-ghost" onClick={() => setDrawerOpen(false)}>Cancel</button>
            <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save Rule'}</button>
          </div>
        </div>
      </Drawer>
    </div>
  );
}
