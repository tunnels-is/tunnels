// World map with the user's country highlighted and the 24 UTC offset bands
// overlaid; the secure-connection button sits centered on top. Lazy-loaded —
// the map path data is large and only needed in simple mode.

import ct from "countries-and-timezones"
import world from "@svg-maps/world"
import { countryName, normalizeCountryCode } from "@/lib/countries"
import { timezoneOffsetMinutes } from "@/lib/geo"

// Lowercase ISO codes of every country with a timezone at the given UTC
// offset — the political timezone "band" at country resolution.
const countriesAtOffset = (offsetMinutes) => {
	const out = new Set()
	for (const country of Object.values(ct.getAllCountries())) {
		for (const tzName of country.timezones) {
			const tz = ct.getTimezone(tzName)
			if (tz && (tz.utcOffset === offsetMinutes || tz.dstOffset === offsetMinutes)) {
				out.add(country.id.toLowerCase())
				break
			}
		}
	}
	return out
}

const TimezoneMap = ({ timezone, country, serverCountry, connecting, connected, onConnect, onDisconnect }) => {
	const tz = timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
	const offset = timezoneOffsetMinutes(tz)
	const offsetLabel = "UTC" + (offset === 0 ? "" : (offset > 0 ? "+" : "-") + Math.abs(offset / 60))
	const cc = normalizeCountryCode(country)?.toLowerCase()
	const scc = normalizeCountryCode(serverCountry)?.toLowerCase()
	const sameOffset = countriesAtOffset(offset)

	const fillFor = (id) => {
		if (id === scc) return "fill-success" // country of the connected server
		if (id === cc) return "animate-pulse fill-success" // closest server country
		if (sameOffset.has(id)) return "fill-primary/30" // shares the user's UTC offset
		return "fill-base-content/15"
	}

	return (
		<div className="relative mb-4 w-full overflow-hidden border border-base-300 bg-base-100 py-4">
			<div className="mx-auto w-[60%] max-w-4xl">
				<svg viewBox={world.viewBox} className="block h-auto w-full">
					{world.locations.map((l) => (
						<path key={l.id} d={l.path} className={fillFor(l.id)}>
							<title>{l.name}</title>
						</path>
					))}
				</svg>
			</div>

			{/* user timezone + country translation — colors match the map */}
			<div className="absolute left-3 top-3 space-y-3">
				<div>
					<div className="text-[10px] font-semibold uppercase tracking-wider text-primary">Current timezone</div>
					<div className="mt-0.5 flex items-center gap-2 text-[13px] font-semibold tracking-tight">
						{tz}
						<span className="font-mono text-[11px] font-normal opacity-50">{offsetLabel}</span>
					</div>
				</div>
				<div>
					<div className="text-[10px] font-semibold uppercase tracking-wider text-success">Closest server</div>
					<div className="mt-0.5 flex items-center gap-2 text-[13px] font-semibold tracking-tight">
						{country ? (
							<>
								<span className={`fi fi-${cc} rounded-[2px]`} title={countryName(country)} />
								{countryName(country)}
							</>
						) : (
							<span className="opacity-40">—</span>
						)}
					</div>
				</div>
			</div>

			{/* action */}
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
