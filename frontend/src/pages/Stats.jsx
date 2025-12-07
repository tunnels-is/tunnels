import { Card, CardContent, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import dayjs from "dayjs";
import { useAtomValue } from "jotai";
import { activeTunnelsAtom } from "../stores/tunnelStore";
import { useTunnels } from "../hooks/useTunnels";
import { toast } from "sonner";
import { disconnectTunnel } from "../api/tunnels";

const Stats = () => {
  const activeTunnels = useAtomValue(activeTunnelsAtom);
  const { data: tunnels } = useTunnels();

  const handleDisconnect = async (ac) => {
    try {
      await disconnectTunnel(ac.CR?.Tag);
      toast.success("Disconnected");
    } catch (e) {
      toast.error("Failed to disconnect");
    }
  };

  const GetEncType = (int) => {
    const types = ["None", "AES128", "AES256", "CHACHA20"];
    return types[int] || "Unknown";
  };

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
    let tunnel = tunnels?.find(t => t.Tag === ac.CR?.Tag);

    return (
      <Card className="p-5" key={ac.CR?.Tag || Math.random()}>

        <CardContent>
          <CardTitle className="text-center mb-2">Tunnel Interface</CardTitle>
          {renderKeyValue("Tag", tunnel?.Tag)}
          {renderKeyValue("Interface", tunnel?.IFName)}
          {renderKeyValue("IP", tunnel?.IPv4Address)}
          {renderKeyValue("MTU", tunnel?.MTU)}
          {renderKeyValue("DNS Blocking", tunnel?.DNSBlocking ? "enabled" : "disabled")}
          {renderKeyValue("DNS Servers", tunnel?.DNSServers?.join(" "))}
          {renderKeyValue("Encryption", GetEncType(tunnel?.EncryptionType))}
          {renderKeyValue("Handshake", "mlkem + x25519")}
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

          {ac.CRResponse?.Routes?.map((r, i) => {
            return <div className="flex flex-row gap-1" key={i}>
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
              {ac.CRResponse?.DNSRecords?.map((r, i) => {
                return <div className="flex flex-row gap-1" key={i}>
                  <div className="">{r.Wildcard ? "*." : ""}{r.Domain}</div>
                </div>
              })}
            </>
          }



        </CardContent>
        <Button
          className={"mt-5 w-full"}
          onClick={() => handleDisconnect(ac)}
        >
          Disconnect
        </Button>
      </Card >
    )
  }

  return (
    <div className="flex">
      {activeTunnels ? activeTunnels?.map(renderCard) : (
        <div className="w-full">
          <span className="text-center">No active tunnels</span>
        </div>
      )}
    </div >
  )
}

export default Stats
