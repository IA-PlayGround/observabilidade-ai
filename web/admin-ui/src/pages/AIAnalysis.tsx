import { useState } from 'react';
import { apiFetch } from '../hooks/useAuth';

export default function AIAnalysis() {
  const [mode, setMode] = useState<'diagnose' | 'release' | 'query'>('diagnose');

  // Diagnose
  const [alertName, setAlertName] = useState('High Error Rate');
  const [service, setService] = useState('checkout');
  const [severity, setSeverity] = useState('critical');
  const [traceId, setTraceId] = useState('');
  const [description, setDescription] = useState('');

  // Release
  const [relService, setRelService] = useState('');
  const [version, setVersion] = useState('');
  const [commitSha, setCommitSha] = useState('');

  // Query
  const [query, setQuery] = useState('');
  const [context, setContext] = useState('');

  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const runDiagnose = async () => {
    setLoading(true);
    setError('');
    setResult(null);
    try {
      const res = await apiFetch('/api/v1/ai/diagnose', {
        method: 'POST',
        body: JSON.stringify({
          incident: {
            alertName,
            serviceName: service,
            severity,
            traceId,
            description,
            startTime: new Date().toISOString(),
          },
          action: 'diagnose',
          telemetry: { metrics: [], spans: [], logs: [] },
        }),
      });
      if (!res.ok) throw new Error('Diagnosis failed');
      setResult(await res.json());
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const runReleaseAnalysis = async () => {
    setLoading(true);
    setError('');
    setResult(null);
    try {
      const res = await apiFetch('/api/v1/ai/release-analysis', {
        method: 'POST',
        body: JSON.stringify({
          service: relService,
          version,
          commitSha,
          preDeployMetrics: [],
          postDeployMetrics: [],
        }),
      });
      if (!res.ok) throw new Error('Release analysis failed');
      setResult(await res.json());
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const runQuery = async () => {
    setLoading(true);
    setError('');
    setResult(null);
    try {
      const res = await apiFetch('/api/v1/ai/query', {
        method: 'POST',
        body: JSON.stringify({ query, context }),
      });
      if (!res.ok) throw new Error('Query failed');
      setResult(await res.json());
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1>AI Analysis</h1>
        <p>AI-powered diagnostics, release analysis, and ad-hoc queries via MCP</p>
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 24 }}>
        {(['diagnose', 'release', 'query'] as const).map(m => (
          <button key={m} className={mode === m ? 'primary' : ''} onClick={() => setMode(m)}>
            {m === 'diagnose' ? 'Diagnose Incident' : m === 'release' ? 'Release Analysis' : 'Ad-Hoc Query'}
          </button>
        ))}
      </div>

      <div className="grid grid-2">
        <div className="card">
          {mode === 'diagnose' && (
            <>
              <div className="form-group">
                <label>Alert Name</label>
                <input value={alertName} onChange={e => setAlertName(e.target.value)} />
              </div>
              <div className="form-group">
                <label>Service</label>
                <input value={service} onChange={e => setService(e.target.value)} />
              </div>
              <div className="form-group">
                <label>Severity</label>
                <select value={severity} onChange={e => setSeverity(e.target.value)}>
                  <option value="critical">Critical</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>
              <div className="form-group">
                <label>Trace ID</label>
                <input value={traceId} onChange={e => setTraceId(e.target.value)} placeholder="Optional" />
              </div>
              <div className="form-group">
                <label>Description</label>
                <textarea
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  placeholder="What happened?"
                  rows={3}
                />
              </div>
              <button className="primary" onClick={runDiagnose} disabled={loading} style={{ width: '100%' }}>
                {loading ? 'Analyzing...' : 'Run Diagnosis'}
              </button>
            </>
          )}

          {mode === 'release' && (
            <>
              <div className="form-group">
                <label>Service</label>
                <input value={relService} onChange={e => setRelService(e.target.value)} />
              </div>
              <div className="form-group">
                <label>Version</label>
                <input value={version} onChange={e => setVersion(e.target.value)} placeholder="v2.3.1" />
              </div>
              <div className="form-group">
                <label>Commit SHA</label>
                <input value={commitSha} onChange={e => setCommitSha(e.target.value)} />
              </div>
              <button className="primary" onClick={runReleaseAnalysis} disabled={loading} style={{ width: '100%' }}>
                {loading ? 'Analyzing...' : 'Run Release Analysis'}
              </button>
            </>
          )}

          {mode === 'query' && (
            <>
              <div className="form-group">
                <label>Question</label>
                <textarea
                  value={query}
                  onChange={e => setQuery(e.target.value)}
                  placeholder="e.g., What caused the latency spike in checkout at 2 PM?"
                  rows={3}
                />
              </div>
              <div className="form-group">
                <label>Context (optional)</label>
                <textarea
                  value={context}
                  onChange={e => setContext(e.target.value)}
                  placeholder="Additional context data..."
                  rows={3}
                />
              </div>
              <button className="primary" onClick={runQuery} disabled={loading} style={{ width: '100%' }}>
                {loading ? 'Querying...' : 'Ask AI'}
              </button>
            </>
          )}
        </div>

        <div>
          {error && (
            <div className="card" style={{ borderColor: 'var(--color-danger)', marginBottom: 16 }}>
              <p style={{ color: 'var(--color-danger)' }}>{error}</p>
            </div>
          )}

          {loading && <div className="loading">AI is analyzing...</div>}

          {result && (
            <div className="card">
              <h3 style={{ marginBottom: 12 }}>Result</h3>
              {result.rootCause && (
                <>
                  <div style={{ marginBottom: 12 }}>
                    <span className="badge info">Model: {result.model}</span>
                    {' '}
                    <span className={`badge ${result.confidence > 0.7 ? 'success' : 'warning'}`}>
                      Confidence: {((result.confidence || 0) * 100).toFixed(0)}%
                    </span>
                    {' '}
                    <span className={`badge ${result.severity === 'critical' ? 'danger' : 'warning'}`}>
                      {result.severity || result.risk || 'N/A'}
                    </span>
                  </div>

                  {result.rootCause && (
                    <div style={{ marginBottom: 12 }}>
                      <strong>Root Cause:</strong>
                      <p style={{ color: 'var(--color-text-muted)', marginTop: 4 }}>{result.rootCause}</p>
                    </div>
                  )}

                  {result.evidence && result.evidence.length > 0 && (
                    <div style={{ marginBottom: 12 }}>
                      <strong>Evidence:</strong>
                      <ul style={{ paddingLeft: 20, marginTop: 4 }}>
                        {result.evidence.map((e: string, i: number) => (
                          <li key={i} style={{ color: 'var(--color-text-muted)', fontSize: 13 }}>{e}</li>
                        ))}
                      </ul>
                    </div>
                  )}

                  {result.recommendations && result.recommendations.length > 0 && (
                    <div style={{ marginBottom: 12 }}>
                      <strong>Recommendations:</strong>
                      <ul style={{ paddingLeft: 20, marginTop: 4 }}>
                        {result.recommendations.map((r: string, i: number) => (
                          <li key={i} style={{ color: 'var(--color-success)', fontSize: 13 }}>{r}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </>
              )}

              {result.metrics && (
                <div style={{ marginBottom: 12 }}>
                  <strong>Metrics Comparison:</strong>
                  <div className="json-preview" style={{ marginTop: 4 }}>
                    {JSON.stringify(result.metrics, null, 2)}
                  </div>
                </div>
              )}

              {result.hasRegression !== undefined && (
                <div style={{ marginTop: 12 }}>
                  <strong>Regression: </strong>
                  <span className={`badge ${result.hasRegression ? 'danger' : 'success'}`}>
                    {result.hasRegression ? 'DETECTED' : 'NONE'}
                  </span>
                  {result.degradationPct > 0 && (
                    <span style={{ marginLeft: 8, color: 'var(--color-text-muted)' }}>
                      {result.degradationPct.toFixed(1)}% degradation
                    </span>
                  )}
                </div>
              )}

              {result.answer && (
                <div>
                  <strong>Answer:</strong>
                  <p style={{ color: 'var(--color-text-muted)', marginTop: 4 }}>{result.answer}</p>
                </div>
              )}

              <div style={{ marginTop: 16 }}>
                <div className="json-preview">{JSON.stringify(result, null, 2)}</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
