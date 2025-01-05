import { useState } from "react"; import STORE from "./store";
import toast from "react-hot-toast";
import axios from "axios"
import dayjs from "dayjs";
import { v4 as uuidv4 } from 'uuid';


const state = (page) => {
	const [value, reload] = useState({ x: 0 })
	const update = () => {
		let x = { ...value }
		x.x += 1
		reload({ ...x })
	}
	STATE.updates[page] = update
	STATE.update = update

	return STATE
}

export var STATE = {
	globalRerender: () => {
		if (STATE.debug) {
			console.log("GLOBAL RERENDER")
		}

		STATE.updates["root"]()
	},
	renderPage: (page) => {
		if (STATE.updates[page]) {
			if (STATE.debug) {
				console.log("RERENDER")
			}
			STATE.updates[page]()
		}
	},
	rerender: () => {
		if (STATE.debug) {
			console.log("RERENDER")
		}
		Object.keys(STATE.updates).forEach(k => {
			STATE.updates[k]()
		})
	},
	DNSListDateFormat: "ddd D. HH:mm:ss",
	TablePageSize: Number(STORE.Cache.Get("pageSize")),
	setPageSize: (id, count) => {
		let pg = STORE.Cache.GetObject("table_" + id)
		console.log("HIT!", id, pg)
		if (pg) {
			pg.TableSize = Number(count)
			STORE.Cache.SetObject("table_" + id, pg)
		}
		STATE.renderPage(id)
	},
	setPage: (id, page) => {
		let pg = STORE.Cache.GetObject("table_" + id)
		console.log("HIT!", id, pg)
		if (pg) {
			pg.CurrentPage = Number(page)
			STORE.Cache.SetObject("table_" + id, pg)
		}
		STATE.renderPage(id)
	},
	deleteBlocklist: (blocklist) => {
		let newLists = STATE.Config.AvailableBlockLists
		const index = newLists.indexOf(blocklist);

		if (index > -1) {
			newLists.splice(index, 1);
		}

		STATE.Config.AvailableBlockLists = newLists
		STATE.ConfigSave()
		STATE.renderPage("dns")
	},
	createConnection: async (new_conn) => {
		try {
			let resp = await STATE.API.method("createConnection", new_conn)
			if (resp.status != 200) {
				STATE.errorNotification("Cannot create new connection! Status Code: " + resp.status)
				return undefined
			} else {
				STATE.Config = resp.data
				STATE.rerender()
				return STATE.Config.Connections[STATE.Config.Connections.length - 1]
			}
		} catch (error) {
			console.dir(error)
		}
	},
	darkMode: STORE.Cache.GetBool("darkMode"),
	getDarkMode: () => {
		if (STATE.modifiedConfig?.DarkMode !== undefined) {
			return STATE.modifiedConfig.DarkMode
		}
		if (STATE.Config) {
			return STATE.Config.DarkMode
		} else {
			let dark = STORE.Cache.GetBool("darkMode")
			if (dark !== undefined) {
				return dark
			}
		}
		return true
	},
	toggleDarkMode: () => {
		STATE.darkMode = !STATE.darkMode
		STATE.toggleKeyAndReloadDom("Config", "DarkMode")
		if (STATE.debug) {
			console.log("Toggling Dark Mode")
			console.log("Dark Mode: " + STATE.darkMode)
		}
	},
	debug: STORE.Cache.GetBool("debug"),
	toggleDebug: () => {
		let debug = STORE.Cache.GetBool("debug")
		if (debug && debug === true) {
			debug = false
		} else {
			debug = true
		}
		STORE.Cache.Set("debug", debug)
		STATE.debug = debug
	},
	update: undefined,
	updates: {},
	logs: STORE.Cache.GetObject("logs"),
	fullRerender: () => {
		if (STATE.debug) {
			console.log("FULL RERENDER")
		}
		Object.keys(STATE.updates).forEach(k => {
			STATE.updates[k]()
		})
	},
	// SYSTEM SPECIFIC
	loading: undefined,
	toggleLoading: (object) => {
		if (object === undefined) {
			STATE.loading = undefined
			STATE.renderPage("loader")
			// return

			// Without timeout the "Saving Config..." msg
			// blinks on/off too fast imo it's better if it's about 1sec
			//
			// const to = setTimeout(() => {
			// 	STATE.loading = undefined
			// }, 1000)
			// return () => {
			// 	STATE.renderPage("loader")
			// 	clearTimeout(to)
			// }
		}
		if (object?.show) {
			STATE.loading = object
			STATE.renderPage("loader")

			const to = setTimeout(() => {
				STATE.loading = undefined
				STATE.renderPage("loader")
				clearTimeout(to)
			}, object.timeout ? object.timeout : 10000)

			return
		} else {
			STATE.loading = undefined
			return () => {
				STATE.renderPage("loader")
				clearTimeout(to)
			}
		}
	},
	toggleError: (e) => {
		let lastFetch = STORE.Cache.Get("error-timeout")
		let now = dayjs().unix()
		if ((now - lastFetch) < 3) {
			return
		}
		toast.error(e);
		STORE.Cache.Set("error-timeout", dayjs().unix())
	},
	errorNotification: (e) => {
		let lastFetch = STORE.Cache.Get("error-timeout")
		let now = dayjs().unix()
		if ((now - lastFetch) < 3) {
			return
		}
		toast.error(e);
		STORE.Cache.Set("error-timeout", dayjs().unix())
	},
	successNotification: (e) => {
		toast.success(e);
	},
	// UTILITY
	resetApp: async () => {

		try {

			await STATE.ConfirmAndExecute("", "resetAll", 15000, "This will disconnect Tunnels and reset everything", "Are you sure ?", async function() {

				STATE.toggleLoading({
					logTag: "reset",
					tag: "reset",
					show: true,
					msg: "Reseting Tunnels...",
					includeLogs: false,
				})
				await STATE.API.method("resetNetwork", {})
				STATE.toggleLoading(undefined)
			})
		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		STORE.Cache.Clear()
		window.close()
	},
	ModifiedNodeMap: new Map(),
	OpenNodes: new Map(),
	editorData: undefined,
	editorOriginal: undefined,
	editorReset: undefined,
	editorOnChange: undefined,
	editorSave: undefined,
	editorError: undefined,
	editorReadOnly: false,
	editorExtraButtons: [],
	editorRerender: 0,
	resetEditor: () => {
		STATE.editorData = undefined
		STATE.editorReset = undefined
		STATE.editorOnChange = function() { }
		STATE.editorSave = undefined
		STATE.editorError = undefined
		STATE.editorReadOnly = false
	},
	OpenEditors: new Map(),
	User: STORE.Cache.GetObject("user"),
	modifiedUser: STORE.Cache.GetObject("modifiedUser"),
	Config: STORE.Cache.GetObject("config"),
	modifiedConfig: STORE.Cache.GetObject("modifiedConfig"),
	SetConfigModifiedState: (state) => {
		STORE.Cache.Set("configIsModified", state)
	},
	IsConfigModified: () => {
		return STORE.Cache.GetBool("configIsModified")
	},
	UserSaveModifiedSate: () => {
		STORE.Cache.SetObject("modifiedUser", STATE.modifiedUser)
	},
	refreshApiKey: () => {
		STATE.createObjectIfEmpty("modifiedUser")
		STATE.modifiedUser.APIKey = uuidv4()
		STATE.UserSaveModifiedSate()
		STATE.rerender()
	},
	changeServerOnConnection: (tag, index) => {
		let cons = [...STATE.Config?.Connections]
		let found = false

		cons.forEach((c, i) => {
			if (c.Tag.toLowerCase() === tag.toLowerCase()) {
				cons[i].ServerID = index
				found = true
				return
			}
		})

		if (found) {
			STATE.SaveConnectionsToModifiedConfig(cons)
			STATE.ConfigSave()
		}
	},
	ConfigSave: async () => {

		STATE.createObjectIfEmpty("modifiedConfig")
		if (!STATE.modifiedConfig.Connections) {
			STATE.modifiedConfig.Connections = []
		}

		STATE.Config.Connections.forEach(cc => {
			let found = false
			STATE.modifiedConfig.Connections.forEach(mc => {
				if (mc.WindowsGUID == cc.WindowsGUID) {
					found = true
				}
			})
			if (!found) {
				STATE.modifiedConfig.Connections.push(cc)
			}
		});
		if (STATE.modifiedLists) {
			STATE.modifiedLists.forEach(l => {
				if (l.Enabled) {
					STATE.Config?.AvailableBlockLists.forEach(al => {
						if (al.Tag === l.Tag) {
							al.Enabled = l.Enabled
						}
					})
				}
			})
		}

		let newConfig = {
			...STATE.Config,
			...STATE.modifiedConfig
		}
		// console.log("SAVE CONFIG")
		// console.dir(STATE.Config)
		// console.dir(STATE.modifiedConfig)
		// console.dir(newConfig)

		let success = false
		try {
			STATE.toggleLoading({
				tag: "config",
				show: true,
				msg: "Saving config..",
			})

			let resp = await STATE.API.method("setConfig", newConfig)
			if (resp === undefined) {
				STATE.errorNotification("Unknown error, please try again in a moment")
			} else if (resp.status === 200) {
				success = true
				STORE.Cache.SetObject("config", newConfig)
				STORE.Cache.Set("darkMode", newConfig.DarkMode)
				STATE.Config = newConfig
				STATE.RemoveModifiedConfig()
				STATE.RemoveModifiedLists()
				STATE.successNotification("Config saved", undefined)
				STATE.SetConfigModifiedState(false)
			}
		} catch (error) {
			console.dir(error)
		}
		STATE.toggleLoading(undefined)
		STATE.globalRerender()
		return success
	},
	RemoveModifiedLists: () => {
		STATE.modifiedLists = undefined
		STATE.SaveModifiedLists()
	},
	RemoveModifiedConfig: () => {
		STATE.modifiedConfig = undefined
		STATE.modifiedLists = undefined
		STATE.ConfigSaveModifiedSate()
		STATE.SetConfigModifiedState(false)
		STATE.globalRerender()
	},
	SaveModifiedLists: () => {
		STORE.Cache.SetObject("modifiedLists", STATE.modifiedLists)
	},
	ConfigSaveModifiedSate: () => {
		STORE.Cache.SetObject("modifiedConfig", STATE.modifiedConfig)
	},
	ConfigSaveOriginalState: () => {
		STORE.Cache.SetObject("config", STATE.Config)
	},
	SaveConnectionsToModifiedConfig: (connections) => {
		STATE.createObjectIfEmpty("modifiedConfig")
		STATE.modifiedConfig.Connections = connections
		STATE.ConfigSaveModifiedSate()
	},
	GetModifiedConnections: () => {
		let cons = STATE.modifiedConfig?.Connections
		if (!cons) {
			return []
		}
		return cons
	},
	DeleteConnection: async (id) => {
		STATE.Config?.Connections.forEach((c, index) => {
			if (c.WindowsGUID === id) {
				// console.log("SPLICING:", c.WindowsGUID)
				STATE.Config?.Connections.splice(index, 1)
			} else if (c.WindowsGUID === "") {
				STATE.Config?.Connections.splice(index, 1)
			}
		})
		STATE.modifiedConfig?.Connections.forEach((c, index) => {
			if (c.WindowsGUID === id) {
				// console.log("SPLICING:", c.WindowsGUID)
				STATE.modifiedConfig?.Connections.splice(index, 1)
			} else if (c.WindowsGUID === "") {
				STATE.modifiedConfig?.Connections.splice(index, 1)
			}
		})
		STATE.ConfigSave()
		STATE.globalRerender()
	},
	createObjectIfEmpty: (type) => {
		if (STATE[type] === undefined) {
			STATE[type] = {}
		}
	},
	createArrayIfEmpty: (type) => {
		if (STATE[type] === undefined) {
			STATE[type] = []
		}
	},
	modifiedLists: STORE.Cache.GetObject("modifiedLists"),
	toggleBlocklist: (list) => {
		list.Enabled = !list.Enabled
		if (!STATE.modifiedLists) {
			// console.log(list.Enabled)
			// list.Enabled = !list.Enabled
			// console.log(list.Enabled)
			let x = [list]
			STORE.Cache.SetObject("modifiedLists", x)
			STATE.modifiedLists = x
			// STATE.renderPage("dns")
			STATE.globalRerender()
			return
		}

		let found = false
		STATE.modifiedLists.forEach((l, i) => {
			if (l.Tag === list.Tag) {
				STATE.modifiedLists[i] = list
				found = true
			}
		})
		if (!found) {
			STATE.modifiedLists.push(list);
		}

		STORE.Cache.SetObject("modifiedLists", STATE.modifiedLists)
		STATE.renderPage("dns")
		// STATE.globalRerender()
	},
	getKey: (type, key) => {
		// console.log("GET KEY")
		try {
			let rv = STATE["modified" + type]
			if (rv[key] !== undefined) {
				// console.log("get mod:", rv)
				return String(rv[key])
			}
		} catch (error) {
			// console.dir(error)
		}
		try {
			// console.log("get:", STATE[type][key])
			return String(STATE[type][key])
		} catch (error) {
			console.dir(error)
		}
	},
	toggleKeyAndReloadDom: (type, key) => {
		try {
			let modObj = STATE["modified" + type]
			if (modObj !== undefined) {
				let modKey = modObj[key]
				if (modKey !== undefined) {
					// console.log("flip on modified")
					STATE["modified" + type][key] = !STATE["modified" + type][key]
				} else {
					// console.log("flip on original")
					STATE["modified" + type][key] = !STATE[type][key]
				}
			} else {
				// console.log("create modified")
				STATE["modified" + type] = {}
				STATE["modified" + type][key] = !STATE[type][key]
			}
			STORE.Cache.SetObject("modified" + type, STATE["modified" + type])
			// STATE[type + "Save"]()
			STATE.rerender()
			return
		} catch (error) {
			console.dir(error)
		}
	},
	setArrayAndReloadDom: (type, key, value) => {
		value = value.split(",")
		STATE.setKeyAndReloadDom(type, key, value)
	},
	setKeyAndReloadDom: (type, key, value) => {
		try {
			let modObj = STATE["modified" + type]
			if (modObj !== undefined) {
				STATE["modified" + type][key] = value
			} else {
				// console.log("create modified")
				STATE["modified" + type] = {}
				STATE["modified" + type][key] = value
			}
			STORE.Cache.SetObject("modified" + type, STATE["modified" + type])
			// STATE[type + "Save"]()
			STATE.rerender()
			return
		} catch (error) {
			console.dir(error)
		}
	},
	ConfirmAndExecute: async (type, id, duration, title, subtitle, method) => {
		if (type === "") {
			type = "success"
		}
		await toast[type]((t) => (
			<div className="content">
				{title &&
					<div className="title">
						{title}
					</div>
				}
				<div className="subtitle">
					{subtitle}
				</div>
				<div className="buttons">
					<div className="button no" onClick={() => toast.dismiss(t.id)}>NO</div>
					<div className="button yes" onClick={async function() {
						toast.dismiss(t.id)
						await method()
					}
					}>YES</div>

				</div>
			</div >

		), { id: id, duration: duration })
	},
	Logout: async () => {

		try {
			await STATE.disconnectAllConnections()
		} catch (error) {
			console.dir(error)
		}

		let theme = STATE.darkMode
		STORE.Cache.Clear()
		STORE.Cache.Set("darkMode", theme)
		window.location.replace("/#/login")
		window.location.reload()
	},
	// API
	// API
	// API
	// API
	UpdateUser: async () => {
		try {

			STATE.toggleLoading({
				logTag: "",
				tag: "USER-UPDATE",
				show: true,
				msg: "Updating User Settings",
				includeLogs: false
			})

			let FORM = {
				Email: STATE.User.Email,
				DeviceToken: STATE.User.DeviceToken.DT,
				APIKey: STATE.modifiedUser.APIKey
			}
			let newUser = {
				...STATE.User,
				...STATE.modifiedUser
			}

			let FR = {
				Path: "v3/user/update",
				Method: "POST",
				JSONData: FORM,
				Timeout: 10000
			}

			let x = await STATE.API.method("forwardToController", FR)
			if (x && x.status === 200) {
				STORE.Cache.SetObject("user", newUser)
				STORE.Cache.DelObject("modifiedUser")
				STORE.User = newUser
				STORE.modifiedUser = undefined
				STATE.showSuccessToast("User updated", undefined)
			} else {
				STATE.toggleError(x)
			}

		} catch (e) {
			console.dir(e)
		}

		STATE.toggleLoading(undefined)
	},
	connectToVPN: async (c, server) => {
		if (server) {
			STATE.Config?.Connections?.forEach(con => {
				if (con.Tag === "tunnels") {
					c = con
					return
				}
			})
		}

		if (!c) {
			STATE.errorNotification("no connection selected")
			return
		}

		try {
			STATE.toggleLoading({
				logTag: "connect",
				tag: "CONNECT",
				show: true,
				msg: "Connecting...",
				includeLogs: true
			})

			let user = STORE.GetUser()
			if (!user.DeviceToken) {
				STATE.errorNotification("You are not logged in")
				STORE.Cache.Clear()
				return
			}

			let method = "connect"
			let connectionRequest = {}
			connectionRequest.UserID = user._id
			connectionRequest.DeviceToken = user.DeviceToken.DT
			connectionRequest.Tag = c.Tag
			connectionRequest.ServerID = c.ServerID
			connectionRequest.EncType = c.EncryptionType
			if (server) {
				connectionRequest.ServerID = server._id
				connectionRequest.ServerIP = server.IP
				connectionRequest.ServerPort = server.Port
			} else if (c.Private === true) {
				connectionRequest.ServerIP = c.PrivateIP
				connectionRequest.ServerPort = c.PrivatePort
			} else {
				STATE.Servers?.forEach(s => {
					if (s._id === c.ServerID) {
						connectionRequest.ServerIP = s.IP
						connectionRequest.ServerPort = s.Port
					}
				})
			}

			let resp = await STATE.API.method(method, connectionRequest)
			if (resp === undefined) {
				STATE.errorNotification("Unknown error, please try again in a moment")
			} else {
				if (resp.status === 401) {
					STATE.successNotification("Unauthorized, logging you out!", undefined)
					LogOut()
				} else if (resp.status === 200) {
					STATE.successNotification("connection ready")
				}
			}
		} catch (error) {
			console.dir(error)
		}
		STATE.toggleLoading(undefined)
		STATE.GetBackendState()
	},
	disconnectAllConnections: async () => {
		STATE.State.ActiveConnections.forEach(c => {
			try {
				STATE.disconnectFromVPN(c)
			} catch (error) {
				console.dir(error)
			}
		})
		STATE.GetBackendState()
	},
	disconnectFromVPN: async (c) => {

		try {
			let x = await STATE.API.method("disconnect", { GUID: c.WindowsGUID }, false, 5000)
			if (x === undefined) {
				STATE.errorNotification("Unknown error, please try again in a moment")
			} else {
				STATE.successNotification("Disconnected", {
					Title: "DISCONNECTED", Body: "You have been disconnected from " + c.Tag, TimeoutType: "default"
				})
				STORE.CleanupOnDisconnect()
			}
		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		STATE.GetBackendState()
	},
	LogoutAllTokens: async () => {
		let user = STORE.Cache.GetObject("user")
		if (!user) {
			return
		}


		let LF = {
			DeviceToken: user.DeviceToken.DT,
			Email: user.Email,
		}

		let FR = {
			Path: "v3/user/logout/all",
			Method: "POST",
			JSONData: LF,
			Timeout: 20000
		}

		let x = await STATE.API.method("forwardToController", FR)
		if (x && x.status === 200) {
			STATE.successNotification(
				"device logged out",
				undefined,
			)

			STATE.Logout()

		} else {
			STATE.errorNotification(
				"Unable to log out device",
				undefined,
			)
		}
	},
	LogoutToken: async (token) => {
		let user = STORE.Cache.GetObject("user")
		if (!user) {
			return
		}

		let LF = {
			DeviceToken: token.DT,
			Email: user.Email,
		}

		let FR = {
			Path: "v3/user/logout",
			Method: "POST",
			JSONData: LF,
			Timeout: 20000
		}
		let x = await STATE.API.method("forwardToController", FR)

		if (x && x.status === 200) {
			STATE.successNotification(
				"device logged out",
				undefined,
			)
			let u = STORE.Cache.GetObject("user")
			if (u?.DeviceToken.DT === token.DT) {
				STATE.Logout()
				return
			}

			let toks = []
			u?.Tokens?.map(t => {
				if (t.DT !== token.DT) {
					toks.push(t)
				}
			})

			u.Tokens = toks
			STORE.Cache.SetObject("user", u)
			STATE.User = u
			STATE.rerender()

		} else {
			STATE.errorNotification(
				"Unable to log out device",
				undefined,
			)
		}
	},

	ForwardToController: async (req, loader) => {
		STATE.toggleLoading(loader)
		let x = await STATE.API.method("forwardToController", req)
		STATE.toggleLoading(undefined)
		return x
	},

	LicenseKey: "",
	UpdateLicenseInput: (value) => {
		STATE.LicenseKey = value
		STATE.rerender()
	},
	GetPrivateServersInProgress: false,
	Servers: STORE.Cache.GetObject("servers"),
	PrivateServers: STORE.Cache.GetObject("private-servers"),
	updatePrivateServers: () => {
		STORE.Cache.SetObject("private-servers", STATE.PrivateServers)
	},
	ModifiedServers: [],
	UpdateModifiedServer: function(server, key, value) {
		let found = false
		STATE.ModifiedServers.forEach((s, i) => {
			if (s._id === server._id) {
				STATE.ModifiedServers[i][key] = value
				found = true
				return
			}
		})
		if (!found) {
			server[key] = value
			STATE.ModifiedServers.push(server)
		}
		STATE.renderPage("inspect-server")
	},
	API_UpdateServer: async (id) => {
		let server = undefined
		STATE.ModifiedServers?.forEach(s => {
			if (s._id === id) {
				server = s
			}
		})

		if (!server) {
			return
		}

		let resp = undefined
		try {

			let FR = {
				Path: "v3/servers/update",
				Method: "POST",
				Timeout: 10000,
			}
			FR.JSONData = {
				UID: STATE.User._id,
				DeviceToken: STATE.User.DeviceToken.DT,
				Server: server
			}


			STATE.toggleLoading({
				tag: "SERVER_UPDATE",
				show: true,
				msg: "Updating your server ..."
			})

			resp = await STATE.API.method("forwardToController", FR)
			if (resp?.status === 200) {

				STATE.ModifiedServers?.forEach((s, i) => {
					if (s._id === id) {
						STATE.ModifiedServers.splice(i, 1)
					}
				})

				STATE.PrivateServers.forEach((s, i) => {
					if (s._id === id) {
						STATE.PrivateServers[i] = server
					}
				})
			}
		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		return resp
	},
	API_CreateServer: async (server) => {

		let resp = undefined
		try {

			let FR = {
				Path: "v3/servers/create",
				Method: "POST",
				Timeout: 10000,
			}
			FR.JSONData = {
				UID: STATE.User._id,
				DeviceToken: STATE.User.DeviceToken.DT,
				Server: server
			}


			STATE.toggleLoading({
				tag: "SERVER_CREATE",
				show: true,
				msg: "Creating your server ..."
			})

			resp = await STATE.API.method("forwardToController", FR)
			if (resp?.status === 200) {
				if (!STATE.PrivateServers) {
					STATE.PrivateServers = []
				}
				STATE.PrivateServers.push(resp.data)
				STATE.updatePrivateServers()
			}

		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		return resp
	},
	GetPrivateServers: async () => {
		if (!STATE.User) {
			return
		}

		if (STATE.GetPrivateServersInProgress) {
			return
		}
		STATE.GetPrivateServersInProgress = true


		let resp = undefined
		try {

			let timeout = STORE.Cache.GetObject("private-servers_ct")
			let now = dayjs().unix()
			let diff = now - timeout
			if (now - timeout > 30 || !timeout) {
			} else {
				STATE.GetPrivateServersInProgress = false
				STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds")
				return
			}

			resp = await STATE.ForwardToController(
				{
					Path: "v3/servers/private",
					Method: "POST",
					JSONData: {
						DeviceToken: STATE.User.DeviceToken.DT,
						UID: STATE.User._id,
						StartIndex: 0,
					},
					Timeout: 10000,
				},
				{
					show: true,
					tag: "server-search",
					msg: "Searching for servers .."
				}
			)
		} catch (error) {
			console.dir(error)
		}


		if (resp?.status === 200) {
			STORE.Cache.SetObject("private-servers", resp.data)
			STATE.PrivateServers = resp.data
			STATE.renderPage("pservers")
		} else {
			STATE.errorNotification("Unable to find servers")
		}

		STATE.GetPrivateServersInProgress = false
	},
	GetServers: async () => {
		if (!STATE.User) {
			return
		}

		if (STATE.GetPrivateServersInProgress) {
			return
		}
		STATE.GetPrivateServersInProgress = true


		let resp = undefined
		try {

			let timeout = STORE.Cache.GetObject("servers_ct")
			let now = dayjs().unix()
			let diff = now - timeout
			if (now - timeout > 30 || !timeout) {
			} else {
				STATE.GetPrivateServersInProgress = false
				STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds")
				return
			}

			resp = await STATE.ForwardToController(
				{
					Path: "v3/servers",
					Method: "POST",
					JSONData: {
						DeviceToken: STATE.User.DeviceToken.DT,
						UID: STATE.User._id,
						StartIndex: 0,
					},
					Timeout: 10000,
				},
				{
					show: true,
					tag: "server-search",
					msg: "Searching for servers .."
				}
			)

		} catch (error) {
			console.dir(error)
		}


		if (resp?.status === 200) {
			STORE.Cache.SetObject("servers", resp.data)
			STATE.Servers = resp.data
			STATE.renderPage("servers")
		} else {
			STATE.errorNotification("Unable to find servers")
		}

		STATE.GetPrivateServersInProgress = false
	},
	ActivateLicense: async () => {

		if (!STATE.User) {
			return
		}

		if (STATE.LicenseKey === "") {
			STATE.errorNotification("License key is required")
			return
		}

		STATE.ForwardToController(
			{
				Path: "v3/key/activate",
				Method: "POST",
				JSONData: {
					Key: STATE.LicenseKey,
					Email: STATE.User.Email
				},
				Timeout: 20000
			},
			{
				tag: "key-activation",
				show: true,
				msg: "activating your license key..."
			})

		STATE.User.Key = {
			Key: "[shown on next login]"
		}

		STORE.Cache.SetObject("user", { ...STATE.User })
		STATE.rerender()
	},
	GetResetCode: async (inputs) => {
		return STATE.ForwardToController(
			{
				Path: "v3/user/reset/code",
				Method: "POST",
				JSONData: inputs,
				Timeout: 20000
			},
			{
				tag: "reset-code",
				show: true,
				msg: "sending you a reset code ..."
			})
	},
	GetQRCode: async (inputs) => {
		return STATE.API.method("getQRCode", inputs)
	},
	ConfirmTwoFactorCode: async (inputs) => {
		return STATE.ForwardToController(
			{
				Path: "v3/user/2fa/confirm",
				Method: "POST",
				JSONData: inputs,
				Timeout: 20000
			},
			{
				tag: "two-factor",
				show: true,
				msg: "confirming two-factor authentication .."
			})
	},
	API_EnableAccount: async (inputs) => {
		return STATE.ForwardToController(
			{
				Path: "v3/user/enable",
				Method: "POST",
				JSONData: inputs,
				Timeout: 20000
			},
			{
				tag: "enable-account",
				show: true,
				msg: "enabling your account .."
			})
	},
	ResetPassword: async (inputs) => {
		return STATE.ForwardToController(
			{
				Path: "v3/user/reset/password",
				Method: "POST",
				JSONData: inputs,
				Timeout: 20000
			},
			{
				tag: "reset-password",
				show: true,
				msg: "resetting your password .."
			})
	},
	Register: async (inputs) => {
		return STATE.ForwardToController(
			{
				Path: "v3/user/create",
				Method: "POST",
				JSONData: inputs,
				Timeout: 20000
			},
			{
				tag: "register",
				show: true,
				msg: "creating your account .."
			})
	},
	login: async (inputs) => {
		STATE.toggleLoading({
			tag: "login",
			show: true,
			msg: "logging you in..."
		})

		let token = STORE.Cache.Get(inputs["email"] + "_" + "TOKEN")

		if (token !== null) {
			inputs.DeviceToken = token
		}

		let FR = {
			Path: "v3/user/login",
			Method: "POST",
			Timeout: 20000,
			JSONData: inputs
		}

		STORE.Cache.Set("default-device-name", inputs["devicename"])
		STORE.Cache.Set("default-email", inputs["email"])

		let x = await STATE.API.method(
			"forwardToController",
			FR,
			true
		)

		console.log("DONE LOGIN")
		console.dir(x)
		if (x?.status === 200) {
			STORE.Cache.Set(inputs["email"] + "_" + "TOKEN", x.data.DeviceToken.DT)

			STATE.User = x.data
			STORE.Cache.SetObject("user", x.data);
			STATE.toggleLoading(undefined)
			window.location.replace("/")
		}

		STATE.toggleLoading(undefined)
	},
	Org: STORE.Cache.GetObject("org"),
	updateOrg: (org) => {
		if (org) {
			STORE.Cache.SetObject("org", org)
			STATE.Org = org
			STATE.rerender()
		}
	},
	GetOrgInProgress: false,
	API_GetOrg: async () => {
		if (!STATE.User) {
			return
		}

		if (STATE.GetOrgInProgress) {
			return
		}
		STATE.GetOrgInProgress = true

		try {

			let timeout = STORE.Cache.GetObject("org_ct")
			let now = dayjs().unix()
			let diff = now - timeout
			if (now - timeout > 30 || !timeout) {
			} else {
				STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds")
				return
			}


			let FR = {
				Path: "v3/org",
				Method: "POST",
				Timeout: 10000,
			}
			if (STATE.User) {
				FR.JSONData = {
					UID: STATE.User._id,
					DeviceToken: STATE.User.DeviceToken.DT,
				}
			} else {
				return undefined
			}


			STATE.toggleLoading({
				tag: "ORG_FETCH",
				show: true,
				msg: "Fetching Your Organization Information ..."
			})

			let resp = await STATE.API.method("forwardToController", FR)
			if (resp?.status === 200) {
				STATE.updateOrg(resp.data)
				STATE.Groups = resp.data.Groups
			}

		} catch (error) {
			console.dir(error)
		}

		STATE.GetOrgInProgress = false
		STATE.toggleLoading(undefined)
	},
	UpdateGroup: (group) => {
		if (STATE.Org) {
			STATE.Org.Groups?.forEach((g, i) => {
				if (g._id === group._id) {
					STATE.Org.Groups[i] = group
					STATE.updateOrg(STATE.Org)
				}
			});
		}
	},
	API_UpdateGroup: async (group) => {

		try {

			let FR = {
				Path: "v3/group/update",
				Method: "POST",
				Timeout: 10000,
			}
			if (STATE.User) {
				FR.JSONData = {
					UID: STATE.User._id,
					DeviceToken: STATE.User.DeviceToken.DT,
					Group: group
				}
			} else {
				return undefined
			}


			STATE.toggleLoading({
				tag: "GROUP_UPDATE",
				show: true,
				msg: "updating ..."
			})

			let resp = await STATE.API.method("forwardToController", FR)
			if (resp && resp.status === 200) {
				STATE.UpdateGroup(group)
			}
		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		return
	},
	API_CreateGroup: async (group) => {

		let resp = undefined
		try {

			let FR = {
				Path: "v3/group/create",
				Method: "POST",
				Timeout: 10000,
			}
			if (STATE.User) {
				FR.JSONData = {
					UID: STATE.User._id,
					DeviceToken: STATE.User.DeviceToken.DT,
					Group: group
				}
			} else {
				return undefined
			}

			STATE.toggleLoading({
				tag: "ORG_FETCH",
				show: true,
				msg: "searching ..."
			})

			resp = await STATE.API.method("forwardToController", FR)
			if (resp?.status === 200) {
				STATE.UpdateGroup(group)
			}
		} catch (error) {
			console.dir(error)
		}

		STATE.toggleLoading(undefined)
		return resp
	},
	State: STORE.Cache.GetObject("state"),
	StateFetchInProgress: false,
	GetURL: () => {
		let host = window.location.origin
		// let port = STORE.Cache.Get("api_port")
		// let ip = STORE.Cache.Get("api_ip")
		host = host.replace("http://", "https://")
		host = host.replace("5173", "7777")
		return host

	},
	GetBackendState: async () => {
		if (STATE.debug) {
			console.log("STATE UPDATE !")
		}
		if (STATE.StateFetchInProgress) {
			return
		}
		STATE.StateFetchInProgress = true

		try {
			let response = undefined
			let host = STATE.GetURL()

			response = await axios.post(
				host + "/v1/method/getState",
				{},
				{ headers: { "Content-Type": "application/json" } }
			)

			if (response.status === 200) {
				STATE.State = response.data
				STATE.Config = response.data.C
				STATE.User = response.data.User
				STATE.darkMode = response.data.DarkMode

				STORE.Cache.SetObject("state", STATE.State)
				STORE.Cache.SetObject("config", STATE.State.C)
				STORE.Cache.Set("darkMode", STATE.Config.DarkMode)
				STORE.Cache.SetObject("user", STATE.State.User)
				STATE.globalRerender()
			}
			STATE.StateFetchInProgress = false


			return response
		} catch (error) {
			STATE.StateFetchInProgress = false
			console.dir(error)
			STATE.errorNotification("unable to load state...")
			return undefined
		}
	},
	API: {
		async method(method, data, noLogout, timeout) {
			try {
				let response = undefined
				let host = STATE.GetURL()

				let to = 30000
				if (timeout) {
					to = timeout
				}

				let body = undefined
				if (data) {
					try {
						body = JSON.stringify(data)
					} catch (error) {
						console.dir(error)
						return
					}
				}

				response = await axios.post(
					host + "/v1/method/" + method,
					body,
					{
						timeout: to,
						headers: { "Content-Type": "application/json" }
					}
				)

				if (response.data?.Message) {
					STATE.successNotification(response?.data?.Message)
				} else if (response.data?.Error) {
					STATE.errorNotification(response?.data?.Error)
				}


				return response
			} catch (error) {
				console.dir(error)


				if (!noLogout || noLogout === false) {
					if (error?.response?.status === 401) {
						STATE.Logout()
						return
					}
				}

				if (error?.response?.data?.Message) {
					STATE.errorNotification(error?.response?.data?.Message)
				} else if (error?.response?.data?.Error) {
					STATE.errorNotification(error?.response?.data?.Error)
				} else if (error?.response?.data?.error) {
					STATE.errorNotification(error?.response?.data.error)
				} else {
					if (typeof error?.response?.data === "string") {
						STATE.errorNotification(error?.response?.data)
					} else {
						STATE.errorNotification("Unknown error")
					}
				}

				return error?.response

			}
		},
	},
	// 299,792,458
	updateNodes: (nodes) => {
		if (nodes && nodes.length > 0) {
			STORE.Cache.SetObject("nodes", nodes)
			STATE.Servers = nodes
			STATE.rerender()
		} else {
			STORE.Cache.SetObject("nodes", [])
			STATE.Servers = []
			STATE.rerender()
		}
	},
	ModifiedNodes: STORE.Cache.GetObject("modifiedNodes"),
	SaveModifiedNodes: () => {
		STORE.Cache.SetObject("modifiedNodes", STATE.ModifiedNodes)
	},
};

export default state
