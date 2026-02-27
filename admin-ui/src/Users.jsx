import { useState, useEffect, useCallback } from 'react';
import { req } from './api';
import { useToast } from './Toast';
import { Users as UsersIcon, Shield, ShieldCheck, Trash2, ChevronDown, ChevronRight } from 'lucide-react';

export default function Users() {
    const toast = useToast();
    const [users, setUsers] = useState([]);
    const [tables, setTables] = useState([]);
    const [expanded, setExpanded] = useState(null); // user id
    const [perms, setPerms] = useState([]); // current user's permissions

    const loadUsers = useCallback(async () => {
        try {
            const data = await req('/api/users');
            setUsers(data || []);
        } catch (e) { toast(e.message, true); }
    }, []);

    const loadTables = useCallback(async () => {
        try {
            const data = await req('/api/schema/tables');
            setTables(data || []);
        } catch (e) { /* ignore for non-admin */ }
    }, []);

    useEffect(() => { loadUsers(); loadTables(); }, []);

    const expandUser = async (userId) => {
        if (expanded === userId) { setExpanded(null); return; }
        try {
            const data = await req(`/api/users/${userId}/permissions`);
            setPerms(data || []);
            setExpanded(userId);
        } catch (e) { toast(e.message, true); }
    };

    const getPermForTable = (tableName) => {
        return perms.find(p => p.table_name === tableName) || {
            table_name: tableName, can_read: false, can_create: false, can_update: false, can_delete: false
        };
    };

    const togglePerm = (tableName, action) => {
        setPerms(prev => {
            const existing = prev.find(p => p.table_name === tableName);
            if (existing) {
                return prev.map(p => p.table_name === tableName ? { ...p, [action]: !p[action] } : p);
            }
            return [...prev, { table_name: tableName, can_read: false, can_create: false, can_update: false, can_delete: false, [action]: true }];
        });
    };

    const savePerms = async (userId) => {
        try {
            await req(`/api/users/${userId}/permissions`, { method: 'PUT', body: perms });
            toast('Permissions saved!');
        } catch (e) { toast(e.message, true); }
    };

    const removeUser = async (userId) => {
        if (!confirm('Remove this user from the workspace?')) return;
        try {
            await req(`/api/users/${userId}`, { method: 'DELETE' });
            toast('User removed');
            loadUsers();
        } catch (e) { toast(e.message, true); }
    };

    const actions = ['can_read', 'can_create', 'can_update', 'can_delete'];
    const actionLabels = { can_read: 'Read', can_create: 'Create', can_update: 'Update', can_delete: 'Delete' };
    const actionColors = { can_read: 'bg-blue-500', can_create: 'bg-green-500', can_update: 'bg-amber-500', can_delete: 'bg-red-500' };

    return (
        <div className="flex-1 overflow-y-auto p-6 bg-surface-0">
            <div className="max-w-4xl mx-auto">
                <div className="flex items-center gap-3 mb-6">
                    <div className="w-10 h-10 bg-brand/10 rounded-xl flex items-center justify-center">
                        <UsersIcon className="w-5 h-5 text-brand" />
                    </div>
                    <div>
                        <h1 className="text-xl font-bold text-txt-0">Users & Permissions</h1>
                        <p className="text-xs text-txt-2">Manage who can access your collections and what they can do.</p>
                    </div>
                </div>

                <div className="space-y-2">
                    {users.map(u => {
                        const isExpanded = expanded === u.id;
                        return (
                            <div key={u.id} className="bg-surface-1 border border-border rounded-xl overflow-hidden">
                                {/* User header */}
                                <button onClick={() => expandUser(u.id)}
                                    className="w-full flex items-center justify-between px-5 py-3.5 hover:bg-surface-0/50 transition-colors">
                                    <div className="flex items-center gap-3">
                                        <div className="w-8 h-8 bg-surface-2 rounded-full flex items-center justify-center">
                                            <span className="text-xs font-bold text-txt-1">{u.email?.[0]?.toUpperCase()}</span>
                                        </div>
                                        <div className="text-left">
                                            <span className="text-sm font-medium text-txt-0">{u.email}</span>
                                            <span className={`ml-2 text-[10px] font-semibold px-2 py-0.5 rounded-full ${u.role === 'admin' ? 'bg-amber-500/15 text-amber-400' : 'bg-blue-500/15 text-blue-400'}`}>
                                                {u.role === 'admin' ? <ShieldCheck className="w-2.5 h-2.5 inline mr-0.5" /> : <Shield className="w-2.5 h-2.5 inline mr-0.5" />}
                                                {u.role}
                                            </span>
                                        </div>
                                    </div>
                                    <div className="flex items-center gap-2">
                                        {u.role !== 'admin' && (
                                            <button onClick={(e) => { e.stopPropagation(); removeUser(u.id); }}
                                                className="p-1.5 text-txt-3 hover:text-red-400 transition-colors rounded-md hover:bg-red-500/10">
                                                <Trash2 className="w-3.5 h-3.5" />
                                            </button>
                                        )}
                                        {isExpanded ? <ChevronDown className="w-4 h-4 text-txt-3" /> : <ChevronRight className="w-4 h-4 text-txt-3" />}
                                    </div>
                                </button>

                                {/* Permission matrix */}
                                {isExpanded && u.role !== 'admin' && (
                                    <div className="border-t border-border px-5 py-4 bg-surface-0/30">
                                        <p className="text-[11px] text-txt-3 mb-3">Toggle permissions for each collection. Changes are saved manually.</p>

                                        {tables.length === 0 ? (
                                            <p className="text-xs text-txt-3 italic">No collections yet.</p>
                                        ) : (
                                            <div className="overflow-x-auto">
                                                <table className="w-full text-xs">
                                                    <thead>
                                                        <tr className="border-b border-border">
                                                            <th className="text-left py-2 pr-4 font-medium text-txt-2 w-1/3">Collection</th>
                                                            {actions.map(a => (
                                                                <th key={a} className="text-center py-2 px-3 font-medium text-txt-2">{actionLabels[a]}</th>
                                                            ))}
                                                        </tr>
                                                    </thead>
                                                    <tbody>
                                                        {tables.map(t => {
                                                            const p = getPermForTable(t);
                                                            return (
                                                                <tr key={t} className="border-b border-border/50 hover:bg-surface-1/50">
                                                                    <td className="py-2.5 pr-4 font-mono text-txt-0">{t}</td>
                                                                    {actions.map(a => (
                                                                        <td key={a} className="text-center py-2.5 px-3">
                                                                            <button onClick={() => togglePerm(t, a)}
                                                                                className={`relative w-8 h-4.5 rounded-full transition-colors ${p[a] ? actionColors[a] : 'bg-surface-3'}`}>
                                                                                <span className={`absolute top-0.5 w-3.5 h-3.5 bg-white rounded-full shadow transition-transform ${p[a] ? 'left-[16px]' : 'left-0.5'}`} />
                                                                            </button>
                                                                        </td>
                                                                    ))}
                                                                </tr>
                                                            );
                                                        })}
                                                    </tbody>
                                                </table>
                                            </div>
                                        )}

                                        <div className="flex justify-end mt-4">
                                            <button onClick={() => savePerms(u.id)}
                                                className="px-4 py-2 bg-brand text-white text-xs font-semibold rounded-md hover:bg-brand-dark transition-colors shadow-lg flex items-center gap-1.5">
                                                <ShieldCheck className="w-3.5 h-3.5" /> Save Permissions
                                            </button>
                                        </div>
                                    </div>
                                )}

                                {isExpanded && u.role === 'admin' && (
                                    <div className="border-t border-border px-5 py-4 bg-surface-0/30">
                                        <div className="flex items-center gap-2 text-xs text-amber-400">
                                            <ShieldCheck className="w-4 h-4" />
                                            <span>Admins have full access to all collections. No permissions needed.</span>
                                        </div>
                                    </div>
                                )}
                            </div>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}
