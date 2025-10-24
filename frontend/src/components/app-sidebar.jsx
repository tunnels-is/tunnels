import { useNavigate, useLocation, Link } from "react-router-dom";
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
  ExitIcon,
} from "@radix-ui/react-icons";
import { cn } from "@/lib/utils";
import GLOBAL_STATE from "@/state";
import { Logs } from "lucide-react";
import { UsersIcon } from "lucide-react";
import {
  Sidebar,
  SidebarFooter,
  SidebarContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenuButton,
  SidebarMenuAction,
} from "@/components/ui/sidebar";

const IconWidth = 20;
const IconHeight = 20;

const AppSidebar = () => {
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

  const logout = () => {
    let t = state.User?.DeviceToken;
    if (t !== "") {
      state.LogoutToken(t, false);
    }
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
  };

  const hasActiveTunnels = () => {
    if (state.ActiveTunnels?.length > 0) {
      return true;
    }
    return false;
  };

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
          { icon: LockClosedIcon, label: "VPN", route: "servers", user: true },
          { icon: ContainerIcon, label: "DNS", route: "dns", user: false },
          {
            icon: MixerHorizontalIcon,
            label: "Connections",
            route: "connections",
            user: true,
            shouldRender: hasActiveTunnels,
          },
        ],
      },
      {
        title: "Admin",
        isManager: true,
        items: [
          {
            icon: PersonIcon,
            label: "Users",
            route: "users",
            user: true,
            shouldRender: isManager,
          },
          {
            icon: DesktopIcon,
            label: "Devices",
            route: "devices",
            user: true,
            shouldRender: isManager,
          },
          {
            icon: HomeIcon,
            label: "Groups",
            route: "groups",
            user: true,
            shouldRender: isManager,
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

          {
            icon: UsersIcon,
            label: "Accounts",
            route: "accounts",
            shouldRender: showLogin,
            user: false,
          },
          { icon: Logs, label: "Logs", route: "logs", user: false },
        ],
      },
      {
        title: "Support",
        items: [
          {
            icon: InfoCircledIcon,
            label: "Support",
            route: "help",
            user: false,
          },
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
          {
            icon: ExitIcon,
            label: "Logout",
            click: logout,
            user: true,
          },
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
    <Sidebar>
      <SidebarContent>
        {menu.groups.map((g) => {
          if (g.user === true && (!user || user.Email === "")) {
            return false;
          }
          if (g.shouldRender && !g.shouldRender()) {
            return false;
          }
          if (g.isManager && !isManager()) {
            return <></>;
          }
          return (
            <SidebarGroup key={g.title}>
              {g.title && (
                <SidebarGroupLabel className="px-3 mb-2">
                  <h2 className="text-xs font-semibold uppercase tracking-wider">
                    {g.title}
                  </h2>
                </SidebarGroupLabel>
              )}

              <SidebarGroupContent>
                <SidebarMenu>
                  {g.items.map((i) => {
                    if (i.user && (!user || user.Email === "")) {
                      return null;
                    }
                    if (i.shouldRender && !i.shouldRender()) {
                      return false;
                    }

                    let isActive = false;
                    if (
                      sp[1].includes(i.route) ||
                      (sp[1] === "" && i.route == "login")
                    ) {
                      isActive = true;
                    }
                    // const isActive = sp[1] === i.route;
                    return (
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          key={i.label}
                          onClick={() => {
                            if (i.click) {
                              i.click();
                            } else {
                              navHandler("/" + i.route);
                            }
                          }}
                          isActive={isActive}
                        >
                          <i.icon
                            className={cn(
                              "flex-shrink-0",
                              isActive && "text-primary"
                            )}
                            width={IconWidth}
                            height={IconHeight}
                          />
                          <span className={isActive && "text-primary"}>{i.label}</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          );
        })}
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild>
              <Link href="/">
                <PersonIcon /> {!user ? "Not logged in" : user.Email}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  );
};

export default AppSidebar;
