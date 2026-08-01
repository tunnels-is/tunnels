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

// Dev vite ports proxy to the local API; production / Wails load the UI from
// the API origin itself (e.g. http://127.0.0.1:7777), so same-origin is enough.
const DEV_PORTS = ["5173", "5174", "5175"]

export const baseURL = () => {
	let host = window.location.origin
	if (import.meta.env.DEV || session.getBool("dev")) {
		DEV_PORTS.forEach((p) => (host = host.replace(p, "7777")))
	}
	return host
}

export const wsURL = (route) => baseURL().replace(/^http/, "ws") + "/" + route

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

// De-dupes identical in-flight controller requests by sharing the same promise.
// Keyed on route + payload so two DIFFERENT requests to the same route (e.g. a
// background poll and a user action) both run — the old set-of-routes collapsed
// them and returned a fake {status:0} to the loser.
const inFlight = new Map()

export const callController = async (server, route, data = {}, { auth = true, timeout = 30000 } = {}) => {
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

	// Key MUST include the target server: two identical-payload requests aimed at
	// different control servers (e.g. a login retried after switching the server
	// dropdown) would otherwise collapse into one, and the second caller would
	// get the first server's response misattributed to its own server.
	const key = JSON.stringify(server) + "|" + route + "|" + JSON.stringify(data)
	const existing = inFlight.get(key)
	if (existing) return existing

	const p = post("/v1/method/forwardToController", request, timeout)
	inFlight.set(key, p)
	try {
		return await p
	} finally {
		inFlight.delete(key)
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
