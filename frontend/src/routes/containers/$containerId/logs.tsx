import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import {
  ArrowLeftIcon,
  CheckIcon,
  ChevronDownIcon,
  CopyIcon,
  DownloadIcon,
  EyeIcon,
  EyeOffIcon,
  FilterIcon,
  PlayIcon,
  RefreshCcwIcon,
  SearchIcon,
  SquareIcon,
  WrapTextIcon
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger
} from "@/components/ui/tooltip";
import {
  getContainerLogsParsed,
  getLogLevelBadgeColor,
  LogEntry,
  LogLevel,
  streamContainerLogsParsed
} from "@/features/containers/api/get-container-logs-parsed";
import { getContainers } from "@/features/containers/api/get-containers";
import {
  formatContainerName,
  formatCreatedDate,
  formatUptime,
  getStateBadgeClass,
  toTitleCase
} from "@/features/containers/components/container-utils";

export const Route = createFileRoute("/containers/$containerId/logs")({
  beforeLoad: async () => {
    const token = localStorage.getItem("logdeck_auth_token");

    // If no token, check if auth is required
    if (!token) {
      try {
        const response = await fetch(
          "http://localhost:8080/api/v1/auth/login",
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ username: "", password: "" }),
          }
        );

        // If 404, auth is disabled - allow access
        if (response.status === 404) {
          return;
        }

        // Auth is enabled but no token - redirect to login
        throw redirect({ to: "/login" });
      } catch (error) {
        // If we can't reach the server, allow access (fail open for development)
        if (error instanceof Error && error.message.includes("redirect")) {
          throw error;
        }
      }
    }
  },
  component: ContainerLogsPage,
});

function ContainerLogsPage() {
  const { containerId } = Route.useParams();
  const navigate = useNavigate();

  const [logLines, setLogLines] = useState(100);
  const [isStreaming, setIsStreaming] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [selectedLevels, setSelectedLevels] = useState<Set<LogLevel>>(
    new Set()
  );
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [wrapText, setWrapText] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [showLabels, setShowLabels] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  // Fetch container info
  const { data: containers } = useQuery({
    queryKey: ["containers"],
    queryFn: getContainers,
  });

  const container = containers?.find((c) => c.id === containerId);

  const scrollToBottom = useCallback(() => {
    if (autoScroll) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [autoScroll]);

  const fetchLogs = useCallback(async () => {
    if (!containerId) return;

    setIsLoadingLogs(true);
    try {
      const logEntries = await getContainerLogsParsed(containerId, {
        tail: logLines,
      });
      setLogs(logEntries);
      setTimeout(scrollToBottom, 100);
    } catch (error) {
      if (error instanceof Error) {
        toast.error(`Failed to fetch logs: ${error.message}`);
      }
      setLogs([]);
    } finally {
      setIsLoadingLogs(false);
    }
  }, [containerId, logLines, scrollToBottom]);

  const startStreaming = useCallback(async () => {
    if (!containerId) return;

    setIsStreaming(true);
    setIsLoadingLogs(true);
    setLogs([]);

    try {
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      const stream = streamContainerLogsParsed(
        containerId,
        {
          tail: logLines,
        },
        abortController.signal
      );

      setIsLoadingLogs(false);

      for await (const entry of stream) {
        if (abortController.signal.aborted) {
          break;
        }

        setLogs((prev) => [...prev, entry]);
        setTimeout(scrollToBottom, 100);
      }
    } catch (error) {
      if (error instanceof Error) {
        const message = error.message.toLowerCase();
        const isAbort =
          error.name === "AbortError" || message.includes("aborted");
        if (!isAbort) {
          toast.error(`Failed to start streaming: ${error.message}`);
        }
      }
      setIsStreaming(false);
    } finally {
      setIsLoadingLogs(false);
      abortControllerRef.current = null;
    }
  }, [containerId, logLines, scrollToBottom]);

  const stopStreaming = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    setIsStreaming(false);
  }, []);

  const handleToggleStream = () => {
    if (isStreaming) {
      stopStreaming();
    } else {
      startStreaming();
    }
  };

  const handleRefresh = () => {
    if (!isStreaming) {
      fetchLogs();
    }
  };

  useEffect(() => {
    fetchLogs();
    return () => {
      stopStreaming();
    };
  }, [fetchLogs, stopStreaming]);

  useEffect(() => {
    if (!isStreaming) {
      fetchLogs();
    }
  }, [logLines, isStreaming, fetchLogs]);

  const handleLogLinesChange = (value: string) => {
    const num = parseInt(value, 10);
    if (!Number.isNaN(num) && num > 0) {
      setLogLines(num);
    }
  };

  const toggleLogLevel = (level: LogLevel) => {
    setSelectedLevels((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(level)) {
        newSet.delete(level);
      } else {
        newSet.add(level);
      }
      return newSet;
    });
  };

  const handleCopyLog = (entry: LogEntry) => {
    const text = entry.message || entry.raw || "";
    navigator.clipboard.writeText(text);
    toast.success("Log entry copied to clipboard");
  };

  const handleDownloadLogs = (format: "json" | "txt") => {
    if (filteredLogs.length === 0) {
      toast.error("No logs to download");
      return;
    }

    const filename = `${container?.names?.[0] || "container"}-logs-${new Date().toISOString()}.${format}`;
    let content: string;
    let mimeType: string;

    if (format === "json") {
      content = JSON.stringify(filteredLogs, null, 2);
      mimeType = "application/json";
    } else {
      content = filteredLogs
        .map((entry) => {
          const timestamp = entry.timestamp
            ? new Date(entry.timestamp).toISOString()
            : "";
          const level = entry.level || "UNKNOWN";
          const message = entry.message || entry.raw || "";
          return `[${timestamp}] [${level}] ${message}`;
        })
        .join("\n");
      mimeType = "text/plain";
    }

    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success(`Logs downloaded as ${format.toUpperCase()}`);
  };

  const filteredLogs = useMemo(() => {
    return logs.filter((entry) => {
      // Filter by search text
      if (searchText) {
        const message = (entry.message || entry.raw || "").toLowerCase();
        if (!message.includes(searchText.toLowerCase())) {
          return false;
        }
      }

      // Filter by log level
      if (selectedLevels.size > 0 && entry.level) {
        if (!selectedLevels.has(entry.level)) {
          return false;
        }
      }

      return true;
    });
  }, [logs, searchText, selectedLevels]);

  const availableLogLevels = useMemo(() => {
    const levels = new Set<LogLevel>();
    logs.forEach((entry) => {
      if (entry.level) {
        levels.add(entry.level);
      }
    });
    return Array.from(levels).sort();
  }, [logs]);

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-6 max-w-7xl">
        <div className="space-y-6">
          {/* Header */}
          <div className="flex items-center gap-4">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => navigate({ to: "/" })}
                >
                  <ArrowLeftIcon className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Back to Dashboard</TooltipContent>
            </Tooltip>
            <div className="flex-1">
              <h1 className="text-2xl font-bold">Container Logs</h1>
              {container && (
                <p className="text-sm text-muted-foreground">
                  {container.names?.[0]?.replace(/^\//, "") || containerId}
                </p>
              )}
            </div>
          </div>

          {/* Container Info Card */}
          {container && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Container Details</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Name
                      </span>
                      <p className="font-medium">
                        {formatContainerName(container.names)}
                      </p>
                    </div>
                    <div className="md:col-span-2">
                      <span className="text-muted-foreground block mb-1">
                        ID
                      </span>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <p className="font-mono text-xs truncate cursor-help">
                            {container.id}
                          </p>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-md">
                          {container.id}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Image
                      </span>
                      <p className="font-medium">{container.image}</p>
                    </div>
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        State
                      </span>
                      <Badge
                        className={`${getStateBadgeClass(container.state)} border-0`}
                      >
                        {toTitleCase(container.state)}
                      </Badge>
                    </div>
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Status
                      </span>
                      <p className="font-medium">{container.status}</p>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Uptime
                      </span>
                      <p className="font-medium">
                        {formatUptime(container.created)}
                      </p>
                    </div>
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Created
                      </span>
                      <p className="font-medium">
                        {formatCreatedDate(container.created)}
                      </p>
                    </div>
                    <div>
                      <span className="text-muted-foreground block mb-1">
                        Command
                      </span>
                      <p className="font-mono text-xs break-all">
                        {container.command}
                      </p>
                    </div>
                  </div>

                  {/* Labels Section */}
                  {container.labels &&
                    Object.keys(container.labels).length > 0 && (
                      <div className="space-y-2 border-t pt-4">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setShowLabels((value) => !value)}
                          className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
                        >
                          <ChevronDownIcon
                            className={`mr-2 size-4 transition-transform ${
                              showLabels ? "rotate-180" : ""
                            }`}
                          />
                          {showLabels ? "Hide" : "Show"} container labels (
                          {Object.keys(container.labels).length})
                        </Button>
                        {showLabels && (
                          <div className="max-h-[200px] space-y-2 overflow-y-auto rounded-md border bg-muted/30 p-3">
                            {Object.entries(container.labels).map(
                              ([key, value]) => (
                                <div
                                  key={key}
                                  className="rounded-md bg-background p-2 text-xs"
                                >
                                  <div className="mb-1 font-semibold text-foreground">
                                    {key}
                                  </div>
                                  <div className="break-all font-mono text-muted-foreground">
                                    {value}
                                  </div>
                                </div>
                              )
                            )}
                          </div>
                        )}
                      </div>
                    )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Logs Card */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">
                  Logs
                  {filteredLogs.length !== logs.length && (
                    <span className="ml-2 text-xs text-muted-foreground font-normal">
                      ({filteredLogs.length} of {logs.length})
                    </span>
                  )}
                </CardTitle>
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-2">
                    <Label
                      htmlFor="log-lines"
                      className="text-xs text-muted-foreground"
                    >
                      Lines
                    </Label>
                    <Input
                      id="log-lines"
                      type="number"
                      min="1"
                      value={logLines}
                      onChange={(e) => handleLogLinesChange(e.target.value)}
                      disabled={isStreaming}
                      className="h-8 w-20 text-xs"
                    />
                  </div>
                  <Button
                    variant={isStreaming ? "default" : "outline"}
                    size="sm"
                    onClick={handleToggleStream}
                    disabled={isLoadingLogs && !isStreaming}
                  >
                    {isStreaming ? (
                      <>
                        <SquareIcon className="mr-2 size-4" />
                        Stop
                      </>
                    ) : (
                      <>
                        <PlayIcon className="mr-2 size-4" />
                        Stream
                      </>
                    )}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleRefresh}
                    disabled={isStreaming || isLoadingLogs}
                  >
                    <RefreshCcwIcon className="mr-2 size-4" />
                  </Button>
                </div>
              </div>

              {/* Search and Filter Controls */}
              <div className="flex flex-wrap items-center gap-2 pt-4">
                <div className="relative flex-1 min-w-[200px]">
                  <SearchIcon className="absolute left-2 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
                  <Input
                    placeholder="Search logs..."
                    value={searchText}
                    onChange={(e) => setSearchText(e.target.value)}
                    className="pl-8 h-9 text-xs"
                  />
                </div>

                <Popover open={showFilters} onOpenChange={setShowFilters}>
                  <PopoverTrigger asChild>
                    <Button variant="outline" size="sm" className="h-9">
                      <FilterIcon className="mr-2 size-4" />
                      Filter
                      {selectedLevels.size > 0 && (
                        <Badge
                          variant="outline"
                          className="ml-2 px-1 py-0 h-4 text-xs"
                        >
                          {selectedLevels.size}
                        </Badge>
                      )}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent align="end" className="w-56">
                    <div className="space-y-3">
                      <div>
                        <h4 className="text-sm font-medium mb-2">Log Levels</h4>
                        <div className="space-y-2">
                          {availableLogLevels.length === 0 ? (
                            <p className="text-xs text-muted-foreground">
                              No log levels available
                            </p>
                          ) : (
                            availableLogLevels.map((level) => (
                              <label
                                key={level}
                                className="flex items-center gap-2 cursor-pointer"
                              >
                                <button
                                  type="button"
                                  onClick={() => toggleLogLevel(level)}
                                  className={`size-4 rounded border flex items-center justify-center ${
                                    selectedLevels.has(level)
                                      ? "bg-primary border-primary"
                                      : "border-input"
                                  }`}
                                >
                                  {selectedLevels.has(level) && (
                                    <CheckIcon className="size-3 text-primary-foreground" />
                                  )}
                                </button>
                                <Badge
                                  variant="outline"
                                  className={`text-xs ${getLogLevelBadgeColor(level)}`}
                                >
                                  {level}
                                </Badge>
                              </label>
                            ))
                          )}
                        </div>
                      </div>
                      {selectedLevels.size > 0 && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setSelectedLevels(new Set())}
                          className="w-full"
                        >
                          Clear Filters
                        </Button>
                      )}
                    </div>
                  </PopoverContent>
                </Popover>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setShowTimestamps(!showTimestamps)}
                      className="h-9"
                    >
                      {showTimestamps ? (
                        <EyeIcon className="size-4" />
                      ) : (
                        <EyeOffIcon className="size-4" />
                      )}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    {showTimestamps ? "Hide" : "Show"} timestamps
                  </TooltipContent>
                </Tooltip>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setAutoScroll(!autoScroll)}
                      className="h-9"
                    >
                      <ChevronDownIcon
                        className={`size-4 ${autoScroll ? "text-primary" : ""}`}
                      />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    Auto-scroll: {autoScroll ? "On" : "Off"}
                  </TooltipContent>
                </Tooltip>

                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setWrapText(!wrapText)}
                      className="h-9"
                    >
                      <WrapTextIcon
                        className={`size-4 ${wrapText ? "text-primary" : ""}`}
                      />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    Text wrap: {wrapText ? "On" : "Off"}
                  </TooltipContent>
                </Tooltip>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm" className="h-9">
                      <DownloadIcon className="mr-2 size-4" />
                      Download
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      onClick={() => handleDownloadLogs("json")}
                    >
                      Download as JSON
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => handleDownloadLogs("txt")}>
                      Download as TXT
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              <div className="h-[calc(100vh-400px)] min-h-[400px] w-full overflow-auto">
                {isLoadingLogs && logs.length === 0 ? (
                  <div className="flex items-center justify-center py-8 text-muted-foreground">
                    <Spinner className="mr-2 size-4" />
                    Loading logs...
                  </div>
                ) : logs.length === 0 ? (
                  <div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
                    No logs available
                  </div>
                ) : filteredLogs.length === 0 ? (
                  <div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
                    No logs match the current filters
                  </div>
                ) : (
                  <div
                    className={`font-mono text-xs min-w-full ${wrapText ? "" : "w-fit"}`}
                  >
                    {filteredLogs
                      .filter((entry) => entry.message?.trim())
                      .map((entry, index) => {
                        const timestamp = entry.timestamp
                          ? new Date(entry.timestamp)
                          : null;
                        const dateLabel = timestamp
                          ? `${timestamp.toLocaleDateString("en-GB", {
                              day: "2-digit",
                              month: "2-digit",
                              year: "numeric",
                            })} ${timestamp.toLocaleTimeString("en-US", {
                              hour12: false,
                              hour: "2-digit",
                              minute: "2-digit",
                              second: "2-digit",
                            })}`
                          : "—";

                        return (
                          <div
                            key={`${entry.timestamp}-${index}`}
                            className={`group flex items-start gap-3 px-4 py-1.5 hover:bg-muted/50 ${
                              wrapText ? "" : "whitespace-nowrap"
                            } ${index % 2 === 0 ? "bg-muted/30" : ""}`}
                          >
                            {showTimestamps && (
                              <span className="text-muted-foreground shrink-0 text-[11px]">
                                {dateLabel}
                              </span>
                            )}
                            <Badge
                              variant="outline"
                              className={`shrink-0 text-xs px-1.5 py-0 h-5 ${getLogLevelBadgeColor(entry.level ?? "UNKNOWN")}`}
                            >
                              {entry.level ?? "UNKNOWN"}
                            </Badge>
                            <span
                              className={`text-foreground flex-1 ${wrapText ? "break-words" : ""}`}
                            >
                              {entry.message ?? ""}
                            </span>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <button
                                  onClick={() => handleCopyLog(entry)}
                                  className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 hover:bg-muted rounded"
                                >
                                  <CopyIcon className="size-3 text-muted-foreground" />
                                </button>
                              </TooltipTrigger>
                              <TooltipContent>Copy log entry</TooltipContent>
                            </Tooltip>
                          </div>
                        );
                      })}
                    <div ref={logsEndRef} />
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
