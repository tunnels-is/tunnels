import { useNavigate, useLocation } from "react-router-dom";
import React, { useEffect, useRef } from "react";

import {
  AccessibilityIcon,
  GearIcon,
  GlobeIcon,
  HomeIcon,
  InfoCircledIcon,
  LayersIcon,
  LockOpen1Icon,
  MobileIcon,
  PersonIcon,
  Share1Icon,
  GitHubLogoIcon,
  DimensionsIcon,
} from "@radix-ui/react-icons";

import GLOBAL_STATE from "../state";
import dayjs from "dayjs";

const IconWidth = 23;
const IconHeight = 23;
import * as runtime from "../../wailsjs/runtime/runtime";

const SideBar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const sideb = useRef(null);

  const captureM = (e) => {
    // if (e.keyCode === 77){
    // 	console.dir(e)
    // 	let removed = false
    // 	sideb?.current?.classList?.forEach(c => {
    // 		if (c==="showsidebar"){
    // 			sideb.current.classList.remove("showsidebar")
    // 			removed = true
    // 		}
    // 	})
    // 	if (!removed){
    // 			sideb.current.classList.add("showsidebar")
    //
    // 	}
    // }
  };

  const state = GLOBAL_STATE("sidebar");

  useEffect(() => {
    document.removeEventListener("keydown", captureM);
    document.addEventListener("keydown", captureM);
  }, []);

  const OpenWindowURL = (url) => {
    window.open(url, "_blank");
    try {
      state.ConfirmAndExecute(
        "",
        "clipboardCopy",
        10000,
        url,
        "Copy link to clipboard ?",
        () => {
          if (navigator?.clipboard) {
            navigator.clipboard.writeText(value);
          }
          runtime.ClipboardSetText(url);
        },
      );
    } catch (e) {
      console.log(e);
    }
  };

  const showLogin = () => {
    if (!state.User || state.User?.Email === "") {
      return true;
    }
    return false;
  };

  const menu = {
    groups: [
      {
        title: "",
        user: false,
        shouldRender: showLogin,
        items: [
          {
            icon: LockOpen1Icon,
            label: "Login",
            route: "login",
          },
        ],
      },
      {
        title: "Servers",
        user: true,
        items: [
          { icon: GlobeIcon, label: "Public", route: "public", user: true },
          { icon: MobileIcon, label: "Private", route: "private", user: true },
        ],
      },
      {
        title: "DNS",
        items: [
          { icon: DimensionsIcon, label: "Proxy", route: "dns", user: false },
          {
            icon: LayersIcon,
            label: "Records",
            route: "dns-records",
            user: false,
          },
        ],
      },
      {
        title: "Settings",
        items: [
          { icon: Share1Icon, label: "Tunnels", route: "tunnels", user: true },
          {
            icon: GearIcon,
            label: "Application",
            route: "settings",
            user: false,
          },

          { icon: HomeIcon, label: "Organization", route: "org", user: true },
          { icon: PersonIcon, label: "Account", route: "account", user: true },
        ],
      },
      {
        title: "Support",
        items: [
          { icon: InfoCircledIcon, label: "Chat", route: "help", user: false },
          {
            icon: AccessibilityIcon,
            label: "Guides",
            route: "guides",
            user: false,

            click: () => OpenWindowURL("https://www.tunnels.is/docs"),
          },
          {
            icon: GitHubLogoIcon,
            label: "Github",
            route: "github",
            user: false,

            click: () =>
              OpenWindowURL("https://www.github.com/tunnels-is/tunnels"),
          },
          // { icon: Share1Icon, label: "Logs", route: "logs", user: false, advanced: false },
        ],
      },
    ],
  };

  let { pathname } = location;
  let sp = pathname.split("/");

  const navHandler = (path) => {
    console.log("navigating to:", path);
    navigate(path);
  };

  let user = state.User;

  return (
    <div className="ab sidebar" ref={sideb} id="sidebar">
      {menu.groups.map((g) => {
        if (g.user === true && (!user || user.Email === "")) {
          return false;
        }
        if (g.shouldRender && !g.shouldRender()) {
          return false;
        }
        return (
          <div className="ab group" key={g.title}>
            <div className="ab title">{g.title}</div>

            {g.items.map((i) => {
              if (i.user && (!user || user.Email === "")) {
                return;
              }
              if (i.shouldRender && !i.shouldRender()) {
                return false;
              }
              return (
                <div
                  className="ab item"
                  key={i.label}
                  onClick={() => {
                    if (i.click) {
                      i.click();
                    } else {
                      navHandler("/" + i.route);
                    }
                  }}
                >
                  <i.icon
                    className="ab icon"
                    width={IconWidth}
                    height={IconHeight}
                  />
                  <div
                    className={`${sp[1] === i.route ? "active" : ""} ab text `}
                  >
                    {i.label}
                  </div>
                </div>
              );
            })}
          </div>
        );
      })}
    </div>
  );
};

export default SideBar;
