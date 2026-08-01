// Timezone -> country translation for auto-connect. No network involved:
// the mapping ships with the IANA tz database via countries-and-timezones.

import ct from "countries-and-timezones"
import { normalizeCountryCode, sameCountry } from "./countries"

// UTC offset in minutes for an IANA timezone name (DST-aware via Intl).
export const timezoneOffsetMinutes = (tz) => {
	try {
		const parts = new Intl.DateTimeFormat("en-US", { timeZone: tz, timeZoneName: "shortOffset" }).formatToParts()
		const name = parts.find((p) => p.type === "timeZoneName")?.value || ""
		const m = name.match(/GMT([+-])(\d+)(?::(\d+))?/)
		if (!m) return 0
		return (m[1] === "-" ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3] || 0))
	} catch {
		return 0
	}
}

// Among the given country codes, the one whose timezones come closest to the
// given UTC offset. Used when the zone itself names no country (UTC, Etc/*).
// Returns the code as the caller spelled it so server matching keeps working.
const closestCountryByOffset = (offsetMinutes, countryCodes) => {
	let best
	let bestDiff = Infinity
	for (const code of new Set(countryCodes.filter(Boolean))) {
		const c = ct.getCountry(normalizeCountryCode(code))
		for (const tzName of c?.timezones || []) {
			const tz = ct.getTimezone(tzName)
			if (!tz) continue
			const diff = Math.min(Math.abs(tz.utcOffset - offsetMinutes), Math.abs(tz.dstOffset - offsetMinutes))
			if (diff < bestDiff) {
				bestDiff = diff
				best = code.toUpperCase()
			}
		}
	}
	return best
}

// Resolves the country to target for auto-connect:
// 1. the country the timezone belongs to,
// 2. else the closest available server country by UTC offset,
// 3. else US.
// When a server country matches, its own spelling is returned (e.g. "UK")
// so filtering against server records stays consistent.
export const resolveTargetCountry = (timezone, serverCountries = []) => {
	const tz = timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
	const direct = ct.getCountryForTimezone(tz)?.id
	if (direct) {
		const serverSpelling = serverCountries.find((c) => sameCountry(c, direct))
		return serverSpelling ? serverSpelling.toUpperCase() : direct
	}
	return closestCountryByOffset(timezoneOffsetMinutes(tz), serverCountries) || "US"
}
