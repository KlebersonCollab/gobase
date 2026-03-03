import React, { useState, useEffect } from 'react';
import { req } from '../api';

export default function RelationshipSelect({ table, value, onChange, labelField = 'id' }) {
    const [options, setOptions] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let isMounted = true;
        setLoading(true);
        req(`/api/collections/${table}`)
            .then(data => {
                if (isMounted) {
                    setOptions(data || []);
                    setLoading(false);
                }
            })
            .catch(() => {
                if (isMounted) setLoading(false);
            });
        return () => { isMounted = false; };
    }, [table]);

    return (
        <select
            role="combobox"
            value={value || ''}
            onChange={(e) => onChange(e.target.value)}
            disabled={loading}
            className="w-full px-3 py-2 bg-surface-2 border border-border rounded-md text-sm text-txt-0 focus:border-brand outline-none transition disabled:opacity-50"
        >
            <option value="">Select {table}...</option>
            {options.map(opt => (
                <option key={opt.id} value={opt.id}>
                    {opt[labelField] || opt.name || opt.id}
                </option>
            ))}
        </select>
    );
}
