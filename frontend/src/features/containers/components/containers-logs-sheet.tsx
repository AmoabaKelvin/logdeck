import { useVirtualizer } from "@tanstack/react-virtual";
import {
  CheckIcon,
  ChevronDownIcon,
  CopyIcon,
  DownloadIcon,
  ExternalLinkIcon,
  EyeIcon,
  EyeOffIcon,
  FilterIcon,
  PlayIcon,
  RefreshCcwIcon,
  SearchIcon,
  SquareIcon,
  WrapTextIcon
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState
} from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger
} from "@/components/ui/tooltip";

import {
  getContainerLogsParsed,
  getLogLevelBadgeColor,
  streamContainerLogsParsed
} from "../api/get-container-logs-parsed";

import {
  formatContainerName,
  formatCreatedDate,
  formatUptime,
  getContainerUrlIdentifier,
  getStateBadgeClass,
  toTitleCase
} from "./container-utils";
import { EnvironmentVariables } from "./environment-variables";

import type { LogEntry, LogLevel } from "@/types/logs";
import type { ContainerInfo } from "../types";
interface ContainersLogsSheetProps {
  container: ContainerInfo | null;
  isOpen: boolean;
  isReadOnly?: boolean;
  onOpenChange: (open: boolean) => void;
  onContainerRecreated?: (newContainerId: string) => void;
}

export function ContainersLogsSheet({
  container,
  isOpen,
  isReadOnly = false,
  onOpenChange,
  onContainerRecreated,
}: ContainersLogsSheetProps) {
  const [showLabels, setShowLabels] = useState(false);
  const [showEnvVariables, setShowEnvVariables] = useState(false);
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
  const abortControllerRef = useRef<AbortController | null>(null);
  const parentRef = useRef<HTMLDivElement>(null);
  const logLinesInputId = useId();

  const scrollToBottom = useCallback(() => {
    if (autoScroll && parentRef.current) {
      // For virtualized list, scroll the parent container to bottom
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
  }, [autoScroll]);

  const fetchLogs = useCallback(async () => {
    if (!container) return;

    setIsLoadingLogs(true);
    try {
      const logEntries = await getContainerLogsParsed(container.id, {
        tail: logLines,
      });
      setLogs(logEntries as LogEntry[]);
      setTimeout(scrollToBottom, 100);
    } catch (error) {
      if (error instanceof Error) {
        toast.error(`Failed to fetch logs: ${error.message}`);
      }
      setLogs([]);
    } finally {
      setIsLoadingLogs(false);
    }
  }, [container, logLines, scrollToBottom]);

  const startStreaming = useCallback(async () => {
    if (!container) return;

    setIsStreaming(true);
    setIsLoadingLogs(true);
    setLogs([]);

    try {
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      const stream = streamContainerLogsParsed(
        container.id,
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

        setLogs((prev) => [...prev, entry as LogEntry]);
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
  }, [container, logLines, scrollToBottom]);

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
    if (!isOpen) {
      setShowLabels(false);
      setShowEnvVariables(false);
      stopStreaming();
      setLogs([]);
    }
  }, [isOpen, stopStreaming]);

  useEffect(() => {
    setShowLabels(false);
    setShowEnvVariables(false);
    stopStreaming();
    setLogs([]);

    if (container && isOpen) {
      fetchLogs();
    }
  }, [container, isOpen, fetchLogs, stopStreaming]);

  useEffect(() => {
    if (container && isOpen && !isStreaming) {
      fetchLogs();
    }
  }, [container, isOpen, isStreaming, fetchLogs]);

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
    navigator.clipboard
      .writeText(text)
      .then(() => {
        toast.success("Log entry copied to clipboard");
      })
      .catch(() => {
        toast.error("Failed to copy to clipboard");
      });
  };

  const handleDownloadLogs = (format: "json" | "txt") => {
    if (filteredLogs.length === 0) {
      toast.error("No logs to download");
      return;
    }

    const containerName = (container?.names?.[0] || "container")
      .replace(/^\//, "")
      .replace(/[/\\:*?"<>|]/g, "-");
    const timestamp = new Date()
      .toISOString()
      .replace(/:/g, "-")
      .replace(/\..+/, "");
    const filename = `${containerName}-logs-${timestamp}.${format}`;
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

  // Virtualization setup
  const rowVirtualizer = useVirtualizer({
    count: filteredLogs.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => (wrapText ? 60 : 36),
    overscan: 5,
  });

  return (
    <Sheet open={isOpen} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-3xl w-full overflow-y-auto p-6">
        <SheetHeader>
          <SheetTitle>Container Logs</SheetTitle>
          <SheetDescription>
            {container && formatContainerName(container.names)}
          </SheetDescription>
        </SheetHeader>

        {container && (
          <div className="mt-6 space-y-6 pr-2">
            <Card>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-medium">Container Details</h3>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        const identifier = getContainerUrlIdentifier(container);
                        window.open(
                          `/containers/${encodeURIComponent(identifier)}/logs`,
                          "_blank"
                        );
                      }}
                    >
                      <ExternalLinkIcon className="mr-2 size-4" />
                      Open in new tab
                    </Button>
                  </div>

                  <div className="grid gap-3 text-sm">
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Name</span>
                      <span className="col-span-2 font-medium">
                        {formatContainerName(container.names)}
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">ID</span>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className="col-span-2 font-mono text-xs truncate cursor-help">
                            {container.id}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-md">
                          {container.id}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Image</span>
                      <span className="col-span-2 font-medium">
                        {container.image}
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">State</span>
                      <span className="col-span-2">
                        <Badge
                          className={`${getStateBadgeClass(container.state)} border-0`}
                        >
                          {toTitleCase(container.state)}
                        </Badge>
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Status</span>
                      <span className="col-span-2 font-medium">
                        {container.status}
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Uptime</span>
                      <span className="col-span-2 font-medium">
                        {formatUptime(container.created)}
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Created</span>
                      <span className="col-span-2 font-medium">
                        {formatCreatedDate(container.created)}
                      </span>
                    </div>
                    <div className="grid grid-cols-3 gap-4">
                      <span className="text-muted-foreground">Command</span>
                      <span className="col-span-2 font-mono text-xs break-all">
                        {container.command}
                      </span>
                    </div>
                    {/* Labels Section */}
                    {container.labels &&
                      Object.keys(container.labels).length > 0 && (
                        <div className="space-y-2 border-t pt-2">
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

                    {/* Environment Variables Section */}
                    <div className="space-y-2 border-t pt-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setShowEnvVariables((value) => !value)}
                        className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
                      >
                        <ChevronDownIcon
                          className={`mr-2 size-4 transition-transform ${
                            showEnvVariables ? "rotate-180" : ""
                          }`}
                        />
                        {showEnvVariables ? "Hide" : "Show"} environment variables
                      </Button>
                      {showEnvVariables && (
                        <div className="max-h-[300px] overflow-y-auto">
                          <EnvironmentVariables
                            containerId={container.id}
                            isReadOnly={isReadOnly}
                            onContainerIdChange={onContainerRecreated}
                          />
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">
                  Logs
                  {filteredLogs.length !== logs.length && (
                    <span className="ml-2 text-xs text-muted-foreground">
                      ({filteredLogs.length} of {logs.length})
                    </span>
                  )}
                </h3>
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-2">
                    <Label
                      htmlFor={logLinesInputId}
                      className="text-xs text-muted-foreground"
                    >
                      Lines
                    </Label>
                    <Input
                      id={logLinesInputId}
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
              <div className="flex flex-wrap items-center gap-2">
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
              <Card>
                <CardContent className="p-0">
                  <div
                    ref={parentRef}
                    className="h-[400px] w-full overflow-auto"
                  >
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
                        style={{
                          height: `${rowVirtualizer.getTotalSize()}px`,
                          width: "100%",
                          position: "relative",
                        }}
                        className={`font-mono text-xs ${wrapText ? "" : "w-fit min-w-full"}`}
                      >
                        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                          const entry = filteredLogs[virtualRow.index];
                          if (!entry.message?.trim()) return null;

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
                              key={virtualRow.key}
                              data-index={virtualRow.index}
                              ref={rowVirtualizer.measureElement}
                              style={{
                                position: "absolute",
                                top: 0,
                                left: 0,
                                width: "100%",
                                transform: `translateY(${virtualRow.start}px)`,
                              }}
                              className={`group flex items-start gap-3 px-4 py-1.5 hover:bg-muted/50 ${
                                wrapText ? "" : "whitespace-nowrap"
                              } ${virtualRow.index % 2 === 0 ? "bg-muted/30" : ""}`}
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
                                    type="button"
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
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
