const PREFIX = "data_"

export const session = {
	get(key) {
		return window.sessionStorage.getItem(key) ?? undefined
	},
	set(key, value) {
		window.sessionStorage.setItem(key, value)
	},
	getBool(key) {
		return window.sessionStorage.getItem(key) === "true"
	},
	getObject(key) {
		try {
			const raw = window.sessionStorage.getItem(PREFIX + key)
			if (!raw || raw === "undefined" || raw === '""') return undefined
			return JSON.parse(raw) ?? undefined
		} catch {
			return undefined
		}
	},
	setObject(key, value) {
		try {
			window.sessionStorage.setItem(PREFIX + key, JSON.stringify(value))
		} catch {

		}
	},
	clear() {
		window.sessionStorage.clear()
	},
}
