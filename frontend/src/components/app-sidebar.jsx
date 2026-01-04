import { useNavigate, useLocation, Link, NavLink } from "react-router-dom";
import React, { useRef } from "react";
import { useAtomValue, useSetAtom, useAtom } from "jotai";
import { userAtom, isAuthenticatedAtom, accountsAtom } from "@/stores/userStore";
import { toast } from "sonner";
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
  Logs,
  PersonStanding,
  Crown,
  Zap
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
import { useTheme } from "@/providers/theme-provider";
import { useSetUser } from "@/hooks/useAuth";

const IconWidth = 20;
const IconHeight = 20;


/**
 * @param {Object} props
 * @param {React.ReactNode} props.Icon
 * @param {string} props.Label
 * @param {string} props.Route
 */
const AppSidebarButton = ({ Icon, Label, Route }) => (
  <SidebarMenuItem>

    <NavLink to={"/" + Route}>
      {
        ({ isActive }) => (
          <SidebarMenuButton isActive={isActive}>
            <Icon
              className={cn(
                "flex-shrink-0",
                isActive && "text-primary"
              )}
              width={IconWidth}
              height={IconHeight}
            />
            <span className={isActive ? "text-primary" : ""}>{Label}</span>
          </SidebarMenuButton>
        )
      }
    </NavLink>
  </SidebarMenuItem>
);

const AppSidebar = () => {
  const navigate = useNavigate();
  const user = useAtomValue(userAtom);
  const isAuthenticated = useAtomValue(isAuthenticatedAtom);
  const setUser = useSetAtom(userAtom);
  const [accounts, setAccounts] = useAtom(accountsAtom);
  const { setTheme } = useTheme();
  const setUserMutation = useSetUser();


  const formatServer = (server) => server.Host + " : " + server.Port;


  const OpenWindowURL = (url) => {
    window.open(url, "_blank");
  };

  const handleLogout = async () => {
    const newAccounts = accounts.filter((u) => u.ID !== user.ID);

    if (user?.DeviceToken) {
      try {
        await logout({ DeviceToken: user.DeviceToken.DT, LogoutToken: user.DeviceToken.DT, UID: user.ID, All: false });
        if (newAccounts.length > 0) {
          await setUserMutation.mutateAsync(newAccounts[0]);
          setUser(newAccounts[0]);
        }
        else {
          await setUserMutation.mutateAsync(null);
          setUser(null);
        }
        setAccounts(newAccounts);
        navigate("/login");
      } catch (e) {
        console.error("Logout failed", e);
        toast.error("Logout failed");
      }
    }

  };

  const handleSwitchAccount = async (account) => {
    setUser(account);
    await setUserMutation.mutateAsync(account);
    navigate("/servers");
  };


  return (
    <Sidebar>
      {!isAuthenticated ? (
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                <AppSidebarButton Icon={Lock} Label="Login" Route="login" />
                <AppSidebarButton Icon={HelpCircle} Label="Help" Route="help" />
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
      ) : (
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                <AppSidebarButton Icon={Shield} Label="VPN" Route="servers" />
                <AppSidebarButton Icon={LinkIcon} Label="Connections" Route="connections" />
                <AppSidebarButton Icon={Container} Label="DNS" Route="dns" />
                <AppSidebarButton Icon={Zap} Label="Tunnels" Route="tunnels" />
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
          {
            user?.IsAdmin && (
              <SidebarGroup>
                <SidebarGroupLabel>Admin</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    <AppSidebarButton Icon={Users} Label="Users" Route="users" />
                    <AppSidebarButton Icon={Monitor} Label="Devices" Route="devices" />
                    <AppSidebarButton Icon={Home} Label="Groups" Route="groups" />
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            )
          }
          <SidebarGroup>
            <SidebarGroupLabel>Settings</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <AppSidebarButton Icon={Settings} Label="Settings" Route="settings" />
                <AppSidebarButton Icon={Logs} Label="Logs" Route="logs" />
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
          <SidebarGroup>
            <SidebarGroupLabel>Support</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <AppSidebarButton Icon={HelpCircle} Label="About" Route="help" />
                <SidebarMenuItem>
                  <SidebarMenuButton onClick={() => OpenWindowURL("https://www.tunnels.is/support")}>
                    <PersonStanding width={IconWidth} height={IconHeight} />
                    Guide
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton onClick={() => OpenWindowURL("https://github.com/tunnels-is/tunnels")}>
                    <Github width={IconWidth} height={IconHeight} />
                    Github
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
      )}
      {isAuthenticated && <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger className="w-full">
                <SidebarMenuButton
                  size="lg"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                    {user.IsAdmin ? <Crown className="size-4" /> : <User className="size-4" />}
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
                          {user.IsAdmin ? <Crown className="size-4" /> : <User className="size-4" />}
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
                  Accounts
                </DropdownMenuLabel>
                {accounts.filter((u) => u.ID !== user?.ID).map((account, idx) => (
                  <DropdownMenuItem key={idx} onClick={() => handleSwitchAccount(account)}>
                    <User className="mr-2 h-4 w-4" />
                    <div className="grid flex-1 text-left text-sm leading-tight">
                      <span className="truncate font-semibold">
                        {account.Email}
                      </span>
                      <span className="truncate text-xs">{formatServer(account.ControlServer)}</span>
                    </div>
                  </DropdownMenuItem>
                ))}
                <DropdownMenuItem asChild>
                  <Link to="/login">
                    <Plus className="h-4 w-4" />
                    <span>Add Account</span>
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuSeparator />

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
                    <DropdownMenuItem asChild>
                      <Link to="/profile">
                        <User className="mr-2 h-4 w-4" />
                        <span className="truncate">Profile</span>
                      </Link>
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

    </Sidebar >
  );
};

export default AppSidebar;
