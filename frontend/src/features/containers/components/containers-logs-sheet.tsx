import { useCallback, useEffect, useRef, useState } from "react";
import {
  ChevronDownIcon,
  ExternalLinkIcon,
  PlayIcon,
  RefreshCcwIcon,
  SquareIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import {
  getContainerLogsParsed,
  getLogLevelBadgeColor,
  streamContainerLogsParsed,
  type LogEntry,
} from "../api/get-container-logs-parsed";
import type { ContainerInfo } from "../types";

import {
  formatContainerName,
  formatCreatedDate,
  formatUptime,
  getStateBadgeClass,
  toTitleCase,
} from "./container-utils";

interface ContainersLogsSheetProps {
  container: ContainerInfo | null;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ContainersLogsSheet({
  container,
  isOpen,
  onOpenChange,
}: ContainersLogsSheetProps) {
  const [showLabels, setShowLabels] = useState(false);
  const [logLines, setLogLines] = useState(100);
  const [isStreaming, setIsStreaming] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const fetchLogs = useCallback(async () => {
    if (!container) return;

    setIsLoadingLogs(true);
    try {
      const logEntries = await getContainerLogsParsed(container.id, {
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
  }, [container, logLines]);

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

        setLogs((prev) => [...prev, entry]);
        setTimeout(scrollToBottom, 100);
      }
    } catch (error) {
      if (error instanceof Error) {
        const message = error.message.toLowerCase();
        const isAbort = error.name === "AbortError" || message.includes("aborted");
        if (!isAbort) {
          toast.error(`Failed to start streaming: ${error.message}`);
        }
      }
      setIsStreaming(false);
    } finally {
      setIsLoadingLogs(false);
      abortControllerRef.current = null;
    }
  }, [container, logLines]);

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
      stopStreaming();
      setLogs([]);
    }
  }, [isOpen, stopStreaming]);

  useEffect(() => {
    setShowLabels(false);
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
  }, [logLines, container, isOpen, isStreaming, fetchLogs]);

  const handleLogLinesChange = (value: string) => {
    const num = parseInt(value, 10);
    if (!isNaN(num) && num > 0) {
      setLogLines(num);
    }
  };

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
                      onClick={() =>
                        window.open(`/containers/${container.id}/logs`, "_blank")
                      }
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
                  </div>
                </div>
              </CardContent>
            </Card>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">Logs</h3>
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
                        Stop Stream
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
                    Refresh
                  </Button>
                </div>
              </div>
              <Card>
                <CardContent className="p-0">
                  <div className="h-[400px] w-full overflow-auto">
                    {isLoadingLogs && logs.length === 0 ? (
                      <div className="flex items-center justify-center py-8 text-muted-foreground">
                        <Spinner className="mr-2 size-4" />
                        Loading logs...
                      </div>
                    ) : logs.length === 0 ? (
                      <div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
                        No logs available
                      </div>
                    ) : (
                      <div className="font-mono text-xs min-w-full w-fit">
                        {logs
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
                                key={index}
                                className={`flex items-start gap-3 px-4 py-1.5 whitespace-nowrap ${
                                  index % 2 === 0 ? "bg-muted/30" : ""
                                }`}
                              >
                                <span className="text-muted-foreground shrink-0">
                                  {dateLabel}
                                </span>
                                <Badge
                                  variant="outline"
                                  className={`shrink-0 text-xs px-1.5 py-0 h-5 ${getLogLevelBadgeColor(entry.level ?? "UNKNOWN")}`}
                                >
                                  {entry.level ?? "UNKNOWN"}
                                </Badge>
                                <span className="text-foreground">
                                  {entry.message ?? ""}
                                </span>
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
        )}
      </SheetContent>
    </Sheet>
  );
}
