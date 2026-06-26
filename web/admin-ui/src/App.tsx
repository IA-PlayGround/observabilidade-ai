import { Routes, Route, NavLink } from 'react-router-dom';
import { useState, useEffect } from 'react';
import Dashboard from './pages/Dashboard';
import Correlations from './pages/Correlations';
import AIAnalysis from './pages/AIAnalysis';
import DataSources from './pages/DataSources';
import SystemHealth from './pages/SystemHealth';
import Login from './pages/Login';
import { AuthProvider, useAuth } from './hooks/useAuth';

function AppLayout() {
  const { token, logout } = useAuth();

  if (!token) return <Login />;

  const navItems = [
    { to: '/', label: 'Dashboard', icon: '📊' },
    { to: '/correlations', label: 'Correlations', icon: '🔗' },
    { to: '/ai', label: 'AI Analysis', icon: '🤖' },
    { to: '/sources', label: 'Data Sources', icon: '🗄' },
    { to: '/health', label: 'System Health', icon: '❤️' },
  ];

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-logo">Spectrum</div>
        <nav>
          {navItems.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) => isActive ? 'active' : ''}
            >
              <span>{item.icon}</span>
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div style={{ padding: '16px 20px', borderTop: '1px solid var(--color-border)' }}>
          <button onClick={logout} style={{ width: '100%' }}>Logout</button>
        </div>
      </aside>
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/correlations" element={<Correlations />} />
          <Route path="/ai" element={<AIAnalysis />} />
          <Route path="/sources" element={<DataSources />} />
          <Route path="/health" element={<SystemHealth />} />
        </Routes>
      </main>
    </div>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/*" element={<AppLayout />} />
      </Routes>
    </AuthProvider>
  );
}
