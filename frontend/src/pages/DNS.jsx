import { useEffect, useState } from "react"
import { FileText, Pencil, RefreshCw, Save, Trash2, X } from "lucide-react"
import Dialog from "@/components/Dialog"
import { Card, Page, TextField, Toggle } from "@/components/ui"
import {
	fetchState,
	getDNSListContent,
	saveConfig,
	setDNSListContent,
	toggleConfigKey,
	updateBlockLists,
	updateWhiteLists,
} from "@/store/actions"
import { useStore } from "@/store/store"

const BEHAVIOUR_OPTIONS = [
	{ key: "DNSOverHTTPS", label: "Secure DNS" },
	{ key: "LogBlockedDomains", label: "Log Blocked" },
	{ key: "LogAllDomains", label: "Log All" },
	{ key: "DNSstats", label: "Stats" },
	{ key: "DNSHTTPSAutomatic", label: "Dynamic Encryption" },
]

const NEW_RECORD = { Domain: "yourdomain.com", IP: ["127.0.0.1"], TXT: ["yourdomain.com text record"], Wildcard: true }

const isCustomList = (list) => (list?.Tag || "").toLowerCase() === "custom"

const ListRow = ({ enabled, onToggle, name, meta, onEdit, onDelete, onEditDomains, custom }) => (
	<div className="group flex items-center gap-3 border-b border-base-200 px-1 py-2 last:border-0">
		<button
			className={"btn btn-xs w-12 shrink-0 " + (enabled ? "btn-success" : "btn-ghost")}
			onClick={onToggle}
			disabled={!onToggle}
			title={onToggle ? (enabled ? "Click to disable" : "Click to enable") : undefined}
		>
			{enabled ? "ON" : "OFF"}
		</button>
		<div className="min-w-0 flex-1">
			<div className="flex items-center gap-1.5">
				<div className="truncate text-[13px] font-medium">{name}</div>
				{custom && (
					<span className="shrink-0 rounded bg-base-200 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide opacity-60">
						local
					</span>
				)}
			</div>
			{meta && <div className="truncate font-mono text-[11px] opacity-50">{meta}</div>}
		</div>
		<div className="flex items-center gap-0.5">
			{onEditDomains && (
				<button className="btn btn-outline btn-xs gap-1" onClick={onEditDomains} title="Edit domains in this list">
					<FileText size={12} />
					Edit
				</button>
			)}
			<div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
				{onEdit && (
					<button className="btn btn-square btn-ghost btn-xs" onClick={onEdit} title="Edit list settings">
						<Pencil size={12} />
					</button>
				)}
				{onDelete && (
					<button className="btn btn-square btn-ghost btn-xs text-error" onClick={onDelete} title="Remove list">
						<Trash2 size={12} />
					</button>
				)}
			</div>
		</div>
	</div>
)

const EmptyRow = ({ children }) => <div className="px-1 py-5 text-center text-[11px] italic opacity-40">{children}</div>

const ListDialog = ({ open, onClose, title, list, onChange, onSave }) => (
	<Dialog
		open={open}
		onClose={onClose}
		title={title}
		actions={
			<>
				<button className="btn btn-ghost btn-sm" onClick={onClose}>
					Cancel
				</button>
				<button className="btn btn-primary btn-sm" onClick={onSave}>
					<Save size={12} /> Save
				</button>
			</>
		}
	>
		{list && (
			<div className="space-y-2">
				<TextField label="Tag" value={list.Tag || ""} onChange={(e) => onChange({ ...list, Tag: e.target.value })} />
				<TextField label="URL" value={list.URL || ""} onChange={(e) => onChange({ ...list, URL: e.target.value })} />
				<Toggle label="Enabled" checked={list.Enabled} onChange={() => onChange({ ...list, Enabled: !list.Enabled })} />
			</div>
		)}
	</Dialog>
)

const DNS = () => {
	const config = useStore((s) => s.config)
	const advanced = useStore((s) => s.advanced)

	const [editing, setEditing] = useState(false)
	const [cfg, setCfg] = useState({ ...config })

	const [dialog, setDialog] = useState(null)
	const [updatingBlockLists, setUpdatingBlockLists] = useState(false)
	const [updatingWhiteLists, setUpdatingWhiteLists] = useState(false)
	const [domainsEditor, setDomainsEditor] = useState(null) // { kind, title, content, loading, saving }

	useEffect(() => {
		fetchState()
	}, [])

	useEffect(() => {
		if (!editing) setCfg({ ...config })
	}, [config, editing])

	const records = config?.DNSRecords || []
	const blockLists = config?.DNSBlockLists || []
	const whiteLists = config?.DNSWhiteLists || []

	const saveServer = async () => {
		const ok = await saveConfig(cfg)
		if (ok) setEditing(false)
	}

	const handleUpdateBlockLists = async () => {
		if (updatingBlockLists) return
		setUpdatingBlockLists(true)
		try {
			await updateBlockLists()
		} finally {
			setUpdatingBlockLists(false)
		}
	}

	const handleUpdateWhiteLists = async () => {
		if (updatingWhiteLists) return
		setUpdatingWhiteLists(true)
		try {
			await updateWhiteLists()
		} finally {
			setUpdatingWhiteLists(false)
		}
	}

	const openDomainsEditor = async (kind) => {
		const title = kind === "blocklist" ? "Edit custom block list" : "Edit custom white list"
		setDomainsEditor({ kind, title, content: "", loading: true, saving: false })
		const data = await getDNSListContent(kind)
		if (!data) {
			setDomainsEditor(null)
			return
		}
		setDomainsEditor({ kind, title, content: data.Content ?? "", loading: false, saving: false })
	}

	const saveDomainsEditor = async () => {
		if (!domainsEditor || domainsEditor.saving) return
		setDomainsEditor({ ...domainsEditor, saving: true })
		const data = await setDNSListContent(domainsEditor.kind, domainsEditor.content)
		if (data) setDomainsEditor(null)
		else setDomainsEditor((prev) => (prev ? { ...prev, saving: false } : null))
	}

	const saveListItem = async (key, value, index) => {
		const list = [...(config[key] || [])]
		if (index >= 0) list[index] = value
		else list.push(value)
		const ok = await saveConfig({ ...config, [key]: list })
		if (ok) setDialog(null)
	}

	const deleteListItem = (key, index) => {
		const list = (config[key] || []).filter((_, i) => i !== index)
		saveConfig({ ...config, [key]: list })
	}

	const toggleListItem = (key, index) => {
		const list = (config[key] || []).map((l, i) => (i === index ? { ...l, Enabled: !l.Enabled } : l))
		saveConfig({ ...config, [key]: list })
	}

	const setAllListsEnabled = (key, enabled) => {
		const current = config[key] || []
		if (current.length === 0) return
		if (current.every((l) => !!l.Enabled === enabled)) return
		const list = current.map((l) => ({ ...l, Enabled: enabled }))
		saveConfig({ ...config, [key]: list })
	}

	const updateDialogValue = (value) => setDialog({ ...dialog, value })

	if (!advanced) {
		return (
			<Page>
				<div className="flex h-40 items-center justify-center text-[13px] opacity-50">
					Enable Advanced mode in Settings to manage DNS.
				</div>
			</Page>
		)
	}

	return (
		<Page>
			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
				<Card
					className="lg:col-span-2 2xl:col-span-3"
					title="DNS server"
					description="Address the resolver listens on and upstream fallback resolvers."
					actions={
						editing ? (
							<>
								<button className="btn btn-primary btn-xs" onClick={saveServer}>
									<Save size={12} /> Save
								</button>
								<button
									className="btn btn-ghost btn-xs"
									onClick={() => {
										setCfg({ ...config })
										setEditing(false)
									}}
								>
									<X size={12} /> Cancel
								</button>
							</>
						) : (
							<button className="btn btn-outline btn-xs" onClick={() => setEditing(true)}>
								<Pencil size={12} /> Edit
							</button>
						)
					}
				>
					{!editing ? (
						<div className="flex flex-wrap items-start gap-x-8 gap-y-3 text-sm">
							<div>
								<div className="mb-1 text-[10px] font-semibold uppercase tracking-wider opacity-50">Listening on</div>
								<code className="font-mono text-[13px]">
									{cfg.DNSServerIP || "0.0.0.0"}:{cfg.DNSServerPort || "53"}
								</code>
							</div>
							<div>
								<div className="mb-1 text-[10px] font-semibold uppercase tracking-wider opacity-50">Primary resolver</div>
								<code className="font-mono text-[13px]">{cfg.DNS1Default || "none"}</code>
							</div>
							<div>
								<div className="mb-1 text-[10px] font-semibold uppercase tracking-wider opacity-50">Backup resolver</div>
								<code className="font-mono text-[13px]">{cfg.DNS2Default || "none"}</code>
							</div>
						</div>
					) : (
						<div className="grid grid-cols-1 gap-x-3 md:grid-cols-4">
							<TextField label="Server IP" value={cfg.DNSServerIP || ""} onChange={(e) => setCfg({ ...cfg, DNSServerIP: e.target.value })} />
							<TextField label="Port" value={cfg.DNSServerPort || ""} onChange={(e) => setCfg({ ...cfg, DNSServerPort: e.target.value })} />
							<TextField label="Primary DNS" value={cfg.DNS1Default || ""} onChange={(e) => setCfg({ ...cfg, DNS1Default: e.target.value })} />
							<TextField label="Backup DNS" value={cfg.DNS2Default || ""} onChange={(e) => setCfg({ ...cfg, DNS2Default: e.target.value })} />
						</div>
					)}
				</Card>

				<Card
					className="lg:col-span-2 2xl:col-span-3"
					title="Behaviour"
					description="Encryption, logging and statistics for the resolver."
				>
					<div className="flex flex-wrap items-center gap-x-6">
						{BEHAVIOUR_OPTIONS.map((opt) => (
							<Toggle key={opt.key} label={opt.label} checked={!!config?.[opt.key]} onChange={() => toggleConfigKey(opt.key)} />
						))}
					</div>
				</Card>

				<Card
					title="Records"
					description="Locally resolved A and TXT records."
					actions={
						<button className="btn btn-primary btn-xs" onClick={() => setDialog({ kind: "record", value: { ...NEW_RECORD }, index: -1 })}>
							Create
						</button>
					}
				>
					{records.length > 0 ? (
						records.map((r, i) => (
							<ListRow
								key={i}
								enabled
								name={r.Domain + (r.Wildcard ? " *" : "")}
								meta={r.IP?.join(", ")}
								onEdit={() => setDialog({ kind: "record", value: structuredClone(r), index: i })}
								onDelete={() => deleteListItem("DNSRecords", i)}
							/>
						))
					) : (
						<EmptyRow>No records configured</EmptyRow>
					)}
				</Card>

				<Card
					title="Block lists"
					description="External lists of domains that will be blocked."
					actions={
						<>
							<button
								className="btn btn-ghost btn-xs"
								onClick={() => setAllListsEnabled("DNSBlockLists", true)}
								disabled={blockLists.length === 0 || blockLists.every((l) => l.Enabled)}
								title="Enable every block list"
							>
								Enable all
							</button>
							<button
								className="btn btn-ghost btn-xs"
								onClick={() => setAllListsEnabled("DNSBlockLists", false)}
								disabled={blockLists.length === 0 || blockLists.every((l) => !l.Enabled)}
								title="Disable every block list"
							>
								Disable all
							</button>
							<button
								className="btn btn-outline btn-xs"
								onClick={handleUpdateBlockLists}
								disabled={updatingBlockLists || blockLists.length === 0}
								title="Re-download all block lists from their URLs"
							>
								<RefreshCw size={12} className={updatingBlockLists ? "animate-spin" : undefined} />
								{updatingBlockLists ? "Updating..." : "Update"}
							</button>
							<button
								className="btn btn-primary btn-xs"
								onClick={() =>
									setDialog({
										kind: "DNSBlockLists",
										value: { Tag: "new-blocklist", URL: "https://example.com/blocklist.txt", Enabled: true, Count: 0 },
										index: -1,
									})
								}
							>
								Create
							</button>
						</>
					}
				>
					{blockLists.length > 0 ? (
						blockLists.map((bl, i) => {
							const custom = isCustomList(bl)
							return (
								<ListRow
									key={i}
									custom={custom}
									enabled={bl.Enabled}
									onToggle={() => toggleListItem("DNSBlockLists", i)}
									name={bl.Tag}
									meta={`${bl.Count?.toLocaleString?.() ?? bl.Count} domains`}
									onEditDomains={custom ? () => openDomainsEditor("blocklist") : undefined}
									onEdit={
										custom
											? undefined
											: () => setDialog({ kind: "DNSBlockLists", value: { ...bl }, index: i })
									}
									onDelete={custom ? undefined : () => deleteListItem("DNSBlockLists", i)}
								/>
							)
						})
					) : (
						<EmptyRow>No block lists configured</EmptyRow>
					)}
				</Card>

				<Card
					title="White lists"
					description="Domains here always resolve, even if they appear on a block list."
					actions={
						<>
							<button
								className="btn btn-outline btn-xs"
								onClick={handleUpdateWhiteLists}
								disabled={updatingWhiteLists || whiteLists.length === 0}
								title="Re-download all white lists from their URLs and reload local lists"
							>
								<RefreshCw size={12} className={updatingWhiteLists ? "animate-spin" : undefined} />
								{updatingWhiteLists ? "Updating..." : "Update"}
							</button>
							<button
								className="btn btn-primary btn-xs"
								onClick={() =>
									setDialog({
										kind: "DNSWhiteLists",
										value: { Tag: "new-whitelist", URL: "https://example.com/whitelist.txt", Enabled: true, Count: 0 },
										index: -1,
									})
								}
							>
								Create
							</button>
						</>
					}
				>
					{whiteLists.length > 0 ? (
						whiteLists.map((wl, i) => {
							const custom = isCustomList(wl)
							return (
								<ListRow
									key={i}
									custom={custom}
									enabled={wl.Enabled}
									onToggle={() => toggleListItem("DNSWhiteLists", i)}
									name={wl.Tag}
									meta={`${wl.Count?.toLocaleString?.() ?? wl.Count} domains`}
									onEditDomains={custom ? () => openDomainsEditor("whitelist") : undefined}
									onEdit={
										custom
											? undefined
											: () => setDialog({ kind: "DNSWhiteLists", value: { ...wl }, index: i })
									}
									onDelete={custom ? undefined : () => deleteListItem("DNSWhiteLists", i)}
								/>
							)
						})
					) : (
						<EmptyRow>No white lists configured</EmptyRow>
					)}
				</Card>
			</div>

			{}
			<Dialog
				open={dialog?.kind === "record"}
				onClose={() => setDialog(null)}
				title={dialog?.index >= 0 ? "Edit DNS record" : "New DNS record"}
				actions={
					<>
						<button className="btn btn-ghost btn-sm" onClick={() => setDialog(null)}>
							Cancel
						</button>
						<button className="btn btn-primary btn-sm" onClick={() => saveListItem("DNSRecords", dialog.value, dialog.index)}>
							<Save size={12} /> Save
						</button>
					</>
				}
			>
				{dialog?.kind === "record" && (
					<div className="space-y-2">
						<TextField
							label="Domain"
							value={dialog.value.Domain || ""}
							onChange={(e) => updateDialogValue({ ...dialog.value, Domain: e.target.value })}
						/>
						<Toggle
							label="Wildcard"
							checked={dialog.value.Wildcard}
							onChange={() => updateDialogValue({ ...dialog.value, Wildcard: !dialog.value.Wildcard })}
						/>
						{["IP", "TXT"].map((key) => (
							<div key={key}>
								<div className="mb-1 text-xs font-semibold opacity-60">{key === "IP" ? "IP addresses" : "TXT records"}</div>
								<div className="space-y-1">
									{(dialog.value[key] || []).map((v, i) => (
										<div key={i} className="flex items-center gap-1">
											<input
												className="input input-sm flex-1"
												value={v}
												onChange={(e) =>
													updateDialogValue({
														...dialog.value,
														[key]: dialog.value[key].map((x, xi) => (xi === i ? e.target.value : x)),
													})
												}
											/>
											<button
												className="btn btn-square btn-ghost btn-xs text-error"
												onClick={() =>
													updateDialogValue({ ...dialog.value, [key]: dialog.value[key].filter((_, xi) => xi !== i) })
												}
											>
												<X size={14} />
											</button>
										</div>
									))}
									<button
										className="btn btn-ghost btn-xs text-success"
										onClick={() => updateDialogValue({ ...dialog.value, [key]: [...(dialog.value[key] || []), ""] })}
									>
										Add {key}
									</button>
								</div>
							</div>
						))}
					</div>
				)}
			</Dialog>

			{}
			<ListDialog
				open={dialog?.kind === "DNSBlockLists" || dialog?.kind === "DNSWhiteLists"}
				onClose={() => setDialog(null)}
				title={
					(dialog?.index >= 0 ? "Edit " : "New ") + (dialog?.kind === "DNSBlockLists" ? "block list" : "white list")
				}
				list={dialog?.value}
				onChange={updateDialogValue}
				onSave={() => saveListItem(dialog.kind, dialog.value, dialog.index)}
			/>

			<Dialog
				open={!!domainsEditor}
				onClose={() => !domainsEditor?.saving && setDomainsEditor(null)}
				title={domainsEditor?.title}
				wide
				actions={
					<>
						<button
							className="btn btn-ghost btn-sm"
							onClick={() => setDomainsEditor(null)}
							disabled={domainsEditor?.saving}
						>
							Cancel
						</button>
						<button
							className="btn btn-primary btn-sm"
							onClick={saveDomainsEditor}
							disabled={domainsEditor?.loading || domainsEditor?.saving}
						>
							<Save size={12} />
							{domainsEditor?.saving ? "Saving..." : "Save"}
						</button>
					</>
				}
			>
				{domainsEditor?.loading ? (
					<div className="py-8 text-center text-sm opacity-50">Loading list…</div>
				) : (
					<div className="space-y-2">
						<p className="text-[11px] leading-relaxed opacity-50">
							One domain per line (e.g. <code className="font-mono">example.com</code>). Lines starting with{" "}
							<code className="font-mono">#</code> are comments. Exact hostnames only —{" "}
							<code className="font-mono">example.com</code> does not include{" "}
							<code className="font-mono">www.example.com</code>.
							{domainsEditor?.kind === "whitelist"
								? " Whitelisted domains always resolve, even if they appear on a block list."
								: " These domains will be blocked by the local DNS resolver."}
						</p>
						<textarea
							className="textarea textarea-bordered h-72 w-full font-mono text-xs leading-relaxed"
							spellCheck={false}
							value={domainsEditor?.content ?? ""}
							onChange={(e) => setDomainsEditor({ ...domainsEditor, content: e.target.value })}
							placeholder={"# one domain per line\nexample.com\ncdn.mycompany.com"}
						/>
					</div>
				)}
			</Dialog>
		</Page>
	)
}

export default DNS
