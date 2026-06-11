// Single zustand store for all global state. Async backend logic lives in
// ./actions.js — this file only holds data and small synchronous UI helpers.

import { create } from "zustand"
import { session } from "./session"

export const useStore = create((set, get) => ({
	// --- backend data (seeded from sessionStorage for instant first paint) ---
	user: session.getObject("user"),
	users: [],
	config: session.getObject("config"),
	state: session.getObject("state"),
	network: undefined,
	tunnels: [],
	activeTunnels: [],
	servers: session.getObject("servers") || [],
	dnsStats: session.getObject("dns-stats") || {},
	logs: session.getObject("logs") || [],
	version: undefined,
	apiVersion: undefined,

	// --- ui state ---
	loading: null, // { msg }
	confirm: null, // { title, subtitle, onConfirm }
	toasts: [], // { id, type: "success" | "error", msg }

	setUser: (user) => {
		session.setObject("user", user)
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

	appendLog: (line) => {
		let logs = get().logs
		if (logs.length > 5000) logs = []
		logs = [...logs, line]
		session.setObject("logs", logs)
		set({ logs })
	},
	clearLogs: () => {
		session.setObject("logs", [])
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
		// errors are throttled so a failing poll doesn't stack toasts
		if (Date.now() - get()._lastError < 3000) return
		set({ _lastError: Date.now() })
		pushToast(set, "error", msg)
	},
	removeToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))

const pushToast = (set, type, msg) => {
	const id = crypto.randomUUID()
	set((s) => ({ toasts: [...s.toasts, { id, type, msg }] }))
	setTimeout(() => useStore.getState().removeToast(id), 3000)
}
