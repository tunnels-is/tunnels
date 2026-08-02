import { v4 as uuid } from "uuid"
import { create } from "zustand"
import { session } from "./session"

export const useStore = create((set, get) => ({

	user: undefined,
	users: [],
	config: session.getObject("config"),
	state: session.getObject("state"),
	tunnels: [],
	activeTunnels: [],
	servers: session.getObject("servers") || [],
	// Controller devices for the active account — never session-persisted.
	devices: [],
	// This machine's devices under accounts/<hash>/devices (no privkeys).
	localDevices: [],
	dnsStats: session.getObject("dns-stats") || {},

	logs: [],
	version: undefined,
	apiVersion: undefined,
	timezone: undefined,

	advanced: window.localStorage.getItem("advanced") === "true",
	setAdvanced: (advanced) => {
		window.localStorage.setItem("advanced", String(advanced))
		set({ advanced })
	},
	loading: null,
	confirm: null,
	toasts: [],

	setUser: (user) => {
		if (user?._id) session.set("activeUserID", user._id)
		set({ user })
	},
	setConfig: (config) => {
		session.setObject("config", config)
		set({ config })
	},
	setServers: (servers) => {
		session.setObject("servers", servers)
		set({ servers })
	},
	setDevices: (devices) => set({ devices: Array.isArray(devices) ? devices : [] }),
	setLocalDevices: (localDevices) => set({ localDevices: Array.isArray(localDevices) ? localDevices : [] }),

	appendLog: (line) => {
		let logs = get().logs

		if (logs.length >= 5000) logs = logs.slice(-4000)
		logs = [...logs, line]
		set({ logs })
	},
	clearLogs: () => {
		set({ logs: [] })
	},

	showLoading: (msg, timeout = 10000) => {
		clearTimeout(get()._loadingTimer)
		const _loadingTimer = setTimeout(() => set({ loading: null }), timeout)
		set({ loading: { msg }, _loadingTimer })
	},
	hideLoading: () => {
		clearTimeout(get()._loadingTimer)
		set({ loading: null })
	},

	askConfirm: (title, subtitle, onConfirm) => set({ confirm: { title, subtitle, onConfirm } }),
	closeConfirm: () => set({ confirm: null }),

	_lastError: 0,
	notifySuccess: (msg) => pushToast(set, "success", msg),
	notifyError: (msg) => {

		if (Date.now() - get()._lastError < 3000) return
		set({ _lastError: Date.now() })
		pushToast(set, "error", msg)
	},
	removeToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))

const pushToast = (set, type, msg) => {
	const id = uuid()
	set((s) => ({ toasts: [...s.toasts, { id, type, msg }] }))
	setTimeout(() => useStore.getState().removeToast(id), 3000)
}
