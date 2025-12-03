import { useEffect, useState, useRef, useMemo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ChevronLeft, ChevronRight, Search, Trash2 } from "lucide-react";

export default function LogsPage() {
  const [logs, setLogs] = useState([]);
  const [connectionStatus, setConnectionStatus] = useState("connecting");
  const [searchTerm, setSearchTerm] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const logsPerPage = 50;
  const logsEndRef = useRef(null);
  const wsRef = useRef(null);

  useEffect(() => {
    const connectWebSocket = () => {
      try {
        let host = window.location.hostname;
        const ws = new WebSocket(`ws://${host}/logs`);
        wsRef.current = ws;

        ws.onopen = () => {
          setConnectionStatus("connected");
        };

        ws.onmessage = (event) => {
          const logLine = event.data;
          const parsed = parseLogLine(logLine);
          if (parsed) {
            setLogs((prev) => [...prev, parsed]);
          }
        };

        ws.onerror = () => {
          setConnectionStatus("error");
        };

        ws.onclose = () => {
          setConnectionStatus("disconnected");
          // Attempt to reconnect after 3 seconds
          setTimeout(() => {
            setConnectionStatus("connecting");
            connectWebSocket();
          }, 3000);
        };
      } catch (error) {
        setConnectionStatus("error");
      }
    };

    connectWebSocket();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);
  useEffect(() => {
    // Auto-scroll to bottom when new logs arrive
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  const filteredLogs = useMemo(() => {
    if (!searchTerm) return logs;

    const lowerSearch = searchTerm.toLowerCase();
    return logs.filter(
      (log) =>
        log.functionName.toLowerCase().includes(lowerSearch) ||
        log.identifier.toLowerCase().includes(lowerSearch) ||
        log.level.toLowerCase().includes(lowerSearch) ||
        log.timestamp.includes(lowerSearch)
    );
  }, [logs, searchTerm]);

  const totalPages = Math.ceil(filteredLogs.length / logsPerPage);
  const startIndex = (currentPage - 1) * logsPerPage;
  const endIndex = startIndex + logsPerPage;
  const paginatedLogs = filteredLogs.slice(startIndex, endIndex);

  const parseLogLine = (line) => {
    // Format: 11-26 22:45:05 || DEBUG/INFO/ERROR || LaunchTunnels || LogMapCleaner
    const parts = line.split("||").map((part) => part.trim());

    if (parts.length !== 4) {
      return null;
    }

    const [timestamp, level, functionName, identifier] = parts;

    if (!["DEBUG", "INFO", "ERROR"].includes(level)) {
      return null;
    }

    return {
      timestamp,
      level: level,
      functionName,
      identifier,
      raw: line,
    };
  };

  const clearLogs = () => {
    setLogs([]);
    setSearchTerm("");
    setCurrentPage(1);
  };

  const getStatusColor = () => {
    switch (connectionStatus) {
      case "connected":
        return "bg-green-500";
      case "connecting":
        return "bg-yellow-500";
      case "disconnected":
      case "error":
        return "bg-red-500";
    }
  };

  const getStatusText = () => {
    switch (connectionStatus) {
      case "connected":
        return "Connected";
      case "connecting":
        return "Connecting...";
      case "disconnected":
        return "Disconnected";
      case "error":
        return "Error";
    }
  };

  const getLevelColor = (level) => {
    switch (level) {
      case "DEBUG":
        return "text-blue-400";
      case "INFO":
        return "text-green-400";
      case "ERROR":
        return "text-red-400";
      default:
        return "text-foreground";
    }
  };

  return (
    <div className="w-full p-4 mt-20">
      <div className="flex flex-row items-center justify-between space-y-0 pb-4">
        <h3 className="text-xl font-semibold">Server Logs</h3>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className={`h-2 w-2 rounded-full ${getStatusColor()}`} />
            <span className="text-sm text-muted-foreground">
              {getStatusText()}
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={clearLogs}
            disabled={logs.length === 0}
          >
            <Trash2 className="h-4 w-4 mr-2" />
            Clear
          </Button>
        </div>
      </div>
      <div className="space-y-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search logs by function, identifier, level, or timestamp..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9"
          />
        </div>

        <div className="h-[600px] overflow-y-auto rounded-lg border bg-black/95 p-4 font-mono text-sm">
          {filteredLogs.length === 0 ? (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              {logs.length === 0
                ? "Waiting for logs..."
                : "No logs match your search."}
            </div>
          ) : (
            <div className="space-y-1">
              {paginatedLogs.map((log, index) => (
                <div
                  key={startIndex + index}
                  className="flex items-start gap-3 text-xs leading-relaxed"
                >
                  <span className="text-gray-400 whitespace-nowrap">
                    {log.timestamp}
                  </span>
                  <Badge
                    variant="outline"
                    className={`${getLevelColor(
                      log.level
                    )} min-w-[60px] justify-center border-0`}
                  >
                    {log.level}
                  </Badge>
                  <span className="text-cyan-400">{log.functionName}</span>
                  <span className="text-gray-300">{log.identifier}</span>
                </div>
              ))}
              <div ref={logsEndRef} />
            </div>
          )}
        </div>

        {filteredLogs.length > 0 && (
          <div className="flex items-center justify-between">
            <div className="text-sm text-muted-foreground">
              Showing {startIndex + 1}-{Math.min(endIndex, filteredLogs.length)}{" "}
              of {filteredLogs.length} logs
              {searchTerm && ` (filtered from ${logs.length} total)`}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage((prev) => Math.max(1, prev - 1))}
                disabled={currentPage === 1}
              >
                <ChevronLeft className="h-4 w-4" />
                Previous
              </Button>
              <span className="text-sm text-muted-foreground">
                Page {currentPage} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  setCurrentPage((prev) => Math.min(totalPages, prev + 1))
                }
                disabled={currentPage === totalPages}
              >
                Next
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
