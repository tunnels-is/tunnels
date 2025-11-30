import { BrowserRouter, Route, Routes, useParams } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { Toaster } from "@/components/ui/sonner";

import "./assets/style/main.css";
import "@fontsource-variable/inter";

import DNSAnswers from "./components/DNSAnswers";
import PrivateServers from "./pages/PrivateServers";
import ServerDevices from "./pages/ServerDevices";
import ScreenLoader from "./pages/ScreenLoader";
import InspectGroup from "./pages/InspectGroup";
import UserSelect from "./pages/UserSelect";
import Enable2FA from "./pages/Enable2FA";
import Settings from "./pages/Settings";
import Devices from "./pages/Devices";
import Tunnels from "./pages/Tunnels";
import Account from "./pages/Account";
import Welcome from "./pages/Welcome";
import Groups from "./pages/Groups";
import Login from "./pages/Login";
import Users from "./pages/Users";
import Stats from "./pages/Stats";
import Logs from "./pages/Logs";

import DNS from "./pages/dns";
import WS from "./ws";
import { SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "./components/app-sidebar";
import { ModeToggle } from "./components/mode-toggle";
import { ThemeProvider } from "./components/theme-provider";

import { useInitialState } from "./hooks/useInitialState";
import { useAtomValue } from "jotai";
import { isAuthenticatedAtom, userAtom } from "./stores/userStore";

const appElement = document.getElementById("app");
const root = createRoot(appElement);

const LaunchApp = () => {
  useInitialState();
  const user = useAtomValue(userAtom);
  const isAuth = useAtomValue(isAuthenticatedAtom);
  return (
    <BrowserRouter>
      {createPortal(
        <Toaster position="bottom-right" theme="light" />,
        document.body
      )}
      <ThemeProvider>
        <ModeToggle />
        <SidebarProvider>
          <ScreenLoader />
          <AppSidebar />

          <Routes>
            <Route path="help" element={<Welcome />} />
            {!isAuth ? (
              <>
                <Route path="*" element={<Login />} />
              </>
            ) : (
              <>
                <Route path="/" element={<Welcome />} />

                <Route path="groups" element={<Groups />} />
                <Route path="users" element={<Users />} />
                <Route path="devices" element={<Devices />} />
                <Route path="groups/:id" element={<InspectGroup />} />

                <Route path="tunnels" element={<Tunnels />} />
                <Route path="connections" element={<Stats />} />
                <Route path="account" element={<Account />} />

                <Route path="servers" element={<PrivateServers />} />
                <Route path="twofactor/create" element={<Enable2FA />} />
                <Route path="server/:id" element={<ServerDevices />} />
                <Route path="logs" element={<Logs />} />
                <Route path="settings" element={<Settings />} />

                <Route path="dns" element={<DNS />} />
                <Route path="dns/answers/:domain" element={<DNSAnswers />} />
                <Route path="accounts" element={<UserSelect />} />
              </>
            )}
          </Routes>
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

import { QueryProvider } from "./providers/QueryProvider";
import NotFound from "./pages/not-found";

// ... existing imports

root.render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryProvider>
        <LaunchApp />
      </QueryProvider>
    </ErrorBoundary>
  </React.StrictMode>
);
