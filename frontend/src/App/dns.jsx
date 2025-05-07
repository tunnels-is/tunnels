import React, { useEffect } from "react";
import { Button } from "@/components/ui/button";

import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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

  const default_lists = [
    "Ads",
    "AdultContent",
    "CryptoCurrency",
    "Drugs",
    "FakeNews",
    "Fraud",
    "Gambling",
    "Malware",
    "SocialMedia",
    "Surveillance",
  ];

  const isDefault = (tag) => {
    return default_lists.includes(tag);
  };

  const generateListTable = (blockLists) => {
    let rows = [];
    blockLists.forEach((i) => {
      let row = {};
      row.items = [
        {
          type: "text",
          value: (
            <div
              className={`${i.Enabled ? "enabled" : "disabled"} clickable`}
              onClick={() => {
                state.toggleBlocklist(i);
              }}
            >
              {" "}
              {i.Enabled ? "Blocked" : "Allowed"}
            </div>
          ),
        },
        { type: "text", value: i.Tag },
        { type: "text", value: i.Count },
        {
          type: "text",
          value: (
            <div
              className={`${isDefault(i.Tag) ? "disabled" : "red"} clickable`}
              onClick={() => {
                state.deleteBlocklist(i);
              }}
            >
              Remove
            </div>
          ),
        },
      ];
      rows.push(row);
    });
    return rows;
  };

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

    stats.forEach((value) => {
      let row = {};
      row.items = [
        { type: "text", value: value.tag, tooltip: true },
        { type: "text", value: value.Tag },
        {
          type: "text",
          value: dayjs(value.FirstSeen).format(state.DNSListDateFormat),
        },
        {
          type: "text",
          value: dayjs(value.LastSeen).format(state.DNSListDateFormat),
        },
        { type: "text", value: value.Count },
      ];
      rows.push(row);
    });
    return rows;
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

    stats.forEach((value) => {
      let row = {};
      row.items = [
        {
          tooltip: true,
          type: "text",
          value: value.tag,
          color: "blue",
          width: 30,
          click: () => {
            navigate("/dns/answers/" + value.tag);
          },
        },
        {
          type: "text",
          value: dayjs(value.FirstSeen).format(state.DNSListDateFormat),
        },
        {
          type: "text",
          value: dayjs(value.LastSeen).format(state.DNSListDateFormat),
        },
        { type: "text", value: value.Count },
      ];
      rows.push(row);
    });
    return rows;
  };

  let rows = generateListTable(blockLists);
  const headers = [
    { value: "Enabled" },
    { value: "Tag" },
    { value: "Domains" },
    { value: "" },
  ];

  let rowsDNSstats = generateBlocksTable();
  const headersDNSstats = [
    { value: "Domain" },
    { value: "List" },
    { value: "First Seen" },
    { value: "Last Seen" },
    { value: "Blocked" },
  ];

  let rowsDNSresolves = generateResolvesTable();
  const headerDNSresolves = [
    { value: "Domain", width: 30 },
    { value: "First Seen" },
    { value: "Last Seen" },
    { value: "Resolved" },
  ];

  const customRowBlockList = (row) => {
    const [status, tag, count, remove] = row.items;
    const isEnabled = status.value.props.className.includes("enabled");

    return (
      <div className="group mb-3 p-2 rounded-2xl bg-black border border-slate-800 shadow-sm hover:shadow-md transition-all duration-300 flex justify-between items-center gap-4 relative overflow-hidden">
        <div className="flex items-center gap-6 z-10">
          <div
            className={`relative flex items-center gap-1.5 text-xs font-medium px-3.5 py-2 rounded-full w-fit cursor-pointer transition-all duration-300 ${
              isEnabled
                ? "bg-emerald-800 text-emerald-300 hover:bg-emerald-700"
                : "bg-amber-800 text-amber-300 hover:bg-amber-700"
            }`}
            onClick={status.value.props.onClick}
          >
            <span
              className={`flex items-center justify-center w-4 h-4 rounded-full ${
                isEnabled ? "bg-emerald-600" : "bg-amber-600"
              }`}
            >
              {isEnabled ? (
                <Check className="w-2.5 h-2.5 text-white" />
              ) : (
                <X className="w-2.5 h-2.5 text-white" />
              )}
            </span>
            <span className="font-semibold">{status.value.props.children}</span>
          </div>

          <div className="flex items-center gap-2.5">
            <div className="flex items-center justify-center w-7 h-7 rounded-full bg-slate-800">
              <Tag className="w-3.5 h-3.5 text-slate-300" />
            </div>
            <div className="text-sm font-semibold text-slate-200 truncate max-w-[180px]">
              {tag.value}
            </div>
          </div>

          <div className="flex items-center gap-2">
            <div className="flex items-center justify-center w-7 h-7 rounded-full bg-slate-800">
              <ExternalLink className="w-3.5 h-3.5 text-slate-300" />
            </div>
            <div className="flex items-center gap-1 text-sm text-slate-400">
              <span className="font-semibold text-slate-300">
                {count.value}
              </span>
              <span>domains</span>
            </div>
          </div>
        </div>

        <div className="flex items-center z-10">
          <button
            className={`flex items-center gap-1.5 px-4 py-2 rounded-lg transition-all duration-300 ${
              remove.value.props.className.includes("disabled")
                ? "bg-slate-800 text-slate-600 cursor-not-allowed"
                : "bg-red-800 text-red-300 hover:bg-red-700 group-hover:shadow-sm"
            }`}
            onClick={remove.value.props.onClick}
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span className="text-sm font-medium">
              {remove.value.props.children}
            </span>
          </button>
        </div>
      </div>
    );
  };

  return (
    <div className="">
      {modified === true && (
        <div className="mb-7 flex gap-[4px] items-center">
          <Button variant="secondary" onClick={() => state.v2_ConfigSave()}>
            Save
          </Button>
          <div className="text-yellow-400 text-xl">
            Your config has un-saved changes
          </div>
        </div>
      )}
      <Tabs defaultValue="settings">
        <TabsList className="">
          <TabsTrigger value="settings">Settings</TabsTrigger>
          <TabsTrigger value="blocklist">Block List</TabsTrigger>
          <TabsTrigger value="blockdomains">Blocked Domains</TabsTrigger>
          <TabsTrigger value="resolveddomains">Resovled Domains</TabsTrigger>
        </TabsList>
        <TabsContent value="settings">
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
          <NewTable
            tableID="dns-lists"
            className="!p-0 mt-5"
            background={true}
            header={headers}
            rows={rows}
            button={{
              text: "New Blocklist",
              click: function () {
                navigate("/inspect/blocklist");
              },
            }}
            customRow={(row) => customRowBlockList(row)}
          />
        </TabsContent>
        <TabsContent value="blockdomains">
          {dnsStats && (
            <>
              <NewTable
                tableID="dns-blocked"
                className="dns-stats"
                background={true}
                header={headersDNSstats}
                rows={rowsDNSstats}
              />
            </>
          )}
        </TabsContent>
        <TabsContent value="resolveddomains">
          {dnsStats && (
            <>
              <NewTable
                tableID="dns-resolved"
                className="dns-stats"
                background={true}
                header={headerDNSresolves}
                rows={rowsDNSresolves}
              />
            </>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default DNS;
