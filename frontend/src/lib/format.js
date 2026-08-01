const dateFmt = new Intl.DateTimeFormat("en-GB", {
	weekday: "short",
	day: "numeric",
	hour: "2-digit",
	minute: "2-digit",
	second: "2-digit",
	hour12: false,
})

const fullFmt = new Intl.DateTimeFormat("en-GB", {
	year: "numeric",
	month: "short",
	day: "numeric",
	hour: "2-digit",
	minute: "2-digit",
	hour12: false,
})

export const shortDate = (value) => {
	const d = new Date(value)
	if (isNaN(d)) return String(value)
	const p = Object.fromEntries(dateFmt.formatToParts(d).map((x) => [x.type, x.value]))
	return `${p.weekday} ${p.day}. ${p.hour}:${p.minute}:${p.second}`
}

export const fullDate = (value) => {
	const d = new Date(value)
	if (isNaN(d)) return String(value)
	return fullFmt.format(d)
}

export const ENC_TYPES = ["None", "AES128", "AES256", "CHACHA20"]
export const encTypeName = (n) => ENC_TYPES[n] ?? "unknown"
