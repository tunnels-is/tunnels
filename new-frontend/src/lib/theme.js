// Theme switching between the daisyUI themes defined in app.css. Persisted in
// sessionStorage and applied as `data-theme` on <html> before React mounts.

import { session } from "@/store/session"

export const THEMES = [
	{ value: "suzko", label: "Suzko Light" },
	{ value: "suzko-dark", label: "Suzko Dark" },
	{ value: "tunnels", label: "Tunnels Light" },
	{ value: "tunnels-dark", label: "Tunnels Dark" },
]

const VALUES = THEMES.map((t) => t.value)

export const getTheme = () => {
	const stored = session.get("theme")
	if (VALUES.includes(stored)) return stored
	return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "suzko-dark" : "suzko"
}

export const setTheme = (theme) => {
	session.set("theme", theme)
	applyTheme(theme)
}

export const applyTheme = (theme) => {
	document.documentElement.setAttribute("data-theme", theme)
}

applyTheme(getTheme())
