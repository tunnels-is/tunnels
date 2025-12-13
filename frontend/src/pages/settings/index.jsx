import React, { useEffect, useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAtom } from "jotai";
import { configAtom } from "@/stores/configStore";
import { debugAtom } from "@/stores/uiStore";
import { useSaveConfig } from "@/hooks/useConfig";
import { getBackendState } from "@/api/app";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import {
  Info,
  Bug,
  AlertTriangle,
  Server,
  Globe,
  Key,
  Network,
  Save,
  Settings as SettingsIcon,
  Monitor,
  Plus,
  Trash2,
} from "lucide-react";

export default function SettingsPage() {
  const [config, setConfig] = useAtom(configAtom);
  const [debug, setDebug] = useAtom(debugAtom);
  const saveConfigMutation = useSaveConfig();

  const [cfg, setCfg] = useState(config || {});
  const [mod, setMod] = useState(false);
  const [backendState, setBackendState] = useState(null);

  useEffect(() => {
    if (config) setCfg(config);
  }, [config]);

  useEffect(() => {
    getBackendState().then(setBackendState).catch(console.error);
  }, []);

  const updatecfg = (key, value) => {
    let x = { ...cfg }
    x[key] = value
    setMod(true)
    setCfg(x)
  }

  const toggleConfigKey = (key) => {
    const newConfig = { ...config, [key]: !config[key] };
    saveConfigMutation.mutate(newConfig);
  };

  let basePath = backendState?.State?.BasePath;
  let logPath = "";
  let logFileName = backendState?.State?.LogFileName?.replace(backendState?.State?.LogPath, "");
  let configPath = backendState?.State?.ConfigFileName;
  if (backendState?.State?.LogPath !== basePath) {
    logPath = backendState?.State?.LogPath;
  }
  let version = backendState?.Version ? backendState?.Version : "unknown";
  let apiversion = backendState?.APIVersion ? backendState?.APIVersion : "unknown";

  const InfoItem = ({ label, value, icon }) => (
    <div className="flex items-center justify-between py-2 border-b border-[#1a1f2d] last:border-0">
      <div className="flex items-center gap-2 text-sm text-gray-400">
        {icon}
        <span>{label}</span>
      </div>
      <span className="text-sm font-mono text-gray-200">{value || "N/A"}</span>
    </div>
  );

  return (
    <div className="w-full space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Settings</h1>
          <p className="text-muted-foreground">Configure application settings and view system information.</p>
        </div>
        {mod === true && (
          <div className="flex items-center gap-4 bg-yellow-900/20 border border-yellow-900/50 px-4 py-2 rounded-md">
            <div className="flex items-center gap-2 text-yellow-500">
              <AlertTriangle className="h-4 w-4" />
              <span className="text-sm font-medium">Unsaved changes</span>
            </div>
            <Button
              size="sm"
              onClick={() => {
                saveConfigMutation.mutate(cfg, {
                  onSuccess: () => setMod(false)
                });
              }}>
              <Save className="h-4 w-4 mr-2" />
              Save Changes
            </Button>
          </div>
        )}
      </div>

      <Tabs defaultValue="general" className="space-y-6">
        <TabsList className="bg-[#0B0E14] border border-[#1a1f2d] p-1 h-auto flex-wrap justify-start">
          <TabsTrigger value="general" className="data-[state=active]:bg-[#1a1f2d]">General Settings</TabsTrigger>
          <TabsTrigger value="apiconfig" className="data-[state=active]:bg-[#1a1f2d]">API Config</TabsTrigger>
          <TabsTrigger value="net" className="data-[state=active]:bg-[#1a1f2d]">Network Info</TabsTrigger>
          <TabsTrigger value="sys" className="data-[state=active]:bg-[#1a1f2d]">System Info</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="space-y-6">
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <SettingsIcon className="h-5 w-5 text-blue-500" />
                Logging Configuration
              </CardTitle>
              <CardDescription>Configure what events are logged and tracked.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-1">
              <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <div className="flex items-center gap-3">
                  <Info className="h-4 w-4 text-blue-500" />
                  <div className="space-y-0.5">
                    <Label className="text-sm font-medium">Basic Logging</Label>
                    <p className="text-xs text-muted-foreground">Logs basic information about operations</p>
                  </div>
                </div>
                <Switch checked={config?.InfoLogging} onCheckedChange={() => toggleConfigKey("InfoLogging")} />
              </div>

              <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <div className="flex items-center gap-3">
                  <AlertTriangle className="h-4 w-4 text-red-500" />
                  <div className="space-y-0.5">
                    <Label className="text-sm font-medium">Error Logging</Label>
                    <p className="text-xs text-muted-foreground">Logs errors and exceptions</p>
                  </div>
                </div>
                <Switch checked={config?.ErrorLogging} onCheckedChange={() => toggleConfigKey("ErrorLogging")} />
              </div>

              <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <div className="flex items-center gap-3">
                  <Bug className="h-4 w-4 text-amber-500" />
                  <div className="space-y-0.5">
                    <Label className="text-sm font-medium">Console Logging</Label>
                    <p className="text-xs text-muted-foreground">Output logs to console</p>
                  </div>
                </div>
                <Switch checked={config?.ConsoleLogging} onCheckedChange={() => toggleConfigKey("ConsoleLogging")} />
              </div>

              <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <div className="flex items-center gap-3">
                  <Bug className="h-4 w-4 text-amber-500" />
                  <div className="space-y-0.5">
                    <Label className="text-sm font-medium">Debug Logging</Label>
                    <p className="text-xs text-muted-foreground">Detailed logs for troubleshooting</p>
                  </div>
                </div>
                <Switch checked={config?.DebugLogging} onCheckedChange={() => toggleConfigKey("DebugLogging")} />
              </div>

              <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <div className="flex items-center gap-3">
                  <Bug className="h-4 w-4 text-purple-500" />
                  <div className="space-y-0.5">
                    <Label className="text-sm font-medium">Debug Mode</Label>
                    <p className="text-xs text-muted-foreground">Enables advanced debugging features</p>
                  </div>
                </div>
                <Switch checked={debug} onCheckedChange={() => setDebug(!debug)} />
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="apiconfig" className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="bg-[#0B0E14] border-[#1a1f2d]">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Server className="h-5 w-5 text-blue-500" />
                  API Server
                </CardTitle>
                <CardDescription>Configure the API server listener.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>API IP</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.APIIP}
                    onChange={(e) => updatecfg("APIIP", e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>API Port</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.APIPort}
                    onChange={(e) => updatecfg("APIPort", e.target.value)}
                  />
                </div>
              </CardContent>
            </Card>

            <Card className="bg-[#0B0E14] border-[#1a1f2d]">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Key className="h-5 w-5 text-orange-500" />
                  SSL Certificates
                </CardTitle>
                <CardDescription>Configure SSL certificate paths.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Certificate Path</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.APICert}
                    onChange={(e) => updatecfg("APICert", e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Key Path</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.APIKey}
                    onChange={(e) => updatecfg("APIKey", e.target.value)}
                  />
                </div>
              </CardContent>
            </Card>

            <Card className="bg-[#0B0E14] border-[#1a1f2d] md:col-span-2">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Globe className="h-5 w-5 text-green-500" />
                  Certificate Domains & IPs
                </CardTitle>
                <CardDescription>Certificate SANs for SSL/TLS.</CardDescription>
              </CardHeader>
              <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <Label>Domains</Label>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0 hover:bg-green-900/30 text-green-500"
                      onClick={() => {
                        const newDomains = [...(cfg.APICertDomains || []), ""];
                        updatecfg("APICertDomains", newDomains);
                      }}
                    >
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>
                  <div className="space-y-2 pl-2 border-l-2 border-[#1a1f2d]">
                    {(cfg.APICertDomains || []).map((domain, i) => (
                      <div key={i} className="flex gap-2 items-center">
                        <Input
                          className="flex-1 bg-[#0B0E14] border-[#1a1f2d]"
                          value={domain}
                          onChange={(e) => {
                            const newDomains = [...cfg.APICertDomains];
                            newDomains[i] = e.target.value;
                            updatecfg("APICertDomains", newDomains);
                          }}
                          placeholder="example.com"
                        />
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
                          onClick={() => {
                            const newDomains = cfg.APICertDomains.filter((_, idx) => idx !== i);
                            updatecfg("APICertDomains", newDomains);
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                    {(!cfg.APICertDomains || cfg.APICertDomains.length === 0) && (
                      <div className="text-xs text-muted-foreground italic">No domains added</div>
                    )}
                  </div>
                </div>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <Label>IP Addresses</Label>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0 hover:bg-green-900/30 text-green-500"
                      onClick={() => {
                        const newIPs = [...(cfg.APICertIPs || []), ""];
                        updatecfg("APICertIPs", newIPs);
                      }}
                    >
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>
                  <div className="space-y-2 pl-2 border-l-2 border-[#1a1f2d]">
                    {(cfg.APICertIPs || []).map((ip, i) => (
                      <div key={i} className="flex gap-2 items-center">
                        <Input
                          className="flex-1 bg-[#0B0E14] border-[#1a1f2d]"
                          value={ip}
                          onChange={(e) => {
                            const newIPs = [...cfg.APICertIPs];
                            newIPs[i] = e.target.value;
                            updatecfg("APICertIPs", newIPs);
                          }}
                          placeholder="192.168.1.1"
                        />
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
                          onClick={() => {
                            const newIPs = cfg.APICertIPs.filter((_, idx) => idx !== i);
                            updatecfg("APICertIPs", newIPs);
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                    {(!cfg.APICertIPs || cfg.APICertIPs.length === 0) && (
                      <div className="text-xs text-muted-foreground italic">No IPs added</div>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="net">
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Network className="h-5 w-5 text-teal-500" />
                Network Information
              </CardTitle>
              <CardDescription>System network configuration details.</CardDescription>
            </CardHeader>
            <CardContent>
              <InfoItem
                label="Interface"
                value={backendState?.Network?.DefaultInterfaceName}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />
              <InfoItem
                label="IP Address"
                value={backendState?.Network?.DefaultInterface}
                icon={<Globe className="h-4 w-4 text-teal-400" />}
              />
              <InfoItem
                label="Interface ID"
                value={backendState?.Network?.DefaultInterfaceID}
                icon={<Info className="h-4 w-4 text-indigo-400" />}
              />
              <InfoItem
                label="Gateway"
                value={backendState?.Network?.DefaultGateway}
                icon={<Server className="h-4 w-4 text-violet-400" />}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="sys">
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Monitor className="h-5 w-5 text-indigo-500" />
                System Information
              </CardTitle>
              <CardDescription>Application and system details.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
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

              <div className="space-y-0">
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
                  value={backendState?.State?.IsAdmin ? "Yes" : "No"}
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


