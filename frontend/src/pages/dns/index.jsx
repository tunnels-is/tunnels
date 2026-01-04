import React, { useEffect, useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import { Switch } from "@/components/ui/switch";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { Button } from "@/components/ui/button";
import DNSAnswers from "./answers/domain";
import { useAtom } from "jotai";
import { configAtom } from "@/stores/configStore";
import { useSaveConfig } from "@/hooks/useConfig";
import { getBackendState } from "@/api/app";
import { getDNSStats } from "@/api/dns";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Server, Shield, Activity, Save, AlertTriangle, Plus, Trash2, Edit } from "lucide-react";

const DNSSort = (a, b) => {
  if (dayjs(a.LastSeen).unix() < dayjs(b.LastSeen).unix()) {
    return 1;
  } else if (dayjs(a.LastSeen).unix() > dayjs(b.LastSeen).unix()) {
    return -1;
  }
  return 0;
};

export default function DNSPage() {
  const navigate = useNavigate();
  const [config, setConfig] = useAtom(configAtom);
  const saveConfigMutation = useSaveConfig();
  const dnsStats = useQuery({
    queryKey: ["dns-stats"],
    queryFn: getDNSStats,
    refetchInterval: 5000, // Refresh every 5 seconds as it's stats
  });
  const [record, setRecord] = useState(undefined)
  const [recordModal, setRecordModal] = useState(false)
  const [isRecordEdit, setIsRecordEdit] = useState(false)
  const [blocklist, setBlocklist] = useState(undefined)
  const [blocklistModal, setBlocklistModal] = useState(false)
  const [isBlocklistEdit, setIsBlocklistEdit] = useState(false)
  const [whitelist, setWhitelist] = useState(undefined)
  const [whitelistModal, setWhitelistModal] = useState(false)
  const [isWhitelistEdit, setIsWhitelistEdit] = useState(false)

  const [cfg, setCfg] = useState(config || {})
  const [mod, setMod] = useState(false)

  useEffect(() => {
    if (config) setCfg(config);
  }, [config]);

  useEffect(() => {
    getBackendState().then(state => {
      if (state?.Config) setConfig(state.Config);
    }).catch(console.error);
  }, [setConfig]);

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

  let blockLists = config?.DNSBlockLists || [];
  let whiteLists = config?.DNSWhiteLists || [];

  const toggleList = (list, type, obj) => {
    const newList = list.map(l => l.Tag === obj.Tag ? { ...l, Enabled: !l.Enabled } : l);
    saveConfigMutation.mutate({ ...config, [type]: newList });
  }

  // --- Columns Definitions ---

  const recordsColumns = useMemo(() => [
    { header: "Domain", accessorKey: "Domain" },
    { header: "IP", accessorKey: "IP", cell: ({ row }) => row.original.IP?.join(", ") || "" },
    { header: "Text", accessorKey: "TXT", cell: ({ row }) => row.original.TXT?.join(", ") || "" },
    {
      header: "Wildcard",
      accessorKey: "Wildcard",
      cell: ({ row }) => row.original.Wildcard ? "yes" : "no"
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setIsRecordEdit(true)
              setRecord(row.original)
              setRecordModal(true)
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => {
              const newRecords = config.DNSRecords.filter((r) => r.Domain !== row.original.Domain);
              saveConfigMutation.mutate({ ...config, DNSRecords: newRecords });
            }}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [config, saveConfigMutation]);

  const whitelistColumns = useMemo(() => [
    { header: "Tag", accessorKey: "Tag" },
    { header: "Count", accessorKey: "Count" },
    {
      header: "Enabled",
      accessorKey: "Enabled",
      cell: ({ row }) => (
        <Switch
          checked={row.original.Enabled}
          onCheckedChange={() => toggleList(config?.DNSWhiteLists || [], "DNSWhiteLists", row.original)}
        />
      )
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setIsWhitelistEdit(true)
              setWhitelist(row.original)
              setWhitelistModal(true)
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => {
              const newList = (config?.DNSWhiteLists || []).filter(l => l.Tag !== row.original.Tag);
              saveConfigMutation.mutate({ ...config, DNSWhiteLists: newList });
            }}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [config, saveConfigMutation]);

  const blocklistColumns = useMemo(() => [
    { header: "Tag", accessorKey: "Tag" },
    { header: "Count", accessorKey: "Count" },
    {
      header: "Enabled",
      accessorKey: "Enabled",
      cell: ({ row }) => (
        <Switch
          checked={row.original.Enabled}
          onCheckedChange={() => toggleList(config?.DNSBlockLists || [], "DNSBlockLists", row.original)}
        />
      )
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setIsBlocklistEdit(true)
              setBlocklist(row.original)
              setBlocklistModal(true)
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => {
              const newList = (config?.DNSBlockLists || []).filter(l => l.Tag !== row.original.Tag);
              saveConfigMutation.mutate({ ...config, DNSBlockLists: newList });
            }}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [config, saveConfigMutation]);

  const blockDomainsData = useMemo(() => {
    let dnsBlocks = dnsStats.data || [];
    let stats = [];
    if (dnsBlocks) {
      Object.entries(dnsBlocks).forEach(([key, value]) => {
        let lb = dayjs(value.LastBlocked);
        let ls = dayjs(value.LastSeen);
        if (ls.diff(lb, "s") > 0) {
          return;
        }
        stats.push({ ...value, tag: key });
      });
    }
    return stats.sort(DNSSort);
  }, [dnsStats.data]);

  const blockDomainsColumns = useMemo(() => [
    { header: "Domain", accessorKey: "tag" },
    { header: "Count", accessorKey: "Count" },
    { header: "FirstSeen", accessorKey: "FirstSeen", cell: ({ row }) => dayjs(row.original.FirstSeen).format("HH:mm:ss DD-MM-YYYY") },
    { header: "LastSeen", accessorKey: "LastSeen", cell: ({ row }) => dayjs(row.original.LastSeen).format("HH:mm:ss DD-MM-YYYY") },
  ], []);

  const resolvedDomainsData = useMemo(() => {
    let dnsResolves = dnsStats.data || [];
    let stats = [];
    if (dnsResolves) {
      Object.entries(dnsResolves).forEach(([key, value]) => {
        let lb = dayjs(value.LastBlocked);
        let ls = dayjs(value.LastSeen);
        if (ls.diff(lb, "s") > 0) {
          stats.push({ ...value, tag: key });
        }
      });
    }
    return stats.sort(DNSSort);
  }, [dnsStats.data]);

  const resolvedDomainsColumns = useMemo(() => [
    {
      header: "Domain",
      accessorKey: "tag",
      cell: ({ row }) => (
        <span
          className="cursor-pointer text-blue-500 hover:text-blue-400 hover:underline"
          onClick={() => navigate("/dns/answers/" + row.original.tag)}
        >
          {row.original.tag}
        </span>
      )
    },
    { header: "Count", accessorKey: "Count" },
    { header: "FirstSeen", accessorKey: "FirstSeen", cell: ({ row }) => dayjs(row.original.FirstSeen).format("HH:mm:ss DD-MM-YYYY") },
    { header: "LastSeen", accessorKey: "LastSeen", cell: ({ row }) => dayjs(row.original.LastSeen).format("HH:mm:ss DD-MM-YYYY") },
  ], [navigate]);


  return (
    <div className="w-full mt-16 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">DNS Configuration</h1>
          <p className="text-muted-foreground">Manage your DNS server settings, records, and filtering.</p>
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

      <Tabs defaultValue="settings" className="space-y-6">
        <TabsList className="bg-[#0B0E14] border border-[#1a1f2d] p-1 h-auto flex-wrap justify-start">
          <TabsTrigger value="settings" className="data-[state=active]:bg-[#1a1f2d]">Settings</TabsTrigger>
          <TabsTrigger value="records" className="data-[state=active]:bg-[#1a1f2d]">DNS Records</TabsTrigger>
          <TabsTrigger value="blocklist" className="data-[state=active]:bg-[#1a1f2d]">Block Lists</TabsTrigger>
          <TabsTrigger value="whitelist" className="data-[state=active]:bg-[#1a1f2d]">White Lists</TabsTrigger>
          <TabsTrigger value="blockdomains" className="data-[state=active]:bg-[#1a1f2d]">Blocked Domains</TabsTrigger>
          <TabsTrigger value="resolveddomains" className="data-[state=active]:bg-[#1a1f2d]">Resolved Domains</TabsTrigger>
        </TabsList>

        <TabsContent value="settings" className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="bg-[#0B0E14] border-[#1a1f2d]">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Server className="h-5 w-5 text-blue-500" />
                  Server Configuration
                </CardTitle>
                <CardDescription>Configure the DNS server listener.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Server IP</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.DNSServerIP}
                    onChange={(e) => updatecfg("DNSServerIP", e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Server Port</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.DNSServerPort}
                    onChange={(e) => updatecfg("DNSServerPort", e.target.value)}
                  />
                </div>
              </CardContent>
            </Card>

            <Card className="bg-[#0B0E14] border-[#1a1f2d]">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Shield className="h-5 w-5 text-green-500" />
                  Upstream DNS
                </CardTitle>
                <CardDescription>Configure upstream resolvers.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Primary DNS</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.DNS1Default}
                    onChange={(e) => updatecfg("DNS1Default", e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Backup DNS</Label>
                  <Input
                    className="bg-[#151a25] border-[#2a3142]"
                    value={cfg.DNS2Default}
                    onChange={(e) => updatecfg("DNS2Default", e.target.value)}
                  />
                </div>
                <div className="flex items-center justify-between pt-2">
                  <div className="space-y-0.5">
                    <Label>Secure DNS (DoH)</Label>
                    <div className="text-xs text-muted-foreground">Use DNS over HTTPS</div>
                  </div>
                  <Switch
                    checked={config?.DNSOverHTTPS}
                    onCheckedChange={() => toggleConfigKey("DNSOverHTTPS")}
                  />
                </div>
              </CardContent>
            </Card>

            <Card className="bg-[#0B0E14] border-[#1a1f2d] md:col-span-2">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Activity className="h-5 w-5 text-purple-500" />
                  Logging & Statistics
                </CardTitle>
                <CardDescription>Configure what events are logged and tracked.</CardDescription>
              </CardHeader>
              <CardContent className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                  <Label className="cursor-pointer" onClick={() => toggleConfigKey("LogBlockedDomains")}>Log Blocked Domains</Label>
                  <Switch
                    checked={config?.LogBlockedDomains}
                    onCheckedChange={() => toggleConfigKey("LogBlockedDomains")}
                  />
                </div>
                <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                  <Label className="cursor-pointer" onClick={() => toggleConfigKey("LogAllDomains")}>Log All Domains</Label>
                  <Switch
                    checked={config?.LogAllDomains}
                    onCheckedChange={() => toggleConfigKey("LogAllDomains")}
                  />
                </div>
                <div className="flex items-center justify-between p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                  <Label className="cursor-pointer" onClick={() => toggleConfigKey("DNSstats")}>Enable Statistics</Label>
                  <Switch
                    checked={config?.DNSstats}
                    onCheckedChange={() => toggleConfigKey("DNSstats")}
                  />
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="whitelist" className="space-y-4">
          <div className="flex justify-end gap-2">
            <Button
              onClick={() => saveConfigMutation.mutate(config)}
              variant="outline"
              size="sm"
            >
              <Save className="h-4 w-4 mr-2" /> Save List
            </Button>
            <Button onClick={() => {
              setIsWhitelistEdit(false)
              setWhitelist({
                Tag: "new-whitelist",
                URL: "https://example.com/whitelist.txt",
                Enabled: true,
                Count: 0
              })
              setWhitelistModal(true)
            }} className="gap-2">
              <Plus className="h-4 w-4" /> Add Whitelist
            </Button>
          </div>
          <CustomTable data={whiteLists || []} columns={whitelistColumns} />
          <EditDialog
            key={whitelist?.Tag || 'new-wl'}
            open={whitelistModal}
            onOpenChange={setWhitelistModal}
            initialData={whitelist}
            title="DNS Whitelist"
            description="Allow specific domains to bypass filters."
            readOnly={false}
            fields={{
              Count: "hidden",
              LastDownload: "hidden"
            }}
            onSubmit={async (values) => {
              let newWhitelists = [...(config?.DNSWhiteLists || [])];
              if (!isWhitelistEdit) {
                newWhitelists.push(values);
              } else {
                if (isWhitelistEdit) {
                  const index = newWhitelists.findIndex(l => l.Tag === values.Tag);
                  if (index !== -1) newWhitelists[index] = values;
                }
              }

              saveConfigMutation.mutate({ ...config, DNSWhiteLists: newWhitelists }, {
                onSuccess: () => {
                  setWhitelistModal(false);
                  setIsWhitelistEdit(false);
                }
              });
            }}
          />
        </TabsContent>

        <TabsContent value="blocklist" className="space-y-4">
          <div className="flex justify-end gap-2">
            <Button
              onClick={() => saveConfigMutation.mutate(config)}
              variant="outline"
              size="sm"
            >
              <Save className="h-4 w-4 mr-2" /> Save List
            </Button>
            <Button onClick={() => {
              setIsBlocklistEdit(false)
              setBlocklist({
                Tag: "new-blocklist",
                URL: "https://example.com/blocklist.txt",
                Enabled: true,
                Count: 0
              })
              setBlocklistModal(true)
            }} className="gap-2">
              <Plus className="h-4 w-4" /> Add Blocklist
            </Button>
          </div>
          <CustomTable data={blockLists || []} columns={blocklistColumns} />
          <EditDialog
            key={blocklist?.Tag || 'new-bl'}
            open={blocklistModal}
            onOpenChange={setBlocklistModal}
            initialData={blocklist}
            title="DNS Blocklist"
            description="Block domains using external lists or custom rules."
            readOnly={false}
            fields={{
              Count: "hidden",
              LastDownload: "hidden"
            }}
            onSubmit={async (values) => {
              let newBlocklists = [...(config?.DNSBlockLists || [])];
              if (!isBlocklistEdit) {
                newBlocklists.push(values);
              } else {
                if (isBlocklistEdit) {
                  const index = newBlocklists.findIndex(l => l.Tag === values.Tag);
                  if (index !== -1) newBlocklists[index] = values;
                }
              }
              saveConfigMutation.mutate({ ...config, DNSBlockLists: newBlocklists }, {
                onSuccess: () => {
                  setBlocklistModal(false);
                  setIsBlocklistEdit(false);
                }
              });
            }}
          />
        </TabsContent>

        <TabsContent value="blockdomains">
          <CustomTable data={blockDomainsData} columns={blockDomainsColumns} />
        </TabsContent>

        <TabsContent value="resolveddomains">
          <CustomTable data={resolvedDomainsData} columns={resolvedDomainsColumns} />
        </TabsContent>

        <TabsContent value="records" className="space-y-4">
          <div className="flex justify-end">
            <Button onClick={() => {
              setIsRecordEdit(false)
              setRecord({ Domain: "yourdomain.com", IP: ["127.0.0.1"], TXT: ["yourdomain.com text record"], Wildcard: true })
              setRecordModal(true)
            }} className="gap-2">
              <Plus className="h-4 w-4" /> Add Record
            </Button>
          </div>
          <CustomTable className={""} data={config?.DNSRecords || []} columns={recordsColumns} />
          <EditDialog
            key={record?.Domain || 'new-rec'}
            open={recordModal}
            onOpenChange={setRecordModal}
            initialData={record}
            title="DNS Record"
            description="Manage custom DNS records."
            readOnly={false}
            onSubmit={async (values) => {
              let newRecords = [...(config?.DNSRecords || [])];
              if (!isRecordEdit) {
                newRecords.push(values);
              } else {
                const index = newRecords.findIndex(r => r.Domain === values.Domain);
                if (index !== -1) newRecords[index] = values;
              }
              saveConfigMutation.mutate({ ...config, DNSRecords: newRecords }, {
                onSuccess: () => {
                  setRecordModal(false);
                  setIsRecordEdit(false);
                }
              });
            }}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};
export { DNSAnswers };

