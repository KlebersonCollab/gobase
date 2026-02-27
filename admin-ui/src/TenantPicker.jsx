import { Database, ChevronRight, Shield, User } from 'lucide-react';
import { req, setToken, setRefreshToken } from './api';
import { useToast } from './Toast';

export default function TenantPicker({ user, tenants, onSelect }) {
    const toast = useToast();

    const selectTenant = async (tenantId) => {
        try {
            const data = await req('/api/auth/select-tenant', {
                method: 'POST',
                body: { tenant_id: tenantId },
            });
            setToken(data.token);
            if (data.refresh_token) setRefreshToken(data.refresh_token);
            onSelect(data.token);
        } catch (err) {
            toast(err.message, true);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-surface-0 px-4">
            <div className="w-full max-w-md">
                <div className="flex items-center justify-center gap-2 mb-6">
                    <div className="w-8 h-8 bg-brand rounded-lg flex items-center justify-center">
                        <Database className="w-4 h-4 text-surface-0" />
                    </div>
                    <span className="text-lg font-bold text-txt-0">GoBase Studio</span>
                </div>

                <div className="bg-surface-1 border border-border rounded-xl p-6 shadow-xl">
                    <div className="flex items-center gap-2 mb-1">
                        <User className="w-4 h-4 text-txt-2" />
                        <span className="text-xs text-txt-2">{user?.email || 'user'}</span>
                    </div>
                    <h2 className="text-base font-semibold text-txt-0 mb-4">Choose a Workspace</h2>

                    <div className="space-y-2">
                        {tenants.map(t => (
                            <button key={t.tenant_id} onClick={() => selectTenant(t.tenant_id)}
                                className="w-full flex items-center justify-between p-3.5 bg-surface-0 border border-border rounded-lg hover:border-brand hover:bg-brand/5 transition-all group">
                                <div className="flex items-center gap-3">
                                    <div className="w-9 h-9 bg-surface-2 rounded-lg flex items-center justify-center group-hover:bg-brand/10">
                                        <Database className="w-4 h-4 text-txt-2 group-hover:text-brand" />
                                    </div>
                                    <div className="text-left">
                                        <span className="text-sm font-medium text-txt-0">{t.tenant_name}</span>
                                        <span className={`ml-2 text-[10px] font-medium px-1.5 py-0.5 rounded-full ${t.role === 'admin' ? 'bg-amber-500/15 text-amber-400' : 'bg-blue-500/15 text-blue-400'}`}>
                                            <Shield className="w-2.5 h-2.5 inline mr-0.5" />{t.role}
                                        </span>
                                    </div>
                                </div>
                                <ChevronRight className="w-4 h-4 text-txt-3 group-hover:text-brand transition-colors" />
                            </button>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
}
