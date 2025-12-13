import React, { useState, useMemo } from "react";
import { useParams } from "react-router-dom";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { useGroup, useGroupEntities, useAddEntityToGroup, useRemoveEntityFromGroup } from "@/hooks/useGroups";
import { toast } from "sonner";
import { useServers } from "@/hooks/useServers";
import { useAtomValue } from "jotai";
import { userAtom } from "@/stores/userStore";
import { Button } from "@/components/ui/button";
import { Plus, Trash2 } from "lucide-react";

export default function InspectGroup() {
  const { id } = useParams()
  const user = useAtomValue(userAtom);
  const { data: group } = useGroup(id);
  const [dialog, setDialog] = useState(false)
  const [addForm, setAddForm] = useState({})
  const [tag, setTag] = useState("user") // Default to user to match useEffect

  // Fetch entities based on current tab
  const { data: users } = useGroupEntities(id, "user", 0, 1000);
  const { data: servers } = useGroupEntities(id, "server", 0, 1000);
  const { data: devices } = useGroupEntities(id, "device", 0, 1000);

  // Fetch all servers to map IDs to Tags (if needed for display)
  const { data: allServers } = useServers(user?.ControlServer);

  const addEntityMutation = useAddEntityToGroup();
  const removeEntityMutation = useRemoveEntityFromGroup();

  const addToGroup = async (values) => {
    const form = values || addForm;
    console.log("ID:", form.id)
    try {
      await addEntityMutation.mutateAsync({
        groupId: id,
        typeId: form.id,
        type: form.type,
        typeTag: form.idtype,
      });
      setDialog(false);
      toast.success("Added to group");
    } catch (e) {
      toast.error("Failed to add to group");
    }
  }

  const removeEntity = (gid, typeid, type) => {
    removeEntityMutation.mutate({ groupId: gid, typeId: typeid, type: type }, {
      onSuccess: () => toast.success("Removed from group"),
      onError: () => toast.error("Failed to remove from group")
    });
  }

  const tagChange = (tag) => {
    setDialog(false)
    setTag(tag)
  }

  const userColumns = useMemo(() => [
    {
      header: "Username",
      accessorKey: "Email",
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => removeEntity(id, row.original._id, "user")}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [id, removeEntity]);

  const serverColumns = useMemo(() => [
    {
      header: "Tag",
      accessorKey: "Tag",
      cell: ({ row }) => {
        const s = allServers?.find(sn => sn._id === row.original._id);
        return s ? s.Tag : "??";
      }
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => removeEntity(id, row.original._id, "server")}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [id, allServers, removeEntity]);

  const deviceColumns = useMemo(() => [
    {
      header: "Tag",
      accessorKey: "Tag", // Note: The original generic implementation had an empty TODO here, assuming Tag exists on device usage in group context?
      // The API likely returns just IDs or partial objects. Assuming Tag is available or we need to fetch. 
      // Original code: `Tag: (_) => { // TODO ... }`
      // Let's assume the backend returns basic info or we display ID if Tag missing.
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => removeEntity(id, row.original._id, "device")}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [id, removeEntity]);


  if (!group) {
    return (
      <div className="ab group-wrapper">			</div>
    )
  }

  return (
    <div className="ab group-wrapper space-y-4">
      <EditDialog
        key={tag + (addForm?.id || "")}
        open={dialog}
        onOpenChange={setDialog}
        initialData={addForm}
        fields={{
          idtype: "hidden",
          type: "hidden",
          id: { label: tag + " ID" }
        }}
        title={tag}
        readOnly={false}
        onSubmit={async (values) => {
          setAddForm(values);
          await addToGroup(values);
        }}
      />

      <Tabs defaultValue="user" className="w-full" onValueChange={(v) => tagChange(v)}>
        <TabsList
          className="border border-[#1a1f2d] cursor-pointer rounded"
        >
          <TabsTrigger className="data-[state=active]:text-[#3168f3]" value="user">Users</TabsTrigger>
          <TabsTrigger className="data-[state=active]:text-[#3168f3]" value="server">Servers</TabsTrigger>
          <TabsTrigger className="data-[state=active]:text-[#3168f3]" value="device">Devices</TabsTrigger>
        </TabsList>
        <TabsContent value="user" className="space-y-4">
          <div className="flex justify-end">
            <Button onClick={() => { setAddForm({ id: "", type: "user", idtype: "" }); setDialog(true); }} className="gap-2">
              <Plus className="h-4 w-4" />
              Add User
            </Button>
          </div>
          <CustomTable data={users || []} columns={userColumns} />
        </TabsContent>
        <TabsContent className="w-full space-y-4" value="server">
          <div className="flex justify-end">
            <Button onClick={() => { setAddForm({ id: "", type: "server", idtype: "" }); setDialog(true); }} className="gap-2">
              <Plus className="h-4 w-4" />
              Add Server
            </Button>
          </div>
          <CustomTable data={servers || []} columns={serverColumns} />
        </TabsContent>
        <TabsContent value="device" className="space-y-4">
          <div className="flex justify-end">
            <Button onClick={() => { setAddForm({ id: "", type: "device", idtype: "" }); setDialog(true); }} className="gap-2">
              <Plus className="h-4 w-4" />
              Add Device
            </Button>
          </div>
          <CustomTable data={devices || []} columns={deviceColumns} />
        </TabsContent>
      </Tabs>

    </div >
  )
}
