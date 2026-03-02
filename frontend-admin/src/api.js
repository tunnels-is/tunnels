import { getAuth, clearAuth } from './auth';

export async function apiPost(path, body = {}) {
  const auth = getAuth();
  const payload = { ...body };
  if (auth) {
    payload.UID = auth._id;
    payload.DeviceToken = auth.DeviceToken;
  }

  const resp = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (resp.status === 401) {
    clearAuth();
    window.location.hash = '/login';
    throw new Error('Unauthorized');
  }

  return resp;
}

export async function apiPostRaw(path, body = {}) {
  const resp = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return resp;
}
