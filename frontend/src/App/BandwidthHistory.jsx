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

// Rotating palette for multiple tunnels
const TUNNEL_COLORS = [
  "#1d4ed8", // blue (site accent)
  "#f59e0b", // amber
  "#10b981", // emerald
  "#f43f5e", // rose
  "#a855f7", // purple
  "#06b6d4", // cyan
  "#ec4899", // pink
  "#84cc16", // lime
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

// series: [{ data: [{ts, eg, ig}], color: string, label: string }]
function MultiGraph({ series, dataKey, rangeSeconds, height = 180 }) {
  const canvasRef = useRef(null)
  const containerRef = useRef(null)

  // Compute global max across all series
  const globalMax = useMemo(() => {
    let max = 1
    for (const s of series) {
      for (const d of s.data) {
        if (d[dataKey] > max) max = d[dataKey]
      }
    }
    return max
  }, [series, dataKey])

  // Find the series with the most data points (for x-axis labels)
  const longestSeries = useMemo(() => {
    let longest = []
    for (const s of series) {
      if (s.data.length > longest.length) longest = s.data
    }
    return longest
  }, [series])

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

    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = "#fdfcf8"
    ctx.fillRect(0, 0, width, height)

    const textColor = "rgba(255,255,255,0.30)"
    const gridColor = "rgba(255,255,255,0.04)"

    if (longestSeries.length === 0) {
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
      const val = globalMax * (1 - i / yTicks)

      ctx.strokeStyle = gridColor
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(paddingLeft, y)
      ctx.lineTo(paddingLeft + graphWidth, y)
      ctx.stroke()

      ctx.fillStyle = textColor
      ctx.fillText(formatBytesPerSec(val), paddingLeft - 8, y + 3)
    }

    // X-axis labels (use longest series for time reference)
    const xLabelCount = Math.min(longestSeries.length, 5)
    ctx.textAlign = "center"
    ctx.fillStyle = textColor
    ctx.font = "10px ui-monospace, SFMono-Regular, monospace"
    for (let i = 0; i < xLabelCount; i++) {
      const idx = Math.floor((i / (xLabelCount - 1)) * (longestSeries.length - 1))
      if (!longestSeries[idx]) continue
      const x = paddingLeft + (idx / (longestSeries.length - 1)) * graphWidth
      const date = new Date(longestSeries[idx].ts)
      ctx.fillText(formatTimeLabel(date, rangeSeconds), x, height - 6)
    }

    // Draw each series
    for (const s of series) {
      if (s.data.length < 2) continue

      const values = s.data.map((d) => d[dataKey])

      // Area fill
      ctx.beginPath()
      ctx.moveTo(paddingLeft, paddingTop + graphHeight)
      for (let i = 0; i < s.data.length; i++) {
        const x = paddingLeft + (i / (s.data.length - 1)) * graphWidth
        const y = paddingTop + graphHeight - (values[i] / globalMax) * graphHeight
        ctx.lineTo(x, y)
      }
      ctx.lineTo(paddingLeft + graphWidth, paddingTop + graphHeight)
      ctx.closePath()
      ctx.fillStyle = s.color + "0a"
      ctx.fill()

      // Line
      ctx.beginPath()
      for (let i = 0; i < s.data.length; i++) {
        const x = paddingLeft + (i / (s.data.length - 1)) * graphWidth
        const y = paddingTop + graphHeight - (values[i] / globalMax) * graphHeight
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.strokeStyle = s.color
      ctx.lineWidth = 1.5
      ctx.lineJoin = "round"
      ctx.stroke()
    }
  }, [series, dataKey, globalMax, longestSeries, height, rangeSeconds])

  return (
    <div ref={containerRef} className="w-full">
      <canvas ref={canvasRef} className="w-full rounded-md" />
    </div>
  )
}

function MultiStatsRow({ series, dataKey }) {
  if (series.length === 0) return null

  return (
    <div className="flex flex-col gap-1.5">
      {series.map((s) => {
        const vals = s.rawData.map((d) => d[dataKey])
        if (vals.length === 0) return null

        const current = vals[vals.length - 1] || 0
        const max = Math.max(...vals)
        const avg = vals.reduce((a, b) => a + b, 0) / vals.length
        const total = vals.reduce((a, b) => a + b, 0)

        return (
          <div key={s.id} className="flex items-center gap-4 px-1">
            <div className="flex items-center gap-1.5 shrink-0 w-28">
              <div className="w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: s.color }} />
              <span className="text-[10px] font-mono text-[#525252] truncate">{s.label}</span>
            </div>
            <div className="flex items-center gap-5">
              <div className="flex items-center gap-1">
                <span className="label !mb-0">Cur</span>
                <span className="text-[11px] font-mono text-[#525252]">{formatBytesPerSec(current)}</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="label !mb-0">Avg</span>
                <span className="text-[11px] font-mono text-[#525252]">{formatBytesPerSec(avg)}</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="label !mb-0">Peak</span>
                <span className="text-[11px] font-mono text-[#525252]">{formatBytesPerSec(max)}</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="label !mb-0">Total</span>
                <span className="text-[11px] font-mono text-[#525252]">{formatBytes(total)}</span>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export default function Bandwidth() {
  const state = GLOBAL_STATE("bandwidth")

  const [range_, setRange] = useState(TIME_RANGES[0])
  const [disabledTunnels, setDisabledTunnels] = useState({})

  const tunnels = state.ActiveTunnels || []

  useEffect(() => {
    const interval = setInterval(() => {
      state.GetBackendState()
    }, 1_000)
    return () => clearInterval(interval)
  }, [])

  const toggleTunnel = (id) => {
    setDisabledTunnels((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  // build per tunnel filtered + aggregated series
  const { egressSeries, ingressSeries, totalSamples } = useMemo(() => {
    const eg = []
    const ig = []
    let samples = 0

    tunnels.forEach((tun, idx) => {
      if (disabledTunnels[tun.ID]) return
      if (!tun.BandwidthHistory || tun.BandwidthHistory.length === 0) return

      const now = new Date()
      const cutoff = new Date(now.getTime() - range_.seconds * 1000)
      const filtered = tun.BandwidthHistory.filter((r) => new Date(r?.ts) >= cutoff)
      const aggregated = aggregateRecords(filtered, range_.seconds)
      const color = TUNNEL_COLORS[idx % TUNNEL_COLORS.length]
      const label = tun.CR?.Tag || tun.ID?.slice(0, 8)

      samples += filtered.length

      eg.push({ id: tun.ID, data: aggregated, rawData: filtered, color, label })
      ig.push({ id: tun.ID, data: aggregated, rawData: filtered, color, label })
    })

    return { egressSeries: eg, ingressSeries: ig, totalSamples: samples }
  }, [tunnels, range_, disabledTunnels])

  if (!state.Config.BandwidthGraphs) { 
    return (
      <div className="flex items-center justify-center h-40 text-[#a3a3a3] text-[13px]">
        Bandwidth history is disabled
      </div>
    )
  }
  if (tunnels.length === 0) {
    return (
      <div className="flex items-center justify-center h-40 text-[#a3a3a3] text-[13px]">
        No active tunnels
      </div>
    )
  }

  return (
    <div>
      {/* Header bar */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#ffffff]/80 border border-[#e7e3d7] mb-6 card-shadow">
        {/* Time range tabs */}
        <div className="flex gap-1">
          {TIME_RANGES.map((r) => (
            <button
              key={r.label}
              onClick={() => setRange(r)}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                range_.label === r.label
                  ? "bg-black/[0.05] text-[#0a0a0a]"
                  : "text-[#a3a3a3] hover:text-[#525252]"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>

        {/* Separator */}
        {tunnels.length > 1 && <div className="w-px h-5 bg-black/[0.06]" />}

        {/* Tunnel toggles */}
        {tunnels.length > 1 && (
          <div className="flex items-center gap-1.5">
            {tunnels.map((tun, idx) => {
              const color = TUNNEL_COLORS[idx % TUNNEL_COLORS.length]
              const disabled = disabledTunnels[tun.ID]
              const label = tun.CR?.Tag || tun.ID?.slice(0, 8)
              return (
                <button
                  key={tun.ID}
                  onClick={() => toggleTunnel(tun.ID)}
                  className={`flex items-center gap-1.5 text-[11px] px-2 py-0.5 rounded transition-colors ${
                    disabled
                      ? "text-[#d5d0c0] hover:text-[#a3a3a3]"
                      : "text-[#525252] hover:text-[#0a0a0a] bg-black/[0.04]"
                  }`}
                >
                  <div
                    className="w-2 h-2 rounded-full shrink-0 transition-opacity"
                    style={{
                      backgroundColor: color,
                      opacity: disabled ? 0.2 : 1,
                    }}
                  />
                  <span className={disabled ? "line-through" : ""}>{label}</span>
                </button>
              )
            })}
          </div>
        )}

        <div className="ml-auto flex items-center gap-3">
          <span className="text-[10px] text-[#a3a3a3] tabular-nums">
            {totalSamples} sample{totalSamples !== 1 ? "s" : ""}
          </span>
        </div>
      </div>

      {/* Egress */}
      <div className="mb-5">
        <div className="flex items-center gap-2 mb-2 pl-1">
          <div className="w-1.5 h-1.5 rounded-full bg-[#b45309]" />
          <span className="label-section">Upload</span>
        </div>
        <div className="rounded-lg bg-[#ffffff] border border-[#e7e3d7] p-3 card-shadow">
          <MultiGraph
            series={egressSeries}
            dataKey="eg"
            rangeSeconds={range_.seconds}
            height={170}
          />
          <div className="mt-2.5 pt-2 border-t border-[#e7e3d7]">
            <MultiStatsRow series={egressSeries} dataKey="eg" />
          </div>
        </div>
      </div>

      {/* Ingress */}
      <div>
        <div className="flex items-center gap-2 mb-2 pl-1">
          <div className="w-1.5 h-1.5 rounded-full bg-[#1d4ed8]/80" />
          <span className="label-section">Download</span>
        </div>
        <div className="rounded-lg bg-[#ffffff] border border-[#e7e3d7] p-3 card-shadow">
          <MultiGraph
            series={ingressSeries}
            dataKey="ig"
            rangeSeconds={range_.seconds}
            height={170}
          />
          <div className="mt-2.5 pt-2 border-t border-[#e7e3d7]">
            <MultiStatsRow series={ingressSeries} dataKey="ig" />
          </div>
        </div>
      </div>
    </div>
  )
}