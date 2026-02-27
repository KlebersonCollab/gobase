const API = '';

export function getToken() {
    return localStorage.getItem('gobase_token') || '';
}

export function setToken(t) {
    localStorage.setItem('gobase_token', t);
}

export function getRefreshToken() {
    return localStorage.getItem('gobase_refresh_token') || '';
}

export function setRefreshToken(t) {
    localStorage.setItem('gobase_refresh_token', t);
}

export function clearToken() {
    localStorage.removeItem('gobase_token');
    localStorage.removeItem('gobase_refresh_token');
}

export function parseJwt(t) {
    try { return JSON.parse(atob(t.split('.')[1])); } catch { return {}; }
}

export function isTokenExpired(t) {
    if (!t) return true;
    const payload = parseJwt(t);
    if (!payload.exp) return true;
    return Date.now() >= payload.exp * 1000;
}

async function refreshAccessToken() {
    const rt = getRefreshToken();
    if (!rt) return false;

    try {
        const res = await fetch(API + '/api/auth/refresh', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: rt }),
        });
        if (!res.ok) return false;

        const data = await res.json();
        setToken(data.token);
        setRefreshToken(data.refresh_token);
        return true;
    } catch {
        return false;
    }
}

export async function req(path, opts = {}) {
    // If token is expired, try refresh before making the request
    if (isTokenExpired(getToken())) {
        const refreshed = await refreshAccessToken();
        if (!refreshed) {
            clearToken();
            window.location.href = '/admin/'; // Force redirect to login
            return;
        }
    }

    const res = await fetch(API + path, {
        method: opts.method || 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${getToken()}`
        },
        body: opts.body ? JSON.stringify(opts.body) : undefined,
    });

    // If 401 despite valid-looking token, try refresh once
    if (res.status === 401) {
        const refreshed = await refreshAccessToken();
        if (refreshed) {
            // Retry original request with new token
            const retry = await fetch(API + path, {
                method: opts.method || 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${getToken()}`
                },
                body: opts.body ? JSON.stringify(opts.body) : undefined,
            });
            if (retry.status === 401) {
                clearToken();
                window.location.reload();
                return;
            }
            if (!retry.ok) {
                const txt = await retry.text();
                throw new Error(txt || `Error ${retry.status}`);
            }
            if (retry.status === 204 || retry.headers.get('content-length') === '0') return null;
            return retry.json();
        }
        clearToken();
        window.location.reload();
        return;
    }

    if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || `Error ${res.status}`);
    }
    if (res.status === 204 || res.headers.get('content-length') === '0') return null;
    return res.json();
}
