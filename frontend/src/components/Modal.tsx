import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';

export function Modal({ isOpen, onClose, title, children, width = 600 }: { isOpen: boolean, onClose: () => void, title: string, children: React.ReactNode, width?: number | string }) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!isOpen || !mounted) return null;

  return createPortal(
    <div style={{ position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <div style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(8px)' }} onClick={onClose} />
      <div className="glass glass-card animate-fade-in" style={{ position: 'relative', width, maxHeight: '90vh', overflowY: 'auto', padding: 0 }}>
        <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', padding: '20px 24px', borderBottom: '1px solid var(--border-subtle)' }}>
          <h3 style={{ margin: 0, fontSize: 18 }}>{title}</h3>
          <button className="btn btn-ghost" style={{ padding: 8 }} onClick={onClose}><X size={18} /></button>
        </div>
        <div style={{ padding: 24 }}>
          {children}
        </div>
      </div>
    </div>,
    document.body
  );
}

export function Drawer({ isOpen, onClose, title, children, width = 450 }: { isOpen: boolean, onClose: () => void, title: string, children: React.ReactNode, width?: number | string }) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!isOpen || !mounted) return null;

  return createPortal(
    <div style={{ position: 'fixed', inset: 0, zIndex: 1000 }}>
      <div style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.4)', backdropFilter: 'blur(4px)' }} onClick={onClose} />
      <div className="glass glass-card animate-fade-in" style={{ 
        position: 'absolute', right: 0, top: 0, bottom: 0, width, maxWidth: '100vw',
        borderRadius: 0, borderRight: 'none', borderTop: 'none', borderBottom: 'none',
        display: 'flex', flexDirection: 'column', padding: 0
      }}>
         <div className="flex-row" style={{ justifyContent: 'space-between', alignItems: 'center', padding: '24px', borderBottom: '1px solid var(--border-subtle)' }}>
          <h3 style={{ margin: 0, fontSize: 20 }}>{title}</h3>
          <button className="btn btn-ghost" style={{ padding: 8 }} onClick={onClose}><X size={20} /></button>
        </div>
        <div style={{ padding: 24, flex: 1, overflowY: 'auto', minHeight: 0 }}>
          {children}
        </div>
      </div>
    </div>,
    document.body
  );
}
