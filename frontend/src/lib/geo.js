import ct from "countries-and-timezones"
import { normalizeCountryCode, sameCountry } from "./countries"

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

export const resolveTargetCountry = (timezone, serverCountries = []) => {
	const tz = timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
	const direct = ct.getCountryForTimezone(tz)?.id
	if (direct) {
		const serverSpelling = serverCountries.find((c) => sameCountry(c, direct))
		return serverSpelling ? serverSpelling.toUpperCase() : direct
	}
	return closestCountryByOffset(timezoneOffsetMinutes(tz), serverCountries) || "US"
}
