import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { TableCell } from "@/components/ui/table";
import { useCreateServer, useServers, useUpdateServer } from "@/hooks/useServers";
import { useConnectTunnel, useDisconnectTunnel, useTunnels, useUpdateTunnel } from "@/hooks/useTunnels";
import { getCountryName } from "@/lib/constants";
import { activeTunnelsAtom } from "@/stores/tunnelStore";
import { userAtom } from "@/stores/userStore";
import { useAtomValue } from "jotai";
import { CircleArrowRight, LogOut, MoreHorizontal, Pencil, Plus, Server, Trash } from "lucide-react";
import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
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

function ConnectToTunnelDialog({ open, onOpenChange, server }) {
  const user = useAtomValue(userAtom);
  const { data, isLoading, isFetched } = useTunnels();

  const [loading, setLoading] = useState(false);
  const [tunnel, setServer] = useState({
    id: "", tag: "", ip: ""
  });
  const [popoverOpen, setPopoverOpen] = useState(false);

  const handleConnectTunnel = async (serverId) => {
    if (!user || !user.DeviceToken) {
      toast.error("You are not logged in");
      return;
    }

    // Simplified connection request construction
    const connectionRequest = {
      UserID: user.ID,
      DeviceToken: user.DeviceToken.DT,
      Tag: tunnel.Tag,
      EncType: tunnel.EncryptionType,
      ServerID: serverId, // Needs proper server resolution if ID is index
      Server: user.ControlServer // Or specific server
    };

    setLoading(true);
    try {
      console.log(connectionRequest);
      await connectTunnel(connectionRequest);
      toast.success("Connection ready");
    } catch (e) {
      // Error handled by api client or here
    } finally {
      setLoading(false);
    }
  };
  console.log(tunnel); console.log('servers', data);
  const queryClient = useQueryClient();
  const connectMutation = useMutation({
    mutationFn: handleConnectTunnel,
    async onSuccess() { await queryClient.invalidateQueries({ queryKey: ["tunnels"] }) }
  });
  return tunnel && (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            Connect Tunnel {tunnel.Tag} to:
          </DialogTitle>
        </DialogHeader>
        <DialogDescription>
          {
            isLoading && <span> <Spinner /> Loading Servers... </span>
          }
          {
            isFetched && (
              <div className="my-2">
                <Button onClick={() => setPopoverOpen(p => !p)} className="w-full" variant="outline">
                  {server.tag && <Badge>{server.tag}</Badge>}
                  {server.ip ? server.ip : "No server selected..."}</Button>
                {popoverOpen &&
                  <Command className="mt-2">
                    <CommandInput placeholder="Search for a server..." />
                    <CommandList>
                      <CommandEmpty>No such server found....</CommandEmpty>
                      <CommandGroup>
                        {data.map(s => (
                          <CommandItem key={s._id} value={s._id} onSelect={cv => {
                            setServer(p => cv !== server.id ? ({ id: cv, tag: s.Tag, ip: s.IP }) : p);
                            setPopoverOpen(false);
                          }}>
                            <Badge>{s.Tag}</Badge>

                            <span>
                              {s.IP}
                              <span className="text-muted">{s._id}</span>
                            </span>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                }</div>
            )}
        </DialogDescription>
        <DialogFooter>
          <DialogClose asChild>
            <Button onClick={() => connectMutation.mutate({ serverId: server.id })}> <CircleArrowRight /> Connect</Button>
          </DialogClose>

        </DialogFooter>
      </DialogContent>
    </Dialog>
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
        UserID: user.ID,
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
    const connectedTunnel = tunnels.find(t => t.ServerID === obj._id);
    let assigned = "NO tunnel assigned";
    if (connectedTunnel) {
      assigned = connectedTunnel.Tag;
    }

    return (
      <TableCell className="w-full text-white">
        <Select
          value={assigned}
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
              {tunnels.map(t => (
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
    <div className="w-full mt-8 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Private Servers</h1>
          <p className="text-muted-foreground">Manage your private VPN servers and tunnel assignments.</p>
        </div>
        <Button onClick={e => {
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

