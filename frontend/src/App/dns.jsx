import React, { useEffect } from "react";
import { Button } from "@/components/ui/button";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import CustomToggle from "./component/CustomToggle";
import FormKeyValue from "./component/formkeyvalue";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import dayjs from "dayjs";
import { Check, ExternalLink, Tag, Trash2, X } from "lucide-react";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { Switch } from "@/components/ui/switch";
import GenericTable from "./GenericTable";
import { TableCell } from "@/components/ui/table";
import { useState } from "react";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { Badge } from "@/components/ui/badge";

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


  let LogBlockedDomains = state.getKey("Config", "LogBlockedDomains");
  let LogAllDomains = state.getKey("Config", "LogAllDomains");
  let dnsStats = state.getKey("Config", "DNSstats");

  let DNS1 = state.getKey("Config", "DNS1Default");
  let DNS2 = state.getKey("Config", "DNS2Default");

  let DNSServerIP = state.getKey("Config", "DNSServerIP");
  let DNSOverHTTPS = state.getKey("Config", "DNSOverHTTPS");
  let DNSServerPort = state.getKey("Config", "DNSServerPort");

  let modified = STORE.Cache.GetBool("modified_Config");

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

  const addRecord = () => {
    if (!state.Config.DNSRecords) {
      state.Config.DNSRecords = [];
    }
    state.Config.DNSRecords.push({
      Domain: "domain.local",
      IP: [""],
      TXT: [""],
      Wildcard: true,
    });
  };

  const generateDNSRecordsTable = () => {

    return {
      data: state.Config?.DNSRecords,
      columns: {
        Domain: true,
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
        // TXT: (obj) => {
        //   return obj.TXT.join(" | ")
        // },
      },
      customColumns: {
        IP: (obj) => {
          return <TableCell className={""}>
            <div className="flex flex-col">
              {obj.IP?.map(ip => {
                return <Badge className={"mt-1 size-fit" + state.Theme?.badgeNeutral} > {ip}</Badge>
              })}
            </div>
          </TableCell >
        },
        TXT: (obj) => {
          return <TableCell className={""}>
            <div className="flex flex-col">
              {obj.TXT?.map((txt) => {
                return <Badge className={"mt-1 size-fit" + state.Theme?.badgeNeutral} > {txt}</Badge>
              })}
            </div>
          </TableCell >
        },
        WildCard: (obj) => {
          return <TableCell className={""}>
            {(obj.Wildcard === true) &&
              <Badge className={"mt-1 size-fit" + state.Theme?.badgeSuccess} > yes</Badge>
            }
            {(obj.Wildcard === false) &&
              <Badge className={"mt-1 size-fit" + state.Theme?.badgeWarning} > no</Badge>
            }
          </TableCell >
        }
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
      Delete: (obj) => {
        state.deleteBlocklist(obj);
      },
      New: () => { },
    },
    headers: ["Tag", "Domain Count", "Enabled"],
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
  }

  return (
    <div className="">
      {modified === true && (
        <div className="mb-7 flex gap-[4px] items-center">
          <Button
            variant="outline"
            className={state.Theme?.successBtn}
            onClick={() => state.v2_ConfigSave()}>
            Save
          </Button>
          <div className="ml-3 text-yellow-400 text-xl">
            Your config has un-saved changes
          </div>
        </div>
      )}
      <Tabs defaultValue="settings" >
        <TabsList
          className={state.Theme?.borderColor}
        >
          <TabsTrigger className={state.Theme?.tabs} value="settings">Settings</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="records">Records</TabsTrigger>
          <TabsTrigger className={state.Theme?.tabs} value="blocklist">Block List</TabsTrigger>
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
                  value={DNSServerIP}
                  onChange={(e) => {
                    state.setKeyAndReloadDom(
                      "Config",
                      "DNSServerIP",
                      e.target.value,
                    );
                    state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Server Port"
              value={
                <Input
                  value={DNSServerPort}
                  onChange={(e) => {
                    state.setKeyAndReloadDom(
                      "Config",
                      "DNSServerPort",
                      e.target.value,
                    );
                    state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Primary DNS"
              value={
                <Input
                  value={DNS1}
                  onChange={(e) => {
                    state.setKeyAndReloadDom(
                      "Config",
                      "DNS1Default",
                      e.target.value,
                    );
                    state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />

            <FormKeyValue
              label="Backup DNS"
              value={
                <Input
                  value={DNS2}
                  onChange={(e) => {
                    state.setKeyAndReloadDom(
                      "Config",
                      "DNS2Default",
                      e.target.value,
                    );
                    state.renderPage("dns");
                  }}
                  type="text"
                />
              }
            />
            <div className="max-w-[300px]">
              <div className="flex items-center justify-between py-1 w-full">
                <Label className="text-white mr-3">Secure DNS</Label>
                <Switch
                  checked={DNSOverHTTPS}
                  onCheckedChange={() => {
                    state.toggleKeyAndReloadDom("Config", "DNSOverHTTPS");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">Log Blocked</Label>
                <Switch
                  checked={LogBlockedDomains}
                  onCheckedChange={() => {
                    state.toggleKeyAndReloadDom("Config", "LogBlockedDomains");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">Log All</Label>
                <Switch
                  checked={LogAllDomains}
                  onCheckedChange={() => {
                    state.toggleKeyAndReloadDom("Config", "LogAllDomains");
                    state.fullRerender();
                  }}
                />
              </div>

              <div className="flex items-center justify-between py-1">
                <Label className="text-white mr-3">DNS Stats</Label>
                <Switch
                  checked={dnsStats}
                  onCheckedChange={() => {
                    state.toggleKeyAndReloadDom("Config", "DNSstats");
                    state.fullRerender();
                  }}
                />
              </div>
            </div>
          </div>

        </TabsContent>
        <TabsContent value="blocklist">
          <GenericTable table={bltable} />
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
                if (!state.Config?.DNSRecord) {
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
