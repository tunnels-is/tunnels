import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Minus, Plus, Save, X } from "lucide-react"
import { Card, Field, Page, TextField, Toggle } from "@/components/ui"
import { fetchServers, fetchState, saveTunnel } from "@/store/actions"
import { ENC_TYPES } from "@/lib/format"
import { useStore } from "@/store/store"

const FEATURE_TOGGLES = [
	{ key: "DNSBlocking", label: "DNS Blocking" },
	{ key: "LocalhostNat", label: "Localhost NAT" },
	{ key: "AutoReconnect", label: "Auto Reconnect" },
	{ key: "AutoConnect", label: "Auto Connect" },
	{ key: "KillSwitch", label: "Kill Switch" },
	{ key: "EnableDefaultRoute", label: "Default Route" },
	{ key: "EnableWAN", label: "WAN Routing" },
]

const RemoveBtn = ({ onClick }) => (
	<button type="button" className="btn btn-square btn-ghost btn-xs shrink-0 text-error" onClick={onClick}>
		<Minus size={14} />
	</button>
)

const AddBtn = ({ label, onClick }) => (
	<button type="button" className="btn btn-ghost btn-xs mt-1 self-start text-success" onClick={onClick}>
		<Plus size={12} /> {label}
	</button>
)

const StringListEditor = ({ items, onChange, placeholder }) => (
	<div className="flex flex-col gap-1.5">
		{items.map((item, i) => (
			<div key={i} className="flex items-center gap-1.5">
			<input
				className="input input-sm flex-1 font-mono"
				placeholder={placeholder}
				value={item}
				onChange={(e) => onChange(items.map((v, x) => (x === i ? e.target.value : v)))}
			/>
				<RemoveBtn onClick={() => onChange(items.filter((_, x) => x !== i))} />
			</div>
		))}
		{items.length === 0 && <p className="py-1 text-xs italic opacity-40">None configured</p>}
		<AddBtn label="Add" onClick={() => onChange([...items, ""])} />
	</div>
)

const DNSRecordEditor = ({ record, onChange, onRemove }) => {
	const set = (key, val) => onChange({ ...record, [key]: val })
	const arrayEditor = (key, label) => (
		<div className="flex-1">
			<span className="text-[10px] font-semibold uppercase tracking-wider opacity-50">{label}</span>
			<div className="mt-1 flex flex-col gap-1">
				{(record[key] || []).map((v, i) => (
					<div key={i} className="flex items-center gap-1">
						<input
							className="input input-xs flex-1 font-mono"
							value={v}
							onChange={(e) => set(key, record[key].map((x, idx) => (idx === i ? e.target.value : x)))}
						/>
						<RemoveBtn onClick={() => set(key, record[key].filter((_, idx) => idx !== i))} />
					</div>
				))}
				<AddBtn label={label} onClick={() => set(key, [...(record[key] || []), ""])} />
			</div>
		</div>
	)
	return (
		<div className="rounded-box border border-base-300 bg-base-200/40 p-3">
			<div className="flex items-center gap-2">
				<input
					className="input input-sm flex-1"
					placeholder="Domain"
					value={record.Domain || ""}
					onChange={(e) => set("Domain", e.target.value)}
				/>
				<button
					type="button"
					title="Wildcard — match all subdomains"
					className={"btn btn-xs " + (record.Wildcard ? "btn-primary" : "btn-ghost")}
					onClick={() => set("Wildcard", !record.Wildcard)}
				>
					*
				</button>
				<RemoveBtn onClick={onRemove} />
			</div>
			<div className="mt-2 flex gap-4">
				{arrayEditor("IP", "IPs")}
				{arrayEditor("TXT", "TXT")}
			</div>
		</div>
	)
}

const TunnelForm = () => {
	const { tag } = useParams()
	const navigate = useNavigate()
	const tunnels = useStore((s) => s.tunnels)
	const servers = useStore((s) => s.servers)
	const activeTunnels = useStore((s) => s.activeTunnels)

	const [form, setForm] = useState(null)
	const [saving, setSaving] = useState(false)

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	useEffect(() => {
		setForm(null)
	}, [tag])

	useEffect(() => {
		if (form) return
		const tunnel = tunnels.find((t) => t.Tag === tag)
		if (!tunnel) return
		const clone = structuredClone(tunnel)
		clone.DNSRecords ||= []
		clone.Networks ||= []
		clone.Routes ||= []
		clone.DNSServers ||= []
		setForm(clone)
	}, [tunnels, tag, form])

	const connected = (activeTunnels || []).some((at) => at.CR?.Tag === tag)

	if (!form) {
		return (
			<Page>
				<div className="flex h-40 items-center justify-center text-[13px] opacity-50">
					{tunnels.length === 0 ? "Loading..." : `Tunnel "${tag}" not found.`}
				</div>
			</Page>
		)
	}

	const set = (key, val) => setForm((f) => ({ ...f, [key]: val }))
	const setItem = (key, i, val) => set(key, form[key].map((v, x) => (x === i ? val : v)))
	const removeItem = (key, i) => set(key, form[key].filter((_, x) => x !== i))
	const addItem = (key, val) => set(key, [...form[key], val])

	const save = async () => {
		setSaving(true)
		const ok = await saveTunnel(form, tag)
		setSaving(false)
		if (ok) navigate("/tunnels")
	}

	return (
		<Page>
			<div className="mb-5 flex flex-wrap items-center gap-3">
				<button
					type="button"
					className="btn btn-square btn-ghost btn-sm shrink-0"
					onClick={() => navigate("/tunnels")}
					title="Back to tunnels"
				>
					<ArrowLeft size={16} />
				</button>
				<div className="flex min-w-0 flex-1 items-center gap-2">
					<h1 className="truncate text-base font-semibold tracking-tight">{tag}</h1>
					{connected && (
						<span className="badge badge-success badge-sm shrink-0 gap-1 border-none bg-success/15 text-success">
							<span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-current" />
							connected
						</span>
					)}
				</div>
				<div className="flex shrink-0 items-center gap-2">
					<button type="button" className="btn btn-ghost btn-sm gap-1.5" onClick={() => navigate("/tunnels")}>
						<X size={14} /> Cancel
					</button>
					<button type="button" className="btn btn-primary btn-sm gap-1.5" disabled={saving} onClick={save}>
						<Save size={14} /> {saving ? "Saving..." : "Save"}
					</button>
				</div>
			</div>

			{connected && (
				<div className="alert alert-warning mb-4 py-2 text-xs">
					This tunnel is connected — disconnect it before saving changes. Peers can be managed while connected
					from the tunnel&apos;s peer list.
				</div>
			)}

			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<Card title="General" description="Identity, server and transport settings.">
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
				</Card>

				<Card title="Features" description="Behaviour of this tunnel while connected.">
					<div className="grid grid-cols-1 sm:grid-cols-2">
						{FEATURE_TOGGLES.map((opt) => {

							if (opt.key === "KillSwitch") {
								const allowed = !!form.EnableDefaultRoute
								return (
									<Toggle
										key={opt.key}
										label={opt.label}
										warning={!allowed ? "Requires Default Route to be enabled" : undefined}
										checked={allowed && !!form.KillSwitch}
										disabled={!allowed}
										onChange={() => set("KillSwitch", !form.KillSwitch)}
									/>
								)
							}

							if (opt.key === "EnableDefaultRoute") {
								return (
									<Toggle
										key={opt.key}
										label={opt.label}
										checked={!!form.EnableDefaultRoute}
										onChange={() =>
											setForm((f) => {
												const next = !f.EnableDefaultRoute
												return { ...f, EnableDefaultRoute: next, KillSwitch: next ? f.KillSwitch : false }
											})
										}
									/>
								)
							}
							return <Toggle key={opt.key} label={opt.label} checked={!!form[opt.key]} onChange={() => set(opt.key, !form[opt.key])} />
						})}
					</div>
				</Card>

				<Card title="DNS Servers" description="Resolvers used while this tunnel is up. Defaults apply when empty.">
					<StringListEditor
						items={form.DNSServers}
						onChange={(v) => set("DNSServers", v)}
						placeholder="9.9.9.9"
					/>
				</Card>

				<Card title="Routes" description="Extra routes installed when the tunnel connects.">
					<div className="flex flex-col gap-1.5">
						{form.Routes.map((route, i) => (
							<div key={i} className="flex items-center gap-1.5">
								<input
									className="input input-sm flex-1 font-mono"
									placeholder="10.0.0.0/24"
									value={route.Address || ""}
									onChange={(e) => setItem("Routes", i, { ...route, Address: e.target.value })}
								/>
								<input
									className="input input-sm w-24 font-mono"
									placeholder="Metric"
									value={route.Metric || ""}
									onChange={(e) => setItem("Routes", i, { ...route, Metric: e.target.value })}
								/>
								<RemoveBtn onClick={() => removeItem("Routes", i)} />
							</div>
						))}
						{form.Routes.length === 0 && <p className="py-1 text-xs italic opacity-40">None configured</p>}
						<AddBtn label="Add Route" onClick={() => addItem("Routes", { Address: "", Metric: "0" })} />
					</div>
				</Card>

				<Card title="Networks" description="Network mappings with optional NAT.">
					<div className="flex flex-col gap-1.5">
						{form.Networks.map((net, i) => (
							<div key={i} className="flex items-center gap-1.5">
								<input
									className="input input-sm w-28"
									placeholder="Tag"
									value={net.Tag || ""}
									onChange={(e) => setItem("Networks", i, { ...net, Tag: e.target.value })}
								/>
								<input
									className="input input-sm flex-1 font-mono"
									placeholder="Network"
									value={net.Network || ""}
									onChange={(e) => setItem("Networks", i, { ...net, Network: e.target.value })}
								/>
								<input
									className="input input-sm flex-1 font-mono"
									placeholder="NAT"
									value={net.Nat || ""}
									onChange={(e) => setItem("Networks", i, { ...net, Nat: e.target.value })}
								/>
								<RemoveBtn onClick={() => removeItem("Networks", i)} />
							</div>
						))}
						{form.Networks.length === 0 && <p className="py-1 text-xs italic opacity-40">None configured</p>}
						<AddBtn label="Add Network" onClick={() => addItem("Networks", { Tag: "", Network: "", Nat: "" })} />
					</div>
				</Card>

				<Card title="DNS Records" description="Custom records answered by the local resolver.">
					<div className="flex flex-col gap-2">
						{form.DNSRecords.map((rec, i) => (
							<DNSRecordEditor
								key={i}
								record={rec}
								onChange={(val) => setItem("DNSRecords", i, val)}
								onRemove={() => removeItem("DNSRecords", i)}
							/>
						))}
						{form.DNSRecords.length === 0 && <p className="py-1 text-xs italic opacity-40">None configured</p>}
						<AddBtn
							label="Add DNS Record"
							onClick={() => addItem("DNSRecords", { Domain: "", Wildcard: false, IP: [], TXT: [] })}
						/>
					</div>
				</Card>

				{(form.WindowsGUID || form.ConfigFormat) && (
					<Card title="System" description="Read-only system identifiers.">
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
					</Card>
				)}
			</div>
		</Page>
	)
}

export default TunnelForm
