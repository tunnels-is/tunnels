// Live log stream from the backend. One websocket, reconnects after 1s.

import { wsURL } from "./client"
import { useStore } from "@/store/store"

let socket

export const connectLogSocket = () => {
	if (socket) return
	try {
		socket = new WebSocket(wsURL("logs"))
	} catch {
		socket = undefined
		setTimeout(connectLogSocket, 1000)
		return
	}
	socket.onmessage = (event) => useStore.getState().appendLog(event.data)
	socket.onclose = reconnect
	socket.onerror = reconnect
}

const reconnect = () => {
	socket?.close()
	socket = undefined
	setTimeout(connectLogSocket, 1000)
}
