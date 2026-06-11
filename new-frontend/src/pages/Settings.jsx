import { useEffect, useState } from "react"
import { Pencil, Save, X } from "lucide-react"
import { Card, InfoRow, Page, TextField, Toggle } from "@/components/ui"
import { fetchState, saveConfig, toggleConfigKey } from "@/store/actions"
import { THEMES, getTheme, setTheme } from "@/lib/theme"
import { session } from "@/store/session"
import { useStore } from "@/store/store"

const LOGGING_OPTIONS = [
	{ key: "InfoLogging", label: "Info" },
	{ key: "ErrorLogging", label: "Errors" },
	{ key: "ConsoleLogging", label: "Console" },
	{ key: "DebugLogging", label: "Debug" },
	{ key: "BandwidthGraphs", label: "Bandwidth Graphs" },
	{ key: "ConsoleLogOnly", label: "Console Only" },
	{ key: "DeepDebugLoggin", label: "Deep Debug" },
]

const UPDATE_OPTIONS = [
	{ key: "DisableUpdates", label: "Disable Updates" },
	{ key: "AutoDownloadUpdate", label: "Auto Download" },
	{ key: "UpdateWhileConnected", label: "While Connected" },
	{ key: "RestartPostUpdate", label: "Restart After" },
	{ key: "ExitPostUpdate", label: "Exit After" },
]

const Settings = () => {
	const config = useStore((s) => s.config)
	const advanced = useStore((s) => s.advanced)
	const setAdvanced = useStore((s) => s.setAdvanced)
	const state = useStore((s) => s.state)
	const network = useStore((s) => s.network)
	const version = useStore((s) => s.version)
	const apiVersion = useStore((s) => s.apiVersion)

	const [editing, setEditing] = useState(false)
	const [cfg, setCfg] = useState({ ...config })
	const [theme, setThemeState] = useState(getTheme())

	useEffect(() => {
		fetchState()
	}, [])

	useEffect(() => {
		if (!editing) setCfg({ ...config })
	}, [config, editing])

	const changeTheme = (next) => {
		setThemeState(next)
		setTheme(next)
	}

	const updateCfg = (key, value) => {
		if (key === "APICertDomains" || key === "APICertIPs") value = value.split(",")
		setCfg((prev) => ({ ...prev, [key]: value }))
	}

	const saveApi = async () => {
		const ok = await saveConfig(cfg)
		if (ok) setEditing(false)
	}

	const toggleDebug = () => {
		session.set("debug", session.getBool("debug") ? "false" : "true")
		window.location.reload()
	}

	const basePath = state?.BasePath
	const logPath = state?.LogPath !== basePath ? state?.LogPath : ""
	const logFileName = state?.LogFileName?.replace(state?.LogPath, "")

	return (
		<Page
			actions={
				<div className="flex items-center gap-3 text-[11px] opacity-60">
					<span>
						App <code className="font-mono">{version || "unknown"}</code>
					</span>
					<span>
						API <code className="font-mono">{apiVersion || "unknown"}</code>
					</span>
				</div>
			}
		>
			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
				<Card
					title="Appearance"
					description="Select a color theme."
					actions={
						<select className="select select-sm" value={theme} onChange={(e) => changeTheme(e.target.value)}>
							{THEMES.map((t) => (
								<option key={t.value} value={t.value}>
									{t.label}
								</option>
							))}
						</select>
					}
				>
					<p className="text-xs opacity-60">
						Current theme: <code className="font-mono">{theme}</code>
					</p>
				</Card>

				<Card title="Advanced" description="Show advanced configuration: API server, updates, network, DNS and system details.">
					<Toggle label="Advanced mode" checked={advanced} onChange={() => setAdvanced(!advanced)} />
				</Card>

				{advanced && (
				<Card
					className="lg:col-span-2 2xl:col-span-3"
					title="API server"
					description="Address the client listens on, plus optional TLS certificate."
					actions={
						editing ? (
							<>
								<button className="btn btn-primary btn-xs" onClick={saveApi}>
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
						<div className="flex flex-wrap items-center gap-x-8 gap-y-3 text-sm">
							<InfoRow label="Address" value={(cfg.APIIP || "0.0.0.0") + ":" + (cfg.APIPort || "—")} mono />
							<InfoRow label="TLS Cert" value={cfg.APICert || "none"} mono />
							<InfoRow label="TLS Key" value={cfg.APIKey || "none"} mono />
						</div>
					) : (
						<div className="grid grid-cols-1 gap-x-3 md:grid-cols-2">
							<TextField label="IP" value={cfg.APIIP || ""} onChange={(e) => updateCfg("APIIP", e.target.value)} />
							<TextField label="Port" value={cfg.APIPort || ""} onChange={(e) => updateCfg("APIPort", e.target.value)} />
							<TextField label="Cert Domains" value={cfg.APICertDomains || ""} onChange={(e) => updateCfg("APICertDomains", e.target.value)} />
							<TextField label="Cert IPs" value={cfg.APICertIPs || ""} onChange={(e) => updateCfg("APICertIPs", e.target.value)} />
							<TextField label="Cert Path" value={cfg.APICert || ""} onChange={(e) => updateCfg("APICert", e.target.value)} />
							<TextField label="Key Path" value={cfg.APIKey || ""} onChange={(e) => updateCfg("APIKey", e.target.value)} />
						</div>
					)}
				</Card>
				)}

				<Card title="Logging" description="Select which event types are captured.">
					<div className="grid grid-cols-1 sm:grid-cols-2">
						{LOGGING_OPTIONS.map((opt) => (
							<Toggle key={opt.key} label={opt.label} checked={!!config?.[opt.key]} onChange={() => toggleConfigKey(opt.key)} />
						))}
						<Toggle label="Debug Mode" checked={session.getBool("debug")} onChange={toggleDebug} />
					</div>
				</Card>

				{advanced && (
					<Card title="Updates" description="Behaviour when a new build of Tunnels is available.">
						<div className="grid grid-cols-1 sm:grid-cols-2">
							{UPDATE_OPTIONS.map((opt) => (
								<Toggle key={opt.key} label={opt.label} checked={!!config?.[opt.key]} onChange={() => toggleConfigKey(opt.key)} />
							))}
						</div>
					</Card>
				)}

				{advanced && (
					<Card title="DNS" description="The local DNS resolver is enabled by default.">
						<Toggle label="Disable DNS" checked={!!config?.DisableDNS} onChange={() => toggleConfigKey("DisableDNS")} />
					</Card>
				)}

				{advanced && (
					<Card title="Network" description="Detected default network interface (read-only).">
						<InfoRow label="Interface" value={network?.DefaultInterfaceName || "unknown"} />
						<InfoRow label="IP Address" value={network?.DefaultInterface || "unknown"} mono />
						<InfoRow label="Interface ID" value={network?.DefaultInterfaceID ?? "unknown"} mono />
						<InfoRow label="Gateway" value={network?.DefaultGateway || "unknown"} mono />
					</Card>
				)}

				{advanced && (
					<Card
						className="lg:col-span-2 2xl:col-span-3"
						title="System"
						description="Paths, files and privileges this app is running with."
					>
						<div className="grid grid-cols-1 gap-x-8 md:grid-cols-2">
							<div>
								<InfoRow label="Base Path" value={basePath || "unknown"} mono />
								<InfoRow label="Config" value={state?.ConfigFileName || "unknown"} mono />
								<InfoRow label="Log Path" value={logPath || "Default"} mono />
							</div>
							<div>
								<InfoRow label="Log File" value={logFileName || "unknown"} mono />
								<InfoRow label="Admin" value={state?.IsAdmin ? "Yes" : "No"} />
								<InfoRow label="API Version" value={apiVersion || "unknown"} mono />
							</div>
						</div>
					</Card>
				)}
			</div>
		</Page>
	)
}

export default Settings
