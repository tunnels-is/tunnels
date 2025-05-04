import React from "react";
import GLOBAL_STATE from "../../state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Edit,
  FileText,
  Network,
  Plus,
  Save,
  Server,
  Trash2,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { PlusCircle } from "lucide-react";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Check } from "lucide-react";

const ConfigDNSRecordEditor = () => {
  const state = GLOBAL_STATE("DNSRecordForm");

  const addRecord = () => {
    if (!state.Config.DNSRecords) {
      state.Config.DNSRecords = [];
    }
    state.Config?.DNSRecords.push({
      Domain: "domain.local",
      IP: [""],
      TXT: [""],
      CNAME: "",
      Wildcard: true,
    });
    state.renderPage("DNSRecordForm");
  };
  const saveAll = () => {
    state.Config.DNSRecords?.forEach((r, i) => {
      r.IP?.forEach((ip, ii) => {
        if (ip === "") {
          state.Config.DNSRecords[i].IP.splice(ii, 1);
        }
      });
      r.TXT?.forEach((txt, ii) => {
        if (txt === "") {
          state.Config.DNSRecords[i].TXT.splice(ii, 1);
        }
      });
    });
    state.ConfigSave();
    state.renderPage("DNSRecordForm");
  };
  const deleteRecord = (index) => {
    if (state.Config.DNSRecords.length === 1) {
      state.Config.DNSRecords = [];
      state.v2_ConfigSave();
      state.globalRerender();
    } else {
      state.Config.DNSRecords.splice(index, 1);
      state.v2_ConfigSave();
      state.globalRerender();
    }
  };

  const updateRecord = (index, subindex, key, value) => {
    console.log("update:", index, subindex, key, value);
    if (key === "IP") {
      try {
        state.Config.DNSRecords[index].IP[subindex] = value;
      } catch (error) {
        console.dir(error);
      }
    } else if (key === "TXT") {
      try {
        state.Config.DNSRecords[index].TXT[subindex] = value;
      } catch (error) {
        console.dir(error);
      }
    } else if (key === "Wildcard") {
      state.Config.DNSRecords[index].Wildcard = value;
    } else {
      state.Config.DNSRecords[index][key] = value;
    }

    state.renderPage("DNSRecordForm");
  };

  const addIP = (index) => {
    state.Config?.DNSRecords[index].IP.push("0.0.0.0");
    state.renderPage("DNSRecordForm");
  };
  const addTXT = (index) => {
    state.Config?.DNSRecords[index].TXT.push("new text record");
    state.renderPage("DNSRecordForm");
  };

  const FormField = ({ label, children }) => (
    <div className="grid gap-2 mb-4">
      <Label className="text-sm font-medium">{label}</Label>
      {children}
    </div>
  );

  return (
    <div className="w-full max-w-4xl mx-auto p-4 space-y-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold tracking-tight text-white">
          DNS Records
        </h2>
        <Button
          onClick={addRecord}
          variant="outline"
          className="flex items-center gap-2 text-white"
        >
          <PlusCircle className="h-4 w-4" />
          <span>Add DNS Record</span>
        </Button>
      </div>

      <div className=" space-y-6">
        {state.Config?.DNSRecords?.map((r, i) => {
          if (!r) return null;

          return (
            <div
              key={`dns-record-${i}`}
              className="w-full flex flex-wrap items-center gap-3 bg-black p-4 rounded-lg border border-gray-800 mb-4 text-white"
            >
              <div className="flex items-center gap-[10px]">
                <Server className="h-4 w-4 text-emerald-500" />
                <div>
                  <span className="font-bold block text-sm">{r.Domain}</span>
                  <span className="text-gray-400 block text-sm">
                    CNAME: {r.CNAME || "Not Found"}
                  </span>
                </div>
              </div>

              {r.Wildcard && (
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="bg-green-700 text-white">
                    Wildcard
                    <Check className="h-4 w-4 text-white" />
                  </Badge>
                </div>
              )}

              <Dialog>
                <DialogTrigger asChild>
                  <Button
                    variant="secondary"
                    size="sm"
                    className="ml-auto bg-gray-800 hover:bg-gray-700"
                  >
                    <Edit className="h-4 w-4 mr-1" /> Edit
                  </Button>
                </DialogTrigger>

                <DialogContent className="bg-black border border-gray-800 text-white max-w-2xl rounded-lg p-6">
                  <div className="bg-gray-800/50 -m-6 mb-6 p-4 border-b border-gray-800">
                    <h3 className="text-lg font-medium flex items-center gap-2">
                      DNS Record {i + 1}
                      {r.Wildcard && (
                        <Badge
                          variant="outline"
                          className="ml-2 border-amber-800 bg-amber-900/30 text-amber-400"
                        >
                          Wildcard
                        </Badge>
                      )}
                    </h3>
                  </div>

                  <div className="space-y-6">
                    <FormField label="Domain">
                      <Input
                        value={r.Domain}
                        onChange={(e) =>
                          updateRecord(i, 0, "Domain", e.target.value)
                        }
                        placeholder="e.g. example.com"
                        className="w-full bg-gray-950 border-gray-700 text-white"
                      />
                    </FormField>

                    <FormField label="CNAME">
                      <Input
                        value={r.CNAME}
                        onChange={(e) =>
                          updateRecord(i, 0, "CNAME", e.target.value)
                        }
                        placeholder="e.g. subdomain.example.com"
                        className="w-full bg-gray-950 border-gray-700 text-white"
                      />
                    </FormField>

                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Network className="h-4 w-4 text-blue-500" />
                          <Label className="text-sm font-medium text-gray-300">
                            IP Addresses
                          </Label>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => addIP(i)}
                          className="h-8 text-xs bg-gray-950 border-gray-700 hover:bg-gray-700 hover:text-white"
                        >
                          <Plus className="h-3 w-3 mr-1" /> Add IP
                        </Button>
                      </div>

                      {r.IP?.map((ip, ii) => (
                        <Input
                          key={`ip-${i}-${ii}`}
                          value={ip}
                          onChange={(e) =>
                            updateRecord(i, ii, "IP", e.target.value)
                          }
                          placeholder="e.g. 192.168.1.1"
                          className="w-full bg-gray-950 border-gray-700 text-white"
                        />
                      ))}
                    </div>

                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <FileText className="h-4 w-4 text-purple-500" />
                          <Label className="text-sm font-medium text-gray-300">
                            TXT Records
                          </Label>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => addTXT(i)}
                          className="h-8 text-xs bg-gray-950 border-gray-700 hover:bg-gray-700 hover:text-white"
                        >
                          <Plus className="h-3 w-3 mr-1" /> Add TXT
                        </Button>
                      </div>

                      {r.TXT?.map((txt, ii) => (
                        <Textarea
                          key={`txt-${i}-${ii}`}
                          value={txt}
                          onChange={(e) =>
                            updateRecord(i, ii, "TXT", e.target.value)
                          }
                          placeholder="Enter text record"
                          className="w-full min-h-[80px] bg-gray-950 border-gray-700 text-white"
                        />
                      ))}
                    </div>

                    <div className="flex items-center space-x-3">
                      <Switch
                        id={`wildcard-${i}`}
                        checked={r.Wildcard}
                        onCheckedChange={(checked) =>
                          updateRecord(i, 0, "Wildcard", checked)
                        }
                      />
                      <Label
                        htmlFor={`wildcard-${i}`}
                        className="text-gray-300"
                      >
                        Wildcard Domain
                      </Label>
                    </div>
                  </div>

                  <div className="flex justify-between mt-6 pt-4 border-t border-gray-800">
                    <Button
                      variant="outline"
                      className="flex items-center gap-2 bg-gray-950 border-gray-700 hover:bg-gray-700"
                      onClick={saveAll}
                    >
                      <Save className="h-4 w-4" />
                      Save
                    </Button>

                    <Button
                      variant="destructive"
                      className="flex items-center gap-2 bg-red-900 hover:bg-red-800"
                      onClick={() => deleteRecord(i)}
                    >
                      <Trash2 className="h-4 w-4" />
                      Remove
                    </Button>
                  </div>
                </DialogContent>
              </Dialog>
            </div>
          );
        })}

        {(!state.Config?.DNSRecords ||
          state.Config.DNSRecords.length === 0) && (
            <div className="text-center p-12 border border-dashed rounded-lg bg-muted/30">
              <p className="text-muted-foreground">
                No DNS records found. Add your first record to get started.
              </p>
            </div>
          )}
      </div>
    </div>
  );
};

export default ConfigDNSRecordEditor;
