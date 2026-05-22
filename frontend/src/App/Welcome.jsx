import React from "react";
import STORE from "../store";
import { ExternalLink } from "lucide-react";

const Welcome = () => {
  const open = (url) => window.open(url, "_blank");

  const resources = [
    { label: "Documentation", value: "tunnels.is/docs", link: "https://www.tunnels.is/docs" },
    { label: "GitHub", value: "tunnels-is/tunnels", link: "https://www.github.com/tunnels-is/tunnels" },
  ];

  const community = STORE.SupportPlatforms.filter((s) => s.type === "link");
  const contact = STORE.SupportPlatforms.filter((s) => s.type === "email");

  return (
    <div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">

        {/* Resources */}
        <div>
          <span className="label-section">Resources</span>
          <div className="space-y-1">
            {resources.map((row, i) => (
              <div
                key={i}
                className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-[#1d4ed8]/30 hover:border-[#1d4ed8]/60 cursor-pointer transition-colors"
                onClick={() => open(row.link)}
              >
                <span className="label-caption shrink-0 w-[110px]">{row.label}</span>
                <code className="text-[13px] text-[#525252] font-mono truncate flex items-center gap-1.5">
                  {row.value} <ExternalLink className="h-3 w-3 text-[#a3a3a3]" />
                </code>
              </div>
            ))}
          </div>
        </div>

        {/* Community */}
        <div>
          <span className="label-section">Community</span>
          <div className="space-y-1">
            {community.map((s, i) => (
              <div
                key={i}
                className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-[#525252]/30 hover:border-[#0a0a0a]/50 cursor-pointer transition-colors"
                onClick={() => open(s.link)}
              >
                <span className="label-caption shrink-0 w-[110px]">{s.name}</span>
                <code className="text-[13px] text-[#525252] font-mono truncate flex items-center gap-1.5">
                  {s.link.replace(/^https?:\/\/(www\.)?/, "")} <ExternalLink className="h-3 w-3 text-[#a3a3a3]" />
                </code>
              </div>
            ))}
          </div>
        </div>

        {/* Contact */}
        <div>
          <span className="label-section">Contact</span>
          <div className="space-y-1">
            {contact.map((s, i) => (
              <div
                key={i}
                className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-[#15803d]/30 hover:border-[#15803d]/50 cursor-pointer transition-colors"
                onClick={() => { window.location.href = `mailto:${s.link}`; }}
              >
                <span className="label-caption shrink-0 w-[110px]">{s.name}</span>
                <code className="text-[13px] text-[#525252] font-mono truncate">{s.link}</code>
              </div>
            ))}
          </div>
        </div>

      </div>
    </div>
  );
};

export default Welcome;
