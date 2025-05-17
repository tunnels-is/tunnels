import { HashRouter, Route, Routes } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";
import { createPortal } from "react-dom";
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
import Settings from "./App/Settings";
import Account from "./App/Account";
import Servers from "./App/Servers";
import Welcome from "./App/Welcome";
import SideBar from "./App/SideBar";
import Login from "./App/Login";
import DNS from "./App/dns";

import GLOBAL_STATE from "./state";
import { STATE } from "./state";
import STORE from "./store";
import WS from "./ws";
import Groups from "./App/Groups";
import Users from "./App/Users";
import Devices from "./App/Devices";
import NewObjectEditor from "./App/NewObjectEdior";
import Tunnels from "./App/Tunnels";
import Logs from "./App/Logs";

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
      {createPortal(
        <Toaster
          toastOptions={{
            className: "toast bg-black !text-white border border-[#2056e1] rounded-none",
            position: "top-right",
            success: {
              duration: 5000,
            },

            icon: null,
            error: {
              duration: 5000,
            },
          }}
        />,
        document.body,
      )}
      <div className=" bg-black w-full">
        <ScreenLoader />
        <SideBar />

        {/* Main Content Area */}
        <main className="pl-44 pb-[300px]">
          <div className="">
            <div className="p-6 w-full">
              <Routes>
                <Route path="account" element={<Account />} />

                <Route path="twofactor/create" element={<Enable2FA />} />

                <Route path="groups" element={<Groups />} />
                <Route path="users" element={<Users />} />
                <Route path="devices" element={<Devices />} />

                <Route path="inspect/group/:id" element={<InspectGroup />} />

                <Route path="tunnels" element={<Tunnels />} />
                <Route path="logs" element={<Logs />} />
                <Route
                  path="inspect/connection/:id"
                  element={<InspectConnection />}
                />
                <Route path="routing" element={<ConnectionTable />} />
                <Route path="settings" element={<Settings />} />

                <Route path="dns" element={<DNS />} />
                <Route path="dns/answers/:domain" element={<DNSAnswers />} />

                <Route path="servers" element={<PrivateServers />} />
                <Route path="all" element={<PrivateServers />} />
                <Route path="private" element={<PrivateServers />} />

                <Route
                  path="inspect/blocklist/"
                  element={<InspectBlocklist />}
                />

                <Route path="login" element={<Login />} />
                <Route path="help" element={<Welcome />} />

                <Route path="test" element={<NewObjectEditor />} />

                {state.User && (
                  <>
                    <Route path="/" element={<Welcome />} />
                    <Route path="*" element={<Servers />} />
                  </>
                )}

                {!state.User && (
                  <>
                    <Route path="/" element={<Login />} />
                    <Route path="*" element={<Login />} />
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
