import { useState, useEffect, useMemo } from "react";
import CustomTable from "@/components/custom-table";
import EditDialog from "@/components/edit-dialog";
import { DropdownMenuItem, DropdownMenu, DropdownMenuTrigger, DropdownMenuContent } from "@/components/ui/dropdown-menu";
import { CircleArrowRight, LayoutGrid, List, Plus, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useTunnels, useCreateTunnel, useUpdateTunnel, useDeleteTunnel } from "@/hooks/useTunnels";
import { connectTunnel, disconnectTunnel } from "@/api/tunnels";
import { getServers } from "@/api/servers";
import { useAtomValue, useSetAtom } from "jotai";
import { userAtom } from "@/stores/userStore";
import { controlServerAtom } from "@/stores/configStore";
import { toast } from "sonner";
import { loadingAtom, toggleLoadingAtom } from "@/stores/uiStore";
import TunnelCard from "./tunnel-card";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export default function TunnelsPage() {
  const { data: tunnels, isLoading } = useTunnels();
  const createTunnelMutation = useCreateTunnel();
  const updateTunnelMutation = useUpdateTunnel();
  const deleteTunnelMutation = useDeleteTunnel();
  const user = useAtomValue(userAtom);
  const controlServer = useAtomValue(controlServerAtom);
  const setLoading = useSetAtom(toggleLoadingAtom);

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

  const handleConnectTunnel = async (obj) => {
    if (!user || !user.DeviceToken) {
      toast.error("You are not logged in");
      return;
    }

    // Simplified connection request construction
    const connectionRequest = {
      UserID: user._id,
      DeviceToken: user.DeviceToken.DT,
      Tag: obj.Tag,
      EncType: obj.EncryptionType,
      ServerID: obj.ServerID, // Needs proper server resolution if ID is index
      Server: user.ControlServer // Or specific server
    };

    setLoading({ show: true, msg: "Connecting..." });
    try {
      await connectTunnel(connectionRequest);
      toast.success("Connection ready");
    } catch (e) {
      // Error handled by api client or here
    } finally {
      setLoading(undefined);
    }
  };

  const newServer = async () => {
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
      header: "ServerID",
      accessorKey: "ServerID",
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="h-8 w-8 p-0">
              <span className="sr-only">Open menu</span>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              onClick={() => handleConnectTunnel(row.original)}
              className="cursor-pointer text-[#3a994c]"
            >
              <CircleArrowRight className="mr-2 h-4 w-4" />
              <span>Connect</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
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
              onClick={() => deleteTunnelMutation.mutate(row.original)}
              className="cursor-pointer text-red-600 focus:text-red-500"
            >
              <Trash2 className="mr-2 h-4 w-4" />
              <span>Delete</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )
    }
  ], [user]);

  if (isLoading) {
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
          <Button onClick={newServer} className="bg-blue-600 hover:bg-blue-700 text-white">
            <Plus className="w-4 h-4 mr-2" /> Create Tunnel
          </Button>
        </div>
      </div>

      {viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {tunnels?.map((t) => (
            <TunnelCard
              key={t.Tag}
              tunnel={t}
              onConnect={() => (
                <Button
                  variant="outline"
                  size="sm"
                  className="text-green-500 border-green-900 hover:bg-green-950/20"
                  onClick={() => handleConnectTunnel(t)}
                >
                  <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
                </Button>
              )}
              onEdit={(obj) => {
                setTunnel(obj);
                setModalOpen(true);
                setTunTag(obj.Tag)
              }}
              onDelete={(obj) => {
                deleteTunnelMutation.mutate(obj);
                setTunnel(undefined);
              }}
            />
          ))}
          {(!tunnels || tunnels.length === 0) && (
            <div className="col-span-full text-center py-10 text-muted-foreground">
              No tunnels found. Create one to get started.
            </div>
          )}
        </div>
      ) : (
        <CustomTable data={tunnels || []} columns={columns} />
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

