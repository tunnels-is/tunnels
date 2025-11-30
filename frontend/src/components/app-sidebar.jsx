import { useNavigate, useLocation, Link, NavLink } from "react-router-dom";
import React, { useRef } from "react";
import { useAtomValue, useSetAtom, useAtom } from "jotai";
import { userAtom, isAuthenticatedAtom } from "@/stores/userStore";
import { controlServerAtom, controlServersAtom } from "@/stores/configStore";
import { activeTunnelsAtom } from "@/stores/tunnelStore";
import { logout } from "@/api/auth";
import { cn } from "@/lib/utils";
import {
  Home,
  Settings,
  Lock,
  Container,
  LogOut,
  User,
  Users,
  Monitor,
  Link as LinkIcon,
  Shield,
  HelpCircle,
  BookOpen,
  Github,
  ChevronUp,
  ChevronsUpDown,
  Check,
  Server,
  Plus,
  Sun,
  Moon,
  Laptop,
  Logs
} from "lucide-react";
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
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { useTheme } from "@/components/theme-provider";

const IconWidth = 20;
const IconHeight = 20;

const AppSidebar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const sideb = useRef(null);
  const user = useAtomValue(userAtom);
  const isAuthenticated = useAtomValue(isAuthenticatedAtom);
  const setUser = useSetAtom(userAtom);
  const [activeServer, setActiveServer] = useAtom(controlServerAtom);
  const activeTunnels = useAtomValue(activeTunnelsAtom);
  const { setTheme } = useTheme();

  const formatServer = (server) => server.Host + " : " + server.Port;


  const OpenWindowURL = (url) => {
    window.open(url, "_blank");
  };

  const handleLogout = async () => {
    if (user?.DeviceToken) {
      try {
        await logout({ DeviceToken: user.DeviceToken.DT, UserID: user.ID, All: false });
      } catch (e) {
        console.error("Logout failed", e);
      }
    }
    setUser(null);
    navigate("/login");
  };

  const showLogin = () => {
    if (!user || user?.Email === "") {
      return true;
    }
    return false;
  };

  const isManager = () => {
    if (user?.IsAdmin === true || user?.IsManager === true) {
      return true;
    }
    return false;
  };

  const hasActiveTunnels = () => {
    if (activeTunnels?.length > 0) {
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
            icon: Lock,
            label: "Login",
            route: "login",
            user: false,
            shouldRender: showLogin,
          },
          { icon: Shield, label: "VPN", route: "servers", user: true },
          { icon: Container, label: "DNS", route: "dns", user: false },
          {
            icon: LinkIcon,
            label: "Connections",
            route: "connections",
            user: true,
            shouldRender: hasActiveTunnels,
          },
          // {
          //   icon: Users,
          //   label: "Groups",
          //   route: "groups",
          //   user: true,
          //   shouldRender: isManager,
          // }
        ],
      },
      {
        title: "Admin",
        isManager: true,
        items: [
          {
            icon: User,
            label: "Users",
            route: "users",
            user: true,
            shouldRender: isManager,
          },
          {
            icon: Monitor,
            label: "Devices",
            route: "devices",
            user: true,
            shouldRender: isManager,
          },
          {
            icon: Home,
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
          { icon: LinkIcon, label: "Tunnels", route: "tunnels", user: true },
          {
            icon: Settings,
            label: "Application",
            route: "settings",
            user: false,
          },
          {
            icon: Users,
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
            icon: HelpCircle,
            label: "Support",
            route: "help",
            user: false,
          },
          {
            icon: BookOpen,
            label: "Guides",
            route: "guides",
            user: false,
            click: () => OpenWindowURL("https://www.tunnels.is/docs"),
          },
          {
            icon: Github,
            label: "Github",
            route: "github",
            user: false,
            click: () => OpenWindowURL("https://www.github.com/tunnels-is/tunnels"),
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

  console.log(`isAuthenticated: ${isAuthenticated}`);
  console.log(`user: ${user}`)

  return (
    <Sidebar>
      <SidebarContent>
        {
          !user ? (
            <SidebarGroup>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild>
                      <Link to="/login"><Lock />Login</Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild>
                      <Link to="/help"><HelpCircle />Help</Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ) : (
            menu.groups.map((g) => {
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
                        return (
                          <SidebarMenuItem key={i.label}>
                            <SidebarMenuButton
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
                              <span className={isActive ? "text-primary" : ""}>{i.label}</span>
                            </SidebarMenuButton>
                          </SidebarMenuItem>
                        );
                      })}
                    </SidebarMenu>
                  </SidebarGroupContent>
                </SidebarGroup>
              );
            })
          )
        }
      </SidebarContent>
      {user && <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger className="w-full">
                <SidebarMenuButton
                  size="lg"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                    <User className="size-4" />
                  </div>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-semibold">
                      {user.Email}
                    </span>
                    <span className="truncate text-xs">
                      {formatServer(user.ControlServer)}
                    </span>
                  </div>
                  <ChevronsUpDown className="ml-auto size-4" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
                side="bottom"
                align="end"
                sideOffset={4}
              >
                {user && (
                  <>
                    <DropdownMenuLabel className="p-0 font-normal">
                      <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                          <User className="size-4" />
                        </div>
                        <div className="grid flex-1 text-left text-sm leading-tight">
                          <span className="truncate font-semibold">
                            {user.Email}
                          </span>
                          <span className="truncate text-xs">{formatServer(user.ControlServer)}</span>
                        </div>
                      </div>
                    </DropdownMenuLabel>
                    <DropdownMenuSeparator />
                  </>
                )}

                <DropdownMenuLabel className="text-xs text-muted-foreground">
                  Theme
                </DropdownMenuLabel>
                <DropdownMenuItem onClick={() => setTheme("light")}>
                  <Sun className="mr-2 h-4 w-4" />
                  <span>Light</span>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTheme("dark")}>
                  <Moon className="mr-2 h-4 w-4" />
                  <span>Dark</span>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTheme("system")}>
                  <Laptop className="mr-2 h-4 w-4" />
                  <span>System</span>
                </DropdownMenuItem>
                <DropdownMenuSeparator />

                {user && (
                  <>
                    <DropdownMenuItem>
                      <User className="mr-2 h-4 w-4" />
                      <span className="truncate">Profile</span>
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={handleLogout}>
                      <LogOut className="mr-2 h-4 w-4" />
                      <span>Logout</span>
                    </DropdownMenuItem>
                  </>
                )}
                {!user && (
                  <DropdownMenuItem onClick={() => navigate("/login")}>
                    <Lock className="mr-2 h-4 w-4" />
                    <span>Login</span>
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>}

    </Sidebar>
  );
};

export default AppSidebar;
