import { Card, InfoRow, Page } from "@/components/ui"
import { disconnect } from "@/store/actions"
import { fullDate } from "@/lib/format"
import { useStore } from "@/store/store"

const Section = ({ title, children }) => (
	<div className="mt-3 border-t border-base-300 pt-3 first:mt-0 first:border-0 first:pt-0">
		<h3 className="mb-1 text-[10px] font-semibold uppercase tracking-wider opacity-50">{title}</h3>
		{children}
	</div>
)

const Connections = () => {
	const user = useStore((s) => s.user)
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)

	const myActiveTunnels = (activeTunnels || []).filter(
		(ac) => ac.CR?.UserID && ac.CR.UserID === user?._id,
	)

	return (
		<Page>
			{!myActiveTunnels?.length && (
				<Card>
					<div className="py-6 text-center text-xs opacity-50">No active connections</div>
				</Card>
			)}
			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
				{myActiveTunnels?.map((ac) => {
					const tunnel = tunnels.find((t) => t.Tag === ac.CR?.Tag)
					return (
						<Card key={ac.ID}>
							<Section title="Tunnel Interface">
								<InfoRow label="Tag" value={tunnel?.Tag} />
								<InfoRow label="User ID" value={ac.CR?.UserID} mono />
								<InfoRow label="Interface" value={tunnel?.IFName} />
								<InfoRow label="IP" value={tunnel?.IPv4Address} mono />
								<InfoRow label="MTU" value={tunnel?.MTU} mono />
								<InfoRow label="DNS Blocking" value={tunnel?.DNSBlocking ? "enabled" : "disabled"} />
								<InfoRow label="DNS Servers" value={tunnel?.DNSServers?.join(" ")} mono />
								<InfoRow label="Handshake" value="mlkem + x25519" />
								<InfoRow label="Auto Connect" value={tunnel?.AutoConnect ? "enabled" : "disabled"} />
								<InfoRow label="Auto Re-Connect" value={tunnel?.AutoReconnect ? "enabled" : "disabled"} />
								<InfoRow label="Download" value={ac.Ingress} />
								<InfoRow label="Upload" value={ac.Egress} />
							</Section>

							<Section title="VPN Server">
								<InfoRow label="CPU" value={ac.CPU + "%"} />
								<InfoRow label="Disk" value={ac.DISK + "%"} />
								<InfoRow label="Memory" value={ac.MEM + "%"} />
								<InfoRow label="Ping" value={Math.floor(ac.MS / 1000) + "ms"} />
								<InfoRow label="Ping Time" value={ac.Ping ? fullDate(ac.Ping) : "—"} />
							</Section>

							<Section title="Local Network">
								<InfoRow label="Hostname" value={ac.DHCP?.Hostname} />
								<InfoRow label="IP" value={ac.DHCP?.IP?.join(".")} mono />
								<InfoRow label="Network" value={ac.LAN?.Network} mono />
								<InfoRow label="NAT" value={ac.LAN?.Nat} mono />
								<InfoRow label="Tag" value={ac.LAN?.Tag} />
							</Section>

							<Section title="Public Network">
								<InfoRow label="IP" value={ac.CRResponse?.InterfaceIP} mono />
								<InfoRow label="Ports" value={ac.CRResponse?.StartPort + "-" + ac.CRResponse?.EndPort} mono />
								<InfoRow label="Internet" value={ac.CRResponse?.InternetAccess ? "yes" : "no"} />
								<InfoRow label="Subnets" value={ac.CRResponse?.LocalNetworkAccess ? "yes" : "no"} />
								<InfoRow label="DNS Servers" value={ac.CRResponse?.DNSServers?.join(" ")} mono />
							</Section>

							<Section title="Routes">
								{tunnel?.EnableDefaultRoute && (
									<div className="flex gap-1 font-mono text-xs">
										default <span className="opacity-40">via</span> {tunnel?.IPv4Address}{" "}
										<span className="opacity-40">metric</span> 0
									</div>
								)}
								{ac.CRResponse?.Routes?.map((r, i) => (
									<div className="flex gap-1 font-mono text-xs" key={i}>
										{r.Address} <span className="opacity-40">via</span> {tunnel?.IPv4Address}{" "}
										<span className="opacity-40">metric</span> {r.Metric}
									</div>
								))}
							</Section>

							{ac.CRResponse?.DNSRecords?.length > 0 && (
								<Section title="Domains">
									{ac.CRResponse.DNSRecords.map((r, i) => (
										<div className="font-mono text-xs" key={i}>
											{r.Wildcard ? "*." : ""}
											{r.Domain}
										</div>
									))}
								</Section>
							)}

							<button className="btn btn-error btn-sm mt-4 w-full" onClick={() => disconnect(ac)}>
								Disconnect
							</button>
						</Card>
					)
				})}
			</div>
		</Page>
	)
}

export default Connections
