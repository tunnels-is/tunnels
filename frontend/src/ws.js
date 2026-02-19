import { STATE } from "./state";
import STORE from "./store";

var WS = {
  sockets: {
    logs: undefined,
    state: undefined,
  },
  ReceiveLogEvent: (event) => {
    let logs = STORE.Cache.GetObject("logs")
    if (!logs) {
      logs = [];
    }
    if (logs.length > 5000) {
      logs = [];
      STORE.Cache.SetObject("logs", logs);
    }
    logs.push(event.data);
    STORE.Cache.SetObject("logs", logs);
    STATE.renderPage("logs");
  },
  GetURL: (route) => {
    if (STATE.isWails()) {
      return "ws://127.0.0.1:7777/" + route;
    }
    let host = window.location.origin;
    host = host.replace("https://", "http://");
    host = host.replace("http://", "ws://");
    host = host.replace("5173", "7777");
    host = host.replace("5174", "7777");
    host = host.replace("5175", "7777");
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
      setTimeout(() => {
        STATE.globalRerender();
      }, 2000);
      return;
    }

    sock.onopen = (event) => {
      console.dir(event);
      console.log("WS:", event.type, url);
    };
    sock.onclose = (event) => {
      console.dir(event);
      if (!event.wasClean) {
        // connection not closed cleanly..
      }
      console.log("WS:", event.type, url);
      if (WS.sockets[tag]) {
        WS.sockets[tag].close();
        WS.sockets[tag] = undefined;
      }
      setTimeout(() => {
        WS.NewSocket(url, tag, messageHandler);
      }, 1000);
    };
    sock.onerror = (event) => {
      console.dir(event);
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

export default WS;
