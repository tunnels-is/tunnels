const STORAGE_KEY = "tunnels-ui-zoom"
const MIN = 0.75
const MAX = 1.75
const STEP = 0.1

const clamp = (v) => Math.min(MAX, Math.max(MIN, Math.round(v * 100) / 100))

const apply = (level) => {
	document.documentElement.style.zoom = String(level)
}

const readStored = () => {
	const raw = window.localStorage.getItem(STORAGE_KEY)
	const n = raw == null ? 1 : Number(raw)
	return Number.isFinite(n) ? clamp(n) : 1
}

export const initZoom = () => {
	let level = readStored()
	apply(level)

	const setLevel = (next) => {
		level = clamp(next)
		apply(level)
		window.localStorage.setItem(STORAGE_KEY, String(level))
	}

	window.addEventListener(
		"keydown",
		(e) => {
			if (!(e.ctrlKey || e.metaKey) || e.altKey) return
			const key = e.key
			if (key === "=" || key === "+" || key === "Add") {
				e.preventDefault()
				setLevel(level + STEP)
			} else if (key === "-" || key === "_" || key === "Subtract") {
				e.preventDefault()
				setLevel(level - STEP)
			} else if (key === "0" || key === "Digit0" || key === "Numpad0") {
				e.preventDefault()
				setLevel(1)
			}
		},
		{ capture: true },
	)

	window.addEventListener(
		"wheel",
		(e) => {
			if (!(e.ctrlKey || e.metaKey)) return
			e.preventDefault()
			setLevel(level + (e.deltaY > 0 ? -STEP : STEP))
		},
		{ passive: false, capture: true },
	)
}
