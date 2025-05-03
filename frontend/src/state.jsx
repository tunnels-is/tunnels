import { useState } from "react";
import STORE from "./store";
import toast from "react-hot-toast";
import axios from "axios";
import dayjs from "dayjs";
import { v4 as uuidv4 } from "uuid";
import https from 'https';

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
        STATE.SaveUser(u);
      }
      return u;
    } catch (err) {
      console.dir(err);
      STORE.Cache.interface = window.sessionStorage;
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
      } else if (resp.status === 200) {
        STATE.successNotification("Tunnel saved", undefined);
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
  },

  v2_ConfigSave: async () => {
    let newConfig = STATE.Config;

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
        STORE.Cache.SetObject("config", newConfig);
        STATE.Config = newConfig;
        STATE.successNotification("Config saved", undefined);
        STORE.Cache.Set("modified_Config", false);
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
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
  toggleLoading: (object) => {
    if (object === undefined) {
      STATE.loading = undefined;
      STATE.renderPage("loader");
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
      STATE.loading = object;
      STATE.renderPage("loader");

      const to = setTimeout(
        () => {
          STATE.loading = undefined;
          STATE.renderPage("loader");
          clearTimeout(to);
        },
        object.timeout ? object.timeout : 10000,
      );

      return;
    } else {
      STATE.loading = undefined;
      return () => {
        STATE.renderPage("loader");
        clearTimeout(to);
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
    let lastFetch = STORE.Cache.Get("error-timeout");
    let now = dayjs().unix();
    if (now - lastFetch < 3) {
      return;
    }
    toast.error(e);
    STORE.Cache.Set("error-timeout", dayjs().unix());
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
  refreshApiKey: () => {
    STATE.createObjectIfEmpty("modifiedUser");
    STATE.modifiedUser.APIKey = uuidv4();
    STATE.UserSaveModifiedSate();
    STATE.rerender();
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
        <div className="content">
          {title && <div className="title">{title}</div>}
          <div className="subtitle">{subtitle}</div>
          <div className="buttons">
            <div className="button no" onClick={() => toast.dismiss(t.id)}>
              NO
            </div>
            <div
              className="button yes"
              onClick={async function() {
                toast.dismiss(t.id);
                await method();
              }}
            >
              YES
            </div>
          </div>
        </div>
      ),
      { id: id, duration: duration },
    );
  },
  UpdateUser: async () => {
    try {
      STATE.toggleLoading({
        logTag: "",
        tag: "USER-UPDATE",
        show: true,
        msg: "Updating User Settings",
        includeLogs: false,
      });

      let FORM = {
        Email: STATE.User.Email,
        DeviceToken: STATE.User.DeviceToken.DT,
        APIKey: STATE.modifiedUser.APIKey,
      };
      let newUser = {
        ...STATE.User,
        ...STATE.modifiedUser,
      };

      let FR = {
        Path: "v3/user/update",
        Method: "POST",
        JSONData: FORM,
        Timeout: 10000,
      };

      let x = await STATE.API.method("forwardToController", FR);
      if (x && x.status === 200) {
        STORE.Cache.SetObject("user", newUser);
        STORE.Cache.DelObject("modifiedUser");
        STORE.User = newUser;
        STORE.modifiedUser = undefined;
        STATE.showSuccessToast("User updated", undefined);
      } else {
        STATE.toggleError(x);
      }
    } catch (e) {
      console.dir(e);
    }

    STATE.toggleLoading(undefined);
  },
  connectToVPN: async (c, server) => {
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

      if (c.ServerIP === "") {
        STATE.Servers?.forEach((s) => {
          if (s._id === c.ServerID) {

            server = s;
            connectionRequest.ServerIP = s.IP;
            connectionRequest.ServerPort = s.Port;
            connectionRequest.ServerID = s._id;
          }

        });
        if (!server) {
          STATE.PrivateServers?.forEach((s) => {
            if (s._id === c.ServerID) {

              server = s;
              connectionRequest.ServerIP = s.IP;
              connectionRequest.ServerPort = s.Port;
              connectionRequest.ServerID = s._id;
            }

          });

        }
      } else {
        connectionRequest.ServerIP = c.ServerIP;
        connectionRequest.ServerPort = c.ServerPort;
        connectionRequest.ServerID = c.ServerID;
      }
    }

    if (server) {
      connectionRequest.ServerIP = server.IP;
      connectionRequest.ServerPort = server.Port;
      connectionRequest.ServerID = server._id;
    }

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
    let user = STORE.Cache.GetObject("user");
    if (!user) {
      return;
    }

    let LF = {
      DeviceToken: user.DeviceToken.DT,
      Email: user.Email,
    };

    let FR = {
      Path: "v3/user/logout/all",
      Method: "POST",
      JSONData: LF,
      Timeout: 20000,
      LogoutUser: true,
      SyncUser: false,
    };

    let x = await STATE.API.method("forwardToController", FR);
    if (x && x.status === 200) {
      STATE.successNotification("device logged out", undefined);
      STATE.FinalizeLogout();
    } else {
      STATE.errorNotification("Unable to log out device", undefined);
    }
  },
  LogoutCurrentToken: async () => {
    if (STATE.User !== undefined) {
      STATE.LogoutToken(STATE.User.DeviceToken?.DT);
    }
  },
  LogoutToken: async (token) => {
    let user = STORE.Cache.GetObject("user");
    if (!user) {
      return;
    }

    let LF = {
      DeviceToken: token.DT,
      Email: user.Email,
    };

    let u = STORE.Cache.GetObject("user");
    let logoutUser = false;
    let syncUser = true;
    if (u?.DeviceToken.DT === token.DT) {
      logoutUser = true;
      syncUser = false;
    }

    let FR = {
      Path: "v3/user/logout",
      Method: "POST",
      JSONData: LF,
      Timeout: 20000,
      SyncUser: syncUser,
      LogoutUser: logoutUser,
    };

    let x = await STATE.API.method("forwardToController", FR);

    if (x && x.status === 200) {
      STATE.successNotification("device logged out", undefined);

      if (logoutUser === true) {
        STATE.FinalizeLogout();
      } else {
        let toks = [];
        u?.Tokens?.map((t) => {
          if (t.DT !== token.DT) {
            toks.push(t);
          }
        });
        u.Tokens = toks;
      }

      STORE.Cache.SetObject("user", u);
      STATE.User = u;
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
  API_UpdateServer: async (server) => {
    // let server = undefined
    // STATE.ModifiedServers?.forEach(s => {
    // 	if (s._id === id) {
    // 		server = s
    // 	}
    // })

    if (!server) {
      return;
    }

    let resp = undefined;
    try {
      let FR = {
        URL: STATE.User.AuthServer,
        Secure: STATE.User.Secure,
        Path: "v3/servers/update",
        Method: "POST",
        Timeout: 10000,
      };
      FR.JSONData = {
        UID: STATE.User._id,
        DeviceToken: STATE.User.DeviceToken.DT,
        Server: server,
      };

      STATE.toggleLoading({
        tag: "SERVER_UPDATE",
        show: true,
        msg: "Updating your server ...",
      });

      resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        // STATE.ModifiedServers?.forEach((s, i) => {
        // 	if (s._id === id) {
        // 		STATE.ModifiedServers.splice(i, 1)
        // 	}
        // })

        STATE.PrivateServers.forEach((s, i) => {
          if (s._id === server._id) {
            STATE.PrivateServers[i] = server;
          }
        });
        STATE.updatePrivateServers();
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return resp;
  },
  API_CreateServer: async (server) => {
    let resp = undefined;
    try {
      let FR = {
        URL: STATE.User.AuthServer,
        Secure: STATE.User.Secure,
        Path: "v3/servers/create",
        Method: "POST",
        Timeout: 10000,
      };
      FR.JSONData = {
        UID: STATE.User._id,
        DeviceToken: STATE.User.DeviceToken.DT,
        Server: server,
      };

      STATE.toggleLoading({
        tag: "SERVER_CREATE",
        show: true,
        msg: "Creating your server ...",
      });

      resp = await STATE.API.method("forwardToController", FR, true, 10000, false);
      if (resp?.status === 200) {
        if (!STATE.PrivateServers) {
          STATE.PrivateServers = [];
        }
        STATE.PrivateServers.push(resp.data);
        STATE.updatePrivateServers();
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return resp;
  },
  GetPrivateServers: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.GetPrivateServersInProgress) {
      return;
    }
    STATE.GetPrivateServersInProgress = true;

    let resp = undefined;
    try {
      let timeout = STORE.Cache.GetObject("private-servers_ct");
      let now = dayjs().unix();
      let diff = now - timeout;
      if (now - timeout > 30 || !timeout) {
      } else {
        STATE.GetPrivateServersInProgress = false;
        STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds");
        // return;
      }

      resp = await STATE.ForwardToController(
        {
          URL: STATE.User.AuthServer,
          Secure: STATE.User.Secure,
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
          msg: "Searching for servers ..",
        },
      );
    } catch (error) {
      console.dir(error);
    }

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
    } else {
      STATE.errorNotification("Unable to find servers");
      STORE.Cache.SetObject("private-servers", []);
      STATE.PrivateServers = [];
    }

    STATE.GetPrivateServersInProgress = false;
  },
  GetServers: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.GetPrivateServersInProgress) {
      return;
    }
    STATE.GetPrivateServersInProgress = true;

    let resp = undefined;
    try {
      let timeout = STORE.Cache.GetObject("servers_ct");
      let now = dayjs().unix();
      let diff = now - timeout;
      if (now - timeout > 30 || !timeout) {
      } else {
        STATE.GetPrivateServersInProgress = false;
        STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds");
        return;
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
          msg: "Searching for servers ..",
        },
      );
    } catch (error) {
      console.dir(error);
    }

    if (resp?.status === 200) {
      STORE.Cache.SetObject("servers", resp.data);
      STATE.Servers = resp.data;
      STATE.renderPage("servers");
    } else {
      STATE.errorNotification("Unable to find servers");
    }

    STATE.GetPrivateServersInProgress = false;
  },
  ActivateLicense: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.LicenseKey === "") {
      STATE.errorNotification("License key is required");
      return;
    }

    STATE.ForwardToController(
      {
        URL: STATE.User.AuthServer,
        Path: "v3/key/activate",
        Method: "POST",
        JSONData: {
          Key: STATE.LicenseKey,
          Email: STATE.User.Email,
        },
        Timeout: 20000,
      },
      {
        tag: "key-activation",
        show: true,
        msg: "activating your license key...",
      },
    );

    STATE.User.Key = {
      Key: "[shown on next login]",
    };

    STORE.Cache.SetObject("user", { ...STATE.User });
    STATE.rerender();
  },
  GetResetCode: async (inputs, url) => {
    return STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/reset/code",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
      },
      {
        tag: "reset-code",
        show: true,
        msg: "sending you a reset code ...",
      },
    );
  },
  GetQRCode: async (inputs) => {
    return STATE.API.method("getQRCode", inputs);
  },
  ConfirmTwoFactorCode: async (inputs, url) => {
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
  API_EnableAccount: async (inputs, url) => {
    return STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/enable",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
      },
      {
        tag: "enable-account",
        show: true,
        msg: "enabling your account ..",
      },
    );
  },
  ResetPassword: async (inputs, url) => {
    return STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/reset/password",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
      },
      {
        tag: "reset-password",
        show: true,
        msg: "resetting your password ..",
      },
    );
  },
  Register: async (inputs, url) => {
    return STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/create",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
        Secure: false
      },
      {
        tag: "register",
        show: true,
        msg: "creating your account ..",
      },
    );
  },
  Login: async (inputs, remember, url) => {
    STORE.Local.setItem("default-device-name", inputs["devicename"]);
    STORE.Cache.Set("default-email", inputs["email"]);

    let x = await STATE.ForwardToController(
      {
        URL: url,
        Path: "v3/user/login",
        Method: "POST",
        JSONData: inputs,
        Timeout: 20000,
        Secure: false,
      },
      {
        tag: "login",
        show: true,
        msg: "logging you in..",
      },
    );
    if (x?.status === 200) {
      STATE.v2_SetUser(x.data, remember, url, false);
      window.location.replace("/");
    }

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
  API_CreateOrg: async (org) => {
    if (STATE.CreateOrgInProgress) {
      return;
    }
    STATE.CreateOrgInProgress = true;

    let resp = undefined;
    try {
      let FR = {
        Path: "v3/org/create",
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
        STATE.CreateOrgInProgress = false;
        return undefined;
      }

      STATE.toggleLoading({
        tag: "ORG_CREATE",
        show: true,
        msg: "creating organization ...",
      });

      resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.updateOrg(resp.data);
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    STATE.CreateOrgInProgress = false;
  },
  GetOrgInProgress: false,
  API_GetOrg: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.GetOrgInProgress) {
      return;
    }
    STATE.GetOrgInProgress = true;

    try {
      let timeout = STORE.Cache.GetObject("org_ct");
      let now = dayjs().unix();
      let diff = now - timeout;
      if (now - timeout > 30 || !timeout) {
      } else {
        STATE.errorNotification("Next refresh in " + (30 - diff) + " seconds");
        return;
      }

      let FR = {
        Path: "v3/org",
        Method: "POST",
        Timeout: 10000,
      };
      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
        };
      } else {
        return undefined;
      }

      STATE.toggleLoading({
        tag: "ORG_FETCH",
        show: true,
        msg: "Fetching Your Organization Information ...",
      });

      let resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.updateOrg(resp.data);
        STATE.Groups = resp.data.Groups;
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.GetOrgInProgress = false;
    STATE.toggleLoading(undefined);
  },
  UpdateGroup: (group) => {
    if (STATE.Org) {
      if (!STATE.Org.Groups) {
        STATE.Org.Groups = [group];
        STATE.updateOrg(STATE.Org);
        return;
      }
      let found = false;
      STATE.Org.Groups?.forEach((g, i) => {
        if (g._id === group._id) {
          found = true;
          STATE.Org.Groups[i] = group;
          STATE.updateOrg(STATE.Org);
        }
      });
      if (found === false) {
        STATE.Org.Groups.push(group);
        STATE.updateOrg(STATE.Org);
      }
    }
  },
  API_UpdateGroup: async (group) => {
    try {
      let FR = {
        Path: "v3/group/update",
        Method: "POST",
        Timeout: 10000,
      };
      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
          Group: group,
        };
      } else {
        return undefined;
      }

      STATE.toggleLoading({
        tag: "GROUP_UPDATE",
        show: true,
        msg: "updating ...",
      });

      let resp = await STATE.API.method("forwardToController", FR);
      if (resp && resp.status === 200) {
        STATE.UpdateGroup(group);
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return;
  },
  API_CreateGroup: async (group) => {
    let resp = undefined;
    try {
      let FR = {
        Path: "v3/group/create",
        Method: "POST",
        Timeout: 10000,
      };
      if (STATE.User) {
        FR.JSONData = {
          UID: STATE.User._id,
          DeviceToken: STATE.User.DeviceToken.DT,
          Group: group,
        };
      } else {
        return undefined;
      }

      STATE.toggleLoading({
        tag: "GROUP_CREATE",
        show: true,
        msg: "creating group ...",
      });

      resp = await STATE.API.method("forwardToController", FR);
      if (resp?.status === 200) {
        STATE.UpdateGroup(resp.data);
      }
    } catch (error) {
      console.dir(error);
    }

    STATE.toggleLoading(undefined);
    return resp.data;
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
};

export default state;
