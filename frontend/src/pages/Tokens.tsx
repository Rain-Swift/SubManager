import { useState, useEffect } from 'react';
import { fetchApi } from '../api';
import { Key, Eye, Settings2, Plus, X, Trash2, Activity, Clock, Layers, ScrollText } from 'lucide-react';
import { Drawer, Modal } from '../components/Modal';
import { InputField, Switch, Tabs, StringArrayInput } from '../components/Forms';

function formatRelative(iso: string | Date): string {
  const d = new Date(iso);
  const diff = Math.floor((Date.now() - d.getTime()) / 1000);
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return d.toLocaleDateString();
}

export default function Tokens() {
  const [items, setItems] = useState<any[]>([]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [subs, setSubs] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeTab, setActiveTab] = useState('General');
  const [saving, setSaving] = useState(false);
  
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewData, setPreviewData] = useState<any>(null);
  const [logOpen, setLogOpen] = useState(false);
  const [logItem, setLogItem] = useState<any>(null);

  const [fv, setFv] = useState<any>({
    name: '', build_profile_id: '', enabled: true, prebuild: false, distribution: {}
  });
  const loadData = async () => {
    setLoading(true);
    try {
      const [data, pData, sData]: any = await Promise.all([
        fetchApi('/download-tokens'), fetchApi('/build-profiles'),
        fetchApi('/subscriptions')
      ]);
      setItems(data.items || []);
      setProfiles(pData.items || []);
      setSubs(sData.items || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadData(); }, []);

  const handlePreview = async (id: string, e: any) => {
    e.stopPropagation();
    try {
      const data = await fetchApi(`/download-tokens/${id}/preview`);
      setPreviewData(data);
      setPreviewOpen(true);
    } catch (err) { alert(err); }
  };

  const handleDelete = async (id: string, e?: any) => {
    if (e) e.stopPropagation();
    if (!window.confirm("Are you sure you want to delete this token? Clients will instantly lose access.")) return;
    try {
      await fetchApi(`/download-tokens/${id}`, { method: 'DELETE' });
      loadData();
    } catch (err: any) { alert(err.message); }
  };

  const openDrawer = (item?: any) => {
    setActiveTab('General');
    if (item) {
      setFv({ ...item, distribution: item.distribution || {} });
      setEditingId(item.id);
    } else {
      setFv({
        name: '', build_profile_id: profiles.length ? profiles[0].id : '', enabled: true, prebuild: false,
        distribution: { subscription_source_ids: [], include_proxy_patterns: [], exclude_proxy_patterns: [], filters: [], renames: [], override_rule_bindings: false, rule_bindings: [], override_groups: false, groups: [], default_group: '', template_override: null }
      });
      setEditingId(null);
    }
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = {
        name: fv.name,
        enabled: fv.enabled,
        prebuild: fv.prebuild,
        distribution: fv.distribution,
        build_profile_id: fv.build_profile_id
      };
      if (editingId) {
        // Can't update BuildProfileID for existing tokens
        await fetchApi(`/download-tokens/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      } else {
        const data: any = await fetchApi(`/download-tokens`, { method: 'POST', body: JSON.stringify(payload) });
        window.prompt("Token created! This is the ONLY time you'll see it. Please copy it:", data.token);
      }
      setDrawerOpen(false);
      loadData();
    } catch (err: any) { alert(err.message || 'Save failed'); } finally { setSaving(false); }
  };

  // Setup deep update helpers
  const dist = fv.distribution || {};
  const setDist = (val: any) => setFv({ ...fv, distribution: val });

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Download Tokens</h1>
          <p>Tokens control public access to compiled profiles with optional overrides.</p>
        </div>
        <button className="btn btn-primary" onClick={() => openDrawer()}><Plus size={16} /> New Token</button>
      </div>

      <div className="flex-col gap-4">
        {loading && items.length === 0 ? (
          <div className="flex-center" style={{ padding: 40 }}><Key className="lucide-spin text-secondary" /></div>
        ) : items.map(item => (
          <div key={item.id} className="glass glass-card flex-col gap-4">
            {/* ─── Header row ─── */}
            <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <div className="flex-col gap-1">
                <div className="flex-row gap-3" style={{ alignItems: 'center' }}>
                  <h3 style={{ fontSize: 16, margin: 0 }}>{item.name}</h3>
                  {item.enabled ? <span className="badge badge-success">Active</span> : <span className="badge badge-warning">Disabled</span>}
                  {item.prebuild && <span className="badge badge-primary">Prebuild</span>}
                </div>
                <p style={{ fontSize: 13, color: 'var(--text-tertiary)', marginTop: 2 }}>Profile: {profiles.find((p: any) => p.id === item.build_profile_id)?.name || item.build_profile_id}</p>
              </div>

              <div className="flex-row gap-2" style={{ alignItems: 'center' }}>
                <div className="flex-row" style={{ alignItems: 'center', background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 16, padding: '4px 12px', gap: 6 }} title="The full token was only shown during creation.">
                  <span style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--text-secondary)' }}>
                    /subscribe/{item.token_prefix}*****
                  </span>
                </div>
                <button className="btn btn-ghost" onClick={() => openDrawer(item)} style={{ padding: 8 }} title="Edit Settings"><Settings2 size={18} /></button>
                <button className="btn btn-ghost text-danger" onClick={(e) => handleDelete(item.id, e)} style={{ padding: 8 }} title="Delete Token"><Trash2 size={18} /></button>
                <button className="btn btn-ghost" onClick={(e) => handlePreview(item.id, e)} style={{ padding: 8 }} title="Preview Nodes & Diff"><Eye size={18} /></button>
              </div>
            </div>

            {/* ─── Stats row ─── */}
            <div className="flex-row gap-4" style={{ paddingTop: 12, borderTop: '1px solid var(--border-subtle)', flexWrap: 'wrap' }}>
              {/* Fetch count */}
              <div className="flex-row gap-2" style={{ alignItems: 'center' }}>
                <Activity size={14} color="var(--primary)" />
                <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                  {item.fetch_count > 0 ? <><span style={{ fontWeight: 600, color: 'white' }}>{item.fetch_count.toLocaleString()}</span> fetches</> : 'Never fetched'}
                </span>
              </div>

              {/* Last used */}
              <div className="flex-row gap-2" style={{ alignItems: 'center' }}>
                <Clock size={14} color="var(--text-tertiary)" />
                <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                  {item.last_used_at
                    ? <>Last pull <span style={{ fontWeight: 600, color: 'white' }}>{formatRelative(item.last_used_at)}</span></>
                    : 'No pulls yet'}
                </span>
              </div>

              {/* Config build info from cached artifact */}
              {item.cached_artifact && (
                <div className="flex-row gap-2" style={{ alignItems: 'center' }}>
                  <Layers size={14} color="var(--success)" />
                  <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                    Config: <span style={{ fontWeight: 600, color: 'white' }}>{item.cached_artifact.summary?.output_proxy_count ?? '?'} nodes</span>
                    {' · '}Built {formatRelative(item.cached_artifact.last_built_at)}
                  </span>
                </div>
              )}

              {/* Access log button */}
              {item.access_log?.length > 0 && (
                <button
                  className="btn btn-ghost"
                  style={{ padding: '2px 10px', fontSize: 12, marginLeft: 'auto', color: 'var(--text-secondary)' }}
                  onClick={() => { setLogItem(item); setLogOpen(true); }}
                >
                  <ScrollText size={13} /> View Pull History ({item.access_log.length})
                </button>
              )}
            </div>
          </div>
        ))}
      </div>


      <Drawer isOpen={drawerOpen} onClose={() => setDrawerOpen(false)} title={editingId ? 'Edit Token' : 'New Download Token'} width={1100}>
        <Tabs tabs={['General', 'Distribution Targets', 'Distribution Processors', 'Distribution Config']} activeTab={activeTab} onChange={setActiveTab} />
        
        <div style={{ flex: 1 }}>
          {activeTab === 'General' && (
            <div className="flex-col gap-2">
              <Switch label="Enabled" checked={fv.enabled} onChange={(v:any) => setFv({...fv, enabled: v})} />
              <Switch label="Prebuild (Async Build)" description="Build ahead of time for fast client downloads." checked={fv.prebuild} onChange={(v:any) => setFv({...fv, prebuild: v})} />
              <InputField label="Name" value={fv.name} onChange={(e:any) => setFv({...fv, name: e.target.value})} required />
              {!editingId && (
                <InputField label="Target Build Profile" type="select" value={fv.build_profile_id} onChange={(e:any) => setFv({...fv, build_profile_id: e.target.value})}>
                  <option value="">Select Profile</option>
                  {profiles.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
                </InputField>
              )}
            </div>
          )}

          {activeTab === 'Distribution Targets' && (
             <div className="flex-col gap-4">
                <div style={{ marginBottom: 16 }}>
                  <label className="input-label">Filter Subs (Override Base Profile Subs)</label>
                  <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>Select the specific subscriptions to use from the underlying profile. Leave empty to use all of them.</p>
                  <div className="flex-col gap-2" style={{ background: 'var(--bg-surface)', padding: 12, borderRadius: 8, maxHeight: 200, overflowY: 'auto' }}>
                    {subs.map(s => (
                      <label key={s.id} className="flex-row gap-2" style={{ alignItems: 'center', cursor: 'pointer' }}>
                        <input type="checkbox" checked={(dist.subscription_source_ids || []).includes(s.id)} onChange={e => {
                          const ids = new Set(dist.subscription_source_ids || []);
                          if (e.target.checked) ids.add(s.id); else ids.delete(s.id);
                          setDist({...dist, subscription_source_ids: Array.from(ids)});
                        }} />
                        {s.name}
                      </label>
                    ))}
                  </div>
                </div>

                <StringArrayInput label="Include Proxy Patterns (Regex)" placeholder="E.g. .*SG.*" value={dist.include_proxy_patterns || []} onChange={(v) => setDist({...dist, include_proxy_patterns: v})} />
                <StringArrayInput label="Exclude Proxy Patterns (Regex)" placeholder="E.g. .*EXPIRE.*" value={dist.exclude_proxy_patterns || []} onChange={(v) => setDist({...dist, exclude_proxy_patterns: v})} />
             </div>
          )}

          {activeTab === 'Distribution Processors' && (
            <div className="flex-col gap-6">
               <p style={{ fontSize: 13, color: 'var(--text-secondary)' }}>These processors run strictly for THIS token distribution, after the base profile processors.</p>
               <div>
                  <label className="input-label">Token Specific Renames</label>
                  <button className="btn btn-ghost" style={{ padding: '0 8px', fontSize: 12 }} onClick={() => setDist({...dist, renames: [...(dist.renames||[]), {pattern: '', replace: ''}]})}>+ Add Rename</button>
                  <div className="flex-col gap-2" style={{ marginTop: 8 }}>
                    {(dist.renames||[]).map((r:any, i:number) => (
                      <div key={i} className="flex-row gap-2">
                        <input className="input-field" value={r.pattern} onChange={e => { const copy = [...dist.renames]; copy[i].pattern = e.target.value; setDist({...dist, renames: copy})}} placeholder="Regex..." style={{ flex: 1 }} />
                        <input className="input-field" value={r.replace} onChange={e => { const copy = [...dist.renames]; copy[i].replace = e.target.value; setDist({...dist, renames: copy})}} placeholder="Replacement..." style={{ flex: 1 }} />
                        <button className="btn btn-ghost" style={{ color: 'var(--danger)' }} onClick={() => setDist({...dist, renames: dist.renames.filter((_:any, idx:number) => idx !== i)})}><X size={16}/></button>
                      </div>
                    ))}
                  </div>
               </div>
            </div>
          )}

          {activeTab === 'Distribution Config' && (
             <div className="flex-col gap-4">
                 <Switch label="Override Groups Base Configuration" checked={dist.override_groups} onChange={(v:any) => setDist({...dist, override_groups: v})} description="Check this to fully replace the profile policy groups. Implementing the form is TBD for UI space." />
                 
                 <div style={{ padding: 16, border: '1px dashed var(--border-strong)', borderRadius: 8 }}>
                    <label className="input-label">Template Override</label>
                    <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 12 }}>You can inject distribution specific overrides (e.g. override DNS or local port just for this user token).</p>
                    <Switch label="Enable Template Override block" checked={dist.template_override != null} onChange={(v:any) => setDist({...dist, template_override: v ? {} : null})} />
                    
                    {dist.template_override != null && (
                      <div className="flex-col gap-2 mt-2">
                        <InputField label="Override Mode" type="select" value={dist.template_override.mode || ''} onChange={(e:any) => {
                          const v = e.target.value;
                          setDist({...dist, template_override: {...dist.template_override, mode: v ? v : undefined}});
                        }}>
                          <option value="">(Inherit)</option><option value="rule">Rule</option><option value="global">Global</option>
                        </InputField>
                        <InputField label="Override Mixed Port" type="number" placeholder="(Inherit)" value={dist.template_override.mixed_port || ''} onChange={(e:any) => {
                          const v = parseInt(e.target.value);
                          setDist({...dist, template_override: {...dist.template_override, mixed_port: isNaN(v) ? undefined : v}});
                        }} />
                      </div>
                    )}
                 </div>
             </div>
          )}

        </div>

        <div style={{ marginTop: 24, paddingTop: 16, borderTop: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
          <button className="btn btn-ghost" onClick={() => setDrawerOpen(false)}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save Token'}</button>
        </div>
      </Drawer>

      <Modal isOpen={previewOpen} onClose={() => setPreviewOpen(false)} title="Token Build Preview" width={800}>
         {previewData && (
            <div className="flex-col gap-4">
                <div className="flex-row gap-4">
                  <div className="glass glass-card" style={{ flex: 1, padding: 16 }}>
                    <h4 style={{ margin: '0 0 12px' }}>Base Profile Proxies</h4>
                    <span style={{ fontSize: 32, fontWeight: 'bold' }}>{previewData.diff?.base_summary?.output_proxy_count || 0}</span>
                  </div>
                  <div className="glass glass-card" style={{ flex: 1, padding: 16, borderColor: 'var(--primary)' }}>
                    <h4 style={{ margin: '0 0 12px', color: 'var(--primary)' }}>Token Distributed Proxies</h4>
                    <span style={{ fontSize: 32, fontWeight: 'bold', color: 'var(--primary)' }}>{previewData.diff?.token_summary?.output_proxy_count || 0}</span>
                  </div>
                </div>

                <div className="glass glass-card" style={{ padding: 16 }}>
                    <h4 style={{ margin: '0 0 12px' }}>Proxy Overrides diff</h4>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                      <div>
                        <strong style={{ color: 'var(--success)', display: 'block', marginBottom: 8 }}>+ Added/Included ({previewData.diff?.added_proxy_names?.length || 0})</strong>
                        <div style={{ maxHeight: 200, overflowY: 'auto', fontSize: 12, fontFamily: 'monospace' }}>
                          {(previewData.diff?.added_proxy_names || []).map((n:any, i:number) => <div key={i}>{n}</div>)}
                        </div>
                      </div>
                      <div>
                         <strong style={{ color: 'var(--danger)', display: 'block', marginBottom: 8 }}>- Removed/Excluded ({previewData.diff?.removed_proxy_names?.length || 0})</strong>
                         <div style={{ maxHeight: 200, overflowY: 'auto', fontSize: 12, fontFamily: 'monospace' }}>
                          {(previewData.diff?.removed_proxy_names || []).map((n:any, i:number) => <div key={i}>{n}</div>)}
                        </div>
                      </div>
                    </div>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16 }}>
                   <button className="btn btn-ghost" onClick={() => setPreviewOpen(false)}>Close</button>
                </div>
            </div>
         )}
      </Modal>

      {/* ─── Pull History Drawer ─── */}
      <Drawer isOpen={logOpen} onClose={() => setLogOpen(false)} title={`Pull History — ${logItem?.name || ''}`} width={480}>
        {logItem && (
          <div className="flex-col gap-2">
            <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', marginBottom: 8, paddingBottom: 12, borderBottom: '1px solid var(--border-subtle)' }}>
              <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                <strong style={{ color: 'white' }}>{logItem.fetch_count?.toLocaleString() ?? 0}</strong> total fetches
              </span>
              <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                Last: {logItem.last_used_at ? formatRelative(logItem.last_used_at) : 'Never'}
              </span>
            </div>
            <div className="flex-col gap-2" style={{ overflowY: 'auto', maxHeight: 'calc(100vh - 180px)' }}>
              {[...(logItem.access_log || [])].reverse().map((entry: any, i: number) => (
                <div key={i} className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', padding: '8px 12px', background: 'var(--bg-surface)', borderRadius: 6, border: '1px solid var(--border-subtle)' }}>
                  <div className="flex-row gap-2" style={{ alignItems: 'center' }}>
                    <Activity size={13} color="var(--primary)" />
                    <span style={{ fontSize: 13, fontFamily: 'monospace', color: entry.ip ? 'white' : 'var(--text-tertiary)' }}>
                      {entry.ip || 'unknown'}
                    </span>
                  </div>
                  <span style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>
                    {new Date(entry.at).toLocaleString()}
                  </span>
                </div>
              ))}
              {(logItem.access_log?.length ?? 0) === 0 && (
                <p style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: 24 }}>No pull history recorded.</p>
              )}
            </div>
          </div>
        )}
      </Drawer>

    </div>
  );
}
