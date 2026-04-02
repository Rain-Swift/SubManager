import { useState } from 'react';
import { Plus, X } from 'lucide-react';

export function InputField({ label, ...props }: any) {
  return (
    <div style={{ marginBottom: 16 }}>
      {label && <label className="input-label">{label}</label>}
      {props.type === 'textarea' ? (
        <textarea className="input-field" style={{ minHeight: 80, resize: 'vertical' }} {...props} />
      ) : props.type === 'select' ? (
        <select className="input-field" {...props}>{props.children}</select>
      ) : (
        <input className="input-field" {...props} />
      )}
    </div>
  );
}

export function Switch({ label, checked, onChange, description }: any) {
  return (
    <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, padding: '12px', background: 'var(--bg-surface)', borderRadius: 8, border: '1px solid var(--border-subtle)' }}>
      <div>
        <div style={{ fontSize: 14, fontWeight: 500 }}>{label}</div>
        {description && <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>{description}</div>}
      </div>
      <div 
        onClick={() => onChange(!checked)}
        style={{ 
          width: 44, height: 24, borderRadius: 12, background: checked ? 'var(--primary)' : 'var(--bg-base)',
          position: 'relative', cursor: 'pointer', transition: 'all 0.2s', border: '1px solid var(--border-strong)',
          flexShrink: 0
        }}
      >
        <div style={{ 
          width: 20, height: 20, borderRadius: 10, background: 'white', position: 'absolute', top: 1,
          left: checked ? 21 : 1, transition: 'all 0.2s'
        }} />
      </div>
    </div>
  );
}

export function StringArrayInput({ label, value, onChange, placeholder = "Pattern..." }: { label: string, value: string[], onChange: (v: string[]) => void, placeholder?: string }) {
  const [adding, setAdding] = useState('');
  
  const handleAdd = (e: any) => {
    e.preventDefault();
    if (adding.trim() && !value.includes(adding.trim())) {
      onChange([...value, adding.trim()]);
    }
    setAdding('');
  };

  return (
    <div style={{ marginBottom: 16 }}>
      {label && <label className="input-label">{label}</label>}
      <div className="flex-col gap-2">
        {value.map((item, idx) => (
          <div key={idx} className="flex-row gap-2" style={{ alignItems: 'center' }}>
            <div className="input-field" style={{ padding: '8px 12px', flex: 1, background: 'var(--bg-surface)' }}>{item}</div>
            <button type="button" className="btn btn-ghost" style={{ padding: 8, color: 'var(--danger)' }} onClick={() => onChange(value.filter((_, i) => i !== idx))}>
              <X size={16} />
            </button>
          </div>
        ))}
        <div className="flex-row gap-2">
          <input 
            type="text" 
            className="input-field" 
            placeholder={placeholder}
            value={adding} 
            onChange={e => setAdding(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleAdd(e)}
            style={{ padding: '8px 12px' }}
          />
          <button type="button" className="btn btn-ghost" style={{ padding: 8 }} onClick={handleAdd}>
            <Plus size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}

export function KeyValueInput({ label, value, onChange, keyPlaceholder = "Key", valPlaceholder = "Value" }: { label: string, value: Record<string, string>, onChange: (v: Record<string, string>) => void, keyPlaceholder?: string, valPlaceholder?: string }) {
  const [addingKey, setAddingKey] = useState('');
  const [addingVal, setAddingVal] = useState('');

  const entries = Object.entries(value || {});

  const handleAdd = (e: any) => {
    e.preventDefault();
    if (addingKey.trim()) {
      onChange({ ...value, [addingKey.trim()]: addingVal.trim() });
    }
    setAddingKey('');
    setAddingVal('');
  };

  return (
    <div style={{ marginBottom: 16 }}>
      {label && <label className="input-label">{label}</label>}
      <div className="flex-col gap-2">
        {entries.map(([k, v]) => (
          <div key={k} className="flex-row gap-2" style={{ alignItems: 'center' }}>
            <div className="input-field" style={{ padding: '8px 12px', flex: 1, background: 'var(--bg-surface)', fontFamily: 'monospace', fontSize: 12 }}>{k}</div>
            <div className="input-field" style={{ padding: '8px 12px', flex: 2, background: 'var(--bg-surface)' }}>{v}</div>
            <button type="button" className="btn btn-ghost" style={{ padding: 8, color: 'var(--danger)' }} onClick={() => {
              const cp = { ...value };
              delete cp[k];
              onChange(cp);
            }}>
              <X size={16} />
            </button>
          </div>
        ))}
        <div className="flex-row gap-2">
          <input type="text" className="input-field" placeholder={keyPlaceholder} value={addingKey} onChange={e => setAddingKey(e.target.value)} style={{ padding: '8px 12px', flex: 1 }} />
          <input type="text" className="input-field" placeholder={valPlaceholder} value={addingVal} onChange={e => setAddingVal(e.target.value)} onKeyDown={e => e.key === 'Enter' && handleAdd(e)} style={{ padding: '8px 12px', flex: 2 }} />
          <button type="button" className="btn btn-ghost" style={{ padding: 8 }} onClick={handleAdd}><Plus size={16} /></button>
        </div>
      </div>
    </div>
  );
}

export function Tabs({ tabs, activeTab, onChange }: { tabs: string[], activeTab: string, onChange: (t: string) => void }) {
  return (
    <div className="flex-row" style={{ borderBottom: '1px solid var(--border-strong)', marginBottom: 24, overflowX: 'auto', gap: 16 }}>
      {tabs.map(t => (
        <div 
          key={t}
          onClick={() => onChange(t)}
          style={{
            padding: '12px 0 10px', fontWeight: activeTab === t ? 600 : 500, fontSize: 14, cursor: 'pointer', whiteSpace: 'nowrap',
            color: activeTab === t ? 'var(--primary)' : 'var(--text-secondary)',
            borderBottom: activeTab === t ? '2px solid var(--primary)' : '2px solid transparent',
            transition: 'all 0.2s'
          }}
        >
          {t}
        </div>
      ))}
    </div>
  );
}
