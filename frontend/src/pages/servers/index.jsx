import React, { useState } from "react";
import CustomTable from "@/components/custom-table";
import { TableCell } from "@/components/ui/table";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import EditDialog from "@/components/edit-dialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { useNavigate } from "react-router-dom";
import { CircleArrowRight, LogOut, Server, MoreHorizontal, Pencil, Trash } from "lucide-react";
import { useServers, useCreateServer, useUpdateServer } from "@/hooks/useServers";
import { useTunnels, useConnectTunnel, useDisconnectTunnel, useUpdateTunnel } from "@/hooks/useTunnels";
import { useAtomValue } from "jotai";
import { userAtom } from "@/stores/userStore";
import { activeTunnelsAtom } from "@/stores/tunnelStore";
import { getCountryName } from "@/lib/constants";
import { toast } from "sonner";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import ServerDevices from "./server-devices";


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
  const user = useAtomValue(userAtom);
  const activeTunnels = useAtomValue(activeTunnelsAtom);
  const { data: servers, isLoading: serversLoading } = useServers(user?.ControlServer);
  const { data: tunnels, isLoading: tunnelsLoading } = useTunnels();

  const createServerMutation = useCreateServer();
  const updateServerMutation = useUpdateServer();
  const connectTunnelMutation = useConnectTunnel();
  const disconnectTunnelMutation = useDisconnectTunnel();
  const updateTunnelMutation = useUpdateTunnel();

  const [server, setServer] = useState(undefined);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const navigate = useNavigate();



  const ConnectColumn = (server) => {
    let servertun = undefined;
    let assignedTunnels = 0;
    tunnels?.forEach(c => {
      if (c.ServerID === server._id) {
        servertun = c;
        assignedTunnels++;
      }
    });

    const handleConnect = () => {
      let tunnelToConnect = undefined;
      if (assignedTunnels < 1) {
        let defaultTunnel = tunnels?.find(t => t.Tag === "tunnels");
        if (defaultTunnel) {
          tunnelToConnect = defaultTunnel;
        }
      } else {
        tunnelToConnect = servertun;
      }

      if (!tunnelToConnect) {
        toast.error("No suitable tunnel found to connect");
        return;
      }

      if (!user?.DeviceToken) {
        toast.error("You are not logged in");
        return;
      }

      const connectionRequest = {
        UserID: user._id,
        DeviceToken: user.DeviceToken.DT,
        Tag: tunnelToConnect.Tag,
        EncType: tunnelToConnect.EncryptionType,
        ServerID: server._id,
        Server: user.ControlServer
      };

      connectTunnelMutation.mutate(connectionRequest);
    };

    let con = activeTunnels?.find(x => x.CR?.ServerID === server._id);

    return (
      <DropdownMenu>
        <DropdownMenuTrigger className="w-full">
          <Button variant="outline" className="w-full">
            <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem
            key="connect"
            onClick={() => {
              if (assignedTunnels > 1) {
                toast.error("Too many tunnels assigned to server");
                return;
              }
              handleConnect();
            }}
            className="cursor-pointer text-[#3a994c]"
          >
            <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
          </DropdownMenuItem>
          {con && (
            <DropdownMenuItem
              key="disconnect"
              onClick={() => disconnectTunnelMutation.mutate(con.ID)}
              className="cursor-pointer text-[#ef4444]"
            >
              <LogOut className="w-4 h-4 mr-2" /> Disconnect
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  const TunnelsColumn = (obj) => {
    let servertun = undefined;
    let assignedTunnels = 0;
    let opts = [];

    tunnels?.forEach(c => {
      if (c.ServerID === obj._id) {
        servertun = c;
        opts.push({ value: c.Tag, key: c.Tag, selected: true });
        assignedTunnels++;
      } else {
        opts.push({ value: c.Tag, key: c.Tag, selected: false });
      }
    });

    let value = undefined;
    let assigned = "Assign to tunnel";
    if (assignedTunnels > 1) {
      assigned = String(assignedTunnels) + " tunnels assigned";
    } else {
      value = servertun?.Tag;
    }

    return (
      <TableCell className="w-full text-white">
        <Select
          value={value}
          onValueChange={(tag) => {
            const tunnel = tunnels?.find(t => t.Tag === tag);
            if (tunnel) {
              const updatedTunnel = { ...tunnel, ServerID: obj._id };
              updateTunnelMutation.mutate({ tunnel: updatedTunnel, oldTag: tunnel.Tag });
            }
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder={assigned} />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {opts?.map(t => (
                <SelectItem
                  key={t.value}
                  value={t.value}
                >
                  {t.key}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </TableCell>
    );
  };

  console.log("servers: ", servers);

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
      id: "select",
      header: "",
      cell: props => <TunnelsColumn obj={props.row.original} />
    },
    {
      id: "connect",
      header: "",
      cell: props => <ConnectColumn obj={props.row.original} />
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
  if (serversLoading || tunnelsLoading) {
    return <div>Loading...</div>;
  }

  return (
    <div className="w-full mt-16 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Private Servers</h1>
          <p className="text-muted-foreground">Manage your private VPN servers and tunnel assignments.</p>
        </div>
      </div>

      <CustomTable data={servers || []} columns={dataCols} />

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
            updateServerMutation.mutate({ controlServer: user?.ControlServer, serverData: values }, {
              onSuccess: () => setEditModalOpen(false)
            });
          } else {
            // create
            createServerMutation.mutate({ controlServer: user?.ControlServer, serverData: values }, {
              onSuccess: () => setEditModalOpen(false)
            });
          }
        }}
      />
    </div>
  );
};

