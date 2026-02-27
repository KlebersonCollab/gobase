import { useEffect, useRef, useState } from 'react';
import { getToken } from './api';
import { Terminal, X, ChevronDown, ChevronUp, Radio } from 'lucide-react';

export default function RealtimeConsole() {
    const [events, setEvents] = useState([]);
    const [isOpen, setIsOpen] = useState(false);
    const [isMinimized, setIsMinimized] = useState(true);
    const wsRef = useRef(null);
    const scrollRef = useRef(null);
    const eventCounter = useRef(0);

    useEffect(() => {
        let retryCount = 0;
        let timeoutId = null;

        const connect = () => {
            const token = getToken();
            if (!token) return;

            const proto = location.protocol === 'https:' ? 'wss' : 'ws';
            const ws = new WebSocket(`${proto}://${location.host}/api/realtime?token=${token}`);
            wsRef.current = ws;

            ws.onmessage = (e) => {
                try {
                    const payload = JSON.parse(e.data);
                    const now = new Date().toLocaleTimeString('en', { hour12: false });
                    eventCounter.current += 1;

                    setEvents(prev => [{
                        ...payload,
                        time: now,
                        // Use a combination of timestamp and counter for guaranteed uniqueness
                        id: `${Date.now()}-${eventCounter.current}`
                    }, ...prev].slice(0, 100));
                } catch { }
            };

            ws.onerror = () => {
                ws.close();
            };

            ws.onclose = (e) => {
                if (e.code === 4001 || e.reason === "Unauthorized") {
                    console.log("Realtime: Unauthorized, stopping retries.");
                    return;
                }

                // Exponential backoff
                const delay = Math.min(30000, 1000 * Math.pow(2, retryCount));
                retryCount++;
                timeoutId = setTimeout(connect, delay);
            };

            ws.onopen = () => {
                retryCount = 0;
            };
        };

        if (isOpen) connect();

        return () => {
            wsRef.current?.close();
            if (timeoutId) clearTimeout(timeoutId);
        };
    }, [isOpen]);

    useEffect(() => {
        if (scrollRef.current && !isMinimized) {
            scrollRef.current.scrollTop = 0;
        }
    }, [events, isMinimized]);

    const colors = {
        INSERT: 'text-brand',
        UPDATE: 'text-amber-400',
        DELETE: 'text-red-400',
        SCHEMA_ADD_COLUMN: 'text-blue-400',
        SCHEMA_DROP_COLUMN: 'text-pink-400',
        SCHEMA_CREATE_TABLE: 'text-purple-400'
    };

    if (!isOpen) {
        return (
            <button
                onClick={() => { setIsOpen(true); setIsMinimized(false); }}
                className="fixed bottom-6 right-6 p-3 bg-surface-2 border border-border rounded-full shadow-2xl hover:border-brand transition-all group z-40"
                title="Open Realtime Console"
            >
                <div className="relative">
                    <Terminal className="w-5 h-5 text-txt-2 group-hover:text-brand transition-colors" />
                    <span className="absolute -top-1 -right-1 block h-2.5 w-2.5 rounded-full bg-brand border-2 border-surface-2 pulse-dot"></span>
                </div>
            </button>
        );
    }

    return (
        <div className={`fixed bottom-0 right-6 z-40 flex flex-col transition-all duration-300 ${isMinimized ? 'h-11' : 'h-[500px]'} w-[600px] bg-surface-1 border-x border-t border-border rounded-t-xl shadow-2xl overflow-hidden`}>
            {/* Header */}
            <header className="px-4 py-2 bg-surface-2 border-b border-border flex items-center justify-between flex-shrink-0 cursor-pointer select-none group"
                onClick={() => setIsMinimized(!isMinimized)}>
                <div className="flex items-center gap-2.5">
                    <Radio className={`w-3.5 h-3.5 ${events.length > 0 ? 'text-brand animate-pulse' : 'text-txt-3'}`} />
                    <span className="text-xs font-bold text-txt-0 tracking-wide uppercase opacity-80">System Events</span>
                    {events.length > 0 && (
                        <span className="text-[10px] bg-brand/20 text-brand px-2 py-0.5 rounded-full font-mono font-bold border border-brand/30">
                            {events.length}
                        </span>
                    )}
                </div>
                <div className="flex items-center gap-1">
                    <button className="p-1 hover:bg-surface-3 rounded transition-colors text-txt-3 hover:text-txt-0">
                        {isMinimized ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                    </button>
                    <button onClick={(e) => { e.stopPropagation(); setIsOpen(false); }} className="p-1 hover:bg-red-500/10 rounded transition-colors text-txt-3 hover:text-red-400">
                        <X className="w-4 h-4" />
                    </button>
                </div>
            </header>

            {/* Event List */}
            {!isMinimized && (
                <div ref={scrollRef} className="flex-1 overflow-y-auto p-0 bg-surface-0/30 custom-scrollbar">
                    {events.length === 0 ? (
                        <div className="h-full flex flex-col items-center justify-center text-txt-3 text-xs italic opacity-40">
                            <Terminal className="w-10 h-10 mb-3 opacity-10" />
                            Listening for live updates...
                        </div>
                    ) : (
                        <div className="divide-y divide-border/20">
                            {events.map(ev => (
                                <div key={ev.id} className="p-3 hover:bg-surface-2/50 transition-colors animate-in fade-in slide-in-from-bottom-1 duration-200">
                                    <div className="flex items-center gap-3 mb-1.5 overflow-hidden">
                                        <span className="text-[10px] text-txt-2 font-mono shrink-0 bg-surface-3 px-1.5 py-0.5 rounded shadow-inner uppercase">{ev.time}</span>
                                        <span className={`text-[10px] font-bold ${colors[ev.action] || 'text-txt-1'} shrink-0 uppercase tracking-wider bg-surface-2 px-2 py-0.5 rounded border border-border/50`}>
                                            {ev.action}
                                        </span>
                                        <span className="text-xs text-brand font-semibold truncate hover:text-brand-light transition-colors cursor-default">
                                            {ev.table}
                                        </span>
                                    </div>
                                    <div className="font-mono text-[11px] text-txt-1 leading-relaxed break-all bg-black/20 p-2 rounded border border-white/5 shadow-inner">
                                        {typeof ev.data === 'object' ? JSON.stringify(ev.data, null, 2) : ev.data}
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
