import { useState } from 'react';
import { useAuth } from '../hooks/useAuth';

export default function Login() {
  const { login } = useAuth();
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('spectrum');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    const ok = await login(username, password);
    setLoading(false);
    if (!ok) {
      setError('Invalid credentials. Try admin / spectrum');
    }
  };

  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      minHeight: '100vh', background: 'var(--color-bg)',
    }}>
      <div className="card" style={{ width: 380 }}>
        <h1 style={{ textAlign: 'center', marginBottom: 8, color: 'var(--color-primary)' }}>
          Spectrum
        </h1>
        <p style={{ textAlign: 'center', color: 'var(--color-text-muted)', marginBottom: 24, fontSize: 14 }}>
          Observability Platform Admin
        </p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Username</label>
            <input
              value={username}
              onChange={e => setUsername(e.target.value)}
              autoFocus
            />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
            />
          </div>

          {error && (
            <div style={{ color: 'var(--color-danger)', marginBottom: 12, fontSize: 13 }}>{error}</div>
          )}

          <button className="primary" type="submit" disabled={loading} style={{ width: '100%' }}>
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>

        <p style={{ textAlign: 'center', color: 'var(--color-text-muted)', fontSize: 11, marginTop: 16 }}>
          Default credentials: admin / spectrum
        </p>
      </div>
    </div>
  );
}
