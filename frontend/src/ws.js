import { getDefaultStore } from "jotai";
import { logsAtom } from "./stores/logStore";

const store = getDefaultStore();

var WS = {
  sockets: {
    logs: undefined,
    state: undefined,
  },
  ReceiveLogEvent: (event) => {
    let logs = store.get(logsAtom);
    if (!logs) {
      logs = [];
    }
    if (logs.length > 5000) {
      logs = [];
    }
    const newLogs = [...logs, event.data];
    store.set(logsAtom, newLogs);
  },
  GetURL: (route) => {
    let host = window.location.origin;
    host = host.replace("http://", "ws://");
    host = host.replace("https://", "ws://");
    host = host.replace("5173", "7777");
    return host + "/" + route;
  },
  NewSocket: (url, tag, messageHandler) => {
    if (WS.sockets[tag]) {
      return;
    }

    let sock = undefined;
    try {
      sock = new WebSocket(url);
      WS.sockets[tag] = sock;
    } catch (error) {
      console.dir(error);
      return;
    }

    sock.onopen = (event) => {
      console.log("WS:", event.type, url);
    };
    sock.onclose = (event) => {
      console.log("WS:", event.type, url);
      if (WS.sockets[tag]) {
        WS.sockets[tag] = undefined;
      }
      setTimeout(() => {
        WS.NewSocket(url, tag, messageHandler);
      }, 1000);
    };
    sock.onerror = (event) => {
      console.log("WS:", event.type, url);
      if (WS.sockets[tag]) {
        WS.sockets[tag].close();
        WS.sockets[tag] = undefined;
      }
      setTimeout(() => {
        WS.NewSocket(url, tag, messageHandler);
      }, 1000);
    };
    sock.onmessage = (event) => {
      messageHandler(event);
    };
  },
};

export const initWebSockets = () => {
  WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent);
};

export default WS;
