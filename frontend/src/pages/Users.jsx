import React, { useState } from "react";
import dayjs from "dayjs";
import GenericTable from "../components/GenericTable";
import NewObjectEditorDialog from "@/components/NewObjectEditorDialog";
import { useUsers, useAdminUpdateUser } from "../hooks/useUsers";
import { toast } from "sonner";

const Users = () => {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(50);
  const { data: users, refetch } = useUsers();
  const adminUpdateUserMutation = useAdminUpdateUser();

  const [selectedUser, setSelectedUser] = useState(undefined)
  const [modalOpen, setModalOpen] = useState(false)

  const saveUser = async (user) => {
    try {
      await adminUpdateUserMutation.mutateAsync(user);
      toast.success("User updated successfully");
      setModalOpen(false);
    } catch (e) {
      toast.error("Failed to update user");
    }
  }

  let table = {
    data: users || [],
    rowClick: (obj) => {
      console.log("row click!")
      console.dir(obj)
    },
    Btn: {
      Edit: (obj) => {
        setSelectedUser(obj)
        setModalOpen(true)
      },
    },
    columns: {
      Email: true,
      ID: (obj) => {
        // alert(obj._id)
      },
      Trial: true,
      SubExpires: true,
      Updated: true,
    },
    columnFormat: {
      Updated: (obj) => {
        return dayjs(obj.Updated).format("HH:mm:ss DD-MM-YYYY")
      },
      SubExpires: (obj) => {
        return dayjs(obj.SubExpiration).format("HH:mm:ss DD-MM-YYYY")
      },
      Trial: (obj) => {
        return obj.Trial === true ? "Yes" : "no"
      }
    },
    columnClass: {},
    headers: ["User", "ID", "Trial", "SubExpiration", "Updated"],
    headerClass: {
      ID: () => {
        return ""
      }
    },
    opts: {
      RowPerPage: 50,
    },
    more: () => { }, // Pagination logic if needed
  }

  return (
    <div className="ab users-wrapper" >
      <GenericTable
        table={table}
      />
      <NewObjectEditorDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        object={selectedUser}
        readOnly={false}
        saveButton={saveUser}
        onChange={(key, value, type) => {
          setSelectedUser(prev => ({ ...prev, [key]: value }));
        }}
        onArrayChange={(key, value, index) => {
          setSelectedUser(prev => {
            const newArr = [...prev[key]];
            newArr[index] = value;
            return { ...prev, [key]: newArr };
          });
        }}
        opts={{
          fields: {
            ID: "readonly",
            APIKey: "hidden",
            Password: "hidden",
            Password2: "hidden",
            ResetCode: "hidden",
            ConfirmCode: "hidden",
            RecoveryCodes: "hidden",
            TwoFactorCode: "hidden",
            TwoFactorEnabled: "hidden",
            Tokens: "hidden",
            IsAdmin: "hidden",
            Groups: "hidden",
            Key: "hidden",
            Updated: "readonly",
            DeviceToken: "hidden"
          },
          nameFormat: {
            IsManager: () => "Manager",
            SubExpiration: () => "Subscription Expiration"
          }
        }}
      />
    </div >
  )
}

export default Users;
