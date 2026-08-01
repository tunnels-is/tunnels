// All async backend interactions. Pages call these (or `api`/`controller`
// directly for page-local data) — nothing else talks to the network.

import { callController, callMethod, errorMessage } from "@/api/client"
import { session } from "./session"
import { useStore } from "./store"

const store = () => useStore.getState()

// Local backend call with shared error handling. 401 logs the user out
// unless opts.logout === false; networkError is treated as "the tunnel came
// up and changed the network" which kills the in-flight request by design.
export const api = async (method, data, opts = {}) => {
	const resp = await callMethod(method, data, opts)
	if (resp.networkError) {
		store().notifySuccess("Tunnel connected, network changed")
		return resp
	}
	if (resp.status === 401 && opts.logout !== false) {
		logoutCurrentToken()
		return resp
	}
	if (resp.status !== 200 && !opts.silent) {
		store().notifyError(errorMessage(resp.data))
	}
	return resp
}

// Controller (remote control server) call, authenticated as the active user.
export const controller = async (route, data = {}, opts = {}) => {
	const user = store().user
	const server = opts.server || user?.ControlServer
	if (!server) {
		store().notifyError("No control server found, please log in again")
		return { status: 0 }
	}
	if (opts.auth !== false) {
		data.UID = user?._id || ""
		data.DeviceToken = user?.DeviceToken?.DT || ""
	}

	store().showLoading(server.Host + route)
	const resp = await callController(server, route, data, opts)
	store().hideLoading()

	if (resp.networkError) {
		store().notifySuccess("Tunnel connected, network changed")
	} else if (resp.status !== 200 && resp.status !== 0 && !opts.silent) {
		store().notifyError(errorMessage(resp.data))
	}
	return resp
}

// ---------------------------------------------------------------- state ---

let stateFetchInProgress = false
export const fetchState = async () => {
	if (stateFetchInProgress) return
	stateFetchInProgress = true
	try {
		const resp = await callMethod("getState")
		if (resp.status !== 200) {
			if (!resp.networkError) store().notifyError("Unable to load state")
			return
		}
		const d = resp.data || {}
		session.setObject("state", d.State)
		session.setObject("config", d.Config)
		useStore.setState({
			state: d.State,
			config: d.Config,
			network: d.Network,
			tunnels: d.Tunnels || [],
			activeTunnels: d.ActiveTunnels || [],
			version: d.Version,
			apiVersion: d.APIVersion,
			timezone: d.Timezone,
		})

		// First load without a user: re-select the previously-active account by
		// its persisted id (the token is never in web storage, so it's re-fetched
		// from the daemon here), else auto-pick a lone account or show the picker.
		// Skipped on the login/account routes so "Add Account" and the Login form
		// aren't immediately bounced away by the auto-select (fetchState is also
		// called from Login's own mount effect).
		const hash = window.location.hash
		const onAuthRoute = hash.startsWith("#/login") || hash.startsWith("#/accounts")
		// Only redirect to the dashboard when there's no meaningful deep-link to
		// preserve (fresh open / root). Since `user` is no longer persisted in web
		// storage, fetchState re-selects the account on every reload; forcing
		// "#/dashboard" here would bounce a user off whatever authenticated page
		// (e.g. /tunnels/foo/edit) they just refreshed.
		const onNoRoute = hash === "" || hash === "#" || hash === "#/"
		if (!store().user && !onAuthRoute) {
			const users = await loadUsers()
			const activeID = session.get("activeUserID")
			const match = activeID && users?.find((u) => u._id === activeID)
			if (match) {
				store().setUser(match)
				if (onNoRoute) window.location.hash = "#/dashboard"
			} else if (users?.length === 1) {
				store().setUser(users[0])
				if (onNoRoute) window.location.hash = "#/dashboard"
			} else if (users?.length > 0) {
				useStore.setState({ users })
				window.location.hash = "#/accounts"
			}
		}
	} finally {
		stateFetchInProgress = false
	}
}

// ---------------------------------------------------------------- users ---

export const loadUsers = async () => {
	const resp = await api("getUsers", null, { logout: false, silent: true })
	// Always return an array so callers' .find/.length can't throw on an
	// unexpected backend shape.
	return resp.status === 200 && Array.isArray(resp.data) ? resp.data : []
}

export const fetchUsers = async () => {
	const users = await loadUsers()
	if (users?.length > 0) useStore.setState({ users })
	return users
}

// Login success: attach the auth server, set active user and optionally
// persist it (encrypted) on disk.
export const loginUser = async (user, remember, server) => {
	user.ControlServer = server
	store().setUser(user)
	if (remember) await saveUserToDisk(user)
}

export const saveUserToDisk = async (user) => {
	const resp = await api("setUser", user, { logout: false })
	if (resp.status === 200 && resp.data?.SaveFileHash) {
		user.SaveFileHash = resp.data.SaveFileHash
		store().setUser({ ...user })
	}
}

export const deleteUserFile = (hash) => api("delUser", { Hash: hash }, { logout: false })

export const finalizeLogout = () => {
	session.clear()
	window.location.replace("/#/login")
	window.location.reload()
}

export const logoutToken = async (token, all) => {
	const user = store().user
	if (!user) {
		finalizeLogout()
		return
	}
	const isOwnToken = user.DeviceToken?.DT === token?.DT

	const resp = await controller("/client/user/logout", { LogoutToken: token?.DT, All: all }, { silent: true })
	if (resp.status === 200) {
		store().notifySuccess("Device logged out")
		if (isOwnToken || all) {
			await deleteUserFile(user.SaveFileHash)
			finalizeLogout()
			return
		}
		user.Tokens = user.Tokens?.filter((t) => t.DT !== token?.DT)
		store().setUser({ ...user })
	} else if (resp.status === 401) {
		// token already invalid server-side — clean up locally regardless
		await deleteUserFile(user.SaveFileHash)
		finalizeLogout()
	} else {
		store().notifyError("Unable to log out device")
		if (isOwnToken || all) finalizeLogout()
	}
}

export const logoutCurrentToken = () => logoutToken(store().user?.DeviceToken, false)
export const logoutAllTokens = () => logoutToken(store().user?.DeviceToken, true)

export const refreshApiKey = async () => {
	// The server generates the key now (the client no longer chooses its own
	// secret); we send a non-empty sentinel to request one and use what's
	// returned.
	const resp = await controller("/client/user/update", { APIKey: "generate" })
	if (resp.status === 200 && resp.data?.APIKey) {
		store().setUser({ ...store().user, APIKey: resp.data.APIKey })
		store().notifySuccess("User updated")
	} else if (resp.status === 200) {
		// Server didn't echo a key — don't blank the displayed one.
		store().notifyError("Key refresh returned no key")
	}
}

export const activateLicense = async (key) => {
	if (!key) {
		store().notifyError("License key is required")
		return
	}
	const resp = await controller("/client/key/activate", { Key: key })
	if (resp.status === 200) {
		const user = { ...store().user, Key: { Key: "[shown on next login]" } }
		store().setUser(user)
		store().notifySuccess("License activated")
	}
}

// -------------------------------------------------------------- tunnels ---

export const createTunnel = async () => {
	const resp = await api("createTunnel")
	if (resp.status === 200) {
		useStore.setState((s) => ({ tunnels: [...s.tunnels, resp.data] }))
	}
}

export const saveTunnel = async (meta, oldTag) => {
	store().showLoading("Saving tunnel..")
	const resp = await api("setTunnel", { Meta: meta, OldTag: oldTag })
	store().hideLoading()
	if (resp.status === 200) {
		store().notifySuccess("Tunnel saved")
		await fetchState()
		return true
	}
	return false
}

// Replaces the tunnel's firewall policy (AllowedHosts + AllowAll). Unlike
// saveTunnel this works while the tunnel is connected — the client announces
// the new policy to the wg-server immediately.
export const setTunnelPeers = async (tag, allowedHosts, allowAll = false) => {
	const resp = await api("setTunnelPeers", { Tag: tag, AllowedHosts: allowedHosts, AllowAll: allowAll })
	if (resp.status === 200) {
		store().notifySuccess("Peer list updated")
		await fetchState()
		return true
	}
	return false
}

export const deleteTunnel = async (tunnel) => {
	store().showLoading("Deleting tunnel..")
	const resp = await api("deleteTunnel", tunnel)
	store().hideLoading()
	if (resp.status === 200) {
		useStore.setState((s) => ({ tunnels: s.tunnels.filter((t) => t.Tag !== tunnel.Tag) }))
		store().notifySuccess("Tunnel deleted")
	}
}

export const assignServerToTunnel = (tunnelTag, serverID) => {
	const tunnel = store().tunnels.find((t) => t.Tag.toLowerCase() === tunnelTag.toLowerCase())
	if (!tunnel) {
		store().notifyError("Tunnel not found")
		return
	}
	tunnel.ServerID = serverID
	return saveTunnel(tunnel, tunnel.Tag)
}

export const connect = async (tunnel, server) => {
	const user = store().user
	if (!user?.DeviceToken) {
		store().notifyError("You are not logged in")
		finalizeLogout() // clear state + route to login (don't leave a stale user)
		return
	}
	if (!server) server = store().servers.find((s) => s._id === tunnel?.ServerID)
	if (!server) {
		store().notifyError("Unable to find server with the given ID")
		return
	}

	store().showLoading("Connecting...")
	const resp = await api("connect", {
		UserID: user._id,
		DeviceToken: user.DeviceToken.DT,
		Tag: tunnel.Tag,
		EncType: tunnel.EncryptionType,
		ServerID: server._id,
		Server: user.ControlServer,
	})
	store().hideLoading()
	const ok = resp.status === 200 || resp.networkError
	if (ok) store().notifySuccess("Connection ready")
	await fetchState()
	return ok
}

// Connects to a specific server using the default tunnel. The client
// reconciles the default tunnel's device to this server before connecting,
// so it can safely move between servers. Use connect() instead when a tunnel
// is linked to its own server.
export const connectServer = async (server) => {
	const user = store().user
	if (!user?.DeviceToken) {
		store().notifyError("You are not logged in")
		finalizeLogout() // clear state + route to login (don't leave a stale user)
		return
	}
	if (!server) {
		store().notifyError("Unable to find server with the given ID")
		return
	}

	store().showLoading("Connecting...")
	const resp = await api("connectServer", {
		UserID: user._id,
		DeviceToken: user.DeviceToken.DT,
		ServerID: server._id,
		Server: user.ControlServer,
	})
	store().hideLoading()
	const ok = resp.status === 200 || resp.networkError
	if (ok) store().notifySuccess("Connection ready")
	await fetchState()
	return ok
}

export const disconnect = async (activeTunnel) => {
	store().showLoading("Disconnecting...")
	// Send Tag too: the live ID churns across auto-reconnects, so Tag lets the
	// client stop exactly this tunnel's reconnect loop / kill switch.
	const resp = await api("disconnect", { ID: activeTunnel.ID, Tag: activeTunnel.CR?.Tag }, { timeout: 20000 })
	store().hideLoading()
	if (resp.status === 200) store().notifySuccess("Disconnected from " + (activeTunnel.CR?.Tag || "tunnel"))
	await fetchState()
}

// -------------------------------------------------------------- servers ---

export const fetchServers = async () => {
	const resp = await controller("/client/servers", { StartIndex: 0 }, { silent: true })
	if (resp.status === 200) {
		store().setServers(resp.data?.length > 0 ? resp.data : [])
		if (!resp.data?.length) store().notifyError("Unable to find servers")
	} else if (resp.status !== 0) {
		store().setServers([])
		store().notifyError("Unable to find servers")
	}
}

// --------------------------------------------------------------- config ---

let configSaveInProgress = false
export const saveConfig = async (next) => {
	if (configSaveInProgress) {
		// Without feedback the second of two quick toggles is silently dropped and
		// the user thinks it saved. Tell them to retry after the in-flight save.
		store().notifyError("A config save is already in progress — please retry in a moment")
		return false
	}
	configSaveInProgress = true
	const config = next || store().config
	try {
		store().showLoading("Saving config..")
		const resp = await api("setConfig", config, { timeout: 120000 })
		if (resp.status === 200) {
			store().setConfig({ ...config })
			store().notifySuccess("Config saved")
			return true
		}
		return false
	} finally {
		configSaveInProgress = false
		store().hideLoading()
	}
}

export const toggleConfigKey = (key) => {
	const config = store().config
	return saveConfig({ ...config, [key]: !config[key] })
}

// ------------------------------------------------------------------ dns ---

export const fetchDnsStats = async () => {
	const resp = await api("getDNSStats", null, { silent: true })
	if (resp.status === 200) {
		session.setObject("dns-stats", resp.data)
		useStore.setState({ dnsStats: resp.data || {} })
	}
}

// Force-re-download all configured DNS block lists (bypasses the 24h cache).
// Downloads can take a while for large lists, so the timeout is generous.
export const updateBlockLists = async () => {
	store().showLoading("Updating block lists...")
	try {
		const resp = await api("updateBlockLists", null, { timeout: 300000 })
		if (resp.status === 200) {
			store().notifySuccess("Block lists updated")
			// Refresh config so domain counts / LastDownload show the new data.
			await fetchState()
			return true
		}
		return false
	} finally {
		store().hideLoading()
	}
}
