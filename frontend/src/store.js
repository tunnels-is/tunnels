import dayjs from "dayjs";
const DATA = "data_";

var STORE = {
  // debug: Boolean(window.localStorage.getItem("debug")),
  ActiveRouterSet(state) {
    if (!state) {
      return false;
    } else if (!state.ActiveRouter) {
      return false;
    } else if (state.ActiveRouter.PublicIP === "") {
      return false;
    }
    return true;
  },
  filterRoutersFromState(state) {
    const routers = state.Routers.filter((r) => {
      if (r !== null) {
        return true;
      }
      return false;
    });
    return routers;
  },

  formatNodeKey(key, value, pub) {
    if (pub === true) {
      switch (key) {
        case "Admin":
          return undefined;
        case "_id":
          return undefined;
        case "Public":
          return undefined;
        case "InternetAccess":
          return undefined;
        case "Country":
          return undefined;
        case "IP":
          return undefined;
        case "EncryptionProtocol":
          return undefined;
        case "Port":
          return undefined;
        case "RouterIP":
          return undefined;
        case "Tag":
          return undefined;
        case "Slots":
          return undefined;
      }
    }

    switch (key) {
      case "Country":
        if (value === "" || value === "icon") {
          return "unknown";
        } else {
          return value;
        }
      case "LastOnline":
        return dayjs(value).format("HH:mm:ss");
      case "Updated":
        return dayjs(value).format("DD/MM/YYYY HH:mm:ss");
      case "Status":
        if (value === 0) {
          return "offline";
        } else {
          return "online";
        }
      default:
        // console.log(key, value)
        return value;
    }
  },

  // debug: Boolean(window.sessionStorage.getItem("debug")) === true ? true : false,
  debug: false,
  // debug: false,
  Session: window.sessionStorage,
  Cache: {
    // interface: window.localStorage,
    interface: window.sessionStorage,
    MEMORY: {
      FetchingState: false,
      DashboardData: undefined,
    },
    Clear: function() {
      return STORE.Cache.interface.clear();
    },
    Get: function(key) {
      let item = STORE.Cache.interface.getItem(key);
      if (item === null) {
        return undefined;
      }
      return item;
    },
    GetBool: function(key) {
      let data = STORE.Cache.interface.getItem(key);
      if (data === null) {
        return undefined;
      }
      if (data === "true") {
        return true;
      }
      return false;
    },
    SetRawData(key, value) {
      STORE.Cache.interface.setItem(DATA + key, value);
    },
    Set: function(key, value) {
      STORE.Cache.interface.setItem(key, value);
    },
    Del: function(key) {
      STORE.Cache.interface.removeItem(key);
    },
    DelObject: function(key) {
      STORE.Cache.interface.removeItem(DATA + key);
      STORE.Cache.interface.removeItem(DATA + key + "_ct");
    },
    GetObject: function(key) {
      // console.trace();
      let jsonData = undefined;
      try {
        let object = STORE.Cache.interface.getItem(DATA + key);
        if (object === "undefined") {
          return undefined;
        }
        if (!object || object === '""') {
          return undefined
        }
        jsonData = JSON.parse(object);
        if (STORE.debug) {
          console.log(
            "%cGET OBJECT:",
            "background: lightgreen; color: black",
            key,
            jsonData,
          );
        }
      } catch (e) {
        if (STORE.debug) {
          console.log("trying to get:", key);
          console.log(e);
        }
        return undefined;
      }

      if (jsonData === null) {
        return undefined;
      }

      return jsonData;
    },
    SetObject: function(key, object) {
      try {
        if (STORE.debug) {
          console.log(
            "%cSET OBJECT:",
            "background: lightgreen; color: black",
            key,
            object,
          );
        }
        STORE.Cache.interface.setItem(DATA + key, JSON.stringify(object));
        STORE.Cache.interface.setItem(DATA + key + "_ct", dayjs().unix());
      } catch (e) {
        if (STORE.debug) {
          console.log("trying to set:", key, object);
          console.log(e);
        }
      }
    },
    GetCatchTimer(key) {
      try {
        let time = STORE.Cache.interface.getItem(DATA + key + "_ct");
        if (time === null) {
          return undefined;
        }
        return time;
      } catch (e) {
        console.log(e);
      }
      return undefined;
    },
  },
};

export default STORE;
