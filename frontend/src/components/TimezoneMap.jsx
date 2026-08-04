import { useMemo } from "react"
import world from "@svg-maps/world"
import { countryName, normalizeCountryCode } from "@/lib/countries"
import { countriesAtOffset, resolveDisplayTimezone, timezoneOffsetMinutes } from "@/lib/geo"

// Inline fills — Tailwind fill-* / opacity modifiers are unreliable on SVG paths
// with daisyUI theme tokens across light/dark.
const FILL = {
	base: "color-mix(in oklab, var(--color-base-content) 14%, transparent)",
	zone: "color-mix(in oklab, var(--color-info) 32%, transparent)",
	user: "color-mix(in oklab, var(--color-info) 72%, transparent)",
	match: "color-mix(in oklab, var(--color-success) 78%, transparent)",
	connected: "var(--color-success)",
}

const TimezoneMap = ({
	timezone,
	userCountry,
	matchedServerCountry,
	serverCountry,
	pickedServer,
	connecting,
	probing,
	connected,
	onConnect,
	onDisconnect,
}) => {
	const tz = resolveDisplayTimezone(timezone)
	const offset = timezoneOffsetMinutes(tz)
	const hours = Math.abs(offset / 60)
	const offsetLabel =
		"UTC" + (offset === 0 ? "" : (offset > 0 ? "+" : "-") + (Number.isInteger(hours) ? hours : hours.toFixed(1)))
	const userCC = normalizeCountryCode(userCountry)?.toLowerCase() || ""
	const matchCC = normalizeCountryCode(matchedServerCountry)?.toLowerCase() || ""
	const scc = normalizeCountryCode(serverCountry || pickedServer?.country)?.toLowerCase() || ""

	// When we cannot pin a country, paint every country that shares this UTC offset.
	const zoneCountries = useMemo(() => {
		if (userCC) return null
		return countriesAtOffset(offset)
	}, [userCC, offset])

	const fillFor = (id) => {
		const lid = (id || "").toLowerCase()
		if (scc && lid === scc) return FILL.connected
		if (matchCC && lid === matchCC) return FILL.match
		if (userCC && lid === userCC) return FILL.user
		if (zoneCountries?.has(lid)) return FILL.zone
		return FILL.base
	}

	const displayServer = pickedServer || (connected && serverCountry ? { country: serverCountry } : null)

	const closestLabel = () => {
		if (!displayServer?.tag && !displayServer?.country) {
			return <span className="opacity-40">—</span>
		}
		const code = normalizeCountryCode(displayServer.country)
		const codeLower = code?.toLowerCase()
		return (
			<div className="min-w-0">
				<div className="flex flex-wrap items-center gap-2">
					{codeLower && (
						<span className={`fi fi-${codeLower} rounded-[2px]`} title={countryName(displayServer.country)} />
					)}
					{displayServer.tag ? (
						<span className="truncate">{displayServer.tag}</span>
					) : (
						<span>{countryName(displayServer.country) || displayServer.country}</span>
					)}
				</div>
				{(displayServer.country || displayServer.latencyMS != null || displayServer.ip) && (
					<div className="mt-0.5 flex flex-wrap items-center gap-x-2 font-mono text-[11px] font-normal opacity-50">
						{displayServer.tag && displayServer.country && (
							<span>{countryName(displayServer.country) || displayServer.country}</span>
						)}
						{displayServer.latencyMS != null && <span>{displayServer.latencyMS}ms</span>}
						{displayServer.ip && <span>{displayServer.ip}</span>}
					</div>
				)}
			</div>
		)
	}

	const locationLabel = () => {
		if (userCountry) {
			return (
				<>
					<span className={`fi fi-${userCC} rounded-[2px]`} title={countryName(userCountry)} />
					{countryName(userCountry)}
				</>
			)
		}
		// No country for this zone — show timezone name, not a wrong country guess.
		return <span className="truncate">{tz || "—"}</span>
	}

	return (
		<div className="relative mb-4 w-full overflow-hidden py-4">
			<div className="mx-auto w-[60%] max-w-4xl">
				<svg viewBox={world.viewBox} className="block h-auto w-full">
					{world.locations.map((l) => (
						<path
							key={l.id}
							d={l.path}
							style={{ fill: fillFor(l.id) }}
							className={userCC && l.id === userCC && !scc && !matchCC ? "animate-pulse" : undefined}
						>
							<title>{l.name}</title>
						</path>
					))}
				</svg>
			</div>

			<div className="absolute left-3 top-3 space-y-3">
				<div>
					<div className="text-[10px] font-semibold uppercase tracking-wider text-info">Your location</div>
					<div className="mt-0.5 flex items-center gap-2 text-[13px] font-semibold tracking-tight">
						{locationLabel()}
					</div>
					<div className="mt-0.5 font-mono text-[11px] opacity-50">
						{tz || "—"} · {offsetLabel}
						{!userCountry && <span className="opacity-70"> · full zone</span>}
					</div>
				</div>
				<div>
					<div className="text-[10px] font-semibold uppercase tracking-wider text-success">Closest server</div>
					<div className="mt-0.5 text-[13px] font-semibold tracking-tight">
						{(probing || connecting) && !pickedServer ? (
							<span className="flex items-center gap-2 opacity-60">
								<span className="loading loading-spinner loading-xs" /> Probing…
							</span>
						) : (
							closestLabel()
						)}
					</div>
				</div>
			</div>

			<div className="absolute bottom-3 left-3">
				{connected ? (
					<button className="btn btn-error shadow-lg" onClick={onDisconnect}>
						Disconnect
					</button>
				) : (
					<button className="btn btn-success shadow-lg" disabled={connecting} onClick={onConnect}>
						{connecting ? (
							<>
								<span className="loading loading-spinner loading-xs" /> Finding fastest server...
							</>
						) : (
							"Secure Connection"
						)}
					</button>
				)}
			</div>
		</div>
	)
}

export default TimezoneMap
