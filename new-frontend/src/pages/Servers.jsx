import { useEffect, useMemo, useState } from "react"
import { ChevronLeft, ChevronRight, Copy, Search, Zap, ZapOff } from "lucide-react"
import "flag-icons/css/flag-icons.min.css"
import { Card, Page, Toolbar } from "@/components/ui"
import { connect, disconnect, fetchServers, fetchState } from "@/store/actions"
import { countryName } from "@/lib/countries"
import { useStore } from "@/store/store"

const PAGE_SIZE = 20

const copyText = (text) => {
	navigator.clipboard?.writeText(text)
	useStore.getState().notifySuccess("Copied to clipboard")
}

const Servers = () => {
	const servers = useStore((s) => s.servers)
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const askConfirm = useStore((s) => s.askConfirm)
	const notifyError = useStore((s) => s.notifyError)

	const [filter, setFilter] = useState("")
	const [page, setPage] = useState(0)

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	// server id -> live connection
	const activeByServer = useMemo(() => {
		const map = {}
		activeTunnels?.forEach((at) => at.CR?.ServerID && (map[at.CR.ServerID] = at))
		return map
	}, [activeTunnels])

	const filtered = useMemo(() => {
		if (!filter) return servers
		const f = filter.toLowerCase()
		return servers.filter(
			(s) =>
				s.Tag?.toLowerCase().includes(f) ||
				s.IP?.toLowerCase().includes(f) ||
				countryName(s.Country)?.toLowerCase().includes(f),
		)
	}, [servers, filter])

	const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
	const safePage = Math.min(page, totalPages - 1)
	const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE)

	const connectToServer = (server) => {
		const assigned = tunnels.filter((t) => t.ServerID === server._id)
		if (assigned.length > 1) {
			notifyError("Too many tunnels assigned to this server")
			return
		}
		const tunnel = assigned[0] || tunnels[0]
		if (!tunnel) {
			notifyError("No tunnel available to connect with")
			return
		}
		askConfirm("Connect", "Connect to " + server.Tag + "?", () => connect(tunnel, assigned[0] ? undefined : server))
	}

	const disconnectFromServer = (server) => {
		const active = activeByServer[server._id]
		if (!active) return
		askConfirm("Disconnect", "Disconnect from " + server.Tag + "?", () => disconnect(active))
	}

	return (
		<Page>
			<Toolbar>
				<div className="ml-auto flex items-center gap-1.5">
					<label className="input input-xs flex w-44 items-center gap-1">
						<Search size={12} className="opacity-40" />
						<input
							placeholder="Filter servers..."
							value={filter}
							onChange={(e) => {
								setFilter(e.target.value)
								setPage(0)
							}}
						/>
					</label>
					{filtered.length > PAGE_SIZE && (
						<div className="flex items-center gap-1">
							<button className="btn btn-square btn-ghost btn-xs" disabled={safePage === 0} onClick={() => setPage(safePage - 1)}>
								<ChevronLeft size={14} />
							</button>
							<span className="font-mono text-[10px] opacity-50">
								{safePage + 1}/{totalPages}
							</span>
							<button
								className="btn btn-square btn-ghost btn-xs"
								disabled={safePage >= totalPages - 1}
								onClick={() => setPage(safePage + 1)}
							>
								<ChevronRight size={14} />
							</button>
						</div>
					)}
				</div>
			</Toolbar>

			<Card className="rounded-none">
				<table className="table table-sm">
					<thead>
						<tr>
							<th className="w-8" />
							<th>Tag</th>
							<th>Country</th>
							<th>Address</th>
							<th>WireGuard</th>
							<th>Subnet</th>
							<th>Public Key</th>
							<th className="w-40 text-right" />
						</tr>
					</thead>
					<tbody>
						{paged.length > 0 ? (
							paged.map((server) => {
								const active = activeByServer[server._id]
								return (
									<tr key={server._id} className="hover">
										<td>
											<div
												className={"h-2 w-2 rounded-full " + (active ? "animate-pulse bg-success" : "bg-base-content/20")}
												title={active ? "connected" : "not connected"}
											/>
										</td>
										<td className="font-medium">{server.Tag}</td>
										<td>
											<div className="flex items-center gap-2 text-xs opacity-70">
												{server.Country && (
													<span
														className={`fi fi-${server.Country.toLowerCase()} rounded-[2px]`}
														title={countryName(server.Country)}
													/>
												)}
												{countryName(server.Country) || "—"}
											</div>
										</td>
										<td className="font-mono text-xs opacity-70">
											{server.IP}:{server.Port}
										</td>
										<td className="font-mono text-xs opacity-70">
											{server.WireGuardIface ? `${server.WireGuardIface}:${server.WireGuardPort}` : "—"}
										</td>
										<td className="font-mono text-xs opacity-70">{server.WireGuardSubnet || "—"}</td>
										<td>
											{server.WireGuardPubKey ? (
												<button
													className="btn btn-ghost btn-xs gap-1 font-mono text-[10px] font-normal opacity-70"
													title="Copy public key"
													onClick={() => copyText(server.WireGuardPubKey)}
												>
													{server.WireGuardPubKey.slice(0, 12)}…
													<Copy size={10} />
												</button>
											) : (
												<span className="text-xs opacity-40">—</span>
											)}
										</td>
										<td>
											<div className="flex justify-end gap-1">
												{active ? (
													<button className="btn btn-outline btn-error btn-xs" onClick={() => disconnectFromServer(server)}>
														<ZapOff size={12} /> Disconnect
													</button>
												) : (
													<button className="btn btn-outline btn-success btn-xs" onClick={() => connectToServer(server)}>
														<Zap size={12} /> Connect
													</button>
												)}
												<button className="btn btn-square btn-ghost btn-xs" title="Copy ID" onClick={() => copyText(server._id)}>
													<Copy size={12} />
												</button>
											</div>
										</td>
									</tr>
								)
							})
						) : (
							<tr>
								<td colSpan={8} className="py-6 text-center text-xs opacity-50">
									{filter ? "No matching servers" : "No servers found"}
								</td>
							</tr>
						)}
					</tbody>
				</table>
			</Card>
		</Page>
	)
}

export default Servers
