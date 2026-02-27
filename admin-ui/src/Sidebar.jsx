import { LayoutDashboard, Table2, Radio, LogOut, User, Database, Users, Shield, ShieldCheck, Settings as SettingsIcon } from 'lucide-react';

export default function Sidebar({ page, setPage, user, role, onLogout }) {
    const nav = [
        { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
        { id: 'collections', label: 'Collections', icon: Table2 },
        { id: 'settings', label: 'Settings', icon: SettingsIcon },
    ];

    // Admin-only pages
    if (role === 'admin') {
        const usersIndex = nav.length - 1; // Insert before settings
        nav.splice(usersIndex, 0, { id: 'users', label: 'Users', icon: Users });
        nav.splice(usersIndex + 1, 0, { id: 'auditLog', label: 'Audit Log', icon: ShieldCheck });
    }

    return (
        <aside className="w-56 bg-surface-1 border-r border-border flex flex-col justify-between h-screen sticky top-0">
            <div>
                <div className="flex items-center gap-2.5 px-5 py-4 border-b border-border">
                    <div className="w-7 h-7 bg-brand rounded-md flex items-center justify-center flex-shrink-0">
                        <Database className="w-3.5 h-3.5 text-surface-0" />
                    </div>
                    <span className="text-sm font-semibold text-txt-0 tracking-tight">GoBase Studio</span>
                </div>
                <nav className="p-3 space-y-0.5">
                    {nav.map(n => (
                        <button key={n.id} onClick={() => setPage(n.id)}
                            className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors ${page === n.id ? 'bg-surface-2 text-txt-0' : 'text-txt-1 hover:bg-surface-2 hover:text-txt-0'}`}>
                            <n.icon className={`w-4 h-4 ${page === n.id ? 'text-brand' : ''}`} />
                            {n.label}
                        </button>
                    ))}
                </nav>
            </div>
            <div className="p-3 border-t border-border">
                <div className="flex items-center justify-between px-3 py-2">
                    <div className="flex items-center gap-2 min-w-0">
                        <div className="w-6 h-6 bg-surface-3 rounded-full flex items-center justify-center flex-shrink-0">
                            <User className="w-3 h-3 text-txt-2" />
                        </div>
                        <div className="flex flex-col min-w-0">
                            <span className="text-xs text-txt-2 truncate">{user}</span>
                            <span className={`text-[9px] font-semibold ${role === 'admin' ? 'text-amber-400' : 'text-blue-400'}`}>
                                <Shield className="w-2 h-2 inline mr-0.5" />{role}
                            </span>
                        </div>
                    </div>
                    <button onClick={onLogout} className="text-txt-2 hover:text-red-400 transition-colors" title="Logout">
                        <LogOut className="w-3.5 h-3.5" />
                    </button>
                </div>
            </div>
        </aside>
    );
}
