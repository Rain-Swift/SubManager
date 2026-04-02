import { useState } from 'react';
import { setToken, fetchApi } from '../api';
import { Shield, ArrowRight, Activity } from 'lucide-react';

export default function Login() {
  const [token, setTokenInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      // Test the token by fetching healthz
      setToken(token);
      await fetchApi('/healthz');
      window.location.href = '/';
    } catch (err: any) {
      setError('Invalid standard token or connection failed.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex-center" style={{ height: '100vh', flexDirection: 'column' }}>
      
      <div className="glass glass-card animate-fade-in" style={{ width: 400, padding: 40, position: 'relative' }}>
        <div style={{ position: 'absolute', top: -30, left: '50%', transform: 'translateX(-50%)', background: 'var(--bg-surface)', padding: 16, borderRadius: '50%', border: '1px solid var(--border-subtle)' }}>
          <Shield color="var(--primary)" size={32} />
        </div>
        
        <div style={{ textAlign: 'center', marginTop: 24, marginBottom: 32 }}>
          <h2 className="page-title" style={{ fontSize: 24, marginBottom: 8 }}>SubManager</h2>
          <p style={{ fontSize: 14 }}>Authentication required for access.</p>
        </div>

        <form onSubmit={handleLogin} className="flex-col gap-4">
          <div>
            <label className="input-label">Superuser Token</label>
            <input 
              type="password" 
              className="input-field" 
              placeholder="••••••••••••"
              value={token}
              onChange={e => setTokenInput(e.target.value)}
              required
            />
          </div>
          
          {error && <div style={{ color: 'var(--danger)', fontSize: 13, background: 'rgba(239, 68, 68, 0.1)', padding: 12, borderRadius: 8 }}>{error}</div>}

          <button type="submit" className="btn btn-primary" style={{ marginTop: 12 }} disabled={loading}>
            {loading ? <Activity className="lucide-spin" size={16} /> : 'Authenticate'}
            {!loading && <ArrowRight size={16} />}
          </button>
        </form>
      </div>

    </div>
  );
}
