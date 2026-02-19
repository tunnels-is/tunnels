import React, { useEffect, useLayoutEffect, useRef, useState, useMemo, useCallback } from "react";
import GLOBAL_STATE from "../state";
import { Badge } from "@/components/ui/badge";
import { Server, Network, Pencil, Trash2, Plus, Copy, Zap, ZapOff } from "lucide-react";
import { cn } from "@/lib/utils";
import TunnelFormDialog from "./TunnelFormDialog";
import ServerFormDialog from "./ServerFormDialog";

const StatPill = ({ label, value, warn }) => (
  <div className="flex items-center gap-1">
    <span className="text-[10px] uppercase tracking-wider text-white/25">{label}</span>
    <span className={cn(
      "text-[11px] font-mono",
      warn ? "text-amber-400" : "text-white/60"
    )}>{value}</span>
  </div>
);

const ConnectionLine = ({ line, hovered, onHover }) => {
  const { x1, y1, x2, y2, active } = line;
  const midX = (x1 + x2) / 2;
  const d = `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;

  if (active) {
    const color = hovered ? "#2dd4bf" : "#4B7BF5";
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
        stroke="#4B7BF5"
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
        stroke="#4B7BF5"
        strokeWidth="2"
        strokeOpacity="0.5"
        strokeDasharray="8 4"
      />
      <circle cx={x2} cy={y2} r="4" fill="#4B7BF5" opacity="0.6" />
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
        "bg-[#0a0d14]",
        hovered
          ? "border-teal-400/60 shadow-[0_0_15px_rgba(45,212,191,0.15)]"
          : selected
            ? "border-amber-400/60 shadow-[0_0_15px_rgba(251,191,36,0.15)]"
            : isActive
              ? "border-emerald-500/50 shadow-[0_0_15px_rgba(16,185,129,0.12)]"
              : linking
                ? "border-[#1e2433] opacity-40"
                : linked
                  ? "border-[#4B7BF5]/25 hover:border-[#4B7BF5]/40"
                  : "border-[#1e2433] hover:border-[#2e3443]"
      )}
    >
      {/* Action buttons */}
      <div className="absolute top-1.5 right-4 flex gap-0.5 opacity-0 group-hover/node:opacity-100 transition-opacity z-10">
        {isActive ? (
          <button
            onClick={(e) => { e.stopPropagation(); onDisconnect?.(active); }}
            className="p-1 rounded text-emerald-400/70 hover:text-red-400 hover:bg-white/5"
            title="Disconnect"
          >
            <ZapOff className="w-3 h-3" />
          </button>
        ) : (
          <button
            onClick={(e) => { e.stopPropagation(); onConnect?.(tunnel); }}
            className="p-1 rounded text-white/30 hover:text-emerald-400 hover:bg-white/5"
            title="Connect"
          >
            <Zap className="w-3 h-3" />
          </button>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); copyToClipboard(tunnel.Tag, state); }}
          className="p-1 rounded text-white/30 hover:text-white/70 hover:bg-white/5"
          title="Copy Tag"
        >
          <Copy className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit?.(tunnel); }}
          className="p-1 rounded text-white/30 hover:text-white/70 hover:bg-white/5"
          title="Edit"
        >
          <Pencil className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete?.(tunnel); }}
          className="p-1 rounded text-white/30 hover:text-red-400 hover:bg-white/5"
          title="Delete"
        >
          <Trash2 className="w-3 h-3" />
        </button>
      </div>

      <div className="flex items-center gap-2 mb-1">
        <div className={cn(
          "w-2 h-2 rounded-full shrink-0",
          isActive ? "bg-emerald-500 animate-pulse" : "bg-white/20"
        )} />
        <span className="text-[13px] font-semibold text-white truncate">
          {tunnel.Tag}
        </span>
      </div>

      <div className="space-y-0.5 ml-4">
        <div className="text-[11px] text-white/40 font-mono">
          {tunnel.IPv4Address || "no address"}
        </div>
        <div className="text-[11px] text-white/30">
          {tunnel.IFName || "no interface"} &middot; {state.GetEncType(tunnel.EncryptionType)}
        </div>
      </div>

      {/* Expanded details on hover */}
      <div className={cn(
        "overflow-hidden transition-all duration-300 ease-in-out",
        expanded ? "max-h-40 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-40 group-hover/node:opacity-100"
      )}>
        <div className="mt-2 pt-2 border-t border-[#1e2433] space-y-0.5 ml-4">
        {tunnel.ServerID && (
          <div className="text-[10px]">
            <span className="text-white/25 uppercase tracking-wider">Server ID </span>
            <span className="text-white/40 font-mono">{tunnel.ServerID}</span>
          </div>
        )}
        <div className="text-[10px]">
          <span className="text-white/25 uppercase tracking-wider">IPv6 </span>
          <span className="text-white/40 font-mono">{tunnel.IPv6Address || "none"}</span>
        </div>
        <div className="text-[10px]">
          <span className="text-white/25 uppercase tracking-wider">Mask </span>
          <span className="text-white/40 font-mono">{tunnel.NetMask || "none"}</span>
        </div>
        <div className="text-[10px]">
          <span className="text-white/25 uppercase tracking-wider">MTU </span>
          <span className="text-white/40 font-mono">{tunnel.MTU}</span>
          <span className="text-white/25 uppercase tracking-wider ml-2">TxQ </span>
          <span className="text-white/40 font-mono">{tunnel.TxQueueLen}</span>
        </div>
        </div>
      </div>

      <div className={cn(
        "absolute right-0 top-1/2 -translate-y-1/2 translate-x-1/2 w-2.5 h-2.5 rounded-full border-2 z-10",
        hovered
          ? "bg-teal-400 border-[#0a0d14]"
          : selected
            ? "bg-amber-400 border-[#0a0d14]"
            : isActive
              ? "bg-emerald-500 border-[#0a0d14]"
              : linked
                ? "bg-[#4B7BF5]/50 border-[#0a0d14]"
                : "bg-[#1e2433] border-[#0a0d14]"
      )} />
    </div>
  );
});
TunnelNode.displayName = "TunnelNode";

const ServerNode = React.forwardRef(({ server, hasActive, hasLinked, activeStats, state, linking, hovered, expanded, onClick, onMouseEnter, onMouseLeave, onEdit, onConnect, onDisconnect }, ref) => {
  return (
    <div
      ref={ref}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className={cn(
        "group/node relative p-3 rounded-lg border transition-all duration-300",
        "bg-[#0a0d14]",
        hovered
          ? "border-teal-400/60 shadow-[0_0_15px_rgba(45,212,191,0.15)]"
          : linking
            ? "border-emerald-500/40 hover:border-emerald-400/70 hover:shadow-[0_0_15px_rgba(52,211,153,0.15)] cursor-pointer"
            : hasActive
              ? "border-[#4B7BF5]/40 shadow-[0_0_15px_rgba(75,123,245,0.1)]"
              : hasLinked
                ? "border-[#4B7BF5]/25"
                : "border-[#1e2433]"
      )}
    >
      {/* Action buttons */}
      <div className="absolute top-1.5 right-2 flex gap-0.5 opacity-0 group-hover/node:opacity-100 transition-opacity z-10">
        {hasActive ? (
          <button
            onClick={(e) => { e.stopPropagation(); onDisconnect?.(server); }}
            className="p-1 rounded text-emerald-400/70 hover:text-red-400 hover:bg-white/5"
            title="Disconnect"
          >
            <ZapOff className="w-3 h-3" />
          </button>
        ) : (
          <button
            onClick={(e) => { e.stopPropagation(); onConnect?.(server); }}
            className="p-1 rounded text-white/30 hover:text-emerald-400 hover:bg-white/5"
            title="Connect"
          >
            <Zap className="w-3 h-3" />
          </button>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); copyToClipboard(server._id, state); }}
          className="p-1 rounded text-white/30 hover:text-white/70 hover:bg-white/5"
          title="Copy ID"
        >
          <Copy className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit?.(server); }}
          className="p-1 rounded text-white/30 hover:text-white/70 hover:bg-white/5"
          title="Edit"
        >
          <Pencil className="w-3 h-3" />
        </button>
      </div>

      <div className={cn(
        "absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1/2 w-2.5 h-2.5 rounded-full border-2 z-10 transition-colors",
        hovered
          ? "bg-teal-400 border-[#0a0d14]"
          : linking
            ? "bg-emerald-500 border-[#0a0d14]"
            : hasActive
              ? "bg-[#4B7BF5] border-[#0a0d14]"
              : hasLinked
                ? "bg-[#4B7BF5]/50 border-[#0a0d14]"
                : "bg-[#1e2433] border-[#0a0d14]"
      )} />

      <div className="flex items-center gap-2 mb-1">
        <Server className="w-3.5 h-3.5 text-[#4B7BF5]/70 shrink-0" />
        <span className="text-[13px] font-semibold text-white truncate">
          {server.Tag}
        </span>
        <span className="text-[11px] text-white/30 ml-auto shrink-0 pr-5">
          {state.GetCountryName(server.Country)}
        </span>
      </div>

      <div className="space-y-0.5 ml-[22px]">
        <div className="text-[11px] text-white/40 font-mono">
          {server.IP}:{server.Port}
        </div>
      </div>

      {/* Expanded details on hover */}
      <div className={cn(
        "overflow-hidden transition-all duration-300 ease-in-out",
        expanded ? "max-h-40 opacity-100" : "max-h-0 opacity-0 group-hover/node:max-h-40 group-hover/node:opacity-100"
      )}>
        <div className="mt-2 pt-2 border-t border-[#1e2433] space-y-0.5 ml-[22px]">
        <div className="text-[10px]">
          <span className="text-white/25 uppercase tracking-wider">ID </span>
          <span className="text-white/40 font-mono">{server._id}</span>
        </div>
        {server.DataPort && (
          <div className="text-[10px]">
            <span className="text-white/25 uppercase tracking-wider">Data Port </span>
            <span className="text-white/40 font-mono">{server.DataPort}</span>
          </div>
        )}
{server.Groups?.length > 0 && (
          <div className="text-[10px]">
            <span className="text-white/25 uppercase tracking-wider">Groups </span>
            <span className="text-white/40 font-mono">{server.Groups.join(", ")}</span>
          </div>
        )}
        </div>
      </div>

      {activeStats && (
        <div className="mt-2 pt-2 border-t border-[#1e2433] flex gap-3 ml-[22px]">
          <StatPill label="PING" value={Math.floor(activeStats.MS / 1000) + "ms"} />
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
  const [editServer, setEditServer] = useState(null);

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

  // Recalculate lines when hover expands/collapses cards
  useEffect(() => {
    const id = requestAnimationFrame(recalculateLines);
    return () => cancelAnimationFrame(id);
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

  const handleServerClick = (server) => {
    if (!selectedTunnel) return;
    state.changeServerOnTunnelUsingTag(selectedTunnel.Tag, server._id);
    setSelectedTunnel(null);
    setVersion(v => v + 1);
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

  const handleEditServer = (server) => {
    setEditServer(server);
    setServerDialogOpen(true);
  };

  const handleNewServer = () => {
    setEditServer(null);
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
      <div className="flex gap-2 mb-5 items-center">
        <Badge className="bg-[#4B7BF5]/10 text-[#6d9aff] border-0">
          {tunnelCount} {tunnelCount === 1 ? "tunnel" : "tunnels"}
        </Badge>
        <Badge className="bg-[#4B7BF5]/10 text-[#6d9aff] border-0">
          {serverCount} {serverCount === 1 ? "server" : "servers"}
        </Badge>
        <Badge className="bg-emerald-500/10 text-emerald-400 border-0">
          {activeCount} active
        </Badge>

        {selectedTunnel && (
          <div className="flex items-center gap-2 ml-3 text-[12px] text-amber-400 animate-pulse">
            <span>Click a server to assign <strong>{selectedTunnel.Tag}</strong></span>
            <span className="text-white/30 text-[11px]">(Esc to cancel)</span>
          </div>
        )}
      </div>

      {isEmpty && (
        <div className="flex flex-col items-center justify-center py-20 text-white/30 border border-dashed border-[#1e2433] rounded-lg">
          <Network className="w-10 h-10 mb-3 text-white/15" />
          <div className="text-[13px]">No tunnels or servers configured</div>
          <div className="text-[11px] mt-1 text-white/20">Add servers and tunnels to see the network graph</div>
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
            <div className="flex items-center justify-between mb-1 pl-1 pr-1">
              <span className="text-[11px] uppercase tracking-widest text-white/25">
                Tunnels
              </span>
              <button
                onClick={handleNewTunnel}
                className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors"
              >
                <Plus className="w-3 h-3" /> New
              </button>
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
            <div className="flex items-center justify-between mb-1 pl-4 pr-1">
              <span className="text-[11px] uppercase tracking-widest text-white/25">
                Servers
              </span>
              {(state.User?.IsAdmin || state.User?.IsManager) && (
                <button
                  onClick={handleNewServer}
                  className="flex items-center gap-1 text-[11px] text-emerald-400/60 hover:text-emerald-400 transition-colors"
                >
                  <Plus className="w-3 h-3" /> New
                </button>
              )}
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
                onEdit={handleEditServer}
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
        server={editServer}
        onSave={handleServerSaved}
      />
    </div>
  );
};

export default Graph;
