import { useEffect, useMemo, useState } from "react"
import { ChevronLeft, ChevronRight, Copy, Search, Zap, ZapOff } from "lucide-react"
import "flag-icons/css/flag-icons.min.css"
import { Page, Toolbar } from "@/components/ui"
import { connectServer, disconnect, fetchServers, fetchState } from "@/store/actions"
import { countryName, normalizeCountryCode } from "@/lib/countries"
import { useStore } from "@/store/store"

const PAGE_SIZE = 20

const copyText = (text) => {
	navigator.clipboard?.writeText(text)
	useStore.getState().notifySuccess("Copied to clipboard")
}

const Flag = ({ code, className = "", size = "md" }) => {
	const box = size === "sm" ? { width: "1.25rem", height: "0.95rem" } : { width: "1.9rem", height: "1.4rem" }
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
	const user = useStore((s) => s.user)
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

	// Only treat as connected when the live tunnel belongs to the active UI account.
	const activeByServer = useMemo(() => {
		const map = {}
		const uid = user?._id
		activeTunnels?.forEach((at) => {
			if (at.CR?.ServerID && uid && at.CR?.UserID === uid) map[at.CR.ServerID] = at
		})
		return map
	}, [activeTunnels, user?._id])

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
				<div className="overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm">
					<div className="overflow-x-auto">
						<table className="w-full border-collapse text-left">
							<thead>
								<tr className="border-b border-base-300 bg-base-200/50">
									<th className="w-10 px-4 py-3" />
									<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
										Server
									</th>
									<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
										Country
									</th>
									<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
										Address
									</th>
									<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 xl:table-cell">
										WireGuard
									</th>
									<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 lg:table-cell">
										Subnet
									</th>
									<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 2xl:table-cell">
										Public Key
									</th>
									<th className="w-44 px-4 py-3" />
								</tr>
							</thead>
							<tbody className="divide-y divide-base-200">
								{paged.length > 0 ? (
									paged.map((server) => {
										const active = activeByServer[server._id]
										return (
											<tr
												key={server._id}
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
															{server.Tag}
														</span>
														{active && (
															<span className="text-[10px] font-medium text-success">Connected</span>
														)}
													</div>
												</td>
												<td className="px-3 py-3.5">
													<div className="flex items-center gap-2">
														<Flag code={server.Country} size="sm" />
														<span className="text-xs text-base-content/60">
															{countryName(server.Country) || "—"}
														</span>
													</div>
												</td>
												<td className="px-3 py-3.5">
													<button
														className="inline-flex max-w-full items-center gap-1.5 rounded-md bg-base-200/70 px-2 py-1 font-mono text-[11px] text-base-content/70 transition-colors hover:bg-base-200 hover:text-base-content"
														title="Copy address"
														onClick={() => copyText(`${server.IP}:${server.Port}`)}
													>
														<span className="truncate">
															{server.IP}
															<span className="text-base-content/35">:{server.Port}</span>
														</span>
														<Copy size={10} className="shrink-0 opacity-40" />
													</button>
												</td>
												<td className="hidden px-3 py-3.5 xl:table-cell">
													<span className="font-mono text-[11px] text-base-content/50">
														{server.WireGuardIface
															? `${server.WireGuardIface}:${server.WireGuardPort}`
															: "—"}
													</span>
												</td>
												<td className="hidden px-3 py-3.5 lg:table-cell">
													<span className="font-mono text-[11px] text-base-content/50">
														{server.WireGuardSubnet || "—"}
													</span>
												</td>
												<td className="hidden px-3 py-3.5 2xl:table-cell">
													{server.WireGuardPubKey ? (
														<button
															className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 font-mono text-[10px] text-base-content/50 transition-colors hover:bg-base-200 hover:text-base-content/80"
															title="Copy public key"
															onClick={() => copyText(server.WireGuardPubKey)}
														>
															{server.WireGuardPubKey.slice(0, 12)}…
															<Copy size={10} className="opacity-40" />
														</button>
													) : (
														<span className="text-xs text-base-content/30">—</span>
													)}
												</td>
												<td className="px-4 py-3.5">
													<div className="flex items-center justify-end gap-1.5">
														{active ? (
															<button
																className="btn btn-outline btn-error btn-xs gap-1"
																onClick={() => disconnectFromServer(server)}
															>
																<ZapOff size={12} /> Disconnect
															</button>
														) : (
															<button
																className="btn btn-success btn-xs gap-1 opacity-90 group-hover:opacity-100"
																onClick={() => connectToServer(server)}
															>
																<Zap size={12} /> Connect
															</button>
														)}
														<button
															className="btn btn-square btn-ghost btn-xs opacity-0 transition-opacity group-hover:opacity-60 hover:!opacity-100"
															title="Copy ID"
															onClick={() => copyText(server._id)}
														>
															<Copy size={12} />
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
												<Search size={20} className="text-base-content/20" />
												<span className="text-[13px] text-base-content/60">
													{filter ? "No matching servers" : "No servers found"}
												</span>
												{filter && (
													<span className="text-[11px] text-base-content/35">
														Try a different tag, IP, or country.
													</span>
												)}
											</div>
										</td>
									</tr>
								)}
							</tbody>
						</table>
					</div>
				</div>
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
