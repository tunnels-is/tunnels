
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import InfoItem from "./component/InfoItem";
import dayjs from "dayjs";

const Stats = () => {
  const state = GLOBAL_STATE("stats")

  const renderKeyValue = (key, value) => {
    return (
      <div className="flex flex-row gap-2 justify-between">
        <p className="text-sm font-medium">
          {key}
        </p>
        <p className="text-sm text-muted-foreground">
          {value}
        </p>
      </div>
    )
  }

  const renderCard = (ac) => {
    let tunnel = undefined
    state.Tunnels?.forEach((t, i) => {
      if (t.Tag === ac.CR?.Tag) {
        tunnel = state.Tunnels[i]
      }
    })
    return (
      <Card className="p-5">
        <CardHeader>
          <CardTitle>tunnels</CardTitle>
          <CardDescription>Connected: 12-23-1232</CardDescription>
        </CardHeader>
        <CardContent>
          <CardTitle className="text-center mb-2">Server</CardTitle>
          {renderKeyValue("CPU", String(ac.CPU) + "%")}
          {renderKeyValue("DISK", String(ac.DISK) + "%")}
          {renderKeyValue("MEMORY", String(ac.MEM) + "%")}
          {renderKeyValue("Ping", String(Math.floor(ac.MS / 1000)) + "ms")}
          {renderKeyValue("Ping Time", dayjs(ac.Ping).format("HH:mm:ss DD-MM-YYYY"))}
          {renderKeyValue("Download", ac.Ingress)}
          {renderKeyValue("Upload", ac.Egress)}

          <CardTitle className="text-center mb-2 mt-5">Local Network</CardTitle>
          {renderKeyValue("Tag", ac.LAN?.Tag)}
          {renderKeyValue("Hostname", ac.DHCP?.Hostname)}
          {renderKeyValue("IP", ac.DHCP?.IP?.join("."))}
          {renderKeyValue("Network", ac.LAN?.Network)}
          {renderKeyValue("NAT", ac.LAN?.Nat)}
          {renderKeyValue("Tag", ac.LAN?.Tag)}

          <CardTitle className="text-center mb-2 mt-5">Public Network</CardTitle>
          {renderKeyValue("IP", ac.CRResponse?.InterfaceIP)}
          {renderKeyValue("Ports", String(ac.CRResponse?.StartPort) + "-" + String(ac.CRResponse?.EndPort))}
          {renderKeyValue("Internet", ac.CRResponse?.InternetAccess ? "yes" : "no")}
          {renderKeyValue("Subnets", ac.CRResponse?.LocalNetworkAccess ? "yes" : "no")}
          {renderKeyValue("DNS Servers", ac.CRResponse?.DNSServers?.join(" "))}

          <CardTitle className="text-center mb-2 mt-5">Routes</CardTitle>
          {(tunnel?.EnableDefaultRoute === true) &&
            <div className="flex flex-row gap-1">
              <div className="">default</div>
              <div className="text-muted-foreground">via</div>
              <div className="">{tunnel?.IPv4Address}</div>
              <div className="text-muted-foreground">metric</div>
              <div className="">0</div>
            </div>
          }

          {ac.CRResponse?.Routes?.map(r => {
            return <div className="flex flex-row gap-1">
              <div className="">{r.Address}</div>
              <div className="text-muted-foreground">via</div>
              <div className="">{tunnel?.IPv4Address}</div>
              <div className="text-muted-foreground">metric</div>
              <div className="">{r.Metric}</div>
            </div>
          })}


        </CardContent>
        <CardFooter className="mt-6">
          <Button
            variant="outline"
            className={"w-full" + state.Theme?.errorBtn}
            onClick={() => {
              state.disconnectFromVPN(tunnel)
            }}
          >
            Disconnect
          </Button>
        </CardFooter>
      </Card >
    )
  }

  return (
    <div className="flex">
      {state.ActiveTunnels?.map(c => {
        return renderCard(c)
      })}
    </div >
  )
}

export default Stats
