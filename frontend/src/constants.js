export const SupportPlatforms = [
  { type: "email", name: "EMAIL", link: "support@tunnels.is" },
  { type: "link", name: "X", link: "https://www.x.com/tunnels_is" },
  { type: "link", name: "DISCORD", link: "https://discord.gg/2v5zX5cG3j" },
  {
    type: "link",
    name: "REDDIT",
    link: "https://www.reddit.com/r/tunnels_is",
  },
  {
    type: "link",
    name: "SIGNAL",
    link: "https://signal.group/#CjQKIGvNLjUd8o3tkkGUZHuh0gfZqHEsn6rxXOG4S1U7m2lEEhBtuWbyxBjMLM_lo1rVjFX0",
  },
];

export const EncryptionTypes = ["None", "AES128", "AES256", "CHACHA20"];
export const ROUTER_Tooltips = [
  "Quality of service is a score calculated from latency, available bandwidth and number of available user spots on the Router. 10 is the best score, 0 is the worst score",

  "Latency from your computer to this Router",

  "Available user slots",

  "Available Gigabits per second of bandwidth",

  "Processor usage in %",

  "Memory usage in %",

  "Disk usage in %",

  "",
  "",
];

export const VPN_Tooltips = [
  "Quality of service is a score calculated from latency, available bandwidth and number of available user spots on the VPN's Router. 10 is the best score, 0 is the worst score",

  "Available user slots on the VPN's Router ( Total / Available )",

  "Available bandwidth in % on the VPN's Router ( Download / Upload )",

  "Processor usage in percentages on the VPN's Router",

  "Memory usage in percentages on the VPN's Router",

  "Available Gigabits per second of bandwidth on the VPN's Router",
];
