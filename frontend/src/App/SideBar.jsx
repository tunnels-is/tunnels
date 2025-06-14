import { useNavigate, useLocation } from "react-router-dom";
import React, { useRef } from "react";
import {
  AccessibilityIcon,
  GearIcon,
  HomeIcon,
  InfoCircledIcon,
  LockOpen1Icon,
  PersonIcon,
  Share1Icon,
  GitHubLogoIcon,
  LockClosedIcon,
  ContainerIcon,
  MixerHorizontalIcon,
  DesktopIcon,
} from "@radix-ui/react-icons";
import { cn } from "@/lib/utils";
import GLOBAL_STATE from "../state";
import { Logs } from "lucide-react";

const IconWidth = 20;
const IconHeight = 20;

const SideBar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const sideb = useRef(null);
  const state = GLOBAL_STATE("sidebar");

  const OpenWindowURL = (url) => {
    window.open(url, "_blank");
    if (navigator?.clipboard) {
      navigator.clipboard.writeText(value);
    }
    // try {
    //   state.ConfirmAndExecute(
    //     "",
    //     "clipboardCopy",
    //     10000,
    //     url,
    //     "Copy link to clipboard ?",
    //     () => {
    //       if (navigator?.clipboard) {
    //         navigator.clipboard.writeText(value);
    //       }
    //       runtime.ClipboardSetText(url);
    //     },
    //   );
    // } catch (e) {
    //   console.log(e);
    // }
  };

  const showLogin = () => {
    if (!state.User || state.User?.Email === "") {
      return true;
    }
    return false;
  };
  const isManager = () => {
    if (state.User?.IsAdmin === true || state.User?.IsManager === true) {
      return true;
    }
    return false;
  }

  const hasActiveTunnels = () => {
    if (state.ActiveTunnels?.length > 0) {
      return true
    }
    return false
  }

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
        title: "",
        user: false,
        items: [
          { icon: LockClosedIcon, label: "VPN", route: "servers", user: true, },
          { icon: ContainerIcon, label: "DNS", route: "dns", user: false },
          { icon: MixerHorizontalIcon, label: "Connections", route: "connections", user: true, shouldRender: hasActiveTunnels },
        ],
      },
      {
        title: "Admin",
        isManager: true,
        items: [
          { icon: PersonIcon, label: "Users", route: "users", user: true, shouldRender: isManager },
          { icon: DesktopIcon, label: "Devices", route: "devices", user: true, shouldRender: isManager },
          { icon: HomeIcon, label: "Groups", route: "groups", user: true, shouldRender: isManager },
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

          { icon: PersonIcon, label: "Account", route: "account", user: true },
          { icon: Logs, label: "Logs", route: "logs", user: true },
        ],
      },
      {
        title: "Support",
        items: [
          { icon: InfoCircledIcon, label: "Support", route: "help", user: false },
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
    <div
      className={"fixed top-0 left-0 w-44 h-screen bg-[#0B0E14] border-r flex flex-col py-6 z-[2000]" + state.Theme?.borderColor}
      ref={sideb}
      id="sidebar"
    >
      {/* Logo or Brand */}
      < div className="flex-1 overflow-y-auto space-y-6" >
        {
          menu.groups.map((g) => {
            if (g.user === true && (!user || user.Email === "")) {
              return false;
            }
            if (g.shouldRender && !g.shouldRender()) {
              return false;
            }
            if (g.isManager && !isManager()) {
              return (<></>)
            }
            return (
              <div className="px-3" key={g.title}>
                {g.title && (
                  <div className="px-3 mb-2">
                    <h2 className="text-xs font-semibold text-white/40 uppercase tracking-wider">
                      {g.title}
                    </h2>
                  </div>
                )}

                <div className="space-y-1">
                  {g.items.map((i) => {
                    if (i.user && (!user || user.Email === "")) {
                      return null;
                    }
                    if (i.shouldRender && !i.shouldRender()) {
                      return false;
                    }

                    let isActive = false
                    if (sp[1].includes(i.route)) {
                      isActive = true

                    }
                    // const isActive = sp[1] === i.route;

                    return (
                      <button
                        key={i.label}
                        onClick={() => {
                          if (i.click) {
                            i.click();
                          } else {
                            navHandler("/" + i.route);
                          }
                        }}
                        className={cn(
                          "flex items-center w-full gap-3 px-5 py-1 rounded-md text-sm font-medium transition-colors",
                          isActive
                            ? "bg-[#4B7BF5]/10 text-[#4B7BF5]"
                            : "text-white/70 hover:text-white hover:bg-white/5"
                        )}
                      >
                        <i.icon
                          className={cn(
                            "flex-shrink-0",
                            isActive ? "text-[#4B7BF5]" : "text-white/70"
                          )}
                          width={IconWidth}
                          height={IconHeight}
                        />
                        <span>{i.label}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })
        }
      </div >

      {/* User Section at Bottom */}
      {
        user && user.Email && (
          <div className="px-3 pt-6 border-t border-[#1a1f2d]">
            <div className="flex items-center gap-3 px-3 py-3 rounded-md bg-[#1a1f2d]/50">
              <div className="w-8 h-8 rounded-full bg-[#4B7BF5]/20 flex items-center justify-center">
                <PersonIcon className="w-4 h-4 text-[#4B7BF5]" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-white truncate">
                  {user.Email}
                </p>
              </div>
            </div>
          </div>
        )
      }
    </div >
  );
};

export default SideBar;
