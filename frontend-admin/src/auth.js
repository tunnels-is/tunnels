const KEY = 'admin_auth';

export function getAuth() {
  try {
    return JSON.parse(localStorage.getItem(KEY) || 'null');
  } catch {
    return null;
  }
}

export function setAuth(auth) {
  localStorage.setItem(KEY, JSON.stringify(auth));
}

export function clearAuth() {
  localStorage.removeItem(KEY);
}

export function isLoggedIn() {
  const auth = getAuth();
  return !!(auth && auth._id && auth.DeviceToken && auth.IsAdmin);
}
