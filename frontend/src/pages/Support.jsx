import { ArrowUpRight, BookOpen, Github, LifeBuoy, Mail, MessageCircle, MessageSquare, Twitter } from "lucide-react"
import { Card, Page } from "@/components/ui"
import { useStore } from "@/store/store"

const PLATFORMS = [
	{ type: "email", name: "EMAIL", link: "support@tunnels.is" },
	{ type: "link", name: "X", link: "https://www.x.com/tunnels_is" },
	{ type: "link", name: "DISCORD", link: "https://discord.gg/2v5zX5cG3j" },
	{ type: "link", name: "REDDIT", link: "https://www.reddit.com/r/tunnels_is" },
	{
		type: "link",
		name: "SIGNAL",
		link: "https://signal.group/#CjQKIGvNLjUd8o3tkkGUZHuh0gfZqHEsn6rxXOG4S1U7m2lEEhBtuWbyxBjMLM_lo1rVjFX0",
	},
]

const COMMUNITY_META = {
	DISCORD: { icon: MessageCircle, label: "Discord" },
	X: { icon: Twitter, label: "X" },
	REDDIT: { icon: MessageSquare, label: "Reddit" },
	SIGNAL: { icon: MessageSquare, label: "Signal" },
}

const stripUrl = (url) => url.replace(/^https?:\/\/(www\.)?/, "")

const LinkRow = ({ icon: Icon, name, value, href }) => (
	<a
		href={href}
		target={href.startsWith("mailto:") ? undefined : "_blank"}
		rel="noopener noreferrer"
		className="group -mx-2 flex items-center gap-3 rounded-md px-2 py-2 transition-colors hover:bg-base-200"
	>
		<Icon size={16} className="shrink-0 opacity-50" />
		<div className="min-w-0 flex-1">
			<div className="text-[13px] font-medium tracking-tight">{name}</div>
			<div className="truncate font-mono text-[11px] opacity-50">{value}</div>
		</div>
		<ArrowUpRight
			size={14}
			className="shrink-0 opacity-0 transition-all group-hover:-translate-y-px group-hover:translate-x-px group-hover:opacity-60"
		/>
	</a>
)

const Support = () => {
	const version = useStore((s) => s.version)
	const apiVersion = useStore((s) => s.apiVersion)
	const community = PLATFORMS.filter((s) => s.type === "link")
	const contact = PLATFORMS.filter((s) => s.type === "email")

	return (
		<Page
			actions={
				<div className="flex items-center gap-3 text-[11px] opacity-60">
					<span>
						App <code className="font-mono">{version || "unknown"}</code>
					</span>
					<span>
						API <code className="font-mono">{apiVersion || "unknown"}</code>
					</span>
				</div>
			}
		>
			<Card className="mb-4">
				<div className="flex items-center gap-4">
					<div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-neutral text-neutral-content">
						<LifeBuoy size={20} />
					</div>
					<div className="min-w-0 flex-1">
						<div className="text-[14px] font-semibold tracking-tight">Need a hand?</div>
						<p className="text-xs leading-relaxed opacity-70">
							Start with the documentation, then reach out on Discord or email if you&apos;re stuck.
						</p>
					</div>
					<div className="flex shrink-0 items-center gap-2">
						<a href="https://www.tunnels.is/docs" target="_blank" rel="noopener noreferrer" className="btn btn-primary btn-sm">
							Read docs <ArrowUpRight size={14} />
						</a>
						<a href="https://discord.gg/2v5zX5cG3j" target="_blank" rel="noopener noreferrer" className="btn btn-outline btn-sm">
							Discord
						</a>
					</div>
				</div>
			</Card>

			<div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
				<Card title="Resources" description="Documentation and source code.">
					<div className="grid grid-cols-1">
						<LinkRow icon={BookOpen} name="Documentation" value="tunnels.is/docs" href="https://www.tunnels.is/docs" />
						<LinkRow icon={Github} name="GitHub" value="tunnels-is/tunnels" href="https://www.github.com/tunnels-is/tunnels" />
					</div>
				</Card>

				<Card title="Direct contact" description="Email is the fastest way to reach us about billing or security.">
					<div className="grid grid-cols-1">
						{contact.map((s) => (
							<LinkRow key={s.name} icon={Mail} name="Email" value={s.link} href={`mailto:${s.link}`} />
						))}
					</div>
				</Card>

				<Card title="Community" description="Join the public chat rooms — we're active in all of them.">
					<div className="grid grid-cols-1">
						{community.map((s) => {
							const meta = COMMUNITY_META[s.name] || { icon: MessageCircle, label: s.name }
							return <LinkRow key={s.name} icon={meta.icon} name={meta.label} value={stripUrl(s.link)} href={s.link} />
						})}
					</div>
				</Card>
			</div>
		</Page>
	)
}

export default Support
