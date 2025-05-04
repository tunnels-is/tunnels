import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import FormKeyValue from "./component/formkeyvalue";
import KeyValue from "./component/keyvalue";
import CustomToggle from "./component/CustomToggle";
import FormKeyInput from "./component/formkeyrawvalue";
import STORE from "../store";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  AlertCircle,
  Save,
  Info,
  Bug,
  AlertTriangle,
  Activity,
  Server,
  Globe,
  Key,
  Settings2,
  Network,
} from "lucide-react";

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
    <div className="space-y-2 py-3">
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

  const InfoItem = ({ label, value, icon }) => (
    <div className="flex flex-col py-2 space-y-1">
      <div className="flex items-center gap-2">
        {icon}
        <Label className="text-sm font-medium">{label}</Label>
      </div>
      <code className="text-xs block font-mono bg-muted/60 px-2 py-1.5 rounded w-full overflow-auto whitespace-normal break-all">
        {value !== undefined && value !== null ? String(value) : "Unknown"}
      </code>
    </div>
  );

  return (
    <div className="container max-w-5xl ">
      <div className="flex items-center justify-between">
        {modified === true && (
          <Button
            className="flex items-center gap-2"
            onClick={() => state.v2_ConfigSave()}
            variant="default"
          >
            <Save className="h-4 w-4" />
            Save Changes
          </Button>
        )}
      </div>

      {modified === true && (
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4 flex items-center gap-3 mb-6">
          <AlertCircle className="h-5 w-5 text-yellow-500" />
          <p className="text-sm">Your configuration has unsaved changes</p>
        </div>
      )}

      <Tabs defaultValue="general" className="w-[400px]">
        <TabsList>
          <TabsTrigger value="general">General Settings</TabsTrigger>
          <TabsTrigger value="apiconfig">API Config</TabsTrigger>
          <TabsTrigger value="net">Network Information</TabsTrigger>
          <TabsTrigger value="sys">System Information</TabsTrigger>
        </TabsList>
        <TabsContent value="general">
          <Card className="mt-5 bg-black border-none">
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

              <SettingToggle
                label="Connection Tracing"
                icon={<Activity className="h-4 w-4 mt-1 text-green-500" />}
                value={ConnectionTracer}
                onToggle={() => {
                  state.toggleKeyAndReloadDom("Config", "ConnectionTracer");
                  state.renderPage("settings");
                }}
                description="Tracks all connection activities"
              />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="apiconfig">
          <Card className="mt-5 bg-black border-none">
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
        <TabsContent value="net">
          {" "}
          <Card className="mt-5 bg-black border-none">
            <CardContent>
              <InfoItem
                label="Interface"
                value={state.Network?.DefaultInterfaceName}
                icon={<Network className="h-4 w-4 text-blue-400" />}
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
        <TabsContent value="sys">
          <Card className="mt-5 bg-black border-none">
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
