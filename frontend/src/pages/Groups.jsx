import React, { useState } from "react";
import dayjs from "dayjs";
import GenericTable from "./GenericTable";
import NewObjectEditorDialog from "./NewObjectEditorDialog";
import { useNavigate } from "react-router-dom";
import { useGroups, useCreateGroup, useUpdateGroup, useDeleteGroup } from "../hooks/useGroups";
import { toast } from "sonner";

const Groups = () => {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(50);
  const { data: groups } = useGroups(offset, limit);
  const createGroupMutation = useCreateGroup();
  const updateGroupMutation = useUpdateGroup();
  const deleteGroupMutation = useDeleteGroup();

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [group, setGroup] = useState(undefined)
  const navigate = useNavigate()

  const saveGroup = async () => {
    try {
      if (group.ID !== undefined) {
        await updateGroupMutation.mutateAsync(group);
        toast.success("Group updated");
      } else {
        await createGroupMutation.mutateAsync(group);
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

  let table = {
    data: groups || [],
    rowClick: (obj) => {
      console.log("row click!")
      console.dir(obj)
    },
    columns: {
      Tag: (obj) => {
        navigate("/groups/" + obj._id)
      },
      _id: true,
      CreatedAt: true,
    },
    columnFormat: {
      CreatedAt: (obj) => {
        return dayjs(obj.CreatedAt).format("HH:mm:ss DD-MM-YYYY")
      }
    },
    Btn: {
      Edit: (obj) => {
        setGroup(obj)
        setEditModalOpen(true)
      },
      Delete: (obj) => {
        deleteGroup(obj._id)
      },
      New: () => {
        console.log("new!")
        newGroup()
      },
    },
    columnClass: {},
    headers: ["Tag", "ID", "CreatedAt"],
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
    more: () => { }, // Pagination logic if needed
  }

  return (
    <div className="groups-page">
      <GenericTable table={table} />

      <NewObjectEditorDialog
        open={editModalOpen}
        onOpenChange={setEditModalOpen}
        object={group}
        title="Group"
        description=""
        readOnly={false}
        saveButton={async () => {
          let ok = await saveGroup()
          if (ok === true) {
            setEditModalOpen(false)
          }
        }}
        onArrayChange={(key, value, index) => {
          setGroup(prev => {
            const newArr = [...prev[key]];
            newArr[index] = value;
            return { ...prev, [key]: newArr };
          });
        }}
        onChange={(key, value, type) => {
          setGroup(prev => ({ ...prev, [key]: value }));
        }}
      />
    </div >
  )

};

export default Groups;
