import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";

const Servers = () => {
  const state = GLOBAL_STATE("servers");
  const [server, setServer] = useState(undefined);

  const editorOpts = {
    baseClass: "server-object-editor",
    maxDepth: 1000,
    onlyKeys: false,
    readOnly: true,
    backButton: {
      title: "Back",
      func: () => {
        setServer(undefined);
      },
    },
  };

  useEffect(() => {
    state.GetServers();
  }, []);

  const generateServerTable = () => {
    let rows = [];

    state.Servers?.forEach((server) => {
      let con = undefined;
      let conButton = function () {
        console.log("doit!");
        state.ConfirmAndExecute(
          "success",
          "connect",
          10000,
          "",
          "Connect to " + server.Tag,
          () => {
            state.connectToVPN(undefined, server);
          },
        );
      };

      state.ActiveTunnels?.forEach((x) => {
        if (x.CR?.ServerID === server._id) {
          con = x;
          return;
        }
      });

      if (con) {
        conButton = function () {
          state.ConfirmAndExecute(
            "success",
            "disconnect",
            10000,
            "",
            "Disconnect from " + server.Tag,
            () => {
              state.disconnectFromVPN(con);
            },
          );
        };
      }

      let country = "icon";
      if (server.Country !== "") {
        country = server.Country.toLowerCase();
      }

      let opts = [];
      state?.Tunnels?.map((c) => {
        if (c.ServerID === server._id) {
          opts.push({ value: c.Tag, key: c.Tag, selected: true });
        } else {
          opts.push({ value: c.Tag, key: c.Tag, selected: false });
        }
      });

      let row = {};
      row.items = [
        {
          type: "img",
          align: "left",
          className: "flag",
          value:
            "https://raw.githubusercontent.com/tunnels-is/media/master/nl-website/v2/flags/" +
            country +
            ".svg",
        },
        {
          type: "text",
          value: server.Tag,
          color: "blue",
          click: function () {
            setServer(server);
            state.renderPage("servers");
          },
        },
        { type: "text", value: server.Server ? server.Server : "Unknown" },
        { type: "text", value: server.IP },
        {
          type: "select",
          opts: opts,
          align: "center",
          value: (
            <CustomSelect
              parentkey={server._id}
              className={"clickable"}
              placeholder={"Assign"}
              setValue={(opt) => {
                state.changeServerOnTunnelUsingTag(opt.value, server._id);
              }}
              options={opts}
            ></CustomSelect>
          ),
        },
        {
          type: "text",
          value: con ? "Disconnect" : "Connect",
          color: con ? "red" : "green",
          click: conButton,
        },
      ];
      rows.push(row);
    });

    return rows;
  };

  let rows = generateServerTable();
  const headers = [
    { value: "" },
    { value: "Tag" },
    { value: "Server" },
    { value: "IP" },
    { value: "Tunnel", align: "center" },
    { value: "" },
  ];

  if (server !== undefined) {
    return (
      <div className="connections">
        <ObjectEditor opts={editorOpts} object={server} />
      </div>
    );
  }

  return (
    <div className="ab router-wrapper public-server-wrapper">
      <NewTable
        title={"Public VPN Servers"}
        tableID={"public-servers"}
        className="router-table"
        placeholder={"Search .."}
        design="public-vpn-servers"
        background={true}
        header={headers}
        rows={rows}
      />
    </div>
  );
};

export default Servers;
