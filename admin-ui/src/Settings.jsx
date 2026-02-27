import { useState, useEffect, useCallback } from 'react';
import { req } from './api';
import { useToast } from './Toast';
import { Settings as SettingsIcon, Database, Plus, Trash2, CheckCircle2 } from 'lucide-react';

export default function Settings({ currentTenantId, onSwitchTenant }) {
    const toast = useToast();
    const [tenants, setTenants] = useState([]);
    const [newTenantName, setNewTenantName] = useState('');

    const loadTenants = useCallback(async () => {
        try {
            const data = await req('/api/auth/tenants');
            setTenants(data || []);
        } catch (e) { toast(e.message, true); }
    }, [toast]);

    useEffect(() => { loadTenants(); }, [loadTenants]);

    const handleCreate = async (e) => {
        e.preventDefault();
        if (!newTenantName.trim()) return;
        try {
            await req('/api/auth/tenants', { method: 'POST', body: { name: newTenantName } });
            toast('Workspace created successfully!');
            setNewTenantName('');
            loadTenants();
        } catch (err) { toast(err.message, true); }
    };

    const handleDelete = async (id) => {
        if (!confirm('Are you sure you want to delete this workspace? This will destroy ALL data and collections inside it. This action cannot be undone.')) return;
        try {
            await req(`/api/auth/tenants/${id}`, { method: 'DELETE' });
            toast('Workspace deleted.');
            if (id === currentTenantId) {
                // If they deleted the active workspace, force them to log in / pick again
                window.location.reload();
            } else {
                loadTenants();
            }
        } catch (err) { toast(err.message, true); }
    };

    return (
        <div className="flex-1 overflow-y-auto p-6 bg-surface-0">
            <div className="max-w-3xl mx-auto space-y-6">
                <div className="flex items-center gap-3 mb-6">
                    <div className="w-10 h-10 bg-surface-2 rounded-xl flex items-center justify-center">
                        <SettingsIcon className="w-5 h-5 text-txt-1" />
                    </div>
                    <div>
                        <h1 className="text-xl font-bold text-txt-0">Settings & Workspaces</h1>
                        <p className="text-xs text-txt-2">Manage your workspaces, create new ones, or switch contexts.</p>
                    </div>
                </div>

                {/* Create New Workspace */}
                <div className="bg-surface-1 border border-border rounded-xl p-5">
                    <h2 className="text-sm font-semibold text-txt-0 mb-3">Create Workspace</h2>
                    <form onSubmit={handleCreate} className="flex gap-3">
                        <input type="text" placeholder="Workspace Name (e.g. Acme Corp)"
                            value={newTenantName} onChange={e => setNewTenantName(e.target.value)}
                            className="flex-1 px-3 py-2 bg-surface-0 border border-border rounded-md text-sm text-txt-0 outline-none focus:border-brand transition-colors" />
                        <button type="submit" disabled={!newTenantName.trim()}
                            className="px-4 py-2 bg-brand text-surface-0 font-medium text-sm rounded-md hover:bg-brand-dark transition-colors disabled:opacity-50 flex items-center gap-2">
                            <Plus className="w-4 h-4" /> Create
                        </button>
                    </form>
                </div>

                {/* List Workspaces */}
                <div className="bg-surface-1 border border-border rounded-xl overflow-hidden">
                    <div className="px-5 py-4 border-b border-border bg-surface-2/50">
                        <h2 className="text-sm font-semibold text-txt-0">Your Workspaces</h2>
                    </div>
                    <div className="divide-y divide-border">
                        {tenants.map(t => {
                            const isActive = t.tenant_id === currentTenantId;
                            return (
                                <div key={t.tenant_id} className={`p-5 flex items-center justify-between transition-colors ${isActive ? 'bg-brand/5' : 'hover:bg-surface-0/50'}`}>
                                    <div className="flex items-center gap-4">
                                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${isActive ? 'bg-brand/20' : 'bg-surface-2'}`}>
                                            <Database className={`w-5 h-5 ${isActive ? 'text-brand' : 'text-txt-2'}`} />
                                        </div>
                                        <div>
                                            <div className="flex items-center gap-2">
                                                <span className="text-sm font-medium text-txt-0">{t.tenant_name}</span>
                                                {isActive && <CheckCircle2 className="w-3.5 h-3.5 text-brand" />}
                                            </div>
                                            <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded-full mt-1 inline-block ${t.role === 'admin' ? 'bg-amber-500/15 text-amber-400' : 'bg-blue-500/15 text-blue-400'}`}>
                                                {t.role}
                                            </span>
                                        </div>
                                    </div>

                                    <div className="flex items-center gap-3">
                                        {!isActive && (
                                            <button onClick={() => onSwitchTenant(t.tenant_id)}
                                                className="px-3 py-1.5 text-xs font-medium bg-surface-2 text-txt-1 hover:bg-brand hover:text-white rounded transition-colors">
                                                Switch to here
                                            </button>
                                        )}
                                        {t.role === 'admin' && (
                                            <button onClick={() => handleDelete(t.tenant_id)}
                                                className="p-2 text-txt-3 hover:text-red-400 hover:bg-red-400/10 rounded-md transition-colors" title="Delete Workspace">
                                                <Trash2 className="w-4 h-4" />
                                            </button>
                                        )}
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                </div>

            </div>
        </div>
    );
}
