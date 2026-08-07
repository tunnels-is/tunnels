import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Key, LogOut, RefreshCw, Shield } from "lucide-react"
import { Card, InfoRow, Page } from "@/components/ui"
import { activateLicense, fetchState, logoutAllTokens, logoutToken, refreshApiKey } from "@/store/actions"
import { fullDate } from "@/lib/format"
import { useStore } from "@/store/store"

const TABS = [
	{ key: "account", label: "Account" },
	{ key: "logins", label: "Logins" },
	{ key: "license", label: "License Key" },
]

const Account = () => {
	const navigate = useNavigate()
	const user = useStore((s) => s.user)
	const [tab, setTab] = useState("account")
	const [licenseKey, setLicenseKey] = useState("")

	useEffect(() => {
		if (!user) {
			navigate("/accounts")
			return
		}
		fetchState()
	}, [])

	const tokens = useMemo(
		() => [...(user?.Tokens || [])].sort((a, b) => new Date(b.C) - new Date(a.C)),
		[user],
	)

	if (!user) return null

	return (
		<Page>
			<div role="tablist" className="tabs tabs-border mb-6">
				{TABS.map((t) => (
					<button
						key={t.key}
						role="tab"
						className={"tab " + (tab === t.key ? "tab-active" : "")}
						onClick={() => setTab(t.key)}
					>
						{t.label}
					</button>
				))}
			</div>

			{tab === "account" && (
				<div className="max-w-lg space-y-6">
					<Card>
						<InfoRow label="User" value={user.Email || "anonymous"} />
						<InfoRow label="ID" value={user._id} mono />
						<InfoRow label="Updated" value={user.Updated ? fullDate(user.Updated) : "—"} />
						{user.SubExpiration && <InfoRow label="Subscription" value={fullDate(user.SubExpiration)} />}
						<InfoRow label="API Key" value={user.APIKey} mono />
						{user.Trial && <InfoRow label="Trial" value="Active" />}
					</Card>

					<div className="flex flex-wrap gap-2">
						<button className="btn btn-outline btn-sm" onClick={() => navigate("/accounts")}>
							Switch Account
						</button>
						<button className="btn btn-outline btn-sm" onClick={refreshApiKey}>
							<RefreshCw size={12} /> Re-Generate API Key
						</button>
						<button className="btn btn-outline btn-sm" onClick={() => navigate("/twofactor/create")}>
							<Shield size={12} /> Two-Factor Auth
						</button>
						<button className="btn btn-outline btn-error btn-sm" onClick={logoutAllTokens}>
							<LogOut size={12} /> Log Out All Devices
						</button>
						<button
							className="btn btn-outline btn-error btn-sm"
							onClick={() => user.DeviceToken?.DT && logoutToken(user.DeviceToken, false)}
						>
							<LogOut size={12} /> Logout
						</button>
					</div>
				</div>
			)}

			{tab === "logins" && (
				<Card className="max-w-3xl">
					<table className="table table-sm">
						<thead>
							<tr>
								<th>Name</th>
								<th className="text-right">Created</th>
								<th className="w-24" />
							</tr>
						</thead>
						<tbody>
							{tokens.length > 0 ? (
								tokens.map((t, i) => {
									const isCurrent =
										(t.DT && t.DT === user.DeviceToken?.DT) ||
										(t.N &&
											t.N === user.DeviceToken?.N &&
											t.C &&
											user.DeviceToken?.C &&
											new Date(t.C).getTime() === new Date(user.DeviceToken.C).getTime())
									return (
									<tr key={i} className="hover">
										<td className="font-medium">
											{t.N}
											{isCurrent && <span className="badge badge-ghost badge-xs ml-2">current</span>}
										</td>
										<td className="text-right text-xs opacity-60">{t.C ? fullDate(t.C) : "—"}</td>
										<td className="text-right">
											<button
												className="btn btn-ghost btn-xs text-error"
												onClick={() => logoutToken(t, false)}
											>
												<LogOut size={12} /> Logout
											</button>
										</td>
									</tr>
									)
								})
							) : (
								<tr>
									<td colSpan={3} className="py-6 text-center text-xs opacity-50">
										No active sessions
									</td>
								</tr>
							)}
						</tbody>
					</table>
				</Card>
			)}

			{tab === "license" && (
				<div className="max-w-lg space-y-4">
					{user.Key?.Key && (
						<Card>
							<InfoRow label="Current" value={user.Key.Key} mono />
						</Card>
					)}
					<Card title="Activate License Key">
						<div className="flex items-center gap-2">
							<input
								className="input input-sm flex-1"
								placeholder="Insert License Key"
								value={licenseKey}
								onChange={(e) => setLicenseKey(e.target.value)}
							/>
							<button className="btn btn-primary btn-sm" onClick={() => activateLicense(licenseKey)}>
								<Key size={12} /> Activate
							</button>
						</div>
					</Card>
				</div>
			)}
		</Page>
	)
}

export default Account
