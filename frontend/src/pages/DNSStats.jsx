import { useEffect, useMemo, useState } from "react"
import { ChevronLeft, ChevronRight, RefreshCw, Search } from "lucide-react"
import { Card, Page, Toolbar } from "@/components/ui"
import { fetchDnsStats } from "@/store/actions"
import { shortDate } from "@/lib/format"
import { useStore } from "@/store/store"

const PAGE_SIZE = 50

const DNSStats = () => {
	const dnsStats = useStore((s) => s.dnsStats)
	const advanced = useStore((s) => s.advanced)
	const [tab, setTab] = useState("blocked")
	const [filter, setFilter] = useState("")
	const [page, setPage] = useState(0)

	useEffect(() => {
		fetchDnsStats()
	}, [])

	const allItems = useMemo(() => {
		const entries = Object.entries(dnsStats || {}).map(([domain, value]) => ({ ...value, domain }))

		const wanted = entries.filter((e) => {
			const blocked = new Date(e.LastSeen) - new Date(e.LastBlocked) <= 0
			return tab === "blocked" ? blocked : !blocked
		})
		return wanted.sort((a, b) => new Date(b.LastSeen) - new Date(a.LastSeen))
	}, [dnsStats, tab])

	const filtered = filter ? allItems.filter((d) => d.domain.toLowerCase().includes(filter.toLowerCase())) : allItems
	const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
	const safePage = Math.min(page, totalPages - 1)
	const paged = filtered.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE)

	if (!advanced) {
		return (
			<Page>
				<div className="flex h-40 items-center justify-center text-[13px] opacity-50">
					Enable Advanced mode in Settings to view DNS stats.
				</div>
			</Page>
		)
	}

	return (
		<Page>
			<Toolbar>
				<div role="tablist" className="tabs tabs-border tabs-sm">
						{["blocked", "resolved"].map((t) => (
							<button
								key={t}
								role="tab"
								className={"tab capitalize " + (tab === t ? "tab-active" : "")}
								onClick={() => {
									setTab(t)
									setPage(0)
									setFilter("")
								}}
							>
								{t}
							</button>
						))}
					</div>
					<div className="ml-auto flex items-center gap-1.5">
						<button className="btn btn-square btn-ghost btn-xs" title="Refresh" onClick={fetchDnsStats}>
							<RefreshCw size={12} />
						</button>
						<label className="input input-xs flex w-44 items-center gap-1">
							<Search size={12} className="opacity-40" />
							<input
								placeholder="Filter domains..."
								value={filter}
								onChange={(e) => {
									setFilter(e.target.value)
									setPage(0)
								}}
							/>
						</label>
						{filtered.length > PAGE_SIZE && (
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
						{filtered.length > 0 && <span className="font-mono text-[10px] opacity-50">{filtered.length}</span>}
				</div>
			</Toolbar>

			<Card>
				<table className="table table-xs">
					<thead>
						<tr>
							<th>Domain</th>
							<th className="w-16 text-right">Count</th>
							<th className="hidden w-32 text-right md:table-cell">First seen</th>
							<th className="w-32 text-right">Last seen</th>
						</tr>
					</thead>
					<tbody>
						{paged.length > 0 ? (
							paged.map((d, i) => (
								<tr key={i} className="hover">
									<td className={"truncate font-mono " + (tab === "blocked" ? "text-error" : "")}>{d.domain}</td>
									<td className="text-right font-mono">{d.Count}</td>
									<td className="hidden text-right text-[11px] opacity-50 md:table-cell">{shortDate(d.FirstSeen)}</td>
									<td className="text-right text-[11px] opacity-50">{shortDate(d.LastSeen)}</td>
								</tr>
							))
						) : (
							<tr>
								<td colSpan={4} className="py-6 text-center text-xs opacity-50">
									{filter ? "No matching domains" : "No data"}
								</td>
							</tr>
						)}
					</tbody>
				</table>
			</Card>
		</Page>
	)
}

export default DNSStats
