import { useEffect, useMemo, useState } from "react"
import { ChevronLeft, ChevronRight, Copy, Search, Zap, ZapOff } from "lucide-react"
import "flag-icons/css/flag-icons.min.css"
import { Card, Page, Toolbar } from "@/components/ui"
import { connectServer, disconnect, fetchServers, fetchState } from "@/store/actions"
import { countryName, normalizeCountryCode } from "@/lib/countries"
import { useStore } from "@/store/store"

const PAGE_SIZE = 20

const copyText = (text) => {
	navigator.clipboard?.writeText(text)
	useStore.getState().notifySuccess("Copied to clipboard")
}

const Flag = ({ code, className = "" }) => {
	const box = { width: "1.9rem", height: "1.4rem" }
	if (!code) {
		return (
			<span
				className={"grid shrink-0 place-items-center rounded-sm bg-base-300 text-[9px] opacity-40 " + className}
				style={box}
			>
				??
			</span>
		)
	}
	return (
		<span
			className={`fi fi-${normalizeCountryCode(code).toLowerCase()} shrink-0 rounded-sm shadow-sm ring-1 ring-base-content/10 ${className}`}
			style={{ ...box, backgroundSize: "cover" }}
			title={countryName(code)}
		/>
	)
}

const StatusPill = ({ active }) =>
	active ? (
		<span className="badge badge-success badge-sm gap-1 border-none bg-success/15 text-success">
			<span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-current" /> Connected
		</span>
	) : (
		<span className="badge badge-ghost badge-sm opacity-50">Idle</span>
	)

const Servers = () => {
	const servers = useStore((s) => s.servers)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const askConfirm = useStore((s) => s.askConfirm)
	const advanced = useStore((s) => s.advanced)

	const [filter, setFilter] = useState("")
	const [page, setPage] = useState(0)

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

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

	const activeCount = useMemo(
		() => (servers || []).filter((s) => activeByServer[s._id]).length,
		[servers, activeByServer],
	)

	const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
	const safePage = Math.min(page, totalPages - 1)
	const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE)

	const connectToServer = (server) => {

		askConfirm("Connect", "Connect to " + server.Tag + "?", () => connectServer(server))
	}

	const disconnectFromServer = (server) => {
		const active = activeByServer[server._id]
		if (!active) return
		askConfirm("Disconnect", "Disconnect from " + server.Tag + "?", () => disconnect(active))
	}

	return (
		<Page>
			<Toolbar>
				<div className="flex items-baseline gap-2">
					<span className="text-sm font-semibold tracking-tight">Servers</span>
					<span className="text-[11px] opacity-40">
						{filtered.length}
						{activeCount > 0 && <span className="text-success"> · {activeCount} connected</span>}
					</span>
				</div>
				<div className="ml-auto flex items-center gap-1.5">
					<label className="input input-xs flex w-48 items-center gap-1">
						<Search size={12} className="opacity-40" />
						<input
							placeholder="Filter by tag, IP, country..."
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

			{advanced ? (
				<Card className="overflow-hidden p-0">
					<table className="table table-sm">
						<thead>
							<tr className="text-[11px] uppercase tracking-wide opacity-50">
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
										<tr key={server._id} className={"transition-colors " + (active ? "bg-success/5" : "hover:bg-base-200/60")}>
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
															className={`fi fi-${normalizeCountryCode(server.Country).toLowerCase()} rounded-[2px]`}
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
														<button className="btn btn-success btn-xs" onClick={() => connectToServer(server)}>
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
			) : paged.length > 0 ? (
				<div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(248px, 1fr))" }}>
					{paged.map((server) => {
						const active = activeByServer[server._id]
						return (
							<div
								key={server._id}
								className={
									"group relative flex flex-col overflow-hidden rounded-box border bg-base-100 transition-all duration-200 " +
									(active
										? "border-success/50 shadow-sm ring-1 ring-success/20"
										: "border-base-300 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md")
								}
							>
								{active && <div className="absolute inset-x-0 top-0 h-0.5 bg-success" />}

								<div className="flex items-start gap-3 p-4 pb-3">
									<Flag code={server.Country} />
									<div className="min-w-0 flex-1">
										<div className="truncate text-[13px] font-semibold leading-tight tracking-tight transition-colors group-hover:text-primary">
											{server.Tag}
										</div>
										<div className="truncate text-[11px] opacity-50">{countryName(server.Country) || "Unknown region"}</div>
									</div>
									<StatusPill active={active} />
								</div>

								<div className="px-4">
									<button
										className="flex w-full items-center gap-2 rounded-btn bg-base-200/60 px-2.5 py-1.5 text-left font-mono text-[11px] opacity-70 transition-colors hover:bg-base-200 hover:opacity-100"
										title="Copy address"
										onClick={() => copyText(`${server.IP}:${server.Port}`)}
									>
										<span className="truncate">
											{server.IP}
											<span className="opacity-40">:{server.Port}</span>
										</span>
										<Copy size={11} className="ml-auto shrink-0 opacity-50" />
									</button>
								</div>

								<div className="p-4 pt-3">
									{active ? (
										<button className="btn btn-outline btn-error btn-sm btn-block gap-1" onClick={() => disconnectFromServer(server)}>
											<ZapOff size={14} /> Disconnect
										</button>
									) : (
										<button className="btn btn-success btn-sm btn-block gap-1" onClick={() => connectToServer(server)}>
											<Zap size={14} /> Connect
										</button>
									)}
								</div>
							</div>
						)
					})}
				</div>
			) : (
				<div className="flex w-full flex-col items-center justify-center gap-2 rounded-box border border-dashed border-base-300 py-20">
					<Search size={22} className="opacity-20" />
					<div className="text-[13px] opacity-70">{filter ? "No matching servers" : "No servers found"}</div>
					{filter && <div className="text-[11px] opacity-40">Try a different tag, IP, or country.</div>}
				</div>
			)}
		</Page>
	)
}

export default Servers
