const STORAGE_KEY = "tunnels-admin:theme";

export const getTheme = () => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {

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
};

export const setTheme = (theme) => {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {

  }
  applyTheme(theme);
};

applyTheme(getTheme());
