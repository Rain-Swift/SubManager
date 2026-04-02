import { useState, useEffect } from 'react';
import { fetchApi } from '../api';
import { Layers, Play, Settings2, Plus, X, Trash2, RefreshCw } from 'lucide-react';
import { Drawer } from '../components/Modal';
import { InputField, Switch, Tabs, StringArrayInput } from '../components/Forms';

export default function Profiles() {
  const [items, setItems] = useState<any[]>([]);
  const [subs, setSubs] = useState<any[]>([]);
  const [rules, setRules] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  
  const [editingId, setEditingId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeTab, setActiveTab] = useState('General');
  const [saving, setSaving] = useState(false);
  
  const [nodeSelectorOpen, setNodeSelectorOpen] = useState(false);
  const [nodeSelectorGroupIdx, setNodeSelectorGroupIdx] = useState<number>(-1);

  const [fv, setFv] = useState<any>({
    name: '', description: '', enabled: true, auto_build: false, build_interval_sec: 0,
    subscription_source_ids: [],
    template: { dns: {} },
    filters: [], renames: [], rule_bindings: [], groups: [], default_group: ''
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [data, sData, rData]: any = await Promise.all([
        fetchApi('/build-profiles'),
        fetchApi('/subscriptions'),
        fetchApi('/rules')
      ]);
      setItems(data.items || []);
      setSubs(sData.items || []);
      setRules(rData.items || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleBuild = async (id: string, e: any) => {
    e.stopPropagation();
    try {
      await fetchApi(`/build-profiles/${id}/build`, { method: 'POST' });
      setTimeout(loadData, 1000);
    } catch (err) { alert(err); }
  };

  const handleDelete = async (id: string, e?: any) => {
    if (e) e.stopPropagation();
    if (!window.confirm("Are you sure you want to delete this build profile? It might affect assigned tokens.")) return;
    try {
      await fetchApi(`/build-profiles/${id}`, { method: 'DELETE' });
      loadData();
    } catch (err: any) { alert(err.message); }
  };

  const openDrawer = (item?: any) => {
    setActiveTab('General');
    if (item) {
      setFv({
        ...item,
        template: item.template || { dns: {} },
        filters: item.filters || [],
        renames: item.renames || [],
        rule_bindings: item.rule_bindings || [],
        groups: item.groups || [],
        subscription_source_ids: item.subscription_source_ids || []
      });
      setEditingId(item.id);
    } else {
      setFv({
        name: '', description: '', enabled: true, auto_build: false, build_interval_sec: 3600,
        subscription_source_ids: [],
        template: { dns: { enable: true, ipv6: false } },
        filters: [], renames: [], rule_bindings: [], groups: [], default_group: ''
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
        description: fv.description,
        enabled: fv.enabled,
        auto_build: fv.auto_build,
        build_interval_sec: Number(fv.build_interval_sec),
        subscription_source_ids: fv.subscription_source_ids || [],
        rule_bindings: fv.rule_bindings || [],
        filters: fv.filters || [],
        renames: fv.renames || [],
        groups: fv.groups || [],
        default_group: fv.default_group,
        template: {
          ...fv.template,
          port: Number(fv.template.port || 0),
          socks_port: Number(fv.template.socks_port || 0),
          mixed_port: Number(fv.template.mixed_port || 0)
        }
      };

      if (editingId) {
        await fetchApi(`/build-profiles/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
      } else {
        await fetchApi(`/build-profiles`, { method: 'POST', body: JSON.stringify(payload) });
      }
      setDrawerOpen(false);
      loadData();
    } catch (err: any) {
      alert(err.message || 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const renderGeneral = () => (
    <div className="flex-col gap-2">
      <Switch label="Enabled" checked={fv.enabled} onChange={(v:any) => setFv({...fv, enabled: v})} />
      <InputField label="Name" value={fv.name} onChange={(e:any) => setFv({...fv, name: e.target.value})} required />
      <InputField label="Description" type="textarea" value={fv.description} onChange={(e:any) => setFv({...fv, description: e.target.value})} />
      
      <div style={{ marginBottom: 16 }}>
        <label className="input-label">Sources (Subscriptions)</label>
        <div className="flex-col gap-2" style={{ background: 'var(--bg-surface)', padding: 12, borderRadius: 8 }}>
          {subs.map(s => (
            <label key={s.id} className="flex-row gap-2" style={{ alignItems: 'center', cursor: 'pointer' }}>
              <input type="checkbox" checked={fv.subscription_source_ids.includes(s.id)} onChange={e => {
                const ids = new Set(fv.subscription_source_ids);
                if (e.target.checked) ids.add(s.id); else ids.delete(s.id);
                setFv({...fv, subscription_source_ids: Array.from(ids)});
              }} />
              {s.name}
            </label>
          ))}
        </div>
      </div>

      <Switch label="Auto Build" description="Trigger build when upstream sources update" checked={fv.auto_build} onChange={(v:any) => setFv({...fv, auto_build: v})} />
      {fv.auto_build && <InputField label="Min Build Interval (sec)" type="number" value={fv.build_interval_sec} onChange={(e:any) => setFv({...fv, build_interval_sec: e.target.value})} />}
    </div>
  );

  const renderTemplate = () => (
    <div className="flex-col gap-4">
      <div className="flex-col gap-2 glass glass-card" style={{ padding: 16 }}>
        <h4 style={{ margin: 0, paddingBottom: 8, borderBottom: '1px solid var(--border-subtle)' }}>Raw Base YAML (Advanced Override)</h4>
        <p style={{ fontSize: 13, color: 'var(--text-tertiary)', margin: '4px 0' }}>If this is provided, the builder will use this YAML directly as the root configuration skeleton and completely ignore the visual UI fields below. Proxy-groups, proxies, rules, and rule-providers will be forcefully injected over it.</p>
        <textarea 
          className="input" 
          style={{ height: 250, fontFamily: 'monospace', fontSize: 12, resize: 'vertical' }}
          placeholder="Paste your full template.yaml content here... (dns, tun, rule-providers, auto groups...)"
          value={fv.template.raw_base_yaml || ''}
          onChange={(e) => setFv({...fv, template:{...fv.template, raw_base_yaml: e.target.value}})}
        />
      </div>

      <div className="flex-col gap-2" style={{ opacity: fv.template.raw_base_yaml ? 0.3 : 1, pointerEvents: fv.template.raw_base_yaml ? 'none' : 'auto' }}>
        <div className="flex-row gap-2">
        <InputField label="Port" type="number" value={fv.template.port || ''} onChange={(e:any) => setFv({...fv, template:{...fv.template, port: e.target.value}})} />
        <InputField label="Socks Port" type="number" value={fv.template.socks_port || ''} onChange={(e:any) => setFv({...fv, template:{...fv.template, socks_port: e.target.value}})} />
        <InputField label="Mixed Port" type="number" value={fv.template.mixed_port || ''} onChange={(e:any) => setFv({...fv, template:{...fv.template, mixed_port: e.target.value}})} />
      </div>
      
      <InputField label="Mode" type="select" value={fv.template.mode || 'rule'} onChange={(e:any) => setFv({...fv, template:{...fv.template, mode: e.target.value}})}>
        <option value="rule">Rule</option><option value="global">Global</option><option value="direct">Direct</option>
      </InputField>
      
      <InputField label="Log Level" type="select" value={fv.template.log_level || 'info'} onChange={(e:any) => setFv({...fv, template:{...fv.template, log_level: e.target.value}})}>
        <option value="info">Info</option><option value="warning">Warning</option><option value="error">Error</option><option value="debug">Debug</option><option value="silent">Silent</option>
      </InputField>

      <Switch label="Allow LAN" checked={fv.template.allow_lan || false} onChange={(v:any) => setFv({...fv, template:{...fv.template, allow_lan: v}})} />
      <Switch label="Unified Delay" checked={fv.template.unified_delay || false} onChange={(v:any) => setFv({...fv, template:{...fv.template, unified_delay: v}})} />
      <Switch label="Enable IPv6" checked={fv.template.ipv6 || false} onChange={(v:any) => setFv({...fv, template:{...fv.template, ipv6: v}})} />

      <h4 style={{ marginTop: 24, paddingBottom: 8, borderBottom: '1px solid var(--border-subtle)' }}>DNS Config</h4>
      <Switch label="Enable DNS" checked={fv.template.dns?.enable || false} onChange={(v:any) => setFv({...fv, template:{...fv.template, dns:{...fv.template.dns, enable: v}}})} />
      <InputField label="Listen IP:Port" value={fv.template.dns?.listen || ''} onChange={(e:any) => setFv({...fv, template:{...fv.template, dns:{...fv.template.dns, listen: e.target.value}}})} />
      <InputField label="Enhanced Mode" type="select" value={fv.template.dns?.enhanced_mode || 'fake-ip'} onChange={(e:any) => setFv({...fv, template:{...fv.template, dns:{...fv.template.dns, enhanced_mode: e.target.value}}})}>
        <option value="fake-ip">fake-ip</option><option value="redir-host">redir-host</option><option value="none">none</option>
      </InputField>
      
      <StringArrayInput label="Nameservers" placeholder="e.g. 1.1.1.1" value={fv.template.dns?.nameserver || []} onChange={(v) => setFv({...fv, template:{...fv.template, dns:{...fv.template.dns, nameserver: v}}})} />
      <StringArrayInput label="Default Nameservers" placeholder="e.g. 1.1.1.1" value={fv.template.dns?.default_nameserver || []} onChange={(v) => setFv({...fv, template:{...fv.template, dns:{...fv.template.dns, default_nameserver: v}}})} />
      </div>
    </div>
  );

  const renderProcessors = () => (
    <div className="flex-col gap-6">
      <div>
        <div className="flex-row" style={{ justifyContent: 'space-between', marginBottom: 8 }} >
          <label className="input-label">Filters (Regex matching to KEEP proxies)</label>
          <button className="btn btn-ghost" style={{ padding: '0 8px', fontSize: 12 }} onClick={() => setFv({...fv, filters: [...fv.filters, {pattern: ''}]})}>+ Add</button>
        </div>
        <div className="flex-col gap-2">
          {fv.filters.map((f:any, i:number) => (
            <div key={i} className="flex-row gap-2">
              <input className="input-field" value={f.pattern} onChange={e => { const copy = [...fv.filters]; copy[i].pattern = e.target.value; setFv({...fv, filters: copy})}} placeholder="Regex..." />
              <button className="btn btn-ghost" style={{ color: 'var(--danger)' }} onClick={() => setFv({...fv, filters: fv.filters.filter((_:any, idx:number) => idx !== i)})}><X size={16}/></button>
            </div>
          ))}
          {fv.filters.length === 0 && <span style={{fontSize: 12, color: 'var(--text-secondary)'}}>No filters configured.</span>}
        </div>
      </div>

      <div>
        <div className="flex-row" style={{ justifyContent: 'space-between', marginBottom: 8 }} >
          <label className="input-label">Renames (Regex replacing proxy names)</label>
          <button className="btn btn-ghost" style={{ padding: '0 8px', fontSize: 12 }} onClick={() => setFv({...fv, renames: [...fv.renames, {pattern: '', replace: ''}]})}>+ Add</button>
        </div>
        <div className="flex-col gap-2">
          {fv.renames.map((r:any, i:number) => (
            <div key={i} className="flex-row gap-2">
              <input className="input-field" value={r.pattern} onChange={e => { const copy = [...fv.renames]; copy[i].pattern = e.target.value; setFv({...fv, renames: copy})}} placeholder="Regex..." style={{ flex: 1 }} />
              <input className="input-field" value={r.replace} onChange={e => { const copy = [...fv.renames]; copy[i].replace = e.target.value; setFv({...fv, renames: copy})}} placeholder="Replacement..." style={{ flex: 1 }} />
              <button className="btn btn-ghost" style={{ color: 'var(--danger)' }} onClick={() => setFv({...fv, renames: fv.renames.filter((_:any, idx:number) => idx !== i)})}><X size={16}/></button>
            </div>
          ))}
          {fv.renames.length === 0 && <span style={{fontSize: 12, color: 'var(--text-secondary)'}}>No rename operations configured.</span>}
        </div>
      </div>

      <div>
        <div className="flex-row" style={{ justifyContent: 'space-between', marginBottom: 8 }} >
          <label className="input-label">Rule Bindings</label>
          <button className="btn btn-ghost" style={{ padding: '0 8px', fontSize: 12 }} onClick={() => setFv({...fv, rule_bindings: [...fv.rule_bindings, {rule_source_id: '', policy: '', mode: 'auto'}]})}>+ Add</button>
        </div>
        <div className="flex-col gap-3">
          {fv.rule_bindings.map((rb:any, i:number) => (
            <div key={i} className="flex-col gap-2" style={{ background: 'var(--bg-surface)', padding: 12, borderRadius: 8 }}>
              <div className="flex-row gap-2" style={{ alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <select className="input-field" value={rb.rule_source_id} onChange={e => { const c = [...fv.rule_bindings]; c[i].rule_source_id = e.target.value; setFv({...fv, rule_bindings: c}); }}>
                    <option value="">Select Rule Source...</option>
                    {rules.map(r => <option key={r.id} value={r.id}>{r.name}</option>)}
                  </select>
                </div>
                <div style={{ flex: 1 }}>
                  <input className="input-field" placeholder="Target Policy Group" value={rb.policy} onChange={e => { const c = [...fv.rule_bindings]; c[i].policy = e.target.value; setFv({...fv, rule_bindings: c}); }} />
                </div>
                <button className="btn btn-ghost" style={{ color: 'var(--danger)' }} onClick={() => setFv({...fv, rule_bindings: fv.rule_bindings.filter((_:any, idx:number) => idx !== i)})}><X size={16}/></button>
              </div>
              <div className="flex-row gap-2">
                <select className="input-field" style={{ flex: 1, padding: 8 }} value={rb.mode} onChange={e => { const c = [...fv.rule_bindings]; c[i].mode = e.target.value; setFv({...fv, rule_bindings: c}); }}>
                  <option value="auto">Auto Map</option>
                  <option value="inline">Inline (append)</option>
                  <option value="provider">Provider Resource</option>
                </select>
              </div>
            </div>
          ))}
          {fv.rule_bindings.length === 0 && <span style={{fontSize: 12, color: 'var(--text-secondary)'}}>No rule bindings.</span>}
        </div>
      </div>
    </div>
  );

  const renderGroups = () => (
    <div className="flex-col gap-4">
      <InputField label="Default Policy Group" placeholder="Usually the main select group name" value={fv.default_group} onChange={(e:any) => setFv({...fv, default_group: e.target.value})} />
      
      <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid var(--border-subtle)', paddingBottom: 8 }}>
        <h4 style={{ margin: 0 }}>Proxy Groups</h4>
        <button className="btn btn-primary" style={{ padding: '6px 12px', fontSize: 12 }} onClick={() => setFv({...fv, groups: [...fv.groups, {name: 'New Group', type: 'select', include_all: false}]})}>+ Add Group</button>
      </div>

      <div className="flex-col gap-3">
        {fv.groups.map((g:any, i:number) => (
          <div key={i} className="flex-col gap-2" style={{ background: 'var(--bg-surface)', padding: 16, borderRadius: 8, border: '1px solid var(--border-subtle)' }}>
            <div className="flex-row gap-2">
              <input className="input-field" placeholder="Group Name" value={g.name} onChange={e => { const c = [...fv.groups]; c[i].name = e.target.value; setFv({...fv, groups: c}); }} style={{ flex: 1, fontWeight: 'bold' }} />
              <select className="input-field" value={g.type} onChange={e => { const c = [...fv.groups]; c[i].type = e.target.value; setFv({...fv, groups: c}); }} style={{ width: 140 }}>
                <option value="select">Select</option><option value="url-test">URL-Test</option><option value="fallback">Fallback</option><option value="load-balance">Load-Balance</option>
              </select>
              <button className="btn btn-ghost" style={{ color: 'var(--danger)', padding: 8 }} onClick={() => setFv({...fv, groups: fv.groups.filter((_:any, idx:number) => idx !== i)})}><Trash2 size={16}/></button>
            </div>
            
            <div className="flex-row gap-6" style={{ marginTop: 8 }}>
              <label className="flex-row gap-2" style={{ alignItems: 'center', fontSize: 13, cursor: 'pointer' }}>
                <input type="checkbox" checked={g.include_all || false} onChange={e => { const c = [...fv.groups]; c[i].include_all = e.target.checked; setFv({...fv, groups: c}); }} />
                Include ALL proxies
              </label>
            </div>

            {!g.include_all && (
              <>
                <div style={{ marginTop: 8 }}>
                  <label className="input-label" style={{ fontSize: 10 }}>Append specific Members (Group Names or Proxies)</label>
                  <div className="flex-row gap-2">
                    <input className="input-field" placeholder="Comma separated, e.g. PROXY, DIRECT" value={(g.members || []).join(', ')} onChange={e => { const c = [...fv.groups]; c[i].members = e.target.value.split(',').map(s=>s.trim()).filter(Boolean); setFv({...fv, groups: c}); }} style={{ flex: 1, padding: '6px 12px' }} />
                    <button className="btn btn-ghost" style={{ fontSize: 12, padding: '0 12px' }} onClick={() => { setNodeSelectorGroupIdx(i); setNodeSelectorOpen(true); }}>
                      🔍 Picker
                    </button>
                  </div>
                </div>
                <div style={{ marginTop: 8 }}>
                  <label className="input-label" style={{ fontSize: 10 }}>Include By Regex</label>
                  <input className="input-field" placeholder="Comma separated regex" value={(g.include_patterns || []).join(', ')} onChange={e => { const c = [...fv.groups]; c[i].include_patterns = e.target.value.split(',').map(s=>s.trim()).filter(Boolean); setFv({...fv, groups: c}); }} style={{ padding: '6px 12px' }} />
                </div>
              </>
            )}

            {(g.type === 'url-test' || g.type === 'fallback') && (
              <div className="flex-row gap-2" style={{ marginTop: 8 }}>
                 <input className="input-field" placeholder="Testing URL" value={g.url || ''} onChange={e => { const c = [...fv.groups]; c[i].url = e.target.value; setFv({...fv, groups: c}); }} style={{ flex: 1, padding: '6px 12px' }} />
                 <input type="number" className="input-field" placeholder="Interval Sec" value={g.interval_sec || 0} onChange={e => { const c = [...fv.groups]; c[i].interval_sec = Number(e.target.value); setFv({...fv, groups: c}); }} style={{ width: 100, padding: '6px 12px' }} />
              </div>
            )}
          </div>
        ))}
        {fv.groups.length === 0 && <span style={{fontSize: 12, color: 'var(--text-secondary)'}}>No groups configured.</span>}
      </div>
    </div>
  );

  return (
    <div className="animate-fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Build Profiles</h1>
          <p>Configure pipeline profiles to assemble target configurations.</p>
        </div>
        <div className="flex-row gap-3">
          <button className="btn btn-ghost" onClick={loadData}><RefreshCw size={16} /> Reload</button>
          <button className="btn btn-primary" onClick={() => openDrawer()}><Plus size={16} /> Add Profile</button>
        </div>
      </div>

      <div className="flex-col gap-4">
        {loading && items.length === 0 ? (
          <div className="flex-center" style={{ padding: 40 }}><Layers className="lucide-spin text-secondary" /></div>
        ) : items.map(item => (
          <div key={item.id} className="glass glass-card flex-col gap-3">
            <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'flex-start' }}>
               <div className="flex-col gap-1">
                  <div className="flex-row gap-3" style={{ alignItems: 'center' }}>
                    <h3 style={{ fontSize: 18, margin: 0 }}>{item.name}</h3>
                    {item.enabled ? <span className="badge badge-success">Active</span> : <span className="badge badge-warning">Disabled</span>}
                    {item.auto_build && <span className="badge badge-primary">Auto-Build</span>}
                  </div>
                  <p style={{ fontSize: 13, color: 'var(--text-tertiary)', marginTop: 4 }}>{item.description || 'No description provided.'}</p>
               </div>
               <div className="flex-row gap-2">
                 <button className="btn btn-ghost" onClick={() => openDrawer(item)} style={{ padding: 8 }}><Settings2 size={18} /></button>
                 <button className="btn btn-ghost text-danger" onClick={(e) => handleDelete(item.id, e)} style={{ padding: 8 }}><Trash2 size={18} /></button>
                 <button className="btn btn-primary" onClick={(e) => handleBuild(item.id, e)}><Play size={16} /> Build Engine</button>
               </div>
            </div>
            
            <div style={{ background: 'var(--bg-base)', padding: 12, borderRadius: 6, fontSize: 12, fontFamily: 'monospace', color: 'var(--text-secondary)' }}>
              Sources: {item.subscription_source_ids?.length || 0}   |   Groups: {item.groups?.length || 0}   |   Status: {item.status}   |   Updated: {new Date(item.updated_at).toLocaleString()}
            </div>
          </div>
        ))}
      </div>

      <Drawer isOpen={drawerOpen} onClose={() => setDrawerOpen(false)} title={editingId ? 'Edit Profile' : 'New Build Profile'} width={1100}>
        <Tabs tabs={['General', 'Template', 'Processors', 'Groups']} activeTab={activeTab} onChange={setActiveTab} />
        
        <div style={{ flex: 1 }}>
          {activeTab === 'General' && renderGeneral()}
          {activeTab === 'Template' && renderTemplate()}
          {activeTab === 'Processors' && renderProcessors()}
          {activeTab === 'Groups' && renderGroups()}
        </div>

        <div style={{ marginTop: 24, paddingTop: 16, borderTop: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
          <button className="btn btn-ghost" onClick={() => setDrawerOpen(false)}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save Profile'}</button>
        </div>
      </Drawer>

      <Drawer isOpen={nodeSelectorOpen} onClose={() => setNodeSelectorOpen(false)} title="Select Proxies Visually" width={700}>
        <div className="flex-col gap-4">
          <p style={{ fontSize: 13, color: 'var(--text-secondary)' }}>Pick proxies from the selected Subscriptions of this profile. Check the boxes to append them to the group.</p>
          <div className="flex-col gap-4" style={{ maxHeight: '60vh', overflowY: 'auto' }}>
            {subs.filter(s => fv.subscription_source_ids?.includes(s.id)).map(s => (
              <div key={s.id} className="flex-col gap-2 glass glass-card" style={{ padding: 12 }}>
                <h4 style={{ margin: 0 }}>{s.name} <span className="badge">{s.snapshot?.proxies?.length || 0} nodes</span></h4>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  {(s.snapshot?.proxies || []).map((px: any) => {
                    const currentMembers = nodeSelectorGroupIdx >= 0 ? fv.groups[nodeSelectorGroupIdx]?.members || [] : [];
                    const isChecked = currentMembers.includes(px.name);
                    return (
                      <label key={px.name} className="flex-row gap-2" style={{ alignItems: 'center', fontSize: 12, cursor: 'pointer', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        <input type="checkbox" checked={isChecked} onChange={e => {
                          const c = [...fv.groups];
                          const set = new Set(c[nodeSelectorGroupIdx].members || []);
                          if (e.target.checked) set.add(px.name); else set.delete(px.name);
                          c[nodeSelectorGroupIdx].members = Array.from(set);
                          setFv({...fv, groups: c});
                        }} />
                        <span title={px.name} style={{ textOverflow: 'ellipsis', overflow: 'hidden' }}>{px.name}</span>
                      </label>
                    );
                  })}
                </div>
              </div>
            ))}
            {fv.subscription_source_ids?.length === 0 && (
              <span style={{ fontSize: 13, color: 'var(--danger)' }}>No subscriptions selected in General tab yet!</span>
            )}
          </div>
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
            <button className="btn btn-primary" onClick={() => setNodeSelectorOpen(false)}>Done</button>
          </div>
        </div>
      </Drawer>
    </div>
  );
}
