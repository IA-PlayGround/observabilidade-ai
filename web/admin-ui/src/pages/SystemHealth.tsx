import { useEffect, useState } from 'react';
import { apiFetch } from '../hooks/useAuth';

interface HealthData {
  status: string;
  services: Record<string, string>;
}

export default function SystemHealth() {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [error, setError] = useState('');
  const [lastRefresh, setLastRefresh] = useState(0);

  const refresh = () => {
    setError('');
    apiFetch('/api/v1/admin/health')
      .then(r => r.json())
      .then(data => { setHealth(data); setLastRefresh(Date.now()); })
      .catch(e => setError(e.message));
  };

  useEffect(() => { refresh(); const i = setInterval(refresh, 10000); return () => clearInterval(i); }, []);

  return (
    <div>
      <div className="page-header">
        <h1>System Health</h1>
        <p>Real-time health status of all Spectrum components</p>
      </div>

      <div style={{ marginBottom: 16, display: 'flex', gap: 8, alignItems: 'center' }}>
        <button onClick={refresh}>Refresh</button>
        <span style={{ color: 'var(--color-text-muted)', fontSize: 12 }}>
          Last updated: {lastRefresh ? new Date(lastRefresh).toLocaleTimeString() : '--'}
        </span>
      </div>

      {error && (
        <div className="card" style={{ borderColor: 'var(--color-danger)', marginBottom: 16 }}>
          <p style={{ color: 'var(--color-danger)' }}>{error}</p>
        </div>
      )}

      {health ? (
        <>
          <div className="card" style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{
                width: 16, height: 16, borderRadius: '50%',
                background: health.status === 'healthy' ? 'var(--color-success)' : 'var(--color-warning)',
              }} />
              <span style={{ fontSize: 18, fontWeight: 600, textTransform: 'capitalize' }}>
                {health.status}
              </span>
            </div>
          </div>

          <div className="card">
            <h3 style={{ marginBottom: 12 }}>Services ({Object.keys(health.services).length})</h3>
            <table>
              <thead>
                <tr><th>Service</th><th>Status</th><th>Uptime</th></tr>
              </thead>
              <tbody>
                {Object.entries(health.services).map(([name, status]) => (
                  <tr key={name}>
                    <td style={{ fontWeight: 500 }}>{name}</td>
                    <td>
                      <span className={`badge ${status === 'healthy' ? 'success' : status === 'degraded' ? 'warning' : 'danger'}`}>
                        {status}
                      </span>
                    </td>
                    <td style={{ color: 'var(--color-text-muted)' }}>--</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        <div className="loading">Connecting to health endpoint...</div>
      )}
    </div>
  );
}
