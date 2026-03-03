import React from 'react';
import RelationshipSelect from './RelationshipSelect';

export default function DynamicForm({ columns, relations = [], value, onChange }) {

    // Auxiliar para encontrar se uma coluna é uma FK
    const getRelation = (colName) => {
        return relations.find(r => r.column === colName);
    };

    return (
        <form className="space-y-4">
            {columns.map(col => {
                const id = `field-${col.name}`;
                const rel = getRelation(col.name);

                return (
                    <div key={col.name} className="flex flex-col gap-1.5">
                        <label htmlFor={id} className="text-xs font-medium text-txt-2 px-1 capitalize">
                            {col.name}
                            {col.required && <span className="text-brand ml-1">*</span>}
                        </label>

                        {rel ? (
                            <RelationshipSelect
                                table={rel.references_table}
                                value={value[col.name]}
                                onChange={(val) => onChange({ ...value, [col.name]: val })}
                                labelField="name"
                            />
                        ) : (col.type === 'jsonb' || col.type === 'json') ? (
                            <textarea
                                id={id}
                                value={value[col.name] && typeof value[col.name] === 'object' ? JSON.stringify(value[col.name], null, 2) : value[col.name] || ''}
                                onChange={(e) => {
                                    let val = e.target.value;
                                    try { val = JSON.parse(e.target.value); } catch (err) { /* string */ }
                                    onChange({ ...value, [col.name]: val });
                                }}
                                className="w-full px-4 py-3 bg-surface-2 border border-border rounded-md text-xs text-txt-1 font-mono focus:border-brand outline-none resize-y"
                                rows={4}
                            />
                        ) : col.type === 'boolean' ? (
                            <input
                                id={id}
                                type="checkbox"
                                checked={!!value[col.name]}
                                onChange={(e) => onChange({ ...value, [col.name]: e.target.checked })}
                                className="w-4 h-4 rounded border-border bg-surface-2 text-brand focus:ring-brand cursor-pointer"
                            />
                        ) : (
                            <input
                                id={id}
                                type={col.type === 'integer' ? 'number' : col.type === 'timestamp' ? 'datetime-local' : 'text'}
                                value={value[col.name] || ''}
                                required={col.required}
                                onChange={(e) => onChange({ ...value, [col.name]: e.target.value })}
                                className="w-full px-3 py-2 bg-surface-2 border border-border rounded-md text-sm text-txt-0 placeholder-txt-3 focus:border-brand outline-none transition"
                                placeholder={col.type === 'uuid' ? '00000000-0000-0000-0000-000000000000' : `Enter ${col.name}...`}
                            />
                        )}
                    </div>
                );
            })}
        </form>
    );
}
