import { lazy, Suspense, useEffect, useMemo, useState } from "react"
import { Page } from "@/components/ui"
import BandwidthCharts from "@/components/BandwidthCharts"
import { api, disconnect, fetchServers, fetchState } from "@/store/actions"
import { resolveTargetCountry } from "@/lib/geo"
import { useStore } from "@/store/store"

const TimezoneMap = lazy(() => import("@/components/TimezoneMap"))

const Dashboard = () => {
	const user = useStore((s) => s.user)
	const servers = useStore((s) => s.servers)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const timezone = useStore((s) => s.timezone)
	const config = useStore((s) => s.config)
	const notifySuccess = useStore((s) => s.notifySuccess)

	const [autoConnecting, setAutoConnecting] = useState(false)

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])

	// the server we're currently connected to (first active tunnel)
	const connectedServer = useMemo(() => {
		const serverID = activeTunnels?.[0]?.CR?.ServerID
		return serverID ? servers.find((s) => s._id === serverID) : undefined
	}, [activeTunnels, servers])

	// timezone country, or the nearest server country by UTC offset when the
	// timezone names no country (UTC, Etc/*)
	const targetCountry = useMemo(
		() => resolveTargetCountry(timezone, servers.map((s) => s.Country)),
		[timezone, servers],
	)

	// The UI only translates timezone -> country; the client backend does the
	// rest (server discovery, latency probing, device bookkeeping, connect).
	const autoConnect = async () => {
		if (autoConnecting) return
		setAutoConnecting(true)
		try {
			const resp = await api(
				"autoConnect",
				{
					Country: targetCountry,
					UserID: user?._id || "",
					DeviceToken: user?.DeviceToken?.DT || "",
					Server: user?.ControlServer,
				},
				{ timeout: 120000 },
			)
			if (resp.status === 200 && resp.data?.ServerTag) {
				notifySuccess(`Connected to ${resp.data.ServerTag} (${resp.data.LatencyMS}ms)`)
			}
			await fetchState()
		} finally {
			setAutoConnecting(false)
		}
	}

	return (
		<Page>
			<Suspense fallback={<div className="mb-4 h-64 w-full border border-base-300 bg-base-100" />}>
				<TimezoneMap
					timezone={timezone}
					country={targetCountry}
					serverCountry={connectedServer?.Country}
					connecting={autoConnecting}
					connected={activeTunnels?.length > 0}
					onConnect={autoConnect}
					onDisconnect={() => activeTunnels?.[0] && disconnect(activeTunnels[0])}
				/>
			</Suspense>

			{config?.BandwidthGraphs && <BandwidthCharts />}
		</Page>
	)
}

export default Dashboard
