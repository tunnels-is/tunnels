// Theme switching between the two daisyUI themes. Persisted in sessionStorage
// and applied as `data-theme` on <html> before React mounts (no flash).

import { session } from "@/store/session"

export const THEMES = { light: "suzko", dark: "suzko-dark" }

export const getTheme = () => {
	const stored = session.get("theme")
	if (stored === THEMES.light || stored === THEMES.dark) return stored
	return window.matchMedia?.("(prefers-color-scheme: dark)").matches
		? THEMES.dark
		: THEMES.light
}

export const setTheme = (theme) => {
	session.set("theme", theme)
	applyTheme(theme)
}

export const applyTheme = (theme) => {
	document.documentElement.setAttribute("data-theme", theme)
}

applyTheme(getTheme())
