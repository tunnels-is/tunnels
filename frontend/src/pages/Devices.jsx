import { useEffect, useState } from "react"
import QRCode from "react-qr-code"
import { Monitor, Network, Trash2 } from "lucide-react"
import Dialog from "@/components/Dialog"
import { Card, Field, Page, TextField } from "@/components/ui"
import { api, controller } from "@/store/actions"
import { fullDate } from "@/lib/format"
import { countryName } from "@/lib/countries"
import { useStore } from "@/store/store"

const Devices = () => {
	const user = useStore((s) => s.user)
	const tunnels = useStore((s) => s.tunnels)
	const askConfirm = useStore((s) => s.askConfirm)
	const notifyError = useStore((s) => s.notifyError)

	const [devices, setDevices] = useState([])
	const [servers, setServers] = useState([])
	const [showCreate, setShowCreate] = useState(false)
	const [tag, setTag] = useState("")
	const [serverID, setServerID] = useState("")
	const [wgConfig, setWgConfig] = useState(null)
	const [submitting, setSubmitting] = useState(false)

	const loadDevices = async () => {
		const resp = await controller("/client/device/list/user", {})
		if (resp.status === 200 && Array.isArray(resp.data)) setDevices(resp.data)
	}

	useEffect(() => {
		loadDevices()
	}, [])

	const openCreate = async () => {
		setTag("")
		setWgConfig(null)
		setShowCreate(true)
		const resp = await controller("/client/servers", { StartIndex: 0 })
		if (resp.status === 200 && Array.isArray(resp.data)) {
			setServers(resp.data)
			if (resp.data.length > 0) setServerID(resp.data[0]._id)
		}
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
		loadDevices()
	}

	const deleteDevice = (device) => {
		askConfirm("Delete Device", `Delete "${device.Tag}"? This cannot be undone.`, async () => {
			const resp = await controller("/client/device/delete", { DeviceID: device._id })
			if (resp.status === 200) setDevices((prev) => prev.filter((d) => d._id !== device._id))
		})
	}

	const localIPs = new Set(tunnels.map((t) => t.IPv4Address).filter(Boolean))

	return (
		<Page>
			<div className="mb-4">
				<button className="btn btn-primary btn-sm" onClick={openCreate}>
					Create
				</button>
			</div>

			{devices.length > 0 ? (
				<div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))" }}>
					{devices.map((d) => {
						const isCurrent = d.WireGuardIP && localIPs.has(d.WireGuardIP)
						return (
							<div
								key={d._id}
								className={
									"relative flex flex-col gap-3 rounded-box border bg-base-100 p-4 transition-colors " +
									(isCurrent ? "border-success/40" : "border-base-300 hover:border-primary/40")
								}
							>
								<button
									className="btn btn-square btn-ghost btn-xs absolute right-2 top-2 text-error"
									onClick={() => deleteDevice(d)}
								>
									<Trash2 size={14} />
								</button>
								<div className="flex min-w-0 items-center gap-2 pr-6">
									<Monitor size={16} className={"shrink-0 " + (isCurrent ? "text-success" : "text-primary/60")} />
									<span className="min-w-0 flex-1 truncate text-[13px] font-medium">{d.Tag}</span>
									{isCurrent && <span className="badge badge-success badge-xs shrink-0">this device</span>}
								</div>
								<div className="flex min-w-0 items-center gap-1.5">
									<Network size={12} className="shrink-0 opacity-30" />
									<span className="truncate font-mono text-xs opacity-70">{d.WireGuardIP || "—"}</span>
								</div>
								<div className="text-[11px] opacity-50">{d.CreatedAt ? fullDate(d.CreatedAt) : "—"}</div>
							</div>
						)
					})}
				</div>
			) : (
				<Card>
					<div className="py-6 text-center text-xs opacity-50">No devices found</div>
				</Card>
			)}

			<Dialog open={showCreate} onClose={closeCreate} title={wgConfig ? "Device Config" : "New Device"}>
				{!wgConfig ? (
					<div className="space-y-2">
						<TextField label="Tag" placeholder="e.g. my-laptop" value={tag} onChange={(e) => setTag(e.target.value)} />
						<Field label="Server">
							<select className="select select-sm w-full" value={serverID} onChange={(e) => setServerID(e.target.value)}>
								{servers.length === 0 && <option value="">No servers available</option>}
								{servers.map((s) => (
									<option key={s._id} value={s._id}>
										{s.Tag} ({countryName(s.Country)})
									</option>
								))}
							</select>
						</Field>
						<button className="btn btn-primary btn-block btn-sm mt-2" disabled={submitting} onClick={createDevice}>
							{submitting ? "Creating..." : "Create Device"}
						</button>
					</div>
				) : (
					<div>
						<div className="alert alert-warning mb-3 text-xs">Save this config — it cannot be shown again</div>
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
