import { BrowserRouter, Route, Routes } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { Toaster } from "react-hot-toast";

import "./assets/style/app.scss";
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
import SideBar from "./pages/SideBar";
import GLOBAL_STATE from "./state";
import Groups from "./pages/Groups";
import Login from "./pages/Login";
import { STATE } from "./state";
import Users from "./pages/Users";
import Stats from "./pages/Stats";
import Logs from "./pages/Logs";
import STORE from "./store";
import DNS from "./pages/dns";
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
    <BrowserRouter>
      {createPortal(
        <Toaster
          toastOptions={{
            className: "toast border-[2px] p-6 !text-white !bg-[#0B0E14] !border-[#1a1f2d]",
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
      <div className=" bg-black w-full">
        <ScreenLoader />
        <SideBar />

        <main className="pl-44 pb-[300px]">
          <div className="">
            <div className="p-6 w-full">
              <Routes>
                {!state.User && (
                  <>
                    <Route path="/" element={<Login />} />
                    <Route path="*" element={<UserSelect />} />
                  </>
                )}

                {state.User && (
                  <>
                    <Route path="/" element={<Welcome />} />
                    <Route path="*" element={<PrivateServers />} />

                    <Route path="groups" element={<Groups />} />
                    <Route path="users" element={<Users />} />
                    <Route path="devices" element={<Devices />} />
                    <Route path="groups/:id" element={<InspectGroup />} />

                    <Route path="tunnels" element={<Tunnels />} />
                    <Route path="connections" element={<Stats />} />
                    <Route path="account" element={<Account />} />

                    <Route path="servers" element={<PrivateServers />} />
                    <Route path="server/:id" element={<ServerDevices />} />

                  </>
                )}
                <Route path="accounts" element={<UserSelect />} />


                <Route path="twofactor/create" element={<Enable2FA />} />

                <Route path="logs" element={<Logs />} />
                <Route path="settings" element={<Settings />} />

                <Route path="dns" element={<DNS />} />
                <Route path="dns/answers/:domain" element={<DNSAnswers />} />

                <Route path="login" element={<Login />} />
                <Route path="login/:modeParam" element={<Login />} />
                <Route path="help" element={<Welcome />} />

              </Routes>
            </div>
          </div>
        </main>
      </div>
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
