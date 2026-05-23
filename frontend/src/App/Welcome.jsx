import React from "react";
import STORE from "../store";
import GLOBAL_STATE from "../state";
import {
  ArrowUpRight,
  BookOpen,
  Github,
  MessageCircle,
  MessageSquare,
  Mail,
  Twitter,
  LifeBuoy,
} from "lucide-react";

/* ─── Building blocks ────────────────────────────────────────────── */

const SettingsCard = ({ title, description, actions, children, className = "" }) => (
  <section className={"rounded-lg bg-white border border-[#e7e3d7] card-shadow p-5 " + className}>
    <header className="flex items-start justify-between gap-3 mb-4">
      <div>
        <h3 className="text-[13px] font-semibold tracking-tight text-[#0a0a0a]">{title}</h3>
        {description && (
          <p className="mt-1 text-[11px] text-[#737373] leading-relaxed">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </header>
    {children}
  </section>
);

// A clickable resource / link tile (used for docs, GitHub, community channels, etc.)
const LinkCard = ({ icon: Icon, name, value, href, tone = "neutral" }) => {
  const external = !href.startsWith("mailto:");
  const toneClasses = {
    info:    "bg-[#1d4ed8]/[0.08] text-[#1d4ed8] ring-1 ring-inset ring-[#1d4ed8]/15",
    success: "bg-[#15803d]/[0.08] text-[#15803d] ring-1 ring-inset ring-[#15803d]/15",
    warning: "bg-[#b45309]/[0.08] text-[#b45309] ring-1 ring-inset ring-[#b45309]/15",
    danger:  "bg-[#dc2626]/[0.08] text-[#dc2626] ring-1 ring-inset ring-[#dc2626]/15",
    neutral: "bg-black/[0.04] text-[#0a0a0a] ring-1 ring-inset ring-black/[0.06]",
  }[tone] || "bg-black/[0.04] text-[#0a0a0a] ring-1 ring-inset ring-black/[0.06]";

  return (
    <a
      href={href}
      target={external ? "_blank" : undefined}
      rel={external ? "noopener noreferrer" : undefined}
      className="group flex items-center gap-3 p-3 rounded-md border border-[#e7e3d7] bg-white card-shadow card-shadow-hover transition-all hover:border-[#0a0a0a]/25"
    >
      <div className={"w-9 h-9 rounded-md flex items-center justify-center shrink-0 " + toneClasses}>
        <Icon className="w-4 h-4" strokeWidth={2} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-[13px] font-semibold tracking-tight text-[#0a0a0a]">{name}</div>
        <div className="text-[11px] text-[#525252] font-mono truncate">{value}</div>
      </div>
      <ArrowUpRight
        className="w-3.5 h-3.5 text-[#a3a3a3] group-hover:text-[#0a0a0a] group-hover:translate-x-[1px] group-hover:-translate-y-[1px] transition-all shrink-0"
        strokeWidth={2}
      />
    </a>
  );
};

/* ─── Community channel descriptors ──────────────────────────────── */

const COMMUNITY_META = {
  DISCORD: { icon: MessageCircle,  label: "Discord",  tone: "info"    },
  X:       { icon: Twitter,        label: "X",        tone: "neutral" },
  REDDIT:  { icon: MessageSquare,  label: "Reddit",   tone: "warning" },
  SIGNAL:  { icon: MessageSquare,  label: "Signal",   tone: "info"    },
};

const stripUrl = (url) => url.replace(/^https?:\/\/(www\.)?/, "");

/* ─── Page ───────────────────────────────────────────────────────── */

const Welcome = () => {
  const state = GLOBAL_STATE("welcome");
  const community = STORE.SupportPlatforms.filter((s) => s.type === "link");
  const contact = STORE.SupportPlatforms.filter((s) => s.type === "email");

  const appVersion = state.Version || "unknown";
  const apiVersion = state.APIVersion || "unknown";

  return (
    <div className="max-w-5xl">

      {/* Page header */}
      <header className="mb-6 flex items-baseline justify-between gap-4">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight text-[#0a0a0a]">Support</h1>
          <p className="mt-1 text-[12px] text-[#737373]">
            Browse the docs, contact us, or ask the community.
          </p>
        </div>
        <div className="flex items-center gap-3 text-[11px] text-[#a3a3a3]">
          <div className="flex items-baseline gap-1.5">
            <span className="uppercase tracking-[0.1em] text-[9px] font-semibold">App</span>
            <code className="font-mono text-[#525252]">{appVersion}</code>
          </div>
          <span className="w-px h-3 bg-[#e7e3d7]" />
          <div className="flex items-baseline gap-1.5">
            <span className="uppercase tracking-[0.1em] text-[9px] font-semibold">API</span>
            <code className="font-mono text-[#525252]">{apiVersion}</code>
          </div>
        </div>
      </header>

      {/* Hero panel */}
      <div className="mb-4 rounded-lg border border-[#e7e3d7] card-shadow overflow-hidden bg-white">
        <div className="flex items-center gap-4 p-5">
          <div className="w-11 h-11 rounded-md bg-[#0a0a0a] text-white shadow-[0_2px_4px_rgba(10,10,10,0.18)] flex items-center justify-center shrink-0">
            <LifeBuoy className="w-5 h-5" strokeWidth={2} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-[14px] font-semibold tracking-tight text-[#0a0a0a]">
              Need a hand?
            </div>
            <p className="text-[12px] text-[#525252] leading-relaxed">
              Start with the documentation, then reach out on Discord or email if you&apos;re stuck.
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <a
              href="https://www.tunnels.is/docs"
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-primary btn-sm"
            >
              Read docs <ArrowUpRight className="w-3.5 h-3.5" />
            </a>
            <a
              href="https://discord.gg/2v5zX5cG3j"
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-outline btn-sm"
            >
              Discord
            </a>
          </div>
        </div>
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">

        {/* ── Resources ── */}
        <SettingsCard
          title="Resources"
          description="Documentation and source code."
        >
          <div className="grid grid-cols-1 gap-2">
            <LinkCard
              icon={BookOpen}
              tone="info"
              name="Documentation"
              value="tunnels.is/docs"
              href="https://www.tunnels.is/docs"
            />
            <LinkCard
              icon={Github}
              tone="neutral"
              name="GitHub"
              value="tunnels-is/tunnels"
              href="https://www.github.com/tunnels-is/tunnels"
            />
          </div>
        </SettingsCard>

        {/* ── Direct contact ── */}
        <SettingsCard
          title="Direct contact"
          description="Email is the fastest way to reach us about billing or security."
        >
          <div className="grid grid-cols-1 gap-2">
            {contact.map((s) => (
              <LinkCard
                key={s.name}
                icon={Mail}
                tone="success"
                name="Email"
                value={s.link}
                href={`mailto:${s.link}`}
              />
            ))}
          </div>
        </SettingsCard>

        {/* ── Community ── */}
        <SettingsCard
          title="Community"
          description="Join the public chat rooms — we&apos;re active in all of them."
          className="lg:col-span-2"
        >
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {community.map((s) => {
              const meta = COMMUNITY_META[s.name] || { icon: MessageCircle, label: s.name, tone: "neutral" };
              return (
                <LinkCard
                  key={s.name}
                  icon={meta.icon}
                  tone={meta.tone}
                  name={meta.label}
                  value={stripUrl(s.link)}
                  href={s.link}
                />
              );
            })}
          </div>
        </SettingsCard>

      </div>
    </div>
  );
};

export default Welcome;
