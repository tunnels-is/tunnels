import { useState, useEffect, useMemo, useRef } from "react"
import GLOBAL_STATE from "@/state"

const TIME_RANGES = [
  { label: "1m", seconds: 60 },
  { label: "5m", seconds: 300 },
  { label: "15m", seconds: 900 },
  { label: "1h", seconds: 3600 },
  { label: "24h", seconds: 86400 },
  { label: "7d", seconds: 604800 },
]

function formatBytes(bytes) {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(1) + " " + units[idx]
}

function formatBytesPerSec(bytes) {
  return formatBytes(bytes) + "/s"
}

function formatTimeLabel(date, rangeSeconds) {
  if (rangeSeconds <= 300) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
  }
  if (rangeSeconds <= 3600) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  }
  if (rangeSeconds <= 86400) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  }
  return date.toLocaleDateString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

function aggregateRecords(records, rangeSeconds) {
  if (!records || records.length === 0) return []

  let bucketSize = 1
  if (rangeSeconds > 300 && rangeSeconds <= 3600) bucketSize = 10
  else if (rangeSeconds > 3600 && rangeSeconds <= 21600) bucketSize = 60
  else if (rangeSeconds > 21600 && rangeSeconds <= 86400) bucketSize = 240
  else if (rangeSeconds > 86400) bucketSize = 1800

  if (bucketSize === 1) return records

  const buckets = []
  for (let i = 0; i < records.length; i += bucketSize) {
    const slice = records.slice(i, i + bucketSize)
    const avgEg = slice.reduce((s, r) => s + r.eg, 0) / slice.length
    const avgIg = slice.reduce((s, r) => s + r.ig, 0) / slice.length
    buckets.push({
      ts: slice[Math.floor(slice.length / 2)].ts,
      eg: avgEg,
      ig: avgIg,
    })
  }
  return buckets
}

function Graph({ data, dataKey, color, label, rangeSeconds, height = 180 }) {
  const canvasRef = useRef(null)
  const containerRef = useRef(null)

  const values = useMemo(() => data.map((d) => d[dataKey]), [data, dataKey])
  const maxVal = useMemo(() => Math.max(...values, 1), [values])

  useEffect(() => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return

    const dpr = window.devicePixelRatio || 1
    const rect = container.getBoundingClientRect()
    const width = rect.width

    canvas.width = width * dpr
    canvas.height = height * dpr
    canvas.style.width = width + "px"
    canvas.style.height = height + "px"

    const ctx = canvas.getContext("2d")
    ctx.scale(dpr, dpr)

    const paddingLeft = 72
    const paddingRight = 16
    const paddingTop = 12
    const paddingBottom = 28
    const graphWidth = width - paddingLeft - paddingRight
    const graphHeight = height - paddingTop - paddingBottom

    // Clear & fill background
    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = "#060810"
    ctx.fillRect(0, 0, width, height)

    const textColor = "rgba(255,255,255,0.30)"
    const gridColor = "rgba(255,255,255,0.04)"
    const areaColor = color + "12"

    if (data.length === 0) {
      ctx.fillStyle = "rgba(255,255,255,0.25)"
      ctx.font = "11px system-ui, -apple-system, sans-serif"
      ctx.textAlign = "center"
      ctx.fillText("No data", width / 2, height / 2)
      return
    }

    // Y-axis grid + labels
    const yTicks = 4
    ctx.font = "10px ui-monospace, SFMono-Regular, monospace"
    ctx.textAlign = "right"
    for (let i = 0; i <= yTicks; i++) {
      const y = paddingTop + (graphHeight / yTicks) * i
      const val = maxVal * (1 - i / yTicks)

      ctx.strokeStyle = gridColor
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(paddingLeft, y)
      ctx.lineTo(paddingLeft + graphWidth, y)
      ctx.stroke()

      ctx.fillStyle = textColor
      ctx.fillText(formatBytesPerSec(val), paddingLeft - 8, y + 3)
    }

    // X-axis labels
    const xLabelCount = Math.min(data.length, 5)
    ctx.textAlign = "center"
    ctx.fillStyle = textColor
    ctx.font = "10px ui-monospace, SFMono-Regular, monospace"
    for (let i = 0; i < xLabelCount; i++) {
      const idx = Math.floor((i / (xLabelCount - 1)) * (data.length - 1))
      if (!data[idx]) continue
      const x = paddingLeft + (idx / (data.length - 1)) * graphWidth
      const date = new Date(data[idx].ts)
      ctx.fillText(formatTimeLabel(date, rangeSeconds), x, height - 6)
    }

    if (data.length < 2) return

    // Area fill
    ctx.beginPath()
    ctx.moveTo(paddingLeft, paddingTop + graphHeight)
    for (let i = 0; i < data.length; i++) {
      const x = paddingLeft + (i / (data.length - 1)) * graphWidth
      const y = paddingTop + graphHeight - (values[i] / maxVal) * graphHeight
      ctx.lineTo(x, y)
    }
    ctx.lineTo(paddingLeft + graphWidth, paddingTop + graphHeight)
    ctx.closePath()
    ctx.fillStyle = areaColor
    ctx.fill()

    // Line
    ctx.beginPath()
    for (let i = 0; i < data.length; i++) {
      const x = paddingLeft + (i / (data.length - 1)) * graphWidth
      const y = paddingTop + graphHeight - (values[i] / maxVal) * graphHeight
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    }
    ctx.strokeStyle = color
    ctx.lineWidth = 1.5
    ctx.lineJoin = "round"
    ctx.stroke()
  }, [data, dataKey, values, maxVal, color, label, height, rangeSeconds])

  return (
    <div ref={containerRef} className="w-full">
      <canvas ref={canvasRef} className="w-full rounded-md" />
    </div>
  )
}

function StatsRow({ data, dataKey }) {
  const vals = useMemo(() => data.map((d) => d[dataKey]), [data, dataKey])
  if (vals.length === 0) return null

  const current = vals[vals.length - 1] || 0
  const max = Math.max(...vals)
  const avg = vals.reduce((a, b) => a + b, 0) / vals.length
  const total = vals.reduce((a, b) => a + b, 0)

  const items = [
    { label: "Current", value: formatBytesPerSec(current) },
    { label: "Average", value: formatBytesPerSec(avg) },
    { label: "Peak", value: formatBytesPerSec(max) },
    { label: "Total", value: formatBytes(total) },
  ]

  return (
    <div className="flex items-center gap-6 px-1">
      {items.map((it) => (
        <div key={it.label} className="flex items-center gap-1.5">
          <span className="text-[10px] uppercase tracking-wider text-white/35">{it.label}</span>
          <span className="text-[11px] font-mono text-white/60">{it.value}</span>
        </div>
      ))}
    </div>
  )
}

export default function Bandwidth() {
  const state = GLOBAL_STATE("bandwidth")

  const [range_, setRange] = useState(TIME_RANGES[0])
  const [records, setRecords] = useState([])

  const getTunnel = () => {
    if (!state.ActiveTunnels || state.ActiveTunnels.length === 0) return null
    return state.ActiveTunnels[0]
  }

  useEffect(() => {
    const interval = setInterval(() => {
      state.GetBackendState()
    }, 1_000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (!getTunnel()?.BandwidthHistory) {
      setRecords([])
      return
    }
    const now = new Date()
    const cutoff = new Date(now.getTime() - range_.seconds * 1000)
    const filtered = getTunnel().BandwidthHistory.filter((r) => {
      return new Date(r?.ts) >= cutoff
    })
    setRecords(filtered)
  }, [state.ActiveTunnels, range_])

  const aggregated = useMemo(
    () => aggregateRecords(records, range_.seconds),
    [records, range_.seconds]
  )

  if (!getTunnel()) {
    return (
      <div className="flex items-center justify-center h-40 text-white/40 text-[13px]">
        No active tunnel
      </div>
    )
  }

  return (
    <div>
      {/* Header bar — matches DNS/Settings style */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-6">
        <div className="flex gap-1">
          {TIME_RANGES.map((r) => (
            <button
              key={r.label}
              onClick={() => setRange(r)}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                range_.label === r.label
                  ? "bg-white/[0.07] text-white/70"
                  : "text-white/40 hover:text-white/60"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
        <div className="ml-auto flex items-center gap-3">
          <span className="text-[10px] text-white/35 tabular-nums">
            {records.length} sample{records.length !== 1 ? "s" : ""}
          </span>
        </div>
      </div>

      {/* Egress */}
      <div className="mb-5">
        <div className="flex items-center gap-2 mb-2 pl-1">
          <div className="w-1.5 h-1.5 rounded-full bg-amber-500/80" />
          <span className="text-[11px] uppercase tracking-widest text-white/45">Upload</span>
        </div>
        <div className="rounded-lg bg-[#0a0d14] border border-[#1e2433] p-3">
          <Graph
            data={aggregated}
            dataKey="eg"
            color="#f59e0b"
            label="Egress"
            rangeSeconds={range_.seconds}
            height={170}
          />
          <div className="mt-2.5 pt-2 border-t border-[#1e2433]">
            <StatsRow data={records} dataKey="eg" />
          </div>
        </div>
      </div>

      {/* Ingress */}
      <div>
        <div className="flex items-center gap-2 mb-2 pl-1">
          <div className="w-1.5 h-1.5 rounded-full bg-[#4B7BF5]/80" />
          <span className="text-[11px] uppercase tracking-widest text-white/45">Download</span>
        </div>
        <div className="rounded-lg bg-[#0a0d14] border border-[#1e2433] p-3">
          <Graph
            data={aggregated}
            dataKey="ig"
            color="#4B7BF5"
            label="Ingress"
            rangeSeconds={range_.seconds}
            height={170}
          />
          <div className="mt-2.5 pt-2 border-t border-[#1e2433]">
            <StatsRow data={records} dataKey="ig" />
          </div>
        </div>
      </div>
    </div>
  )
}