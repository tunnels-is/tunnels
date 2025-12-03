import React from "react";
import dayjs from "dayjs";
import GenericTable from "../components/GenericTable";
import { useParams } from "react-router-dom";
import { useServers } from "../hooks/useServers";
import { useConnectedDevices } from "../hooks/useDevices";
import { useAtomValue } from "jotai";
import { userAtom } from "../stores/userStore";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Activity, Server } from "lucide-react";

const ServerDevices = () => {
  const user = useAtomValue(userAtom);
  const { id } = useParams()

  // We need the server list to find the IP of the current server
  const { data: servers } = useServers(user?.ControlServer);

  const server = servers?.find(s => s._id === id);
  const serverIp = server?.IP;

  const { data: connectedDevicesData } = useConnectedDevices(user?.ControlServer, serverIp);
  const connectedDevices = connectedDevicesData || { Devices: [], DHCPAssigned: 0 };

  let table = {
    data: connectedDevices.Devices || [],
    rowClick: (obj) => {
      console.log("row click!")
      console.dir(obj)
    },
    columns: {
      Created: true,
      Activity: true,
      IP: true,
      Token: true,
      Hostname: true,
      Ports: true,
      CPU: true,
      RAM: true,
      Disk: true,
    },
    customColumns: {
    },
    columnFormat: {
      Created: (obj) => {
        return dayjs(obj.Created).format("HH:mm:ss DD-MM-YYYY")
      },
      Activity: (obj) => {
        return obj.DHCP?.Activity ? dayjs(obj.DHCP.Activity).format("HH:mm:ss DD-MM-YYYY") : ""
      },
      IP: (obj) => {
        return obj.DHCP?.IP ? obj.DHCP.IP.join(".") : ""
      },
      Token: (obj) => {
        return obj.DHCP?.Token ? obj.DHCP.Token : ""
      },
      Hostname: (obj) => {
        return obj.DHCP?.Hostname ? obj.DHCP.Hostname : ""
      },
      Ports: (obj) => {
        return "" + obj.StartPort + " - " + obj.EndPort
      },
    },
    Btn: {
      Delete: (obj) => {
        // deleteDevice(obj._id) // Not implemented in original code snippet provided, assuming it exists or TODO
      },
    },
    columnClass: {},
    headerFormat: {
      Created: () => {
        return "Connected"
      }
    },
    headers: ["Created", "Activity", "IP", "Device", "Hostname", "Ports", "CPU", "RAM", "DISK"],
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
  }

  return (
    <div className="w-full mt-16 space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">
            {server?.Tag ? `${server.Tag} - Connected Devices` : "Server Devices"}
          </h1>
          <p className="text-muted-foreground">View devices connected to this server.</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <Card className="bg-[#0B0E14] border-[#1a1f2d]">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connected Devices</CardTitle>
            <Activity className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{connectedDevices.DHCPAssigned}</div>
            <p className="text-xs text-muted-foreground">Currently active</p>
          </CardContent>
        </Card>

        <Card className="bg-[#0B0E14] border-[#1a1f2d]">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Server</CardTitle>
            <Server className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{server?.Tag || "N/A"}</div>
            <p className="text-xs text-muted-foreground font-mono">{serverIp || "No IP"}</p>
          </CardContent>
        </Card>
      </div>

      <GenericTable table={table} />
    </div>
  )
}

export default ServerDevices;
