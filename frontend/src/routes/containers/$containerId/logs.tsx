import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  ArrowDownIcon,
  ArrowDownToLineIcon,
  ArrowLeftIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CopyIcon,
  DownloadIcon,
  PlayIcon,
  RefreshCcwIcon,
  SearchIcon,
  SquareIcon,
  TerminalIcon,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select";
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
} from "@/features/containers/api/get-container-logs-parsed";
import { getContainers } from "@/features/containers/api/get-containers";
import {
  formatContainerName,
  formatCPUPercent,
  formatCreatedDate,
  formatMemoryStats,
  formatUptime,
  getStateBadgeClass,
  toTitleCase
} from "@/features/containers/components/container-utils";
import { EnvironmentVariables } from "@/features/containers/components/environment-variables";
import { SelectionActionBar } from "@/features/containers/components/selection-action-bar";
import { Terminal } from "@/features/containers/components/terminal";
import { useContainerStats } from "@/features/containers/hooks/use-container-stats";
import { requireAuthIfEnabled } from "@/lib/auth-guard";
import { isJsonString } from "@/lib/json-format";
import { escapeRegExp } from "@/lib/utils";

import type {
  LogEntry,
  LogLevel,
} from "@/features/containers/api/get-container-logs-parsed";
import { CollapsibleJson } from "@/features/containers/components/collapsible-json";
export const Route = createFileRoute("/containers/$containerId/logs")({
  beforeLoad: async () => {
    await requireAuthIfEnabled();
  },
  component: ContainerLogsPage,
});

function ContainerLogsPage() {
  const { containerId: encodedContainerId } = Route.useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [logLines, setLogLines] = useState(100);
  const [isStreaming, setIsStreaming] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [excludeMatches, setExcludeMatches] = useState(false);
  const [selectedLevels, setSelectedLevels] = useState<Set<LogLevel>>(
    new Set()
  );
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [wrapText, setWrapText] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [showLabels, setShowLabels] = useState(false);
  const [showEnvVariables, setShowEnvVariables] = useState(false);
  const [showTerminal, setShowTerminal] = useState(false);
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0);
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(new Set());
  const [pinnedLogIndices, setPinnedLogIndices] = useState<Set<number>>(new Set());
  const [currentPinnedIndex, setCurrentPinnedIndex] = useState(0);
  const [expandedJsonRows, setExpandedJsonRows] = useState<Set<number>>(new Set());
  const [lastClickedIndex, setLastClickedIndex] = useState<number | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const parentRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const autoScrollRef = useRef(autoScroll);
  const logLinesInputId = useId();

  // Keep ref in sync with state to avoid stale closures in setTimeout
  useEffect(() => {
    autoScrollRef.current = autoScroll;
  }, [autoScroll]);

  // Decode the URL parameter (could be name or ID)
  const containerIdentifier = decodeURIComponent(encodedContainerId);

  // Fetch container info
  const { data: containersData } = useQuery({
    queryKey: ["containers"],
    queryFn: getContainers,
  });
  const { statsMap } = useContainerStats();

  const containers = containersData?.containers ?? [];

  // Find container by name (preferred) or ID (fallback for backward compatibility)
  const container = containers.find((c) => {
    // Check if identifier matches the container name (without leading slash)
    if (c.names && c.names.length > 0) {
      const cleanName = c.names[0].startsWith("/")
        ? c.names[0].slice(1)
        : c.names[0];
      if (cleanName === containerIdentifier) {
        return true;
      }
    }
    // Fallback: check if it matches the ID (full or short)
    return c.id === containerIdentifier || c.id.startsWith(containerIdentifier);
  });

  // Use the actual container ID for API calls (Docker API accepts both name and ID, but we'll use ID for consistency)
  const actualContainerId = container?.id || containerIdentifier;

  const handleContainerRecreated = async (_newContainerId: string) => {
    await queryClient.invalidateQueries({ queryKey: ["containers"] });
    if (isStreaming) {
      stopStreaming();
      await new Promise((resolve) => setTimeout(resolve, 100));
      void startStreaming();
    } else {
      // If not streaming, just refetch logs
      await fetchLogs();
    }
  };

  const scrollToBottom = useCallback(() => {
    if (autoScrollRef.current && parentRef.current) {
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
  }, []);

  const fetchLogs = useCallback(async () => {
    if (!actualContainerId || !container?.host) return;

    setIsLoadingLogs(true);
    try {
      setPinnedLogIndices(new Set());
      setCurrentPinnedIndex(0);
      const logEntries = await getContainerLogsParsed(actualContainerId, container.host, {
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
  }, [actualContainerId, container?.host, logLines, scrollToBottom]);

  const startStreaming = useCallback(async () => {
    if (!actualContainerId || !container?.host) return;

    setIsStreaming(true);
    setIsLoadingLogs(true);
    setPinnedLogIndices(new Set());
    setCurrentPinnedIndex(0);
    setLogs([]);

    try {
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      const stream = streamContainerLogsParsed(
        actualContainerId,
        container.host,
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
  }, [actualContainerId, container?.host, logLines, scrollToBottom]);

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
  }, [isStreaming, fetchLogs]);

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

  const clearSelection = useCallback(() => {
    setSelectedIndices(new Set());
    setLastClickedIndex(null);
  }, []);

  const toggleJsonExpanded = useCallback((index: number) => {
    setExpandedJsonRows(prev => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }, []);

  const handleLogClick = useCallback(
    (index: number, event: React.MouseEvent) => {
      if (event.shiftKey && lastClickedIndex !== null) {
        // Shift-click: range selection
        const start = Math.min(lastClickedIndex, index);
        const end = Math.max(lastClickedIndex, index);
        const newSelected = new Set<number>();
        for (let i = start; i <= end; i++) {
          newSelected.add(i);
        }
        setSelectedIndices(newSelected);
      } else {
        // Regular click: toggle single selection and set anchor
        setSelectedIndices((prev) => {
          const newSet = new Set(prev);
          if (newSet.has(index)) {
            newSet.delete(index);
          } else {
            newSet.add(index);
          }
          return newSet;
        });
        setLastClickedIndex(index);
      }
    },
    [lastClickedIndex]
  );

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

  // Filter logs by level and optionally exclude search matches
  const filteredLogs = useMemo(() => {
    return logs.filter((entry) => {
      if (selectedLevels.size > 0 && entry.level) {
        if (!selectedLevels.has(entry.level)) {
          return false;
        }
      }
      if (excludeMatches && searchText) {
        const message = (entry.message || entry.raw || "").toLowerCase();
        if (message.includes(searchText.toLowerCase())) {
          return false;
        }
      }
      return true;
    });
  }, [logs, selectedLevels, excludeMatches, searchText]);

  const filteredToOriginalIndex = useMemo(() => {
    const indices: number[] = [];
    let searchFrom = 0;

    filteredLogs.forEach((entry) => {
      for (let i = searchFrom; i < logs.length; i++) {
        if (logs[i] === entry) {
          indices.push(i);
          searchFrom = i + 1;
          return;
        }
      }
      indices.push(-1);
    });

    return indices;
  }, [filteredLogs, logs]);

  const pinnedFilteredIndices = useMemo(() => {
    const next = new Set<number>();
    filteredToOriginalIndex.forEach((originalIndex, filteredIndex) => {
      if (pinnedLogIndices.has(originalIndex)) {
        next.add(filteredIndex);
      }
    });
    return next;
  }, [filteredToOriginalIndex, pinnedLogIndices]);

  const selectedOriginalIndices = useMemo(() => {
    return Array.from(selectedIndices)
      .map((index) => filteredToOriginalIndex[index] ?? -1)
      .filter((index) => index >= 0);
  }, [selectedIndices, filteredToOriginalIndex]);

  const allSelectedArePinned = useMemo(() => {
    return selectedOriginalIndices.length > 0 &&
      selectedOriginalIndices.every((index) => pinnedLogIndices.has(index));
  }, [selectedOriginalIndices, pinnedLogIndices]);

  const sortedPinnedIndices = useMemo(() => {
    return Array.from(pinnedLogIndices).sort((a, b) => a - b);
  }, [pinnedLogIndices]);

  const handleTogglePinSelected = useCallback(() => {
    if (selectedOriginalIndices.length === 0) return;

    setPinnedLogIndices((prev) => {
      const next = new Set(prev);
      if (allSelectedArePinned) {
        selectedOriginalIndices.forEach((index) => {
          next.delete(index);
        });
      } else {
        selectedOriginalIndices.forEach((index) => {
          next.add(index);
        });
      }
      return next;
    });

    toast.success(
      allSelectedArePinned
        ? `${selectedOriginalIndices.length} ${selectedOriginalIndices.length === 1 ? "line" : "lines"} unpinned`
        : `${selectedOriginalIndices.length} ${selectedOriginalIndices.length === 1 ? "line" : "lines"} pinned`
    );
  }, [selectedOriginalIndices, allSelectedArePinned]);

  const handleCopySelected = useCallback(() => {
    if (selectedIndices.size === 0) return;

    const sortedIndices = Array.from(selectedIndices).sort((a, b) => a - b);
    // Filter out any invalid indices as a safety measure
    const validIndices = sortedIndices.filter(
      (idx) => idx >= 0 && idx < filteredLogs.length
    );
    if (validIndices.length === 0) {
      clearSelection();
      return;
    }

    const selectedLogs = validIndices.map((idx) => filteredLogs[idx]);

    const content = selectedLogs
      .map((entry) => {
        const timestamp = entry.timestamp
          ? new Date(entry.timestamp).toISOString()
          : "";
        const level = entry.level || "UNKNOWN";
        const message = entry.message || entry.raw || "";
        return `[${timestamp}] [${level}] ${message}`;
      })
      .join("\n");

    navigator.clipboard
      .writeText(content)
      .then(() => {
        toast.success(`${validIndices.length} log entries copied to clipboard`);
      })
      .catch(() => {
        toast.error("Failed to copy to clipboard");
      });
  }, [selectedIndices, filteredLogs, clearSelection]);

  // Find all matching log indices for search navigation
  const searchMatches = useMemo(() => {
    if (!searchText) return [];
    const matches: number[] = [];
    filteredLogs.forEach((entry, index) => {
      const message = (entry.message || entry.raw || "").toLowerCase();
      if (message.includes(searchText.toLowerCase())) {
        matches.push(index);
      }
    });
    return matches;
  }, [filteredLogs, searchText]);

  // Reset current match index when search changes
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally reset when searchText changes
  useEffect(() => {
    setCurrentMatchIndex(0);
  }, [searchText]);

  // Clear selection when filters or search settings change
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally clear selection on data changes
  useEffect(() => {
    clearSelection();
    setExpandedJsonRows(new Set());
  }, [searchText, excludeMatches, selectedLevels]);

  useEffect(() => {
    if (sortedPinnedIndices.length === 0) {
      setCurrentPinnedIndex(0);
    } else if (currentPinnedIndex >= sortedPinnedIndices.length) {
      setCurrentPinnedIndex(sortedPinnedIndices.length - 1);
    }
  }, [sortedPinnedIndices, currentPinnedIndex]);

  const availableLogLevels = useMemo(() => {
    const levels = new Set<LogLevel>();
    logs.forEach((entry) => {
      if (entry.level) {
        levels.add(entry.level);
      }
    });
    return Array.from(levels).sort();
  }, [logs]);

  // Virtualization setup (must be before navigation functions)
  const rowVirtualizer = useVirtualizer({
    count: filteredLogs.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => (wrapText ? 60 : 36),
    overscan: 5,
  });

  // Navigate to previous match
  const goToPreviousMatch = useCallback(() => {
    if (searchMatches.length === 0) return;
    const newIndex = currentMatchIndex > 0 ? currentMatchIndex - 1 : searchMatches.length - 1;
    setCurrentMatchIndex(newIndex);
    rowVirtualizer.scrollToIndex(searchMatches[newIndex], { align: "center" });
  }, [searchMatches, currentMatchIndex, rowVirtualizer]);

  // Navigate to next match
  const goToNextMatch = useCallback(() => {
    if (searchMatches.length === 0) return;
    const newIndex = currentMatchIndex < searchMatches.length - 1 ? currentMatchIndex + 1 : 0;
    setCurrentMatchIndex(newIndex);
    rowVirtualizer.scrollToIndex(searchMatches[newIndex], { align: "center" });
  }, [searchMatches, currentMatchIndex, rowVirtualizer]);

  const goToPinnedByOffset = useCallback((offset: 1 | -1) => {
    if (sortedPinnedIndices.length === 0) return;

    const newPinnedIndex =
      (currentPinnedIndex + offset + sortedPinnedIndices.length) % sortedPinnedIndices.length;

    setCurrentPinnedIndex(newPinnedIndex);
    const targetOriginalIndex = sortedPinnedIndices[newPinnedIndex];
    const targetFilteredIndex = filteredToOriginalIndex.indexOf(targetOriginalIndex);

    if (targetFilteredIndex === -1) {
      toast.info("Pinned line is hidden by current filters");
      return;
    }

    setSelectedIndices(new Set([targetFilteredIndex]));
    setLastClickedIndex(targetFilteredIndex);
    rowVirtualizer.scrollToIndex(targetFilteredIndex, { align: "center" });
  }, [sortedPinnedIndices, currentPinnedIndex, filteredToOriginalIndex, rowVirtualizer]);

  const goToAdjacentLogLine = useCallback((direction: 1 | -1) => {
    if (filteredLogs.length === 0) return;
    const selected = Array.from(selectedIndices);
    const fallbackIndex =
      selected.length > 0
        ? direction > 0
          ? Math.max(...selected)
          : Math.min(...selected)
        : direction > 0
          ? -1
          : filteredLogs.length;
    const baseIndex = lastClickedIndex ?? fallbackIndex;
    const nextIndex = Math.min(
      filteredLogs.length - 1,
      Math.max(0, baseIndex + direction)
    );
    if (nextIndex === baseIndex) return;

    setSelectedIndices(new Set([nextIndex]));
    setLastClickedIndex(nextIndex);
    rowVirtualizer.scrollToIndex(nextIndex, { align: "center" });
  }, [filteredLogs.length, lastClickedIndex, rowVirtualizer, selectedIndices]);

  const extendSelectionByLine = useCallback((direction: 1 | -1) => {
    if (filteredLogs.length === 0) return;

    const selected = Array.from(selectedIndices);
    const minSelected = selected.length > 0 ? Math.min(...selected) : null;
    const maxSelected = selected.length > 0 ? Math.max(...selected) : null;

    let anchorIndex: number;
    if (lastClickedIndex !== null) {
      anchorIndex = lastClickedIndex;
    } else {
      if (selected.length > 0) {
        anchorIndex = direction > 0 ? (minSelected as number) : (maxSelected as number);
      } else {
        anchorIndex = direction > 0 ? 0 : filteredLogs.length - 1;
      }
      setLastClickedIndex(anchorIndex);
    }

    const activeIndex =
      selected.length > 0
        ? direction > 0
          ? (maxSelected as number)
          : (minSelected as number)
        : anchorIndex;
    const targetIndex = Math.min(
      filteredLogs.length - 1,
      Math.max(0, activeIndex + direction)
    );
    if (targetIndex === activeIndex) return;

    const rangeStart = Math.min(anchorIndex, targetIndex);
    const rangeEnd = Math.max(anchorIndex, targetIndex);
    const nextSelected = new Set<number>();
    for (let i = rangeStart; i <= rangeEnd; i++) {
      nextSelected.add(i);
    }
    setSelectedIndices(nextSelected);
    rowVirtualizer.scrollToIndex(targetIndex, { align: "center" });
  }, [filteredLogs.length, lastClickedIndex, rowVirtualizer, selectedIndices]);

  const focusSearchInput = useCallback(() => {
    searchInputRef.current?.focus();
    searchInputRef.current?.select();
  }, []);

  useEffect(() => {
    const handleGlobalKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
      const lowerKey = event.key.toLowerCase();

      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.isContentEditable ||
          target.closest("input, textarea, select, [contenteditable='true'], [role='textbox']"))
      ) {
        return;
      }

      if (event.key === "/") {
        event.preventDefault();
        focusSearchInput();
        return;
      }

      if (lowerKey === "j" || event.key === "ArrowDown") {
        event.preventDefault();
        if (event.shiftKey) {
          extendSelectionByLine(1);
        } else {
          goToAdjacentLogLine(1);
        }
        return;
      }

      if (lowerKey === "k" || event.key === "ArrowUp") {
        event.preventDefault();
        if (event.shiftKey) {
          extendSelectionByLine(-1);
        } else {
          goToAdjacentLogLine(-1);
        }
        return;
      }

      if (lowerKey === "n") {
        event.preventDefault();
        if (event.shiftKey) {
          goToPreviousMatch();
        } else {
          goToNextMatch();
        }
        return;
      }

      if (lowerKey === "p") {
        event.preventDefault();
        if (event.shiftKey) {
          goToPinnedByOffset(-1);
        } else {
          goToPinnedByOffset(1);
        }
      }
    };

    window.addEventListener("keydown", handleGlobalKeyDown);
    return () => {
      window.removeEventListener("keydown", handleGlobalKeyDown);
    };
  }, [
    focusSearchInput,
    goToAdjacentLogLine,
    extendSelectionByLine,
    goToNextMatch,
    goToPinnedByOffset,
    goToPreviousMatch,
  ]);

  const highlightSearchText = useCallback((text: string, isCurrentMatch: boolean): React.ReactNode => {
    if (!searchText || !text) return text;

    const escaped = escapeRegExp(searchText);
    const parts = text.split(new RegExp(`(${escaped})`, "gi"));

    return parts.map((part, i) =>
      part.toLowerCase() === searchText.toLowerCase() ? (
        <mark key={`${i}-${part}`} className={`px-0.5 rounded ${isCurrentMatch ? "bg-yellow-400 dark:bg-yellow-500" : "bg-yellow-200 dark:bg-yellow-700"}`}>
          {part}
        </mark>
      ) : (
        <span key={`${i}-${part}`}>{part}</span>
      )
    );
  }, [searchText]);

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
                  onClick={() =>
                    navigate({
                      to: "/",
                    })
                  }
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
                  {container.names?.[0]?.replace(/^\//, "") ||
                    containerIdentifier}
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

                  {/* Resource Usage Section - only for running containers */}
                  {container.state.toLowerCase() === "running" && (
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                      <div className="md:col-span-3">
                        <span className="text-muted-foreground block mb-1">
                          Resource Usage
                        </span>
                        <div className="flex gap-6 font-mono text-sm">
                          <div>
                            <span className="text-muted-foreground">CPU: </span>
                            <span className="font-medium">
                              {formatCPUPercent(statsMap[container.id]?.cpu_percent)}
                            </span>
                          </div>
                          <div>
                            <span className="text-muted-foreground">Memory: </span>
                            <span className="font-medium">
                              {formatMemoryStats(statsMap[container.id])}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

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

                  {/* Environment Variables Section */}
                  <div className="space-y-2 border-t pt-4">
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
                    {showEnvVariables && container && (
                      <div className="max-h-[300px] overflow-y-auto">
                        <EnvironmentVariables
                          containerId={actualContainerId}
                          containerHost={container.host}
                          onContainerIdChange={handleContainerRecreated}
                        />
                      </div>
                    )}
                  </div>

                  {/* Terminal Section */}
                  <div className="space-y-2 border-t pt-4">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setShowTerminal((value) => !value)}
                      className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
                    >
                      <ChevronDownIcon
                        className={`mr-2 size-4 transition-transform ${
                          showTerminal ? "rotate-180" : ""
                        }`}
                      />
                      <TerminalIcon className="mr-2 size-4" />
                      {showTerminal ? "Hide" : "Show"} terminal
                    </Button>
                    {container && (
                      <div className={`mt-2 ${showTerminal ? "" : "hidden"}`}>
                        <Terminal
                          containerId={actualContainerId}
                          host={container.host}
                        />
                      </div>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Logs Card */}
          <Card>
            <CardHeader>
              <div className="space-y-3">
                {/* Row 1: Search + Lines + Stream/Refresh */}
                <div className="flex items-center gap-2">
                  <CardTitle className="text-base shrink-0">
                    Logs
                    {filteredLogs.length !== logs.length && (
                      <span className="ml-2 text-xs text-muted-foreground font-normal">
                        ({filteredLogs.length} of {logs.length})
                      </span>
                    )}
                  </CardTitle>

                  <div className="relative flex-1 min-w-[140px]">
                    <SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
                    <Input
                      ref={searchInputRef}
                      placeholder="Search logs..."
                      value={searchText}
                      onChange={(e) => setSearchText(e.target.value)}
                      className="pl-8 h-8 text-xs"
                    />
                  </div>
                  {searchText && !excludeMatches && (
                    <div className="flex items-center gap-0.5 shrink-0">
                      <span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-1">
                        {searchMatches.length > 0
                          ? `${currentMatchIndex + 1} of ${searchMatches.length}`
                          : "No matches"}
                      </span>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={goToPreviousMatch}
                        disabled={searchMatches.length === 0}
                        className="h-8 w-8 p-0"
                      >
                        <ChevronLeftIcon className="size-3.5" />
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={goToNextMatch}
                        disabled={searchMatches.length === 0}
                        className="h-8 w-8 p-0"
                      >
                        <ChevronRightIcon className="size-3.5" />
                      </Button>
                    </div>
                  )}

                  {sortedPinnedIndices.length > 0 && (
                    <div className="flex items-center gap-0.5 shrink-0">
                      <span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-1">
                        {`${Math.min(currentPinnedIndex + 1, sortedPinnedIndices.length)} of ${sortedPinnedIndices.length} pinned`}
                      </span>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => goToPinnedByOffset(-1)}
                        className="h-8 w-8 p-0"
                      >
                        <ChevronLeftIcon className="size-3.5" />
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => goToPinnedByOffset(1)}
                        className="h-8 w-8 p-0"
                      >
                        <ChevronRightIcon className="size-3.5" />
                      </Button>
                    </div>
                  )}

                  <div className="flex items-center gap-2 shrink-0">
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
                    className="shrink-0"
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
                    className="shrink-0"
                  >
                    <RefreshCcwIcon className="size-4" />
                  </Button>
                </div>
                <p className="text-[11px] text-muted-foreground">
                  Shortcuts: <kbd className="font-mono">/</kbd> search,{" "}
                  <kbd className="font-mono">j</kbd>/<kbd className="font-mono">k</kbd>{" "}
                  lines, <kbd className="font-mono">n</kbd>/<kbd className="font-mono">N</kbd>{" "}
                  matches, <kbd className="font-mono">p</kbd>/<kbd className="font-mono">P</kbd>{" "}
                  pins
                </p>
                {/* Row 2: Options bar */}
                <div className="flex items-center gap-2">
                  {searchText && (
                    <Select
                      value={excludeMatches ? "exclude" : "highlight"}
                      onValueChange={(v) => setExcludeMatches(v === "exclude")}
                    >
                      <SelectTrigger size="sm" className="w-[160px] text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="highlight">Highlight matches</SelectItem>
                        <SelectItem value="exclude">Exclude matches</SelectItem>
                      </SelectContent>
                    </Select>
                  )}

                  <Popover open={showFilters} onOpenChange={setShowFilters}>
                    <PopoverTrigger asChild>
                      <Button variant="outline" size="sm" className="h-8 text-xs">
                        Log level
                        {selectedLevels.size > 0 && (
                          <Badge
                            variant="secondary"
                            className="ml-1.5 px-1 py-0 h-4 text-[10px] leading-none"
                          >
                            {selectedLevels.size}
                          </Badge>
                        )}
                        <ChevronDownIcon className="ml-1 size-3.5 opacity-50" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent align="start" className="w-56">
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

                  <Button
                    variant={showTimestamps ? "default" : "outline"}
                    size="sm"
                    onClick={() => setShowTimestamps(!showTimestamps)}
                    className="h-8 text-xs"
                  >
                    Timestamps
                  </Button>

                  <Button
                    variant={wrapText ? "default" : "outline"}
                    size="sm"
                    onClick={() => setWrapText(!wrapText)}
                    className="h-8 text-xs"
                  >
                    Wrap
                  </Button>

                  <Button
                    variant={autoScroll ? "default" : "outline"}
                    size="sm"
                    onClick={() => setAutoScroll(!autoScroll)}
                    className="h-8 text-xs"
                  >
                    {autoScroll ? (
                      <ArrowDownToLineIcon className="mr-1.5 size-3.5" />
                    ) : (
                      <ArrowDownIcon className="mr-1.5 size-3.5" />
                    )}
                    Auto-scroll
                  </Button>

                  <div className="flex-1" />

                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" size="sm" className="h-8 text-xs">
                        <DownloadIcon className="mr-1.5 size-3.5" />
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
              </div>
            </CardHeader>
            <CardContent className="p-0 relative">
              {/* Selection Action Bar - sticky at top of logs */}
              <SelectionActionBar
                selectedCount={selectedIndices.size}
                onCopy={handleCopySelected}
                onTogglePin={handleTogglePinSelected}
                pinActionLabel={allSelectedArePinned ? "unpin" : "pin"}
                onClear={clearSelection}
              />
              <div
                ref={parentRef}
                className="h-[calc(100vh-400px)] min-h-[400px] w-full overflow-auto"
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

                      // Check if this row is the current search match
                      const isCurrentMatch = searchMatches.length > 0 && searchMatches[currentMatchIndex] === virtualRow.index;
                      const hasMatch = searchMatches.includes(virtualRow.index);
                      const isSelected = selectedIndices.has(virtualRow.index);
                      const isPinned = pinnedFilteredIndices.has(virtualRow.index);

                      return (
                        // biome-ignore lint/a11y/useSemanticElements: div required for virtual scrolling absolute positioning
                        <div
                          key={virtualRow.key}
                          data-index={virtualRow.index}
                          ref={rowVirtualizer.measureElement}
                          role="button"
                          tabIndex={0}
                          onClick={(e) => handleLogClick(virtualRow.index, e)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              handleLogClick(virtualRow.index, e as unknown as React.MouseEvent);
                            }
                          }}
                          style={{
                            position: "absolute",
                            top: 0,
                            left: 0,
                            width: wrapText ? "100%" : "max-content",
                            minWidth: "100%",
                            transform: `translateY(${virtualRow.start}px)`,
                            cursor: "pointer",
                          }}
                          className={`group flex items-start gap-3 px-4 py-1.5 transition-all duration-150 ease-out ${
                            wrapText ? "" : "whitespace-nowrap"
                          } ${
                            isSelected && isPinned
                              ? "bg-amber-200/80 dark:bg-amber-800/45 border-l-[3px] border-amber-500 shadow-[inset_0_0_0_1px_rgba(245,158,11,0.35)]"
                              : isSelected
                              ? "bg-primary/[0.08] dark:bg-primary/[0.15] border-l-[3px] border-primary shadow-[inset_0_0_0_1px_rgba(var(--primary),0.1)]"
                              : isPinned
                                ? "bg-amber-100/80 dark:bg-amber-900/35 border-l-[3px] border-amber-500 hover:bg-amber-100 dark:hover:bg-amber-900/45"
                              : isCurrentMatch
                                ? "bg-yellow-100 dark:bg-yellow-900/30 border-y-2 border-yellow-400 dark:border-yellow-600"
                                : virtualRow.index % 2 === 0
                                  ? "bg-muted/30 border-l-[3px] border-transparent hover:bg-muted/50"
                                  : "border-l-[3px] border-transparent hover:bg-muted/50"
                          }`}
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
                            {isJsonString(entry.message ?? "") ? (
                              <CollapsibleJson
                                text={entry.message ?? ""}
                                isExpanded={expandedJsonRows.has(virtualRow.index)}
                                onToggle={() => toggleJsonExpanded(virtualRow.index)}
                                isCurrentMatch={isCurrentMatch}
                                highlightSearchText={hasMatch ? highlightSearchText : undefined}
                              />
                            ) : hasMatch ? (
                              highlightSearchText(entry.message ?? "", isCurrentMatch)
                            ) : (
                              entry.message ?? ""
                            )}
                          </span>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleCopyLog(entry);
                                }}
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
    </div>
  );
}
