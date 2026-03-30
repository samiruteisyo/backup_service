const API_BASE = '/api';

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || `HTTP ${res.status}`);
  }

  return res.json();
}

export const api = {
  login: (username, password) =>
    request('/login', { method: 'POST', body: JSON.stringify({ username, password }) }),

  logout: () => request('/logout', { method: 'POST' }),

  getProjects: () => request('/projects'),

  getProject: (name) => request(`/projects/${encodeURIComponent(name)}`),

  backupProject: (name) =>
    request(`/projects/${encodeURIComponent(name)}/backup`, { method: 'POST' }),

  deleteBackup: (name, timestamp) =>
    request(`/projects/${encodeURIComponent(name)}/backup/${encodeURIComponent(timestamp)}`, { method: 'DELETE' }),

  restoreProject: (name, timestamp) =>
    request(`/projects/${encodeURIComponent(name)}/restore`, {
      method: 'POST',
      body: JSON.stringify({ timestamp }),
    }),

  deployProject: (name) =>
    request(`/projects/${encodeURIComponent(name)}/deploy`, { method: 'POST' }),

  rollbackProject: (name, sha) =>
    request(`/projects/${encodeURIComponent(name)}/rollback`, {
      method: 'POST',
      body: JSON.stringify({ sha }),
    }),
};
