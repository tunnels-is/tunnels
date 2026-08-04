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


export const isGenericTimezone = (tz) => {
	if (!tz || typeof tz !== "string") return true
	const t = tz.trim()
	return t === "" || t === "UTC" || t === "GMT" || t.startsWith("Etc/")
}


export const resolveDisplayTimezone = (daemonTimezone) => {
	const browserTz =
		typeof Intl !== "undefined" ? Intl.DateTimeFormat().resolvedOptions().timeZone : ""
	if (!isGenericTimezone(browserTz)) return browserTz
	if (!isGenericTimezone(daemonTimezone)) return daemonTimezone
	return browserTz || daemonTimezone || "UTC"
}


const countryForTimezoneName = (tz) => {
	if (!tz || typeof tz !== "string" || isGenericTimezone(tz)) return null

	const direct = ct.getCountryForTimezone(tz)?.id
	if (direct) return direct.toUpperCase()

	const tzi = ct.getTimezone(tz)
	if (tzi?.countries?.[0]) return tzi.countries[0].toUpperCase()


	if (tz.startsWith("Europe/")) {
		const alt = "Atlantic/" + tz.slice("Europe/".length)
		const via = ct.getCountryForTimezone(alt)?.id || ct.getTimezone(alt)?.countries?.[0]
		if (via) return via.toUpperCase()
	}

	return null
}


export const resolveUserCountry = (timezone) => {
	const tz = resolveDisplayTimezone(timezone)
	return countryForTimezoneName(tz)
}


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


export const matchServerCountry = (userCountry, serverCountries = []) => {
	if (!userCountry) return null
	const hit = (serverCountries || []).find((c) => sameCountry(c, userCountry))
	return hit ? normalizeCountryCode(hit) : null
}


export const resolveTargetCountry = (timezone) => resolveUserCountry(timezone) || ""
