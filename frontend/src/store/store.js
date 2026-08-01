// Single zustand store for all global state. Async backend logic lives in
// ./actions.js — this file only holds data and small synchronous UI helpers.

import { v4 as uuid } from "uuid"
import { create } from "zustand"
import { session } from "./session"

export const useStore = create((set, get) => ({
	// --- backend data (seeded from sessionStorage for instant first paint) ---
	// The `user` object (which carries the device token, API key and license
	// key) is deliberately NOT persisted to web storage — it is held in memory
	// only and re-fetched from the local daemon on reload (see fetchState).
	// Only the active user's id is persisted, so the right account can be
	// re-selected without exposing any secret.
	user: undefined,
	users: [],
	config: session.getObject("config"),
	state: session.getObject("state"),
	network: undefined,
	tunnels: [],
	activeTunnels: [],
	servers: session.getObject("servers") || [],
	dnsStats: session.getObject("dns-stats") || {},
	// Logs may contain tokens/IPs when debug logging is on — kept in memory only.
	logs: [],
	version: undefined,
	apiVersion: undefined,
	timezone: undefined,

	// --- ui state ---
	// advanced mode exposes the full configuration surface; defaults to off.
	// Stored in localStorage (not sessionStorage) so it survives restarts.
	advanced: window.localStorage.getItem("advanced") === "true",
	setAdvanced: (advanced) => {
		window.localStorage.setItem("advanced", String(advanced))
		set({ advanced })
	},
	loading: null, // { msg }
	confirm: null, // { title, subtitle, onConfirm }
	toasts: [], // { id, type: "success" | "error", msg }

	setUser: (user) => {
		// Persist only the non-secret active-user id; the full user (with its
		// device token) stays in memory. The daemon re-provides it on reload.
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

	appendLog: (line) => {
		let logs = get().logs
		// Ring-buffer: keep the most recent lines instead of discarding the whole
		// history when the cap is hit (which blanked the Logs view).
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
		// errors are throttled so a failing poll doesn't stack toasts
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
