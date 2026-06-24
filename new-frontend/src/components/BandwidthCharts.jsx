import { useEffect, useMemo, useRef, useState } from "react"
import { Card, Toolbar } from "@/components/ui"
import { fetchState } from "@/store/actions"
import { useStore } from "@/store/store"

const TIME_RANGES = [
	{ label: "1m", seconds: 60 },
	{ label: "5m", seconds: 300 },
	{ label: "15m", seconds: 900 },
	{ label: "1h", seconds: 3600 },
	{ label: "24h", seconds: 86400 },
	{ label: "7d", seconds: 604800 },
]

// chart series palette — theme red first, then distinguishable neutrals/accents
// chart series palette — the active theme's primary color first, then
// distinguishable neutrals/accents
const seriesColors = () => {
	const primary = getComputedStyle(document.documentElement).getPropertyValue("--color-primary").trim() || "#d82e2e"
	return [primary, "#737373", "#f59e0b", "#22c55e", "#a855f7", "#ec4899", "#84cc16", "#14b8a6"]
}

const formatBytes = (bytes) => {
	if (bytes === 0) return "0 B"
	const units = ["B", "KB", "MB", "GB", "TB"]
	const i = Math.min(Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)), units.length - 1)
	return (bytes / Math.pow(1024, i)).toFixed(1) + " " + units[i]
}
const formatRate = (bytes) => formatBytes(bytes) + "/s"

const formatTimeLabel = (date, rangeSeconds) => {
	if (rangeSeconds <= 300) return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
	if (rangeSeconds <= 86400) return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
	return date.toLocaleDateString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

// average raw 1s records into buckets so long ranges stay readable
const aggregateRecords = (records, rangeSeconds) => {
	if (!records?.length) return []
	let bucketSize = 1
	if (rangeSeconds > 300 && rangeSeconds <= 3600) bucketSize = 10
	else if (rangeSeconds > 3600 && rangeSeconds <= 21600) bucketSize = 60
	else if (rangeSeconds > 21600 && rangeSeconds <= 86400) bucketSize = 240
	else if (rangeSeconds > 86400) bucketSize = 1800
	if (bucketSize === 1) return records

	const buckets = []
	for (let i = 0; i < records.length; i += bucketSize) {
		const slice = records.slice(i, i + bucketSize)
		buckets.push({
			ts: slice[Math.floor(slice.length / 2)].ts,
			eg: slice.reduce((s, r) => s + r.eg, 0) / slice.length,
			ig: slice.reduce((s, r) => s + r.ig, 0) / slice.length,
		})
	}
	return buckets
}

// series: [{ data: [{ts, eg, ig}], color, label }]
const MultiGraph = ({ series, dataKey, rangeSeconds, height = 170 }) => {
	const canvasRef = useRef(null)
	const containerRef = useRef(null)

	const globalMax = useMemo(
		() => Math.max(1, ...series.flatMap((s) => s.data.map((d) => d[dataKey]))),
		[series, dataKey],
	)
	const longestSeries = useMemo(
		() => series.reduce((longest, s) => (s.data.length > longest.length ? s.data : longest), []),
		[series],
	)

	useEffect(() => {
		const canvas = canvasRef.current
		const container = containerRef.current
		if (!canvas || !container) return

		const dpr = window.devicePixelRatio || 1
		const width = container.getBoundingClientRect().width
		canvas.width = width * dpr
		canvas.height = height * dpr
		canvas.style.width = width + "px"
		canvas.style.height = height + "px"

		const ctx = canvas.getContext("2d")
		ctx.scale(dpr, dpr)

		const pad = { left: 72, right: 16, top: 12, bottom: 28 }
		const graphWidth = width - pad.left - pad.right
		const graphHeight = height - pad.top - pad.bottom

		ctx.clearRect(0, 0, width, height)
		const textColor = "rgba(128,128,128,0.7)"
		const gridColor = "rgba(128,128,128,0.12)"

		if (longestSeries.length === 0) {
			ctx.fillStyle = textColor
			ctx.font = "11px system-ui, sans-serif"
			ctx.textAlign = "center"
			ctx.fillText("No data", width / 2, height / 2)
			return
		}

		// y grid + labels
		const yTicks = 4
		ctx.font = "10px ui-monospace, monospace"
		ctx.textAlign = "right"
		for (let i = 0; i <= yTicks; i++) {
			const y = pad.top + (graphHeight / yTicks) * i
			ctx.strokeStyle = gridColor
			ctx.beginPath()
			ctx.moveTo(pad.left, y)
			ctx.lineTo(pad.left + graphWidth, y)
			ctx.stroke()
			ctx.fillStyle = textColor
			ctx.fillText(formatRate(globalMax * (1 - i / yTicks)), pad.left - 8, y + 3)
		}

		// x labels
		const xLabelCount = Math.min(longestSeries.length, 5)
		ctx.textAlign = "center"
		ctx.fillStyle = textColor
		for (let i = 0; i < xLabelCount; i++) {
			const idx = Math.floor((i / (xLabelCount - 1)) * (longestSeries.length - 1))
			if (!longestSeries[idx]) continue
			const x = pad.left + (idx / (longestSeries.length - 1)) * graphWidth
			ctx.fillText(formatTimeLabel(new Date(longestSeries[idx].ts), rangeSeconds), x, height - 6)
		}

		for (const s of series) {
			if (s.data.length < 2) continue
			const values = s.data.map((d) => d[dataKey])
			const pointX = (i) => pad.left + (i / (s.data.length - 1)) * graphWidth
			const pointY = (i) => pad.top + graphHeight - (values[i] / globalMax) * graphHeight

			ctx.beginPath()
			ctx.moveTo(pad.left, pad.top + graphHeight)
			values.forEach((_, i) => ctx.lineTo(pointX(i), pointY(i)))
			ctx.lineTo(pad.left + graphWidth, pad.top + graphHeight)
			ctx.closePath()
			ctx.globalAlpha = 0.08
			ctx.fillStyle = s.color
			ctx.fill()
			ctx.globalAlpha = 1

			ctx.beginPath()
			values.forEach((_, i) => (i === 0 ? ctx.moveTo(pointX(i), pointY(i)) : ctx.lineTo(pointX(i), pointY(i))))
			ctx.strokeStyle = s.color
			ctx.lineWidth = 1.5
			ctx.lineJoin = "round"
			ctx.stroke()
		}
	}, [series, dataKey, globalMax, longestSeries, height, rangeSeconds])

	return (
		<div ref={containerRef} className="w-full">
			<canvas ref={canvasRef} className="w-full" />
		</div>
	)
}

// Compact per-tunnel summary bar shown above the charts: identity (tag, server,
// assigned IPv4) and session totals over the selected range.
const TunnelInfoBar = ({ rows }) => {
	if (rows.length === 0) return null
	return (
		<div className="mb-3 flex flex-col gap-1.5">
			{rows.map((r) => (
				<div
					key={r.id}
					className="flex flex-wrap items-center gap-x-5 gap-y-1.5 rounded-box border border-base-300 bg-base-100 px-3 py-2 text-[11px]"
				>
					<div className="flex min-w-0 items-center gap-2">
						<div className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: r.color }} />
						<span className="truncate font-semibold tracking-tight">{r.tag}</span>
					</div>

					<span className="flex items-center gap-1.5">
						<span className="text-[10px] uppercase opacity-40">Server</span>
						<span className="font-mono opacity-80">{r.server}</span>
					</span>
					<span className="flex items-center gap-1.5">
						<span className="text-[10px] uppercase opacity-40">IPv4</span>
						<span className="font-mono opacity-80">{r.ipv4}</span>
					</span>

					<span className="flex items-center gap-3 sm:ml-auto">
						<span className="flex items-center gap-1.5">
							<span className="text-[10px] uppercase opacity-40">Down</span>
							<span className="font-mono opacity-80">{formatBytes(r.totalIg)}</span>
						</span>
						<span className="flex items-center gap-1.5">
							<span className="text-[10px] uppercase opacity-40">Up</span>
							<span className="font-mono opacity-80">{formatBytes(r.totalEg)}</span>
						</span>
					</span>
				</div>
			))}
		</div>
	)
}

const StatsRow = ({ series, dataKey }) => (
	<div className="flex flex-col gap-1.5">
		{series.map((s) => {
			const vals = s.rawData.map((d) => d[dataKey])
			if (vals.length === 0) return null
			const current = vals[vals.length - 1] || 0
			const peak = Math.max(...vals)
			const avg = vals.reduce((a, b) => a + b, 0) / vals.length
			const total = vals.reduce((a, b) => a + b, 0)
			return (
				<div key={s.id} className="flex flex-wrap items-center gap-x-5 gap-y-1 px-1 text-[11px]">
					<div className="flex w-28 shrink-0 items-center gap-1.5">
						<div className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: s.color }} />
						<span className="truncate font-mono opacity-70">{s.label}</span>
					</div>
					{[
						["Cur", formatRate(current)],
						["Avg", formatRate(avg)],
						["Peak", formatRate(peak)],
						["Total", formatBytes(total)],
					].map(([label, value]) => (
						<span key={label}>
							<span className="text-[10px] uppercase opacity-40">{label} </span>
							<span className="font-mono">{value}</span>
						</span>
					))}
				</div>
			)
		})}
	</div>
)

// Live bandwidth charts (1s state polling while mounted). The parent is
// responsible for only rendering this when Config.BandwidthGraphs is on.
const BandwidthCharts = () => {
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const [range, setRange] = useState(TIME_RANGES[0])
	const [disabledTunnels, setDisabledTunnels] = useState({})

	useEffect(() => {
		const interval = setInterval(fetchState, 1000)
		return () => clearInterval(interval)
	}, [])

	const tunnels = activeTunnels || []

	const { series, totalSamples } = useMemo(() => {
		const colors = seriesColors()
		const out = []
		let samples = 0
		tunnels.forEach((tun, idx) => {
			if (disabledTunnels[tun.ID] || !tun.BandwidthHistory?.length) return
			const cutoff = Date.now() - range.seconds * 1000
			const filtered = tun.BandwidthHistory.filter((r) => new Date(r?.ts) >= cutoff)
			samples += filtered.length
			out.push({
				id: tun.ID,
				data: aggregateRecords(filtered, range.seconds),
				rawData: filtered,
				color: colors[idx % colors.length],
				label: tun.CR?.Tag || tun.ID?.slice(0, 8),
			})
		})
		return { series: out, totalSamples: samples }
	}, [tunnels, range, disabledTunnels])

	const infoRows = useMemo(() => {
		const colors = seriesColors()
		const serverMap = Object.fromEntries(servers.map((s) => [s._id, s]))
		const cutoff = Date.now() - range.seconds * 1000
		return tunnels.map((tun, idx) => {
			const records = (tun.BandwidthHistory || []).filter((r) => new Date(r?.ts) >= cutoff)
			return {
				id: tun.ID,
				color: colors[idx % colors.length],
				tag: tun.CR?.Tag || tun.ID?.slice(0, 8),
				server: serverMap[tun.CR?.ServerID]?.Tag || tun.CR?.ServerIP || "—",
				ipv4: tun.CRResponse?.WireGuardIP || "—",
				totalEg: records.reduce((a, r) => a + r.eg, 0),
				totalIg: records.reduce((a, r) => a + r.ig, 0),
			}
		})
	}, [tunnels, servers, range])

	if (tunnels.length === 0) return null

	return (
		<div>
			<Toolbar>
				<div className="flex gap-1">
						{TIME_RANGES.map((r) => (
							<button
								key={r.label}
								className={"btn btn-xs " + (range.label === r.label ? "btn-primary" : "btn-ghost")}
								onClick={() => setRange(r)}
							>
								{r.label}
							</button>
						))}
					</div>

					{tunnels.length > 1 && (
						<div className="flex items-center gap-1.5">
							{tunnels.map((tun, idx) => {
								const colors = seriesColors()
								const color = colors[idx % colors.length]
								const disabled = disabledTunnels[tun.ID]
								return (
									<button
										key={tun.ID}
										className={"btn btn-ghost btn-xs gap-1.5 " + (disabled ? "opacity-40" : "")}
										onClick={() => setDisabledTunnels((p) => ({ ...p, [tun.ID]: !p[tun.ID] }))}
									>
										<div className="h-2 w-2 rounded-full" style={{ backgroundColor: color, opacity: disabled ? 0.2 : 1 }} />
										<span className={disabled ? "line-through" : ""}>{tun.CR?.Tag || tun.ID?.slice(0, 8)}</span>
									</button>
								)
							})}
						</div>
					)}

				<span className="ml-auto font-mono text-[10px] opacity-50">
					{totalSamples} sample{totalSamples !== 1 ? "s" : ""}
				</span>
			</Toolbar>

			<TunnelInfoBar rows={infoRows} />

			<div className="grid grid-cols-1 gap-4 2xl:grid-cols-2">
				{[
					{ key: "eg", label: "Upload", dot: "bg-warning" },
					{ key: "ig", label: "Download", dot: "bg-success" },
				].map(({ key, label, dot }) => (
					<Card
						key={key}
						title={
							<span className="flex items-center gap-2">
								<span className={"h-1.5 w-1.5 rounded-full " + dot} />
								{label}
							</span>
						}
					>
						<MultiGraph series={series} dataKey={key} rangeSeconds={range.seconds} />
						<div className="mt-2.5 border-t border-base-300 pt-2">
							<StatsRow series={series} dataKey={key} />
						</div>
					</Card>
				))}
			</div>
		</div>
	)
}

export default BandwidthCharts
