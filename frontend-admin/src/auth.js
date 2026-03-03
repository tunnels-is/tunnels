const USER_META_KEY = 'admin_meta';

export function getUserMeta() {
  try {
    return JSON.parse(localStorage.getItem(USER_META_KEY) || 'null');
  } catch {
    return null;
  }
}

export function setUserMeta(meta) {
  localStorage.setItem(USER_META_KEY, JSON.stringify(meta));
}

export function clearUserMeta() {
  localStorage.removeItem(USER_META_KEY);
}

export function isLoggedIn() {
  const meta = getUserMeta();
  return !!(meta && (meta.IsAdmin || meta.IsManager));
}
