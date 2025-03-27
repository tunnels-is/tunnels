import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import FormKeyValue from "./component/formkeyvalue";
import KeyValue from "./component/keyvalue";
import CustomToggle from "./component/CustomToggle";
import FormKeyInput from "./component/formkeyrawvalue";
import STORE from "../store";

const Settings = () => {
  const state = GLOBAL_STATE("settings");

  let DebugLogging = state.getKey("Config", "DebugLogging");
  let ErrorLogging = state.getKey("Config", "ErrorLogging");
  let ConnectionTracer = state.getKey("Config", "ConnectionTracer");
  let InfoLogging = state.getKey("Config", "InfoLogging");

  let DarkMode = state.getKey("Config", "DarkMode");

  let APICertDomains = state.getKey("Config", "APICertDomains");
  let APICertIPs = state.getKey("Config", "APICertIPs");
  let APICert = state.getKey("Config", "APICert");
  let APIKey = state.getKey("Config", "APIKey");
  let APIIP = state.getKey("Config", "APIIP");
  let APIPort = state.getKey("Config", "APIPort");

  let modified = STORE.Cache.GetBool("modified_Config");

  useEffect(() => {
    state.GetBackendState();
  }, []);

  let basePath = state.State?.BasePath;
  let logPath = "";
  let tracePath = "";
  let logFileName = state.State?.LogFileName?.replace(state.State?.LogPath, "");
  let traceFileName = state.State?.TraceFileName?.replace(
    state.State?.TracePath,
    "",
  );
  let configPath = state.State?.ConfigFileName;
  if (state.State?.LogPath !== basePath) {
    logPath = state.State?.LogPath;
  }
  if (state.State?.TracePath !== basePath) {
    tracePath = state.State?.TracePath;
  }
  let version = state.Version ? state.Version : "unknown";
  let apiversion = state.APIVersion ? state.APIVersion : "unknown";

  return (
    <div className="settings-wrapper">
      {modified === true && (
        <div className="save-banner">
          <div className="button" onClick={() => state.v2_ConfigSave()}>
            Save
          </div>
          <div className="notice">Your config has un-saved changes</div>
        </div>
      )}

      <div className="general panel">
        <div className="title">General Settings</div>

        <CustomToggle
          label="Basic Logging"
          value={InfoLogging}
          toggle={() => {
            state.toggleKeyAndReloadDom("Config", "InfoLogging");
            state.renderPage("settings");
          }}
        />

        <CustomToggle
          label="Error Logging"
          value={ErrorLogging}
          toggle={() => {
            state.toggleKeyAndReloadDom("Config", "ErrorLogging");
            state.renderPage("settings");
          }}
        />

        <CustomToggle
          label="Debug Logging"
          value={DebugLogging}
          toggle={() => {
            state.toggleKeyAndReloadDom("Config", "DebugLogging");
            state.renderPage("settings");
          }}
        />

        <CustomToggle
          label="Debug Mode"
          value={state?.debug}
          toggle={() => {
            state.toggleDebug();
            state.renderPage("settings");
          }}
        />

        <CustomToggle
          label={"Tracing"}
          value={ConnectionTracer}
          toggle={() => {
            state.toggleKeyAndReloadDom("Config", "ConnectionTracer");
            state.renderPage("settings");
          }}
        />
      </div>

      <div className="advanced panel">
        <div className="title">Advanced Settings</div>

        <FormKeyValue
          label={"API IP"}
          value={
            <input
              value={APIIP}
              onChange={(e) => {
                state.setKeyAndReloadDom("Config", "APIIP", e.target.value);

                state.renderPage("settings");
              }}
              type="text"
            />
          }
        />

        <FormKeyValue
          label={"API Port"}
          value={
            <input
              value={APIPort}
              onChange={(e) => {
                state.setKeyAndReloadDom("Config", "APIPort", e.target.value);

                state.renderPage("settings");
              }}
              type="text"
            />
          }
        />

        <FormKeyInput
          label={"API Cert Domains"}
          type="text"
          value={APICertDomains}
          onChange={(e) => {
            state.setArrayAndReloadDom(
              "Config",
              "APICertDomains",
              e.target.value,
            );
            state.renderPage("settings");
          }}
        />

        <FormKeyInput
          label={"API Cert IPs"}
          type="text"
          value={APICertIPs}
          onChange={(e) => {
            state.setArrayAndReloadDom("Config", "APICertIPs", e.target.value);
            state.renderPage("settings");
          }}
        />

        <FormKeyValue
          label={"API Cert"}
          value={
            <input
              value={APICert}
              onChange={(e) => {
                state.setKeyAndReloadDom("Config", "APICert", e.target.value);

                state.renderPage("settings");
              }}
              type="text"
            />
          }
        />

        <FormKeyValue
          label={"API Key"}
          value={
            <input
              value={APIKey}
              onChange={(e) => {
                state.setKeyAndReloadDom("Config", "APIKey", e.target.value);
                state.renderPage("settings");
              }}
              type="text"
            />
          }
        />
      </div>

      <div className="net-state panel">
        <div className="title">Default Network </div>

        <KeyValue
          label="Interface"
          defaultValue={"Unknown"}
          value={state.Network?.DefaultInterfaceName}
        />
        <KeyValue
          label="IP"
          defaultValue={"Unknown"}
          value={state.Network?.DefaultInterface}
        />
        <KeyValue
          label="ID"
          defaultValue={"Unknown"}
          value={state.Network?.DefaultInterfaceID}
        />
        <KeyValue
          label="Gateway"
          defaultValue={"Unknown"}
          value={state.Network?.DefaultGateway}
        />
      </div>

      <div className="state panel">
        <div className="title">Application State</div>

        <KeyValue label="API Version" value={apiversion} />
        <KeyValue label="Version" value={version} />
        <KeyValue label="Base Path" value={basePath} />
        <KeyValue label="Config File" value={configPath} />
        <KeyValue label="Log Path" value={logPath} />
        <KeyValue label="Log File" value={logFileName} />
        <KeyValue label="Trace Path" value={tracePath} />
        <KeyValue label="Trace File" value={traceFileName} />
        <KeyValue label="Admin" value={state.State?.IsAdmin} />
      </div>
    </div>
  );
};

export default Settings;
