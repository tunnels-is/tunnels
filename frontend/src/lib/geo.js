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

/** True for zones that are not a real place (UTC / Etc/*). */
export const isGenericTimezone = (tz) => {
	if (!tz || typeof tz !== "string") return true
	const t = tz.trim()
	return t === "" || t === "UTC" || t === "GMT" || t.startsWith("Etc/")
}

/**
 * Timezone used for the map / location label.
 * Prefer the browser/OS zone the UI is running in over a daemon that reports bare UTC
 * (common on servers/containers), then fall back to the daemon zone.
 */
export const resolveDisplayTimezone = (daemonTimezone) => {
	const browserTz =
		typeof Intl !== "undefined" ? Intl.DateTimeFormat().resolvedOptions().timeZone : ""
	if (!isGenericTimezone(browserTz)) return browserTz
	if (!isGenericTimezone(daemonTimezone)) return daemonTimezone
	return browserTz || daemonTimezone || "UTC"
}

/** Resolve ISO country for a single IANA timezone name. Never guesses from language/locale. */
const countryForTimezoneName = (tz) => {
	if (!tz || typeof tz !== "string" || isGenericTimezone(tz)) return null

	const direct = ct.getCountryForTimezone(tz)?.id
	if (direct) return direct.toUpperCase()

	const tzi = ct.getTimezone(tz)
	if (tzi?.countries?.[0]) return tzi.countries[0].toUpperCase()

	// Deprecated / alternate prefixes (e.g. Europe/Reykjavik → Atlantic/Reykjavik).
	if (tz.startsWith("Europe/")) {
		const alt = "Atlantic/" + tz.slice("Europe/".length)
		const via = ct.getCountryForTimezone(alt)?.id || ct.getTimezone(alt)?.countries?.[0]
		if (via) return via.toUpperCase()
	}

	return null
}

/**
 * Country for the user's location, or null when the timezone has no country
 * (UTC / unknown). Callers should highlight the whole offset band in that case.
 */
export const resolveUserCountry = (timezone) => {
	const tz = resolveDisplayTimezone(timezone)
	return countryForTimezoneName(tz)
}

/** All ISO country codes (lowercase) that share the same UTC offset as the zone. */
export const countriesAtOffset = (offsetMinutes) => {
	const out = new Set()
	for (const country of Object.values(ct.getAllCountries())) {
		for (const tzName of country.timezones || []) {
			const tzi = ct.getTimezone(tzName)
			if (!tzi) continue
			if (tzi.utcOffset === offsetMinutes || tzi.dstOffset === offsetMinutes) {
				out.add(country.id.toLowerCase())
				break
			}
		}
	}
	return out
}

/**
 * If any server country matches the user's country, return that spelling from
 * the inventory so filters stay consistent (e.g. UK vs GB).
 */
export const matchServerCountry = (userCountry, serverCountries = []) => {
	if (!userCountry) return null
	const hit = (serverCountries || []).find((c) => sameCountry(c, userCountry))
	return hit ? normalizeCountryCode(hit) : null
}

/**
 * Country to send for auto-connect / probe. Empty string when unknown —
 * the daemon then skips country match and goes straight to full-list ping probe.
 */
export const resolveTargetCountry = (timezone) => resolveUserCountry(timezone) || ""
