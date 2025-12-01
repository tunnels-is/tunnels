import React, { useEffect, useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import { Switch } from "@/components/ui/switch";
import GenericTable from "../components/GenericTable";
import { TableCell } from "@/components/ui/table";
import NewObjectEditorDialog from "@/components/NewObjectEditorDialog";
import { Button } from "@/components/ui/button";

import { useAtom } from "jotai";
import { configAtom } from "../stores/configStore";
import { useSaveConfig } from "../hooks/useConfig";
import { useDNSStats } from "../hooks/useDNS";
import { getBackendState } from "../api/app";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Server, Shield, Activity, Save, AlertTriangle } from "lucide-react";

const DNSSort = (a, b) => {
  if (dayjs(a.LastSeen).unix() < dayjs(b.LastSeen).unix()) {
    return 1;
  } else if (dayjs(a.LastSeen).unix() > dayjs(b.LastSeen).unix()) {
    return -1;
  }
  return 0;
};

const DNS = () => {
  const navigate = useNavigate();
  const [config, setConfig] = useAtom(configAtom);
  const saveConfigMutation = useSaveConfig();
  const { data: dnsStats } = useDNSStats();

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

  const generateDNSRecordsTable = () => {
    return {
      data: config?.DNSRecords,
      columns: {
        Domain: true,
        IP: true,
        TXT: true,
        Wildcard: true,
      },
      headerFormat: {
        TXT: () => "Text"
      },
      columnFormat: {
        Wildcard: (obj) => obj.Wildcard === true ? "yes" : "no",
      },
      Btn: {
        Edit: (obj) => {
          setIsRecordEdit(true)
          setRecord(obj)
          setRecordModal(true)
        },
        Delete: (obj) => {
          const newRecords = config.DNSRecords.filter((r) => r.Domain !== obj.Domain);
          saveConfigMutation.mutate({ ...config, DNSRecords: newRecords });
        },
        New: () => {
          setIsRecordEdit(false)
          setRecord({ Domain: "yourdomain.com", IP: ["127.0.0.1"], TXT: ["yourdomain.com text record"], Wildcard: true })
          setRecordModal(true)
        }
      },
      headers: ["Domain", "IP", "Text", "Wildcard"],
    }
  }

  const generateBlocksTable = () => {
    let dnsBlocks = dnsStats || [];
    let rows = [];
    if (!dnsBlocks || dnsBlocks.length === 0) {
      return rows;
    }

    let stats = [];
    Object.entries(dnsBlocks).forEach(([key, value]) => {
      let lb = dayjs(value.LastBlocked);
      let ls = dayjs(value.LastSeen);
      if (ls.diff(lb, "s") > 0) {
        return true;
      }
      stats.push({ ...value, tag: key });
    });

    stats = stats.sort(DNSSort);
    return {
      data: stats,
      columns: {
        tag: true,
        Tag: true,
        Count: true,
        FirstSeen: true,
        LastSeen: true,
      },
      headerFormat: {
        "tag": () => "Domain"
      },
      columnFormat: {
        FirstSeen: (obj) => dayjs(obj.FirstSeen).format("HH:mm:ss DD-MM-YYYY"),
        LastSeen: (obj) => dayjs(obj.LastSeen).format("HH:mm:ss DD-MM-YYYY")
      },
      headers: ["tag", "Count", "FirstSeen", "LastSeen"],
    }
  };

  const generateResolvesTable = () => {
    let dnsResolves = dnsStats || [];
    let rows = [];
    if (!dnsResolves || dnsResolves.length === 0) {
      return rows;
    }

    let stats = [];
    Object.entries(dnsResolves).forEach(([key, value]) => {
      let lb = dayjs(value.LastBlocked);
      let ls = dayjs(value.LastSeen);
      if (ls.diff(lb, "s") > 0) {
        stats.push({ ...value, tag: key });
      }
    });

    stats = stats.sort(DNSSort);
    return {
      data: stats,
      columns: {
        tag: (value) => {
          navigate("/dns/answers/" + value.tag);
        },
        Count: true,
        FirstSeen: true,
        LastSeen: true,
      },
      headerFormat: {
        "tag": () => "Domain"
      },
      columnFormat: {
        FirstSeen: (obj) => dayjs(obj.FirstSeen).format("HH:mm:ss DD-MM-YYYY"),
        LastSeen: (obj) => dayjs(obj.LastSeen).format("HH:mm:ss DD-MM-YYYY")
      },
      headers: ["tag", "Count", "FirstSeen", "LastSeen"],
    }
  };

  const toggleList = (list, type, obj) => {
    const newList = list.map(l => l.Tag === obj.Tag ? { ...l, Enabled: !l.Enabled } : l);
    saveConfigMutation.mutate({ ...config, [type]: newList });
  }

  const EnableColumn = (obj) => {
    return <TableCell className={"w-[10px] text-sky-100"}  >
      <Switch checked={obj.Enabled} onCheckedChange={() => toggleList(blockLists, "DNSBlockLists", obj)} />
    </TableCell >
  }

  const EnableColumnWhitelist = (obj) => {
    return <TableCell className={"w-[10px] text-sky-100"}  >
      <Switch checked={obj.Enabled} onCheckedChange={() => toggleList(whiteLists, "DNSWhiteLists", obj)} />
    </TableCell >
  }

  let bltable = {
    data: blockLists,
    columns: {
      Tag: true,
      Count: true,
    },
    customColumns: {
      Enabled: EnableColumn,
    },
    columnClass: {
      Enabled: (obj) => obj.Enabled === true ? "text-green-400" : "text-red-400",
    },
    Btn: {
      Edit: (obj) => {
        setIsBlocklistEdit(true)
        setBlocklist(obj)
        setBlocklistModal(true)
      },
      Delete: (obj) => {
        const newList = blockLists.filter(l => l.Tag !== obj.Tag);
        saveConfigMutation.mutate({ ...config, DNSBlockLists: newList });
      },
      New: () => {
        setIsBlocklistEdit(false)
        setBlocklist({
          Tag: "new-blocklist",
          URL: "https://example.com/blocklist.txt",
          Enabled: true,
          Count: 0
        })
        setBlocklistModal(true)
      },
      Save: () => saveConfigMutation.mutate(config)
    },
    headers: ["Tag", "Domains", "Blocked"],
    opts: {
      RowPerPage: 50,
    },
  }

  let wltable = {
    data: whiteLists,
    columns: {
      Tag: true,
      Count: true,
    },
    customColumns: {
      Enabled: EnableColumnWhitelist,
    },
    columnClass: {
      Enabled: (obj) => obj.Enabled === true ? "text-green-400" : "text-red-400",
    },
    Btn: {
      Edit: (obj) => {
        setIsWhitelistEdit(true)
        setWhitelist(obj)
        setWhitelistModal(true)
      },
      Delete: (obj) => {
        const newList = whiteLists.filter(l => l.Tag !== obj.Tag);
        saveConfigMutation.mutate({ ...config, DNSWhiteLists: newList });
      },
      New: () => {
        setIsWhitelistEdit(false)
        setWhitelist({
          Tag: "new-whitelist",
          URL: "https://example.com/whitelist.txt",
          Enabled: true,
          Count: 0
        })
        setWhitelistModal(true)
      },
      Save: () => saveConfigMutation.mutate(config)
    },
    headers: ["Tag", "Domains", "Allowed"],
    opts: {
      RowPerPage: 50,
    },
  }

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

        <TabsContent value="whitelist">
          <GenericTable table={wltable} />
          <NewObjectEditorDialog
            open={whitelistModal}
            onOpenChange={setWhitelistModal}
            object={whitelist}
            title="DNS Whitelist"
            description="Allow specific domains to bypass filters."
            readOnly={false}
            opts={{
              fields: {
                Count: "hidden",
                LastDownload: "hidden"
              }
            }}
            saveButton={async (obj) => {
              let newWhitelists = [...(config?.DNSWhiteLists || [])];
              if (!isWhitelistEdit) {
                newWhitelists.push(obj);
              } else {
                if (isWhitelistEdit) {
                  const index = newWhitelists.findIndex(l => l.Tag === obj.Tag);
                  if (index !== -1) newWhitelists[index] = obj;
                }
              }

              saveConfigMutation.mutate({ ...config, DNSWhiteLists: newWhitelists }, {
                onSuccess: () => {
                  setWhitelistModal(false);
                  setIsWhitelistEdit(false);
                }
              });
            }}
            onChange={(key, value, type) => {
              setWhitelist(prev => ({ ...prev, [key]: value }));
            }}
            onArrayChange={(key, value, index) => {
              setWhitelist(prev => {
                const newArr = [...prev[key]];
                newArr[index] = value;
                return { ...prev, [key]: newArr };
              });
            }}
          />
        </TabsContent>

        <TabsContent value="blocklist">
          <GenericTable table={bltable} />
          <NewObjectEditorDialog
            open={blocklistModal}
            onOpenChange={setBlocklistModal}
            object={blocklist}
            title="DNS Blocklist"
            description="Block domains using external lists or custom rules."
            readOnly={false}
            opts={{
              fields: {
                Count: "hidden",
                LastDownload: "hidden"
              }
            }}
            saveButton={async (obj) => {
              let newBlocklists = [...(config?.DNSBlockLists || [])];
              if (!isBlocklistEdit) {
                newBlocklists.push(obj);
              } else {
                if (isBlocklistEdit) {
                  const index = newBlocklists.findIndex(l => l.Tag === obj.Tag);
                  if (index !== -1) newBlocklists[index] = obj;
                }
              }
              saveConfigMutation.mutate({ ...config, DNSBlockLists: newBlocklists }, {
                onSuccess: () => {
                  setBlocklistModal(false);
                  setIsBlocklistEdit(false);
                }
              });
            }}
            onChange={(key, value, type) => {
              setBlocklist(prev => ({ ...prev, [key]: value }));
            }}
            onArrayChange={(key, value, index) => {
              setBlocklist(prev => {
                const newArr = [...prev[key]];
                newArr[index] = value;
                return { ...prev, [key]: newArr };
              });
            }}
          />
        </TabsContent>

        <TabsContent value="blockdomains">
          <GenericTable table={generateBlocksTable()} />
        </TabsContent>

        <TabsContent value="resolveddomains">
          <GenericTable table={generateResolvesTable()} />
        </TabsContent>

        <TabsContent value="records">
          <GenericTable className={""} table={generateDNSRecordsTable()} />
          <NewObjectEditorDialog
            open={recordModal}
            onOpenChange={setRecordModal}
            object={record}
            title="DNS Record"
            description="Manage custom DNS records."
            readOnly={false}
            saveButton={async (obj) => {
              let newRecords = [...(config?.DNSRecords || [])];
              if (!isRecordEdit) {
                newRecords.push(obj);
              } else {
                const index = newRecords.findIndex(r => r.Domain === obj.Domain);
                if (index !== -1) newRecords[index] = obj;
              }
              saveConfigMutation.mutate({ ...config, DNSRecords: newRecords }, {
                onSuccess: () => {
                  setRecordModal(false);
                  setIsRecordEdit(false);
                }
              });
            }}
            onChange={(key, value, type) => {
              setRecord(prev => ({ ...prev, [key]: value }));
            }}
            onArrayChange={(key, value, index) => {
              setRecord(prev => {
                const newArr = [...prev[key]];
                newArr[index] = value;
                return { ...prev, [key]: newArr };
              });
            }}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default DNS;
