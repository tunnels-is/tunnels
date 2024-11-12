import { STATE } from "./state"
import STORE from "./store"


var WS = {
	sockets: {
		logs: undefined,
		state: undefined,
	},
	ReceiveLogEvent: (event) => {
		if (!STATE.logs) {
			STATE.logs = []
		}
		if (STATE.logs.length > 400) {
			STATE.logs.splice(0, 5000);
		}
		STATE.logs.push(event.data)
		STORE.Cache.SetObject("logs", STATE.logs)
		STATE.renderPage("loader")

	},
	GetURL: (route) => {
		// console.log("connecting socket...")
		let host = window.location.origin
		// let port = STORE.Cache.Get("api_port")
		// let ip = STORE.Cache.Get("api_ip")
		host = host.replace("http://", "wss://")
		host = host.replace("https://", "wss://")
		host = host.replace("5173", "7777")
		return host + "/" + route
	},
	NewSocket: (url, tag, messageHandler) => {
		if (WS.sockets[tag]) {
			return
		}

		let sock = undefined
		try {
			sock = new WebSocket(url);
			WS.sockets[tag] = sock
		} catch (error) {
			console.dir(error)
			setTimeout(() => {
				STATE.globalRerender()
			}, 2000)
			return
		}

		sock.onopen = (event) => {
			console.dir(event)
			console.log("WS:", event.type, url)
		}
		sock.onclose = (event) => {
			console.dir(event)
			if (!event.wasClean) {
				// connection not closed cleanly..
			}
			console.log("WS:", event.type, url)
			if (WS.sockets[tag]) {
				WS.sockets[tag].close()
				WS.sockets[tag] = undefined
			}
			setTimeout(() => {
				WS.NewSocket(url, tag, messageHandler)
			}, 1000)
		}
		sock.onerror = (event) => {
			console.dir(event)
			console.log("WS:", event.type, url)
			if (WS.sockets[tag]) {
				WS.sockets[tag].close()
				WS.sockets[tag] = undefined
			}
			setTimeout(() => {
				WS.NewSocket(url, tag, messageHandler)
			}, 1000)

		}
		sock.onmessage = (event) => {
			messageHandler(event)
		};

	},
}

export default WS
