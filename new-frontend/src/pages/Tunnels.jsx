import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { Copy, Network, Pencil, Server, Trash2, Zap, ZapOff } from "lucide-react"
import { Page } from "@/components/ui"
import TunnelFormDialog from "@/pages/TunnelFormDialog"
import ServerFormDialog from "@/pages/ServerFormDialog"
import { connect, createTunnel, deleteTunnel, disconnect, fetchServers, fetchState, assignServerToTunnel } from "@/store/actions"
import { countryName } from "@/lib/countries"
import { encTypeName } from "@/lib/format"
import { useStore } from "@/store/store"

const LINK = "var(--color-primary)"
const ACTIVE = "var(--color-success)"

const copyText = (text) => {
	navigator.clipboard?.writeText(text)
	useStore.getState().notifySuccess("Copied to clipboard")
}

const MetaRow = ({ label, value }) => (
	<div className="flex items-baseline gap-3 py-[3px]">
		<span className="w-16 shrink-0 text-[9px] font-semibold uppercase tracking-wider opacity-40">{label}</span>
		<span className="min-w-0 flex-1 truncate font-mono text-[11px] opacity-70">{value}</span>
	</div>
)

const ConnectionLine = ({ line, hovered, onHover }) => {
	const { x1, y1, x2, y2, active } = line
	const midX = (x1 + x2) / 2
	const d = `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`
	const hitbox = (
		<path
			d={d}
			fill="none"
			stroke="transparent"
			strokeWidth="20"
			style={{ pointerEvents: "stroke", cursor: "pointer" }}
			onMouseEnter={() => onHover(line)}
			onMouseLeave={() => onHover(null)}
		/>
	)

	if (!active) {
		return (
			<g>
				<path d={d} fill="none" stroke={LINK} strokeWidth="1.5" strokeDasharray="6 4" strokeOpacity="0.35" />
				{hitbox}
			</g>
		)
	}
	return (
		<g>
			<path d={d} fill="none" stroke={ACTIVE} strokeWidth="6" strokeOpacity={hovered ? 0.2 : 0.1} />
			<path d={d} fill="none" stroke={ACTIVE} strokeWidth={hovered ? 2.5 : 2} strokeOpacity={hovered ? 0.9 : 0.7} />
			<circle r="3" fill={ACTIVE} opacity="0.9">
				<animateMotion dur="3s" repeatCount="indefinite" path={d} />
			</circle>
			<circle r="3" fill={ACTIVE} opacity="0.5">
				<animateMotion dur="3s" repeatCount="indefinite" path={d} begin="1.5s" />
			</circle>
			{hitbox}
		</g>
	)
}

const DragLine = ({ x1, y1, x2, y2 }) => {
	const midX = (x1 + x2) / 2
	return (
		<g>
			<path
				d={`M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`}
				fill="none"
				stroke={LINK}
				strokeWidth="2"
				strokeOpacity="0.5"
				strokeDasharray="8 4"
			/>
			<circle cx={x2} cy={y2} r="4" fill={LINK} opacity="0.6" />
		</g>
	)
}

const NodeActions = ({ children }) => (
	<div className="absolute right-2 top-1.5 z-10 flex gap-0.5 opacity-0 transition-opacity group-hover/node:opacity-100">
		{children}
	</div>
)

const IconBtn = ({ title, onClick, danger, success, children }) => (
	<button
		title={title}
		className={
			"btn btn-square btn-ghost btn-xs " + (danger ? "text-error" : success ? "text-success" : "opacity-60")
		}
		onClick={(e) => {
			e.stopPropagation()
			onClick()
		}}
	>
		{children}
	</button>
)

const TunnelNode = React.forwardRef(function TunnelNode(
	{ tunnel, active, selected, linking, linked, hovered, expanded, onClick, onMouseEnter, onMouseLeave, onEdit, onDelete, onConnect, onDisconnect },
	ref,
) {
	const border = hovered
		? "border-base-content/40"
		: selected
			? "border-warning/60"
			: active
				? "border-success/60"
				: linking
					? "border-base-300 opacity-40"
					: linked
						? "border-primary/30 hover:border-primary/50"
						: "border-base-300 hover:border-base-content/20"
	return (
		<div
			ref={ref}
			onClick={onClick}
			onMouseEnter={onMouseEnter}
			onMouseLeave={onMouseLeave}
			className={"group/node relative cursor-pointer rounded-box border bg-base-100 p-3 shadow-sm transition-all duration-300 " + border}
		>
			<NodeActions>
				{active ? (
					<IconBtn title="Disconnect" danger onClick={() => onDisconnect(active)}>
						<ZapOff size={12} />
					</IconBtn>
				) : (
					<IconBtn title="Connect" success onClick={() => onConnect(tunnel)}>
						<Zap size={12} />
					</IconBtn>
				)}
				<IconBtn title="Copy Tag" onClick={() => copyText(tunnel.Tag)}>
					<Copy size={12} />
				</IconBtn>
				<IconBtn title="Edit" onClick={() => onEdit(tunnel)}>
					<Pencil size={12} />
				</IconBtn>
				<IconBtn title="Delete" danger onClick={() => onDelete(tunnel)}>
					<Trash2 size={12} />
				</IconBtn>
			</NodeActions>

			<div className="mb-1.5 flex items-center gap-2">
				<div className={"h-2 w-2 shrink-0 rounded-full " + (active ? "animate-pulse bg-success" : "bg-base-content/20")} />
				<span className="truncate text-[13px] font-semibold tracking-tight">{tunnel.Tag}</span>
			</div>

			<div className="ml-4 space-y-1">
				<div className="font-mono text-[11px] opacity-70">
					{tunnel.IPv4Address || <span className="font-sans italic opacity-50">no address</span>}
				</div>
				<div className="flex flex-wrap items-center gap-1.5">
					<span className="text-[10px] opacity-70">
						{tunnel.IFName || <span className="italic opacity-50">no interface</span>}
					</span>
					<span className="badge badge-ghost badge-xs font-semibold uppercase tracking-wider">
						{encTypeName(tunnel.EncryptionType)}
					</span>
				</div>
			</div>

			<div
				className={
					"overflow-hidden transition-all duration-300 ease-in-out " +
					(expanded ? "max-h-44 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-44 group-hover/node:opacity-100")
				}
			>
				<div className="ml-4 mt-2.5 border-t border-base-300 pt-2.5">
					{tunnel.ServerID && <MetaRow label="Server ID" value={tunnel.ServerID} />}
					<MetaRow label="IPv6" value={tunnel.IPv6Address || "none"} />
					<MetaRow label="Mask" value={tunnel.NetMask || "none"} />
					<MetaRow label="MTU / TxQ" value={`${tunnel.MTU} / ${tunnel.TxQueueLen}`} />
				</div>
			</div>

			<div
				className={
					"absolute right-0 top-1/2 z-10 h-2.5 w-2.5 -translate-y-1/2 translate-x-1/2 rounded-full border-2 border-base-100 " +
					(hovered ? "bg-base-content" : selected ? "bg-warning" : active ? "bg-success" : linked ? "bg-primary" : "bg-base-300")
				}
			/>
		</div>
	)
})

const ServerNode = React.forwardRef(function ServerNode(
	{ server, hasActive, hasLinked, activeStats, linking, hovered, expanded, onClick, onMouseEnter, onMouseLeave, onConnect, onDisconnect },
	ref,
) {
	const border = hovered
		? "border-base-content/40"
		: linking
			? "cursor-pointer border-success/50 hover:border-success"
			: hasActive
				? "border-success/50"
				: hasLinked
					? "border-primary/30"
					: "border-base-300"
	return (
		<div
			ref={ref}
			onClick={onClick}
			onMouseEnter={onMouseEnter}
			onMouseLeave={onMouseLeave}
			className={"group/node relative rounded-box border bg-base-100 p-3 shadow-sm transition-all duration-300 " + border}
		>
			<NodeActions>
				{hasActive ? (
					<IconBtn title="Disconnect" danger onClick={() => onDisconnect(server)}>
						<ZapOff size={12} />
					</IconBtn>
				) : (
					<IconBtn title="Connect" success onClick={() => onConnect(server)}>
						<Zap size={12} />
					</IconBtn>
				)}
				<IconBtn title="Copy ID" onClick={() => copyText(server._id)}>
					<Copy size={12} />
				</IconBtn>
			</NodeActions>

			<div
				className={
					"absolute left-0 top-1/2 z-10 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-base-100 transition-colors " +
					(hovered ? "bg-base-content" : linking || hasActive ? "bg-success" : hasLinked ? "bg-primary" : "bg-base-300")
				}
			/>

			<div className="mb-1.5 flex items-center gap-2">
				<Server size={14} className="shrink-0 text-primary/70" />
				<span className="truncate text-[13px] font-semibold tracking-tight">{server.Tag}</span>
			</div>

			<div className="ml-[22px] space-y-1">
				<div className="font-mono text-[11px] opacity-70">
					{server.IP}
					<span className="opacity-40">:</span>
					{server.Port}
				</div>
				{server.Country && <div className="text-[10px] opacity-50">{countryName(server.Country)}</div>}
			</div>

			<div
				className={
					"overflow-hidden transition-all duration-300 ease-in-out " +
					(expanded ? "max-h-44 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-44 group-hover/node:opacity-100")
				}
			>
				<div className="ml-[22px] mt-2.5 border-t border-base-300 pt-2.5">
					<MetaRow label="ID" value={server._id} />
					{server.DataPort && <MetaRow label="Data Port" value={server.DataPort} />}
					{server.Groups?.length > 0 && <MetaRow label="Groups" value={server.Groups.join(", ")} />}
				</div>
			</div>

			{activeStats && (
				<div className="ml-[22px] mt-2.5 flex gap-1.5 border-t border-base-300 pt-2.5">
					<span className={"badge badge-xs font-mono " + (activeStats.CPU > 80 ? "badge-warning" : "badge-ghost")}>
						CPU {activeStats.CPU}%
					</span>
					<span className={"badge badge-xs font-mono " + (activeStats.MEM > 80 ? "badge-warning" : "badge-ghost")}>
						MEM {activeStats.MEM}%
					</span>
				</div>
			)}
		</div>
	)
})

const Tunnels = () => {
	const user = useStore((s) => s.user)
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const askConfirm = useStore((s) => s.askConfirm)
	const notifyError = useStore((s) => s.notifyError)

	const containerRef = useRef(null)
	const tunnelRefs = useRef({})
	const serverRefs = useRef({})
	const [lines, setLines] = useState([])
	const [hoveredConn, setHoveredConn] = useState(null) // { tunnelTag, serverId, fromLine }
	const [selectedTunnel, setSelectedTunnel] = useState(null)
	const [mousePos, setMousePos] = useState({ x: 0, y: 0 })
	const [editTunnel, setEditTunnel] = useState(null)
	const [tunnelDialogOpen, setTunnelDialogOpen] = useState(false)
	const [serverDialogOpen, setServerDialogOpen] = useState(false)

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	useEffect(() => {
		const onKeyDown = (e) => e.key === "Escape" && setSelectedTunnel(null)
		window.addEventListener("keydown", onKeyDown)
		return () => window.removeEventListener("keydown", onKeyDown)
	}, [])

	const serverMap = useMemo(() => Object.fromEntries(servers.map((s) => [s._id, s])), [servers])
	const activeMap = useMemo(
		() => Object.fromEntries((activeTunnels || []).map((at) => [at.CR?.Tag, at])),
		[activeTunnels],
	)
	const connections = useMemo(
		() => tunnels.map((t) => ({ tunnel: t, server: serverMap[t.ServerID] || null, active: activeMap[t.Tag] || null })),
		[tunnels, serverMap, activeMap],
	)
	const serverActiveStats = useMemo(() => {
		const map = {}
		activeTunnels?.forEach((at) => at.CR?.ServerID && (map[at.CR.ServerID] = at))
		return map
	}, [activeTunnels])
	const serverHasActive = useMemo(() => {
		const map = {}
		connections.forEach((c) => c.server && c.active && (map[c.server._id] = true))
		return map
	}, [connections])
	const serverHasLinked = useMemo(() => {
		const map = {}
		connections.forEach((c) => c.server && (map[c.server._id] = true))
		return map
	}, [connections])
	const connByTunnel = useMemo(() => {
		const map = {}
		connections.forEach((c) => c.server && (map[c.tunnel.Tag] = c))
		return map
	}, [connections])
	const connsByServer = useMemo(() => {
		const map = {}
		connections.forEach((c) => {
			if (c.server) (map[c.server._id] ||= []).push(c)
		})
		return map
	}, [connections])

	const recalculateLines = useCallback(() => {
		const containerEl = containerRef.current
		if (!containerEl) return
		const cRect = containerEl.getBoundingClientRect()
		setLines(
			connections
				.map((conn) => {
					if (!conn.server) return null
					const tEl = tunnelRefs.current[conn.tunnel.Tag]
					const sEl = serverRefs.current[conn.server._id]
					if (!tEl || !sEl) return null
					const tRect = tEl.getBoundingClientRect()
					const sRect = sEl.getBoundingClientRect()
					return {
						x1: tRect.right - cRect.left,
						y1: tRect.top + tRect.height / 2 - cRect.top,
						x2: sRect.left - cRect.left,
						y2: sRect.top + sRect.height / 2 - cRect.top,
						active: conn.active,
						tunnel: conn.tunnel,
						server: conn.server,
					}
				})
				.filter(Boolean),
		)
	}, [connections])

	useLayoutEffect(() => {
		recalculateLines()
	}, [recalculateLines])

	useEffect(() => {
		const observer = new ResizeObserver(recalculateLines)
		if (containerRef.current) observer.observe(containerRef.current)
		return () => observer.disconnect()
	}, [recalculateLines])

	// keep line endpoints attached while the 300ms hover expand/collapse runs
	useEffect(() => {
		let running = true
		const loop = () => {
			if (!running) return
			recalculateLines()
			requestAnimationFrame(loop)
		}
		requestAnimationFrame(loop)
		const timeout = setTimeout(() => (running = false), 350)
		return () => {
			running = false
			clearTimeout(timeout)
		}
	}, [hoveredConn, recalculateLines])

	const handleServerClick = (server) => {
		if (!selectedTunnel) return
		const tunnel = selectedTunnel
		const wasActive = !!activeMap[tunnel.Tag]
		setSelectedTunnel(null)
		Promise.resolve(assignServerToTunnel(tunnel.Tag, server._id)).then(() => {
			if (wasActive) connect(tunnel, server)
		})
	}

	const handleConnectServer = (server) => {
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

	const handleDisconnectServer = (server) => {
		const active = activeTunnels?.find((x) => x.CR?.ServerID === server._id)
		if (!active) return
		askConfirm("Disconnect", "Disconnect from " + server.Tag + "?", () => disconnect(active))
	}

	const dragStart = (() => {
		if (!selectedTunnel || !containerRef.current) return null
		const tEl = tunnelRefs.current[selectedTunnel.Tag]
		if (!tEl) return null
		const cRect = containerRef.current.getBoundingClientRect()
		const tRect = tEl.getBoundingClientRect()
		return { x: tRect.right - cRect.left, y: tRect.top + tRect.height / 2 - cRect.top }
	})()

	const tunnelCount = tunnels.length
	const serverCount = servers.length
	const containerHeight = Math.max(tunnelCount, serverCount) * 90 + 60

	return (
		<Page>
			{selectedTunnel && (
				<div className="mb-4 inline-flex items-center gap-2 rounded-full bg-warning/10 px-3 py-1 ring-1 ring-inset ring-warning/30">
					<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-warning" />
					<span className="text-[11px]">
						Click a server to assign <span className="font-semibold">{selectedTunnel.Tag}</span>
					</span>
					<span className="text-[10px] tracking-wide opacity-50">ESC to cancel</span>
				</div>
			)}

			{tunnelCount === 0 && serverCount === 0 ? (
				<div className="flex w-full flex-col items-center justify-center rounded-box border border-dashed border-base-300 py-20">
					<Network size={40} className="mb-3 opacity-30" />
					<div className="text-[13px] opacity-70">No tunnels or servers configured</div>
					<div className="mt-1 text-[11px] opacity-40">Add servers and tunnels to see the network graph</div>
				</div>
			) : (
				<div
					ref={containerRef}
					className="relative w-full"
					style={{ minHeight: containerHeight }}
					onMouseMove={(e) => {
						if (!selectedTunnel) return
						const cRect = containerRef.current?.getBoundingClientRect()
						if (cRect) setMousePos({ x: e.clientX - cRect.left, y: e.clientY - cRect.top })
					}}
					onClick={(e) => e.target === e.currentTarget && setSelectedTunnel(null)}
				>
					{/* tunnels — left column */}
					<div className="absolute left-0 top-0 z-[2] w-[260px] space-y-3">
						<div className="mb-2 flex items-center gap-3 px-1">
							<button className="btn btn-primary btn-xs" onClick={createTunnel}>
								Create
							</button>
							<span className="text-[10px] font-semibold uppercase tracking-wider opacity-50">
								Tunnels <span className="font-mono">{tunnelCount}</span>
							</span>
						</div>
						{tunnels.map((tunnel) => {
							const conn = connByTunnel[tunnel.Tag]
							return (
								<TunnelNode
									key={tunnel.Tag}
									ref={(el) => (tunnelRefs.current[tunnel.Tag] = el)}
									tunnel={tunnel}
									active={activeMap[tunnel.Tag]}
									selected={selectedTunnel?.Tag === tunnel.Tag}
									linking={selectedTunnel && selectedTunnel.Tag !== tunnel.Tag}
									linked={!!serverMap[tunnel.ServerID]}
									hovered={hoveredConn?.tunnelTag === tunnel.Tag}
									expanded={hoveredConn?.tunnelTag === tunnel.Tag && !hoveredConn?.fromLine}
									onClick={() => setSelectedTunnel(selectedTunnel?.Tag === tunnel.Tag ? null : tunnel)}
									onMouseEnter={() => conn && setHoveredConn({ tunnelTag: tunnel.Tag, serverId: conn.server._id })}
									onMouseLeave={() => setHoveredConn(null)}
									onEdit={(t) => {
										setEditTunnel(t)
										setTunnelDialogOpen(true)
									}}
									onDelete={(t) => askConfirm("Delete", "Delete tunnel " + t.Tag + "?", () => deleteTunnel(t))}
									onConnect={(t) => askConfirm("Connect", "Connect " + t.Tag + "?", () => connect(t))}
									onDisconnect={(at) => askConfirm("Disconnect", "Disconnect " + at.CR?.Tag + "?", () => disconnect(at))}
								/>
							)
						})}
					</div>

					{/* connection lines */}
					<svg className="absolute inset-0 z-[1] h-full w-full" style={{ pointerEvents: "none" }}>
						{lines.map((line, i) => (
							<ConnectionLine
								key={`${line.tunnel.Tag}-${line.server?._id}-${i}`}
								line={line}
								hovered={hoveredConn?.tunnelTag === line.tunnel.Tag && hoveredConn?.serverId === line.server?._id}
								onHover={(l) =>
									setHoveredConn(l ? { tunnelTag: l.tunnel.Tag, serverId: l.server?._id, fromLine: true } : null)
								}
							/>
						))}
						{dragStart && <DragLine x1={dragStart.x} y1={dragStart.y} x2={mousePos.x} y2={mousePos.y} />}
					</svg>

					{/* servers — right column */}
					<div className="absolute right-0 top-0 z-[2] w-[260px] space-y-3">
						<div className="mb-2 flex items-center gap-3 px-1">
							{(user?.IsAdmin || user?.IsManager) && (
								<button className="btn btn-primary btn-xs" onClick={() => setServerDialogOpen(true)}>
									Create
								</button>
							)}
							<span className="text-[10px] font-semibold uppercase tracking-wider opacity-50">
								Servers <span className="font-mono">{serverCount}</span>
							</span>
						</div>
						{servers.map((server) => (
							<ServerNode
								key={server._id}
								ref={(el) => (serverRefs.current[server._id] = el)}
								server={server}
								hasActive={!!serverHasActive[server._id]}
								hasLinked={!!serverHasLinked[server._id]}
								activeStats={serverActiveStats[server._id]}
								linking={!!selectedTunnel}
								hovered={hoveredConn?.serverId === server._id}
								expanded={hoveredConn?.serverId === server._id && !hoveredConn?.fromLine}
								onClick={() => handleServerClick(server)}
								onMouseEnter={() => {
									const conns = connsByServer[server._id]
									if (conns?.length) setHoveredConn({ tunnelTag: conns[0].tunnel.Tag, serverId: server._id })
								}}
								onMouseLeave={() => setHoveredConn(null)}
								onConnect={handleConnectServer}
								onDisconnect={handleDisconnectServer}
							/>
						))}
					</div>
				</div>
			)}

			<TunnelFormDialog
				open={tunnelDialogOpen}
				onClose={() => setTunnelDialogOpen(false)}
				tunnel={editTunnel}
				servers={servers}
			/>
			<ServerFormDialog open={serverDialogOpen} onClose={() => setServerDialogOpen(false)} />
		</Page>
	)
}

export default Tunnels
