import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Plus, Trash2, Users } from "lucide-react"
import { Card, Page, Toggle } from "@/components/ui"
import { fetchState, setTunnelPeers } from "@/store/actions"
import { useStore } from "@/store/store"

// Loose shape check for instant feedback; the client backend does the real
// validation and rejects anything that does not parse as an IP address.
const looksLikeIP = (s) => /^[0-9a-fA-F:.]+$/.test(s) && (s.includes(".") || s.includes(":"))

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

	// apply preserves whichever half of the policy isn't being changed.
	const apply = async (next, all = allowAll) => {
		setBusy(true)
		const ok = await setTunnelPeers(tag, next, all)
		setBusy(false)
		return ok
	}

	const addPeer = async () => {
		const ip = newPeer.trim()
		if (!ip) return
		if (!looksLikeIP(ip)) {
			notifyError("Peer must be an IP address")
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
				<Users size={16} className="opacity-60" />
				<h1 className="text-base font-semibold tracking-tight">{tag}</h1>
				{connected ? (
					<span className="badge badge-success badge-sm">connected</span>
				) : (
					<span className="badge badge-ghost badge-sm">disconnected</span>
				)}
			</div>

			<div className="max-w-xl">
				<Card
					title={`Peers (${peers.length})`}
					description="Devices allowed to reach this device through the VPN. When the server firewall is
						enabled nobody can reach this device unless their VPN IP is listed here. Changes are saved
						immediately and announced to the server while connected."
				>
					<div className="mb-3 rounded-box border border-warning/30 bg-warning/5 px-3 py-1">
						<Toggle
							label="Allow all peers (disables the firewall for this device)"
							checked={allowAll}
							disabled={busy}
							onChange={() => apply(peers, !allowAll)}
						/>
					</div>

					<div className={"mb-3 flex items-center gap-2 " + (allowAll ? "opacity-40" : "")}>
						<input
							className="input input-sm flex-1 font-mono"
							placeholder="10.42.0.7"
							value={newPeer}
							disabled={busy || allowAll}
							onChange={(e) => setNewPeer(e.target.value)}
							onKeyDown={(e) => e.key === "Enter" && addPeer()}
						/>
						<button className="btn btn-primary btn-sm" disabled={busy || allowAll || !newPeer.trim()} onClick={addPeer}>
							<Plus size={14} /> Add
						</button>
					</div>

					{allowAll ? (
						<div className="rounded-box border border-dashed border-warning/40 py-8 text-center">
							<p className="text-[13px] opacity-70">All peers allowed</p>
							<p className="mt-1 text-[11px] opacity-40">
								Any device on the VPN can reach this device. The allowlist below is ignored until you turn this off.
							</p>
						</div>
					) : peers.length > 0 ? (
						<ul className="flex flex-col">
							{peers.map((ip) => (
								<li
									key={ip}
									className="flex items-center justify-between gap-3 border-b border-base-200 py-1.5 last:border-0"
								>
									<code className="font-mono text-sm">{ip}</code>
									<button
										className="btn btn-square btn-ghost btn-xs text-error"
										title="Remove peer"
										disabled={busy}
										onClick={() => removePeer(ip)}
									>
										<Trash2 size={13} />
									</button>
								</li>
							))}
						</ul>
					) : (
						<div className="rounded-box border border-dashed border-base-300 py-8 text-center">
							<p className="text-[13px] opacity-70">No peers allowed</p>
							<p className="mt-1 text-[11px] opacity-40">
								Other devices cannot reach this device while the server firewall is enabled.
							</p>
						</div>
					)}
				</Card>
			</div>
		</Page>
	)
}

export default TunnelPeers
