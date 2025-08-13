import { useState } from "react";
import GLOBAL_STATE from "../state";
import GenericTable from "./GenericTable";
import { useEffect } from "react";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { CircleArrowRight } from "lucide-react";
import { LogOut } from "lucide-react";

const Tunnels = () => {
  const state = GLOBAL_STATE("tunnels");
  const [tunnel, setTunnel] = useState(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [tunTag, setTunTag] = useState("")

  useEffect(() => {
    let x = async () => {
      if (!state.User) {
        return
      }
      await state.GetServers();
      state.GetBackendState();
    };
    x();
  }, []);

  const ConnectButton = (obj) => {
    let active = undefined;
    state.ActiveTunnels?.map((x) => {
      if (x.CR?.Tag === obj.Tag) {
        active = x
        return;
      }
    });

    let connect = () => {
      state.ConfirmAndExecute(
        "success",
        "connect",
        10000,
        "",
        "Connect to " + obj.Tag,
        () => {
          state.connectToVPN(obj);
        },
      );
    };

    let disconnect = undefined
    if (active) {
      disconnect = () => {
        state.ConfirmAndExecute(
          "success",
          "disconnect",
          10000,
          "",
          "Disconnect from " + obj.Tag,
          () => {
            state.disconnectFromVPN(active);
          },
        );
      };
    }

    return <div>
      <DropdownMenuItem
        key="connect"
        onClick={() => connect()}
        className="cursor-pointer text-[#3a994c] "
      >
        <CircleArrowRight className="w-4 h-4 mr-2" /> Connect
      </DropdownMenuItem >
      {disconnect &&
        <DropdownMenuItem
          key="disconnect"
          onClick={() => disconnect()}
          className={"cursor-pointer text-[#ef4444]"}
        >
          <LogOut className="w-4 h-4 mr-2" /> Disconnect
        </DropdownMenuItem >
      }
    </div>

  };

  const newServer = async () => {
    await state.createTunnel();
  };

  let table = {
    data: state?.Tunnels,
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
        state.v2_TunnelDelete(obj);
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
    more: () => { },
  };

  return (
    <div className="tunnels">
      <GenericTable table={table} />
      <NewObjectEditorDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        object={tunnel}
        title="Tunnel"
        opts={{
          nameFormat: {
            EncryptionType: (obj) => {
              return "Encryption [ " + state.GetEncType(obj.EncryptionType) + " ]"
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
          let ok = await state.v2_TunnelSave(tunnel, tunTag)
          if (ok === true) {
            setModalOpen(false)
          }
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

export default Tunnels;
