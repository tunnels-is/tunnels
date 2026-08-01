import { callController, callMethod, errorMessage } from "@/api/client"
import { session } from "./session"
import { useStore } from "./store"

const store = () => useStore.getState()

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

		const hash = window.location.hash
		const onAuthRoute = hash.startsWith("#/login") || hash.startsWith("#/accounts")

		const onNoRoute = hash === "" || hash === "#" || hash === "#/"
		if (!store().user && !onAuthRoute) {
			const users = await loadUsers()
			const activeID = session.get("activeUserID")
			const match = activeID && users?.find((u) => u._id === activeID)
			if (match) {
				store().setUser(match)
				await api("setUser", match, { logout: false, silent: true })
				if (onNoRoute) window.location.hash = "#/dashboard"
			} else if (users?.length === 1) {
				store().setUser(users[0])
				await api("setUser", users[0], { logout: false, silent: true })
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

export const loadUsers = async () => {
	const resp = await api("getUsers", null, { logout: false, silent: true })

	return resp.status === 200 && Array.isArray(resp.data) ? resp.data : []
}

export const fetchUsers = async () => {
	const users = await loadUsers()
	if (users?.length > 0) useStore.setState({ users })
	return users
}

export const loginUser = async (user, remember, server) => {
	clearAccountScopedCache()
	user.ControlServer = server
	store().setUser(user)
	if (remember) await saveUserToDisk(user)
}

export const switchAccount = async (user) => {
	clearAccountScopedCache()
	store().setUser(user)
	// Persist + activate account workspace on the local daemon (tunnels/devices paths).
	await api("setUser", user, { logout: false, silent: true })
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
	clearAccountScopedCache()
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

	const resp = await controller("/client/user/update", { APIKey: "generate" })
	if (resp.status === 200 && resp.data?.APIKey) {
		store().setUser({ ...store().user, APIKey: resp.data.APIKey })
		store().notifySuccess("User updated")
	} else if (resp.status === 200) {

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
		finalizeLogout()
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

export const connectServer = async (server) => {
	const user = store().user
	if (!user?.DeviceToken) {
		store().notifyError("You are not logged in")
		finalizeLogout()
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

	const resp = await api("disconnect", { ID: activeTunnel.ID, Tag: activeTunnel.CR?.Tag }, { timeout: 20000 })
	store().hideLoading()
	if (resp.status === 200) store().notifySuccess("Disconnected from " + (activeTunnel.CR?.Tag || "tunnel"))
	await fetchState()
}

const SERVERS_TTL_MS = 30_000
let serversFetchedAt = 0
let serversInFlight = null
// Which user id the current devices[] list was fetched for (null = empty/unknown).
let devicesFetchedForUser = null
let devicesInFlight = null

// Drop controller-side lists that belong to a previous account so a
// switch/login cannot briefly show the wrong servers or devices.
export const clearAccountScopedCache = () => {
	serversFetchedAt = 0
	serversInFlight = null
	devicesFetchedForUser = null
	devicesInFlight = null
	session.setObject("servers", [])
	useStore.setState({ servers: [], devices: [], localDevices: [] })
}

/** Fetch /client/servers with a 30s TTL cache and in-flight dedupe. Pass { force: true } to bypass. */
export const fetchServers = async ({ force = false } = {}) => {
	if (!force && Date.now() - serversFetchedAt < SERVERS_TTL_MS) {
		return store().servers
	}
	if (serversInFlight) return serversInFlight

	serversInFlight = (async () => {
		try {
			const resp = await controller("/client/servers", { StartIndex: 0 }, { silent: true })
			if (resp.status === 200) {
				store().setServers(resp.data?.length > 0 ? resp.data : [])
				serversFetchedAt = Date.now()
				if (!resp.data?.length) store().notifyError("Unable to find servers")
			} else if (resp.status !== 0) {
				store().setServers([])
				store().notifyError("Unable to find servers")
			}
			return store().servers
		} finally {
			serversInFlight = null
		}
	})()

	return serversInFlight
}

/** Fetch controller devices + local devices for the active account. */
export const fetchDevices = async ({ force = false } = {}) => {
	const userID = store().user?._id || ""
	if (!userID) {
		store().setDevices([])
		store().setLocalDevices([])
		devicesFetchedForUser = null
		return []
	}
	if (!force && devicesFetchedForUser === userID) {
		return store().devices
	}
	if (devicesInFlight) return devicesInFlight

	devicesInFlight = (async () => {
		try {
			const [remote, local] = await Promise.all([
				controller("/client/device/list/user", {}, { silent: true }),
				api("getLocalDevices", { UserID: userID }, { silent: true, logout: false }),
			])
			if (remote.status === 200 && Array.isArray(remote.data)) {
				store().setDevices(remote.data)
			} else if (remote.status !== 0) {
				store().setDevices([])
			}
			if (local.status === 200 && Array.isArray(local.data)) {
				store().setLocalDevices(local.data)
			} else {
				store().setLocalDevices([])
			}
			devicesFetchedForUser = userID
			return store().devices
		} finally {
			devicesInFlight = null
		}
	})()

	return devicesInFlight
}

let configSaveInProgress = false
export const saveConfig = async (next) => {
	if (configSaveInProgress) {

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

export const fetchDnsStats = async () => {
	const resp = await api("getDNSStats", null, { silent: true })
	if (resp.status === 200) {
		session.setObject("dns-stats", resp.data)
		useStore.setState({ dnsStats: resp.data || {} })
	}
}

export const updateBlockLists = async () => {
	store().showLoading("Updating block lists...")
	try {
		const resp = await api("updateBlockLists", null, { timeout: 300000 })
		if (resp.status === 200) {
			store().notifySuccess("Block lists updated")

			await fetchState()
			return true
		}
		return false
	} finally {
		store().hideLoading()
	}
}
