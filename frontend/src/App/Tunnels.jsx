import { useState } from "react";
import GLOBAL_STATE from "../state";
import GenericTable from "./GenericTable";
import { useEffect } from "react";
import { TableCell } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { v4 as uuidv4 } from "uuid";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { AccessibilityIcon } from "lucide-react";

const Tunnels = () => {
  const state = GLOBAL_STATE("tunnels");
  const [tunnel, setTunnel] = useState(undefined);
  const [tunnels, setTunnels] = useState([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [tunTag, setTunTag] = useState("")

  useEffect(() => {
    let x = async () => {
      let user = await state.GetUser();
      if (!user) {
        return <Navigate to={"/login"} />;
      }
      await state.GetServers();
      state.GetBackendState();
    };
    x();
  }, []);

  const ConnectButton = (obj) => {
    let active = false;
    state.ActiveTunnels?.map((x) => {
      if (x.CR?.Tag === obj.Tag) {
        active = true;
        return;
      }
    });

    let connect = undefined;
    let label = "";

    if (active) {
      label = "Disconnect";
      connect = () => {
        state.ConfirmAndExecute(
          "success",
          "disconnect",
          10000,
          "",
          "Disconnect from " + obj.Tag,
          () => {
            state.disconnectFromVPN(obj);
          },
        );
      };
    } else {
      label = "Connect";
      connect = () => {
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
    }

    return <DropdownMenuItem
      key="connect"
      onClick={() => connect()}
      className="cursor-pointer text-emerald-500"
    >
      <AccessibilityIcon className="w-4 h-4 mr-2" /> {label}
    </DropdownMenuItem >
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
    headers: ["Tag", "IFName", "ServerID"],
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
      />
    </div>
  );
};

export default Tunnels;
