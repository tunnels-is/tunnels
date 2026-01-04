import React, { useState, useMemo } from "react";
import dayjs from "dayjs";
import EditDialog from "@/components/edit-dialog";
import CustomTable from "@/components/custom-table";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Edit, Trash2, Plus } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getDevices, updateDevice, createDevice, deleteDevice } from "@/api/devices";



export default function DevicesPage() {
  const devices = useQuery({
    queryKey: ["devices"],
    queryFn: () => getDevices({ offset: 0, limit: 50 })
  });
  const queryClient = useQueryClient();
  const [device, setDevice] = useState(undefined)
  const [editModalOpen, setEditModalOpen] = useState(false)

  const saveDevice = async (values) => {
    const dataToSave = values || device;
    try {
      if (dataToSave._id !== undefined) {
        await updateDevice(dataToSave);
        toast.success("Device updated");
      } else {
        await createDevice(dataToSave);
        toast.success("Device created");
      }
      setEditModalOpen(false);
      queryClient.invalidateQueries({ queryKey: ["devices"] });
    } catch (e) {
      toast.error("Failed to save device");
    }
  }

  const newDevice = () => {
    setDevice({ Tag: "", Groups: [] })
    setEditModalOpen(true)
  }

  const removeDevice = async (id) => {
    try {
      await deleteDevice(id);
      queryClient.invalidateQueries({ queryKey: ["devices"] });
      toast.success("Deleted device")
    }
    catch (e) {
      toast.error("Failed to delete device");
    }
  }
  const columns = useMemo(() => [
    {
      header: "Tag",
      accessorKey: "Tag",
    },
    {
      header: "ID",
      accessorKey: "_id",
      cell: ({ row }) => <span className="font-mono text-xs">{row.original._id}</span>
    },
    {
      header: "Created At",
      accessorKey: "CreatedAt",
      cell: ({ row }) => dayjs(row.original.CreatedAt).format("HH:mm:ss DD-MM-YYYY"),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setDevice(row.original);
              setEditModalOpen(true);
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-red-500 hover:text-red-700 hover:bg-red-950/20"
            onClick={async () => {
              await removeDevice(row.original._id)
            }}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )
    }
  ], [devices.data]);

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button onClick={newDevice} className="gap-2">
          <Plus className="h-4 w-4" />
          New Device
        </Button>
      </div>
      <CustomTable data={devices.data || []} columns={columns} />

      <EditDialog
        key={device?._id || 'new'}
        open={editModalOpen}
        onOpenChange={setEditModalOpen}
        initialData={device}
        title="Device"
        description={device?._id ? "Edit device details" : "Create new device"}
        readOnly={false}
        onSubmit={async (values) => {
          await saveDevice(values);
        }}
      />

    </div >
  )
}
