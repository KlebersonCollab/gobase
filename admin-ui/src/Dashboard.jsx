import { useEffect, useState } from 'react';
import { Table2, Radio, ShieldCheck } from 'lucide-react';
import { req } from './api';

export default function Dashboard() {
    const [count, setCount] = useState('—');

    useEffect(() => {
        req('/api/schema/tables').then(t => setCount((t || []).length)).catch(() => { });
    }, []);

    const cards = [
        { label: 'Collections', value: count, icon: Table2, color: 'text-brand', bg: 'bg-brand/10' },
        { label: 'Realtime', value: 'Connected', icon: Radio, color: 'text-blue-400', bg: 'bg-blue-500/10' },
        { label: 'Auth / RLS', value: 'Active', icon: ShieldCheck, color: 'text-amber-400', bg: 'bg-amber-500/10' },
    ];

    return (
        <div className="flex-1 flex flex-col">
            <header className="px-8 py-6 border-b border-border">
                <h1 className="text-xl font-semibold text-txt-0">Dashboard</h1>
                <p className="text-sm text-txt-2 mt-1">Overview of your GoBase instance</p>
            </header>
            <div className="p-8 grid grid-cols-1 md:grid-cols-3 gap-4">
                {cards.map(c => (
                    <div key={c.label} className="bg-surface-1 border border-border rounded-lg p-5">
                        <div className="flex items-center gap-3 mb-3">
                            <div className={`w-9 h-9 ${c.bg} rounded-lg flex items-center justify-center`}>
                                <c.icon className={`w-4 h-4 ${c.color}`} />
                            </div>
                            <span className="text-sm text-txt-1">{c.label}</span>
                        </div>
                        <p className="text-2xl font-semibold text-txt-0">{c.value}</p>
                    </div>
                ))}
            </div>
        </div>
    );
}
