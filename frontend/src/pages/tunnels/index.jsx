import { useState, useEffect, useMemo } from "react";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { DropdownMenuItem, DropdownMenu, DropdownMenuTrigger, DropdownMenuContent } from "@/components/ui/dropdown-menu";
import { CircleArrowRight, LayoutGrid, List, Plus, MoreHorizontal, Pencil, Trash2, Network, Server, Shield, Copy, Edit } from "lucide-react";
import { useTunnels, useCreateTunnel, useUpdateTunnel, useDeleteTunnel, useDisconnectTunnel } from "@/hooks/useTunnels";
import { connectTunnel, disconnectTunnel, getActiveTunnels } from "@/api/tunnels";
import { getServers } from "@/api/servers";
import { useAtomValue, useSetAtom } from "jotai";
import { userAtom } from "@/stores/userStore";
import { controlServerAtom } from "@/stores/configStore";
import { toast } from "sonner";
import { loadingAtom, toggleLoadingAtom } from "@/stores/uiStore";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useServers } from "@/hooks/useServers";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger, DialogClose, DialogDescription } from "@/components/ui/dialog";
import { Card, CardContent, CardTitle, CardAction, CardHeader, CardFooter } from "@/components/ui/card";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from "@/components/ui/popover";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { ChevronsUpDown } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GetEncType } from "@/lib/helpers";
import { Field, FieldContent, FieldLabel } from "@/components/ui/field";
import { Fragment } from "react";
import { Unplug } from "lucide-react";




function ConnectToServerDialog({ open, onOpenChange, tunnel }) {
  const user = useAtomValue(userAtom);
  const { data, isLoading, isFetched } = useServers(user.ControlServer);
  const queryClient = useQueryClient();
  const [loading, setLoading] = useState(false);
  const [server, setServer] = useState({
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
      queryClient.invalidateQueries({ queryKey: ["tunnles", "activeTunnels"] })
    } catch (e) {
      // Error handled by api client or here
    } finally {
      setLoading(false);
    }
  };
  console.log(tunnel); console.log('servers', data);
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
            <Button onClick={() => connectMutation.mutate(server.id)}> <CircleArrowRight /> Connect</Button>
          </DialogClose>

        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
function TunnelCard({ tunnel, serverId, connectionDialogOpener, onEdit, onDelete, isConnected, onDisconnect }) {
  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    toast.success("Copied to clipboard");
  };
  console.log(isConnected)
  return (
    <Card className="border-[#1a1f2d] text-white hover:border-[#2a3142] transition-colors">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-lg font-bold flex items-center gap-2">
          <Network className="w-5 h-5 text-blue-400" />
          {tunnel.Tag}
        </CardTitle>
        <Badge
          variant="outline"
          className="bg-primary hover:bg-white hover:text-black text-white"
        >
          {GetEncType(tunnel.EncryptionType)}
        </Badge>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 py-4">
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Server className="w-4 h-4" />
            <span>Server ID: {serverId || "(Tunnel not connected)"}</span>
          </div>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Shield className="w-4 h-4" />
            <span>Interface: {tunnel.IFName}</span>
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between bg-[#151a25] p-2 rounded text-xs font-mono">
              <span className="text-gray-300">IPv4: {tunnel.IPv4Address}</span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => copyToClipboard(tunnel.IPv4Address)}
              >
                <Copy className="w-3 h-3" />
              </Button>
            </div>
            <div className="flex items-center justify-between bg-[#151a25] p-2 rounded text-xs font-mono">
              <span className="text-gray-300">IPv6: {tunnel.IPv6Address}</span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => copyToClipboard(tunnel.IPv6Address)}
              >
                <Copy className="w-3 h-3" />
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
      <CardFooter className="flex justify-between">
        <Button className={isConnected ? "text-destructive" : "text-green-400"} variant="outline" onClick={isConnected ? onDisconnect : connectionDialogOpener}>
          {isConnected ? "Disconnect" : "Connect"}
        </Button>
        <div className="flex gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onEdit(tunnel)}
            className="h-8 w-8 text-gray-400 hover:text-white"
          >
            <Edit className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onDelete(tunnel)}
            className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
};


export default function TunnelsPage() {
  const tunnels = useTunnels();
  console.dir(tunnels.data);
  const [connectionDialogOpen, setConnectionDialogOpen] = useState(false);
  const createTunnelMutation = useCreateTunnel();
  const updateTunnelMutation = useUpdateTunnel();
  const deleteTunnelMutation = useDeleteTunnel();
  const dcTunnel = useDisconnectTunnel();

  const [tunnel, setTunnel] = useState(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [tunTag, setTunTag] = useState("")
  const [viewMode, setViewMode] = useState("grid"); // 'grid' | 'table'

  // Helper for encryption type display
  const GetEncType = (int) => {
    switch (String(int)) {
      case "0": return "None";
      case "1": return "AES128";
      case "2": return "AES256";
      case "3": return "CHACHA20";
      default: return "unknown";
    }
  };


  const activeTunnels = useQuery({
    queryKey: ["activeTunnels"],
    queryFn: getActiveTunnels
  });

  const newServer = () => {
    createTunnelMutation.mutate();
  };

  const columns = useMemo(() => [
    {
      header: "Tag",
      accessorKey: "Tag",
    },
    {
      header: "IPv4",
      accessorKey: "IPv4Address",
    },
    {
      header: "IPv6",
      accessorKey: "IPv6Address",
    },
    {
      header: "Interface",
      accessorKey: "IFName",
    },
    {
      header: "Status",
      accessorKey: "status",
      cell: ({ row }) => {
        const corr = activeTunnels.data.find(at => at.CR.Tag === row.original.Tag);
        return (
          corr ? <Badge className="bg-green-400 text-white">Connected</Badge> : <Badge variant="destructive">Disconnected</Badge>
        )
      }
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const corr = activeTunnels.data.find(at => at.CR.Tag === row.original.Tag);

        return (
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger >
              <Button variant="ghost" className="h-8 w-8 p-0">
                <span className="sr-only">Open menu</span>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {!corr ?
                <DropdownMenuItem
                  onSelect={() => {
                    setTunnel(row.original);
                    setTunTag(row.original.Tag);
                    setConnectionDialogOpen(true)
                  }}
                >
                  <CircleArrowRight className="mr-2 h-4 w-4" />
                  <span>Connect</span>
                </DropdownMenuItem>
                : <DropdownMenuItem onSelect={() => {
                  dcTunnel.mutate(corr.ID)
                }}>
                  <Unplug className="mr-2 h-4 w-4" />
                  <span>Disconnect</span>
                </DropdownMenuItem>
              }
              <DropdownMenuItem
                onSelect={() => {
                  setTunnel(row.original);
                  setTunTag(row.original.Tag)
                  setModalOpen(true);
                }}
                className="cursor-pointer"
              >
                <Pencil className="mr-2 h-4 w-4" />
                <span>Edit</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => deleteTunnelMutation.mutate(row.original)}
                className="cursor-pointer text-red-600 focus:text-red-500"
              >
                <Trash2 className="mr-2 h-4 w-4" />
                <span>Delete</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      }
    }
  ], [activeTunnels.data]);

  if (tunnels.isLoading || activeTunnels.isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <div className="w-full">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 p-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Tunnels</h1>
          <p className="text-muted-foreground">Manage your secure tunnel connections.</p>
        </div>
        <div className="flex items-center gap-2">
          <ToggleGroup type="single" value={viewMode} onValueChange={(value) => value && setViewMode(value)}>
            <ToggleGroupItem value="grid" aria-label="Grid view">
              <LayoutGrid className="h-4 w-4" />
            </ToggleGroupItem>
            <ToggleGroupItem value="table" aria-label="Table view">
              <List className="h-4 w-4" />
            </ToggleGroupItem>
          </ToggleGroup>
          <Button onClick={newServer}>
            <Plus className="w-4 h-4 mr-2" /> Create Tunnel
          </Button>
        </div>
      </div>

      <ConnectToServerDialog open={connectionDialogOpen} onOpenChange={setConnectionDialogOpen} tunnel={tunnel} />
      {viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {tunnels.data?.map((t) => {
            const corr = activeTunnels.data.find(at => at.CR.Tag === t.Tag);
            return (
              <TunnelCard
                key={t.Tag}
                tunnel={t}
                serverId={corr ? corr.CR.ServerID : ""}
                isConnected={!!corr}
                connectionDialogOpener={() => {
                  setTunnel(t);
                  setConnectionDialogOpen(true);
                }}
                onEdit={(obj) => {
                  setTunnel(obj);
                  setModalOpen(true);
                  setTunTag(obj.Tag)
                }}
                onDelete={(obj) => {
                  deleteTunnelMutation.mutate(obj);
                  setTunnel();
                }}
                onDisconnect={() => {
                  if (corr) dcTunnel.mutate(corr.ID, {
                    onSuccess: () => toast.success(`Disconnected tunnel ${t.Tag}`),
                    onError() { toast.error("Error disconnecting") }
                  });
                }}
              />
            )
          })}
          {(!tunnels || tunnels.length === 0) && (
            <div className="col-span-full text-center py-10 text-muted-foreground">
              No tunnels found. Create one to get started.
            </div>
          )}
        </div>
      ) : (
        <CustomTable data={tunnels.data || []} columns={columns} />
      )}

      <EditDialog
        key={tunnel?._id || 'new'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        initialData={tunnel}
        title="Tunnel"
        fields={{
          WindowsGUID: "readonly",
          DHCPToken: "readonly",
          DNSRecords: "hidden",
          Networks: "hidden",
          Routes: "hidden",
          CurveType: "hidden",
          EncryptionType: { label: tunnel ? "Encryption [ " + GetEncType(tunnel.EncryptionType) + " ]" : "Encryption" }
        }}
        description={tunnel?._id ? "Edit tunnel configuration" : "Create new tunnel"}
        readOnly={false}
        onSubmit={async (values) => {
          updateTunnelMutation.mutate({ tunnel: values, oldTag: tunTag }, {
            onSuccess: () => {
              setModalOpen(false)
            }
          });
        }}
      />
    </div>
  );
};

