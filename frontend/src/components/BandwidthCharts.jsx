import { useEffect, useMemo, useRef, useState } from "react"
import { ArrowDown, ArrowUp, Activity } from "lucide-react"
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

const DOWNLOAD_COLOR = "#22c55e"
const UPLOAD_COLOR = "#f59e0b"

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

const summarize = (records, key) => {
	if (!records?.length) return { current: 0, avg: 0, peak: 0, total: 0 }
	const vals = records.map((d) => d[key])
	const total = vals.reduce((a, b) => a + b, 0)
	return {
		current: vals[vals.length - 1] || 0,
		avg: total / vals.length,
		peak: vals.reduce((m, v) => (v > m ? v : m), 0),
		total,
	}
}

/** Dual-line chart: download + upload for a single tunnel */
const DualRateGraph = ({ data, rangeSeconds, height = 160 }) => {
	const canvasRef = useRef(null)
	const containerRef = useRef(null)

	const globalMax = useMemo(
		() => Math.max(1, ...data.flatMap((d) => [d.ig, d.eg])),
		[data],
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
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

		const pad = { left: 64, right: 12, top: 10, bottom: 26 }
		const graphWidth = Math.max(1, width - pad.left - pad.right)
		const graphHeight = height - pad.top - pad.bottom

		ctx.clearRect(0, 0, width, height)
		const textColor = "rgba(128,128,128,0.65)"
		const gridColor = "rgba(128,128,128,0.10)"

		if (data.length === 0) {
			ctx.fillStyle = textColor
			ctx.font = "11px system-ui, sans-serif"
			ctx.textAlign = "center"
			ctx.fillText("No data yet", width / 2, height / 2)
			return
		}

		// Y grid + labels
		const yTicks = 4
		ctx.font = "10px ui-monospace, monospace"
		ctx.textAlign = "right"
		for (let i = 0; i <= yTicks; i++) {
			const y = pad.top + (graphHeight / yTicks) * i
			ctx.strokeStyle = gridColor
			ctx.lineWidth = 1
			ctx.beginPath()
			ctx.moveTo(pad.left, y)
			ctx.lineTo(pad.left + graphWidth, y)
			ctx.stroke()
			ctx.fillStyle = textColor
			ctx.fillText(formatRate(globalMax * (1 - i / yTicks)), pad.left - 8, y + 3)
		}

		// X labels
		const xLabelCount = Math.min(data.length, 5)
		ctx.textAlign = "center"
		ctx.fillStyle = textColor
		for (let i = 0; i < xLabelCount; i++) {
			const idx = xLabelCount === 1 ? 0 : Math.floor((i / (xLabelCount - 1)) * (data.length - 1))
			if (!data[idx]) continue
			const x = pad.left + (data.length === 1 ? graphWidth / 2 : (idx / (data.length - 1)) * graphWidth)
			ctx.fillText(formatTimeLabel(new Date(data[idx].ts), rangeSeconds), x, height - 6)
		}

		const pointX = (i) =>
			data.length === 1 ? pad.left + graphWidth / 2 : pad.left + (i / (data.length - 1)) * graphWidth
		const pointY = (v) => pad.top + graphHeight - (v / globalMax) * graphHeight

		const drawSeries = (key, color) => {
			if (data.length < 2) {
				// Single point: draw a soft marker
				const x = pointX(0)
				const y = pointY(data[0][key])
				ctx.beginPath()
				ctx.arc(x, y, 3, 0, Math.PI * 2)
				ctx.fillStyle = color
				ctx.fill()
				return
			}

			// Fill under curve
			ctx.beginPath()
			ctx.moveTo(pad.left, pad.top + graphHeight)
			data.forEach((d, i) => ctx.lineTo(pointX(i), pointY(d[key])))
			ctx.lineTo(pad.left + graphWidth, pad.top + graphHeight)
			ctx.closePath()
			const grad = ctx.createLinearGradient(0, pad.top, 0, pad.top + graphHeight)
			grad.addColorStop(0, color + "28")
			grad.addColorStop(1, color + "00")
			ctx.fillStyle = grad
			ctx.fill()

			// Stroke
			ctx.beginPath()
			data.forEach((d, i) => (i === 0 ? ctx.moveTo(pointX(i), pointY(d[key])) : ctx.lineTo(pointX(i), pointY(d[key]))))
			ctx.strokeStyle = color
			ctx.lineWidth = 1.75
			ctx.lineJoin = "round"
			ctx.lineCap = "round"
			ctx.stroke()
		}

		// Download first (under), upload on top
		drawSeries("ig", DOWNLOAD_COLOR)
		drawSeries("eg", UPLOAD_COLOR)
	}, [data, globalMax, height, rangeSeconds])

	return (
		<div ref={containerRef} className="w-full">
			<canvas ref={canvasRef} className="w-full" />
		</div>
	)
}

const StatChip = ({ label, value }) => (
	<div className="flex flex-col gap-0.5">
		<span className="text-[9px] font-medium uppercase tracking-wider text-base-content/35">{label}</span>
		<span className="font-mono text-[11px] tabular-nums text-base-content/80">{value}</span>
	</div>
)

const RatePill = ({ direction, rate }) => {
	const isDown = direction === "down"
	const Icon = isDown ? ArrowDown : ArrowUp
	return (
		<div
			className={
				"inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-mono text-[11px] tabular-nums " +
				(isDown ? "bg-success/10 text-success" : "bg-warning/10 text-warning")
			}
		>
			<Icon size={12} strokeWidth={2.5} />
			{formatRate(rate)}
		</div>
	)
}

const TunnelCard = ({ tunnel, server, range, nested = false }) => {
	const rawData = useMemo(() => {
		const cutoff = Date.now() - range.seconds * 1000
		return (tunnel.BandwidthHistory || []).filter((r) => new Date(r?.ts) >= cutoff)
	}, [tunnel.BandwidthHistory, range.seconds])
	const data = useMemo(() => aggregateRecords(rawData, range.seconds), [rawData, range.seconds])
	const down = useMemo(() => summarize(rawData, "ig"), [rawData])
	const up = useMemo(() => summarize(rawData, "eg"), [rawData])

	const tag = tunnel.CR?.Tag || tunnel.ID?.slice(0, 8) || "Tunnel"
	const serverLabel = server?.Tag || tunnel.CR?.ServerIP || "—"
	const ipv4 = tunnel.CRResponse?.WireGuardIP || "—"
	const country = server?.Country

	return (
		<div
			className={
				"overflow-hidden bg-base-100 " +
				(nested
					? "rounded-box border border-base-300 shadow-sm transition-shadow hover:shadow-md"
					: "")
			}
		>
			{/* Identity + live rates */}
			<div className="flex flex-wrap items-start gap-3 border-b border-base-200 px-4 py-3.5 sm:items-center">
				<div className="min-w-0 flex-1">
					<div className="flex flex-wrap items-center gap-2">
						<span className="inline-block h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-success" />
						<span className="truncate text-[13px] font-semibold tracking-tight">{tag}</span>
						{country && (
							<span className="rounded-md bg-base-200/80 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-base-content/50">
								{country}
							</span>
						)}
					</div>
					<div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-base-content/50">
						<span className="flex items-center gap-1.5">
							<span className="text-[9px] font-medium uppercase tracking-wider text-base-content/35">Server</span>
							<span className="font-mono text-base-content/70">{serverLabel}</span>
						</span>
						<span className="hidden h-3 w-px bg-base-300 sm:inline-block" />
						<span className="flex items-center gap-1.5">
							<span className="text-[9px] font-medium uppercase tracking-wider text-base-content/35">IPv4</span>
							<span className="font-mono text-base-content/70">{ipv4}</span>
						</span>
					</div>
				</div>

				<div className="flex shrink-0 items-center gap-2">
					<RatePill direction="down" rate={down.current} />
					<RatePill direction="up" rate={up.current} />
				</div>
			</div>

			{/* Chart */}
			<div className="bg-gradient-to-b from-base-100 to-base-200/30 px-2 pb-1 pt-3 sm:px-3">
				<div className="mb-1 flex items-center justify-end gap-3 px-2">
					<span className="flex items-center gap-1.5 text-[10px] text-base-content/45">
						<span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: DOWNLOAD_COLOR }} />
						Download
					</span>
					<span className="flex items-center gap-1.5 text-[10px] text-base-content/45">
						<span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: UPLOAD_COLOR }} />
						Upload
					</span>
				</div>
				<DualRateGraph data={data} rangeSeconds={range.seconds} />
			</div>

			{/* Stats footer — both directions, one cohesive strip */}
			<div className="grid grid-cols-1 gap-px border-t border-base-200 bg-base-200/60 sm:grid-cols-2">
				<div className="bg-base-100 px-4 py-3">
					<div className="mb-2 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-success/80">
						<ArrowDown size={11} strokeWidth={2.5} />
						Download
					</div>
					<div className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
						<StatChip label="Cur" value={formatRate(down.current)} />
						<StatChip label="Avg" value={formatRate(down.avg)} />
						<StatChip label="Peak" value={formatRate(down.peak)} />
						<StatChip label="Total" value={formatBytes(down.total)} />
					</div>
				</div>
				<div className="bg-base-100 px-4 py-3">
					<div className="mb-2 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-warning/80">
						<ArrowUp size={11} strokeWidth={2.5} />
						Upload
					</div>
					<div className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
						<StatChip label="Cur" value={formatRate(up.current)} />
						<StatChip label="Avg" value={formatRate(up.avg)} />
						<StatChip label="Peak" value={formatRate(up.peak)} />
						<StatChip label="Total" value={formatBytes(up.total)} />
					</div>
				</div>
			</div>
		</div>
	)
}

const BandwidthCharts = () => {
	const activeTunnels = useStore((s) => s.activeTunnels)
	const servers = useStore((s) => s.servers)
	const [range, setRange] = useState(TIME_RANGES[0])

	useEffect(() => {
		const interval = setInterval(fetchState, 1000)
		return () => clearInterval(interval)
	}, [])

	const tunnels = activeTunnels || []
	const serverMap = useMemo(() => Object.fromEntries(servers.map((s) => [s._id, s])), [servers])

	const totalSamples = useMemo(() => {
		const cutoff = Date.now() - range.seconds * 1000
		return tunnels.reduce((n, tun) => {
			const hist = tun.BandwidthHistory || []
			return n + hist.filter((r) => new Date(r?.ts) >= cutoff).length
		}, 0)
	}, [tunnels, range])

	if (tunnels.length === 0) return null

	return (
		<div className="mt-4 overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm">
			{/* Shared toolbar */}
			<div className="flex flex-wrap items-center gap-3 border-b border-base-200 bg-base-200/40 px-4 py-2.5">
				<div className="flex items-center gap-2">
					<Activity size={14} className="text-base-content/40" />
					<span className="text-[13px] font-semibold tracking-tight">Bandwidth</span>
					{tunnels.length > 1 && (
						<span className="rounded-full bg-base-300/80 px-2 py-0.5 text-[10px] font-medium text-base-content/50">
							{tunnels.length} tunnels
						</span>
					)}
				</div>

				<div className="flex items-center gap-0.5 rounded-lg bg-base-100 p-0.5 ring-1 ring-base-300">
					{TIME_RANGES.map((r) => (
						<button
							key={r.label}
							className={
								"rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors " +
								(range.label === r.label
									? "bg-primary text-primary-content shadow-sm"
									: "text-base-content/50 hover:bg-base-200 hover:text-base-content/80")
							}
							onClick={() => setRange(r)}
						>
							{r.label}
						</button>
					))}
				</div>

				<span className="ml-auto font-mono text-[10px] tabular-nums text-base-content/40">
					{totalSamples} sample{totalSamples !== 1 ? "s" : ""}
				</span>
			</div>

			{/* Per-tunnel panels — nested cards when multiple so each connection reads as a unit */}
			{tunnels.length === 1 ? (
				<TunnelCard tunnel={tunnels[0]} server={serverMap[tunnels[0].CR?.ServerID]} range={range} />
			) : (
				<div className="grid gap-3 p-3 xl:grid-cols-2">
					{tunnels.map((tun) => (
						<TunnelCard
							key={tun.ID}
							tunnel={tun}
							server={serverMap[tun.CR?.ServerID]}
							range={range}
							nested
						/>
					))}
				</div>
			)}
		</div>
	)
}

export default BandwidthCharts
