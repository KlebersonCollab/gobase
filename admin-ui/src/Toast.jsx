import { createContext, useContext, useState, useCallback } from 'react';

const ToastCtx = createContext();

export function ToastProvider({ children }) {
    const [toasts, setToasts] = useState([]);

    const toast = useCallback((msg, error = false) => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, msg, error }]);
        setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 3500);
    }, []);

    return (
        <ToastCtx.Provider value={toast}>
            {children}
            <div className="fixed bottom-4 right-4 z-50 space-y-2">
                {toasts.map(t => (
                    <div key={t.id} className={`toast-enter px-4 py-2.5 rounded-lg text-sm shadow-lg border ${t.error ? 'bg-red-950 border-red-800 text-red-200' : 'bg-surface-2 border-border text-txt-0'}`}>
                        {t.msg}
                    </div>
                ))}
            </div>
        </ToastCtx.Provider>
    );
}

export function useToast() { return useContext(ToastCtx); }
