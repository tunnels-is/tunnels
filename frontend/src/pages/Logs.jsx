import { useMemo, useState } from "react"
import { ChevronLeft, ChevronRight, Search, Trash2 } from "lucide-react"
import { Page, Toolbar } from "@/components/ui"
import { LOG_FONT_SIZES, resolveLogFontSize } from "@/lib/logFontSize"
import { useStore } from "@/store/store"

const PAGE_SIZE = 100

const TAGS = [
	{ key: "", label: "All" },
	{ key: "INFO", label: "Info" },
	{ key: "ERROR", label: "Error" },
	{ key: "DEBUG", label: "Debug" },
	{ key: "ROUTINE", label: "Routine" },
]

const tagColor = (tag) =>
	({ ERROR: "text-error", DEBUG: "text-warning", INFO: "text-success", ROUTINE: "opacity-40" })[tag] || "opacity-60"

const Logs = () => {
	const logs = useStore((s) => s.logs)
	const clearLogs = useStore((s) => s.clearLogs)
	const logFontSize = useStore((s) => s.logFontSize)
	const setLogFontSize = useStore((s) => s.setLogFontSize)
	const [page, setPage] = useState(0)
	const [filter, setFilter] = useState("")
	const [tagFilter, setTagFilter] = useState("")

	const size = resolveLogFontSize(logFontSize)

	const filteredLogs = useMemo(() => {
		let filtered = logs
		if (filter) filtered = filtered.filter((line) => line.toLowerCase().includes(filter.toLowerCase()))

		filtered = filtered.filter((line) => {
			const tag = line.split(" || ")[1]?.trim()
			return tagFilter ? tag === tagFilter : tag !== "ROUTINE"
		})
		return filtered.toReversed()
	}, [logs, filter, tagFilter])

	const totalPages = Math.max(1, Math.ceil(filteredLogs.length / PAGE_SIZE))
	const safePage = Math.min(page, totalPages - 1)
	const paged = filteredLogs.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE)

	return (
		<Page>
			<div className="flex h-[calc(100vh-3rem)] flex-col">
			<Toolbar>
				<label className="input input-xs flex w-52 items-center gap-1">
						<Search size={12} className="opacity-40" />
						<input
							placeholder="Filter logs..."
							value={filter}
							onChange={(e) => {
								setFilter(e.target.value)
								setPage(0)
							}}
						/>
					</label>
					<button className="btn btn-square btn-ghost btn-xs text-error" title="Clear logs" onClick={clearLogs}>
						<Trash2 size={12} />
					</button>
					<select
						className="select select-xs w-auto"
						value={logFontSize}
						onChange={(e) => setLogFontSize(e.target.value)}
						title="Log font size"
					>
						{LOG_FONT_SIZES.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
							</option>
						))}
					</select>
					{filteredLogs.length > PAGE_SIZE && (
						<div className="flex items-center gap-1">
							<button className="btn btn-square btn-ghost btn-xs" disabled={safePage === 0} onClick={() => setPage(safePage - 1)}>
								<ChevronLeft size={14} />
							</button>
							<span className="font-mono text-[10px] opacity-50">
								{safePage + 1}/{totalPages}
							</span>
							<button
								className="btn btn-square btn-ghost btn-xs"
								disabled={safePage >= totalPages - 1}
								onClick={() => setPage(safePage + 1)}
							>
								<ChevronRight size={14} />
							</button>
						</div>
					)}
					{filteredLogs.length > 0 && <span className="font-mono text-[10px] opacity-50">{filteredLogs.length}</span>}

					<div className="ml-auto flex gap-1">
						{TAGS.map((t) => (
							<button
								key={t.key}
								className={"btn btn-xs " + (tagFilter === t.key ? "btn-primary" : "btn-ghost")}
								onClick={() => {
									setTagFilter(t.key)
									setPage(0)
								}}
							>
								{t.label}
							</button>
						))}
					</div>
			</Toolbar>

			<div className="min-h-0 flex-1 overflow-y-auto border border-base-300 bg-base-100 p-2">
				{paged.length > 0 ? (
					paged.map((line, i) => {
						const [timestamp, tag, func, ...rest] = line.split(" || ")
						return (
							<div key={i} className="flex items-baseline gap-1.5 pl-1 transition-colors hover:bg-base-300/30">
								<span className={"shrink-0 font-mono opacity-40 " + size.meta}>{timestamp}</span>
								<span className={"shrink-0 font-medium uppercase " + size.tagW + " " + size.meta + " " + tagColor(tag?.trim())}>
									{tag?.trim()}
								</span>
								<span className={"hidden shrink-0 truncate font-mono opacity-40 lg:block " + size.funcW + " " + size.meta}>
									{func}
								</span>
								<span className={"min-w-0 truncate font-mono " + size.message}>{rest.join(" || ")}</span>
							</div>
						)
					})
				) : (
					<div className={"py-6 pl-3 opacity-50 " + size.message}>
						{filter || tagFilter ? "No matching logs" : "No logs"}
					</div>
				)}
			</div>
			</div>
		</Page>
	)
}

export default Logs
