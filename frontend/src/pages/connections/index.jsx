
import { Card, CardAction, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useTunnels, useDisconnectTunnel } from "@/hooks/useTunnels";
import { toast } from "sonner";
import dayjs from "dayjs";
import { useQuery } from "@tanstack/react-query";
import { getBackendState } from "@/api/app";
import { MoveHorizontal } from "lucide-react";
import { Info } from "lucide-react";
import { Sheet, SheetClose, SheetContent, SheetFooter, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { Fragment } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Unplug } from "lucide-react";
import { Link } from "react-router-dom";
import { Spinner } from "@/components/ui/spinner";






const GetEncType = (int) => {
  const types = ["None", "AES128", "AES256", "CHACHA20"];
  return types[int] || "Unknown";
};

const renderKeyValue = (key, value) => {
  return (
    <div className="flex flex-row gap-2 justify-between my-4">
      <p className="text-sm font-medium">
        {key}
      </p>
      <p className="text-sm text-muted-foreground">
        {value}
      </p>
    </div>
  )
}

const renderCard = (tunnels, handleDisconnect) => (ac) => {
  const tunnel = tunnels?.find(t => t.Tag === ac.CR?.Tag);

  return <ConnectionCard tunnel={tunnel} ac={ac} onDisconnect={handleDisconnect} />
};

function ConnectionCard({ tunnel, ac, onDisconnect }) {
  return (
    <Sheet>
      <Card>
        <CardHeader>
          <CardTitle className="flex justify-between">
            <span className="flex gap-2 items-center">
              {tunnel?.IPv4Address} <MoveHorizontal /> {ac.CRResponse?.InterfaceIP}
            </span>

          </CardTitle>
          <CardAction>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon"> <Info /> </Button>

            </SheetTrigger>
          </CardAction>
        </CardHeader>
        <CardContent>
          {renderKeyValue("Interface", tunnel.IFName)}
          {renderKeyValue("IP", tunnel.IPv4Address)}
          {renderKeyValue("DNS Blocking", tunnel.DNSBlocking ? "enabled" : "disabled")}
          {renderKeyValue("Encryption", GetEncType(tunnel.EncryptionType))}
        </CardContent>
        <CardFooter>
          <Button onClick={e => onDisconnect(ac)} className="text-red-400" variant="outline">
            Disconnect
          </Button>
        </CardFooter>
      </Card>
      <SheetContent className="p-4">
        <SheetHeader>
          <SheetTitle>More information about this connection</SheetTitle>
        </SheetHeader>
        <ScrollArea className="px-3 w-full">
          <h2 className="text-center font-semibold">Tunnel Interface</h2>
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

          <h2 className="text-center font-semibold">VPN Server</h2>
          {renderKeyValue("CPU", String(ac.CPU) + "%")}
          {renderKeyValue("DISK", String(ac.DISK) + "%")}
          {renderKeyValue("MEMORY", String(ac.MEM) + "%")}
          {renderKeyValue("Ping", String(Math.floor(ac.MS / 1000)) + "ms")}
          {renderKeyValue("Ping Time", dayjs(ac.Ping).format("HH:mm:ss DD-MM-YYYY"))}
          {renderKeyValue("Hostname", ac.DHCP?.Hostname)}
          {renderKeyValue("IP", ac.DHCP?.IP?.join("."))}
          {renderKeyValue("Network", ac.LAN?.Network)}
          {renderKeyValue("NAT", ac.LAN?.Nat)}
          {renderKeyValue("Tag", ac.LAN?.Tag)}

          <h2 className="text-center font-semibold">Public Network</h2>
          {renderKeyValue("IP", ac.CRResponse?.InterfaceIP)}
          {renderKeyValue("Ports", String(ac.CRResponse?.StartPort) + "-" + String(ac.CRResponse?.EndPort))}
          {renderKeyValue("Internet", ac.CRResponse?.InternetAccess ? "yes" : "no")}
          {renderKeyValue("Subnets", ac.CRResponse?.LocalNetworkAccess ? "yes" : "no")}
          {renderKeyValue("DNS Servers", ac.CRResponse?.DNSServers?.join(" "))}
          <h2 className="text-center font-semibold">Routes</h2>
          {tunnel?.EnableDefaultRoute &&
            <div className="flex flex-row gap-1">
              <div className="">default</div>
              <div className="text-muted-foreground"> via</div>
              <div className="">{tunnel?.IPv4Address}</div>
              <div className="text-muted-foreground">metric</div>
              <div className="">0</div>
            </div>
          }
          {
            ac.CRResponse?.Routes?.map((r, i) => (
              <div key={i} className="flex flex-row gap-1">
                <div className="">{r.Address}</div>
                <div className="text-muted-foreground">via</div>
                <div className="">{tunnel?.IPv4Address}</div>
                <div className="text-muted-foreground">metric</div>
                <div className="">{r.Metric}</div>
              </div>
            ))
          }
          {
            ac.CRResponse?.DNSRecords?.length > 0 && <>
              <span className="text-center mb-2 mt-5">Domains</span>
              {
                ac.CRResponse?.DNSRecords?.map((r, i) => (
                  <div className="flex flex-row gap-1" key={i}>
                    <div className="">{r.Wildcard ? "*." : ""}{r.Domain}</div>
                  </div>
                ))
              }
            </>
          }
        </ScrollArea>
        <SheetFooter>
          <SheetClose asChild>
            <Button onClick={_ => onDisconnect(ac)} variant="destructive">Disconnect</Button>
          </SheetClose>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export default function StatsPage() {
  const activeTunnels = useQuery({
    queryKey: ["activeTunnels"],
    queryFn: async () => (await getBackendState()).ActiveTunnels ?? []
  });

  const tunnels = useTunnels();
  const dcTunnelMutation = useDisconnectTunnel();



  const handleDisconnect = async (ac) => {
    try {
      await dcTunnelMutation.mutateAsync(ac.CR?.Tag);
      toast.success("Disconnected");
    } catch (e) {
      toast.error("Failed to disconnect");
    }
  };
  return (
    <Fragment>
      <div className="p-2">
        <span className="text-lg">Manage connections.</span>
        {
          activeTunnels.isLoading && (
            <span> <Spinner /> Loading... </span>
          )
        }
        {
          activeTunnels.isFetched
            && activeTunnels.data.length > 0 ?
            <div className="grid grid-cols-3">
              {activeTunnels.data?.map(renderCard(tunnels, handleDisconnect))}
            </div> : (
              <Empty>
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <Unplug />
                  </EmptyMedia>
                  <EmptyTitle>No connections yet</EmptyTitle>
                  <EmptyDescription>
                    There's no active connections yet. Go look at listed private servers or tunnels to create a connection.
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <div className="flex gap-2">
                    <Button asChild>
                      <Link to="/tunnels">
                        Tunnels
                      </Link>
                    </Button>
                    <Button variant="outline" asChild>
                      <Link to="/servers">Private Servers</Link>
                    </Button>
                  </div>
                </EmptyContent>
              </Empty>
            )}
      </div>

    </Fragment>
  )
}
