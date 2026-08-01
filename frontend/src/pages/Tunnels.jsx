import { useEffect, useMemo } from "react"
import { useNavigate } from "react-router-dom"
import { Pencil, Server, Shield, Trash2, Zap, ZapOff } from "lucide-react"
import { Page } from "@/components/ui"
import { connect, createTunnel, deleteTunnel, disconnect, fetchServers, fetchState } from "@/store/actions"
import { useStore } from "@/store/store"

const InfoLine = ({ label, value }) => (
	<div className="flex items-baseline justify-between gap-3 py-0.5">
		<span className="shrink-0 text-[10px] font-semibold uppercase tracking-wider opacity-40">{label}</span>
		<span className="min-w-0 truncate text-right font-mono text-[11px] opacity-80">{value}</span>
	</div>
)

const Tunnels = () => {
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const askConfirm = useStore((s) => s.askConfirm)
	const advanced = useStore((s) => s.advanced)
	const navigate = useNavigate()

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	const serverMap = useMemo(() => Object.fromEntries(servers.map((s) => [s._id, s])), [servers])
	const activeMap = useMemo(
		() => Object.fromEntries((activeTunnels || []).map((at) => [at.CR?.Tag, at])),
		[activeTunnels],
	)

	if (!advanced) {
		return (
			<Page>
				<div className="flex h-40 items-center justify-center text-[13px] opacity-50">
					Enable Advanced mode in Settings to manage tunnels.
				</div>
			</Page>
		)
	}

	return (
		<Page>
			<div className="mb-4">
				<button className="btn btn-primary btn-sm" onClick={createTunnel}>
					Create
				</button>
			</div>

			{tunnels.length > 0 ? (
				<div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))" }}>
					{tunnels.map((tunnel) => {
						const active = activeMap[tunnel.Tag]
						const server = serverMap[tunnel.ServerID]
						return (
							<div
								key={tunnel.Tag}
								className={
									"group relative rounded-box border bg-base-100 p-4 transition-colors " +
									(active ? "border-success/50" : "border-base-300 hover:border-primary/40")
								}
							>
								<div className="absolute right-2 top-2 flex gap-0.5">
									<button
										className="btn btn-square btn-ghost btn-xs opacity-60"
										title="Edit"
										onClick={() => navigate(`/tunnels/${encodeURIComponent(tunnel.Tag)}/edit`)}
									>
										<Pencil size={12} />
									</button>
									<button
										className="btn btn-square btn-ghost btn-xs text-error"
										title="Delete"
										onClick={() => askConfirm("Delete", "Delete tunnel " + tunnel.Tag + "?", () => deleteTunnel(tunnel))}
									>
										<Trash2 size={12} />
									</button>
								</div>

								<div className="mb-2 flex items-center gap-2 pr-14">
									<div className={"h-2 w-2 shrink-0 rounded-full " + (active ? "animate-pulse bg-success" : "bg-base-content/20")} />
									<span className="truncate text-[13px] font-semibold tracking-tight">{tunnel.Tag}</span>
								</div>

								<InfoLine
									label="Server"
									value={
										server ? (
											<span className="inline-flex items-center gap-1">
												<Server size={10} className="text-primary/70" />
												{server.Tag}
											</span>
										) : (
											<span className="italic opacity-50">none</span>
										)
									}
								/>
								<InfoLine label="Interface" value={tunnel.IFName || <span className="italic opacity-50">none</span>} />
								<InfoLine
									label="IPv4"
									value={active?.CRResponse?.WireGuardIP || <span className="italic opacity-50">none</span>}
								/>
								<InfoLine label="MTU / TxQ" value={`${tunnel.MTU} / ${tunnel.TxQueueLen}`} />
								{active && (
									<div className="mt-2 flex gap-4 border-t border-base-200 pt-2">
										<InfoLine label="Down" value={active.Ingress} />
										<InfoLine label="Up" value={active.Egress} />
									</div>
								)}

								<button
									className="btn btn-ghost btn-sm mt-3 w-full gap-2 border border-base-300"
									onClick={() => navigate(`/tunnels/${encodeURIComponent(tunnel.Tag)}/peers`)}
								>
									<Shield size={14} />
									Firewall
								</button>

								{active ? (
									<button
										className="btn btn-sm mt-2 w-full gap-2 btn-error btn-outline"
										onClick={() => askConfirm("Disconnect", "Disconnect " + tunnel.Tag + "?", () => disconnect(active))}
									>
										<ZapOff size={14} />
										Disconnect
									</button>
								) : (
									<button
										className="btn btn-sm mt-2 w-full gap-2 btn-success btn-outline"
										onClick={() => askConfirm("Connect", "Connect " + tunnel.Tag + "?", () => connect(tunnel))}
									>
										<Zap size={14} />
										Connect
									</button>
								)}
							</div>
						)
					})}
				</div>
			) : (
				<div className="flex w-full flex-col items-center justify-center rounded-box border border-dashed border-base-300 py-20">
					<div className="text-[13px] opacity-70">No tunnels configured</div>
					<div className="mt-1 text-[11px] opacity-40">Create a tunnel to get started</div>
				</div>
			)}
		</Page>
	)
}

export default Tunnels
