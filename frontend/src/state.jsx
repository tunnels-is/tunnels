import { useState } from "react";
import STORE from "./store";
import toast from "react-hot-toast";
import axios from "axios";
import dayjs from "dayjs";
import { v4 as uuidv4 } from "uuid";

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
    borderColor: " border border-[#1e2433] cursor-pointer rounded-md",
    menuBG: " bg-[#0a0d14]",
    mainBG: " bg-[#060810]",
    neutralBtn: " text-white bg-[#4B7BF5] hover:bg-[#5d8af7] hover:text-white cursor-pointer rounded-md",
    successBtn: " text-white bg-emerald-600 hover:bg-emerald-500 hover:text-white cursor-pointer rounded-md",
    warningBtn: " text-white bg-amber-600 hover:bg-amber-500 hover:text-white cursor-pointer rounded-md",
    errorBtn: " text-white bg-red-600 hover:bg-red-500 hover:text-white cursor-pointer rounded-md",
    activeSelect: " bg-[#4B7BF5] text-white cursor-pointer rounded-md",
    neutralSelect: " text-white/80 focus:text-[#4B7BF5] cursor-pointer",
    tabs: "data-[state=active]:text-[#4B7BF5]",
    greenIcon: " text-emerald-500 border-emerald-500 hover:text-white cursor-pointer",
    redIcon: " text-red-600 border-red-600 hover:text-white cursor-pointer",
    badgeNeutral: " bg-[#4B7BF5]/15 text-[#6d9aff] hover:bg-[#4B7BF5]/25 rounded-md",
    badgeSuccess: " bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25 rounded-md",
    badgeWarning: " bg-amber-500/15 text-amber-400 hover:bg-amber-500/25 rounded-md",
    badgeError: " bg-red-500/15 text-red-400 hover:bg-red-500/25 rounded-md",
    toast: " !text-white bg-[#0a0d14] border-[#1e2433]"
  },

  confirmDialog: null,

  // new form
  GetEncType: (int) => {
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
    let resp = await STATE.callController(null, "POST", "/v3/servers", { StartIndex: 0 }, false, false)
    if (resp?.status === 200) {
      if (resp.data?.length > 0) {
        STORE.Cache.SetObject("servers", resp.data);
        STATE.PrivateServers = resp.data;
      } else {
        STATE.toggleError("Unable to find servers");
        STORE.Cache.SetObject("servers", []);
        STATE.PrivateServers = [];
      }
      STATE.renderPage("pservers");
    } else if (resp?.status !== 0) {
      STATE.toggleError("Unable to find servers");
      STORE.Cache.SetObject("servers", []);
      STATE.PrivateServers = [];
    }
  },
  // NEW API
  calls: new Map(),
  callController: async (server, method, route, data, skipAuth, boolResponse) => {
    if (STATE.calls.get(route) === true) {
      console.log("call already in progress, backing off")
      return { status: 0, }
    }
    STATE.calls.set(route, true)

    let SRV = server ? server : STATE.User?.ControlServer
    if (!SRV || SRV === "") {
      console.log("no user auth server found")
      STATE.calls.set(route, false)
      return { status: 0, }
    }
    try {
      STATE.toggleLoading({
        logTag: "",
        tag: uuidv4(),
        show: true,
        msg: SRV.Host + route,
        includeLogs: false,
      });

      if (!skipAuth || skipAuth === false) {
        data.UID = STATE.User?._id ? STATE.User?._id : ""
        // data.Email = STATE.User?.Email ? STATE.User?.Email : ""
        data.DeviceToken = STATE.User?.DeviceToken?.DT ? STATE.User?.DeviceToken?.DT : ""
        if (!data.DeviceToken || data.DeviceToken === "") {
          STATE.toggleError("No auth token found, please log in again");
          STATE.calls.set(route, false)
          return { data: { Error: "Auth token not found, please log in again" }, status: 401 }
        }
        if ((!data.Email || data.Email === "") && (!data.UID || data.UID == "")) {
          STATE.toggleError("No user email/username found, please log in again");
          STATE.calls.set(route, false)
          return { data: { Error: "No user email/username found, please log in again" }, status: 401 }
        }
      }

      let FR = {
        Server: SRV,
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
        STATE.toggleError(error?.response?.data?.Error);
      } else {
        STATE.toggleError("unknown error");
      }

      if (boolResponse === true) {
        return false
      } else {
        return { data: error.response?.data, status: error.response?.status }
      }

    }

  },
  SetUser: (u) => {
    STORE.Cache.SetObject("user", u);
    STATE.User = u;
  },
  GetUsers: async () => {
    try {
      let users = await STATE.LoadUsers();
      console.log("POST FETCH");
      console.dir(users);
      if (users && users.length > 0) {
        STATE.Users = users
      }
      STATE.renderPage("user-select")
      return users;
    } catch (err) {
      console.dir(err);
    }
  },
  v2_SetUser: async (u, saveToDisk, server) => {
    try {
      u.ControlServer = server
      STATE.User = u;
      STORE.Cache.SetObject("user", u);
      STATE.renderPage("root")
      if (saveToDisk) {
        await STATE.SaveUser(u);
      }
    } catch (err) {
      console.dir(err);
    }
  },
  DelUser: async (hash) => {
    try {
      console.log("DELETING USER FROM DISK");
      await STATE.API.method("delUser", { Hash: hash }, true, 10000, false);
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
  LoadUsers: async () => {
    try {
      let resp = await STATE.API.method("getUsers", null, true, 10000, true);
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
  v2_TunnelDelete: async (tun) => {
    try {
      STATE.toggleLoading({
        tag: "tunnel_delete",
        show: true,
        msg: "Deleting tunnel..",
      });

      let resp = await STATE.API.method("deleteTunnel", tun);
      if (resp === undefined) {
        STATE.toggleError("Unknown error, please try again in a moment");
      } else if (resp.status === 200) {
        STATE.Tunnels.forEach((t, i) => {
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
        STATE.toggleError("Unknown error, please try again in a moment");
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
        STATE.toggleError("Unknown error, please try again in a moment");
      } else if (resp.status === 200) {
        ok = true
        STORE.Cache.SetObject("config", newConfig);
        STATE.Config = newConfig;
        STATE.successNotification("Config saved", undefined);
      }
    } catch (error) {
      console.dir(error);
    }
    STATE.ConfigSaveInProgress = false
    STATE.toggleLoading(undefined);
    STATE.globalRerender();
    return ok
  },
  deleteBlocklist: (blocklist) => {
    let newLists = STATE.Config.DNSBlockLists;
    const index = newLists.indexOf(blocklist);

    if (index > -1) {
      newLists.splice(index, 1);
    }

    STATE.Config.DNSBlockLists = newLists;
    STATE.renderPage("dns");
  },
  deleteWhitelist: (whitelist) => {
    let newLists = STATE.Config.DNSWhiteLists;
    const index = newLists.indexOf(whitelist);

    if (index > -1) {
      newLists.splice(index, 1);
    }

    STATE.Config.DNSWhiteLists = newLists;
    STATE.renderPage("dns");
  },
  createTunnel: async () => {
    try {
      let resp = await STATE.API.method("createTunnel");
      if (resp.status != 200) {
        STATE.toggleError(
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
  debug: STORE.Cache.GetBool("debug") === true ? true : false,
  toggleDebug: () => {
    let debug = STORE.Cache.GetBool("debug");
    if (!debug || debug === false) {
      debug = true
    } else {
      debug = false
    }
    STORE.Cache.Set("debug", debug);
    STATE.debug = debug;
    window.location.reload()
  },
  update: undefined,
  updates: {},
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
  successNotification: (e) => {
    toast.success(e);
  },
  User: STORE.Cache.GetObject("user"),
  Users: [],
  Config: STORE.Cache.GetObject("config"),
  refreshApiKey: async () => {
    STATE.User.APIKey = uuidv4();
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

    STATE.renderPage("dns");
  },
  toggleWhitelist: (list) => {
    let found = false;
    STATE.Config?.DNSWhiteLists.forEach((l, i) => {
      if (l.Tag === list.Tag) {
        STATE.Config.DNSWhiteLists[i].Enabled =
          !STATE.Config.DNSWhiteLists[i].Enabled;
        found = true;
      }
    });

    if (!found) {
      STATE.Config.DNSWhiteLists.push(list);
    }

    STATE.renderPage("dns");
  },
  toggleConfigKeyAndSave: (_, key) => {
    if (STATE.ConfigSaveInProgress) {
      return
    }
    try {
      STATE.Config[key] = !STATE.Config[key]
    } catch (error) {
      console.dir(error);
    }
    STATE.v2_ConfigSave()
  },
  ConfirmAndExecute: (type, _id, _duration, title, subtitle, method) => {
    STATE.confirmDialog = {
      open: true,
      title: title || "",
      subtitle: subtitle || "",
      type: type || "success",
      onConfirm: method,
    };
    STATE.renderPage("confirm");
  },
  UpdateUser: async () => {
    try {
      let newUser = STATE.User

      let x = await STATE.callController(null, "POST", "/v3/user/update",
        { APIKey: newUser.APIKey },
        false, true)
      if (x === true) {
        STORE.Cache.SetObject("user", newUser);
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
    if (!server && !c) {
      STATE.toggleError("no server or tunnel given when connecting");
      return;
    }

    let user = STATE.User;
    if (!user.DeviceToken) {
      STATE.toggleError("You are not logged in");
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
      STATE.toggleError("unable to find server with the given ID")
      return
    }

    connectionRequest.Server = STATE.User.ControlServer
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
        STATE.toggleError("Unknown error, please try again in a moment");
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
        STATE.toggleError("Unknown error, please try again in a moment");
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
      STATE.FinalizeLogout();
      return;
    }

    let logoutUser = false;
    if (user.DeviceToken?.DT === token.DT) {
      logoutUser = true;
    }

    let resp = await STATE.callController(null, "POST", "/v3/user/logout",
      { DeviceToken: token.DT, UserID: user._id, All: all },
      false, false)
    if (resp && resp.status === 200) {

      STATE.successNotification("device logged out", undefined);
      if (logoutUser === true || all === true) {
        STATE.DelUser(user);
        STATE.FinalizeLogout();
        return
      } else {
        let toks = [];
        user.Tokens?.forEach((t) => {
          if (t.DT !== token.DT) {
            toks.push(t);
          }
        });
        user.Tokens = toks;
      }

      STORE.Cache.SetObject("user", user);
      STATE.User = user;
    } else {
      STATE.toggleError("Unable to log out device", undefined);
      if (logoutUser === true || all === true) {
        STATE.FinalizeLogout();
      }
    }
    STATE.rerender();
  },

  LicenseKey: "",
  UpdateLicenseInput: (value) => {
    STATE.LicenseKey = value;
    STATE.rerender();
  },
  PrivateServers: STORE.Cache.GetObject("servers"),
  updatePrivateServers: () => {
    STORE.Cache.SetObject("servers", STATE.PrivateServers);
  },
  ActivateLicense: async () => {
    if (!STATE.User) {
      return;
    }

    if (STATE.LicenseKey === "") {
      STATE.toggleError("License key is required");
      return;
    }

    let ok = await STATE.callController(null, "POST", "/v3/key/activate",
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
  isWails: () => {
    let h = window.location.hostname;
    return h === 'wails.localhost' || h === 'wails' || window.location.protocol === 'wails:';
  },
  GetURL: () => {
    if (STATE.isWails()) {
      return "http://127.0.0.1:7777";
    }
    let host = window.location.origin;
    host = host.replace("https://", "http://");
    host = host.replace("5173", "7777");
    host = host.replace("5174", "7777");
    host = host.replace("5175", "7777");
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
          STATE.Tunnels = response.data?.Tunnels;
          STATE.ActiveTunnels = response.data?.ActiveTunnels;
          STATE.Version = response.data?.Version;
          STATE.APIVersion = response.data?.APIVersion;

          STORE.Cache.SetObject("state", STATE.State);
          STORE.Cache.SetObject("config", STATE.Config);
          STATE.renderPage("login")
        }

        STATE.globalRerender();
      }
      STATE.StateFetchInProgress = false;

      return response;
    } catch (error) {
      STATE.StateFetchInProgress = false;
      console.dir(error);
      STATE.toggleError("unable to load state...");
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
          STATE.toggleError(response?.data?.Error);
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
            STATE.toggleError(error?.response?.data?.Message);
          } else if (error?.response?.data?.Error) {
            STATE.toggleError(error?.response?.data?.Error);
          } else if (error?.response?.data?.error) {
            STATE.toggleError(error?.response?.data.error);
          } else {
            console.log(typeof error.response.data);
            console.dir(error.response.data);
            if (typeof error?.response?.data === "string") {
              STATE.toggleError(error?.response?.data);
            } else {
              try {
                let out = "";
                error?.response?.data?.forEach((err) => {
                  out = out + err + ".\n";
                });
                STATE.toggleError(out);
              } catch (error) {
                STATE.toggleError("Unknown error");
              }
            }
          }
        }

        return error?.response;
      }
    },
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

