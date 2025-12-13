import React from "react";
import { SupportPlatforms } from "@/lib/constants";
import Logo from "@/assets/images/fav/logo.svg";
import DiscordLogo from "@/assets/images/Discord-Symbol-White.svg";
import RedditLogo from "@/assets/images/reddit-logo-fill-svgrepo-com.svg";

import { Card, CardContent } from "@/components/ui/card";

import {
  ExternalLink,
  Mail,
  MessageCircle,
  Twitter,
  Globe
} from "lucide-react";

export default function HelpPage() {
  const version = "1.0.0";

  const getIcon = (name) => {
    switch (name.toLowerCase()) {
      case "email":
        return <Mail className="h-5 w-5" />;
      case "x":
        return <Twitter className="h-5 w-5" />;
      case "discord":
        return <img src={DiscordLogo} alt="Discord" className="h-5 w-5" />;
      case "reddit":
        return <img src={RedditLogo} alt="Reddit" className="h-5 w-5" />;
      case "signal":
        return <MessageCircle className="h-5 w-5" />;
      default:
        return <Globe className="h-5 w-5" />;
    }
  };

  const handleOpenLink = (link) => {
    window.open(link, "_blank");
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-[80vh] p-6 space-y-12 animate-in fade-in duration-500">
      {/* Hero Section */}
      <div className="text-center space-y-4 flex flex-col items-center">
        <img src={Logo} alt="Tunnels Logo" className="h-24 w-24 drop-shadow-lg" />
        <h1 className="text-4xl md:text-6xl font-bold tracking-tighter bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
          Tunnels
        </h1>
        <div className="flex items-center justify-center gap-2">
          <span className="px-3 py-1 text-sm font-medium rounded-full bg-muted text-muted-foreground border">
            v{version}
          </span>
        </div>
        <p className="text-muted-foreground max-w-[600px] mx-auto text-lg">
          Secure, fast, and private networking for everyone.
        </p>
      </div>

      {/* Support Channels */}
      <div className="w-full max-w-4xl space-y-6">
        <div className="text-center space-y-2">
          <h2 className="text-xl font-semibold tracking-tight">Community & Support</h2>
          <p className="text-sm text-muted-foreground">
            Join our community or get in touch with us.
          </p>
        </div>

        <div className="flex flex-wrap justify-center gap-4">
          {SupportPlatforms.map((s) => (
            <Card
              key={s.name}
              className="w-full sm:w-[280px] group hover:border-primary/50 transition-all duration-300 cursor-pointer hover:shadow-lg"
              onClick={() => handleOpenLink(s.link)}
            >
              <CardContent className="p-6 flex items-center gap-4">
                <div className="p-3 rounded-full bg-primary/10 text-primary transition-colors duration-300">
                  {getIcon(s.name)}
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="font-medium truncate">{s.name}</h3>
                  <p className="text-xs text-muted-foreground truncate">
                    {s.type === "email" ? "Contact Support" : "Join Community"}
                  </p>
                </div>
                <ExternalLink className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  );
};
