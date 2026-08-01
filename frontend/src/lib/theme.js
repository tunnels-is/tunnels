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
const DEFAULT_THEME = "tunnels"

// Light/dark counterparts grouped by family, so "follow system" can swap
// between them while keeping the user's chosen family.
const LIGHT_DARK = {
	suzko: { light: "suzko", dark: "suzko-dark" },
	"suzko-dark": { light: "suzko", dark: "suzko-dark" },
	tunnels: { light: "tunnels", dark: "tunnels-dark" },
	"tunnels-dark": { light: "tunnels", dark: "tunnels-dark" },
}

const prefersDark = () => window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false

// The theme the user explicitly selected, before any "follow system" override.
export const getStoredTheme = () => {
	const stored = session.get("theme")
	return VALUES.includes(stored) ? stored : DEFAULT_THEME
}

export const followsSystem = () => session.getBool("themeSystem")

// The theme actually applied to <html>. When following the system, the chosen
// family's light or dark variant is picked based on `prefers-color-scheme`.
export const getTheme = () => {
	const base = getStoredTheme()
	if (!followsSystem()) return base
	const pair = LIGHT_DARK[base] ?? LIGHT_DARK[DEFAULT_THEME]
	return prefersDark() ? pair.dark : pair.light
}

export const setTheme = (theme) => {
	session.set("theme", theme)
	applyTheme(getTheme())
}

export const setFollowSystem = (on) => {
	session.set("themeSystem", on ? "true" : "false")
	applyTheme(getTheme())
}

export const applyTheme = (theme) => {
	document.documentElement.setAttribute("data-theme", theme)
}

// Re-apply when the OS light/dark preference changes while following the system.
window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => {
	if (followsSystem()) applyTheme(getTheme())
})

applyTheme(getTheme())
