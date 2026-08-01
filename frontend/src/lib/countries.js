// ISO 3166-1 alpha-2 country helpers.

const NAMES = new Intl.DisplayNames(["en"], { type: "region" })

// Common non-ISO aliases seen in server configs.
const ALIASES = { UK: "GB" }

// Uppercased ISO code with aliases resolved ("uk" -> "GB").
export const normalizeCountryCode = (code) => {
	if (!code) return code
	const upper = code.toUpperCase()
	return ALIASES[upper] || upper
}

// True when two codes refer to the same country, alias-aware.
export const sameCountry = (a, b) => !!a && !!b && normalizeCountryCode(a) === normalizeCountryCode(b)

export const countryName = (code) => {
	if (!code) return code
	try {
		return NAMES.of(normalizeCountryCode(code)) || code
	} catch {
		return code
	}
}
