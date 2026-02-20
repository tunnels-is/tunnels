import { useNavigate, useLocation } from "react-router-dom";
import React from "react";
import {
  GearIcon,
  HomeIcon,
  InfoCircledIcon,
  LockOpen1Icon,
  PersonIcon,
  ContainerIcon,
  MixerHorizontalIcon,
  DesktopIcon,
} from "@radix-ui/react-icons";
import { cn } from "@/lib/utils";
import GLOBAL_STATE from "../state";
import logoSvg from "../assets/images/fav/logo.svg";
import { Logs, Network, BarChart3 } from "lucide-react";
import { UsersIcon } from "lucide-react";

const SideBar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const state = GLOBAL_STATE("sidebar");

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
        items: [
          {
            icon: LockOpen1Icon,
            label: "Login",
            route: "login",
            user: false,
            shouldRender: showLogin,
          },
          { icon: Network, label: "Tunnels", route: "tunnels", user: true },
          { icon: MixerHorizontalIcon, label: "Connections", route: "connections", user: true, shouldRender: hasActiveTunnels },
        ],
      },
      {
        title: "DNS",
        items: [
          { icon: ContainerIcon, label: "Settings", route: "dns", user: false },
          { icon: BarChart3, label: "Stats", route: "dnsstats", user: false },
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
          {
            icon: GearIcon,
            label: "Settings",
            route: "settings",
            user: false,
          },

          { icon: UsersIcon, label: "Accounts", route: "accounts", shouldRender: showLogin, user: false },
          { icon: Logs, label: "Logs", route: "logs", user: false },
          { icon: InfoCircledIcon, label: "Support", route: "help", user: false },
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
      className="group/sidebar fixed top-0 left-0 w-14 hover:w-[200px] h-screen bg-[#0a0d14] border-r border-[#1e2433] flex flex-col z-[2000] transition-all duration-200 overflow-hidden"
      id="sidebar"
    >
      {/* Brand */}
      <div className="h-12 flex items-center px-[16px] border-b border-[#1e2433] shrink-0">
        <img src={logoSvg} alt="Tunnels" className="w-6 h-6 shrink-0" />
        <span className="ml-3 text-[13px] font-semibold text-white opacity-0 group-hover/sidebar:opacity-100 transition-opacity duration-200 whitespace-nowrap">
          Tunnels
        </span>
      </div>

      <div className="flex-1 overflow-y-auto py-3 space-y-3">
        {
          menu.groups.map((g) => {
            if (g.user === true && (!user || user.Email === "")) {
              return false;
            }
            if (g.shouldRender && !g.shouldRender()) {
              return false;
            }
            if (g.isManager && !isManager()) {
              return null
            }
            return (
              <div key={g.title}>
                {g.title && (
                  <div className="px-[20px] mb-1 overflow-hidden">
                    <h2 className="text-[11px] font-semibold text-white/45 uppercase tracking-widest whitespace-nowrap opacity-0 group-hover/sidebar:opacity-100 transition-opacity duration-200">
                      {g.title}
                    </h2>
                  </div>
                )}

                <div className="space-y-0.5">
                  {g.items.map((i) => {
                    if (i.user && (!user || user.Email === "")) {
                      return null;
                    }
                    if (i.shouldRender && !i.shouldRender()) {
                      return false;
                    }

                    let isActive = false
                    if (sp[1].includes(i.route) || (sp[1] === "" && i.route === "login") || (sp[1] === "" && i.route === "tunnels")) {
                      isActive = true
                    }

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
                          "flex items-center w-full gap-3 px-[20px] py-1.5 rounded-md text-[13px] font-medium transition-colors overflow-hidden",
                          isActive
                            ? "bg-[#4B7BF5]/10 text-[#4B7BF5]"
                            : "text-white/50 hover:text-white/80 hover:bg-white/[0.03]"
                        )}
                      >
                        <i.icon
                          className={cn(
                            "shrink-0",
                            isActive ? "text-[#4B7BF5]" : "text-white/40"
                          )}
                          width={16}
                          height={16}
                        />
                        <span className="whitespace-nowrap opacity-0 group-hover/sidebar:opacity-100 transition-opacity duration-200">
                          {i.label}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })
        }
      </div>

      {
        user && user.Email && (
          <div className="pb-2 pt-3 cursor-pointer shrink-0 border-t border-[#1e2433]" onClick={() => navigate("/account")}>
            <div className="flex items-center px-[14px] py-1.5 rounded-md hover:bg-white/[0.03] transition-colors overflow-hidden">
              <div className="w-7 h-7 rounded-full bg-[#4B7BF5]/15 flex items-center justify-center shrink-0">
                <PersonIcon className="w-3.5 h-3.5 text-[#4B7BF5]" />
              </div>
              <div className="flex-1 min-w-0 ml-2 opacity-0 group-hover/sidebar:opacity-100 transition-opacity duration-200">
                <div className="text-xs font-medium text-white/70 truncate">
                  {user.Email}
                </div>
              </div>
            </div>
          </div>
        )
      }

    </div>
  );
};

export default SideBar;
