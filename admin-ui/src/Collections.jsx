import { useState, useEffect, useCallback } from 'react';
import { req } from './api';
import { useToast } from './Toast';
import {
    Table2, Plus, Search, ArrowRight, ArrowLeft, Columns, Rows3, FilePlus,
    Terminal, Trash2, Pencil, Link, Unlink, RefreshCw, Table, Braces, Send, X, Shield, ShieldCheck, Settings, Layout
} from 'lucide-react';
import DynamicForm from './components/DynamicForm';

export default function Collections() {
    const toast = useToast();
    const SYSTEM_COLUMNS = ['id', 'created_at', 'modified_at', 'created_by', 'modified_by', 'tenant_id', 'id_key'];
    const [tables, setTables] = useState([]);
    const [current, setCurrent] = useState('');
    const [tab, setTab] = useState('schema');
    const [showNew, setShowNew] = useState(false);
    const [newName, setNewName] = useState('');

    // Schema state
    const [columns, setColumns] = useState([]);
    const [relations, setRelations] = useState([]);
    const [addCol, setAddCol] = useState({ name: '', type: 'text', required: false });
    const [editCol, setEditCol] = useState(null); // { name, type, required }
    const [editColDraft, setEditColDraft] = useState({ name: '', type: 'text' });
    const [auditLogs, setAuditLogs] = useState([]);

    const highlightJson = (obj) => {
        const jsonStr = JSON.stringify(obj, null, 2);
        const parts = [];
        const regex = /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g;
        let lastIdx = 0;
        let match;

        while ((match = regex.exec(jsonStr)) !== null) {
            // Add plain text before match
            if (match.index > lastIdx) {
                parts.push(jsonStr.substring(lastIdx, match.index));
            }

            const token = match[0];
            let cls = 'text-blue-400'; // number
            if (/^"/.test(token)) {
                if (/:$/.test(token)) cls = 'text-brand'; // key
                else cls = 'text-green-400'; // string
            } else if (/true|false/.test(token)) cls = 'text-amber-400'; // boolean
            else if (/null/.test(token)) cls = 'text-txt-3'; // null

            parts.push(<span key={match.index} className={cls}>{token}</span>);
            lastIdx = regex.lastIndex;
        }

        // Add remaining text
        if (lastIdx < jsonStr.length) {
            parts.push(jsonStr.substring(lastIdx));
        }

        return parts;
    };
    const [fk, setFk] = useState({ col: '', refTable: '', onDelete: 'CASCADE' });

    // Data state
    const [dataView, setDataView] = useState('table');
    const [rows, setRows] = useState([]);
    const [filters, setFilters] = useState([]);
    const [filterDraft, setFilterDraft] = useState({ col: '', op: 'eq', val: '' });

    // Insert state
    const [insertData, setInsertData] = useState({});
    const [insertMode, setInsertMode] = useState('form'); // 'form' or 'json'
    const [insertJson, setInsertJson] = useState('{}');

    // Edit modal
    const [editRow, setEditRow] = useState(null);
    const [editData, setEditData] = useState({});
    const [editJson, setEditJson] = useState('');

    // Rule modal
    const [ruleCol, setRuleCol] = useState(null);
    const [ruleColType, setRuleColType] = useState('');
    const [ruleDraft, setRuleDraft] = useState({ notEmptyOn: false, minOn: false, min: '', maxOn: false, max: '', patternOn: false, pattern: '', emailOn: false });

    // API Console state
    const [apiMethod, setApiMethod] = useState('GET');
    const [apiPath, setApiPath] = useState('');
    const [apiBody, setApiBody] = useState('{}');
    const [apiResponse, setApiResponse] = useState(null);
    const [apiMeta, setApiMeta] = useState(null);
    const [apiLoading, setApiLoading] = useState(false);

    const testEndpoint = async () => {
        setApiLoading(true);
        const start = performance.now();
        try {
            const options = { method: apiMethod };
            if (['POST', 'PUT', 'PATCH'].includes(apiMethod)) {
                options.body = apiBody;
            }
            const res = await fetch(apiPath, {
                ...options,
                headers: {
                    ...options.headers,
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('gobase_token')}`
                }
            });
            const duration = (performance.now() - start).toFixed(0);
            const status = res.status;
            let data;
            try { data = await res.json(); } catch { data = { message: 'No JSON response' }; }

            setApiResponse(data);
            setApiMeta({ status, time: duration });
            if (!res.ok) toast(`API Error: ${status}`, true);
        } catch (err) {
            setApiResponse({ error: err.message });
            setApiMeta({ status: 'ERR', time: 0 });
            toast(err.message, true);
        } finally {
            setApiLoading(false);
        }
    };

    // Load tables list
    const loadTables = useCallback(async () => {
        try {
            const t = await req('/api/schema/tables');
            setTables(t || []);
        } catch { }
    }, []);

    useEffect(() => { loadTables(); }, [loadTables]);

    // Open a collection
    const openCollection = async (name) => {
        try {
            const t = await req('/api/schema/tables');
            if (!t || !t.includes(name)) {
                toast(`Collection "${name}" not found. Use "+ New" to create it.`, true);
                return;
            }
        } catch { return; }
        setCurrent(name);
        setFilters([]);
        setTab('schema');
        setApiPath(`/api/collections/${name}`);
        setApiResponse(null);
        setApiMeta(null);
    };

    // Back to list
    const backToList = () => { setCurrent(''); loadTables(); };

    // Load schema
    useEffect(() => {
        if (!current) return;
        req(`/api/schema/tables/${current}/columns`).then(c => setColumns(c || [])).catch(() => { });
        req(`/api/schema/tables/${current}/relations`).then(r => setRelations(r || [])).catch(() => { });
    }, [current]);

    // Generate insert template
    useEffect(() => {
        if (!current || !columns.length) return;
        const tpl = {};
        columns.forEach(c => {
            if (SYSTEM_COLUMNS.includes(c.name)) return;
            if (c.type === 'text' || c.type === 'character varying') tpl[c.name] = '';
            else if (c.type === 'integer' || c.type === 'bigint') tpl[c.name] = 0;
            else if (c.type === 'boolean') tpl[c.name] = false;
            else if (c.type === 'jsonb') tpl[c.name] = {};
            else tpl[c.name] = '';
        });
        setInsertData(tpl);
        setInsertJson(JSON.stringify(tpl, null, 2));
    }, [current, columns]);

    // Load data
    const loadData = useCallback(async () => {
        if (!current) return;
        const params = new URLSearchParams();
        filters.forEach(f => params.set(f.col, `${f.op}.${f.val}`));
        try {
            const d = await req(`/api/collections/${current}${params.toString() ? '?' + params : ''}`);
            setRows(d || []);
        } catch { setRows([]); }
    }, [current, filters]);

    useEffect(() => { if (tab === 'data') loadData(); }, [tab, loadData]);

    // CRUD operations
    const createTable = async () => {
        if (!newName) return;
        try {
            await req('/api/schema/tables', { method: 'POST', body: { name: newName } });
            toast(`Collection [${newName}] created`);
            setShowNew(false);
            loadTables();
            openCollection(newName);
            setNewName('');
        } catch (e) { toast(e.message, true); }
    };

    const dropTable = async (name) => {
        if (!confirm(`DROP TABLE "${name}"? All data will be permanently lost!`)) return;
        try {
            await req(`/api/schema/tables/${name}`, { method: 'DELETE' });
            toast(`Collection [${name}] dropped`);
            if (current === name) backToList();
            else loadTables();
        } catch (e) { toast(e.message, true); }
    };

    const addColumn = async () => {
        if (!addCol.name) return toast('Column name is required', true);
        try {
            await req(`/api/schema/tables/${current}/columns`, { method: 'POST', body: addCol });
            toast(`Column [${addCol.name}] added`);
            setAddCol({ name: '', type: 'text', required: false });
            const c = await req(`/api/schema/tables/${current}/columns`);
            setColumns(c || []);
        } catch (e) { toast(e.message, true); }
    };

    const dropColumn = async (col) => {
        if (!confirm(`Drop column '${col}'? Data will be permanently lost.`)) return;
        try {
            await req(`/api/schema/tables/${current}/columns/${col}`, { method: 'DELETE' });
            toast(`Dropped [${col}]`);
            const c = await req(`/api/schema/tables/${current}/columns`);
            setColumns(c || []);
        } catch (e) { toast(e.message, true); }
    };

    const updateColumn = async () => {
        if (!editColDraft.name) return toast('Column name is required', true);
        try {
            await req(`/api/schema/tables/${current}/columns/${editCol.name}`, {
                method: 'PATCH',
                body: { new_name: editColDraft.name, new_type: editColDraft.type }
            });
            toast(`Column [${editCol.name}] updated`);
            setEditCol(null);
            const c = await req(`/api/schema/tables/${current}/columns`);
            setColumns(c || []);
        } catch (e) { toast(e.message, true); }
    };

    const addFk = async () => {
        if (!fk.col || !fk.refTable) return toast('Fill all FK fields', true);
        try {
            await req(`/api/schema/tables/${current}/relations`, {
                method: 'POST', body: { column: fk.col, references_table: fk.refTable, on_delete: fk.onDelete }
            });
            toast(`FK created`);
            setFk({ col: '', refTable: '', onDelete: 'CASCADE' });
            const r = await req(`/api/schema/tables/${current}/relations`);
            setRelations(r || []);
        } catch (e) { toast(e.message, true); }
    };

    const dropFk = async (constraint) => {
        if (!confirm(`Drop constraint '${constraint}'?`)) return;
        try {
            await req(`/api/schema/tables/${current}/relations/${constraint}`, { method: 'DELETE' });
            toast(`Removed FK`);
            const r = await req(`/api/schema/tables/${current}/relations`);
            setRelations(r || []);
        } catch (e) { toast(e.message, true); }
    };

    const openRuleModal = async (colName) => {
        const colMeta = columns.find(c => c.name === colName);
        try {
            const rules = await req(`/api/schema/tables/${current}/columns/${colName}/rules`) || {};
            setRuleCol(colName);
            setRuleColType(colMeta?.type || 'text');
            setRuleDraft({
                notEmptyOn: !!rules.notEmpty,
                minOn: rules.min !== undefined,
                min: rules.min !== undefined ? String(rules.min) : '',
                maxOn: rules.max !== undefined,
                max: rules.max !== undefined ? String(rules.max) : '',
                patternOn: !!rules.pattern,
                pattern: rules.pattern || '',
                emailOn: !!rules.email,
            });
        } catch (e) { toast(e.message, true); }
    };

    const saveRules = async () => {
        const payload = {};
        if (ruleDraft.notEmptyOn) payload.notEmpty = true;
        if (ruleDraft.minOn && ruleDraft.min !== '') payload.min = Number(ruleDraft.min);
        if (ruleDraft.maxOn && ruleDraft.max !== '') payload.max = Number(ruleDraft.max);
        if (ruleDraft.patternOn && ruleDraft.pattern) payload.pattern = ruleDraft.pattern;
        if (ruleDraft.emailOn) payload.email = true;
        try {
            await req(`/api/schema/tables/${current}/columns/${ruleCol}/rules`, { method: 'PUT', body: payload });
            toast(`Rules saved for [${ruleCol}]`);
            setRuleCol(null);
        } catch (e) { toast(e.message, true); }
    };

    const commitInsert = async () => {
        let payload;
        if (insertMode === 'json') {
            try { payload = JSON.parse(insertJson); } catch { return toast('Invalid JSON syntax', true); }
        } else {
            payload = insertData;
        }

        try {
            await req(`/api/collections/${current}`, { method: 'POST', body: payload });
            toast('Record inserted');
            loadData();
        } catch (e) { toast(e.message, true); }
    };

    const deleteRecord = async (id) => {
        if (!confirm('Delete this record?')) return;
        try {
            await req(`/api/collections/${current}/${id}`, { method: 'DELETE' });
            toast('Record deleted');
            loadData();
        } catch (e) { toast(e.message, true); }
    };

    const saveEdit = async () => {
        let payload;
        if (insertMode === 'json') {
            try { payload = JSON.parse(editJson); } catch { return toast('Invalid JSON', true); }
        } else {
            payload = editData;
        }

        try {
            await req(`/api/collections/${current}/${editRow.id}`, { method: 'PUT', body: payload });
            toast('Record updated');
            setEditRow(null);
            loadData();
        } catch (e) { toast(e.message, true); }
    };

    const addFilter = () => {
        if (!filterDraft.col || !filterDraft.val) return toast('Select column and value', true);
        setFilters([...filters, { ...filterDraft }]);
        setFilterDraft({ ...filterDraft, val: '' });
    };

    const apiUrl = `GET /api/collections/${current}${filters.length ? '?' + filters.map(f => `${f.col}=${f.op}.${f.val}`).join('&') : ''}`;

    // ─── Tabs Config ───────────────────
    const TABS = [
        { id: 'schema', label: 'Schema', icon: Columns },
        { id: 'data', label: 'Data', icon: Rows3 },
        { id: 'insert', label: 'Insert', icon: FilePlus },
        { id: 'api', label: 'API', icon: Terminal },
    ];

    const userCols = columns.filter(c => !['id', 'created_at', 'modified_at', 'created_by', 'modified_by', 'tenant_id'].includes(c.name));

    // ═══════════════════════════════════
    // RENDER: Collections List
    // ═══════════════════════════════════
    if (!current) {
        return (
            <div className="flex-1 flex flex-col overflow-hidden">
                <header className="px-6 py-4 border-b border-border flex items-center gap-4 bg-surface-1 flex-shrink-0">
                    <h1 className="text-base font-semibold text-txt-0 flex items-center gap-2 flex-shrink-0">
                        <Table2 className="w-4 h-4 text-brand" /> Collections
                    </h1>
                    <div className="flex items-center gap-2 flex-1 max-w-lg">
                        <div className="relative flex-1">
                            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-txt-3 pointer-events-none" />
                            <input type="text" placeholder="Enter collection name..."
                                onKeyDown={e => e.key === 'Enter' && openCollection(e.target.value.trim())}
                                className="w-full pl-9 pr-3 py-1.5 bg-surface-2 border border-border rounded-md text-sm text-txt-0 placeholder-txt-3 focus:border-brand outline-none font-mono transition" />
                        </div>
                    </div>
                    <button onClick={() => setShowNew(true)}
                        className="ml-auto px-3 py-1.5 bg-surface-2 border border-border text-txt-1 text-sm rounded-md hover:bg-surface-3 hover:text-txt-0 transition-colors flex items-center gap-1.5 flex-shrink-0">
                        <Plus className="w-3.5 h-3.5" /> New
                    </button>
                </header>

                <div className="flex-1 overflow-auto p-6">
                    {tables.length === 0 ? (
                        <div className="text-center py-16">
                            <div className="w-16 h-16 mx-auto mb-4 bg-surface-2 rounded-2xl flex items-center justify-center">
                                <Table2 className="w-7 h-7 text-txt-3" />
                            </div>
                            <p className="text-txt-2 text-sm">No collections found. Create one to get started.</p>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                            {tables.map(t => (
                                <div key={t} className="bg-surface-1 border border-border rounded-lg p-4 hover:border-brand/50 hover:bg-surface-2 transition-all group relative">
                                    <button onClick={() => openCollection(t)} className="text-left w-full">
                                        <div className="flex items-center gap-2.5 mb-1">
                                            <div className="w-7 h-7 bg-brand/10 rounded-md flex items-center justify-center group-hover:bg-brand/20 transition-colors">
                                                <Table2 className="w-3.5 h-3.5 text-brand" />
                                            </div>
                                            <span className="font-mono text-sm text-txt-0 font-medium">{t}</span>
                                        </div>
                                        <p className="text-[10px] text-txt-3 ml-9">Click to open</p>
                                    </button>
                                    <button onClick={() => dropTable(t)} className="absolute top-3 right-3 p-1.5 text-txt-3 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all" title="Drop">
                                        <Trash2 className="w-3.5 h-3.5" />
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                </div>

                {/* New Collection Modal */}
                {showNew && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
                        <div className="bg-surface-1 border border-border rounded-xl p-6 w-full max-w-sm shadow-2xl">
                            <h3 className="text-base font-semibold text-txt-0 mb-4">Create Collection</h3>
                            <input type="text" value={newName} onChange={e => setNewName(e.target.value)} placeholder="collection_name"
                                onKeyDown={e => e.key === 'Enter' && createTable()}
                                className="w-full px-4 py-2.5 bg-surface-2 border border-border rounded-md text-sm text-txt-0 font-mono placeholder-txt-3 focus:border-brand outline-none mb-4" autoFocus />
                            <div className="flex gap-2 justify-end">
                                <button onClick={() => setShowNew(false)} className="px-4 py-2 bg-surface-2 text-txt-1 text-sm rounded-md hover:bg-surface-3 transition-colors">Cancel</button>
                                <button onClick={createTable} className="px-4 py-2 bg-brand text-surface-0 text-sm font-medium rounded-md hover:bg-brand-dark transition-colors">Create</button>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        );
    }

    // ═══════════════════════════════════
    // RENDER: Active Collection Workspace
    // ═══════════════════════════════════
    return (
        <div className="flex-1 flex flex-col overflow-hidden">
            {/* Breadcrumb */}
            <div className="flex items-center gap-2 px-6 py-2 border-b border-border bg-surface-0/50 flex-shrink-0">
                <button onClick={backToList} className="text-txt-2 hover:text-txt-0 transition-colors flex items-center gap-1 text-xs">
                    <ArrowLeft className="w-3.5 h-3.5" /> Collections
                </button>
                <span className="text-txt-3 text-xs">/</span>
                <span className="text-brand text-xs font-mono font-medium">{current}</span>
            </div>

            {/* Tabs */}
            <div className="flex items-center border-b border-border bg-surface-1 px-6 flex-shrink-0">
                {TABS.map(t => (
                    <button key={t.id} onClick={() => setTab(t.id)}
                        className={`px-4 py-3 text-sm border-b-2 transition-colors flex items-center gap-1.5 ${tab === t.id ? 'text-txt-0 border-brand' : 'text-txt-2 hover:text-txt-0 border-transparent'}`}>
                        <t.icon className="w-3.5 h-3.5" /> {t.label}
                    </button>
                ))}
            </div>

            {/* Tab content */}
            <div className="flex-1 overflow-auto">

                {/* ──── SCHEMA TAB ──── */}
                {tab === 'schema' && (() => {
                    const userCols = columns.filter(c => !['id', 'created_at', 'modified_at', 'created_by', 'modified_by', 'tenant_id'].includes(c.name));
                    return (
                        <div className="p-6 grid grid-cols-1 lg:grid-cols-2 gap-6">
                            {/* Columns */}
                            <div className="bg-surface-1 border border-border rounded-lg overflow-hidden">
                                <div className="px-5 py-3 border-b border-border flex items-center justify-between">
                                    <h3 className="text-sm font-medium text-txt-0">Columns</h3>
                                    <span className="text-xs text-txt-3 font-mono">{columns.length} cols</span>
                                </div>
                                <div className="divide-y divide-border max-h-[360px] overflow-y-auto">
                                    {columns.map(c => {
                                        const sys = SYSTEM_COLUMNS.includes(c.name);
                                        return (
                                            <div key={c.name} className="flex items-center justify-between px-5 py-2.5 group hover:bg-surface-0/30 transition-colors">
                                                <div className="flex items-center gap-3">
                                                    <span className="font-mono text-sm text-txt-0">{c.name}</span>
                                                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-surface-3 text-txt-2 font-mono">{c.type}</span>
                                                    {c.required && <span className="text-[9px] text-amber-400 font-bold">NOT NULL</span>}
                                                </div>
                                                {sys
                                                    ? <span className="text-[9px] text-txt-3 uppercase tracking-widest">system</span>
                                                    : <div className="flex items-center gap-1">
                                                        <button onClick={() => {
                                                            setEditCol(c);
                                                            setEditColDraft({ name: c.name, type: c.type });
                                                        }} className="text-txt-3 hover:text-blue-400 transition-colors opacity-0 group-hover:opacity-100" title="Edit Column"><Pencil className="w-3.5 h-3.5" /></button>
                                                        <button onClick={() => openRuleModal(c.name)} className="text-txt-3 hover:text-brand transition-colors opacity-0 group-hover:opacity-100" title="Validation Rules"><Shield className="w-3.5 h-3.5" /></button>
                                                        <button onClick={() => dropColumn(c.name)} className="text-txt-3 hover:text-red-400 transition-colors opacity-0 group-hover:opacity-100"><Trash2 className="w-3.5 h-3.5" /></button>
                                                    </div>
                                                }
                                            </div>
                                        );
                                    })}
                                </div>
                                <div className="p-4 border-t border-border bg-surface-0/50">
                                    <div className="flex gap-2">
                                        <input type="text" value={addCol.name} onChange={e => setAddCol({ ...addCol, name: e.target.value })} placeholder="column_name"
                                            className="flex-[2] min-w-0 px-3 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono focus:border-brand outline-none" />
                                        <select value={addCol.type} onChange={e => setAddCol({ ...addCol, type: e.target.value })}
                                            className="flex-1 px-2 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                            <option value="text">TEXT</option><option value="integer">INT</option><option value="boolean">BOOL</option>
                                            <option value="uuid">UUID</option><option value="jsonb">JSON</option><option value="timestamp">TIMESTAMP</option>
                                        </select>
                                    </div>
                                    <div className="flex items-center justify-between mt-2">
                                        <label className="flex items-center gap-2 text-xs text-txt-2 cursor-pointer">
                                            <input type="checkbox" checked={addCol.required} onChange={e => setAddCol({ ...addCol, required: e.target.checked })}
                                                className="w-3.5 h-3.5 rounded" /> NOT NULL
                                        </label>
                                        <button onClick={addColumn} className="px-3 py-1 bg-brand text-surface-0 text-xs font-medium rounded hover:bg-brand-dark transition-colors flex items-center gap-1">
                                            <Plus className="w-3 h-3" /> Add
                                        </button>
                                    </div>
                                </div>
                            </div>

                            {/* Relations */}
                            <div className="bg-surface-1 border border-border rounded-lg overflow-hidden">
                                <div className="px-5 py-3 border-b border-border flex items-center justify-between">
                                    <h3 className="text-sm font-medium text-txt-0">Foreign Keys</h3>
                                    <span className="text-xs text-txt-3 font-mono">{relations.length} FKs</span>
                                </div>
                                <div className="divide-y divide-border max-h-[200px] overflow-y-auto">
                                    {relations.length === 0 && <div className="p-4 text-center text-txt-3 text-xs italic">No foreign keys defined</div>}
                                    {relations.map(r => (
                                        <div key={r.constraint_name} className="flex items-center justify-between px-5 py-2.5 group hover:bg-surface-0/30 transition-colors">
                                            <div className="flex items-center gap-2 text-xs">
                                                <Link className="w-3 h-3 text-blue-400" />
                                                <span className="font-mono text-blue-300 font-medium">{r.column}</span>
                                                <span className="text-txt-3">→</span>
                                                <span className="font-mono text-txt-1">{r.references_table}.{r.references_column}</span>
                                            </div>
                                            <button onClick={() => dropFk(r.constraint_name)} className="text-txt-3 hover:text-red-400 transition-colors opacity-0 group-hover:opacity-100">
                                                <Unlink className="w-3.5 h-3.5" />
                                            </button>
                                        </div>
                                    ))}
                                </div>
                                <div className="p-4 border-t border-border bg-surface-0/50 space-y-3">
                                    {/* Source Column */}
                                    <div className="space-y-1">
                                        <label className="text-[10px] uppercase font-semibold text-txt-3 px-1">Source Column</label>
                                        <select value={fk.col} onChange={e => setFk({ ...fk, col: e.target.value })}
                                            className="w-full px-3 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                            <option value="">Select source...</option>
                                            {userCols.map(c => (
                                                <option key={c.name} value={c.name}>
                                                    {c.name} ({c.type})
                                                </option>
                                            ))}
                                        </select>
                                    </div>

                                    {/* Target Table */}
                                    <div className="space-y-1">
                                        <label className="text-[10px] uppercase font-semibold text-txt-3 px-1">References Table</label>
                                        <select value={fk.refTable} onChange={e => setFk({ ...fk, refTable: e.target.value })}
                                            className="w-full px-3 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                            <option value="">Select table...</option>
                                            {tables.filter(t => t !== current).map(t => <option key={t} value={t}>{t}</option>)}
                                        </select>
                                    </div>

                                    {/* Link Action & Compatibility */}
                                    <div className="pt-1">
                                        {fk.col && fk.refTable && (
                                            <div className="mb-3 px-3 py-2 bg-surface-2 border border-border rounded flex items-center justify-between">
                                                <div className="flex items-center gap-2">
                                                    {userCols.find(c => c.name === fk.col)?.type === 'uuid' ? (
                                                        <>
                                                            <ShieldCheck className="w-3.5 h-3.5 text-green-400" />
                                                            <span className="text-[11px] text-green-400 font-medium">Type Compatible (UUID)</span>
                                                        </>
                                                    ) : (
                                                        <>
                                                            <Shield className="w-3.5 h-3.5 text-amber-400" />
                                                            <span className="text-[11px] text-amber-400 font-medium">Type Mismatch ({userCols.find(c => c.name === fk.col)?.type} vs UUID)</span>
                                                        </>
                                                    )}
                                                </div>
                                                {userCols.find(c => c.name === fk.col)?.type !== 'uuid' && (
                                                    <button
                                                        onClick={async () => {
                                                            const c = userCols.find(x => x.name === fk.col);
                                                            try {
                                                                await req(`/api/schema/tables/${current}/columns/${fk.col}`, {
                                                                    method: 'PATCH',
                                                                    body: { new_type: 'uuid' }
                                                                });
                                                                toast(`Converted [${fk.col}] to UUID`);
                                                                // Reload columns
                                                                const upd = await req(`/api/schema/tables/${current}/columns`);
                                                                setColumns(upd || []);
                                                            } catch (err) { toast(err.message, true); }
                                                        }}
                                                        className="text-[10px] px-2 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/20 rounded hover:bg-amber-500/20 transition-colors"
                                                    >
                                                        Fix Type
                                                    </button>
                                                )}
                                            </div>
                                        )}

                                        <div className="flex gap-2">
                                            <select value={fk.onDelete} onChange={e => setFk({ ...fk, onDelete: e.target.value })}
                                                className="flex-1 px-3 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                                <option value="CASCADE">CASCADE onDelete</option>
                                                <option value="SET NULL">SET NULL onDelete</option>
                                                <option value="RESTRICT">RESTRICT onDelete</option>
                                            </select>
                                            <button onClick={addFk} className="px-4 py-1.5 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700 transition-colors flex items-center justify-center gap-1.5">
                                                <Link className="w-3 h-3" /> Create Relationship
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    );
                })()}

                {/* ──── DATA TAB ──── */}
                {tab === 'data' && (
                    <div className="flex flex-col h-full">
                        {/* Toolbar */}
                        <div className="px-6 py-3 border-b border-border bg-surface-0/50 flex items-center gap-3 flex-wrap flex-shrink-0">
                            <div className="flex bg-surface-2 rounded-md border border-border overflow-hidden">
                                <button onClick={() => setDataView('table')}
                                    className={`px-3 py-1.5 text-xs flex items-center gap-1.5 transition-colors ${dataView === 'table' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                    <Table className="w-3 h-3" /> Table
                                </button>
                                <button onClick={() => setDataView('json')}
                                    className={`px-3 py-1.5 text-xs flex items-center gap-1.5 transition-colors ${dataView === 'json' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                    <Braces className="w-3 h-3" /> JSON
                                </button>
                            </div>
                            <div className="flex items-center gap-1.5 flex-1 min-w-0">
                                <select value={filterDraft.col} onChange={e => setFilterDraft({ ...filterDraft, col: e.target.value })}
                                    className="px-2 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                    <option value="">column</option>
                                    {columns.map(c => <option key={c.name} value={c.name}>{c.name}</option>)}
                                </select>
                                <select value={filterDraft.op} onChange={e => setFilterDraft({ ...filterDraft, op: e.target.value })}
                                    className="px-2 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                    <option value="eq">=</option><option value="neq">≠</option><option value="gt">&gt;</option>
                                    <option value="gte">≥</option><option value="lt">&lt;</option><option value="lte">≤</option>
                                    <option value="like">LIKE</option><option value="ilike">ILIKE</option><option value="in">IN</option><option value="is">IS</option>
                                </select>
                                <input type="text" value={filterDraft.val} onChange={e => setFilterDraft({ ...filterDraft, val: e.target.value })} placeholder="value..."
                                    onKeyDown={e => e.key === 'Enter' && addFilter()}
                                    className="flex-1 min-w-[60px] max-w-[200px] px-2 py-1.5 bg-surface-2 border border-border rounded text-xs text-txt-0 font-mono focus:border-brand outline-none" />
                                <button onClick={addFilter} className="px-2 py-1.5 bg-brand text-surface-0 text-xs rounded hover:bg-brand-dark transition-colors">
                                    <Plus className="w-3 h-3" />
                                </button>
                                <button onClick={loadData} className="px-2 py-1.5 bg-surface-2 border border-border text-txt-2 text-xs rounded hover:bg-surface-3 transition-colors">
                                    <RefreshCw className="w-3 h-3" />
                                </button>
                            </div>
                        </div>

                        {/* Filter chips */}
                        {filters.length > 0 && (
                            <div className="px-6 py-2 border-b border-border bg-surface-0/30 flex items-center gap-2 flex-wrap flex-shrink-0">
                                <span className="text-[10px] text-txt-3 uppercase tracking-wider">Filters:</span>
                                {filters.map((f, i) => (
                                    <span key={i} className="inline-flex items-center gap-1 px-2 py-0.5 bg-brand/10 text-brand text-[10px] rounded-full font-mono border border-brand/20">
                                        {f.col} {f.op} {f.val}
                                        <button onClick={() => setFilters(filters.filter((_, j) => j !== i))} className="hover:text-red-400 ml-0.5"><X className="w-2.5 h-2.5" /></button>
                                    </span>
                                ))}
                            </div>
                        )}

                        {/* API preview */}
                        <div className="px-6 py-1.5 border-b border-border bg-surface-0 flex-shrink-0">
                            <code className="text-[10px] text-brand font-mono">{apiUrl}</code>
                        </div>

                        {/* Data */}
                        <div className="flex-1 overflow-auto">
                            {rows.length === 0 ? (
                                <div className="h-full flex flex-col items-center justify-center text-txt-3">
                                    <Table className="w-10 h-10 mb-3 opacity-30" />
                                    <p className="text-sm">No records found</p>
                                </div>
                            ) : dataView === 'table' ? (
                                <table className="data-table">
                                    <thead><tr>
                                        {columns.map(c => <th key={c.name}>{c.name}</th>)}
                                        <th className="w-20 text-right">Actions</th>
                                    </tr></thead>
                                    <tbody>
                                        {rows.map(row => (
                                            <tr key={row.id}>
                                                {columns.map((col, i) => {
                                                    const v = row[col.name];
                                                    if (v === undefined || v === null) return <td key={i}><span className="text-txt-3 italic">null</span></td>;
                                                    if (v && typeof v === 'object') {
                                                        return (
                                                            <td key={i}>
                                                                <div className="max-h-32 overflow-y-auto bg-surface-3/30 p-2 rounded border border-border/50 custom-scrollbar">
                                                                    <pre className="text-[10px] leading-relaxed font-mono whitespace-pre-wrap">
                                                                        {highlightJson(v)}
                                                                    </pre>
                                                                </div>
                                                            </td>
                                                        );
                                                    }
                                                    return <td key={i}>{String(v)}</td>;
                                                })}
                                                <td className="text-right">
                                                    <div className="flex gap-1 justify-end">
                                                        <button onClick={() => {
                                                            setEditRow(row);
                                                            const e = { ...row };
                                                            const sys = ['id', 'created_at', 'modified_at', 'created_by', 'modified_by', 'tenant_id', 'id_key'];
                                                            sys.forEach(k => delete e[k]);
                                                            setEditData(e);
                                                            setEditJson(JSON.stringify(e, null, 2));
                                                        }}
                                                            title="Edit Record"
                                                            className="p-1 text-txt-3 hover:text-blue-400 transition-colors"><Pencil className="w-3.5 h-3.5" /></button>
                                                        <button onClick={() => deleteRecord(row.id)}
                                                            className="p-1 text-txt-3 hover:text-red-400 transition-colors"><Trash2 className="w-3.5 h-3.5" /></button>
                                                    </div>
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            ) : (
                                <div className="p-4 space-y-3">
                                    {rows.map(row => (
                                        <div key={row.id} className="bg-surface-1 border border-border rounded-lg p-4 hover:border-border-light transition-colors group">
                                            <div className="flex items-center justify-between mb-2 border-b border-border/50 pb-2">
                                                <span className="text-[10px] font-mono text-txt-3 bg-surface-3 px-2 py-0.5 rounded shadow-sm">{row.id || '—'}</span>
                                                <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                                    <button onClick={() => {
                                                        setEditRow(row);
                                                        const e = { ...row };
                                                        SYSTEM_COLUMNS.forEach(k => delete e[k]);
                                                        setEditData(e);
                                                        setEditJson(JSON.stringify(e, null, 2));
                                                    }}
                                                        title="Edit Record"
                                                        className="p-1 px-2 text-xs bg-surface-2 text-txt-2 hover:bg-blue-500/10 hover:text-blue-400 rounded transition-colors flex items-center gap-1.5"><Pencil className="w-3 h-3" /> Edit</button>
                                                    <button onClick={() => deleteRecord(row.id)}
                                                        className="p-1 px-2 text-xs bg-surface-2 text-txt-2 hover:bg-red-500/10 hover:text-red-400 rounded transition-colors flex items-center gap-1.5"><Trash2 className="w-3 h-3" /> Delete</button>
                                                </div>
                                            </div>
                                            <div className="bg-surface-0/50 p-3 rounded-md border border-border/30 overflow-x-auto custom-scrollbar">
                                                <pre className="text-xs font-mono text-txt-1 whitespace-pre-wrap break-all leading-relaxed">
                                                    {highlightJson(row)}
                                                </pre>
                                            </div>
                                        </div>
                                    )
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                )}

                {/* ──── INSERT TAB ──── */}
                {tab === 'insert' && (
                    <div className="p-6 max-w-2xl">
                        <div className="bg-surface-1 border border-border rounded-lg overflow-hidden">
                            <div className="px-5 py-3 border-b border-border flex items-center justify-between">
                                <h3 className="text-sm font-medium text-txt-0 flex items-center gap-2">
                                    <FilePlus className="w-4 h-4 text-brand" /> Insert Document
                                </h3>
                                <div className="flex bg-surface-2 rounded-md border border-border overflow-hidden">
                                    <button onClick={() => setInsertMode('form')}
                                        className={`px-3 py-1 text-[10px] flex items-center gap-1.5 transition-colors ${insertMode === 'form' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                        <Layout className="w-3 h-3" /> Form
                                    </button>
                                    <button onClick={() => setInsertMode('json')}
                                        className={`px-3 py-1 text-[10px] flex items-center gap-1.5 transition-colors ${insertMode === 'json' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                        <Braces className="w-3 h-3" /> JSON
                                    </button>
                                </div>
                            </div>
                            <div className="p-5">
                                {insertMode === 'form' ? (
                                    <DynamicForm
                                        columns={columns.filter(c => !SYSTEM_COLUMNS.includes(c.name))}
                                        relations={relations}
                                        value={insertData}
                                        onChange={setInsertData}
                                    />
                                ) : (
                                    <textarea value={insertJson} onChange={e => setInsertJson(e.target.value)} rows={14}
                                        className="w-full px-4 py-3 bg-surface-0 border border-border rounded-md text-sm text-txt-0 font-mono focus:border-brand outline-none resize-y leading-relaxed" />
                                )}
                                <button onClick={commitInsert}
                                    className="mt-6 w-full py-2.5 bg-brand text-surface-0 text-sm font-medium rounded-md hover:bg-brand-dark transition-colors flex items-center justify-center gap-2 shadow-lg shadow-brand/20">
                                    <Send className="w-4 h-4" /> Commit Record
                                </button>
                            </div>
                        </div>
                    </div>
                )}

                {/* ──── API TAB ──── */}
                {tab === 'api' && (
                    <div className="p-6 h-full flex flex-col lg:flex-row gap-6">
                        {/* Left: Endpoint Reference (Documentation) */}
                        <div className="lg:w-1/3 flex flex-col gap-4">
                            <div className="bg-surface-1 border border-border rounded-lg overflow-hidden flex flex-col min-h-0">
                                <div className="px-5 py-3 border-b border-border bg-surface-0/30">
                                    <h3 className="text-sm font-medium text-txt-0 flex items-center gap-2">
                                        <Braces className="w-4 h-4 text-brand" /> API Reference
                                    </h3>
                                    <p className="text-[10px] text-txt-3 mt-1">Click an endpoint to load it into the console</p>
                                </div>
                                <div className="flex-1 overflow-auto p-3 space-y-2">
                                    {[
                                        { m: 'GET', p: `/api/collections/${current}`, d: 'List all records', b: '{}' },
                                        { m: 'GET', p: `/api/collections/${current}?select=*,relation(*)`, d: 'Expansion (Nesting)', b: '{}' },
                                        { m: 'GET', p: `/api/collections/${current}?id=eq.uuid`, d: 'Filter by value', b: '{}' },
                                        { m: 'GET', p: `/api/collections/${current}?order=created_at.desc&limit=1`, d: 'Sort & Limit', b: '{}' },
                                        { m: 'POST', p: `/api/collections/${current}`, d: 'Insert record', b: '{\n  "name": "example"\n}' },
                                        { m: 'PUT', p: `/api/collections/${current}/{id}`, d: 'Update record', b: '{\n  "name": "updated"\n}' },
                                        { m: 'DELETE', p: `/api/collections/${current}/{id}`, d: 'Delete record', b: '{}' },
                                        { m: 'GET', p: `/api/schema/tables/${current}/columns`, d: 'Get schema', b: '{}' },
                                    ].map((ep, i) => {
                                        const colors = { GET: 'text-brand', POST: 'text-blue-400', PUT: 'text-amber-400', DELETE: 'text-red-400' };
                                        return (
                                            <button key={i} onClick={() => {
                                                setApiMethod(ep.m);
                                                setApiPath(ep.p);
                                                setApiBody(ep.b);
                                                setApiResponse(null);
                                                setApiMeta(null);
                                            }}
                                                className="w-full flex items-start gap-3 p-3 bg-surface-0 rounded-md border border-border hover:border-brand/40 hover:bg-brand/5 transition-all text-left group">
                                                <span className={`font-mono font-bold text-[10px] ${colors[ep.m]} w-10 flex-shrink-0 mt-0.5`}>{ep.m}</span>
                                                <div className="min-w-0">
                                                    <code className="text-[11px] text-txt-0 break-all group-hover:text-brand transition-colors">{ep.p}</code>
                                                    <p className="text-[9px] text-txt-3 mt-0.5">{ep.d}</p>
                                                </div>
                                            </button>
                                        );
                                    })}
                                </div>
                            </div>
                        </div>

                        {/* Right: Interactive Console + Response */}
                        <div className="lg:w-2/3 flex flex-col gap-6">
                            {/* Request Builder */}
                            <div className="bg-surface-1 border border-border rounded-lg overflow-hidden flex-shrink-0">
                                <div className="px-5 py-3 border-b border-border flex items-center justify-between bg-surface-0/30">
                                    <h3 className="text-sm font-medium text-txt-0 flex items-center gap-2">
                                        <Terminal className="w-4 h-4 text-brand" /> Console Explorer
                                    </h3>
                                    {apiMeta && (
                                        <div className="flex items-center gap-3">
                                            <span className={`text-[10px] px-2 py-0.5 rounded font-bold ${apiMeta.status < 400 ? 'bg-green-500/10 text-green-400' : 'bg-red-500/10 text-red-400'}`}>
                                                {apiMeta.status} {apiMeta.status < 300 ? 'SUCCESS' : 'FAILURE'}
                                            </span>
                                            <span className="text-[10px] text-txt-3 font-mono">{apiMeta.time}ms</span>
                                        </div>
                                    )}
                                </div>

                                <div className="p-5 space-y-4">
                                    <div className="flex gap-2">
                                        <select value={apiMethod} onChange={e => setApiMethod(e.target.value)}
                                            className={`px-3 py-2 rounded-md border border-border bg-surface-2 text-xs font-bold font-mono outline-none focus:border-brand cursor-pointer transition-colors ${apiMethod === 'GET' ? 'text-brand' :
                                                apiMethod === 'POST' ? 'text-blue-400' :
                                                    apiMethod === 'PUT' ? 'text-amber-400' : 'text-red-400'
                                                }`}>
                                            <option value="GET">GET</option>
                                            <option value="POST">POST</option>
                                            <option value="PUT">PUT</option>
                                            <option value="DELETE">DELETE</option>
                                        </select>
                                        <input type="text" value={apiPath} onChange={e => setApiPath(e.target.value)}
                                            className="flex-1 px-4 py-2 bg-surface-2 border border-border rounded-md text-xs text-txt-0 font-mono focus:border-brand outline-none" />
                                        <button onClick={testEndpoint} disabled={apiLoading}
                                            className={`px-6 py-2 bg-brand text-surface-0 text-sm font-bold rounded-md hover:bg-brand-dark transition-all flex items-center gap-2 shadow-lg shadow-brand/20 disabled:opacity-50`}>
                                            {apiLoading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                                            {apiLoading ? 'Run' : 'Send'}
                                        </button>
                                    </div>

                                    {['POST', 'PUT'].includes(apiMethod) && (
                                        <div className="space-y-1.5">
                                            <label className="text-[10px] uppercase font-bold text-txt-3 ml-1">Payload Editor</label>
                                            <textarea value={apiBody} onChange={e => setApiBody(e.target.value)} rows={5}
                                                className="w-full px-4 py-3 bg-surface-0 border border-border rounded-md text-xs text-txt-1 font-mono focus:border-brand outline-none resize-y" />
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Response Viewer */}
                            <div className="flex-1 min-h-[300px] flex flex-col bg-surface-1 border border-border rounded-lg overflow-hidden">
                                <div className="px-5 py-2.5 border-b border-border bg-surface-0/30 flex items-center justify-between">
                                    <span className="text-[10px] uppercase font-bold text-txt-3 tracking-wider">Output</span>
                                    {apiResponse && (
                                        <button onClick={() => setApiResponse(null)} className="text-txt-3 hover:text-txt-1 transition-colors">
                                            <X className="w-3.5 h-3.5" />
                                        </button>
                                    )}
                                </div>
                                <div className="flex-1 overflow-auto p-5 font-mono text-xs leading-relaxed bg-surface-2/30">
                                    {apiResponse ? (
                                        <pre className="whitespace-pre-wrap">{highlightJson(apiResponse)}</pre>
                                    ) : (
                                        <div className="h-full flex flex-col items-center justify-center text-txt-3 italic">
                                            <Terminal className="w-8 h-8 mb-3 opacity-20" />
                                            <span>Select a template or type a path and click Send...</span>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </div>

            {/* Edit Modal */}
            {
                editRow && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 overflow-y-auto pt-10 pb-10">
                        <div className="bg-surface-1 border border-border rounded-xl p-6 w-full max-w-lg shadow-2xl my-auto">
                            <div className="flex items-center justify-between mb-6">
                                <h3 className="text-base font-semibold text-txt-0 flex items-center gap-2">
                                    <Pencil className="w-4 h-4 text-blue-400" /> Edit Record
                                </h3>
                                <div className="flex bg-surface-2 rounded-md border border-border overflow-hidden">
                                    <button onClick={() => setInsertMode('form')}
                                        className={`px-3 py-1 text-[10px] flex items-center gap-1.5 transition-colors ${insertMode === 'form' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                        <Layout className="w-3 h-3" /> Form
                                    </button>
                                    <button onClick={() => setInsertMode('json')}
                                        className={`px-3 py-1 text-[10px] flex items-center gap-1.5 transition-colors ${insertMode === 'json' ? 'bg-brand text-surface-0' : 'text-txt-2 hover:text-txt-0'}`}>
                                        <Braces className="w-3 h-3" /> JSON
                                    </button>
                                </div>
                            </div>

                            <div className="max-h-[60vh] overflow-y-auto mb-6 pr-2 custom-scrollbar">
                                {insertMode === 'form' ? (
                                    <DynamicForm
                                        columns={columns.filter(c => !SYSTEM_COLUMNS.includes(c.name))}
                                        relations={relations}
                                        value={editData}
                                        onChange={setEditData}
                                    />
                                ) : (
                                    <textarea value={editJson} onChange={e => setEditJson(e.target.value)} rows={14}
                                        className="w-full px-4 py-3 bg-surface-0 border border-border rounded-md text-sm text-txt-0 font-mono focus:border-brand outline-none resize-y leading-relaxed" />
                                )}
                            </div>

                            <div className="flex gap-2 justify-end border-t border-border pt-4">
                                <button onClick={() => setEditRow(null)} className="px-4 py-2 bg-surface-2 text-txt-1 text-sm rounded-md hover:bg-surface-3 transition-colors">Cancel</button>
                                <button onClick={saveEdit} className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors shadow-lg shadow-blue-500/20">Save Changes</button>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* Edit Column Modal */}
            {
                editCol && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
                        <div className="bg-surface-1 border border-border rounded-xl p-6 w-full max-w-sm shadow-2xl">
                            <h3 className="text-base font-semibold text-txt-0 mb-4 flex items-center gap-2">
                                <Pencil className="w-4 h-4 text-blue-400" /> Edit Column: <span className="font-mono text-brand">{editCol.name}</span>
                            </h3>
                            <div className="space-y-4 mb-6">
                                <div>
                                    <label className="block text-[10px] text-txt-3 uppercase tracking-wider mb-1.5 ml-1">Column Name</label>
                                    <input type="text" value={editColDraft.name} onChange={e => setEditColDraft({ ...editColDraft, name: e.target.value })}
                                        className="w-full px-4 py-2.5 bg-surface-2 border border-border rounded-md text-sm text-txt-0 font-mono focus:border-brand outline-none" autoFocus />
                                </div>
                                <div>
                                    <label className="block text-[10px] text-txt-3 uppercase tracking-wider mb-1.5 ml-1">Data Type</label>
                                    <select value={editColDraft.type} onChange={e => setEditColDraft({ ...editColDraft, type: e.target.value })}
                                        className="w-full px-4 py-2.5 bg-surface-2 border border-border rounded-md text-sm text-txt-0 font-mono cursor-pointer focus:border-brand outline-none">
                                        <option value="text">TEXT</option><option value="integer">INT</option><option value="boolean">BOOL</option>
                                        <option value="uuid">UUID</option><option value="jsonb">JSON</option><option value="timestamp">TIMESTAMP</option>
                                    </select>
                                </div>
                            </div>
                            <div className="flex gap-2 justify-end">
                                <button onClick={() => setEditCol(null)} className="px-4 py-2 bg-surface-2 text-txt-1 text-sm rounded-md hover:bg-surface-3 transition-colors">Cancel</button>
                                <button onClick={updateColumn} className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors">Apply Changes</button>
                            </div>
                        </div>
                    </div>
                )
            }

            {/* Validation Rules Modal */}
            {
                ruleCol && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
                        <div className="bg-surface-1 border border-border rounded-xl p-6 w-full max-w-md shadow-2xl">
                            <div className="flex items-center justify-between mb-5">
                                <h3 className="text-base font-semibold text-txt-0 flex items-center gap-2">
                                    <Shield className="w-4 h-4 text-brand" /> Validation: <span className="text-brand font-mono">{ruleCol}</span>
                                </h3>
                                <button onClick={() => setRuleCol(null)} className="text-txt-2 hover:text-txt-0 transition-colors"><X className="w-4 h-4" /></button>
                            </div>

                            <p className="text-[11px] text-txt-3 mb-5">Enable the rules you want. Data that doesn't match will be rejected automatically.</p>

                            <div className="space-y-4">
                                {/* Not Empty */}
                                <div className={`p-3 rounded-lg border transition-colors ${ruleDraft.notEmptyOn ? 'bg-brand/5 border-brand/30' : 'bg-surface-0 border-border'}`}>
                                    <div className="flex items-center justify-between">
                                        <div>
                                            <span className="text-sm font-medium text-txt-0">Required (Not Empty)</span>
                                            <p className="text-[10px] text-txt-3 mt-0.5">Reject empty values, blank strings, and whitespace-only text</p>
                                        </div>
                                        <button onClick={() => setRuleDraft({ ...ruleDraft, notEmptyOn: !ruleDraft.notEmptyOn })}
                                            className={`relative w-9 h-5 rounded-full transition-colors ${ruleDraft.notEmptyOn ? 'bg-brand' : 'bg-surface-3'}`}>
                                            <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${ruleDraft.notEmptyOn ? 'left-[18px]' : 'left-0.5'}`} />
                                        </button>
                                    </div>
                                </div>

                                {/* Min */}
                                <div className={`p-3 rounded-lg border transition-colors ${ruleDraft.minOn ? 'bg-brand/5 border-brand/30' : 'bg-surface-0 border-border'}`}>
                                    <div className="flex items-center justify-between mb-1">
                                        <div>
                                            <span className="text-sm font-medium text-txt-0">{['integer', 'bigint', 'numeric'].includes(ruleColType) ? 'Minimum Value' : 'Minimum Length'}</span>
                                            <p className="text-[10px] text-txt-3 mt-0.5">{['integer', 'bigint', 'numeric'].includes(ruleColType) ? 'Smallest number allowed' : 'Fewest characters allowed'}</p>
                                        </div>
                                        <button onClick={() => setRuleDraft({ ...ruleDraft, minOn: !ruleDraft.minOn })}
                                            className={`relative w-9 h-5 rounded-full transition-colors ${ruleDraft.minOn ? 'bg-brand' : 'bg-surface-3'}`}>
                                            <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${ruleDraft.minOn ? 'left-[18px]' : 'left-0.5'}`} />
                                        </button>
                                    </div>
                                    {ruleDraft.minOn && (
                                        <input type="number" value={ruleDraft.min} onChange={e => setRuleDraft({ ...ruleDraft, min: e.target.value })} placeholder="e.g. 3"
                                            className="mt-2 w-full px-3 py-2 bg-surface-2 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none transition" autoFocus />
                                    )}
                                </div>

                                {/* Max */}
                                <div className={`p-3 rounded-lg border transition-colors ${ruleDraft.maxOn ? 'bg-brand/5 border-brand/30' : 'bg-surface-0 border-border'}`}>
                                    <div className="flex items-center justify-between mb-1">
                                        <div>
                                            <span className="text-sm font-medium text-txt-0">{['integer', 'bigint', 'numeric'].includes(ruleColType) ? 'Maximum Value' : 'Maximum Length'}</span>
                                            <p className="text-[10px] text-txt-3 mt-0.5">{['integer', 'bigint', 'numeric'].includes(ruleColType) ? 'Largest number allowed' : 'Most characters allowed'}</p>
                                        </div>
                                        <button onClick={() => setRuleDraft({ ...ruleDraft, maxOn: !ruleDraft.maxOn })}
                                            className={`relative w-9 h-5 rounded-full transition-colors ${ruleDraft.maxOn ? 'bg-brand' : 'bg-surface-3'}`}>
                                            <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${ruleDraft.maxOn ? 'left-[18px]' : 'left-0.5'}`} />
                                        </button>
                                    </div>
                                    {ruleDraft.maxOn && (
                                        <input type="number" value={ruleDraft.max} onChange={e => setRuleDraft({ ...ruleDraft, max: e.target.value })} placeholder="e.g. 255"
                                            className="mt-2 w-full px-3 py-2 bg-surface-2 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none transition" />
                                    )}
                                </div>

                                {/* Pattern - only for text types */}
                                {['text', 'character varying'].includes(ruleColType) && (
                                    <div className={`p-3 rounded-lg border transition-colors ${ruleDraft.patternOn ? 'bg-brand/5 border-brand/30' : 'bg-surface-0 border-border'}`}>
                                        <div className="flex items-center justify-between mb-1">
                                            <div>
                                                <span className="text-sm font-medium text-txt-0">Format (Pattern)</span>
                                                <p className="text-[10px] text-txt-3 mt-0.5">Only allow text that matches a specific format</p>
                                            </div>
                                            <button onClick={() => setRuleDraft({ ...ruleDraft, patternOn: !ruleDraft.patternOn })}
                                                className={`relative w-9 h-5 rounded-full transition-colors ${ruleDraft.patternOn ? 'bg-brand' : 'bg-surface-3'}`}>
                                                <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${ruleDraft.patternOn ? 'left-[18px]' : 'left-0.5'}`} />
                                            </button>
                                        </div>
                                        {ruleDraft.patternOn && (
                                            <div className="mt-2 space-y-2">
                                                <input type="text" value={ruleDraft.pattern} onChange={e => setRuleDraft({ ...ruleDraft, pattern: e.target.value })} placeholder="e.g. ^[A-Za-z ]+$"
                                                    className="w-full px-3 py-2 bg-surface-2 border border-border rounded-md text-sm text-txt-0 font-mono focus:border-brand outline-none transition" />
                                                <div className="flex gap-1.5 flex-wrap">
                                                    {[
                                                        { label: 'Only letters', val: '^[A-Za-z ]+$' },
                                                        { label: 'Letters & numbers', val: '^[A-Za-z0-9 ]+$' },
                                                        { label: 'URL slug', val: '^[a-z0-9-]+$' },
                                                    ].map(p => (
                                                        <button key={p.val} onClick={() => setRuleDraft({ ...ruleDraft, pattern: p.val })}
                                                            className="px-2 py-1 bg-surface-2 text-[10px] text-txt-2 rounded border border-border hover:border-brand hover:text-brand transition-colors">
                                                            {p.label}
                                                        </button>
                                                    ))}
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                )}

                                {/* Email - only for text types */}
                                {['text', 'character varying'].includes(ruleColType) && (
                                    <div className={`p-3 rounded-lg border transition-colors ${ruleDraft.emailOn ? 'bg-brand/5 border-brand/30' : 'bg-surface-0 border-border'}`}>
                                        <div className="flex items-center justify-between">
                                            <div>
                                                <span className="text-sm font-medium text-txt-0">Must be a valid Email</span>
                                                <p className="text-[10px] text-txt-3 mt-0.5">Only accept valid email addresses (e.g. user@example.com)</p>
                                            </div>
                                            <button onClick={() => setRuleDraft({ ...ruleDraft, emailOn: !ruleDraft.emailOn })}
                                                className={`relative w-9 h-5 rounded-full transition-colors ${ruleDraft.emailOn ? 'bg-brand' : 'bg-surface-3'}`}>
                                                <span className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${ruleDraft.emailOn ? 'left-[18px]' : 'left-0.5'}`} />
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </div>

                            <div className="flex gap-2 justify-end mt-6">
                                <button onClick={() => setRuleCol(null)} className="px-4 py-2 bg-surface-2 text-txt-1 text-sm rounded-md hover:bg-surface-3 transition-colors">Cancel</button>
                                <button onClick={saveRules} className="px-4 py-2 bg-brand text-surface-0 text-sm font-medium rounded-md hover:bg-brand-dark transition-colors flex items-center gap-2 shadow-lg">
                                    <ShieldCheck className="w-4 h-4" /> Save Rules
                                </button>
                            </div>
                        </div>
                    </div>
                )
            }
        </div >
    );
}
