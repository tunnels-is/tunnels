import { useEffect, useState } from "react"
import Dialog from "@/components/Dialog"
import { Field, TextField } from "@/components/ui"
import { createServer } from "@/store/actions"

const EMPTY = { Tag: "", Country: "", IP: "", Port: "", DataPort: "", PubKey: "" }

const ServerFormDialog = ({ open, onClose }) => {
	const [form, setForm] = useState(EMPTY)
	const [saving, setSaving] = useState(false)

	useEffect(() => {
		if (open) setForm(EMPTY)
	}, [open])

	const set = (key) => (e) => setForm({ ...form, [key]: e.target.value })

	const save = async () => {
		setSaving(true)
		const ok = await createServer(form)
		setSaving(false)
		if (ok) onClose()
	}

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title="New Server"
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
			<div className="grid grid-cols-2 gap-x-3">
				<TextField label="Tag" value={form.Tag} onChange={set("Tag")} />
				<TextField label="Country" value={form.Country} onChange={set("Country")} />
				<TextField label="IP" value={form.IP} onChange={set("IP")} />
				<TextField label="Port" value={form.Port} onChange={set("Port")} />
				<TextField label="Data Port" value={form.DataPort} onChange={set("DataPort")} />
			</div>
			<Field label="Public Key">
				<textarea className="textarea min-h-16 w-full font-mono text-xs" value={form.PubKey} onChange={set("PubKey")} />
			</Field>
		</Dialog>
	)
}

export default ServerFormDialog
