import dayjs from "dayjs";
import axios from "axios";
import { toast } from "sonner";

export function GetEncType(int) {
  switch (String(int)) {
    case "0":
      return "None";
    case "1":
      return "AES128";
    case "2":
      return "AES256";
    case "3":
      return "CHACHA20";
    default:
      return "unknown";
  }
}

export function formatNodeKey(key, value, pub) {
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
}


export function ActiveRouterSet(state) {
  if (!state) {
    return false;
  } else if (!state.ActiveRouter) {
    return false;
  } else if (state.ActiveRouter.PublicIP === "") {
    return false;
  }
  return true;
}

export function filterRoutersFromState(state) {
  if (!state || !state.Routers) return [];
  const routers = state.Routers.filter((r) => {
    if (r !== null) {
      return true;
    }
    return false;
  });
  return routers;
}


