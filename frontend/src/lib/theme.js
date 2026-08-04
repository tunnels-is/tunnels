export const THEMES = [
	{ value: "suzko", label: "Suzko" },
	{ value: "tunnels", label: "Tunnels Light" },
	{ value: "tunnels-dark", label: "Tunnels Dark" },
]

const VALUES = THEMES.map((t) => t.value)
const DEFAULT_THEME = "tunnels"


const KEY_THEME = "theme"
const KEY_SYSTEM = "themeSystem"


const THEME_ALIASES = {
	"suzko-dark": "suzko",
}

const LIGHT_DARK = {

	suzko: { light: "suzko", dark: "suzko" },
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

	}
}


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

	}
}

const normalizeTheme = (theme) => {
	if (!theme) return DEFAULT_THEME
	const mapped = THEME_ALIASES[theme] || theme
	return VALUES.includes(mapped) ? mapped : DEFAULT_THEME
}

migrateFromSession()


;(() => {
	const stored = storageGet(KEY_THEME)
	if (stored && THEME_ALIASES[stored]) {
		storageSet(KEY_THEME, THEME_ALIASES[stored])
	}
})()

export const getStoredTheme = () => normalizeTheme(storageGet(KEY_THEME))

export const followsSystem = () => storageGet(KEY_SYSTEM) === "true"

export const getTheme = () => {
	const base = getStoredTheme()
	if (!followsSystem()) return base
	const pair = LIGHT_DARK[base] ?? LIGHT_DARK[DEFAULT_THEME]
	return prefersDark() ? pair.dark : pair.light
}

export const setTheme = (theme) => {
	storageSet(KEY_THEME, normalizeTheme(theme))
	applyTheme(getTheme())
}

export const setFollowSystem = (on) => {
	storageSet(KEY_SYSTEM, on ? "true" : "false")
	applyTheme(getTheme())
}

export const applyTheme = (theme) => {
	document.documentElement.setAttribute("data-theme", normalizeTheme(theme))
}

window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => {
	if (followsSystem()) applyTheme(getTheme())
})

applyTheme(getTheme())
