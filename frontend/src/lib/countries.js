const NAMES = new Intl.DisplayNames(["en"], { type: "region" })

const ALIASES = { UK: "GB" }

export const normalizeCountryCode = (code) => {
	if (!code) return code
	const upper = code.toUpperCase()
	return ALIASES[upper] || upper
}

export const sameCountry = (a, b) => !!a && !!b && normalizeCountryCode(a) === normalizeCountryCode(b)

export const countryName = (code) => {
	if (!code) return code
	try {
		return NAMES.of(normalizeCountryCode(code)) || code
	} catch {
		return code
	}
}
