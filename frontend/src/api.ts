export const API_BASE = '/api';

export function setToken(token: string) {
  localStorage.setItem('SUPERUSER_TOKEN', token);
}

export function getToken() {
  return localStorage.getItem('SUPERUSER_TOKEN') || '';
}

export function hasToken() {
  return !!getToken();
}

export function logout() {
  localStorage.removeItem('SUPERUSER_TOKEN');
  window.location.href = '/login';
}

export async function fetchApi<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken();
  const headers = new Headers(options?.headers);
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (!headers.has('Content-Type') && options?.body && typeof options.body === 'string') {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    logout();
    throw new Error('Unauthorized');
  }

  const isJson = res.headers.get('content-type')?.includes('application/json');
  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    if (isJson) {
      const errBody = await res.json();
      msg = errBody.error || msg;
    } else {
      msg = await res.text() || msg;
    }
    throw new Error(msg);
  }

  if (isJson) {
    return res.json();
  }
  return res.text() as unknown as T;
}
