import { useVirtualizer } from "@tanstack/react-virtual";
import {
  ArrowDownIcon,
  ArrowDownToLineIcon,
  CheckIcon,
  PauseIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CopyIcon,
  DownloadIcon,
  ExternalLinkIcon,
  PlayIcon,
  RefreshCcwIcon,
  SearchIcon,
  SquareIcon,
} from "lucide-react";
import type React from "react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState
} from "react";
import { toast } from "sonner";

import { Badge } from "@/components/logdeck-demo/ui/badge";
import { Button } from "@/components/logdeck-demo/ui/button";
import { Card, CardContent } from "@/components/logdeck-demo/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "@/components/logdeck-demo/ui/dropdown-menu";
import { Input } from "@/components/logdeck-demo/ui/input";
import { Label } from "@/components/logdeck-demo/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from "@/components/logdeck-demo/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/logdeck-demo/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from "@/components/logdeck-demo/ui/sheet";
import { Spinner } from "@/components/logdeck-demo/ui/spinner";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger
} from "@/components/logdeck-demo/ui/tooltip";

import {
  getContainerLogsParsedDemo,
  getLogLevelBadgeColor,
  streamContainerLogsParsedDemo
} from "@/lib/logdeck-demo/demo-api";
import { useContainerLogStream } from "./use-container-log-stream";

import { isJsonString } from "@/lib/json-format";
import {
  formatContainerName,
  formatCreatedDate,
  formatUptime,
  getContainerUrlIdentifier,
  getStateBadgeClass,
  toTitleCase
} from "./container-utils";
import { CollapsibleJson } from "./collapsible-json";
import { EnvironmentVariables } from "./environment-variables";
import { SelectionActionBar } from "./selection-action-bar";

import type { LogEntry, LogLevel } from "@/types/logs";
import type { ContainerInfo } from "./types";
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
  const [searchText, setSearchText] = useState("");
  const [excludeMatches, setExcludeMatches] = useState(false);
  const [selectedLevels, setSelectedLevels] = useState<Set<LogLevel>>(
    new Set()
  );
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [wrapText, setWrapText] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [currentMatchIndex, setCurrentMatchIndex] = useState(0);
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(new Set());
  const [pinnedLogIndices, setPinnedLogIndices] = useState<Set<number>>(new Set());
  const [currentPinnedIndex, setCurrentPinnedIndex] = useState(0);
  const [expandedJsonRows, setExpandedJsonRows] = useState<Set<number>>(new Set());
  const [lastClickedIndex, setLastClickedIndex] = useState<number | null>(null);
  const parentRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const autoScrollRef = useRef(autoScroll);
  const logLinesInputId = useId();

  // Keep ref in sync with state
  useEffect(() => {
    autoScrollRef.current = autoScroll;
  }, [autoScroll]);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = "auto") => {
    if (autoScrollRef.current && parentRef.current) {
      // For virtualized list, scroll the parent container to bottom
      if (behavior === "smooth") {
        parentRef.current.scrollTo({
          top: parentRef.current.scrollHeight,
          behavior: "smooth",
        });
        return;
      }
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
  }, []);
  const {
    animatedRange,
    bufferedCount,
    clearLogs,
    fetchLogs,
    isLoadingLogs,
    isStreamPaused,
    isStreaming,
    logs,
    stopStreaming,
    togglePauseStreaming,
    toggleStreaming,
  } = useContainerLogStream<LogEntry>({
    containerId: container?.id,
    host: container?.host,
    tail: logLines,
    getLogs: async (containerId, host, options) =>
      (await getContainerLogsParsedDemo(containerId, host, options)) as LogEntry[],
    streamLogs: async function* (containerId, host, options, signal) {
      for await (const entry of streamContainerLogsParsedDemo(
        containerId,
        host,
        options,
        signal
      )) {
        yield entry as LogEntry;
      }
    },
    scrollToBottom,
    onResetState: () => {
      setPinnedLogIndices(new Set());
      setCurrentPinnedIndex(0);
    },
    onFetchError: (error) => {
      toast.error(`Failed to fetch logs: ${error.message}`);
    },
    onStreamError: (error) => {
      toast.error(`Failed to start streaming: ${error.message}`);
    },
  });

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
      clearLogs();
    }
  }, [clearLogs, isOpen, stopStreaming]);

  useEffect(() => {
    setShowLabels(false);
    setShowEnvVariables(false);
    stopStreaming();
    clearLogs();

    if (container && isOpen) {
      fetchLogs();
    }
  }, [clearLogs, container, isOpen, fetchLogs, stopStreaming]);

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

  const clearSelection = useCallback(() => {
    setSelectedIndices(new Set());
    setLastClickedIndex(null);
  }, []);
  const activeToggleButtonClass =
    "h-8 text-xs data-[active=true]:bg-muted data-[active=true]:text-foreground data-[active=true]:border-border data-[active=true]:ring-1 data-[active=true]:ring-primary/30";

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

  // Filter logs by level and optionally exclude search matches.
  const filteredLogItems = useMemo(() => {
    const searchLower = searchText.toLowerCase();

    const items = logs
      .map((entry, originalIndex) => ({ entry, originalIndex }))
      .filter(({ entry }) => {
      if (selectedLevels.size > 0 && entry.level) {
        if (!selectedLevels.has(entry.level)) {
          return false;
        }
      }
        if (excludeMatches && searchText) {
          const message = (entry.message || entry.raw || "").toLowerCase();
          if (message.includes(searchLower)) {
            return false;
          }
        }
        return true;
      });

    return items;
  }, [
    logs,
    selectedLevels,
    excludeMatches,
    searchText,
  ]);

  const filteredLogs = useMemo(
    () => filteredLogItems.map((item) => item.entry),
    [filteredLogItems]
  );

  const filteredToOriginalIndex = useMemo(
    () => filteredLogItems.map((item) => item.originalIndex),
    [filteredLogItems]
  );

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
    if (!isOpen) return;

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
    isOpen,
    focusSearchInput,
    goToAdjacentLogLine,
    extendSelectionByLine,
    goToNextMatch,
    goToPinnedByOffset,
    goToPreviousMatch,
  ]);

  // Helper to highlight search text in message
  const highlightSearchText = useCallback((text: string, isCurrentMatch: boolean): React.ReactNode => {
    if (!searchText || !text) return text;

    const lowerText = text.toLowerCase();
    const lowerSearch = searchText.toLowerCase();
    const index = lowerText.indexOf(lowerSearch);

    if (index === -1) return text;

    const before = text.slice(0, index);
    const match = text.slice(index, index + searchText.length);
    const after = text.slice(index + searchText.length);

    return (
      <>
        {before}
        <mark className={`px-0.5 rounded ${isCurrentMatch ? "bg-yellow-400 dark:bg-yellow-500" : "bg-yellow-200 dark:bg-yellow-700"}`}>
          {match}
        </mark>
        {highlightSearchText(after, isCurrentMatch)}
      </>
    );
  }, [searchText]);

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
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className="col-span-2 font-medium truncate cursor-help">
                            {container.image}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-md break-all">
                          {container.image}
                        </TooltipContent>
                      </Tooltip>
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
                            containerHost={container.host}
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
              <div className="space-y-3">
                {/* Row 1: Search + Lines + Stream/Refresh */}
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-medium shrink-0">
                    Logs
                    {filteredLogs.length !== logs.length && (
                      <span className="ml-2 text-xs text-muted-foreground font-normal">
                        ({filteredLogs.length} of {logs.length})
                      </span>
                    )}
                  </h3>

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
                    variant="outline"
                    size="sm"
                    data-active={isStreaming}
                    onClick={toggleStreaming}
                    disabled={isLoadingLogs && !isStreaming}
                    className={`shrink-0 ${activeToggleButtonClass}`}
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
                    data-active={isStreamPaused}
                    onClick={togglePauseStreaming}
                    disabled={!isStreaming}
                    className={`shrink-0 ${activeToggleButtonClass}`}
                  >
                    {isStreamPaused ? (
                      <>
                        <PlayIcon className="mr-2 size-4" />
                        Resume
                        {bufferedCount > 0 && (
                          <span className="ml-1 text-[10px] tabular-nums">
                            ({bufferedCount})
                          </span>
                        )}
                      </>
                    ) : (
                      <>
                        <PauseIcon className="mr-2 size-4" />
                        Pause
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
                <div className="flex flex-wrap items-center gap-2">
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
                    variant="outline"
                    size="sm"
                    data-active={showTimestamps}
                    onClick={() => setShowTimestamps(!showTimestamps)}
                    className={activeToggleButtonClass}
                  >
                    Timestamps
                  </Button>

                  <Button
                    variant="outline"
                    size="sm"
                    data-active={wrapText}
                    onClick={() => setWrapText(!wrapText)}
                    className={activeToggleButtonClass}
                  >
                    Wrap
                  </Button>

                  <Button
                    variant="outline"
                    size="sm"
                    data-active={autoScroll}
                    onClick={() => setAutoScroll(!autoScroll)}
                    className={activeToggleButtonClass}
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
              <Card>
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
                              } ${
                                animatedRange &&
                                virtualRow.index >= animatedRange.start &&
                                virtualRow.index <= animatedRange.end
                                  ? "log-stream-row-enter"
                                  : ""
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
        )}
      </SheetContent>
    </Sheet>
  );
}
