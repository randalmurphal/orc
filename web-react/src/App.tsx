import { useState, useEffect } from 'react';

interface HealthResponse {
  status: string;
  version?: string;
}

function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/health')
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: HealthResponse) => {
        setHealth(data);
        setLoading(false);
      })
      .catch((err: Error) => {
        setError(err.message);
        setLoading(false);
      });
  }, []);

  return (
    <div className="app">
      <header className="header">
        <h1>Orc - Task Orchestrator</h1>
        <p className="subtitle">React 19 Migration</p>
      </header>

      <main className="main">
        <section className="status-card">
          <h2>API Health Check</h2>
          {loading && <p className="loading">Checking API...</p>}
          {error && (
            <p className="error">
              API Error: {error}
              <br />
              <small>Make sure the API server is running on port 8080</small>
            </p>
          )}
          {health && (
            <p className="success">
              API Status: {health.status}
              {health.version && ` (v${health.version})`}
            </p>
          )}
        </section>

        <section className="info-card">
          <h2>Project Structure</h2>
          <ul>
            <li><code>src/components/</code> - UI components</li>
            <li><code>src/pages/</code> - Route pages</li>
            <li><code>src/stores/</code> - Zustand stores</li>
            <li><code>src/hooks/</code> - Custom hooks</li>
            <li><code>src/lib/</code> - Shared utilities</li>
          </ul>
        </section>
      </main>
    </div>
  );
}

export default App;
