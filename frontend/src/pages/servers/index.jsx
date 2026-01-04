import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useCreateServer, useServers, useUpdateServer } from "@/hooks/useServers";
import { useConnectTunnel, useDisconnectTunnel, useTunnels, useUpdateTunnel } from "@/hooks/useTunnels";
import { MoreHorizontal, Pencil, Plus, Trash } from "lucide-react";
import { useState } from "react";
import ServerDevices from "./server-devices";
import { useQuery } from "@tanstack/react-query";
import { getServers } from "@/api/servers";


function RowActions({ setEditModalOpen, deleteFn }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Button variant="ghost" className="h-8 w-8 p-0 text-white">
          <span className="sr-only">Open menu</span>
          <MoreHorizontal className="w-4 h-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem key="edit"
          onSelect={() => { console.log("edit"); setEditModalOpen() }}>
          <Pencil className="w-4 h-4 mr-2" /> Edit
        </DropdownMenuItem>
        <DropdownMenuItem
          key="delete"
          onSelect={() => {
            deleteFn();
          }}
        >
          <Trash className="w-4 h-4 mr-2" /> Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}


export { ServerDevices };
export default function ServersPage() {
  const privateServers = useQuery({
    queryKey: ["servers"],
    queryFn: getServers
  });
  const tunnels = useTunnels();

  const createServerMutation = useCreateServer();
  const updateServerMutation = useUpdateServer();

  const [server, setServer] = useState(undefined);
  const [editModalOpen, setEditModalOpen] = useState(false);

  console.log("servers: ", privateServers.data);

  const dataCols = [
    {
      id: "Tag",
      header: "Name",
      accessorKey: "Tag",
      cell: (info) => info.getValue()
    },
    {
      id: "Country",
      header: "Country",
      accessorKey: "Country",
      cell: (info) => info.getValue()
    },
    {
      id: "IP",
      header: "IP",
      accessorKey: "IP",
      cell: (info) => info.getValue()
    },
    {
      id: "Port",
      header: "Port",
      accessorKey: "Port",
      cell: (info) => info.getValue()
    },
    {
      id: "_id",
      header: "ID",
      accessorKey: "_id",
      cell: (info) => info.getValue()
    },
    {
      id: "actions",
      header: "",
      cell: props =>
        <RowActions
          deleteFn={() => deleteServer(props.row.original)}
          setEditModalOpen={() => {
            setServer(props.row.original);
            setEditModalOpen(true);
          }}
        />
    },
  ]
  if (privateServers.isLoading || tunnels.isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <div className="w-full mt-8 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Private Servers</h1>
          <p className="text-muted-foreground">Manage your private VPN servers and tunnel assignments.</p>
        </div>
        <Button onClick={() => {
          setServer({
            Tag: "",
            Country: "",
            IP: "",
            Port: "",
            DataPort: "",
            PubKey: "",
            Groups: []
          });
          setEditModalOpen(true);

        }}>
          <Plus /> Create Server
        </Button>
      </div>

      <CustomTable data={privateServers.data || []} columns={dataCols} />

      <EditDialog
        key={server?._id || 'new'}
        open={editModalOpen}
        onOpenChange={setEditModalOpen}
        initialData={server}
        title={server?._id ? "Edit Server" : "Create Server"}
        description="Configure private server settings."
        readOnly={false}
        onSubmit={async (values) => {
          if (values._id) {
            // update
            updateServerMutation.mutate({ serverData: values }, {
              onSuccess: () => setEditModalOpen(false)
            });
          } else {
            // create
            createServerMutation.mutate({ serverData: values }, {
              onSuccess: () => setEditModalOpen(false)
            });
          }
        }}
      />
    </div>
  );
};

