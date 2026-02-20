const DATA = "data_";

var STORE = {
  SupportPlatforms: [
    { type: "email", name: "EMAIL", link: "support@tunnels.is" },
    { type: "link", name: "X", link: "https://www.x.com/tunnels_is" },
    { type: "link", name: "DISCORD", link: "https://discord.gg/2v5zX5cG3j" },
    {
      type: "link",
      name: "REDDIT",
      link: "https://www.reddit.com/r/tunnels_is",
    },
    {
      type: "link",
      name: "SIGNAL",
      link: "https://signal.group/#CjQKIGvNLjUd8o3tkkGUZHuh0gfZqHEsn6rxXOG4S1U7m2lEEhBtuWbyxBjMLM_lo1rVjFX0",
    },
  ],
  debug: false,
  Cache: {
    interface: window.localStorage,
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
    Set: function(key, value) {
      STORE.Cache.interface.setItem(key, value);
    },
    GetObject: function(key) {
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
      } catch (e) {
        if (STORE.debug) {
          console.log("trying to set:", key, object);
          console.log(e);
        }
      }
    },
  },
};

export default STORE;
