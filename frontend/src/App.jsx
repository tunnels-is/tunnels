import { HashRouter, Route, Routes } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";

import { Toaster } from "react-hot-toast";

import "./assets/style/app.scss";
import "@fontsource-variable/inter";

import InspectBlocklist from "./App/InspectBlocklist";
import InspectConnection from "./App/InspectConnection";
import ConnectionTable from "./App/ConnectionTable";
import DNSAnswers from "./App/component/DNSAnswers";
import PrivateServers from "./App/PrivateServers";
import ScreenLoader from "./App/ScreenLoader";
import InspectGroup from "./App/InspectGroup";
import Enable2FA from "./App/Enable2FA";
import ServersFull from "./App/ServersFull";
import Settings from "./App/Settings";
import Account from "./App/Account";
import Servers from "./App/Servers";
import Welcome from "./App/Welcome";
import SideBar from "./App/SideBar";
import Login from "./App/Login";
import Org from "./App/Org";
import DNS from "./App/dns";
import DNSRecords from "./App/DNSRecords";

import GLOBAL_STATE from "./state";
import { STATE } from "./state";
import STORE from "./store";
import WS from "./ws";

// Use this to automatically turn on debug
STORE.Cache.Set("debug", true);

const appElement = document.getElementById("app");
const root = createRoot(appElement);

const LaunchApp = () => {
  const state = GLOBAL_STATE("root");

  useEffect(() => {
    state.GetUser();
    state.GetBackendState();
    WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent);
  }, []);

  return (
    <HashRouter>
      <Toaster
        toastOptions={{
          className: "toast",
          position: "top-right",
          success: {
            duration: 5000,
          },
          icon: null,
          error: {
            duration: 5000,
          },
        }}
      />

      <div className="min-h-screen bg-black w-full">
        <SideBar />
        
        {/* Main Content Area */}
        <main className="pl-64">
          <div className="min-h-screen">
            <ScreenLoader />
            <div className="p-6 w-full">
              <Routes>
                {(state.User?.Email === "" || !state.User) && (
                  <>
                    <Route path="/" element={<Login />} />
                    <Route path="login" element={<Login />} />
                    <Route path="settings" element={<Settings />} />
                    <Route path="help" element={<Welcome />} />
                    <Route path="dns" element={<DNS />} />
                    <Route path="dns-records" element={<DNSRecords />} />
                    <Route path="*" element={<Login />} />
                  </>
                )}

                {state.User && (
                  <>
                    <Route path="/" element={<Welcome />} />
                    <Route path="account" element={<Account />} />

                    <Route path="twofactor" element={<Enable2FA />} />
                    <Route path="org" element={<Org />} />

                    <Route path="inspect/group/:id" element={<InspectGroup />} />
                    <Route path="inspect/group" element={<InspectGroup />} />

                    <Route path="tunnels" element={<ConnectionTable />} />
                    <Route
                      path="inspect/connection/:id"
                      element={<InspectConnection />}
                    />
                    <Route path="routing" element={<ConnectionTable />} />
                    <Route path="settings" element={<Settings />} />

                    <Route path="dns" element={<DNS />} />
                    <Route path="dns-records" element={<DNSRecords />} />
                    <Route path="dns/answers/:domain" element={<DNSAnswers />} />

                    <Route path="servers" element={<Servers />} />
                    <Route path="all" element={<ServersFull />} />
                    <Route path="private" element={<PrivateServers />} />

                    <Route path="inspect/blocklist/" element={<InspectBlocklist />} />

                    <Route path="login" element={<Login />} />
                    <Route path="help" element={<Welcome />} />

                    <Route path="*" element={<Servers />} />
                  </>
                )}
              </Routes>
            </div>
          </div>
        </main>
      </div>
    </HashRouter>
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
    STORE.Cache.Clear();
    window.location.reload();
  }

  async quit() {
    STORE.Cache.Clear();
    window.location.reload();
  }

  async ProductionCheck() {
    if (!STATE.debug) {
      window.console.apply = function () {};
      window.console.dir = function () {};
      window.console.log = function () {};
      window.console.info = function () {};
      window.console.warn = function () {};
      window.console.error = function () {};
      window.console.debug = function () {};
    }
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
      <LaunchApp />
    </ErrorBoundary>
  </React.StrictMode>,
);