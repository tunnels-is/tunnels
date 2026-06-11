import { useEffect, useState } from "react"
import { ChevronDown, ChevronRight, Minus, Plus } from "lucide-react"
import Dialog from "@/components/Dialog"
import { Field, TextField } from "@/components/ui"
import { saveTunnel } from "@/store/actions"
import { ENC_TYPES } from "@/lib/format"

const FEATURE_TOGGLES = [
	{ key: "DNSBlocking", label: "DNS Blocking" },
	{ key: "LocalhostNat", label: "Localhost NAT" },
	{ key: "AutoReconnect", label: "Auto Reconnect" },
	{ key: "AutoConnect", label: "Auto Connect" },
	{ key: "KillSwitch", label: "Kill Switch", warn: true },
	{ key: "EnableDefaultRoute", label: "Default Route" },
]

const Section = ({ title, children, defaultOpen = true }) => {
	const [open, setOpen] = useState(defaultOpen)
	return (
		<div className="pt-4">
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider opacity-60 transition-opacity hover:opacity-100"
			>
				{open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
				{title}
			</button>
			{open && children}
		</div>
	)
}

const RemoveBtn = ({ onClick }) => (
	<button type="button" className="btn btn-square btn-ghost btn-xs shrink-0 text-error" onClick={onClick}>
		<Minus size={14} />
	</button>
)

const AddBtn = ({ label, onClick }) => (
	<button type="button" className="btn btn-ghost btn-xs text-success" onClick={onClick}>
		<Plus size={12} /> {label}
	</button>
)

const StringArrayField = ({ items, onChange }) => (
	<div className="space-y-1">
		{items.map((item, i) => (
			<div key={i} className="flex items-center gap-1">
				<input
					className="input input-sm flex-1"
					value={item}
					onChange={(e) => onChange(items.map((v, x) => (x === i ? e.target.value : v)))}
				/>
				<RemoveBtn onClick={() => onChange(items.filter((_, x) => x !== i))} />
			</div>
		))}
		<AddBtn label="Add" onClick={() => onChange([...items, ""])} />
	</div>
)

const DNSRecordEditor = ({ record, onChange, onRemove }) => {
	const set = (key, val) => onChange({ ...record, [key]: val })
	const arrayEditor = (key) => (
		<div>
			<span className="text-[10px] font-semibold uppercase tracking-wider opacity-50">{key === "IP" ? "IPs" : "TXT"}</span>
			<div className="mt-0.5 space-y-1">
				{(record[key] || []).map((v, i) => (
					<div key={i} className="flex items-center gap-1">
						<input
							className="input input-xs flex-1"
							value={v}
							onChange={(e) => set(key, record[key].map((x, idx) => (idx === i ? e.target.value : x)))}
						/>
						<RemoveBtn onClick={() => set(key, record[key].filter((_, idx) => idx !== i))} />
					</div>
				))}
				<AddBtn label={key} onClick={() => set(key, [...(record[key] || []), ""])} />
			</div>
		</div>
	)
	return (
		<div className="mt-2 space-y-2 border-l-2 border-primary/20 py-2 pl-3">
			<div className="flex items-center gap-2">
				<input
					className="input input-sm flex-1"
					placeholder="Domain"
					value={record.Domain || ""}
					onChange={(e) => set("Domain", e.target.value)}
				/>
				<button
					type="button"
					title="Wildcard"
					className={"btn btn-xs " + (record.Wildcard ? "btn-primary" : "btn-ghost")}
					onClick={() => set("Wildcard", !record.Wildcard)}
				>
					*
				</button>
				<RemoveBtn onClick={onRemove} />
			</div>
			{arrayEditor("IP")}
			{arrayEditor("TXT")}
		</div>
	)
}

const TunnelFormDialog = ({ open, onClose, tunnel, servers }) => {
	const [form, setForm] = useState(null)
	const [originalTag, setOriginalTag] = useState("")
	const [saving, setSaving] = useState(false)

	useEffect(() => {
		if (!open || !tunnel) {
			setForm(null)
			setOriginalTag("")
			return
		}
		const clone = structuredClone(tunnel)
		clone.DNSRecords ||= []
		clone.Networks ||= []
		clone.Routes ||= []
		clone.AllowedHosts ||= []
		clone.DNSServers ||= []
		setForm(clone)
		setOriginalTag(tunnel.Tag)
	}, [open, tunnel])

	if (!open || !form) return null

	const set = (key, val) => setForm((f) => ({ ...f, [key]: val }))
	const setItem = (key, i, val) => set(key, form[key].map((v, x) => (x === i ? val : v)))
	const removeItem = (key, i) => set(key, form[key].filter((_, x) => x !== i))
	const addItem = (key, val) => set(key, [...form[key], val])

	const save = async () => {
		setSaving(true)
		const ok = await saveTunnel(form, originalTag || form.Tag)
		setSaving(false)
		if (ok) onClose()
	}

	return (
		<Dialog
			open={open}
			onClose={onClose}
			wide
			title={originalTag ? `Edit Tunnel: ${originalTag}` : "New Tunnel"}
			actions={
				<>
					<button className="btn btn-ghost btn-sm" onClick={onClose}>
						Cancel
					</button>
					<button className="btn btn-primary btn-sm" disabled={saving} onClick={save}>
						{saving ? "Saving..." : "Save"}
					</button>
				</>
			}
		>
			<div className="max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
				<Section title="Settings">
					<div className="grid grid-cols-2 gap-x-3">
						<TextField label="Tag" value={form.Tag || ""} onChange={(e) => set("Tag", e.target.value)} />
						<TextField label="Interface" value={form.IFName || ""} onChange={(e) => set("IFName", e.target.value)} />
						<Field label="Server">
							<select
								className="select select-sm w-full"
								value={form.ServerID || ""}
								onChange={(e) => set("ServerID", e.target.value)}
							>
								<option value="">None</option>
								{servers?.map((s) => (
									<option key={s._id} value={s._id}>
										{s.Tag} ({s.IP})
									</option>
								))}
							</select>
						</Field>
						<Field label="Encryption">
							<select
								className="select select-sm w-full"
								value={form.EncryptionType ?? 0}
								onChange={(e) => set("EncryptionType", Number(e.target.value))}
							>
								{ENC_TYPES.map((label, i) => (
									<option key={i} value={i}>
										{label}
									</option>
								))}
							</select>
						</Field>
						<TextField
							label="MTU"
							type="number"
							value={form.MTU ?? 1420}
							onChange={(e) => set("MTU", Number(e.target.value))}
						/>
						<TextField
							label="TX Queue Length"
							type="number"
							value={form.TxQueueLen ?? 2000}
							onChange={(e) => set("TxQueueLen", Number(e.target.value))}
						/>
					</div>
				</Section>

				<Section title="Features">
					<div className="flex flex-wrap gap-1.5">
						{FEATURE_TOGGLES.map((opt) => (
							<button
								key={opt.key}
								type="button"
								className={"btn btn-xs " + (form[opt.key] ? (opt.warn ? "btn-warning" : "btn-primary") : "btn-ghost")}
								onClick={() => set(opt.key, !form[opt.key])}
							>
								{opt.label}
							</button>
						))}
					</div>
				</Section>

				<Section title={`DNS Servers (${form.DNSServers.length})`} defaultOpen={false}>
					<StringArrayField items={form.DNSServers} onChange={(v) => set("DNSServers", v)} />
				</Section>

				<Section title={`Firewall: Allowed Hosts (${form.AllowedHosts.length})`} defaultOpen={false}>
					<StringArrayField items={form.AllowedHosts} onChange={(v) => set("AllowedHosts", v)} />
				</Section>

				<Section title={`DNS Records (${form.DNSRecords.length})`} defaultOpen={false}>
					{form.DNSRecords.map((rec, i) => (
						<DNSRecordEditor
							key={i}
							record={rec}
							onChange={(val) => setItem("DNSRecords", i, val)}
							onRemove={() => removeItem("DNSRecords", i)}
						/>
					))}
					<AddBtn
						label="Add DNS Record"
						onClick={() => addItem("DNSRecords", { Domain: "", Wildcard: false, IP: [], TXT: [] })}
					/>
				</Section>

				<Section title={`Networks (${form.Networks.length})`} defaultOpen={false}>
					{form.Networks.map((net, i) => (
						<div key={i} className="mt-1.5 flex items-center gap-1.5 border-l-2 border-base-300 py-1 pl-3">
							<input className="input input-sm flex-1" placeholder="Tag" value={net.Tag || ""} onChange={(e) => setItem("Networks", i, { ...net, Tag: e.target.value })} />
							<input className="input input-sm flex-1" placeholder="Network" value={net.Network || ""} onChange={(e) => setItem("Networks", i, { ...net, Network: e.target.value })} />
							<input className="input input-sm flex-1" placeholder="Nat" value={net.Nat || ""} onChange={(e) => setItem("Networks", i, { ...net, Nat: e.target.value })} />
							<RemoveBtn onClick={() => removeItem("Networks", i)} />
						</div>
					))}
					<AddBtn label="Add Network" onClick={() => addItem("Networks", { Tag: "", Network: "", Nat: "" })} />
				</Section>

				<Section title={`Routes (${form.Routes.length})`} defaultOpen={false}>
					{form.Routes.map((route, i) => (
						<div key={i} className="mt-1.5 flex items-center gap-1.5 border-l-2 border-warning/30 py-1 pl-3">
							<input className="input input-sm flex-1" placeholder="Address" value={route.Address || ""} onChange={(e) => setItem("Routes", i, { ...route, Address: e.target.value })} />
							<input className="input input-sm w-24" placeholder="Metric" value={route.Metric || ""} onChange={(e) => setItem("Routes", i, { ...route, Metric: e.target.value })} />
							<RemoveBtn onClick={() => removeItem("Routes", i)} />
						</div>
					))}
					<AddBtn label="Add Route" onClick={() => addItem("Routes", { Address: "", Metric: "" })} />
				</Section>

				{(form.WindowsGUID || form.ConfigFormat) && (
					<Section title="System" defaultOpen={false}>
						{form.WindowsGUID && (
							<div className="flex items-baseline gap-3 py-1 text-xs">
								<span className="w-28 shrink-0 opacity-50">Windows GUID</span>
								<code className="truncate font-mono">{form.WindowsGUID}</code>
							</div>
						)}
						{form.ConfigFormat && (
							<div className="flex items-baseline gap-3 py-1 text-xs">
								<span className="w-28 shrink-0 opacity-50">Config Format</span>
								<code className="font-mono">{form.ConfigFormat}</code>
							</div>
						)}
					</Section>
				)}
			</div>
		</Dialog>
	)
}

export default TunnelFormDialog
