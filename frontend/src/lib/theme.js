export const THEMES = [
	{ value: "suzko", label: "Suzko Light" },
	{ value: "suzko-dark", label: "Suzko Dark" },
	{ value: "tunnels", label: "Tunnels Light" },
	{ value: "tunnels-dark", label: "Tunnels Dark" },
]

const VALUES = THEMES.map((t) => t.value)
const DEFAULT_THEME = "tunnels"

// localStorage so the choice survives app restarts (sessionStorage does not in Wails/WebView2).
const KEY_THEME = "theme"
const KEY_SYSTEM = "themeSystem"

const LIGHT_DARK = {
	suzko: { light: "suzko", dark: "suzko-dark" },
	"suzko-dark": { light: "suzko", dark: "suzko-dark" },
	tunnels: { light: "tunnels", dark: "tunnels-dark" },
	"tunnels-dark": { light: "tunnels", dark: "tunnels-dark" },
}

const prefersDark = () => window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false

const storageGet = (key) => {
	try {
		return window.localStorage.getItem(key) ?? undefined
	} catch {
		return undefined
	}
}

const storageSet = (key, value) => {
	try {
		window.localStorage.setItem(key, value)
	} catch {
		/* private mode / blocked storage */
	}
}

/** One-time migration from the old sessionStorage keys. */
const migrateFromSession = () => {
	try {
		if (!storageGet(KEY_THEME)) {
			const old = window.sessionStorage.getItem(KEY_THEME)
			if (old) storageSet(KEY_THEME, old)
		}
		if (storageGet(KEY_SYSTEM) == null) {
			const old = window.sessionStorage.getItem(KEY_SYSTEM)
			if (old != null) storageSet(KEY_SYSTEM, old)
		}
	} catch {
		/* ignore */
	}
}

migrateFromSession()

export const getStoredTheme = () => {
	const stored = storageGet(KEY_THEME)
	return VALUES.includes(stored) ? stored : DEFAULT_THEME
}

export const followsSystem = () => storageGet(KEY_SYSTEM) === "true"

export const getTheme = () => {
	const base = getStoredTheme()
	if (!followsSystem()) return base
	const pair = LIGHT_DARK[base] ?? LIGHT_DARK[DEFAULT_THEME]
	return prefersDark() ? pair.dark : pair.light
}

export const setTheme = (theme) => {
	storageSet(KEY_THEME, theme)
	applyTheme(getTheme())
}

export const setFollowSystem = (on) => {
	storageSet(KEY_SYSTEM, on ? "true" : "false")
	applyTheme(getTheme())
}

export const applyTheme = (theme) => {
	document.documentElement.setAttribute("data-theme", theme)
}

window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => {
	if (followsSystem()) applyTheme(getTheme())
})

applyTheme(getTheme())
