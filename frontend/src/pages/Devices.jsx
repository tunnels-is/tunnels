import { useEffect, useMemo, useState } from "react"
import QRCode from "react-qr-code"
import { Copy, Monitor, Plus, Search, Trash2 } from "lucide-react"
import Dialog from "@/components/Dialog"
import { Field, Page, TextField, Toolbar } from "@/components/ui"
import { api, controller, fetchDevices, fetchServers, fetchState } from "@/store/actions"
import { fullDate } from "@/lib/format"
import { countryName } from "@/lib/countries"
import { useStore } from "@/store/store"

const Devices = () => {
	const user = useStore((s) => s.user)
	const devices = useStore((s) => s.devices)
	const localDevices = useStore((s) => s.localDevices)
	const setDevices = useStore((s) => s.setDevices)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const askConfirm = useStore((s) => s.askConfirm)
	const notifyError = useStore((s) => s.notifyError)
	const notifySuccess = useStore((s) => s.notifySuccess)

	const [showCreate, setShowCreate] = useState(false)
	const [tag, setTag] = useState("")
	const [serverID, setServerID] = useState("")
	const [wgConfig, setWgConfig] = useState(null)
	const [submitting, setSubmitting] = useState(false)
	const [filter, setFilter] = useState("")

	// Reload devices whenever the active account changes (not only on first mount).
	useEffect(() => {
		fetchState()
	}, [])

	useEffect(() => {
		if (!user?._id) {
			setDevices([])
			return
		}
		fetchDevices({ force: true })
	}, [user?._id, setDevices])

	const openCreate = async () => {
		setTag("")
		setWgConfig(null)
		setShowCreate(true)
		const list = await fetchServers({ force: true })
		if (list?.length > 0) setServerID(list[0]._id)
	}

	const createDevice = async () => {
		if (!tag.trim()) return notifyError("Please enter a device tag")
		if (!serverID) return notifyError("Please select a server")
		setSubmitting(true)
		const resp = await api("createDeviceWithKeys", {
			Server: user?.ControlServer,
			Tag: tag.trim(),
			ServerID: serverID,
			DeviceToken: user?.DeviceToken?.DT || "",
			UID: user?._id || "",
		})
		setSubmitting(false)
		if (resp.status === 200 && resp.data?.WGConfig) setWgConfig(resp.data.WGConfig)
	}

	const downloadConfig = () => {
		const url = URL.createObjectURL(new Blob([wgConfig], { type: "text/plain" }))
		const a = document.createElement("a")
		a.href = url
		a.download = `${tag.trim() || "device"}.conf`
		a.click()
		URL.revokeObjectURL(url)
	}

	const closeCreate = () => {
		setShowCreate(false)
		setWgConfig(null)
		setTag("")
		setServerID("")
		fetchDevices({ force: true })
	}

	const deleteDevice = (device) => {
		askConfirm("Delete Device", `Delete "${device.Tag}"? This cannot be undone.`, async () => {
			const resp = await controller("/client/device/delete", { DeviceID: device._id })
			if (resp.status === 200) setDevices(devices.filter((d) => d._id !== device._id))
		})
	}

	const copyText = (text) => {
		navigator.clipboard?.writeText(text)
		notifySuccess("Copied to clipboard")
	}

	// Connected: controller device WG IP matches a live tunnel.
	const connectedIPs = useMemo(() => {
		const ips = new Set()
		for (const at of activeTunnels || []) {
			const ip = at?.CRResponse?.WireGuardIP || at?.ServerResponse?.WireGuardIP
			if (ip) ips.add(ip)
		}
		return ips
	}, [activeTunnels])

	// Local: this machine has a devices/ file (match by controller id or pubkey).
	const localIDs = useMemo(() => new Set((localDevices || []).map((d) => d.ID).filter(Boolean)), [localDevices])
	const localPubs = useMemo(
		() => new Set((localDevices || []).map((d) => d.WireGuardPubKey).filter(Boolean)),
		[localDevices],
	)
	const isLocalDevice = (d) =>
		!!(d && ((d._id && localIDs.has(d._id)) || (d.WireGuardKey && localPubs.has(d.WireGuardKey))))

	const filtered = useMemo(() => {
		const f = filter.toLowerCase()
		const list = !filter
			? [...devices]
			: devices.filter(
					(d) =>
						d.Tag?.toLowerCase().includes(f) ||
						d.WireGuardIP?.toLowerCase().includes(f),
				)
		list.sort((a, b) => {
			const aConn = a.WireGuardIP && connectedIPs.has(a.WireGuardIP) ? 1 : 0
			const bConn = b.WireGuardIP && connectedIPs.has(b.WireGuardIP) ? 1 : 0
			if (aConn !== bConn) return bConn - aConn
			const aLoc = isLocalDevice(a) ? 1 : 0
			const bLoc = isLocalDevice(b) ? 1 : 0
			if (aLoc !== bLoc) return bLoc - aLoc
			return (a.Tag || "").localeCompare(b.Tag || "")
		})
		return list
	}, [devices, filter, connectedIPs, localIDs, localPubs])

	const connectedCount = useMemo(
		() => devices.filter((d) => d.WireGuardIP && connectedIPs.has(d.WireGuardIP)).length,
		[devices, connectedIPs],
	)
	const localCount = useMemo(() => devices.filter((d) => isLocalDevice(d)).length, [devices, localIDs, localPubs])

	return (
		<Page>
			<Toolbar>
				<div className="flex items-baseline gap-2">
					<span className="text-sm font-semibold tracking-tight">Devices</span>
					<span className="text-[11px] opacity-40">
						{filtered.length}
						{filter && devices.length !== filtered.length && <span> of {devices.length}</span>}
						{localCount > 0 && <span className="text-info"> · {localCount} on this device</span>}
						{connectedCount > 0 && (
							<span className="text-success"> · {connectedCount} connected</span>
						)}
					</span>
				</div>
				<div className="ml-auto flex items-center gap-1.5">
					<label className="input input-xs flex w-48 items-center gap-1">
						<Search size={12} className="opacity-40" />
						<input
							placeholder="Filter by tag or IP..."
							value={filter}
							onChange={(e) => setFilter(e.target.value)}
						/>
					</label>
					<button className="btn btn-primary btn-xs gap-1" onClick={openCreate}>
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
									Device
								</th>
								<th className="px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45">
									WireGuard IP
								</th>
								<th className="hidden px-3 py-3 text-[10px] font-semibold uppercase tracking-wider text-base-content/45 md:table-cell">
									Created
								</th>
								<th className="w-28 px-4 py-3" />
							</tr>
						</thead>
						<tbody className="divide-y divide-base-200">
							{filtered.length > 0 ? (
								filtered.map((d) => {
									const isConnected = !!(d.WireGuardIP && connectedIPs.has(d.WireGuardIP))
									const isLocal = isLocalDevice(d)
									return (
										<tr
											key={d._id}
											className={
												"transition-colors duration-150 " +
												(isConnected
													? "bg-success/[0.04] hover:bg-success/[0.07]"
													: isLocal
														? "bg-info/[0.04] hover:bg-info/[0.07]"
														: "hover:bg-base-200/40")
											}
										>
											<td className="px-4 py-3.5">
												<div className="flex items-center justify-center">
													<span
														className={
															"block h-2 w-2 rounded-full ring-2 " +
															(isConnected
																? "animate-pulse bg-success ring-success/25"
																: isLocal
																	? "bg-info ring-info/25"
																	: "bg-base-content/15 ring-transparent")
														}
														title={isConnected ? "Connected" : isLocal ? "On this device" : "Other machine"}
													/>
												</div>
											</td>
											<td className="px-3 py-3.5">
												<div className="flex min-w-0 items-center gap-2.5">
													<div
														className={
															"grid h-8 w-8 shrink-0 place-items-center rounded-lg " +
															(isConnected
																? "bg-success/10 text-success"
																: isLocal
																	? "bg-info/10 text-info"
																	: "bg-base-200 text-base-content/40")
														}
													>
														<Monitor size={14} />
													</div>
													<div className="min-w-0">
														<div className="truncate text-[13px] font-semibold tracking-tight">
															{d.Tag}
														</div>
														<div className="flex flex-wrap gap-1.5">
															{isLocal && (
																<span className="text-[10px] font-medium text-info">This device</span>
															)}
															{isConnected && (
																<span className="text-[10px] font-medium text-success">Connected</span>
															)}
															{!isLocal && !isConnected && (
																<span className="text-[10px] font-medium text-base-content/35">Other machine</span>
															)}
														</div>
													</div>
												</div>
											</td>
											<td className="px-3 py-3.5">
												{d.WireGuardIP ? (
													<button
														className="inline-flex max-w-full items-center gap-1.5 rounded-md bg-base-200/70 px-2 py-1 font-mono text-[11px] text-base-content/70 transition-colors hover:bg-base-200 hover:text-base-content"
														title="Copy WireGuard IP"
														onClick={() => copyText(d.WireGuardIP)}
													>
														<span className="truncate">{d.WireGuardIP}</span>
														<Copy size={10} className="shrink-0 opacity-40" />
													</button>
												) : (
													<span className="text-xs text-base-content/30">—</span>
												)}
											</td>
											<td className="hidden px-3 py-3.5 md:table-cell">
												<span className="text-xs text-base-content/50">
													{d.CreatedAt ? fullDate(d.CreatedAt) : "—"}
												</span>
											</td>
											<td className="px-4 py-3.5">
												<div className="flex items-center justify-end">
													<button
														className="btn btn-ghost btn-xs gap-1 text-error/70 hover:bg-error/10 hover:text-error"
														title="Delete device"
														onClick={() => deleteDevice(d)}
													>
														<Trash2 size={12} /> Delete
													</button>
												</div>
											</td>
										</tr>
									)
								})
							) : (
								<tr>
									<td colSpan={5} className="px-4 py-16 text-center">
										<div className="flex flex-col items-center gap-2">
											{filter ? (
												<>
													<Search size={20} className="text-base-content/20" />
													<span className="text-[13px] text-base-content/60">No matching devices</span>
													<span className="text-[11px] text-base-content/35">
														Try a different tag or IP.
													</span>
												</>
											) : (
												<>
													<Monitor size={20} className="text-base-content/20" />
													<span className="text-[13px] text-base-content/60">No devices found</span>
													<span className="text-[11px] text-base-content/35">
														Create a device to get a WireGuard config
													</span>
													<button className="btn btn-primary btn-xs mt-2 gap-1" onClick={openCreate}>
														<Plus size={12} /> Create device
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

			<Dialog open={showCreate} onClose={closeCreate} title={wgConfig ? "Device Config" : "New Device"}>
				{!wgConfig ? (
					<div className="space-y-2">
						<TextField
							label="Tag"
							placeholder="e.g. my-laptop"
							value={tag}
							onChange={(e) => setTag(e.target.value)}
						/>
						<Field label="Server">
							<select
								className="select select-sm w-full"
								value={serverID}
								onChange={(e) => setServerID(e.target.value)}
							>
								{servers.length === 0 && <option value="">No servers available</option>}
								{servers.map((s) => (
									<option key={s._id} value={s._id}>
										{s.Tag} ({countryName(s.Country)})
									</option>
								))}
							</select>
						</Field>
						<button
							className="btn btn-primary btn-block btn-sm mt-2"
							disabled={submitting}
							onClick={createDevice}
						>
							{submitting ? "Creating..." : "Create Device"}
						</button>
					</div>
				) : (
					<div>
						<div className="alert alert-warning mb-3 text-xs">
							Save this config — it cannot be shown again
						</div>
						<div className="mx-auto mb-3 w-fit rounded-box bg-white p-4">
							<QRCode value={wgConfig} style={{ height: "auto", width: "188px" }} viewBox="0 0 256 256" />
						</div>
						<div className="flex gap-2">
							<button className="btn btn-primary btn-sm flex-1" onClick={downloadConfig}>
								Download .conf
							</button>
							<button className="btn btn-outline btn-sm flex-1" onClick={closeCreate}>
								Done
							</button>
						</div>
					</div>
				)}
			</Dialog>
		</Page>
	)
}

export default Devices
