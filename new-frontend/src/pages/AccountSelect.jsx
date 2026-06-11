import { useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { X } from "lucide-react"
import { Card, InfoRow, Page } from "@/components/ui"
import { deleteUserFile, fetchUsers } from "@/store/actions"
import { fullDate } from "@/lib/format"
import { useStore } from "@/store/store"

const AccountSelect = () => {
	const navigate = useNavigate()
	const users = useStore((s) => s.users)
	const setUser = useStore((s) => s.setUser)

	useEffect(() => {
		fetchUsers()
	}, [])

	const selectUser = (user) => {
		setUser(user)
		navigate("/account")
		window.location.reload()
	}

	const removeUser = async (e, user) => {
		e.stopPropagation()
		await deleteUserFile(user.SaveFileHash)
		fetchUsers()
	}

	return (
		<Page
			actions={
				<button className="btn btn-primary btn-sm" onClick={() => navigate("/login/1")}>
					Add Account
				</button>
			}
		>
			<div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))" }}>
				{users?.map((u) => (
					<div key={u._id} className="cursor-pointer" onClick={() => selectUser(u)}>
						<Card
							className="transition-colors hover:border-primary"
							actions={
								<button className="btn btn-square btn-ghost btn-xs text-error" onClick={(e) => removeUser(e, u)}>
									<X size={14} />
								</button>
							}
						>
							<InfoRow label="Email" value={u.Email || "anonymous"} />
							<InfoRow label="ID" value={u._id} mono />
							<InfoRow label="Server" value={u.ControlServer ? u.ControlServer.Host + ":" + u.ControlServer.Port : "?"} />
							<InfoRow label="Expiration" value={fullDate(u.SubExpiration)} />
						</Card>
					</div>
				))}
			</div>
		</Page>
	)
}

export default AccountSelect
