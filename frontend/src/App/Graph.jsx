import React, { useEffect, useLayoutEffect, useRef, useState, useMemo, useCallback } from "react";
import GLOBAL_STATE from "../state";
import { Server, Network, Pencil, Trash2, Copy, Zap, ZapOff, Activity } from "lucide-react";
import { cn } from "@/lib/utils";
import TunnelFormDialog from "./TunnelFormDialog";
import ServerFormDialog from "./ServerFormDialog";

const StatPill = ({ label, value, warn }) => (
  <div className={cn(
    "inline-flex items-baseline gap-1.5 px-2 py-0.5 rounded",
    warn
      ? "bg-[#b45309]/[0.06] ring-1 ring-inset ring-[#b45309]/20"
      : "bg-black/[0.03] ring-1 ring-inset ring-black/[0.06]"
  )}>
    <span className={cn(
      "text-[9px] font-semibold uppercase tracking-[0.1em]",
      warn ? "text-[#b45309]" : "text-[#a3a3a3]"
    )}>{label}</span>
    <span className={cn(
      "text-[11px] font-mono tabular-nums",
      warn ? "text-[#b45309]" : "text-[#0a0a0a]"
    )}>{value}</span>
  </div>
);

// Stat tile — top-of-page counter (icon + value + uppercase label) with paper-raised depth
const StatTile = ({ icon: Icon, label, value, tone = "neutral" }) => {
  const toneClasses = {
    neutral: {
      tile: "bg-white ring-black/[0.08] hover:ring-black/[0.16] shadow-[0_1px_2px_rgba(10,10,10,0.04)] hover:shadow-[0_3px_8px_rgba(10,10,10,0.06)]",
      icon: "text-[#a3a3a3]",
      value: "text-[#0a0a0a]",
      label: "text-[#737373]",
    },
    muted: {
      tile: "bg-white ring-black/[0.08] hover:ring-black/[0.14] shadow-[0_1px_2px_rgba(10,10,10,0.04)]",
      icon: "text-[#c4c4c4]",
      value: "text-[#a3a3a3]",
      label: "text-[#a3a3a3]",
    },
    success: {
      tile: "bg-gradient-to-b from-[#15803d]/[0.06] to-white ring-[#15803d]/25 hover:ring-[#15803d]/40 shadow-[0_1px_2px_rgba(21,128,61,0.06),0_2px_4px_rgba(21,128,61,0.04)] hover:shadow-[0_4px_10px_rgba(21,128,61,0.10)]",
      icon: "text-[#15803d]",
      value: "text-[#15803d]",
      label: "text-[#15803d]/80",
    },
  }[tone];

  return (
    <div className={cn(
      "inline-flex items-center gap-2.5 pl-2.5 pr-3.5 py-1.5 rounded-md ring-1 ring-inset transition-all",
      toneClasses.tile,
    )}>
      <div className={cn(
        "w-7 h-7 flex items-center justify-center rounded-md bg-white shadow-[0_1px_1px_rgba(10,10,10,0.05),inset_0_0_0_1px_rgba(10,10,10,0.05)]",
      )}>
        <Icon className={cn("w-3.5 h-3.5", toneClasses.icon)} strokeWidth={2} />
      </div>
      <div className="flex flex-col items-start leading-none">
        <span className={cn("text-[15px] font-semibold tabular-nums tracking-tight", toneClasses.value)}>
          {value}
        </span>
        <span className={cn(
          "mt-1 text-[9px] font-semibold uppercase tracking-[0.12em]",
          toneClasses.label,
        )}>
          {label}
        </span>
      </div>
    </div>
  );
};

// Compact key/value row used inside the hover-expand details on tunnel/server cards
const MetaRow = ({ label, value, mono = true }) => (
  <div className="flex items-baseline gap-3 py-[3px]">
    <span className="w-[58px] shrink-0 text-[9px] font-semibold uppercase tracking-[0.08em] text-[#a3a3a3]">
      {label}
    </span>
    <span className={cn(
      "min-w-0 flex-1 text-[11px] truncate",
      mono ? "font-mono text-[#525252]" : "text-[#525252]"
    )}>
      {value}
    </span>
  </div>
);


const ConnectionLine = ({ line, hovered, onHover }) => {
  const { x1, y1, x2, y2, active } = line;
  const midX = (x1 + x2) / 2;
  const d = `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;

  if (active) {
    const color = hovered ? "#2dd4bf" : "#1d4ed8";
    return (
      <g>
        <path
          d={d}
          fill="none"
          stroke={color}
          strokeWidth="6"
          strokeOpacity={hovered ? "0.15" : "0.08"}
          filter={hovered ? "url(#glowTeal)" : "url(#glow)"}
        />
        <path
          d={d}
          fill="none"
          stroke={color}
          strokeWidth={hovered ? "2.5" : "2"}
          strokeOpacity={hovered ? "0.9" : "0.7"}
        />
        <circle r="3" fill={color} opacity="0.9">
          <animateMotion dur="3s" repeatCount="indefinite" path={d} />
        </circle>
        <circle r="3" fill={color} opacity="0.5">
          <animateMotion dur="3s" repeatCount="indefinite" path={d} begin="1.5s" />
        </circle>
        <path
          d={d}
          fill="none"
          stroke="transparent"
          strokeWidth="20"
          style={{ pointerEvents: "stroke", cursor: "pointer" }}
          onMouseEnter={() => onHover(line)}
          onMouseLeave={() => onHover(null)}
        />
      </g>
    );
  }

  return (
    <g>
      <path
        d={d}
        fill="none"
        stroke="#1d4ed8"
        strokeWidth="1.5"
        strokeDasharray="6 4"
        strokeOpacity="0.3"
      />
      <path
        d={d}
        fill="none"
        stroke="transparent"
        strokeWidth="16"
        style={{ pointerEvents: "stroke", cursor: "pointer" }}
        onMouseEnter={() => onHover(line)}
        onMouseLeave={() => onHover(null)}
      />
    </g>
  );
};

const DragLine = ({ x1, y1, x2, y2 }) => {
  const midX = (x1 + x2) / 2;
  const d = `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;
  return (
    <g>
      <path
        d={d}
        fill="none"
        stroke="#1d4ed8"
        strokeWidth="2"
        strokeOpacity="0.5"
        strokeDasharray="8 4"
      />
      <circle cx={x2} cy={y2} r="4" fill="#1d4ed8" opacity="0.6" />
    </g>
  );
};

const copyToClipboard = (text, state) => {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      state.successNotification("Copied to clipboard");
    });
  } else {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
    state.successNotification("Copied to clipboard");
  }
};

const TunnelNode = React.forwardRef(({ tunnel, active, state, selected, linking, linked, hovered, expanded, onClick, onMouseEnter, onMouseLeave, onEdit, onDelete, onConnect, onDisconnect }, ref) => {
  const isActive = !!active;

  return (
    <div
      ref={ref}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className={cn(
        "group/node relative p-3 rounded-lg border transition-all duration-300 cursor-pointer",
        "bg-[#ffffff] card-shadow card-shadow-hover",
        hovered
          ? "border-[#0a0a0a]/40 "
          : selected
            ? "border-[#b45309]/50 "
            : isActive
              ? "border-[#15803d]/50 "
              : linking
                ? "border-[#e7e3d7] opacity-40"
                : linked
                  ? "border-[#1d4ed8]/25 hover:border-[#1d4ed8]/40"
                  : "border-[#e7e3d7] hover:border-[#d5d0c0]"
      )}
    >
      {/* Action buttons */}
      <div className="absolute top-1.5 right-4 flex gap-0.5 opacity-0 group-hover/node:opacity-100 transition-opacity z-10">
        {isActive ? (
          <button
            onClick={(e) => { e.stopPropagation(); onDisconnect?.(active); }}
            className="btn-icon btn-icon-state-success btn-icon-danger"
            title="Disconnect"
          >
            <ZapOff className="w-3 h-3" />
          </button>
        ) : (
          <button
            onClick={(e) => { e.stopPropagation(); onConnect?.(tunnel); }}
            className="btn-icon btn-icon-success"
            title="Connect"
          >
            <Zap className="w-3 h-3" />
          </button>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); copyToClipboard(tunnel.Tag, state); }}
          className="btn-icon"
          title="Copy Tag"
        >
          <Copy className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit?.(tunnel); }}
          className="btn-icon"
          title="Edit"
        >
          <Pencil className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete?.(tunnel); }}
          className="btn-icon btn-icon-danger"
          title="Delete"
        >
          <Trash2 className="w-3 h-3" />
        </button>
      </div>

      <div className="flex items-center gap-2 mb-1.5">
        <div className={cn(
          "w-2 h-2 rounded-full shrink-0",
          isActive ? "bg-[#15803d] animate-pulse" : "bg-black/20"
        )} />
        <span className="text-[13px] font-semibold tracking-tight text-[#0a0a0a] truncate">
          {tunnel.Tag}
        </span>
      </div>

      <div className="ml-4 space-y-1">
        <div className="text-[11px] font-mono text-[#525252]">
          {tunnel.IPv4Address || (
            <span className="font-sans italic text-[#a3a3a3]">no address</span>
          )}
        </div>
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[10px] text-[#525252]">
            {tunnel.IFName || (
              <span className="italic text-[#a3a3a3]">no interface</span>
            )}
          </span>
          <span className="text-[9px] font-semibold uppercase tracking-[0.1em] text-[#a3a3a3] px-1.5 py-[1px] rounded bg-black/[0.04] ring-1 ring-inset ring-black/[0.05]">
            {state.GetEncType(tunnel.EncryptionType)}
          </span>
        </div>
      </div>

      {/* Expanded details on hover */}
      <div className={cn(
        "overflow-hidden transition-all duration-300 ease-in-out",
        expanded ? "max-h-44 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-44 group-hover/node:opacity-100"
      )}>
        <div className="mt-2.5 pt-2.5 border-t border-[#e7e3d7] ml-4">
          {tunnel.ServerID && <MetaRow label="Server ID" value={tunnel.ServerID} />}
          <MetaRow label="IPv6" value={tunnel.IPv6Address || "none"} />
          <MetaRow label="Mask" value={tunnel.NetMask || "none"} />
          <div className="flex items-baseline gap-3 py-[3px]">
            <span className="w-[58px] shrink-0 text-[9px] font-semibold uppercase tracking-[0.08em] text-[#a3a3a3]">
              MTU
            </span>
            <span className="text-[11px] font-mono text-[#525252]">{tunnel.MTU}</span>
            <span className="ml-3 text-[9px] font-semibold uppercase tracking-[0.08em] text-[#a3a3a3]">
              TxQ
            </span>
            <span className="text-[11px] font-mono text-[#525252]">{tunnel.TxQueueLen}</span>
          </div>
        </div>
      </div>

      <div className={cn(
        "absolute right-0 top-1/2 -translate-y-1/2 translate-x-1/2 w-2.5 h-2.5 rounded-full border-2 z-10",
        hovered
          ? "bg-[#0a0a0a] border-[#ffffff]"
          : selected
            ? "bg-[#b45309] border-[#ffffff]"
            : isActive
              ? "bg-[#15803d] border-[#ffffff]"
              : linked
                ? "bg-[#1d4ed8]/50 border-[#ffffff]"
                : "bg-[#e7e3d7] border-[#ffffff]"
      )} />
    </div>
  );
});
TunnelNode.displayName = "TunnelNode";

const ServerNode = React.forwardRef(({ server, hasActive, hasLinked, activeStats, state, linking, hovered, expanded, onClick, onMouseEnter, onMouseLeave, onConnect, onDisconnect }, ref) => {
  return (
    <div
      ref={ref}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className={cn(
        "group/node relative p-3 rounded-lg border transition-all duration-300",
        "bg-[#ffffff] card-shadow card-shadow-hover",
        hovered
          ? "border-[#0a0a0a]/40 "
          : linking
            ? "border-[#15803d]/40 hover:border-[#15803d]/70 cursor-pointer"
            : hasActive
              ? "border-[#1d4ed8]/40 "
              : hasLinked
                ? "border-[#1d4ed8]/25"
                : "border-[#e7e3d7]"
      )}
    >
      {/* Action buttons */}
      <div className="absolute top-1.5 right-2 flex gap-0.5 opacity-0 group-hover/node:opacity-100 transition-opacity z-10">
        {hasActive ? (
          <button
            onClick={(e) => { e.stopPropagation(); onDisconnect?.(server); }}
            className="btn-icon btn-icon-state-success btn-icon-danger"
            title="Disconnect"
          >
            <ZapOff className="w-3 h-3" />
          </button>
        ) : (
          <button
            onClick={(e) => { e.stopPropagation(); onConnect?.(server); }}
            className="btn-icon btn-icon-success"
            title="Connect"
          >
            <Zap className="w-3 h-3" />
          </button>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); copyToClipboard(server._id, state); }}
          className="btn-icon"
          title="Copy ID"
        >
          <Copy className="w-3 h-3" />
        </button>
      </div>

      <div className={cn(
        "absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1/2 w-2.5 h-2.5 rounded-full border-2 z-10 transition-colors",
        hovered
          ? "bg-[#0a0a0a] border-[#ffffff]"
          : linking
            ? "bg-[#15803d] border-[#ffffff]"
            : hasActive
              ? "bg-[#1d4ed8] border-[#ffffff]"
              : hasLinked
                ? "bg-[#1d4ed8]/50 border-[#ffffff]"
                : "bg-[#e7e3d7] border-[#ffffff]"
      )} />

      <div className="flex items-center gap-2 mb-1.5">
        <Server className="w-3.5 h-3.5 text-[#1d4ed8]/70 shrink-0" />
        <span className="text-[13px] font-semibold tracking-tight text-[#0a0a0a] truncate">
          {server.Tag}
        </span>
      </div>

      <div className="ml-[22px] space-y-1">
        <div className="text-[11px] font-mono text-[#525252]">
          {server.IP}<span className="text-[#a3a3a3]">:</span>{server.Port}
        </div>
        {state.GetCountryName(server.Country) && (
          <div className="text-[10px] text-[#737373]">{state.GetCountryName(server.Country)}</div>
        )}
      </div>

      {/* Expanded details on hover */}
      <div className={cn(
        "overflow-hidden transition-all duration-300 ease-in-out",
        expanded ? "max-h-44 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-44 group-hover/node:opacity-100"
      )}>
        <div className="mt-2.5 pt-2.5 border-t border-[#e7e3d7] ml-[22px]">
          <MetaRow label="ID" value={server._id} />
          {server.DataPort && <MetaRow label="Data Port" value={server.DataPort} />}
          {server.Groups?.length > 0 && <MetaRow label="Groups" value={server.Groups.join(", ")} mono={false} />}
        </div>
      </div>

      {activeStats && (
        <div className="mt-2.5 pt-2.5 border-t border-[#e7e3d7] flex gap-1.5 ml-[22px]">
          <StatPill label="CPU" value={activeStats.CPU + "%"} warn={activeStats.CPU > 80} />
          <StatPill label="MEM" value={activeStats.MEM + "%"} warn={activeStats.MEM > 80} />
        </div>
      )}
    </div>
  );
});
ServerNode.displayName = "ServerNode";

const Graph = () => {
  const state = GLOBAL_STATE("graph");
  const containerRef = useRef(null);
  const tunnelRefs = useRef({});
  const serverRefs = useRef({});
  const [lines, setLines] = useState([]);
  const [hoveredConn, setHoveredConn] = useState(null); // { tunnelTag, serverId, fromLine? }
  const [selectedTunnel, setSelectedTunnel] = useState(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const [version, setVersion] = useState(0);

  // Dialog state
  const [tunnelDialogOpen, setTunnelDialogOpen] = useState(false);
  const [editTunnel, setEditTunnel] = useState(null);
  const [serverDialogOpen, setServerDialogOpen] = useState(false);

  useEffect(() => {
    state.GetServers();
    state.GetBackendState();
  }, []);

  // Escape key cancels linking
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === "Escape") setSelectedTunnel(null);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const serverMap = useMemo(() => {
    const map = {};
    state.PrivateServers?.forEach(s => { map[s._id] = s; });
    return map;
  }, [state.PrivateServers]);

  const activeMap = useMemo(() => {
    const map = {};
    state.ActiveTunnels?.forEach(at => { map[at.CR?.Tag] = at; });
    return map;
  }, [state.ActiveTunnels]);

  const connections = useMemo(() => {
    return state.Tunnels?.map(t => ({
      tunnel: t,
      server: serverMap[t.ServerID] || null,
      active: activeMap[t.Tag] || null,
    })) || [];
  }, [state.Tunnels, serverMap, activeMap, version]);

  const uniqueServers = useMemo(() => {
    const servers = [];
    const seen = new Set();
    state.PrivateServers?.forEach(s => {
      if (!seen.has(s._id)) {
        seen.add(s._id);
        servers.push(s);
      }
    });
    return servers;
  }, [state.PrivateServers]);

  const serverActiveStats = useMemo(() => {
    const map = {};
    state.ActiveTunnels?.forEach(at => {
      if (at.CR?.ServerID) {
        map[at.CR.ServerID] = at;
      }
    });
    return map;
  }, [state.ActiveTunnels]);

  const serverHasActive = useMemo(() => {
    const map = {};
    connections.forEach(c => {
      if (c.server && c.active) {
        map[c.server._id] = true;
      }
    });
    return map;
  }, [connections]);

  const serverHasLinked = useMemo(() => {
    const map = {};
    connections.forEach(c => {
      if (c.server) {
        map[c.server._id] = true;
      }
    });
    return map;
  }, [connections]);

  // Lookup: tunnelTag → connection, serverId → [connections]
  const connByTunnel = useMemo(() => {
    const map = {};
    connections.forEach(c => {
      if (c.server) map[c.tunnel.Tag] = c;
    });
    return map;
  }, [connections]);

  const connsByServer = useMemo(() => {
    const map = {};
    connections.forEach(c => {
      if (c.server) {
        if (!map[c.server._id]) map[c.server._id] = [];
        map[c.server._id].push(c);
      }
    });
    return map;
  }, [connections]);

  const recalculateLines = useCallback(() => {
    const containerEl = containerRef.current;
    if (!containerEl) return;
    const containerRect = containerEl.getBoundingClientRect();

    const newLines = connections.map(conn => {
      if (!conn.server) return null;
      const tunnelEl = tunnelRefs.current[conn.tunnel.Tag];
      const serverEl = serverRefs.current[conn.server._id];
      if (!tunnelEl || !serverEl) return null;

      const tRect = tunnelEl.getBoundingClientRect();
      const sRect = serverEl.getBoundingClientRect();

      return {
        x1: tRect.right - containerRect.left,
        y1: tRect.top + tRect.height / 2 - containerRect.top,
        x2: sRect.left - containerRect.left,
        y2: sRect.top + sRect.height / 2 - containerRect.top,
        active: conn.active,
        tunnel: conn.tunnel,
        server: conn.server,
      };
    }).filter(Boolean);

    setLines(newLines);
  }, [connections]);

  useLayoutEffect(() => {
    recalculateLines();
  }, [recalculateLines]);

  useEffect(() => {
    const observer = new ResizeObserver(() => {
      recalculateLines();
    });
    if (containerRef.current) observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [recalculateLines]);

  // Recalculate lines continuously during the 300ms hover expand/collapse transition
  useEffect(() => {
    let running = true;
    const loop = () => {
      if (!running) return;
      recalculateLines();
      requestAnimationFrame(loop);
    };
    requestAnimationFrame(loop);
    const timeout = setTimeout(() => { running = false; }, 350);
    return () => { running = false; clearTimeout(timeout); };
  }, [hoveredConn, recalculateLines]);

  const handleContainerMouseMove = (e) => {
    if (!selectedTunnel) return;
    const containerRect = containerRef.current?.getBoundingClientRect();
    if (!containerRect) return;
    setMousePos({
      x: e.clientX - containerRect.left,
      y: e.clientY - containerRect.top,
    });
  };

  const handleTunnelClick = (tunnel) => {
    if (selectedTunnel?.Tag === tunnel.Tag) {
      setSelectedTunnel(null);
    } else {
      setSelectedTunnel(tunnel);
    }
  };

  const handleServerClick = async (server) => {
    if (!selectedTunnel) return;

    const wasActive = !!activeMap[selectedTunnel.Tag];
    const tunnel = selectedTunnel;
    state.changeServerOnTunnelUsingTag(tunnel.Tag, server._id);
    setSelectedTunnel(null);
    setVersion(v => v + 1);

    if (wasActive) {
      state.connectToVPN(tunnel, server).then(() => {
        setVersion(v => v + 1);
      });
    }
  };

  // CRUD handlers
  const handleEditTunnel = (tunnel) => {
    setEditTunnel(tunnel);
    setTunnelDialogOpen(true);
  };

  const handleNewTunnel = async () => {
    await state.createTunnel();
    setVersion(v => v + 1);
  };

  const handleDeleteTunnel = (tunnel) => {
    state.ConfirmAndExecute(
      "success",
      "delete-tunnel",
      10000,
      "",
      "Delete tunnel " + tunnel.Tag + "?",
      async () => {
        await state.v2_TunnelDelete(tunnel);
        setVersion(v => v + 1);
      },
    );
  };

  const handleTunnelSaved = () => {
    state.GetBackendState();
    setVersion(v => v + 1);
  };

  const handleNewServer = () => {
    setServerDialogOpen(true);
  };

  const handleServerSaved = () => {
    state.GetServers();
    setVersion(v => v + 1);
  };

  // Connect/disconnect handlers
  const handleConnectTunnel = (tunnel) => {
    state.ConfirmAndExecute(
      "success",
      "connect",
      10000,
      "",
      "Connect " + tunnel.Tag + "?",
      async () => {
        await state.connectToVPN(tunnel);
        setVersion(v => v + 1);
      },
    );
  };

  const handleDisconnectTunnel = (activeTunnel) => {
    state.ConfirmAndExecute(
      "success",
      "disconnect",
      10000,
      "",
      "Disconnect " + activeTunnel.CR?.Tag + "?",
      async () => {
        await state.disconnectFromVPN(activeTunnel);
        setVersion(v => v + 1);
      },
    );
  };

  const handleConnectServer = (server) => {
    let servertun = undefined;
    let assignedTunnels = 0;
    state.Tunnels?.forEach(c => {
      if (c.ServerID === server._id) {
        servertun = c;
        assignedTunnels++;
      }
    });

    if (assignedTunnels > 1) {
      state.toggleError("Too many tunnels assigned to this server");
      return;
    }

    const connectFn = assignedTunnels < 1
      ? () => state.connectToVPN(undefined, server)
      : () => state.connectToVPN(servertun);

    state.ConfirmAndExecute(
      "success",
      "connect",
      10000,
      "",
      "Connect to " + server.Tag + "?",
      async () => {
        await connectFn();
        setVersion(v => v + 1);
      },
    );
  };

  const handleDisconnectServer = (server) => {
    let activeTunnel = undefined;
    state.ActiveTunnels?.forEach(x => {
      if (x.CR?.ServerID === server._id) activeTunnel = x;
    });
    if (!activeTunnel) return;

    state.ConfirmAndExecute(
      "success",
      "disconnect",
      10000,
      "",
      "Disconnect from " + server.Tag + "?",
      async () => {
        await state.disconnectFromVPN(activeTunnel);
        setVersion(v => v + 1);
      },
    );
  };

  // Get the anchor point for the selected tunnel's drag line
  const getDragLineStart = () => {
    if (!selectedTunnel || !containerRef.current) return null;
    const tunnelEl = tunnelRefs.current[selectedTunnel.Tag];
    if (!tunnelEl) return null;
    const containerRect = containerRef.current.getBoundingClientRect();
    const tRect = tunnelEl.getBoundingClientRect();
    return {
      x: tRect.right - containerRect.left,
      y: tRect.top + tRect.height / 2 - containerRect.top,
    };
  };

  const dragStart = selectedTunnel ? getDragLineStart() : null;

  const tunnelCount = state.Tunnels?.length || 0;
  const serverCount = uniqueServers.length;
  const activeCount = state.ActiveTunnels?.length || 0;
  const containerHeight = Math.max(tunnelCount, serverCount) * 90 + 60;

  const isEmpty = tunnelCount === 0 && serverCount === 0;

  return (
    <div>
      <div className="flex gap-2.5 mb-6 items-center">
        <StatTile icon={Network}  label="Tunnels" value={tunnelCount} />
        <StatTile icon={Server}   label="Servers" value={serverCount} />
        <StatTile icon={Activity} label="Active"  value={activeCount} tone={activeCount > 0 ? "success" : "muted"} />

        {selectedTunnel && (
          <div className="ml-3 inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-[#b45309]/[0.06] ring-1 ring-inset ring-[#b45309]/25">
            <span className="w-1.5 h-1.5 rounded-full bg-[#b45309] animate-pulse" />
            <span className="text-[11px] text-[#b45309]">
              Click a server to assign <span className="font-semibold">{selectedTunnel.Tag}</span>
            </span>
            <span className="text-[10px] text-[#a3a3a3] tracking-wide">ESC to cancel</span>
          </div>
        )}
      </div>

      {isEmpty && (
        <div className="flex flex-col items-center justify-center py-20 text-[#525252] border border-dashed border-[#e7e3d7] rounded-lg">
          <Network className="w-10 h-10 mb-3 text-[#a3a3a3]" />
          <div className="text-[13px]">No tunnels or servers configured</div>
          <div className="text-[11px] mt-1 text-[#a3a3a3]">Add servers and tunnels to see the network graph</div>
        </div>
      )}

      {!isEmpty && (
        <div
          ref={containerRef}
          className="relative"
          style={{ minHeight: containerHeight }}
          onMouseMove={handleContainerMouseMove}
          onClick={(e) => {
            // Click on empty space cancels selection
            if (e.target === e.currentTarget) setSelectedTunnel(null);
          }}
        >
          {/* Tunnels — left column */}
          <div className="absolute left-0 top-0 w-[260px] space-y-3" style={{ zIndex: 2 }}>
            <div className="flex items-center gap-3 mb-2 px-1">
              <button
                onClick={handleNewTunnel}
                className="btn btn-primary btn-xs"
              >
                Create
              </button>
              <div className="flex items-baseline gap-2">
                <span className="label-section !mb-0">Tunnels</span>
                <span className="text-[10px] font-mono tabular-nums text-[#a3a3a3]">{tunnelCount}</span>
              </div>
            </div>
            {state.Tunnels?.map(tunnel => {
              const conn = connByTunnel[tunnel.Tag];
              return (
                <TunnelNode
                  key={tunnel.Tag}
                  ref={el => { tunnelRefs.current[tunnel.Tag] = el; }}
                  tunnel={tunnel}
                  active={activeMap[tunnel.Tag]}
                  state={state}
                  selected={selectedTunnel?.Tag === tunnel.Tag}
                  linking={selectedTunnel && selectedTunnel.Tag !== tunnel.Tag}
                  linked={!!serverMap[tunnel.ServerID]}
                  hovered={hoveredConn?.tunnelTag === tunnel.Tag}
                  expanded={hoveredConn?.tunnelTag === tunnel.Tag && !hoveredConn?.fromLine}
                  onClick={() => handleTunnelClick(tunnel)}
                  onMouseEnter={() => conn && setHoveredConn({ tunnelTag: tunnel.Tag, serverId: conn.server._id })}
                  onMouseLeave={() => setHoveredConn(null)}
                  onEdit={handleEditTunnel}
                  onDelete={handleDeleteTunnel}
                  onConnect={handleConnectTunnel}
                  onDisconnect={handleDisconnectTunnel}
                />
              );
            })}
          </div>

          {/* SVG connections */}
          <svg
            className="absolute inset-0 w-full h-full"
            style={{ zIndex: 1, pointerEvents: "none" }}
          >
            <defs>
              <filter id="glow">
                <feGaussianBlur stdDeviation="4" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
              <filter id="glowTeal">
                <feGaussianBlur stdDeviation="6" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>
            {lines.map((line, i) => (
              <ConnectionLine
                key={`${line.tunnel.Tag}-${line.server?._id}-${i}`}
                line={line}
                hovered={hoveredConn?.tunnelTag === line.tunnel.Tag && hoveredConn?.serverId === line.server?._id}
                onHover={(l) => l ? setHoveredConn({ tunnelTag: l.tunnel.Tag, serverId: l.server?._id, fromLine: true }) : setHoveredConn(null)}
              />
            ))}
            {/* Drag line from selected tunnel to cursor */}
            {dragStart && (
              <DragLine
                x1={dragStart.x}
                y1={dragStart.y}
                x2={mousePos.x}
                y2={mousePos.y}
              />
            )}
          </svg>

          {/* Servers — right column */}
          <div className="absolute right-0 top-0 w-[260px] space-y-3" style={{ zIndex: 2 }}>
            <div className="flex items-center gap-3 mb-2 px-1">
              {(state.User?.IsAdmin || state.User?.IsManager) && (
                <button
                  onClick={handleNewServer}
                  className="btn btn-primary btn-xs"
                >
                  Create
                </button>
              )}
              <div className="flex items-baseline gap-2">
                <span className="label-section !mb-0">Servers</span>
                <span className="text-[10px] font-mono tabular-nums text-[#a3a3a3]">{serverCount}</span>
              </div>
            </div>
            {uniqueServers.map(server => (
              <ServerNode
                key={server._id}
                ref={el => { serverRefs.current[server._id] = el; }}
                server={server}
                hasActive={!!serverHasActive[server._id]}
                hasLinked={!!serverHasLinked[server._id]}
                activeStats={serverActiveStats[server._id]}
                state={state}
                linking={!!selectedTunnel}
                hovered={hoveredConn?.serverId === server._id}
                expanded={hoveredConn?.serverId === server._id && !hoveredConn?.fromLine}
                onClick={() => handleServerClick(server)}
                onMouseEnter={() => {
                  const conns = connsByServer[server._id];
                  if (conns?.length) setHoveredConn({ tunnelTag: conns[0].tunnel.Tag, serverId: server._id });
                }}
                onMouseLeave={() => setHoveredConn(null)}
                onConnect={handleConnectServer}
                onDisconnect={handleDisconnectServer}
              />
            ))}
          </div>

        </div>
      )}

      {/* Tunnel form dialog */}
      <TunnelFormDialog
        open={tunnelDialogOpen}
        onOpenChange={setTunnelDialogOpen}
        tunnel={editTunnel}
        servers={uniqueServers}
        onSave={handleTunnelSaved}
      />

      {/* Server form dialog */}
      <ServerFormDialog
        open={serverDialogOpen}
        onOpenChange={setServerDialogOpen}
        onSave={handleServerSaved}
      />
    </div>
  );
};

export default Graph;
