import { useState } from 'react';
import { apiFetch } from '../hooks/useAuth';

interface CorrelationResult {
  traceId: string;
  rootSpanId: string;
  serviceName: string;
  spans: Array<{ name: string; serviceName: string; duration: number; statusCode: number }>;
  metrics: Array<{ name: string; value: number; unit: string }>;
  logs: Array<{ timestamp: string; severity: string; body: string }>;
  correlatedAt: string;
}

export default function Correlations() {
  const [traceId, setTraceId] = useState('');
  const [serviceName, setServiceName] = useState('');
  const [result, setResult] = useState<CorrelationResult | null>(null);
  const [serviceResults, setServiceResults] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [mode, setMode] = useState<'trace' | 'service'>('trace');

  const searchByTrace = async () => {
    if (!traceId.trim()) return;
    setLoading(true);
    setError('');
    setResult(null);
    setServiceResults(null);
    try {
      const res = await apiFetch(`/api/v1/correlation/${traceId}`);
      if (!res.ok) throw new Error('Trace not found');
      const data = await res.json();
      setResult(data);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const searchByService = async () => {
    if (!serviceName.trim()) return;
    setLoading(true);
    setError('');
    setResult(null);
    setServiceResults(null);
    try {
      const res = await apiFetch(`/api/v1/correlation/service/${serviceName}`);
      if (!res.ok) throw new Error('No traces found');
      const data = await res.json();
      setServiceResults(data);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1>Correlation Explorer</h1>
        <p>Search correlated metrics, traces, and logs</p>
      </div>

      <div className="card" style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
          <button
            className={mode === 'trace' ? 'primary' : ''}
            onClick={() => setMode('trace')}
          >
            By Trace ID
          </button>
          <button
            className={mode === 'service' ? 'primary' : ''}
            onClick={() => setMode('service')}
          >
            By Service
          </button>
        </div>

        {mode === 'trace' ? (
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              placeholder="Enter Trace ID (e.g. 0af7651916cd43dd8448eb211c80319c)"
              value={traceId}
              onChange={e => setTraceId(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && searchByTrace()}
            />
            <button className="primary" onClick={searchByTrace} disabled={loading}>
              Search
            </button>
          </div>
        ) : (
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              placeholder="Enter Service Name (e.g. checkout)"
              value={serviceName}
              onChange={e => setServiceName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && searchByService()}
            />
            <button className="primary" onClick={searchByService} disabled={loading}>
              Search
            </button>
          </div>
        )}
      </div>

      {loading && <div className="loading">Searching...</div>}
      {error && <div className="card" style={{ borderColor: 'var(--color-danger)' }}><p style={{ color: 'var(--color-danger)' }}>{error}</p></div>}

      {result && (
        <div>
          <div className="card" style={{ marginBottom: 16 }}>
            <h3>
              Trace: <code>{result.traceId}</code>
            </h3>
            <p style={{ color: 'var(--color-text-muted)' }}>
              Root Span: {result.rootSpanId} | Service: {result.serviceName}
              | Correlated: {new Date(result.correlatedAt).toLocaleString()}
            </p>
          </div>

          {result.spans.length > 0 && (
            <div className="card" style={{ marginBottom: 16 }}>
              <h3 style={{ marginBottom: 12 }}>Spans ({result.spans.length})</h3>
              <table>
                <thead>
                  <tr><th>Name</th><th>Service</th><th>Duration</th><th>Status</th></tr>
                </thead>
                <tbody>
                  {result.spans.map((s, i) => (
                    <tr key={i}>
                      <td>{s.name}</td>
                      <td>{s.serviceName}</td>
                      <td>{typeof s.duration === 'number' ? `${(s.duration / 1e6).toFixed(2)}ms` : String(s.duration)}</td>
                      <td>
                        <span className={`badge ${s.statusCode === 0 ? 'success' : 'danger'}`}>
                          {s.statusCode === 0 ? 'OK' : s.statusCode}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {result.logs.length > 0 && (
            <div className="card" style={{ marginBottom: 16 }}>
              <h3 style={{ marginBottom: 12 }}>Logs ({result.logs.length})</h3>
              <div className="json-preview">
                {result.logs.slice(0, 20).map((log, i) => (
                  <div key={i} style={{ marginBottom: 8 }}>
                    <span style={{ color: 'var(--color-text-muted)' }}>
                      {new Date(log.timestamp).toISOString()}
                    </span>
                    {' '}
                    <span className={`badge ${log.severity === 'ERROR' ? 'danger' : log.severity === 'WARN' ? 'warning' : 'info'}`}>
                      {log.severity}
                    </span>
                    {' '}
                    {log.body}
                  </div>
                ))}
              </div>
            </div>
          )}

          {result.metrics.length > 0 && (
            <div className="card">
              <h3 style={{ marginBottom: 12 }}>Metrics</h3>
              <div className="json-preview">{JSON.stringify(result.metrics, null, 2)}</div>
            </div>
          )}
        </div>
      )}

      {serviceResults && (
        <div className="card">
          <h3 style={{ marginBottom: 12 }}>
            Service: {serviceResults.service} ({serviceResults.traceCount} traces)
          </h3>
          <div className="json-preview">{JSON.stringify(serviceResults.correlations?.slice(0, 5), null, 2)}</div>
        </div>
      )}
    </div>
  );
}
