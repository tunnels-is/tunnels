// HTTP client for the local tunnels backend.
//
// Two call shapes:
//   callMethod(method, data)              -> POST {base}/v1/method/{method}
//   callController(server, route, data)   -> POST {base}/v1/method/forwardToController
//
// Both resolve to { status, data, networkError } and never throw.
// status 0 + networkError means the request never reached the backend —
// expected when connecting a tunnel changes the network underneath us.

import { session } from "@/store/session"

export const isWails = () => {
	const h = window.location.hostname
	return h === "wails.localhost" || h === "wails" || window.location.protocol === "wails:"
}

const DEV_PORTS = ["5173", "5174", "5175"]

export const baseURL = () => {
	if (isWails()) return "http://127.0.0.1:7777"
	let host = window.location.origin
	if (import.meta.env.DEV || session.getBool("dev")) {
		DEV_PORTS.forEach((p) => (host = host.replace(p, "7777")))
	}
	return host
}

export const wsURL = (route) => {
	if (isWails()) return "ws://127.0.0.1:7777/" + route
	return baseURL().replace(/^http/, "ws") + "/" + route
}

const post = async (path, body, timeout) => {
	const controller = new AbortController()
	const timer = setTimeout(() => controller.abort(), timeout)
	try {
		const resp = await fetch(baseURL() + path, {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(body ?? {}),
			signal: controller.signal,
		})
		let data
		try {
			data = await resp.json()
		} catch {
			data = undefined
		}
		return { status: resp.status, data, networkError: false }
	} catch {
		return { status: 0, data: undefined, networkError: true }
	} finally {
		clearTimeout(timer)
	}
}

export const callMethod = (method, data, { timeout = 30000 } = {}) =>
	post("/v1/method/" + method, data, timeout)

// Tracks in-flight controller routes so a slow request isn't fired twice.
const inFlight = new Set()

export const callController = async (server, route, data = {}, { auth = true, timeout = 30000 } = {}) => {
	if (inFlight.has(route)) return { status: 0, data: undefined, networkError: false }

	if (auth) {
		data.UID = data.UID || ""
		data.DeviceToken = data.DeviceToken || ""
		if (!data.DeviceToken) {
			return { status: 401, data: { Error: "No auth token found, please log in again" }, networkError: false }
		}
	}

	const request = {
		Server: server,
		Path: route,
		Method: "POST",
		JSONData: data,
		Timeout: 20000,
		Headers: auth ? { "X-Device-Token": data.DeviceToken, "X-UID": data.UID } : undefined,
	}

	inFlight.add(route)
	try {
		return await post("/v1/method/forwardToController", request, timeout)
	} finally {
		inFlight.delete(route)
	}
}

// Extracts a human-readable message from the backend's various error shapes:
// {Error}, {Message}, {error}, plain string, or an array of strings.
export const errorMessage = (data, fallback = "Unknown error") => {
	if (!data) return fallback
	if (typeof data === "string") return data
	if (Array.isArray(data)) return data.join(".\n")
	return data.Error || data.Message || data.error || fallback
}
