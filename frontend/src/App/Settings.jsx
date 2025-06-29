import React, { useEffect } from "react";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import {
  Card,
  CardContent,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import {
  Info,
  Bug,
  AlertTriangle,
  Activity,
  Server,
  Globe,
  Key,
  Network,
} from "lucide-react";
import InfoItem from "./component/InfoItem";

const Settings = () => {
  const state = GLOBAL_STATE("settings");

  let DebugLogging = state.getKey("Config", "DebugLogging");
  let ConsoleLogging = state.getKey("Config", "ConsoleLogging");
  let ErrorLogging = state.getKey("Config", "ErrorLogging");
  let InfoLogging = state.getKey("Config", "InfoLogging");
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

  const SettingToggle = ({ label, icon, value, onToggle, description }) => (
    <div className="flex items-center justify-between py-3">
      <div className="flex items-start gap-3">
        {icon}
        <div className="space-y-0.5">
          <Label className="text-sm font-medium">{label}</Label>
          {description && (
            <p className="text-xs text-muted-foreground">{description}</p>
          )}
        </div>
      </div>
      <Switch checked={value} onCheckedChange={onToggle} />
    </div>
  );

  const SettingInput = ({
    label,
    icon,
    value,
    onChange,
    type = "text",
    placeholder = "",
  }) => (
    <div className="space-y-1 py-2">
      <div className="flex items-center gap-2">
        {icon}
        <Label className="text-sm font-medium">{label}</Label>
      </div>
      <Input
        value={value}
        onChange={onChange}
        type={type}
        placeholder={placeholder}
        className="w-full"
      />
    </div>
  );


  return (
    <div className="container max-w-5xl ">
      <div className="flex items-center justify-between">
        {modified === true && (
          <div className="mb-7 flex gap-[4px] items-center">
            <Button
              className={state.Theme?.successBtn}
              onClick={() => state.v2_ConfigSave()}>
              Save
            </Button>
            <div className="ml-3 text-yellow-400 text-xl">
              Your config has un-saved changes
            </div>
          </div>
        )}
      </div>


      <Tabs defaultValue="general" className="size-fit">
        <TabsList
          className={state.Theme?.borderColor}
        >
          <TabsTrigger className={state.Theme?.tabs} value="general">General Settings</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="apiconfig">API Config</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="net">Network Information</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="sys">System Information</TabsTrigger>
        </TabsList>
        <TabsContent value="general" className="pl-2">
          <Card className="bg-black border-none">
            <CardContent>
              <SettingToggle
                label="Basic Logging"
                icon={<Info className="h-4 w-4 mt-1 text-blue-500" />}
                value={InfoLogging}
                onToggle={() => {
                  state.toggleKeyAndReloadDom("Config", "InfoLogging");
                  state.renderPage("settings");
                }}
                description="Logs basic information about application operations"
              />

              <SettingToggle
                label="Error Logging"
                icon={<AlertTriangle className="h-4 w-4 mt-1 text-red-500" />}
                value={ErrorLogging}
                onToggle={() => {
                  state.toggleKeyAndReloadDom("Config", "ErrorLogging");
                  state.renderPage("settings");
                }}
                description="Logs errors and exceptions"
              />
              <SettingToggle
                label="Console Logging"
                icon={<Bug className="h-4 w-4 mt-1 text-amber-500" />}
                value={ConsoleLogging}
                onToggle={() => {
                  state.toggleKeyAndReloadDom("Config", "ConsoleLogging");
                  state.renderPage("settings");
                }}
                description="Detailed logs for troubleshooting"
              />

              <SettingToggle
                label="Debug Logging"
                icon={<Bug className="h-4 w-4 mt-1 text-amber-500" />}
                value={DebugLogging}
                onToggle={() => {
                  state.toggleKeyAndReloadDom("Config", "DebugLogging");
                  state.renderPage("settings");
                }}
                description="Detailed logs for troubleshooting"
              />

              <SettingToggle
                label="Debug Mode"
                icon={<Bug className="h-4 w-4 mt-1 text-purple-500" />}
                value={state?.debug}
                onToggle={() => {
                  state.toggleDebug();
                  state.renderPage("settings");
                }}
                description="Enables advanced debugging features"
              />

            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="apiconfig" className="pl-2">
          <Card className="bg-black border-none">
            <CardContent className="space-y-0">
              <SettingInput
                label="API IP"
                icon={<Globe className="h-4 w-4 text-blue-500" />}
                value={APIIP}
                onChange={(e) => {
                  state.setKeyAndReloadDom("Config", "APIIP", e.target.value);
                  state.renderPage("settings");
                }}
                placeholder="Enter API IP address"
              />

              <SettingInput
                label="API Port"
                icon={<Server className="h-4 w-4 text-indigo-500" />}
                value={APIPort}
                onChange={(e) => {
                  state.setKeyAndReloadDom("Config", "APIPort", e.target.value);
                  state.renderPage("settings");
                }}
                placeholder="Enter API port"
              />

              <SettingInput
                label="API Cert Domains"
                icon={<Globe className="h-4 w-4 text-green-500" />}
                value={APICertDomains}
                onChange={(e) => {
                  state.setArrayAndReloadDom(
                    "Config",
                    "APICertDomains",
                    e.target.value,
                  );
                  state.renderPage("settings");
                }}
                placeholder="Enter domain names"
              />

              <SettingInput
                label="API Cert IPs"
                icon={<Network className="h-4 w-4 text-cyan-500" />}
                value={APICertIPs}
                onChange={(e) => {
                  state.setArrayAndReloadDom(
                    "Config",
                    "APICertIPs",
                    e.target.value,
                  );
                  state.renderPage("settings");
                }}
                placeholder="Enter IP addresses"
              />

              <SettingInput
                label="API Cert"
                icon={<Key className="h-4 w-4 text-amber-500" />}
                value={APICert}
                onChange={(e) => {
                  state.setKeyAndReloadDom("Config", "APICert", e.target.value);
                  state.renderPage("settings");
                }}
                placeholder="Enter certificate path"
              />

              <SettingInput
                label="API Key"
                icon={<Key className="h-4 w-4 text-rose-500" />}
                value={APIKey}
                onChange={(e) => {
                  state.setKeyAndReloadDom("Config", "APIKey", e.target.value);
                  state.renderPage("settings");
                }}
                placeholder="Enter API key"
              />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="net" className="pl-2">
          <Card className="bg-black border-none">
            <CardContent className="space-y-0">
              <InfoItem
                label="Interface"
                value={state.Network?.DefaultInterfaceName}
                icon={<Network className="h-5 w-4 text-blue-400" />}
              />

              <InfoItem
                label="IP Address"
                value={state.Network?.DefaultInterface}
                icon={<Globe className="h-4 w-4 text-teal-400" />}
              />

              <InfoItem
                label="Interface ID"
                value={state.Network?.DefaultInterfaceID}
                icon={<Info className="h-4 w-4 text-indigo-400" />}
              />

              <InfoItem
                label="Gateway"
                value={state.Network?.DefaultGateway}
                icon={<Server className="h-4 w-4 text-violet-400" />}
              />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="sys" className="pl-2">
          <Card className="bg-black border-none">
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <InfoItem
                  label="API Version"
                  value={apiversion}
                  icon={<Info className="h-4 w-4 text-blue-400" />}
                />

                <InfoItem
                  label="App Version"
                  value={version}
                  icon={<Info className="h-4 w-4 text-green-400" />}
                />
              </div>

              <div className="space-y-1">
                <InfoItem
                  label="Base Path"
                  value={basePath}
                  icon={<Server className="h-4 w-4 text-neutral-400" />}
                />

                <InfoItem
                  label="Config File"
                  value={configPath}
                  icon={<Server className="h-4 w-4 text-amber-400" />}
                />

                <InfoItem
                  label="Log Path"
                  value={logPath || "Default"}
                  icon={<Server className="h-4 w-4 text-red-400" />}
                />

                <InfoItem
                  label="Log File"
                  value={logFileName}
                  icon={<Info className="h-4 w-4 text-red-400" />}
                />

                <InfoItem
                  label="Trace Path"
                  value={tracePath || "Default"}
                  icon={<Activity className="h-4 w-4 text-purple-400" />}
                />

                <InfoItem
                  label="Trace File"
                  value={traceFileName}
                  icon={<Activity className="h-4 w-4 text-purple-400" />}
                />

                <InfoItem
                  label="Admin"
                  value={state.State?.IsAdmin ? "Yes" : "No"}
                  icon={<Key className="h-4 w-4 text-yellow-400" />}
                />
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default Settings;
