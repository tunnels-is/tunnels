import { clearUserMeta } from './auth';

export async function apiPost(path, body = {}) {
  const resp = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(body),
  });

  if (resp.status === 401) {
    clearUserMeta();
    window.location.hash = '/login';
    throw new Error('Unauthorized');
  }

  return resp;
}

export async function apiPostRaw(path, body = {}) {
  const resp = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(body),
  });
  return resp;
}

export async function apiGet(path) {
  const resp = await fetch(path, {
    method: 'GET',
    credentials: 'include',
  });

  if (resp.status === 401) {
    clearUserMeta();
    window.location.hash = '/login';
    throw new Error('Unauthorized');
  }

  return resp;
}
