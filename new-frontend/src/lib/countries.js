// ISO 3166-1 alpha-2 country names for server listings.

const NAMES = new Intl.DisplayNames(["en"], { type: "region" })

export const countryName = (code) => {
	if (!code) return code
	try {
		return NAMES.of(code.toUpperCase()) || code
	} catch {
		return code
	}
}
