import { useState } from "react";
import STORE from "./store";
import toast from "react-hot-toast";
import axios from "axios";
import dayjs from "dayjs";
import { v4 as uuidv4 } from "uuid";
import { Button } from "./components/ui/button";

const state = (page) => {
  const [value, reload] = useState({ x: 0 });
  const update = () => {
    let x = { ...value };
    x.x += 1;
    reload({ ...x });
  };
  STATE.updates[page] = update;
  STATE.update = update;

  return STATE;
};

export var STATE = {
  globalRerender: () => {
    if (STATE.debug) {
      console.log("GLOBAL RERENDER");
    }

    STATE.updates["root"]();
  },
  renderPage: (page) => {
    if (STATE.updates[page]) {
      if (STATE.debug) {
        console.log("RERENDER:", page);
      }
      STATE.updates[page]();
    }
  },
  rerender: () => {
    if (STATE.debug) {
      console.log("GLOBAL RERENDER");
    }
    Object.keys(STATE.updates).forEach((k) => {
      STATE.updates[k]();
    });
  },

  Theme: {
    borderColor: " border border-[#1a1f2d]  cursor-pointer",
    menuBG: " bg-[#0B0E14]",
    mainBG: " bg-black",
    neutralBtn: " text-white bg-[#2056e1] hover:bg-blue-500 hover:text-white cursor-pointer",
    successBtn: " text-white bg-emerald-500 hover:bg-emerald-400 hover:border-emerald-300 hover:text-white cursor-pointer",
    warningBtn: " text-white bg-orange-500 hover:bg-orange-400 hover:text-white cursor-pointer",
    // neutralBtn: " text-[#2056e1] border-[#2056e1] hover:bg-[#2056e1] hover:text-white cursor-pointer",
    // successBtn: " text-emerald-500 border-emerald-500 hover:bg-emerald-500 hover:text-white cursor-pointer",
    // warningBtn: " text-orange-500 border-orange-500 hover:bg-orange-500 hover:text-white cursor-pointer",
    errorBtn: " text-red-700 border-red-700 cursor-pointer hover:bg-red-500",
    activeSelect: " bg-[#2056e1] text-white cursor-pointer",
    neutralSelect: "  text-white focus:text-[#3168f3] cursor-pointer",
    tabs: "data-[state=active]:text-[#3168f3]",
    greenIcon: " text-emerald-500 border-emerald-500 hover:text-white cursor-pointer",
    redIcon: " text-red-700 border-red-700 hover:text-white cursor-pointer",
    badgeNeutral: " bg-[#2a1db5] hove:bg-white hover:text-black text-white ",
    badgeSuccess: " bg-emerald-500 hove:bg-white hover:text-black text-white ",
    badgeWarning: " bg-orange-500 hove:bg-white hover:text-black text-white ",
    badgeError: " bg-red-500 hove:bg-white hover:text-black text-white ",
    toast: " !text-white bg-[#0B0E14] border-[#1a1f2d]"
  },

  // new form
  GetCurveType: (int) => {
    console.log("CT:", int)
    switch (String(int)) {
      case "0":
        return "P521"
      case "1":
        return "X25519"
      default:
        return "unknown"
    }
  },
  GetEncType: (int) => {
    console.log("CT:", int)
    switch (String(int)) {
      case "0":
        return "None"
      case "1":
        return "AES128"
      case "2":
        return "AES256"
      case "3":
        return "CHACHA20"
      default:
        return "unknown"
    }
  },

  GetServers: async () => {
    let resp = await STATE.callController(null, null, "POST", "/v3/servers", { StartIndex: 0 }, false, false)
    if (resp?.status === 200) {
      if (resp.data?.length > 0) {
        STORE.Cache.SetObject("private-servers", resp.data);
        STATE.PrivateServers = resp.data;
      } else {
        STATE.errorNotification("Unable to find servers");
        STORE.Cache.SetObject("private-servers", []);
        STATE.PrivateServers = [];
      }
      STATE.renderPage("pservers");
    } else if (resp?.status !== 0) {
      STATE.errorNotification("Unable to find servers");
      STORE.Cache.SetObject("private-servers", []);
      STATE.PrivateServers = [];
    }
  },
  // NEW API
  calls: new Map(),
  callController: async (url, secure, method, route, data, skipAuth, boolResponse) => {
    if (STATE.calls.get(route) === true) {
      console.log("call already in progress, backing off")
      return { status: 0, }
    }
    STATE.calls.set(route, true)

    let URL = url ? url : STATE.User?.AuthServer
    if (!URL || URL === "") {
      console.log("no user auth server found")
      STATE.calls.set(route, false)
      return { status: 0, }
    }
    try {
      STATE.toggleLoading({
        logTag: "",
        tag: uuidv4(),
        show: true,
        msg: URL + route,
        includeLogs: false,
      });

      if (!skipAuth || skipAuth === false) {
        data.UID = STATE.User?._id ? STATE.User?._id : ""
        data.Email = STATE.User?.Email ? STATE.User?.Email : ""
        data.DeviceToken = STATE.User?.DeviceToken?.DT ? STATE.User?.DeviceToken?.DT : ""
        if (!data.DeviceToken || data.DeviceToken === "") {
          STATE.errorNotification("No auth token found, please log in again");
          STATE.calls.set(route, false)
          return { data: { Error: "Auth token not found, please log in again" }, status: 401 }
        }
        if (!data.Email || data.Email === "") {
          STATE.errorNotification("No user email/username found, please log in again");
          STATE.calls.set(route, false)
          return { data: { Error: "No user email/username found, please log in again" }, status: 401 }
        }
      }

      let FR = {
        URL: URL,
        // URL: url ? url : STATE.User.AuthServer,
        Secure: secure !== undefined ? secure : STATE.User.Secure,
        Path: route,
        Method: method,
        JSONData: data,
        Timeout: 20000,
      };

      let body = undefined;
      if (FR) {
        try {
          body = JSON.stringify(FR);
        } catch (error) {
          STATE.calls.set(route, false)
          console.dir(error);
          return;
        }
      }

      let resp = await axios.post(STATE.GetURL() + "/v1/method/forwardToController", body, {
        timeout: 10000,
        headers: { "Content-Type": "application/json" },
      });

      console.log("RESPONSE: ", FR.URL, FR.Path)
      console.dir(resp)
      // STATE.callInProgress = false
      STATE.calls.set(route, false)
      STATE.toggleLoading(undefined);
      if (resp && resp.status === 200) {
        if (boolResponse === true) {
          return true
        }
      }
      return { data: resp.data, status: resp.status }

    } catch (error) {
      // STATE.callInProgress = false
      STATE.calls.set(route, false)
      console.dir(error)
      STATE.toggleLoading(undefined);

      if (error?.message === "Network Error") {
        STATE.successNotification("Tunnel connected, network changed");
        return undefined;
      }

      if (error?.response?.data?.Error) {
        STATE.errorNotification(error?.response?.data?.Error);
      }

      if (boolResponse === true) {
        return false
      } else {
        return { data: error.respones?.data, status: error.response?.status }
      }

    }

  },



  // NEW
  // NEW
  // NEW
  // NEW
  // NEW
  // NEW
  GetUser: async () => {
    try {
      let user = STORE.Cache.GetObject("user");
      if (!user) {
        user = await STATE.LoadUser();
        console.log("POST FETCH");
        console.dir(user);
        if (user) {
          STORE.Cache.SetObject("user", user);
        }
      }
      if (user) {
        STATE.User = user;
      }
      return user;
    } catch (err) {
      console.dir(err);
    }
  },
  v2_SetUser: async (u, saveToDisk, server, secure) => {
    try {
      u.AuthServer = server
      u.Secure = secure
      STATE.User = u;
      STORE.Cache.SetObject("user", u);
      if (saveToDisk) {
        await STATE.SaveUser(u);
      }
    } catch (err) {
      console.dir(err);
      // STORE.Cache.interface = window.sessionStorage;
    }
  },
  DelUser: async () => {
    try {
      console.log("DELETING USER FROM DISK");
      await STATE.API.method("delUser", null, true, 10000, false);
    } catch (error) {
      STATE.toggleError("unable to delete encrypted user from disk");
    }
  },
  SaveUser: async (user) => {
    try {
      await STATE.API.method("setUser", user, true, 10000, false);
    } catch (error) {
      STATE.toggleError("unable to save encrypted user to disk");
    }
  },
  LoadUser: async () => {
    try {
      let resp = await STATE.API.method("getUser", null, true, 10000, true);
      console.log("LOAD USER");
      console.dir(resp);
      if (resp?.status === 200 && resp.data) {
        return resp.data;
      } else {
        return undefined;
      }
    } catch (error) {
      STATE.toggleError("unable to get encrypted user from disk");
      return undefined;
    }
  },
  v2_Cleanup: () => { },
  v2_TunnelDelete: async (tun) => {
    try {
      STATE.toggleLoading({
        tag: "tunnel_delete",
        show: true,
        msg: "Deleting tunnel..",
      });

      let resp = await STATE.API.method("deleteTunnel", tun);
      if (resp === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
      } else if (resp.status === 200) {
        STATE.Tunnels.map((t, i) => {
          if (t.Tag === tun.Tag) {
            STATE.Tunnels.splice(i, 1);
          }
        });

        STATE.successNotification("Tunnel deleted", undefined);
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
  },

  v2_TunnelSave: async (tunnel, oldTunnelTag) => {
    let ok = false
    try {

      STATE.toggleLoading({
        tag: "tunnel_save",
        show: true,
        msg: "Saving tunnel..",
      });

      let out = {
        Meta: tunnel,
        OldTag: oldTunnelTag,
      };

      let resp = await STATE.API.method("setTunnel", out);
      if (resp === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
        ok = false
      } else if (resp.status === 200) {
        STATE.successNotification("Tunnel saved", undefined);
        ok = true
      }
    } catch (error) {
      ok = false
      console.dir(error);
    }
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
    return ok
  },

  ConfigSaveInProgress: false,
  v2_ConfigSave: async () => {
    if (STATE.ConfigSaveInProgress) {
      return
    }
    STATE.ConfigSaveInProgress = true

    let newConfig = STATE.Config;
    let ok = false

    try {
      STATE.toggleLoading({
        tag: "config_save",
        show: true,
        msg: "Saving config..",
      });

      let resp = await STATE.API.method("setConfig", newConfig, 120000, false);
      if (resp === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
      } else if (resp.status === 200) {
        ok = true
        STORE.Cache.SetObject("config", newConfig);
        STATE.Config = newConfig;
        STATE.successNotification("Config saved", undefined);
        STORE.Cache.Set("modified_Config", false);
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.ConfigSaveInProgress = false
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
    return ok
  },

  // OLD
  // OLD
  // OLD
  // OLD
  // OLD
  // OLD
  // OLD
  TablePageSize: Number(STORE.Cache.Get("pageSize")),
  setPageSize: (id, count) => {
    let pg = STORE.Cache.GetObject("table_" + id);
    console.log("HIT!", id, pg);
    if (pg) {
      pg.TableSize = Number(count);
      STORE.Cache.SetObject("table_" + id, pg);
    }
    STATE.renderPage(id);
  },
  setPage: (id, page) => {
    let pg = STORE.Cache.GetObject("table_" + id);
    console.log("HIT!", id, pg);
    if (pg) {
      pg.CurrentPage = Number(page);
      STORE.Cache.SetObject("table_" + id, pg);
    }
    STATE.renderPage(id);
  },
  deleteBlocklist: (blocklist) => {
    let newLists = STATE.Config.DNSBlockLists;
    const index = newLists.indexOf(blocklist);

    if (index > -1) {
      newLists.splice(index, 1);
    }

    STATE.Config.DNSBlockLists = newLists;
    STORE.Cache.Set("modified_Config", true);
    STATE.renderPage("dns");
  },
  createTunnel: async () => {
    try {
      let resp = await STATE.API.method("createTunnel");
      if (resp.status != 200) {
        STATE.errorNotification(
          "Cannot create new connection! Status Code: " + resp.status,
        );
        return undefined;
      } else {
        STATE.Tunnels.push(resp.data);
        STATE.rerender();
      }
    } catch (error) {
      console.dir(error);
      STATE.toggleError("unable to create tunnel");
    }
  },
  debug: STORE.Cache.GetBool("debug"),
  toggleDebug: () => {
    let debug = STORE.Cache.GetBool("debug");
    if (debug && debug === true) {
      debug = false;
    } else {
      debug = true;
    }
    STORE.Cache.Set("debug", debug);
    STATE.debug = debug;
  },
  update: undefined,
  updates: {},
  logs: STORE.Cache.GetObject("logs"),
  fullRerender: () => {
    if (STATE.debug) {
      console.log("FULL RERENDER");
    }
    Object.keys(STATE.updates).forEach((k) => {
      STATE.updates[k]();
    });
  },
  // SYSTEM SPECIFIC
  loading: undefined,
  loadTimeout: undefined,
  toggleLoading: (object) => {
    if (object === undefined) {
      STATE.loading = undefined;
      STATE.renderPage("loader");
    }
    if (object?.show) {
      STATE.loading = object;
      STATE.renderPage("loader");

      STATE.loadTimeout = setTimeout(
        () => {
          STATE.loading = undefined;
          STATE.renderPage("loader");
          clearTimeout(STATE.loadTimeout);
        },
        object.timeout ? object.timeout : 10000,
      );

      return;
    } else {
      STATE.loading = undefined;
      return () => {
        STATE.renderPage("loader");
        clearTimeout(STATE.loadTimeout);
      };
    }
  },
  toggleError: (e) => {
    let lastFetch = STORE.Cache.Get("error-timeout");
    let now = dayjs().unix();
    if (now - lastFetch < 3) {
      return;
    }
    toast.error(e);
    STORE.Cache.Set("error-timeout", dayjs().unix());
  },
  errorNotification: (e) => {
    STATE.toggleError(e)
  },
  successNotification: (e) => {
    toast.success(e);
  },
  User: STORE.Cache.GetObject("user"),
  modifiedUser: STORE.Cache.GetObject("modifiedUser"),
  Config: STORE.Cache.GetObject("config"),
  modifiedConfig: STORE.Cache.GetObject("modifiedConfig"),
  SetConfigModifiedState: (state) => {
    STORE.Cache.Set("configIsModified", state);
  },
  UserSaveModifiedSate: () => {
    STORE.Cache.SetObject("modifiedUser", STATE.modifiedUser);
  },
  refreshApiKey: async () => {
    STATE.createObjectIfEmpty("modifiedUser");
    STATE.modifiedUser.APIKey = uuidv4();
    STATE.UserSaveModifiedSate();
    await STATE.UpdateUser()
    STATE.renderPage("account");
  },
  changeServerOnTunnelUsingTag: (tunTag, index) => {
    console.dir(tunTag);
    console.dir(index);
    let tunnels = [...STATE.Tunnels];

    let tun = undefined;
    tunnels?.forEach((t, i) => {
      if (t.Tag.toLowerCase() === tunTag.toLowerCase()) {
        tunnels[i].ServerID = index;
        tun = t;
        return;
      }
    });

    if (tun === undefined) {
      STATE.toggleError("tunnel not found");
      return;
    }
    STATE.v2_TunnelSave(tun, tun.Tag);
  },
  changeServerOnTunnel: (tun, index) => {
    let tunnels = [...STATE.Tunnels];

    tunnels?.forEach((t, i) => {
      if (t.Tag.toLowerCase() === tun.Tag.toLowerCase()) {
        tunnels[i].ServerID = index;
        return;
      }
    });

    STATE.v2_TunnelSave(tun, tun.Tag);
  },
  ConfigSave: async () => {
    // STATE.createObjectIfEmpty("modifiedConfig")
    // if (!STATE.modifiedConfig.Connections) {
    // 	STATE.modifiedConfig.Connections = []
    // }
    //
    // STATE.Tunnels?.forEach(cc => {
    // 	let found = false
    // 	STATE.modifiedConfig.Connections.forEach(mc => {
    // 		if (mc.WindowsGUID == cc.WindowsGUID) {
    // 			found = true
    // 		}
    // 	})
    // 	if (!found) {
    // 		STATE.modifiedConfig.Connections.push(cc)
    // 	}
    // });
    // if (STATE.modifiedLists) {
    // 	STATE.modifiedLists.forEach(l => {
    // 		if (l.Enabled) {
    // 			STATE.Config?.AvailableBlockLists.forEach(al => {
    // 				if (al.Tag === l.Tag) {
    // 					al.Enabled = l.Enabled
    // 				}
    // 			})
    // 		}
    // 	})
    // }
    STATE.modifiedConfig?.DNSRecords?.forEach((r, i) => {
      r.IP = r.IP?.filter((ip) => ip !== "");
      r.TXT = r.TXT?.filter((txt) => txt !== "");
    });

    let newConfig = {
      ...STATE.Config,
      ...STATE.modifiedConfig,
    };

    let success = false;
    try {
      STATE.toggleLoading({
        tag: "config",
        show: true,
        msg: "Saving config..",
      });

      let resp = await STATE.API.method("setConfig", newConfig);
      if (resp === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
      } else if (resp.status === 200) {
        success = true;
        STORE.Cache.SetObject("config", newConfig);
        STORE.Cache.Set("darkMode", newConfig.DarkMode);
        STATE.Config = newConfig;
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
    return success;
  },
  RemoveModifiedConfig: () => {
    STATE.modifiedConfig = undefined;
    STATE.modifiedLists = undefined;
    STATE.ConfigSaveModifiedSate();
    STATE.SetConfigModifiedState(false);
    STATE.globalRerender();
  },
  ConfigSaveModifiedSate: () => {
    STORE.Cache.SetObject("modifiedConfig", STATE.modifiedConfig);
  },
  ConfigSaveOriginalState: () => {
    STORE.Cache.SetObject("config", STATE.Config);
  },
  SaveConnectionsToModifiedConfig: (connections) => {
    STATE.createObjectIfEmpty("modifiedConfig");
    STATE.modifiedConfig.Connections = connections;
    STATE.ConfigSaveModifiedSate();
  },
  GetModifiedConnections: () => {
    let cons = STATE.modifiedConfig?.Connections;
    if (!cons) {
      return [];
    }
    return cons;
  },
  DeleteConnection: async (id) => {
    STATE.Config?.Connections.forEach((c, index) => {
      if (c.WindowsGUID === id) {
        // console.log("SPLICING:", c.WindowsGUID)
        STATE.Config?.Connections.splice(index, 1);
      } else if (c.WindowsGUID === "") {
        STATE.Config?.Connections.splice(index, 1);
      }
    });
    STATE.modifiedConfig?.Connections.forEach((c, index) => {
      if (c.WindowsGUID === id) {
        // console.log("SPLICING:", c.WindowsGUID)
        STATE.modifiedConfig?.Connections.splice(index, 1);
      } else if (c.WindowsGUID === "") {
        STATE.modifiedConfig?.Connections.splice(index, 1);
      }
    });
    STATE.ConfigSave();
    STATE.globalRerender();
  },
  createObjectIfEmpty: (type) => {
    if (STATE[type] === undefined) {
      STATE[type] = {};
    }
  },
  createArrayIfEmpty: (type) => {
    if (STATE[type] === undefined) {
      STATE[type] = [];
    }
  },
  toggleBlocklist: (list) => {
    let found = false;
    STATE.Config?.DNSBlockLists.forEach((l, i) => {
      if (l.Tag === list.Tag) {
        STATE.Config.DNSBlockLists[i].Enabled =
          !STATE.Config.DNSBlockLists[i].Enabled;
        found = true;
      }
    });

    if (!found) {
      STATE.Config.DNSBlockLists.push(list);
    }

    STORE.Cache.Set("modified_Config", true);
    STATE.renderPage("dns");
  },
  getKey: (type, key) => {
    try {
      if (!STATE[type]) {
        return undefined;
      }
      return STATE[type][key];
    } catch (error) {
      console.dir(error);
    }
    return undefined;
  },
  toggleKeyAndReloadDom: (type, key) => {
    try {
      let mod = STATE[type];
      if (mod === undefined) {
        STATE[type] = {};
      }

      STATE[type][key] = !STATE[type][key];
      STORE.Cache.SetObject(type, STATE[type]);
      STORE.Cache.Set("modified_" + type, true);
      STATE.rerender();
      return;
    } catch (error) {
      console.dir(error);
    }
  },
  setArrayAndReloadDom: (type, key, value) => {
    value = value.split(",");
    STATE.setKeyAndReloadDom(type, key, value);
  },
  setKeyAndReloadDom: (type, key, value) => {
    try {
      let mod = STATE[type];
      if (mod === undefined) {
        STATE[type] = {};
      }
      STATE[type][key] = value;
      STORE.Cache.SetObject(type, STATE[type]);
      STORE.Cache.Set("modified_" + type, true);
      STATE.rerender();
      return;
    } catch (error) {
      console.dir(error);
    }
  },
  ConfirmAndExecute: async (type, id, duration, title, subtitle, method) => {
    if (type === "") {
      type = "success";
    }
    await toast[type](
      (t) => (
        <div className={"text-center"} >
          {title && <div className="text-2xl font-bold mb-3">{title}</div>}
          < div className="text-base mb-6" > {subtitle}</div>
          <div className="flex justify-center gap-4">
            <Button
              className={STATE.Theme?.errorBtn}
              onClick={() => toast.dismiss(t.id)}
            >
              NO
            </Button>
            <Button
              className={STATE.Theme?.successBtn}
              onClick={async () => {
                toast.dismiss(t.id);
                await method();
              }}
            >
              YES
            </Button>
          </div>
        </div >
      ),
      { id: id, duration: duration },
    );
  },
  UpdateUser: async () => {
    try {
      let newUser = {
        ...STATE.User,
        ...STATE.modifiedUser,
      };

      let x = await STATE.callController(null, null, "POST", "/v3/user/update",
        { APIKey: newUser.APIKey },
        false, true)
      if (x === true) {
        STORE.Cache.SetObject("user", newUser);
        STORE.Cache.DelObject("modifiedUser");
        STORE.User = newUser;
        STORE.modifiedUser = undefined;
        STATE.successNotification("User updated")
      } else {
        STATE.toggleError(x);
      }
    } catch (e) {
      console.dir(e);
    }

    STATE.toggleLoading(undefined);
  },
  connectToVPN: async (c, server) => {
    console.log("CONNECTING")
    console.dir(c)
    console.dir(server)
    if (!server && !c) {
      STATE.errorNotification("no server or tunnel given when connecting");
      return;
    }

    let user = await STATE.GetUser();
    if (!user.DeviceToken) {
      STATE.errorNotification("You are not logged in");
      STORE.Cache.Clear();
      return;
    }

    let connectionRequest = {};
    connectionRequest.UserID = user._id;
    connectionRequest.DeviceToken = user.DeviceToken.DT;

    if (!c) {
      STATE.Tunnels?.forEach((con) => {
        if (con.Tag === "tunnels") {
          connectionRequest.Tag = con.Tag;
          connectionRequest.EncType = con.EncryptionType;
          c = con;
          return;
        }
      });
    } else {
      connectionRequest.Tag = c.Tag;
      connectionRequest.EncType = c.EncryptionType;
    }

    STATE.Servers?.forEach((s) => {
      if (s._id === c.ServerID) {
        server = s;
        connectionRequest.ServerID = s._id;
      }
    });

    if (!server) {
      STATE.PrivateServers?.forEach((s) => {
        if (s._id === c.ServerID) {
          server = s;
          connectionRequest.ServerID = s._id;
        }
      });
    }

    if (server) {
      connectionRequest.ServerID = server._id;
    } else {
      STATE.errorNotification("unable to find server with the given ID")
      return
    }

    connectionRequest.URL = STATE.User.AuthServer
    connectionRequest.Secure = STATE.User.Secure

    console.log("CONR");
    console.dir(connectionRequest);

    try {
      STATE.toggleLoading({
        logTag: "connect",
        tag: "CONNECT",
        show: true,
        msg: "Connecting...",
        includeLogs: true,
      });

      let method = "connect";
      let resp = await STATE.API.method(method, connectionRequest);
      if (resp === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
      } else {
        if (resp.status === 401) {
          STATE.successNotification(
            "Unauthorized, logging you out!",
            undefined,
          );
          STATE.LogoutCurrentToken();
        } else if (resp.status === 200) {
          STATE.successNotification("connection ready");
        }
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    STATE.GetBackendState();
  },
  disconnectFromVPN: async (c) => {
    STATE.toggleLoading({
      logTag: "disconnecting",
      tag: "DISCONNECT",
      show: true,
      msg: "Disconnecting ...",
      includeLogs: true,
    });

    try {
      let x = await STATE.API.method(
        "disconnect",
        { ID: c.ID },
        false,
        20000,
      );
      if (x === undefined) {
        STATE.errorNotification("Unknown error, please try again in a moment");
      } else {
        STATE.successNotification("Disconnected", {
          Title: "DISCONNECTED",
          Body: "You have been disconnected from " + c.Tag,
          TimeoutType: "default",
        });
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    STATE.GetBackendState();
  },
  FinalizeLogout: async () => {
    STATE.DelUser();
    STORE.Cache.Clear();
    STATE.GetBackendState();
    window.location.replace("/#/login");
    window.location.reload();
  },
  LogoutAllTokens: async () => {
    if (STATE.User) {
      STATE.LogoutToken(STATE.User.DeviceToken?.DT, true);
    }
  },
  LogoutCurrentToken: async () => {
    if (STATE.User) {
      STATE.LogoutToken(STATE.User.DeviceToken?.DT, false);
    }
  },
  LogoutToken: async (token, all) => {
    let user = STORE.Cache.GetObject("user");
    if (!user) {
      return;
    }

    let logoutUser = false;
    if (user.DeviceToken?.DT === token.DT) {
      logoutUser = true;
    }

    let resp = await STATE.callController(null, null, "POST", "/v3/user/logout",
      { DeviceToken: token.DT, UserID: user._id, All: all },
      false, false)
    if (resp && resp.status === 200) {

      STATE.successNotification("device logged out", undefined);

      if (logoutUser === true || all === true) {
        STATE.FinalizeLogout();
        return
      } else {
        let toks = [];
        user.Tokens?.map((t) => {
          if (t.DT !== token.DT) {
            toks.push(t);
          }
        });
        user.Tokens = toks;
      }

      STORE.Cache.SetObject("user", user);
      STATE.User = user;
    } else {
      STATE.errorNotification("Unable to log out device", undefined);
    }
    STATE.rerender();
  },

  ForwardToController: async (req, loader) => {
    if (!req.URL || req.URL === "") {
      req.URL = state.User?.AuthServer
    }
    if (!req.URL || req.URL === "") {
      STATE.toggleError("no server selected")
    }
    req.Secure = req.Secure === undefined ? true : req.Secure
    STATE.toggleLoading(loader);
    console.log("FW:", req.Secure)
    let x = await STATE.API.method("forwardToController", req);
    STATE.toggleLoading(undefined);
    return x;
  },
  // ForwardToController: async (req, loader) => {
  //   if (!req.URL) {
  //     req.URL = state.User?.AuthServer
  //   }
  //   if (!req.URL) {
  //     STATE.toggleError("no server selected")
  //     return
  //   }
  //   if (req.URL) {
  //     if (!req.URL.endsWith("/")) {
  //       req.URL += "/"
  //     }
  //   }

  //   let url = req.URL + req.Path
  //   let data = req.JSONData ? req.JSONData : null
  //   let noLogout = req.NoLogout ? req.NoLogout : true
  //   let timeout = req.Timeout ? req.Timeout : 20000
  //   let ignoreError = req.InoreError ? req.IgnoreError : false


  //   STATE.toggleLoading(loader);
  //   let x = await STATE.API.methodv2(url, data, noLogout, timeout, ignoreError);
  //   STATE.toggleLoading(undefined);
  //   return x;
  // },

  LicenseKey: "",
  UpdateLicenseInput: (value) => {
    STATE.LicenseKey = value;
    STATE.rerender();
  },
  GetPrivateServersInProgress: false,
  Servers: STORE.Cache.GetObject("servers"),
  PrivateServers: STORE.Cache.GetObject("private-servers"),
  updatePrivateServers: () => {
    STORE.Cache.SetObject("private-servers", STATE.PrivateServers);
  },
  ModifiedServers: [],
  UpdateModifiedServer: function(server, key, value) {
    let found = false;
    STATE.ModifiedServers.forEach((s, i) => {
      if (s._id === server._id) {
        STATE.ModifiedServers[i][key] = value;
        found = true;
        return;
      }
    });
    if (!found) {
      server[key] = value;
      STATE.ModifiedServers.push(server);
    }
    STATE.renderPage("inspect-server");
  },
  ActivateLicense: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.LicenseKey === "") {
      STATE.errorNotification("License key is required");
      return;
    }

    let ok = await STATE.callController(null, null, "POST", "/v3/key/activate",
      { Key: STATE.LicenseKey },
      false, true)
    if (ok) {
      STATE.User.Key = {
        Key: "[shown on next login]",
      };
    }

    STORE.Cache.SetObject("user", { ...STATE.User });
    STATE.rerender();
  },
  GetQRCode: async (inputs) => {
    return STATE.API.method("getQRCode", inputs);
  },
  ConfirmTwoFactorCode: async (inputs, url) => {


    return await STATE.callController(null, null, "POST", "/v3/user/2fa/confirm",
      inputs,
      false, false)

    return STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/2fa/confirm",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
      },
      {
        tag: "two-factor",
        show: true,
        msg: "confirming two-factor authentication ..",
      },
    );
  },

  Org: STORE.Cache.GetObject("org"),
  updateOrg: (org) => {
    if (org) {
      STORE.Cache.SetObject("org", org);
      STATE.Org = org;
      STATE.rerender();
    }
  },
  UpdateOrgInProgress: false,
  API_UpdateOrg: async (org) => {
    if (STATE.UpdateOrgInProgress) {
      return;
    }
    STATE.UpdateOrgInProgress = true;

    let resp = undefined;
    try {
      let FR = {
        Path: "v3/org/update",
        Method: "POST",
        Timeout: 10000,
      };

      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
          Org: org,
        };
      } else {
        STATE.UpdateOrgInProgress = false;
        return undefined;
      }

      STATE.toggleLoading({
        tag: "ORG_UPDATE",
        show: true,
        msg: "updating organization ...",
      });

      resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.updateOrg(org);
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    STATE.UpdateOrgInProgress = false;
  },
  CreateOrgInProgress: false,
  API_GetUsers: async (offset, limit) => {
    try {
      let FR = {
        URL: STATE.User.AuthServer,
        Secure: STATE.User.Secure,
        Path: "v3/user/list",
        Method: "POST",
        Timeout: 10000,
      };
      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
          Offset: offset,
          Limit: limit,
        };
      } else {
        return undefined;
      }

      STATE.toggleLoading({
        tag: "USERS_LIST",
        show: true,
        msg: "fetching users..",
      });

      let resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.toggleLoading(undefined);
        return resp.data
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return undefined;
  },
  API_GetDevices: async (offset, limit) => {
    try {
      let FR = {
        URL: STATE.User.AuthServer,
        Secure: STATE.User.Secure,
        Path: "v3/device/list",
        Method: "POST",
        Timeout: 10000,
      };
      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
          Offset: offset,
          Limit: limit,
        };
      } else {
        return undefined;
      }

      STATE.toggleLoading({
        tag: "DEVICE_LIST",
        show: true,
        msg: "fetching devices..",
      });

      let resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.toggleLoading(undefined);
        return resp.data
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return undefined;
  },
  DNSStats: STORE.Cache.GetObject("dns-stats"),
  DNSListDateFormat: "ddd D. HH:mm:ss",
  GetDNSStats: async () => {
    try {
      let resp = await STATE.API.method("getDNSStats", null);
      if (resp?.status === 200) {
        STATE.DNSStats = resp.data
        STORE.Cache.SetObject("dns-stats", resp.data);
      }
    } catch (error) {
      console.dir(error)
    }

  },
  State: STORE.Cache.GetObject("state"),
  StateFetchInProgress: false,
  GetURL: () => {
    let host = window.location.origin;
    // let port = STORE.Cache.Get("api_port")
    // let ip = STORE.Cache.Get("api_ip")
    host = host.replace("http://", "https://");
    host = host.replace("5173", "7777");
    return host;
  },
  GetBackendState: async () => {
    if (STATE.debug) {
      console.log("STATE UPDATE !");
    }
    if (STATE.StateFetchInProgress) {
      return;
    }
    STATE.StateFetchInProgress = true;

    try {
      let response = undefined;
      let host = STATE.GetURL();

      response = await axios.post(
        host + "/v1/method/getState",
        {},
        { headers: { "Content-Type": "application/json" } },
      );

      if (response.status === 200) {
        if (response.data) {
          STATE.State = response.data?.State;
          STATE.Config = response.data?.Config;
          STATE.Network = response.data?.Network;
          // STATE.User = response.data?.User;
          STATE.Tunnels = response.data?.Tunnels;
          STATE.ActiveTunnels = response.data?.ActiveTunnels;
          STATE.Version = response.data?.Version;
          STATE.APIVersion = response.data?.APIVersion;

          STORE.Cache.SetObject("active-tunnels", STATE.ActiveTunnels);
          STORE.Cache.SetObject("tunnels", STATE.Tunnels);
          STORE.Cache.SetObject("state", STATE.State);
          STORE.Cache.SetObject("config", STATE.Config);

          // STORE.Cache.SetObject("user", STATE.User);
          // STORE.Cache.Set("darkMode", STATE.Config.DarkMode);
        }

        STATE.globalRerender();
      }
      STATE.StateFetchInProgress = false;

      return response;
    } catch (error) {
      STATE.StateFetchInProgress = false;
      console.dir(error);
      STATE.errorNotification("unable to load state...");
      return undefined;
    }
  },
  API: {
    async method(method, data, noLogout, timeout, ignoreError) {
      try {
        let response = undefined;
        let host = STATE.GetURL();

        let to = 30000;
        if (timeout) {
          to = timeout;
        }

        let body = undefined;
        if (data) {
          try {
            body = JSON.stringify(data);
          } catch (error) {
            console.dir(error);
            return;
          }
        }

        response = await axios.post(host + "/v1/method/" + method, body, {
          timeout: to,
          headers: { "Content-Type": "application/json" },
        });

        if (response.data?.Message) {
          STATE.successNotification(response?.data?.Message);
        } else if (response.data?.Error) {
          STATE.errorNotification(response?.data?.Error);
        }

        return response;
      } catch (error) {
        console.dir(error);
        if (error?.message === "Network Error") {
          STATE.successNotification("Tunnel connected, network changed");
          return undefined;
        }

        if (!ignoreError) {
          if (!noLogout || noLogout === false) {
            if (error?.response?.status === 401) {
              STATE.LogoutCurrentToken();
              return;
            }
          }

          if (error?.response?.data?.Message) {
            STATE.errorNotification(error?.response?.data?.Message);
          } else if (error?.response?.data?.Error) {
            STATE.errorNotification(error?.response?.data?.Error);
          } else if (error?.response?.data?.error) {
            STATE.errorNotification(error?.response?.data.error);
          } else {
            console.log(typeof error.response.data);
            console.dir(error.response.data);
            if (typeof error?.response?.data === "string") {
              STATE.errorNotification(error?.response?.data);
            } else {
              try {
                let out = "";
                error?.response?.data?.forEach((err) => {
                  out = out + err + ".\n";
                });
                STATE.errorNotification(out);
              } catch (error) {
                STATE.errorNotification("Unknown error");
              }
            }
          }
        }

        return error?.response;
      }
    },
  },
  // 299,792,458
  updateNodes: (nodes) => {
    if (nodes && nodes.length > 0) {
      STORE.Cache.SetObject("nodes", nodes);
      STATE.Servers = nodes;
      STATE.rerender();
    } else {
      STORE.Cache.SetObject("nodes", []);
      STATE.Servers = [];
      STATE.rerender();
    }
  },
  ModifiedNodes: STORE.Cache.GetObject("modifiedNodes"),
  SaveModifiedNodes: () => {
    STORE.Cache.SetObject("modifiedNodes", STATE.ModifiedNodes);
  },
  GetCountryName: (countryCode) => {
    let x = STATE.countryCodeMap[countryCode]
    if (x === undefined) {
      return countryCode
    }
    return x
  },
  countryCodeMap: {
    "AF": "Afghanistan",
    "AX": "Aland Islands",
    "AL": "Albania",
    "DZ": "Algeria",
    "AS": "American Samoa",
    "AD": "Andorra",
    "AO": "Angola",
    "AI": "Anguilla",
    "AQ": "Antarctica",
    "AG": "Antigua and Barbuda",
    "AR": "Argentina",
    "AM": "Armenia",
    "AW": "Aruba",
    "AU": "Australia",
    "AT": "Austria",
    "AZ": "Azerbaijan",
    "BS": "Bahamas",
    "BH": "Bahrain",
    "BD": "Bangladesh",
    "BB": "Barbados",
    "BY": "Belarus",
    "BE": "Belgium",
    "BZ": "Belize",
    "BJ": "Benin",
    "BM": "Bermuda",
    "BT": "Bhutan",
    "BO": "Bolivia",
    "BA": "Bosnia and Herzegovina",
    "BW": "Botswana",
    "BV": "Bouvet Island",
    "BR": "Brazil",
    "IO": "British Indian Ocean Territory",
    "BN": "Brunei Darussalam",
    "BG": "Bulgaria",
    "BF": "Burkina Faso",
    "BI": "Burundi",
    "KH": "Cambodia",
    "CM": "Cameroon",
    "CA": "Canada",
    "CV": "Cape Verde",
    "KY": "Cayman Islands",
    "CF": "Central African Republic",
    "TD": "Chad",
    "CL": "Chile",
    "CN": "China",
    "CX": "Christmas Island",
    "CC": "Cocos (Keeling) Islands",
    "CO": "Colombia",
    "KM": "Comoros",
    "CG": "Congo",
    "CD": "Congo, The Democratic Republic of the",
    "CK": "Cook Islands",
    "CR": "Costa Rica",
    "CI": "Cote D'Ivoire",
    "HR": "Croatia",
    "CU": "Cuba",
    "CY": "Cyprus",
    "CZ": "Czech Republic",
    "DK": "Denmark",
    "DJ": "Djibouti",
    "DM": "Dominica",
    "DO": "Dominican Republic",
    "EC": "Ecuador",
    "EG": "Egypt",
    "SV": "El Salvador",
    "GQ": "Equatorial Guinea",
    "ER": "Eritrea",
    "EE": "Estonia",
    "ET": "Ethiopia",
    "FK": "Falkland Islands (Malvinas)",
    "FO": "Faroe Islands",
    "FJ": "Fiji",
    "FI": "Finland",
    "FR": "France",
    "GF": "French Guiana",
    "PF": "French Polynesia",
    "TF": "French Southern Territories",
    "GA": "Gabon",
    "GM": "Gambia",
    "GE": "Georgia",
    "DE": "Germany",
    "GH": "Ghana",
    "GI": "Gibraltar",
    "GR": "Greece",
    "GL": "Greenland",
    "GD": "Grenada",
    "GP": "Guadeloupe",
    "GU": "Guam",
    "GT": "Guatemala",
    "GG": "Guernsey",
    "GN": "Guinea",
    "GW": "Guinea-Bissau",
    "GY": "Guyana",
    "HT": "Haiti",
    "HM": "Heard Island and Mcdonald Islands",
    "VA": "Holy See (Vatican City State)",
    "HN": "Honduras",
    "HK": "Hong Kong",
    "HU": "Hungary",
    "IS": "Iceland",
    "IN": "India",
    "ID": "Indonesia",
    "IR": "Iran, Islamic Republic Of",
    "IQ": "Iraq",
    "IE": "Ireland",
    "IM": "Isle of Man",
    "IL": "Israel",
    "IT": "Italy",
    "JM": "Jamaica",
    "JP": "Japan",
    "JE": "Jersey",
    "JO": "Jordan",
    "KZ": "Kazakhstan",
    "KE": "Kenya",
    "KI": "Kiribati",
    "KR": "Korea",
    "KW": "Kuwait",
    "KG": "Kyrgyzstan",
    "LA": "Lao People's Democratic Republic",
    "LV": "Latvia",
    "LB": "Lebanon",
    "LS": "Lesotho",
    "LR": "Liberia",
    "LY": "Libyan Arab Jamahiriya",
    "LI": "Liechtenstein",
    "LT": "Lithuania",
    "LU": "Luxembourg",
    "MO": "Macao",
    "MK": "Macedonia, The Former Yugoslav Republic of",
    "MG": "Madagascar",
    "MW": "Malawi",
    "MY": "Malaysia",
    "MV": "Maldives",
    "ML": "Mali",
    "MT": "Malta",
    "MH": "Marshall Islands",
    "MQ": "Martinique",
    "MR": "Mauritania",
    "MU": "Mauritius",
    "YT": "Mayotte",
    "MX": "Mexico",
    "FM": "Micronesia, Federated States of",
    "MD": "Moldova, Republic of",
    "MC": "Monaco",
    "MN": "Mongolia",
    "MS": "Montserrat",
    "MA": "Morocco",
    "MZ": "Mozambique",
    "MM": "Myanmar",
    "NA": "Namibia",
    "NR": "Nauru",
    "NP": "Nepal",
    "NL": "Netherlands",
    "AN": "Netherlands Antilles",
    "NC": "New Caledonia",
    "NZ": "New Zealand",
    "NI": "Nicaragua",
    "NE": "Niger",
    "NG": "Nigeria",
    "NU": "Niue",
    "NF": "Norfolk Island",
    "MP": "Northern Mariana Islands",
    "NO": "Norway",
    "OM": "Oman",
    "PK": "Pakistan",
    "PW": "Palau",
    "PS": "Palestinian Territory, Occupied",
    "PA": "Panama",
    "PG": "Papua New Guinea",
    "PY": "Paraguay",
    "PE": "Peru",
    "PH": "Philippines",
    "PN": "Pitcairn",
    "PL": "Poland",
    "PT": "Portugal",
    "PR": "Puerto Rico",
    "QA": "Qatar",
    "RE": "Reunion",
    "RO": "Romania",
    "RU": "Russian Federation",
    "RW": "Rwanda",
    "SH": "Saint Helena",
    "KN": "Saint Kitts and Nevis",
    "LC": "Saint Lucia",
    "PM": "Saint Pierre and Miquelon",
    "VC": "Saint Vincent and the Grenadines",
    "WS": "Samoa",
    "SM": "San Marino",
    "ST": "Sao Tome and Principe",
    "SA": "Saudi Arabia",
    "SN": "Senegal",
    "CS": "Serbia and Montenegro",
    "SC": "Seychelles",
    "SL": "Sierra Leone",
    "SG": "Singapore",
    "SK": "Slovakia",
    "SI": "Slovenia",
    "SB": "Solomon Islands",
    "SO": "Somalia",
    "ZA": "South Africa",
    "GS": "South Georgia and the South Sandwich Islands",
    "ES": "Spain",
    "LK": "Sri Lanka",
    "SD": "Sudan",
    "SR": "Suriname",
    "SJ": "Svalbard and Jan Mayen",
    "SZ": "Swaziland",
    "SE": "Sweden",
    "CH": "Switzerland",
    "SY": "Syrian Arab Republic",
    "TW": "Taiwan, Province of China",
    "TJ": "Tajikistan",
    "TZ": "Tanzania, United Republic of",
    "TH": "Thailand",
    "TL": "Timor-Leste",
    "TG": "Togo",
    "TK": "Tokelau",
    "TO": "Tonga",
    "TT": "Trinidad and Tobago",
    "TN": "Tunisia",
    "TR": "Turkey",
    "TM": "Turkmenistan",
    "TC": "Turks and Caicos Islands",
    "TV": "Tuvalu",
    "UG": "Uganda",
    "UA": "Ukraine",
    "AE": "United Arab Emirates",
    "GB": "United Kingdom",
    "US": "United States",
    "UM": "United States Minor Outlying Islands",
    "UY": "Uruguay",
    "UZ": "Uzbekistan",
    "VU": "Vanuatu",
    "VE": "Venezuela",
    "VN": "Viet Nam",
    "VG": "Virgin Islands, British",
    "VI": "Virgin Islands, U.S.",
    "WF": "Wallis and Futuna",
    "EH": "Western Sahara",
    "YE": "Yemen",
    "ZM": "Zambia",
    "ZW": "Zimbabwe"
  }
};

export default state;

