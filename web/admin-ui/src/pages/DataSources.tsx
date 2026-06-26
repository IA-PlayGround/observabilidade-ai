import { useEffect, useState } from 'react';
import { apiFetch } from '../hooks/useAuth';

interface DataSource {
  id: string;
  name: string;
  type: string;
  endpoint: string;
  enabled: boolean;
}

export default function DataSources() {
  const [sources, setSources] = useState<DataSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  const [form, setForm] = useState({ name: '', type: 'prometheus', endpoint: '' });

  useEffect(() => {
    apiFetch('/api/v1/admin/sources')
      .then(r => r.json())
      .then(data => { setSources(Array.isArray(data) ? data : []); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  const createSource = async () => {
    try {
      const res = await apiFetch('/api/v1/admin/sources', {
        method: 'POST',
        body: JSON.stringify({ ...form, enabled: true }),
      });
      if (res.ok) {
        const created = await res.json();
        setSources([...sources, created]);
        setShowForm(false);
        setForm({ name: '', type: 'prometheus', endpoint: '' });
      }
    } catch (e) {
      console.error(e);
    }
  };

  const deleteSource = async (id: string) => {
    await apiFetch(`/api/v1/admin/sources/${id}`, { method: 'DELETE' });
    setSources(sources.filter(s => s.id !== id));
  };

  return (
    <div>
      <div className="page-header">
        <h1>Data Sources</h1>
        <p>Manage telemetry backends and data pipelines</p>
      </div>

      <div style={{ marginBottom: 16 }}>
        <button className="primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancel' : '+ Add Source'}
        </button>
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="form-group">
            <label>Name</label>
            <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
          </div>
          <div className="form-group">
            <label>Type</label>
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })}>
              <option value="prometheus">Prometheus</option>
              <option value="tempo">Tempo</option>
              <option value="loki">Loki</option>
              <option value="kafka">Kafka</option>
              <option value="elasticsearch">Elasticsearch</option>
            </select>
          </div>
          <div className="form-group">
            <label>Endpoint</label>
            <input
              value={form.endpoint}
              onChange={e => setForm({ ...form, endpoint: e.target.value })}
              placeholder="http://host:port"
            />
          </div>
          <button className="primary" onClick={createSource}>Save</button>
        </div>
      )}

      {loading ? (
        <div className="loading">Loading...</div>
      ) : sources.length === 0 ? (
        <div className="empty">No data sources configured</div>
      ) : (
        <div className="card">
          <table>
            <thead>
              <tr><th>Name</th><th>Type</th><th>Endpoint</th><th>Status</th><th>Actions</th></tr>
            </thead>
            <tbody>
              {sources.map(s => (
                <tr key={s.id}>
                  <td>{s.name}</td>
                  <td><span className="badge info">{s.type}</span></td>
                  <td><code style={{ fontSize: 12 }}>{s.endpoint}</code></td>
                  <td>
                    <span className={`badge ${s.enabled ? 'success' : 'warning'}`}>
                      {s.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td>
                    <button className="danger" onClick={() => deleteSource(s.id)} style={{ padding: '4px 8px', fontSize: 12 }}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
