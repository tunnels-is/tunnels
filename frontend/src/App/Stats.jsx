import { Card, CardContent } from "@/components/ui/card";
import GLOBAL_STATE from "../state";
import { Button } from "@/components/ui/button";
import dayjs from "dayjs";

const Stats = () => {
  const state = GLOBAL_STATE("stats")

  const renderKeyValue = (key, value) => {
    return (
      <div className="flex flex-row justify-between py-1">
        <p className="text-[13px] font-medium text-white/60">
          {key}
        </p>
        <p className="text-[13px] text-white/80">
          {value}
        </p>
      </div>
    )
  }

  const SectionTitle = ({ children }) => (
    <div className="pt-4 pb-1 mt-2 border-t border-[#1e2433]">
      <h3 className="text-[11px] uppercase tracking-widest text-white/50">{children}</h3>
    </div>
  )

  const renderCard = (ac) => {
    let tunnel = undefined
    state.Tunnels?.forEach((t, i) => {
      if (t.Tag === ac.CR?.Tag) {
        tunnel = state.Tunnels[i]
      }
    })
    return (
      <Card className="bg-[#0a0d14] border-[#1e2433] p-4" key={ac.CR?.Tag}>
        <CardContent>
          <SectionTitle>Tunnel Interface</SectionTitle>
          {renderKeyValue("Tag", tunnel?.Tag)}
          {renderKeyValue("Interface", tunnel?.IFName)}
          {renderKeyValue("IP", tunnel?.IPv4Address)}
          {renderKeyValue("MTU", tunnel?.MTU)}
          {renderKeyValue("DNS Blocking", tunnel?.DNSBlocking ? "enabled" : "disabled")}
          {renderKeyValue("DNS Servers", tunnel?.DNSServers?.join(" "))}
          {renderKeyValue("Encryption", state.GetEncType(tunnel?.EncryptionType))}
          {renderKeyValue("Handshake", "mlkem + x25519")}
          {renderKeyValue("Auto Connect", tunnel?.AutoConnect ? "enabled" : "disabled")}
          {renderKeyValue("Auto Re-Connect", tunnel?.AutoReconnect ? "enabled" : "disabled")}
          {renderKeyValue("Download", ac.Ingress)}
          {renderKeyValue("Upload", ac.Egress)}

          <SectionTitle>VPN Server</SectionTitle>
          {renderKeyValue("CPU", String(ac.CPU) + "%")}
          {renderKeyValue("DISK", String(ac.DISK) + "%")}
          {renderKeyValue("MEMORY", String(ac.MEM) + "%")}
          {renderKeyValue("Ping", String(Math.floor(ac.MS / 1000)) + "ms")}
          {renderKeyValue("Ping Time", dayjs(ac.Ping).format("HH:mm:ss DD-MM-YYYY"))}

          <SectionTitle>Local Network</SectionTitle>
          {renderKeyValue("Hostname", ac.DHCP?.Hostname)}
          {renderKeyValue("IP", ac.DHCP?.IP?.join("."))}
          {renderKeyValue("Network", ac.LAN?.Network)}
          {renderKeyValue("NAT", ac.LAN?.Nat)}
          {renderKeyValue("Tag", ac.LAN?.Tag)}

          <SectionTitle>Public Network</SectionTitle>
          {renderKeyValue("IP", ac.CRResponse?.InterfaceIP)}
          {renderKeyValue("Ports", String(ac.CRResponse?.StartPort) + "-" + String(ac.CRResponse?.EndPort))}
          {renderKeyValue("Internet", ac.CRResponse?.InternetAccess ? "yes" : "no")}
          {renderKeyValue("Subnets", ac.CRResponse?.LocalNetworkAccess ? "yes" : "no")}
          {renderKeyValue("DNS Servers", ac.CRResponse?.DNSServers?.join(" "))}

          <SectionTitle>Routes</SectionTitle>
          {(tunnel?.EnableDefaultRoute === true) &&
            <div className="flex flex-row gap-1 text-[13px]">
              <div>default</div>
              <div className="text-white/40">via</div>
              <div>{tunnel?.IPv4Address}</div>
              <div className="text-white/40">metric</div>
              <div>0</div>
            </div>
          }

          {ac.CRResponse?.Routes?.map((r, idx) => {
            return <div className="flex flex-row gap-1 text-[13px]" key={idx}>
              <div>{r.Address}</div>
              <div className="text-white/40">via</div>
              <div>{tunnel?.IPv4Address}</div>
              <div className="text-white/40">metric</div>
              <div>{r.Metric}</div>
            </div>
          })}

          {ac.CRResponse?.DNSRecords?.length > 0 &&
            <>
              <SectionTitle>Domains</SectionTitle>
              {ac.CRResponse?.DNSRecords?.map((r, idx) => {
                return <div className="flex flex-row gap-1 text-[13px]" key={idx}>
                  <div>{r.Wildcard ? "*." : ""}{r.Domain}</div>
                </div>
              })}
            </>
          }

        </CardContent>
        <Button
          className={"mt-4 w-full" + state.Theme?.errorBtn}
          onClick={() => {
            state.disconnectFromVPN(ac)
          }}
        >
          Disconnect
        </Button>
      </Card>
    )
  }

  return (
    <div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {state.ActiveTunnels?.map(c => {
          return renderCard(c)
        })}
      </div>
    </div>
  )
}

export default Stats
