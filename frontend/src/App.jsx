import { HashRouter, Route, Routes } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { Toaster } from "react-hot-toast";

import "./assets/style/app.scss";
import "@fontsource-variable/inter";

import DNSAnswers from "./App/component/DNSAnswers";
import ServerDevices from "./App/ServerDevices";
import ScreenLoader from "./App/ScreenLoader";
import InspectGroup from "./App/InspectGroup";
import UserSelect from "./App/UserSelect";
import Enable2FA from "./App/Enable2FA";
import Settings from "./App/Settings";
import Devices from "./App/Devices";
import Account from "./App/Account";
import Welcome from "./App/Welcome";
import SideBar from "./App/SideBar";
import GLOBAL_STATE from "./state";
import Groups from "./App/Groups";
import Login from "./App/Login";
import { STATE } from "./state";
import Users from "./App/Users";
import Stats from "./App/Stats";
import Logs from "./App/Logs";
import STORE from "./store";
import Graph from "./App/Graph";
import DNSStats from "./App/DNSStats";
import ConfirmDialog from "./App/ConfirmDialog";
import DNS from "./App/dns";
import WS from "./ws";

const appElement = document.getElementById("app");
const root = createRoot(appElement);

const LaunchApp = () => {
  const state = GLOBAL_STATE("root");


  useEffect(() => {
    state.GetBackendState();
    WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent);
  }, []);

  return (
    <HashRouter>
      {createPortal(
        <Toaster
          toastOptions={{
            className: "toast border !text-white !bg-[#0a0d14] !border-[#1e2433]",
            position: "top-right",
            success: {
              duration: 2000,
            },

            icon: null,
            error: {
              duration: 2000,
            },
          }
          }
        />,
        document.body,
      )}
      <div className="bg-[#060810] w-full min-h-screen">
        <ScreenLoader />
        <SideBar />

        <main className="pl-14 pb-8 min-h-screen">
          <div className="px-6 py-5">
            <Routes>
              {!state.User && (
                <>
                  <Route path="/" element={<Login />} />
                  <Route path="*" element={<UserSelect />} />
                </>
              )}

              {state.User && (
                <>
                  <Route path="/" element={<Graph />} />
                  <Route path="*" element={<Graph />} />

                  <Route path="groups" element={<Groups />} />
                  <Route path="users" element={<Users />} />
                  <Route path="devices" element={<Devices />} />
                  <Route path="groups/:id" element={<InspectGroup />} />

                  <Route path="connections" element={<Stats />} />
                  <Route path="tunnels" element={<Graph />} />
                  <Route path="account" element={<Account />} />

                  <Route path="server/:id" element={<ServerDevices />} />
                </>
              )}
              <Route path="accounts" element={<UserSelect />} />

              <Route path="twofactor/create" element={<Enable2FA />} />

              <Route path="logs" element={<Logs />} />
              <Route path="settings" element={<Settings />} />

              <Route path="dns" element={<DNS />} />
              <Route path="dns/answers/:domain" element={<DNSAnswers />} />
              <Route path="dnsstats" element={<DNSStats />} />

              <Route path="login" element={<Login />} />
              <Route path="login/:modeParam" element={<Login />} />
              <Route path="help" element={<Welcome />} />
            </Routes>
          </div>
        </main>
      </div>
      <ConfirmDialog />
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
      window.console.apply = function() { };
      window.console.dir = function() { };
      window.console.log = function() { };
      window.console.info = function() { };
      window.console.warn = function() { };
      window.console.error = function() { };
      window.console.debug = function() { };
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
