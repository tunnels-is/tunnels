import React, { useState, useMemo } from "react";
import dayjs from "dayjs";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { useNavigate } from "react-router-dom";
import { useGroups, useCreateGroup, useUpdateGroup, useDeleteGroup } from "@/hooks/useGroups";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Plus, Edit, Trash2 } from "lucide-react";
import InspectGroup from "./inspect-group.jsx";


export default function GroupsPage() {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(50);
  const { data: groups } = useGroups(offset, limit);
  const createGroupMutation = useCreateGroup();
  const updateGroupMutation = useUpdateGroup();
  const deleteGroupMutation = useDeleteGroup();

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [group, setGroup] = useState(undefined)
  const navigate = useNavigate()

  const saveGroup = async (values) => {
    const dataToSave = values || group;
    try {
      if (dataToSave.ID !== undefined) {
        await updateGroupMutation.mutateAsync(dataToSave);
        toast.success("Group updated");
      } else {
        await createGroupMutation.mutateAsync(dataToSave);
        toast.success("Group created");
      }
      return true;
    } catch (e) {
      toast.error("Unable to save group");
      return false;
    }
  }

  const newGroup = () => {
    setGroup({ Tag: "my-new-group", Description: "This is a new group" })
    setEditModalOpen(true)
  }

  const deleteGroup = (id) => {
    deleteGroupMutation.mutate(id, {
      onSuccess: () => toast.success("Group deleted"),
      onError: () => toast.error("Failed to delete group")
    });
  }

  const columns = useMemo(() => [
    {
      header: "Tag",
      accessorKey: "Tag",
      cell: ({ row }) => (
        <span
          className="cursor-pointer text-blue-500 hover:text-blue-400 hover:underline"
          onClick={() => navigate("/groups/" + row.original._id)}
        >
          {row.original.Tag}
        </span>
      )
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      header: "Created",
      accessorKey: "CreatedAt",
      cell: ({ row }) => dayjs(row.original.CreatedAt).format("HH:mm:ss DD-MM-YYYY")
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setGroup(row.original);
              setEditModalOpen(true);
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={() => deleteGroup(row.original._id)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [navigate]);

  return (
    <div className="groups-page space-y-4">
      <div className="flex justify-end">
        <Button onClick={newGroup} className="gap-2">
          <Plus className="h-4 w-4" />
          New Group
        </Button>
      </div>
      <CustomTable data={groups || []} columns={columns} />

      <EditDialog
        key={group?._id || 'new'}
        open={editModalOpen}
        onOpenChange={setEditModalOpen}
        initialData={group}
        title="Group"
        description={group?._id ? "Edit group" : "Create group"}
        readOnly={false}
        onSubmit={async (values) => {
          setGroup(values);
          const ok = await saveGroup(values);
          if (ok === true) {
            setEditModalOpen(false)
          }
        }}
      />
    </div >
  )

};
export { InspectGroup };

