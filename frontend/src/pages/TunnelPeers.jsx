import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Globe, Plus, Shield, ShieldCheck, ShieldOff, Trash2 } from "lucide-react"
import { Card, Page } from "@/components/ui"
import { fetchState, setTunnelPeers } from "@/store/actions"
import { useStore } from "@/store/store"

const looksLikePeer = (s) =>
	/^\*:\d{1,5}$/.test(s) ||
	(/^[0-9a-fA-F:.[\]]+$/.test(s) && (s.includes(".") || s.includes(":")))

const parsePeer = (s) => {
	let m = s.match(/^\*:(\d{1,5})$/)
	if (m) return { host: "Any device", port: m[1], any: true }
	m = s.match(/^\[(.+)\]:(\d{1,5})$/)
	if (m) return { host: m[1], port: m[2] }
	m = s.match(/^([0-9.]+):(\d{1,5})$/)
	if (m) return { host: m[1], port: m[2] }
	return { host: s, port: null }
}

const TunnelPeers = () => {
	const { tag } = useParams()
	const navigate = useNavigate()
	const tunnels = useStore((s) => s.tunnels)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const notifyError = useStore((s) => s.notifyError)

	const [newPeer, setNewPeer] = useState("")
	const [busy, setBusy] = useState(false)

	useEffect(() => {
		fetchState()
	}, [])

	const tunnel = tunnels.find((t) => t.Tag === tag)
	const connected = (activeTunnels || []).some((at) => at.CR?.Tag === tag)
	const peers = tunnel?.AllowedHosts || []
	const allowAll = !!tunnel?.AllowAll

	if (!tunnel) {
		return (
			<Page>
				<div className="flex h-40 items-center justify-center text-[13px] opacity-50">
					{tunnels.length === 0 ? "Loading..." : `Tunnel "${tag}" not found.`}
				</div>
			</Page>
		)
	}

	const apply = async (next, all = allowAll) => {
		setBusy(true)
		const ok = await setTunnelPeers(tag, next, all)
		setBusy(false)
		return ok
	}

	const addPeer = async () => {
		const ip = newPeer.trim()
		if (!ip) return
		if (!looksLikePeer(ip)) {
			notifyError("Peer must be an IP, IP:PORT, or *:PORT")
			return
		}
		if (peers.includes(ip)) {
			notifyError("Peer is already in the list")
			return
		}
		if (await apply([...peers, ip])) setNewPeer("")
	}

	const removePeer = (ip) => apply(peers.filter((p) => p !== ip))

	return (
		<Page>
			<div className="mb-4 flex items-center gap-3">
				<button className="btn btn-square btn-ghost btn-sm" onClick={() => navigate("/tunnels")}>
					<ArrowLeft size={16} />
				</button>
				<Shield size={16} className="opacity-60" />
				<h1 className="text-base font-semibold tracking-tight">{tag}</h1>
				<span className="text-[11px] opacity-40">firewall</span>
				<span className="flex-1" />
				{connected ? (
					<span className="badge badge-success badge-sm gap-1">
						<span className="inline-block h-1.5 w-1.5 rounded-full bg-current" /> connected
					</span>
				) : (
					<span className="badge badge-ghost badge-sm">disconnected</span>
				)}
			</div>

			<div className="max-w-xl space-y-4">
				<Card title="Firewall">
					{}
					<div
						className={
							"flex items-start gap-3 rounded-box border px-4 py-3 " +
							(allowAll ? "border-warning/40 bg-warning/10" : "border-success/30 bg-success/10")
						}
					>
						{allowAll ? (
							<ShieldOff size={20} className="mt-0.5 shrink-0 text-warning" />
						) : (
							<ShieldCheck size={20} className="mt-0.5 shrink-0 text-success" />
						)}
						<div className="flex-1">
							<p className="text-sm font-medium">
								{allowAll ? "Firewall disabled for this device" : "Enforcing allowlist"}
							</p>
							<p className="mt-0.5 text-[11px] leading-relaxed opacity-60">
								{allowAll
									? "Any device on the VPN can reach this device."
									: peers.length === 0
										? "No devices can reach this device while the server firewall is on."
										: `${peers.length} peer${peers.length === 1 ? "" : "s"} allowed to reach this device.`}
							</p>
						</div>
						<label className="flex cursor-pointer items-center gap-2 pt-0.5">
							<span className="text-[11px] opacity-60">Allow all</span>
							<input
								type="checkbox"
								className="toggle toggle-warning toggle-sm"
								checked={allowAll}
								disabled={busy}
								onChange={() => apply(peers, !allowAll)}
							/>
						</label>
					</div>
					<p className="mt-3 text-[11px] leading-relaxed opacity-50">
						When the server firewall is enabled, nobody can reach this device unless their VPN IP is listed
						below. Use <code className="font-mono">IP</code> for all ports, <code className="font-mono">IP:PORT</code>{" "}
						for one port, or <code className="font-mono">*:PORT</code> for any device on that port. Changes save
						immediately and are announced to the server while connected.
					</p>
				</Card>

				<Card title={`Allowlist${allowAll ? "" : ` · ${peers.length}`}`} className={allowAll ? "opacity-60" : ""}>
					<div className="mb-3 flex items-center gap-2">
						<label className="input input-sm flex flex-1 items-center gap-2 font-mono">
							<Plus size={13} className="opacity-40" />
							<input
								className="grow"
								placeholder="10.42.0.7   ·   10.42.0.7:22   ·   *:443"
								value={newPeer}
								disabled={busy || allowAll}
								onChange={(e) => setNewPeer(e.target.value)}
								onKeyDown={(e) => e.key === "Enter" && addPeer()}
							/>
						</label>
						<button className="btn btn-primary btn-sm" disabled={busy || allowAll || !newPeer.trim()} onClick={addPeer}>
							Add peer
						</button>
					</div>

					{allowAll ? (
						<div className="flex flex-col items-center gap-1 rounded-box border border-dashed border-warning/40 py-8 text-center">
							<ShieldOff size={22} className="text-warning/60" />
							<p className="text-[13px] opacity-70">Allowlist ignored</p>
							<p className="max-w-xs text-[11px] opacity-40">
								Turn off “Allow all” above to enforce the list below.
							</p>
						</div>
					) : peers.length > 0 ? (
						<ul className="flex flex-col gap-1">
							{peers.map((ip) => {
								const p = parsePeer(ip)
								return (
									<li
										key={ip}
										className="group flex items-center gap-3 rounded-box border border-base-200 bg-base-100 px-3 py-2 transition-colors hover:border-base-300 hover:bg-base-200"
									>
										{p.any ? (
											<Globe size={15} className="shrink-0 text-warning/70" />
										) : (
											<ShieldCheck size={15} className="shrink-0 text-success/70" />
										)}
										<code className={"flex-1 truncate font-mono text-sm " + (p.any ? "italic opacity-70" : "")}>
											{p.host}
										</code>
										<span className={"badge badge-sm " + (p.port ? "badge-ghost font-mono" : "badge-outline opacity-50")}>
											{p.port ? ":" + p.port : "all ports"}
										</span>
										<button
											className="btn btn-square btn-ghost btn-xs text-error opacity-0 transition-opacity group-hover:opacity-100"
											title="Remove peer"
											disabled={busy}
											onClick={() => removePeer(ip)}
										>
											<Trash2 size={13} />
										</button>
									</li>
								)
							})}
						</ul>
					) : (
						<div className="flex flex-col items-center gap-1 rounded-box border border-dashed border-base-300 py-8 text-center">
							<Shield size={22} className="opacity-30" />
							<p className="text-[13px] opacity-70">No peers allowed yet</p>
							<p className="max-w-xs text-[11px] opacity-40">
								Add a VPN IP above to let that device reach this one.
							</p>
						</div>
					)}
				</Card>
			</div>
		</Page>
	)
}

export default TunnelPeers
