import { useEffect, useState } from 'react';
import { apiFetch } from '../hooks/useAuth';

interface HealthData {
  status: string;
  services: Record<string, string>;
}

interface StatCard {
  label: string;
  value: string;
  status: 'success' | 'warning' | 'danger' | 'info';
}

export default function Dashboard() {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [stats, setStats] = useState<StatCard[]>([
    { label: 'Services Monitored', value: '--', status: 'info' },
    { label: 'Traces / min', value: '--', status: 'info' },
    { label: 'Active Incidents', value: '--', status: 'success' },
    { label: 'MQ Lag', value: '--', status: 'success' },
  ]);

  useEffect(() => {
    apiFetch('/api/v1/admin/health')
      .then(r => r.json())
      .then(setHealth)
      .catch(console.error);
  }, []);

  const serviceCount = health
    ? Object.keys(health.services).length
    : 0;
  const healthyCount = health
    ? Object.values(health.services).filter(s => s === 'healthy').length
    : 0;

  return (
    <div>
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>Spectrum Observability Platform Overview</p>
      </div>

      <div className="grid grid-4" style={{ marginBottom: 24 }}>
        <div className="card stat-card">
          <div className="stat-value">{serviceCount}</div>
          <div className="stat-label">Services</div>
        </div>
        <div className="card stat-card">
          <div className="stat-value">{healthyCount}/{serviceCount}</div>
          <div className="stat-label">Healthy</div>
        </div>
        <div className="card stat-card">
          <div className="stat-value" style={{ color: 'var(--color-success)' }}>0</div>
          <div className="stat-label">Active Incidents</div>
        </div>
        <div className="card stat-card">
          <div className="stat-value">--</div>
          <div className="stat-label">Avg. Correlation Time</div>
        </div>
      </div>

      <div className="grid grid-2">
        <div className="card">
          <h3 style={{ marginBottom: 16 }}>Service Health</h3>
          {health ? (
            <table>
              <thead>
                <tr><th>Service</th><th>Status</th></tr>
              </thead>
              <tbody>
                {Object.entries(health.services).map(([name, status]) => (
                  <tr key={name}>
                    <td>{name}</td>
                    <td>
                      <span className={`badge ${status === 'healthy' ? 'success' : 'danger'}`}>
                        {status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="loading">Loading...</div>
          )}
        </div>

        <div className="card">
          <h3 style={{ marginBottom: 16 }}>Platform Status</h3>
          <p style={{ color: 'var(--color-text-muted)', marginBottom: 12 }}>
            System health: {health ? (
              <span className={`badge ${health.status === 'healthy' ? 'success' : 'warning'}`}>
                {health.status}
              </span>
            ) : 'loading...'}
          </p>
          <p style={{ color: 'var(--color-text-muted)', fontSize: 14 }}>
            The Spectrum platform provides unified observability with AI-powered diagnostics.
            Navigate through the sidebar to explore correlations, run AI analysis,
            manage data sources, and monitor system health.
          </p>
          <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
            <a href="/api/v1/grafana" target="_blank" rel="noopener">
              <button className="primary">Open Grafana</button>
            </a>
            <a href="/ai" onClick={e => { e.preventDefault(); window.location.href = '/ai'; }}>
              <button>AI Analysis</button>
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
