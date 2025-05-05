import React, { useState } from "react";
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
  Check,
  PlusCircle,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";


const ConfigDNSRecordEditor = () => {
  const state = GLOBAL_STATE("DNSRecordForm");
  const [selectedIndex, setSelectedIndex] = useState(null);

  const addRecord = () => {
    if (!state.Config.DNSRecords) {
      state.Config.DNSRecords = [];
    }
    state.Config.DNSRecords.push({
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
      r.IP = r.IP?.filter((ip) => ip !== "");
      r.TXT = r.TXT?.filter((txt) => txt !== "");
    });
    state.ConfigSave();
    state.renderPage("DNSRecordForm");
  };

  const deleteRecord = (index) => {
    if (state.Config.DNSRecords.length === 1) {
      state.Config.DNSRecords = [];
    } else {
      state.Config.DNSRecords.splice(index, 1);
    }
    state.v2_ConfigSave();
    state.globalRerender();
  };

  const updateRecord = (index, subindex, key, value) => {
    if (key === "IP") {
      state.Config.DNSRecords[index].IP[subindex] = value;
    } else if (key === "TXT") {
      state.Config.DNSRecords[index].TXT[subindex] = value;
    } else if (key === "Wildcard") {
      state.Config.DNSRecords[index].Wildcard = value;
    } else {
      state.Config.DNSRecords[index][key] = value;
    }
    state.renderPage("DNSRecordForm");
  };

  const addIP = (index) => {
    state.Config.DNSRecords[index].IP.push("0.0.0.0");
    state.renderPage("DNSRecordForm");
  };

  const addTXT = (index) => {
    state.Config.DNSRecords[index].TXT.push("new text record");
    state.renderPage("DNSRecordForm");
  };

  const openDialog = (index) => {
    setSelectedIndex(index);
  };

  const closeDialog = () => {
    setSelectedIndex(null);
  };

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

      <div className="space-y-6">
        {state.Config?.DNSRecords?.map((r, i) => (
          <div
            key={i}
            className="w-full flex flex-wrap items-center gap-3 bg-black p-4 rounded-lg border border-gray-800 mb-4 text-white"
          >
            <div className="flex items-center gap-3">
              <Server className="h-4 w-4 text-emerald-500" />
              <div>
                <span className="font-bold block text-sm">{r.Domain}</span>
                <span className="text-gray-400 block text-sm">
                  CNAME: {r.CNAME || "Not Found"}
                </span>
              </div>
            </div>
            {r.Wildcard && (
              <Badge variant="outline" className="bg-green-700 text-white">
                Wildcard <Check className="h-4 w-4" />
              </Badge>
            )}
            <Button
              variant="secondary"
              size="sm"
              className="ml-auto bg-gray-800 hover:bg-gray-700"
              onClick={() => openDialog(i)}
            >
              <Edit className="h-4 w-4 mr-1" /> Edit
            </Button>
          </div>
        ))}
        {(!state.Config?.DNSRecords ||
          state.Config.DNSRecords.length === 0) && (
            <div className="text-center p-12 border border-dashed rounded-lg bg-muted/30">
              <p className="text-muted-foreground">
                No DNS records found. Add your first record to get started.
              </p>
            </div>
          )}
      </div>

      {selectedIndex !== null && (
        <DNSRecordDialog
          record={state.Config.DNSRecords[selectedIndex]}
          index={selectedIndex}
          updateRecord={updateRecord}
          addIP={addIP}
          addTXT={addTXT}
          saveAll={saveAll}
          deleteRecord={deleteRecord}
          closeDialog={closeDialog}
        />
      )}
    </div>
  );
};

const DNSRecordDialog = ({
  record,
  index,
  updateRecord,
  addIP,
  addTXT,
  saveAll,
  deleteRecord,
  closeDialog,
}) => {
  return (
    <Dialog open={true} onOpenChange={closeDialog}>
      <DialogContent className="bg-black border border-gray-800 text-white max-w-2xl rounded-lg p-6">
        <div className="bg-gray-800/50 -m-6 mb-6 p-4 border-b border-gray-800">
          <h3 className="text-lg font-medium flex items-center gap-2">
            DNS Record {index + 1}
            {record.Wildcard && (
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
              value={record.Domain}
              onChange={(e) => updateRecord(index, 0, "Domain", e.target.value)}
              placeholder="e.g. example.com"
              className="w-full bg-gray-950 border-gray-700 text-white"
            />
          </FormField>

          <FormField label="CNAME">
            <Input
              value={record.CNAME}
              onChange={(e) => updateRecord(index, 0, "CNAME", e.target.value)}
              placeholder="e.g. subdomain.example.com"
              className="w-full bg-gray-950 border-gray-700 text-white"
            />
          </FormField>

          <RecordList
            label="IP Addresses"
            values={record.IP}
            index={index}
            onAdd={addIP}
            onChange={updateRecord}
            type="IP"
            icon={<Network className="h-4 w-4 text-blue-500" />}
          />

          <RecordList
            label="TXT Records"
            values={record.TXT}
            index={index}
            onAdd={addTXT}
            onChange={updateRecord}
            type="TXT"
            icon={<FileText className="h-4 w-4 text-purple-500" />}
            isTextarea
          />

          <div className="flex items-center space-x-3">
            <Switch
              id={`wildcard-${index}`}
              checked={record.Wildcard}
              onCheckedChange={(checked) =>
                updateRecord(index, 0, "Wildcard", checked)
              }
            />
            <Label htmlFor={`wildcard-${index}`} className="text-gray-300">
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
            <Save className="h-4 w-4" /> Save
          </Button>

          <Button
            variant="destructive"
            className="flex items-center gap-2 bg-red-900 hover:bg-red-800"
            onClick={() => deleteRecord(index)}
          >
            <Trash2 className="h-4 w-4" /> Delete
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
};

const FormField = ({ label, children }) => (
  <div className="grid gap-2 mb-4">
    <Label className="text-sm font-medium">{label}</Label>
    {children}
  </div>
);

const RecordList = ({
  label,
  values,
  index,
  onAdd,
  onChange,
  type,
  icon,
  isTextarea = false,
}) => (
  <div className="space-y-3">
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-2">
        {icon}
        <Label className="text-sm font-medium text-gray-300">{label}</Label>
      </div>
      <Button
        size="sm"
        variant="outline"
        onClick={() => onAdd(index)}
        className="h-8 text-xs bg-gray-950 border-gray-700 hover:bg-gray-700 hover:text-white"
      >
        <Plus className="h-3 w-3 mr-1" /> Add {type}
      </Button>
    </div>
    {values?.map((val, ii) =>
      isTextarea ? (
        <Textarea
          key={`${type}-${index}-${ii}`}
          value={val}
          onChange={(e) => onChange(index, ii, type, e.target.value)}
          placeholder={`Enter ${type} record`}
          className="w-full min-h-[80px] bg-gray-950 border-gray-700 text-white"
        />
      ) : (
        <Input
          key={`${type}-${index}-${ii}`}
          value={val}
          onChange={(e) => onChange(index, ii, type, e.target.value)}
          placeholder={`e.g. ${type === "IP" ? "192.168.1.1" : "example"}`}
          className="w-full bg-gray-950 border-gray-700 text-white"
        />
      ),
    )}
  </div>
);

export default ConfigDNSRecordEditor;
