// Lightweight theme manager — persists user choice to localStorage and
// toggles a `dark` class on <html>. Imported once at app startup so the
// class is applied before React mounts (prevents a flash).

const STORAGE_KEY = "tunnels:theme";

export const getTheme = () => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // ignore
  }
  return "light";
};

export const applyTheme = (theme) => {
  const root = document.documentElement;
  if (theme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
  // Keep the placeholder #app bg in sync with the theme so the brief
  // pre-paint flash matches.
  const appEl = document.getElementById("app");
  if (appEl) {
    appEl.style.backgroundColor = theme === "dark" ? "#0e1116" : "#fdfcf8";
  }
};

export const setTheme = (theme) => {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // ignore
  }
  applyTheme(theme);
};

// Apply on module import (synchronous, before React mounts).
applyTheme(getTheme());
