import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react"
import { Page } from "@/components/ui"
import BandwidthCharts from "@/components/BandwidthCharts"
import { api, disconnect, fetchServers, fetchState } from "@/store/actions"
import { matchServerCountry, resolveUserCountry } from "@/lib/geo"
import { useStore } from "@/store/store"

const TimezoneMap = lazy(() => import("@/components/TimezoneMap"))

const toPicked = (data, list = []) => {
	if (!data) return null
	const tag = data.ServerTag || data.tag || ""
	const ip = data.ServerIP || data.ip || ""
	const fromList = list.find((s) => (tag && s.Tag === tag) || (ip && s.IP === ip))
	return {
		tag: tag || fromList?.Tag,
		ip: ip || fromList?.IP,
		country: data.Country || data.country || fromList?.Country,
		latencyMS: data.LatencyMS ?? data.latencyMS,
		serverID: fromList?._id || fromList?.ID,
	}
}

const Dashboard = () => {
	const user = useStore((s) => s.user)
	const servers = useStore((s) => s.servers)
	const activeTunnels = useStore((s) => s.activeTunnels)
	const timezone = useStore((s) => s.timezone)
	const config = useStore((s) => s.config)
	const notifySuccess = useStore((s) => s.notifySuccess)

	const [autoConnecting, setAutoConnecting] = useState(false)
	const [probing, setProbing] = useState(false)

	const [pickedServer, setPickedServer] = useState(null)


	const myActiveTunnels = useMemo(
		() => (activeTunnels || []).filter((at) => at.CR?.UserID && at.CR.UserID === user?._id),
		[activeTunnels, user?._id],
	)

	const connectedServer = useMemo(() => {
		const serverID = myActiveTunnels?.[0]?.CR?.ServerID
		return serverID ? servers.find((s) => s._id === serverID || s.ID === serverID) : undefined
	}, [myActiveTunnels, servers])


	const userCountry = useMemo(() => resolveUserCountry(timezone), [timezone])

	const matchedServerCountry = useMemo(
		() => matchServerCountry(userCountry, (servers || []).map((s) => s.Country)),
		[userCountry, servers],
	)

	const runProbe = useCallback(async () => {
		if (!user?._id || !user?.ControlServer) return
		setProbing(true)
		try {
			let list = servers || []
			if (!list.length) {
				list = (await fetchServers({ force: true })) || []
			}
			const resp = await api(
				"probeServer",
				{
					Country: userCountry || "",
					UserID: user._id || "",
					DeviceToken: user?.DeviceToken?.DT || "",
					Server: user.ControlServer,
				},
				{ timeout: 120000, silent: true },
			)
			if (resp.status === 200 && resp.data) {
				setPickedServer(toPicked(resp.data, list))
			}
		} finally {
			setProbing(false)
		}
	}, [user, servers, userCountry])

	useEffect(() => {
		fetchServers()
		fetchState()
	}, [])


	useEffect(() => {
		if (!user?._id || !user?.ControlServer) return
		if (myActiveTunnels.length > 0) return
		if (pickedServer) return
		runProbe()

	}, [user?._id, user?.ControlServer, servers?.length, myActiveTunnels.length])


	useEffect(() => {
		if (!connectedServer) return
		setPickedServer((prev) => ({
			tag: connectedServer.Tag || prev?.tag,
			ip: connectedServer.IP || prev?.ip,
			country: connectedServer.Country || prev?.country,
			latencyMS: prev?.latencyMS,
			serverID: connectedServer._id || connectedServer.ID || prev?.serverID,
		}))
	}, [connectedServer])

	const autoConnect = async () => {
		if (autoConnecting) return
		setAutoConnecting(true)
		try {
			let list = servers || []
			if (!list.length) {
				list = (await fetchServers({ force: true })) || []
			}
			const resp = await api(
				"autoConnect",
				{
					Country: userCountry || "",
					UserID: user?._id || "",
					DeviceToken: user?.DeviceToken?.DT || "",
					Server: user?.ControlServer,
				},
				{ timeout: 120000 },
			)
			if (resp.status === 200 && resp.data) {
				const pick = toPicked(resp.data, list)
				setPickedServer(pick)
				if (pick?.tag) {
					notifySuccess(`Connected to ${pick.tag} (${pick.latencyMS ?? "?"}ms)`)
				}
			}
			await fetchState()
			await fetchServers({ force: true })
		} finally {
			setAutoConnecting(false)
		}
	}

	return (
		<Page>
			<Suspense fallback={<div className="mb-4 h-64 w-full border border-base-300 bg-base-100" />}>
				<TimezoneMap
					timezone={timezone}
					userCountry={userCountry}
					matchedServerCountry={matchedServerCountry}
					serverCountry={connectedServer?.Country || pickedServer?.country}
					pickedServer={pickedServer}
					connecting={autoConnecting}
					probing={probing}
					connected={myActiveTunnels?.length > 0}
					onConnect={autoConnect}
					onDisconnect={() => myActiveTunnels?.[0] && disconnect(myActiveTunnels[0])}
				/>
			</Suspense>

			{config?.BandwidthGraphs && <BandwidthCharts />}
		</Page>
	)
}

export default Dashboard
