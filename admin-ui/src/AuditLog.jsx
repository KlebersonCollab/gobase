import { useState, useEffect, useRef } from 'react';
import { req } from './api';
import {
    ShieldCheck, Search, Clock, User, HardDrive,
    Zap, Info, X, ChevronRight, Filter,
    ArrowUpDown, History, Activity
} from 'lucide-react';

export default function AuditLog() {
    const [logs, setLogs] = useState([]);
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState('');
    const [selectedLog, setSelectedLog] = useState(null);
    const [sortOrder, setSortOrder] = useState('desc');

    useEffect(() => {
        loadLogs();
    }, []);

    const loadLogs = async () => {
        setLoading(true);
        try {
            const data = await req('/api/audit-logs');
            setLogs(data);
        } catch (err) {
            console.error('Failed to load logs:', err);
        } finally {
            setLoading(false);
        }
    };

    const filteredLogs = logs.filter(l =>
        l.user_email.toLowerCase().includes(filter.toLowerCase()) ||
        l.table_name.toLowerCase().includes(filter.toLowerCase()) ||
        l.action.toLowerCase().includes(filter.toLowerCase())
    ).sort((a, b) => {
        const dateA = new Date(a.created_at);
        const dateB = new Date(b.created_at);
        return sortOrder === 'desc' ? dateB - dateA : dateA - dateB;
    });

    const getActionType = (action) => {
        if (action.includes('INSERT')) return { label: 'Create', color: 'text-brand border-brand/20 bg-brand/5' };
        if (action.includes('UPDATE')) return { label: 'Update', color: 'text-amber-400 border-amber-400/20 bg-amber-400/5' };
        if (action.includes('DELETE')) return { label: 'Delete', color: 'text-red-400 border-red-400/20 bg-red-400/5' };
        if (action.includes('SCHEMA')) return { label: 'Schema', color: 'text-blue-400 border-blue-400/20 bg-blue-400/5' };
        return { label: action, color: 'text-txt-2 border-border bg-surface-2' };
    };

    return (
        <div className="flex-1 flex flex-col min-h-0 bg-surface-0 font-sans selection:bg-brand/30 selection:text-white">
            <header className="px-8 pt-8 pb-6 border-b border-border bg-surface-0/50 backdrop-blur-xl sticky top-0 z-20">
                <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6 max-w-[1600px] mx-auto w-full">
                    <div className="flex items-center gap-5">
                        <div className="relative">
                            <div className="absolute -inset-1 bg-brand/20 blur-lg rounded-full animate-pulse"></div>
                            <div className="relative p-3.5 bg-brand/10 border border-brand/20 rounded-2xl">
                                <History className="w-7 h-7 text-brand" />
                            </div>
                        </div>
                        <div>
                            <h1 className="text-3xl font-extrabold text-txt-0 tracking-tight flex items-center gap-2">
                                Audit Trails
                                <span className="text-[10px] font-black uppercase text-brand bg-brand/10 px-2 py-0.5 rounded-md border border-brand/20">Pro</span>
                            </h1>
                            <div className="flex items-center gap-2 mt-1">
                                <p className="text-txt-2 text-sm font-medium opacity-70">Security monitoring & historical record keeping</p>
                            </div>
                        </div>
                    </div>

                    <div className="flex items-center gap-4">
                        <div className="relative group min-w-[320px]">
                            <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-txt-3 group-focus-within:text-brand transition-all duration-300" />
                            <input
                                type="text"
                                placeholder="Search trails..."
                                value={filter}
                                onChange={(e) => setFilter(e.target.value)}
                                className="w-full pl-11 pr-4 py-3 bg-surface-1 border border-border rounded-2xl text-sm text-txt-0 focus:outline-none focus:ring-4 focus:ring-brand/10 focus:border-brand transition-all font-medium shadow-inner-md hover:border-txt-3"
                            />
                        </div>
                        <button
                            onClick={() => setSortOrder(prev => prev === 'desc' ? 'asc' : 'desc')}
                            className="p-3 bg-surface-1 border border-border rounded-2xl text-txt-2 hover:bg-surface-2 hover:text-txt-0 transition-all shadow-sm"
                            title="Sort by date"
                        >
                            <ArrowUpDown className="w-5 h-5" />
                        </button>
                        <button
                            onClick={loadLogs}
                            className="px-6 py-3 bg-brand text-stone-900 rounded-2xl text-sm font-black hover:bg-brand-light active:scale-95 transition-all flex items-center gap-2 shadow-lg shadow-brand/10"
                        >
                            <Zap className="w-4 h-4" /> Refresh
                        </button>
                    </div>
                </div>
            </header>

            <div className="flex-1 overflow-auto bg-surface-0/30 custom-scrollbar p-8">
                <div className="max-w-[1600px] mx-auto w-full">
                    {loading ? (
                        <div className="h-[400px] flex items-center justify-center italic text-txt-3 gap-3">
                            <div className="w-6 h-6 border-2 border-brand border-t-transparent rounded-full animate-spin"></div>
                            Syncing historical data...
                        </div>
                    ) : filteredLogs.length === 0 ? (
                        <div className="h-[400px] flex flex-col items-center justify-center text-txt-3 opacity-30 italic">
                            <ShieldCheck className="w-20 h-20 mb-6 opacity-5" />
                            <p className="text-lg font-medium">No results found for "{filter}"</p>
                            <button onClick={() => setFilter('')} className="mt-4 text-brand font-bold hover:underline">Clear all filters</button>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 gap-1">
                            {/* Table Header */}
                            <div className="grid grid-cols-[1fr,2fr,1.5fr,1.5fr,100px] gap-4 px-6 py-4 bg-surface-1/50 border border-border rounded-t-2xl text-[10px] font-black text-txt-3 uppercase tracking-widest sticky top-0 z-10 backdrop-blur-md">
                                <div>Time</div>
                                <div>Member</div>
                                <div>Action</div>
                                <div>Resource</div>
                                <div className="text-right">Details</div>
                            </div>

                            {/* Table Body */}
                            <div className="bg-surface-1/20 border-x border-b border-border rounded-b-2xl overflow-hidden divide-y divide-border/20">
                                {filteredLogs.map((log) => {
                                    const action = getActionType(log.action);
                                    return (
                                        <div key={log.id}
                                            onClick={() => setSelectedLog(log)}
                                            className="grid grid-cols-[1fr,2fr,1.5fr,1.5fr,100px] gap-4 px-6 py-5 items-center hover:bg-surface-2/40 cursor-pointer transition-all group duration-200">
                                            <div className="flex flex-col">
                                                <span className="text-xs font-bold text-txt-1">{new Date(log.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>
                                                <span className="text-[10px] font-medium text-txt-3">{new Date(log.created_at).toLocaleDateString()}</span>
                                            </div>
                                            <div className="flex items-center gap-3">
                                                <div className="w-8 h-8 rounded-full bg-surface-3 flex items-center justify-center border border-white/5 shadow-inner">
                                                    <User className="w-4 h-4 text-txt-2" />
                                                </div>
                                                <div className="flex flex-col min-w-0">
                                                    <span className="text-sm font-bold text-txt-0 truncate transition-colors group-hover:text-brand">{log.user_email}</span>
                                                    <span className="text-[10px] text-txt-3 font-medium opacity-60">Verified Admin</span>
                                                </div>
                                            </div>
                                            <div>
                                                <span className={`text-[10px] font-black px-2.5 py-1 rounded-lg border tracking-[0.05em] uppercase shadow-sm ${action.color}`}>
                                                    {action.label}
                                                </span>
                                            </div>
                                            <div className="flex items-center gap-2">
                                                <div className="p-1.5 bg-surface-3 rounded-lg border border-white/5">
                                                    <Database className="w-3.5 h-3.5 text-brand opacity-60" />
                                                </div>
                                                <span className="text-xs font-black text-txt-1 uppercase tracking-tight">{log.table_name}</span>
                                            </div>
                                            <div className="flex justify-end">
                                                <div className="w-8 h-8 rounded-xl bg-surface-2 border border-border flex items-center justify-center opacity-0 group-hover:opacity-100 group-hover:bg-brand group-hover:border-brand-light transition-all shadow-lg group-hover:scale-110">
                                                    <ChevronRight className="w-4 h-4 text-txt-0 group-hover:text-stone-900" />
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Modal Detail View */}
            {selectedLog && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 animate-in fade-in duration-300">
                    <div className="absolute inset-0 bg-black/80 backdrop-blur-md" onClick={() => setSelectedLog(null)}></div>
                    <div className="relative w-full max-w-2xl bg-surface-1 border border-border shadow-[0_0_100px_rgba(0,0,0,0.5)] rounded-3xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-200">
                        {/* Modal Header */}
                        <header className="px-8 py-6 border-b border-border flex items-center justify-between bg-surface-2/50">
                            <div className="flex items-center gap-4">
                                <div className={`p-3 rounded-2xl border ${getActionType(selectedLog.action).color}`}>
                                    <Activity className="w-6 h-6" />
                                </div>
                                <div>
                                    <h2 className="text-xl font-black text-txt-0 tracking-tight">Event Specification</h2>
                                    <p className="text-xs text-txt-3 font-medium mt-0.5">Reference ID: <span className="font-mono">{selectedLog.id}</span></p>
                                </div>
                            </div>
                            <button
                                onClick={() => setSelectedLog(null)}
                                className="p-2.5 hover:bg-surface-3 rounded-xl transition-all border border-border text-txt-3 hover:text-txt-0 active:scale-95"
                            >
                                <X className="w-5 h-5" />
                            </button>
                        </header>

                        {/* Modal Body */}
                        <div className="flex-1 overflow-auto p-8 custom-scrollbar space-y-8">
                            {/* Summary Cards */}
                            <div className="grid grid-cols-2 gap-4">
                                <div className="p-4 bg-surface-2/40 rounded-2xl border border-border">
                                    <label className="text-[10px] font-black uppercase text-txt-3 tracking-widest block mb-2">Subject</label>
                                    <div className="flex items-center gap-2">
                                        <User className="w-4 h-4 text-brand" />
                                        <span className="text-sm font-bold text-txt-1">{selectedLog.user_email}</span>
                                    </div>
                                </div>
                                <div className="p-4 bg-surface-2/40 rounded-2xl border border-border">
                                    <label className="text-[10px] font-black uppercase text-txt-3 tracking-widest block mb-2">Resource Path</label>
                                    <div className="flex items-center gap-2">
                                        <Database className="w-4 h-4 text-brand" />
                                        <span className="text-sm font-black text-txt-1 uppercase">/public/{selectedLog.table_name}</span>
                                    </div>
                                </div>
                            </div>

                            {/* Payload Section */}
                            <div>
                                <div className="flex items-center justify-between mb-3 px-1">
                                    <label className="text-[10px] font-black uppercase text-txt-3 tracking-widest">Mutation Payload</label>
                                    <span className="text-[10px] font-mono text-brand opacity-60">JSON Schema v1.0</span>
                                </div>
                                <div className="relative group">
                                    <div className="absolute -inset-0.5 bg-brand/5 blur-md opacity-0 group-hover:opacity-100 transition-opacity"></div>
                                    <pre className="relative p-6 bg-black/60 rounded-2xl border border-white/5 font-mono text-xs text-brand leading-relaxed overflow-auto custom-scrollbar shadow-inner ring-1 ring-white/5">
                                        {JSON.stringify(selectedLog.payload, null, 2)}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex items-center gap-2 text-[10px] font-medium text-txt-3 bg-surface-2/30 p-4 rounded-xl border border-border italic">
                                <Info className="w-3.5 h-3.5 text-brand shrink-0" />
                                This log entry is immutable and was cryptographically signed by the GoBase core engine at capture.
                            </div>
                        </div>

                        {/* Modal Footer */}
                        <footer className="px-8 py-5 border-t border-border bg-surface-2/50 flex justify-end">
                            <button
                                onClick={() => setSelectedLog(null)}
                                className="px-8 py-3 bg-surface-3 border border-border rounded-xl text-sm font-bold text-txt-1 hover:bg-surface-4 hover:text-txt-0 transition-all shadow-sm"
                            >
                                Close Inspection
                            </button>
                        </footer>
                    </div>
                </div>
            )}
        </div>
    );
}

// Sub-component for icons to keep it clean
function Database({ className }) {
    return (
        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={className}><ellipse cx="12" cy="5" rx="9" ry="3" /><path d="M3 5V19A9 3 0 0 0 21 19V5" /><path d="M3 12A9 3 0 0 0 21 12" /></svg>
    );
}
