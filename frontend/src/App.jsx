import { BrowserRouter, Route, Routes, useParams, Navigate } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { Toaster } from "@/components/ui/sonner";

import "./assets/style/main.css";
import "@fontsource-variable/inter";

import ServersPage, { ServerDevices } from "./pages/servers";
import SettingsPage from "./pages/settings";
import DevicesPage from "./pages/devices";
import TunnelsPage from "./pages/tunnels";
import HelpPage from "./pages/help";
import GroupsPage, { InspectGroup } from "./pages/groups";
import LoginPage from "./pages/login";
import StatsPage from "./pages/connections";
import LogsPage from "./pages/logs";
import ProfilePage from "./pages/profile";
import DNSPage, { DNSAnswers } from "./pages/dns";
import UsersPage from "./pages/users";
import { SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "./components/app-sidebar";

import { QueryProvider } from "./providers/query-provider";
import { ThemeProvider } from "./providers/theme-provider";

import { useAtomValue } from "jotai";
import { isAuthenticatedAtom } from "./stores/userStore";

const appElement = document.getElementById("app");
const root = createRoot(appElement);

const LaunchApp = () => {
  // useInitialState();
  const isAuth = useAtomValue(isAuthenticatedAtom);
  return (
    <BrowserRouter>
      {createPortal(
        <Toaster position="bottom-right" />,
        document.body
      )}
      <ThemeProvider>
        <SidebarProvider>
          <AppSidebar />
          <div className="px-4 w-full">
            <Routes>

              {isAuth ?
                (
                  <>
                    <Route path="help" element={<HelpPage />} />

                    <Route path="groups" element={<GroupsPage />} />
                    <Route path="users" element={<UsersPage />} />
                    <Route path="devices" element={<DevicesPage />} />
                    <Route path="groups/:id" element={<InspectGroup />} />

                    <Route path="tunnels" element={<TunnelsPage />} />
                    <Route path="connections" element={<StatsPage />} />

                    <Route path="servers" element={<ServersPage />} />
                    <Route path="server/:id" element={<ServerDevices />} />
                    <Route path="logs" element={<LogsPage />} />
                    <Route path="settings" element={<SettingsPage />} />

                    <Route path="dns" element={<DNSPage />} />
                    <Route path="dns/answers/:domain" element={<DNSAnswers />} />
                    <Route path="profile" element={<ProfilePage />} />
                    <Route path="login" element={<LoginPage />} />
                    <Route path="*" element={<Navigate to="/help" />} />
                  </>
                ) :
                (
                  <>
                    <Route index path="help" element={<HelpPage />} />
                    <Route path="login" element={<LoginPage />} />
                    <Route path="*" element={<Navigate to="/help" />} />
                  </>)
              }
            </Routes>
          </div>


        </SidebarProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
};

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      hasError: false,
      title:
        "Something unexpected happened, please press Reload. If that doesn't work try pressing 'Close And Reset'. If nothing works, please contact customer support",
    };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch() {
    this.state.hasError = true;
  }

  reloadAll() {
    sessionStorage.clear();
    window.location.reload();
  }

  async quit() {
    sessionStorage.clear();
    window.location.reload();
  }

  async ProductionCheck() {
    // if (!STATE.debug) {
    // window.console.apply = function () {};
    // window.console.dir = function () {};
    // window.console.log = function () {};
    // window.console.info = function () {};
    // window.console.warn = function () {};
    // window.console.error = function () {};
    // window.console.debug = function () {};
    // }
  }

  render() {
    this.ProductionCheck();

    if (this.state.hasError) {
      return (
        <>
          <h1 className="exception-title">{this.state.title}</h1>
          <button className="exception-button" onClick={() => this.reloadAll()}>
            Reload
          </button>
        </>
      );
    }

    return this.props.children;
  }
}


root.render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryProvider>
        <LaunchApp />
      </QueryProvider>
    </ErrorBoundary>
  </React.StrictMode>
);
