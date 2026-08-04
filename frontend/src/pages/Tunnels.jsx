import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Network, Pencil, Plus, Search, Server, Shield, Trash2, Zap, ZapOff } from "lucide-react"
import { Page, Toolbar } from "@/components/ui"
import { connect, createTunnel, deleteTunnel, disconnect, fetchServers, fetchState } from "@/store/actions"
import { useStore } from "@/store/store"

const Tunnels = () => {
	const user = useStore((s) => s.user)
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const askConfirm = useStore((s) => s.askConfirm)
	const advanced = useStore((s) => s.advanced)
	const navigate = useNavigate()

	const [filter, setFilter] = useState("")

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	const serverMap = useMemo(() => Object.fromEntries(servers.map((s) => [s._id, s])), [servers])
	// Only this UI account's connections count as "connected" for highlighting.
	const activeMap = useMemo(() => {
		const uid = user?._id
		const map = {}
		for (const at of activeTunnels || []) {
			if (at.CR?.Tag && uid && at.CR?.UserID === uid) map[at.CR.Tag] = at
		}
		return map
	}, [activeTunnels, user?._id])

	const activeCount = useMemo(
		() => (tunnels || []).filter((t) => activeMap[t.Tag]).length,
		[tunnels, activeMap],
	)

	const filtered = useMemo(() => {
		if (!filter) return tunnels || []
		const f = filter.toLowerCase()
		return (tunnels || []).filter((t) => {
			const server = serverMap[t.ServerID]
			return (
				t.Tag?.toLowerCase().includes(f) ||
				t.IFName?.toLowerCase().includes(f) ||
				server?.Tag?.toLowerCase().includes(f) ||
				server?.IP?.toLowerCase().includes(f)
			)
		})
	}, [tunnels, filter, serverMap])

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
			<Toolbar>
				<div className="flex items-baseline gap-2">
					<span className="text-sm font-semibold tracking-tight">Tunnels</span>
					<span className="text-[11px] opacity-40">
						{filtered.length}
						{filter && tunnels?.length !== filtered.length && (
							<span> of {tunnels?.length || 0}</span>
						)}
						{activeCount > 0 && <span className="text-success"> · {activeCount} connected</span>}
					</span>
				</div>
				<div className="ml-auto flex items-center gap-1.5">
					<label className="input input-xs flex w-48 items-center gap-1">
						<Search size={12} className="opacity-40" />
						<input
							placeholder="Filter by tag, interface, server..."
							value={filter}
							onChange={(e) => setFilter(e.target.value)}
						/>
					</label>
					<button className="btn btn-primary btn-xs gap-1" onClick={createTunnel}>
						<Plus size={12} /> Create
					</button>
				</div>
			</Toolbar>

			<div className="overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm">
				<div className="overflow-x-auto">
					<table className="w-full border-collapse text-left">
						<thead>
							<tr className="border-b border-base-300 bg-base-200/50">
								<th className="w-10 px-4 py-3" />
								<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
									Tunnel
								</th>
								<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
									Server
								</th>
								<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 md:table-cell">
									Interface
								</th>
								<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 lg:table-cell">
									IPv4
								</th>
								<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 xl:table-cell">
									MTU / TxQ
								</th>
								<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 2xl:table-cell">
									Traffic
								</th>
								<th className="w-52 px-4 py-3" />
							</tr>
						</thead>
						<tbody className="divide-y divide-base-200">
							{filtered.length > 0 ? (
								filtered.map((tunnel) => {
									const active = activeMap[tunnel.Tag]
									const server = serverMap[tunnel.ServerID]
									const ipv4 = active?.CRResponse?.WireGuardIP
									return (
										<tr
											key={tunnel.Tag}
											className={
												"group transition-colors duration-150 " +
												(active
													? "bg-success/[0.04] hover:bg-success/[0.07]"
													: "hover:bg-base-200/40")
											}
										>
											<td className="px-4 py-3.5">
												<div className="flex items-center justify-center">
													<span
														className={
															"block h-2 w-2 rounded-full ring-2 " +
															(active
																? "animate-pulse bg-success ring-success/25"
																: "bg-base-content/15 ring-transparent")
														}
														title={active ? "Connected" : "Idle"}
													/>
												</div>
											</td>
											<td className="px-3 py-3.5">
												<div className="flex min-w-0 flex-col gap-0.5">
													<span className="truncate text-[13px] font-semibold tracking-tight">
														{tunnel.Tag}
													</span>
													{active && (
														<span className="text-[10px] font-medium text-success">Connected</span>
													)}
												</div>
											</td>
											<td className="px-3 py-3.5">
												{server ? (
													<div className="flex min-w-0 items-center gap-1.5">
														<Server size={12} className="shrink-0 text-base-content/35" />
														<div className="min-w-0">
															<div className="truncate text-xs font-medium text-base-content/80">
																{server.Tag}
															</div>
															<div className="truncate font-mono text-[10px] text-base-content/40">
																{server.IP}
																{server.Port != null && (
																	<span className="text-base-content/25">:{server.Port}</span>
																)}
															</div>
														</div>
													</div>
												) : (
													<span className="text-xs italic text-base-content/35">None</span>
												)}
											</td>
											<td className="hidden px-3 py-3.5 md:table-cell">
												<span className="font-mono text-[11px] text-base-content/55">
													{tunnel.IFName || "—"}
												</span>
											</td>
											<td className="hidden px-3 py-3.5 lg:table-cell">
												{ipv4 ? (
													<span className="inline-flex rounded-md bg-base-200/70 px-2 py-1 font-mono text-[11px] text-base-content/70">
														{ipv4}
													</span>
												) : (
													<span className="text-xs text-base-content/30">—</span>
												)}
											</td>
											<td className="hidden px-3 py-3.5 xl:table-cell">
												<span className="font-mono text-[11px] text-base-content/50">
													{tunnel.MTU ?? "—"}
													<span className="text-base-content/25"> / </span>
													{tunnel.TxQueueLen ?? "—"}
												</span>
											</td>
											<td className="hidden px-3 py-3.5 2xl:table-cell">
												{active ? (
													<div className="flex flex-col gap-0.5 font-mono text-[10px]">
														<span className="text-success/80">
															↓ {active.Ingress || "0"}
														</span>
														<span className="text-warning/80">
															↑ {active.Egress || "0"}
														</span>
													</div>
												) : (
													<span className="text-xs text-base-content/30">—</span>
												)}
											</td>
											<td className="px-4 py-3.5">
												<div className="flex items-center justify-end gap-1">
													{active ? (
														<button
															className="btn btn-outline btn-error btn-xs gap-1"
															onClick={() =>
																askConfirm("Disconnect", "Disconnect " + tunnel.Tag + "?", () =>
																	disconnect(active),
																)
															}
														>
															<ZapOff size={12} /> Disconnect
														</button>
													) : (
														<button
															className="btn btn-success btn-xs gap-1 opacity-90 group-hover:opacity-100"
															onClick={() =>
																askConfirm("Connect", "Connect " + tunnel.Tag + "?", () =>
																	connect(tunnel),
																)
															}
														>
															<Zap size={12} /> Connect
														</button>
													)}
													<button
														className="btn btn-ghost btn-xs gap-1 text-base-content/55 hover:text-base-content"
														title="Firewall / peers"
														onClick={() =>
															navigate(`/tunnels/${encodeURIComponent(tunnel.Tag)}/peers`)
														}
													>
														<Shield size={12} /> Firewall
													</button>
													<button
														className="btn btn-square btn-ghost btn-xs text-base-content/50 hover:text-base-content"
														title="Edit"
														onClick={() =>
															navigate(`/tunnels/${encodeURIComponent(tunnel.Tag)}/edit`)
														}
													>
														<Pencil size={13} />
													</button>
													<button
														className="btn btn-square btn-ghost btn-xs text-error/70"
														title="Delete"
														onClick={() =>
															askConfirm("Delete", "Delete tunnel " + tunnel.Tag + "?", () =>
																deleteTunnel(tunnel),
															)
														}
													>
														<Trash2 size={13} />
													</button>
												</div>
											</td>
										</tr>
									)
								})
							) : (
								<tr>
									<td colSpan={8} className="px-4 py-16 text-center">
										<div className="flex flex-col items-center gap-2">
											{filter ? (
												<>
													<Search size={20} className="text-base-content/20" />
													<span className="text-[13px] text-base-content/60">No matching tunnels</span>
													<span className="text-[11px] text-base-content/35">
														Try a different tag, interface, or server.
													</span>
												</>
											) : (
												<>
													<Network size={20} className="text-base-content/20" />
													<span className="text-[13px] text-base-content/60">No tunnels configured</span>
													<span className="text-[11px] text-base-content/35">
														Create a tunnel to get started
													</span>
													<button className="btn btn-primary btn-xs mt-2 gap-1" onClick={createTunnel}>
														<Plus size={12} /> Create tunnel
													</button>
												</>
											)}
										</div>
									</td>
								</tr>
							)}
						</tbody>
					</table>
				</div>
			</div>
		</Page>
	)
}

export default Tunnels
