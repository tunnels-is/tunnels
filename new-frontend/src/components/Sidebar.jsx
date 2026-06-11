import { useLocation, useNavigate } from "react-router-dom"
import { BarChart3, CircleUser, Gauge, Globe, Info, Lock, Logs, Monitor, Network, Settings, Users } from "lucide-react"
import { useStore } from "@/store/store"

const Sidebar = () => {
	const navigate = useNavigate()
	const { pathname } = useLocation()
	const user = useStore((s) => s.user)
	const config = useStore((s) => s.config)

	const loggedIn = !!user?.Email || !!user?._id
	const groups = [
		{
			title: "",
			items: [
				{ icon: Lock, label: "Login", route: "login", show: !loggedIn },
				{ icon: Network, label: "Tunnels", route: "tunnels", show: loggedIn },
				{ icon: Monitor, label: "Devices", route: "devices", show: loggedIn },
				{ icon: Gauge, label: "Bandwidth", route: "bandwidth", show: loggedIn && !!config?.BandwidthGraphs },
			],
		},
		{
			title: "DNS",
			items: [
				{ icon: Globe, label: "Settings", route: "dns", show: true },
				{ icon: BarChart3, label: "Stats", route: "dnsstats", show: true },
			],
		},
		{
			title: "App",
			items: [
				{ icon: Settings, label: "Settings", route: "settings", show: true },
				{ icon: Users, label: "Accounts", route: "accounts", show: !loggedIn },
				{ icon: Logs, label: "Logs", route: "logs", show: true },
				{ icon: Info, label: "Support", route: "help", show: true },
			],
		},
	]

	const parts = pathname.split("/")
	const isActive = (route) =>
		parts.includes(route) || (parts[1] === "" && (route === "login" || route === "tunnels"))

	return (
		<div className="group/sidebar fixed top-0 left-0 z-50 flex h-screen w-14 flex-col overflow-hidden border-r border-base-300 bg-base-100 transition-all duration-200 hover:w-52">
			<div className="flex-1 space-y-3 overflow-y-auto py-3">
				{groups.map((g) => (
					<div key={g.title}>
						{g.title && (
							<div className="mb-1 overflow-hidden px-5">
								<h2 className="whitespace-nowrap text-[10px] font-semibold uppercase tracking-wider opacity-0 transition-opacity duration-200 group-hover/sidebar:opacity-60">
									{g.title}
								</h2>
							</div>
						)}
						<div className="space-y-0.5">
							{g.items.filter((i) => i.show).map((i) => (
								<button
									key={i.label}
									onClick={() => navigate("/" + i.route)}
									className={
										"flex w-full items-center gap-3 overflow-hidden px-5 py-1.5 text-[13px] font-medium transition-all " +
										(isActive(i.route)
											? "border-r-2 border-primary bg-base-200 text-base-content"
											: "text-base-content/50 hover:bg-base-200 hover:text-base-content")
									}
								>
									<i.icon size={16} className="shrink-0" />
									<span className="whitespace-nowrap opacity-0 transition-opacity duration-200 group-hover/sidebar:opacity-100">
										{i.label}
									</span>
								</button>
							))}
						</div>
					</div>
				))}
			</div>

			{loggedIn && (
				<div
					className="shrink-0 cursor-pointer border-t border-base-300 pb-2 pt-3"
					onClick={() => navigate("/account")}
				>
					<div className="flex items-center overflow-hidden rounded-md px-3.5 py-1.5 transition-colors hover:bg-base-200">
						<div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-base-300">
							<CircleUser size={14} />
						</div>
						<div className="ml-2 min-w-0 flex-1 opacity-0 transition-opacity duration-200 group-hover/sidebar:opacity-100">
							<div className="truncate text-xs font-medium">{user.Email || "anonymous"}</div>
						</div>
					</div>
				</div>
			)}
		</div>
	)
}

export default Sidebar
