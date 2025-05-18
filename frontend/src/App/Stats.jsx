
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

        <CardContent>
          <CardTitle className="text-center mb-2">Tunnel Interface</CardTitle>
          {renderKeyValue("Tag", tunnel?.Tag)}
          {renderKeyValue("Interface", tunnel?.IFName)}
          {renderKeyValue("IP", tunnel?.IPv4Address)}
          {renderKeyValue("MTU", tunnel?.MTU)}
          {renderKeyValue("DNS Blocking", tunnel?.DNSBlocking ? "enabled" : "disabled")}
          {renderKeyValue("DNS Servers", tunnel?.DNSServers?.join(" "))}
          {renderKeyValue("Encryption", state.GetEncType(tunnel?.EncryptionType))}
          {renderKeyValue("Curve", state.GetCurveType(tunnel?.CurveType))}
          {renderKeyValue("Auto Connect", tunnel?.AutoConnect ? "enabled" : "disabled")}
          {renderKeyValue("Auto Re-Connect", tunnel?.AutoReconnect ? "enabled" : "disabled")}
          {renderKeyValue("Download", ac.Ingress)}
          {renderKeyValue("Upload", ac.Egress)}

          <CardTitle className="text-center mb-2 mt-5">VPN Server</CardTitle>
          {renderKeyValue("CPU", String(ac.CPU) + "%")}
          {renderKeyValue("DISK", String(ac.DISK) + "%")}
          {renderKeyValue("MEMORY", String(ac.MEM) + "%")}
          {renderKeyValue("Ping", String(Math.floor(ac.MS / 1000)) + "ms")}
          {renderKeyValue("Ping Time", dayjs(ac.Ping).format("HH:mm:ss DD-MM-YYYY"))}

          <CardTitle className="text-center mb-2 mt-5">Local Network</CardTitle>
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

          {ac.CRResponse?.DNSRecords?.length > 0 &&
            <>
              <CardTitle className="text-center mb-2 mt-5">Domains</CardTitle>
              {ac.CRResponse?.DNSRecords?.map(r => {
                return <div className="flex flex-row gap-1">
                  <div className="">{r.Wildcard ? "*." : ""}{r.Domain}</div>
                </div>
              })}
            </>
          }



        </CardContent>
        <Button
          variant="outline"
          className={"mt-5 w-full" + state.Theme?.errorBtn}
          onClick={() => {
            state.disconnectFromVPN(ac)
          }}
        >
          Disconnect
        </Button>
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
