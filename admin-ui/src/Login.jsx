import { useState } from 'react';
import { Database, Building2, UserPlus } from 'lucide-react';
import { setToken, setRefreshToken } from './api';
import { useToast } from './Toast';

export default function Login({ onAuth }) {
    const toast = useToast();
    const [isReg, setIsReg] = useState(false);
    const [regMode, setRegMode] = useState('create'); // 'create' | 'join'
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [tenantName, setTenantName] = useState('');
    const [error, setError] = useState('');

    const handleLogin = async (e) => {
        e.preventDefault(); setError('');
        try {
            const res = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password }),
            });
            if (!res.ok) throw new Error('Invalid credentials');
            const data = await res.json();
            // Always save tokens (light or full)
            setToken(data.token);
            if (data.refresh_token) setRefreshToken(data.refresh_token);

            if (data.tenants?.length === 1) {
                // Single tenant → go straight to app
                onAuth(data.token, data.tenants);
            } else {
                // Multi-tenant → show picker (token is light, no tenant_id)
                onAuth(data.token, data.tenants, data.user);
            }
        } catch (err) { setError(err.message); }
    };

    const handleRegister = async (e) => {
        e.preventDefault(); setError('');
        if (!tenantName) { setError('Workspace name is required'); return; }
        try {
            const res = await fetch('/api/auth/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password, tenant_name: tenantName, mode: regMode }),
            });
            if (!res.ok) { const t = await res.text(); throw new Error(t); }
            toast(regMode === 'create' ? 'Workspace created! Logging in...' : 'Joined workspace! Logging in...');
            // Auto-login after register
            setTimeout(() => handleLogin({ preventDefault: () => { } }), 500);
        } catch (err) { setError(err.message); }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-surface-0 px-4">
            <div className="w-full max-w-sm">
                <div className="flex items-center justify-center gap-2 mb-8">
                    <div className="w-8 h-8 bg-brand rounded-lg flex items-center justify-center">
                        <Database className="w-4 h-4 text-surface-0" />
                    </div>
                    <span className="text-lg font-bold text-txt-0">GoBase Studio</span>
                </div>

                <form onSubmit={isReg ? handleRegister : handleLogin}
                    className="bg-surface-1 border border-border rounded-xl p-6 shadow-xl space-y-4">
                    <h2 className="text-center text-sm font-semibold text-txt-0">
                        {isReg ? 'Create Account' : 'Sign In'}
                    </h2>

                    {isReg && (
                        <div className="flex gap-1 p-1 bg-surface-0 rounded-lg border border-border">
                            <button type="button" onClick={() => setRegMode('create')}
                                className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-md text-xs font-medium transition-all ${regMode === 'create' ? 'bg-brand text-white shadow-sm' : 'text-txt-2 hover:text-txt-0'}`}>
                                <Building2 className="w-3.5 h-3.5" /> New Workspace
                            </button>
                            <button type="button" onClick={() => setRegMode('join')}
                                className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-md text-xs font-medium transition-all ${regMode === 'join' ? 'bg-brand text-white shadow-sm' : 'text-txt-2 hover:text-txt-0'}`}>
                                <UserPlus className="w-3.5 h-3.5" /> Join Existing
                            </button>
                        </div>
                    )}

                    {isReg && (
                        <div>
                            <label className="text-xs text-txt-2 mb-1 block">
                                {regMode === 'create' ? 'Workspace Name' : 'Workspace to Join'}
                            </label>
                            <input value={tenantName} onChange={e => setTenantName(e.target.value)}
                                placeholder={regMode === 'create' ? 'My Company' : 'Existing workspace name'}
                                className="w-full px-3 py-2 bg-surface-0 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none" />
                        </div>
                    )}

                    <div>
                        <label className="text-xs text-txt-2 mb-1 block">Email</label>
                        <input type="email" value={email} onChange={e => setEmail(e.target.value)}
                            className="w-full px-3 py-2 bg-surface-0 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none" />
                    </div>
                    <div>
                        <label className="text-xs text-txt-2 mb-1 block">Password</label>
                        <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                            className="w-full px-3 py-2 bg-surface-0 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none" />
                    </div>

                    {error && <p className="text-xs text-red-400 text-center">{error}</p>}

                    <button type="submit"
                        className="w-full py-2.5 bg-brand text-white text-sm font-semibold rounded-md hover:bg-brand-dark transition-colors shadow-lg">
                        {isReg ? (regMode === 'create' ? 'Create Workspace' : 'Join Workspace') : 'Sign In'}
                    </button>

                    <p className="text-center text-xs text-txt-2">
                        {isReg ? 'Already have an account? ' : "Don't have an account? "}
                        <button type="button" onClick={() => { setIsReg(!isReg); setError(''); }}
                            className="text-brand hover:underline">
                            {isReg ? 'Sign in' : 'Create one'}
                        </button>
                    </p>
                </form>
            </div>
        </div>
    );
}
