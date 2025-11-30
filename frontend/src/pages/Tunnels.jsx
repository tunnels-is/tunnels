import { useState, useEffect } from "react";
import GenericTable from "./GenericTable";
import NewObjectEditorDialog from "./NewObjectEditorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { CircleArrowRight, LayoutGrid, List, Plus } from "lucide-react";
import { LogOut } from "lucide-react";
import { useTunnels, useCreateTunnel, useUpdateTunnel, useDeleteTunnel } from "../hooks/useTunnels";
import { connectTunnel, disconnectTunnel } from "../api/tunnels";
import { getServers } from "../api/servers";
import { useAtomValue, useSetAtom } from "jotai";
import { userAtom } from "../stores/userStore";
import { controlServerAtom } from "../stores/configStore";
import { toast } from "sonner";
import { loadingAtom, toggleLoadingAtom } from "../stores/uiStore";
import TunnelCard from "../components/TunnelCard";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export default function Tunnels() {
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

  const ConnectButton = (obj) => {
    // Logic to check active status would need activeTunnels state, 
    // assuming we might fetch that or it's part of the tunnel object if updated.
    // For now, implementing the action handlers.

    // TODO: Fetch active tunnels to determine state
    let active = undefined;

    const handleConnect = async () => {
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

    const handleDisconnect = async () => {
      setLoading({ show: true, msg: "Disconnecting..." });
      try {
        // active.ID needed here
        // await disconnectTunnel(active.ID);
        toast.success("Disconnected");
      } catch (e) {
        // Error handled
      } finally {
        setLoading(undefined);
      }
    };

    // For Card View, return a Button
    if (viewMode === 'grid') {
      return (
        <Button
          variant="outline"
          size="sm"
          className="text-green-500 border-green-900 hover:bg-green-950/20"
          onClick={handleConnect}
        >
          <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
        </Button>
      )
    }

    // For Table View, return DropdownMenuItem
    return <div>
      <DropdownMenuItem
        key="connect"
        onClick={handleConnect}
        className="cursor-pointer text-[#3a994c] "
      >
        <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
      </DropdownMenuItem >
      {/* Disconnect button conditionally rendered if active */}
    </div>

  };

  const newServer = async () => {
    createTunnelMutation.mutate();
  };

  let table = {
    data: tunnels || [],
    rowClick: (obj) => {
      console.log("row click!");
      console.dir(obj);
    },
    columns: {
      Tag: true,
      IPv4Address: true,
      IPv6Address: true,
      IFName: true,
      ServerID: true,
    },
    customBtn: {
      Connect: ConnectButton,
    },
    Btn: {
      Edit: (obj) => {
        setTunnel(obj);
        setModalOpen(true);
        setTunTag(obj.Tag)
      },
      Delete: (obj) => {
        deleteTunnelMutation.mutate(obj);
        setTunnel(undefined);
      },
      New: () => {
        newServer();
      },
    },
    headers: ["Tag", "IPv4", "IPv6", "IFName", "ServerID"],
    headerFormat: {
      IFName: () => {
        return "Interface";
      },
    },
    opts: {
      RowPerPage: 50,
    },
  };

  if (isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <div className="w-full p-4">
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
              onConnect={ConnectButton}
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
        <GenericTable table={{ ...table, Btn: { ...table.Btn, New: undefined } }} /> // Hide New button in table since it's in header
      )}

      <NewObjectEditorDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        object={tunnel}
        title="Tunnel"
        opts={{
          nameFormat: {
            EncryptionType: (obj) => {
              return "Encryption [ " + GetEncType(obj.EncryptionType) + " ]"
            },
          },
          fields: {
            WindowsGUID: "readonly",
            DHCPToken: "readonly",
            DNSRecords: "hidden",
            Networks: "hidden",
            Routes: "hidden",
            CurveType: "hidden",
          }
        }}
        description=""
        readOnly={false}
        saveButton={async () => {
          updateTunnelMutation.mutate({ tunnel, oldTag: tunTag }, {
            onSuccess: () => {
              setModalOpen(false)
            }
          });
        }}
        onChange={(key, value, type) => {
          tunnel[key] = value;
          // console.log(key, value, type);
        }}
        onArrayChange={(key, value, index) => {
          tunnel[key][index] = value;
        }}
      />
    </div>
  );
};

