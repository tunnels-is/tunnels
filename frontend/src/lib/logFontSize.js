export const LOG_FONT_SIZES = [
	{ value: "small", label: "Small" },
	{ value: "medium", label: "Medium" },
	{ value: "large", label: "Large" },
	{ value: "xlarge", label: "Very large" },
]

export const LOG_FONT_SIZE_STYLES = {
	small: {
		meta: "text-[10px]",
		message: "text-[11px]",
		tagW: "w-12",
		funcW: "max-w-44",
	},
	medium: {
		meta: "text-xs",
		message: "text-sm",
		tagW: "w-14",
		funcW: "max-w-48",
	},
	large: {
		meta: "text-sm",
		message: "text-base",
		tagW: "w-16",
		funcW: "max-w-56",
	},
	xlarge: {
		meta: "text-base",
		message: "text-lg",
		tagW: "w-20",
		funcW: "max-w-64",
	},
}

export const DEFAULT_LOG_FONT_SIZE = "medium"

export const resolveLogFontSize = (value) =>
	LOG_FONT_SIZE_STYLES[value] || LOG_FONT_SIZE_STYLES[DEFAULT_LOG_FONT_SIZE]
