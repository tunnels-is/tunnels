import React, { useEffect, useState } from "react";

import GLOBAL_STATE from "../state";
import ConfigDNSRecordEditor from "./component/ConfigDNSRecordEditor";
import STORE from "../store";
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
import { useNavigate } from "react-router-dom";


const Groups = () => {
  const state = GLOBAL_STATE("dns");
  const [groups, setGroups] = useState([])
  const navigate = useNavigate()


  let getGroups = async () => {
    let resp = await state.DoStuff(null, null, "POST", "/v3/group/list", {}, false, false)
    if (resp.status === 200) {
      setGroups(resp.data)
    } else {
      state.toggleError("unable to list groups")
    }
  }

  useEffect(() => {
    // state.GetBackendState();
    getGroups()
  }, []);


  const onFieldChange = (i, field, value) => {
    groups[i][field] = value
    state.renderPage("dns");
  }

  const saveGroup = async (i) => {
    let resp = undefined
    if (groups[i]._id !== undefined) {
      resp = await state.DoStuff(null, null, "POST", "/v3/group/update", { Group: groups[i], }, false, false)
    } else {
      resp = await state.DoStuff(null, null, "POST", "/v3/group/create", { Group: groups[i], }, false, false)
    }

    if (resp.status === 200) {
      groups[i] = resp.data
    } else {
      state.toggleError("unable to create group")
    }
    state.renderPage("dns");
  }

  const newGroup = () => {
    groups.push({ Tag: "my-new-group", Description: "This is a new group" })
    state.renderPage("dns");
  }

  const deleteGroup = async (i) => {
    let resp = await state.DoStuff(null, null, "POST", "/v3/group/delete", { GID: groups[i]._id, }, false, false)
    if (resp.status === 200) {
      delete groups[i]
    }
    setGroups([...groups])
  }

  const FormField = ({ label, children }) => (
    <div className="grid gap-2 mb-4">
      <Label className="text-sm font-medium">{label}</Label>
      {children}
    </div>
  );

  return (
    <div className="groups-page">

      <div className="w-full max-w-4xl mx-auto p-4 space-y-6">
        <div className="flex items-center justify-between mb-6">
          <Button
            onClick={() => newGroup()}
            variant="outline"
            className="flex items-center gap-2 text-white"
          >
            <PlusCircle className="h-4 w-4" />
            <span>New Group</span>
          </Button>
        </div>

        <div className=" space-y-6">
          {groups && groups?.map((g, i) => {
            if (!g) return null;

            return (
              <div
                key={`group-${i}`}
                onClick={() => navigate("/inspect/group/" + g._id)}

                className="w-full flex flex-wrap items-center gap-3 bg-black p-4 rounded-lg border border-gray-800 mb-4 text-white"
              >
                <div className="flex items-center gap-[10px]">
                  <Server className="h-4 w-4 text-emerald-500" />
                  <div>
                    <span className="font-bold block text-sm">{g.Tag}</span>
                    <span className="text-gray-400 block text-sm">
                      {g.Description}
                    </span>
                  </div>
                </div>




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
                        {g.Tag}
                      </h3>
                      <Label className="text-sm font-medium">{g._id}</Label>
                    </div>

                    <div className="space-y-6">
                      <FormField label="Tag">
                        <Input
                          value={g.Tag}
                          onChange={(e) =>
                            onFieldChange(i, "Tag", e.target.value)
                          }
                          placeholder="e.g. example.com"
                          className="w-full bg-gray-950 border-gray-700 text-white"
                        />
                      </FormField>

                      <FormField label="Description">
                        <Input
                          value={g.Description}
                          onChange={(e) =>
                            onFieldChange(i, "Description", e.target.value)
                          }
                          placeholder="e.g. subdomain.example.com"
                          className="w-full bg-gray-950 border-gray-700 text-white"
                        />
                      </FormField>

                    </div>

                    <div className="flex justify-between mt-6 pt-4 border-t border-gray-800">
                      <Button
                        variant="outline"
                        className="flex items-center gap-2 bg-gray-950 border-gray-700 hover:bg-gray-700"
                        onClick={() => saveGroup(i)}
                      >
                        <Save className="h-4 w-4" />
                        Save
                      </Button>

                      <Button
                        variant="destructive"
                        className="flex items-center gap-2 bg-red-900 hover:bg-red-800"
                        onClick={() => deleteGroup(i)}
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

          {(!groups ||
            groups.length === 0) && (
              <div className="text-center p-12 border border-dashed rounded-lg bg-muted/30">
                <p className="text-muted-foreground">
                  No groups found. Add your first group to get started.
                </p>
              </div>
            )}
        </div>
      </div >
    </div >
  )

};

export default Groups;
