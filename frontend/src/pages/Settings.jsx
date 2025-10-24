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
import InfoItem from "../components/InfoItem";
import { useState } from "react";
import FormKeyValue from "../components/formkeyvalue";

const Settings = () => {
  const state = GLOBAL_STATE("settings");
  const [cfg, setCfg] = useState({ ...state.Config })
  const [mod, setMod] = useState(false)

  const updatecfg = (key, value) => {
    if (key === "APICertDomains" || key === "APICertIPs") {
      value = value.split(",")
    }
    let x = { ...cfg }
    x[key] = value
    setMod(true)
    setCfg(x)
  }

  useEffect(() => {
    state.GetBackendState();
  }, []);

  let basePath = state.State?.BasePath;
  let logPath = "";
  let tracePath = "";
  let logFileName = state.State?.LogFileName?.replace(state.State?.LogPath, "");
  let configPath = state.State?.ConfigFileName;
  if (state.State?.LogPath !== basePath) {
    logPath = state.State?.LogPath;
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

  return (
    <div className="container max-w-5xl p-4">
      <div className="flex items-center justify-between">
        {mod === true && (
          <div className="mb-7 flex gap-[4px] items-center">
            <Button
              className={state.Theme?.successBtn}
              onClick={async () => {
                state.Config = cfg
                let ok = await state.v2_ConfigSave()
                if (ok === true) {
                  setMod(false)
                }
              }}>
              Save
            </Button>
            <div className="ml-3 text-yellow-400 text-xl">
              Your config has un-saved changes
            </div>
          </div>
        )}
      </div>


      <Tabs defaultValue="general" className="size-fit">
        <TabsList>
          <TabsTrigger value="general">General Settings</TabsTrigger>
          <TabsTrigger value="apiconfig">API Config</TabsTrigger>
          <TabsTrigger value="net">Network Information</TabsTrigger>
          <TabsTrigger value="sys">System Information</TabsTrigger>
        </TabsList>
        <TabsContent value="general" className="pl-2">
          <Card className="border-none shadow-none">
            <CardContent>
              <SettingToggle
                label="Basic Logging"
                icon={<Info className="h-4 w-4 mt-1 text-blue-500" />}
                value={state?.Config?.InfoLogging}
                onToggle={() => {
                  state.toggleConfigKeyAndSave("Config", "InfoLogging");
                  state.renderPage("settings");
                }}
                description="Logs basic information about application operations"
              />

              <SettingToggle
                label="Error Logging"
                icon={<AlertTriangle className="h-4 w-4 mt-1 text-red-500" />}
                value={state?.Config?.ErrorLogging}
                onToggle={() => {
                  state.toggleConfigKeyAndSave("Config", "ErrorLogging");
                  state.renderPage("settings");
                }}
                description="Logs errors and exceptions"
              />
              <SettingToggle
                label="Console Logging"
                icon={<Bug className="h-4 w-4 mt-1 text-amber-500" />}
                value={state?.Config?.ConsoleLogging}
                onToggle={() => {
                  state.toggleConfigKeyAndSave("Config", "ConsoleLogging");
                  state.renderPage("settings");
                }}
                description="Detailed logs for troubleshooting"
              />

              <SettingToggle
                label="Debug Logging"
                icon={<Bug className="h-4 w-4 mt-1 text-amber-500" />}
                value={state?.Config?.DebugLogging}
                onToggle={() => {
                  state.toggleConfigKeyAndSave("Config", "DebugLogging");
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
          <Card className="border-none shadow-none">
            <CardContent className="space-y-0">

              <FormKeyValue
                label="API IP"
                icon={Network}
                iconClass={"text-green-400"}
                value={
                  <Input
                    value={cfg.APIIP}
                    onChange={(e) => {
                      updatecfg("APIIP", e.target.value)
                    }}
                    type="text"
                  />
                }
              />

              <FormKeyValue
                label="API Port"
                icon={Server}
                iconClass={"text-green-400"}
                value={
                  <Input
                    value={cfg.APIPort}
                    onChange={(e) => {
                      updatecfg("APIPort", e.target.value)
                    }}
                    type="text"
                  />
                }
              />

              <FormKeyValue
                label="API Certificate Domains"
                icon={Globe}
                value={
                  <Input
                    value={cfg.APICertDomains}
                    onChange={(e) => {
                      updatecfg("APICertDomains", e.target.value)
                    }}
                    type="text"
                  />
                }
              />

              <FormKeyValue
                label="API Certificate IPs"
                icon={Network}
                value={
                  <Input
                    value={cfg.APICertIPs}
                    onChange={(e) => {
                      updatecfg("APICertIPs", e.target.value)
                    }}
                    type="text"
                  />
                }
              />

              <FormKeyValue
                label="API Certificate Path"
                icon={Key}
                iconClass={"text-orange-400"}
                value={
                  <Input
                    value={cfg.APICert}
                    onChange={(e) => {
                      updatecfg("APICert", e.target.value)
                    }}
                    type="text"
                  />
                }
              />
              <FormKeyValue
                label="API Key Path"
                icon={Key}
                iconClass={"text-orange-400"}
                value={
                  <Input
                    value={cfg.APIKey}
                    onChange={(e) => {
                      updatecfg("APIKey", e.target.value)
                    }}
                    type="text"
                  />
                }
              />

            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="net" className="pl-2">
          <Card className="border-none shadow-none">
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
          <Card className="border-none shadow-none">
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
