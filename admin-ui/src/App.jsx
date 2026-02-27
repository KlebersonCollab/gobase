import { useState } from 'react';
import { getToken, setToken, clearToken, parseJwt, setRefreshToken } from './api';
import { ToastProvider } from './Toast';
import Login from './Login';
import TenantPicker from './TenantPicker';
import Sidebar from './Sidebar';
import Dashboard from './Dashboard';
import Collections from './Collections';
import Users from './Users';
import Settings from './Settings';
import AuditLog from './AuditLog';
import RealtimeConsole from './RealtimeConsole';

function AppInner() {
  const [token, setTokenState] = useState(getToken());
  const [page, setPage] = useState('dashboard');
  const [pendingTenants, setPendingTenants] = useState(null);
  const [pendingUser, setPendingUser] = useState(null);

  const claims = token ? parseJwt(token) : {};
  const hasTenant = !!claims.tenant_id;

  const handleAuth = (t, tenants, user) => {
    setTokenState(t);
    if (tenants && tenants.length > 1) {
      setPendingTenants(tenants);
      setPendingUser(user);
    } else {
      setPendingTenants(null);
      setPendingUser(null);
    }
  };

  const handleTenantSelect = (t) => {
    setTokenState(t);
    setPendingTenants(null);
    setPendingUser(null);
  };

  const handleLogout = () => {
    clearToken();
    setTokenState('');
    setPendingTenants(null);
    setPendingUser(null);
    setPage('dashboard');
  };

  const handleSwitchTenant = async (newTenantId) => {
    try {
      const res = await fetch('/api/auth/select-tenant', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ tenant_id: newTenantId }),
      });
      if (!res.ok) throw new Error('Failed to switch workspace');
      const data = await res.json();
      setToken(data.token);
      setTokenState(data.token);
      if (data.refresh_token) setRefreshToken(data.refresh_token);
      setPage('dashboard');
    } catch (err) {
      console.error(err);
    }
  };

  if (!token) return <Login onAuth={handleAuth} />;
  if (token && !hasTenant && pendingTenants) return <TenantPicker user={pendingUser} tenants={pendingTenants} onSelect={handleTenantSelect} />;
  if (token && !hasTenant) return <Login onAuth={handleAuth} />;

  const user = claims.email || 'user';
  const role = claims.role || 'member';
  const currentTenantId = claims.tenant_id;

  return (
    <div className="flex min-h-screen">
      <Sidebar page={page} setPage={setPage} user={user} role={role} onLogout={handleLogout} />
      <main className="flex-1 flex flex-col min-h-screen overflow-hidden relative">
        {page === 'dashboard' && <Dashboard />}
        {page === 'collections' && <Collections />}
        {page === 'users' && <Users />}
        {page === 'auditLog' && <AuditLog />}
        {page === 'settings' && <Settings currentTenantId={currentTenantId} onSwitchTenant={handleSwitchTenant} />}

        <RealtimeConsole />
      </main>
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <AppInner />
    </ToastProvider>
  );
}
