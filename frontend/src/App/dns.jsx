import React, { useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import FormKeyValue from "./component/formkeyvalue";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { Switch } from "@/components/ui/switch";
import GenericTable from "./GenericTable";
import { TableCell } from "@/components/ui/table";
import { useState } from "react";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { Globe } from "lucide-react";
import { Button } from "@/components/ui/button";

const DNSSort = (a, b) => {
  if (dayjs(a.LastSeen).unix() < dayjs(b.LastSeen).unix()) {
    return 1;
  } else if (dayjs(a.LastSeen).unix() > dayjs(b.LastSeen).unix()) {
    return -1;
  }
  return 0;
};

const DNS = () => {
  const state = GLOBAL_STATE("dns");
  const navigate = useNavigate();
  const [record, setRecord] = useState(undefined)
  const [recordModal, setRecordModal] = useState(false)
  const [isRecordEdit, setIsRecordEdit] = useState(false)
  const [blocklist, setBlocklist] = useState(undefined)
  const [blocklistModal, setBlocklistModal] = useState(false)
  const [isBlocklistEdit, setIsBlocklistEdit] = useState(false)
  const [cfg, setCfg] = useState({ ...state.Config })
  const [mod, setMod] = useState(false)

  const updatecfg = (key, value) => {
    console.log(key, value)
    let x = { ...cfg }
    x[key] = value
    setMod(true)
    setCfg(x)
  }

  useEffect(() => {
    state.GetBackendState();
    state.GetDNSStats();
  }, []);

  let blockLists = state.Config?.DNSBlockLists;
  state.modifiedLists?.forEach((l) => {
    blockLists?.forEach((ll, i) => {
      if (ll.Tag === l.Tag) {
        blockLists[i] = l;
      }
    });
  });
  if (!blockLists) {
    blockLists = [];
  }

  const generateDNSRecordsTable = () => {

    return {
      data: state.Config?.DNSRecords,
      columns: {
        Domain: true,
        IP: true,
        TXT: true,
        Wildcard: true,
      },
      headerFormat: {
        TXT: () => {
          return "Text"
        }
      },
      columnFormat: {
        // IP: (obj) => {
        //   return obj.IP.join(" | ")
        // },
        Wildcard: (obj) => {
          return obj.Wildcard === true ? "yes" : "no"
        },
      },
      customColumns: {
      },
      columnClass: {},
      Btn: {
        Edit: (obj) => {
          setIsRecordEdit(true)
          setRecord(obj)
          setRecordModal(true)
        },
        Delete: (obj) => {
          state.Config.DNSRecords = state.Config.DNSRecords.filter((r) => r.Domain !== obj.Domain)
          state.v2_ConfigSave();
        },
        New: () => {
          setIsRecordEdit(false)
          setRecord({ Domain: "yourdomain.com", IP: ["127.0.0.1"], TXT: ["yourdomain.com text record"], Wildcard: true })
          setRecordModal(true)
        }
      },
      headers: ["Domain", "IP", "Text", "Wildcard"],
      headerClass: {},
    }

  }

  const generateBlocksTable = () => {
    let dnsBlocks = state.DNSStats ? state.DNSStats : [];
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
        "tag": () => {
          return "Domain"
        },
        "Tag": () => {
          return "List"
        }
      },
      columnFormat: {
        FirstSeen: (obj) => {
          return dayjs(obj.FirstSeen).format(state.DNSListDateFormat)
        },
        LastSeen: (obj) => {
          return dayjs(obj.LastSeen).format(state.DNSListDateFormat)
        }
      },
      customColumns: {},
      columnClass: {},
      Btn: {},
      headers: ["tag", "Tag", "Count", "FirstSeen", "LastSeen"],
      headerClass: {},
    }
  };

  const generateResolvesTable = () => {
    let dnsResolves = state.DNSStats ? state.DNSStats : [];
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
        tag: () => {
          navigate("/dns/answers/" + value.tag);
        },
        Count: true,
        FirstSeen: true,
        LastSeen: true,
      },
      headerFormat: {
        "tag": () => {
          return "Domain"
        },
      },
      columnFormat: {
        FirstSeen: (obj) => {
          return dayjs(obj.FirstSeen).format(state.DNSListDateFormat)
        },
        LastSeen: (obj) => {
          return dayjs(obj.LastSeen).format(state.DNSListDateFormat)
        }
      },
      customColumns: {},
      columnClass: {},
      Btn: {},
      headers: ["tag", "Count", "FirstSeen", "LastSeen"],
      headerClass: {},
    }

  };

  const EnableColumn = (obj) => {
    return <TableCell className={"w-[10px] text-sky-100"}  >
      <Switch checked={obj.Enabled} onCheckedChange={() => state.toggleBlocklist(obj)} />
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
      Enabled: (obj) => {
        if (obj.Enabled === true) {
          return "text-green-400"
        }
        return "text-red-400"
      },
    },
    Btn: {
      Edit: (obj) => {
        setIsBlocklistEdit(true)
        setBlocklist(obj)
        setBlocklistModal(true)
      },
      Delete: (obj) => {
        state.deleteBlocklist(obj);
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
      Save: state.v2_ConfigSave
    },
    headers: ["Tag", "Domains", "Blocked"],
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
  }

  return (
    <div className="">
      <Tabs defaultValue="settings" >
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
        <TabsList
          className={state.Theme?.borderColor + " rounded"}
        >
          <TabsTrigger className={state.Theme?.tabs} value="settings">Settings</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="records">DNS Records</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="blocklist">Block Lists</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="blockdomains">Blocked Domains</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="resolveddomains">Resovled Domains</TabsTrigger>
        </TabsList>
        <TabsContent value="settings" className="pl-2">
          <div className="">
            <div className="text-yellow-300">
              Enabling blocklists will increase memory usage.
            </div>

            <FormKeyValue
              label="Server IP"
              value={
                <Input
                  value={cfg.DNSServerIP}
                  onChange={(e) => {
                    updatecfg("DNSServerIP", e.target.value)
                    // state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Server Port"
              value={
                <Input
                  value={cfg.DNSServerPort}
                  onChange={(e) => {
                    updatecfg("DNSServerPort", e.target.value)
                    // state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Primary DNS"
              value={
                <Input
                  value={cfg.DNS1Default}
                  onChange={(e) => {
                    updatecfg("DNS1Default", e.target.value)
                    // state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Backup DNS"
              value={
                <Input
                  value={cfg.DNS2Default}
                  onChange={(e) => {
                    updatecfg("DNS2Default", e.target.value)
                    // state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />
            <div className="max-w-[300px]">
              <div className="flex items-center justify-between py-1 w-full">
                <Label className="text-white mr-3">Secure DNS</Label>
                <Switch
                  checked={state?.Config?.DNSOverHTTPS}
                  onCheckedChange={() => {
                    state.toggleConfigKeyAndSave("Config", "DNSOverHTTPS");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">Log Blocked</Label>
                <Switch
                  checked={state?.Config?.LogBlockedDomains}
                  onCheckedChange={() => {
                    state.toggleConfigKeyAndSave("Config", "LogBlockedDomains");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">Log All</Label>
                <Switch
                  checked={state?.Config?.LogAllDomains}
                  onCheckedChange={() => {
                    state.toggleConfigKeyAndSave("Config", "LogAllDomains");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">DNS Stats</Label>
                <Switch
                  checked={state?.Config?.DNSstats}
                  onCheckedChange={() => {
                    state.toggleConfigKeyAndSave("Config", "DNSstats");
                    state.fullRerender();
                  }}
                />
              </div>
            </div>
          </div>

        </TabsContent>
        <TabsContent value="blocklist">
          <GenericTable table={bltable} />
          <NewObjectEditorDialog
            open={blocklistModal}
            onOpenChange={setBlocklistModal}
            object={blocklist}
            title="DNS Blocklist"
            description=""
            readOnly={false}
            opts={{
              fields: {
                Count: "hidden"
              }
            }}
            saveButton={async (obj) => {
              if (!isBlocklistEdit) {
                if (!state.Config?.DNSBlockLists) {
                  state.Config.DNSBlockLists = []
                }
                state.Config?.DNSBlockLists.push(obj)
              }
              let ok = await state.v2_ConfigSave();
              if (ok === true) {
                setBlocklistModal(false)
                setIsBlocklistEdit(false)
              }
            }}
            onChange={(key, value, type) => {
              blocklist[key] = value;
            }}
            onArrayChange={(key, value, index) => {
              blocklist[key][index] = value;
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
            description=""
            readOnly={false}
            saveButton={async (obj) => {
              if (!isRecordEdit) {
                if (!state.Config?.DNSRecords) {
                  state.Config.DNSRecords = []
                }
                state.Config?.DNSRecords.push(obj)
              }
              let ok = await state.v2_ConfigSave();
              if (ok === true) {
                setRecordModal(false)
                setIsRecordEdit(false)
              }
            }}
            onChange={(key, value, type) => {
              record[key] = value;
            }}
            onArrayChange={(key, value, index) => {
              record[key][index] = value;
            }}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default DNS;
