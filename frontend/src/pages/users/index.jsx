import React, { useState, useMemo } from "react";
import dayjs from "dayjs";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { useUsers, useUpdateUser } from "@/hooks/useUsers";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Edit } from "lucide-react";

export default function UsersPage() {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(50);
  const { data: users, refetch } = useUsers();
  const updateUserMutation = useUpdateUser();

  const [selectedUser, setSelectedUser] = useState(undefined)
  const [modalOpen, setModalOpen] = useState(false)

  const saveUser = async (user) => {
    try {
      await updateUserMutation.mutateAsync(user);
      toast.success("User updated successfully");
      setModalOpen(false);
    } catch (e) {
      toast.error("Failed to update user");
    }
  }

  const columns = useMemo(() => [
    {
      header: "User",
      accessorKey: "Email",
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      header: "Trial",
      accessorKey: "Trial",
      cell: ({ row }) => (row.original.Trial === true ? "Yes" : "No"),
    },
    {
      header: "Subscription Expiration",
      accessorKey: "SubExpiration",
      cell: ({ row }) => dayjs(row.original.SubExpiration).format("HH:mm:ss DD-MM-YYYY"),
    },
    {
      header: "Updated",
      accessorKey: "Updated",
      cell: ({ row }) => dayjs(row.original.Updated).format("HH:mm:ss DD-MM-YYYY"),
    },
    {
      id: "actions",
      cell: ({ row }) => {
        return (
          <div className="flex justify-end">
            <Button
              variant="ghost"
              size="icon"
              onClick={(e) => {
                e.stopPropagation();
                setSelectedUser(row.original);
                setModalOpen(true);
              }}
            >
              <Edit className="h-4 w-4" />
            </Button>
          </div>
        );
      },
    },
  ], []);

  return (
    <div>
      <p className="text-3xl font-bold">Manage Users</p>
      <CustomTable
        data={users || []}
        columns={columns}
      />
      <EditDialog
        key={selectedUser?._id || 'new'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        initialData={selectedUser}
        readOnly={false}
        onSubmit={async (values) => {
          await saveUser(values);
        }}
        fields={{
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
          DeviceToken: "hidden",
          IsManager: { label: "Manager" },
          SubExpiration: { label: "Subscription Expiration" }
        }}
      />
    </div >
  )
}

