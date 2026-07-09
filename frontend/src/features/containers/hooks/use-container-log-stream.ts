import { useCallback, useEffect, useRef, useState } from "react";

import type { ContainerLogsOptions } from "@/features/containers/api/get-container-logs-parsed";

export const DEFAULT_MAX_LOG_LINES = 10000;
const STREAM_FLUSH_INTERVAL_MS = 100;

interface UseContainerLogStreamOptions<TLogEntry> {
  containerId?: string;
  host?: string;
  tail: number;
  search?: string;
  maxLogLines?: number;
  getLogs: (
    containerId: string,
    host: string,
    options: ContainerLogsOptions
  ) => Promise<TLogEntry[]>;
  streamLogs: (
    containerId: string,
    host: string,
    options: ContainerLogsOptions,
    signal: AbortSignal
  ) => AsyncGenerator<TLogEntry, void, unknown>;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  onResetState?: () => void;
  onFetchError?: (error: Error) => void;
  onStreamError?: (error: Error) => void;
}

export function useContainerLogStream<TLogEntry>({
  containerId,
  host,
  tail,
  search,
  maxLogLines = DEFAULT_MAX_LOG_LINES,
  getLogs,
  streamLogs,
  scrollToBottom,
  onResetState,
  onFetchError,
  onStreamError,
}: UseContainerLogStreamOptions<TLogEntry>) {
  const [logs, setLogs] = useState<TLogEntry[]>([]);
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);
  const [isStreaming, setIsStreaming] = useState(false);
  const [isStreamPaused, setIsStreamPaused] = useState(false);
  const [bufferedCount, setBufferedCount] = useState(0);
  const [droppedCount, setDroppedCount] = useState(0);
  const [animatedRange, setAnimatedRange] = useState<{
    start: number;
    end: number;
  } | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const isStreamPausedRef = useRef(false);
  const bufferedLogsRef = useRef<TLogEntry[]>([]);
  const pendingLogsRef = useRef<TLogEntry[]>([]);
  const logsLengthRef = useRef(0);
  const droppedCountRef = useRef(0);
  const maxLogLinesRef = useRef(maxLogLines);
  const flushIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const scrollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const animationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const getLogsRef = useRef(getLogs);
  const streamLogsRef = useRef(streamLogs);
  const scrollToBottomRef = useRef(scrollToBottom);
  const onResetStateRef = useRef(onResetState);
  const onFetchErrorRef = useRef(onFetchError);
  const onStreamErrorRef = useRef(onStreamError);

  useEffect(() => {
    isStreamPausedRef.current = isStreamPaused;
  }, [isStreamPaused]);

  useEffect(() => {
    maxLogLinesRef.current = maxLogLines;
  }, [maxLogLines]);

  useEffect(() => {
    getLogsRef.current = getLogs;
  }, [getLogs]);

  useEffect(() => {
    streamLogsRef.current = streamLogs;
  }, [streamLogs]);

  useEffect(() => {
    scrollToBottomRef.current = scrollToBottom;
  }, [scrollToBottom]);

  useEffect(() => {
    onResetStateRef.current = onResetState;
  }, [onResetState]);

  useEffect(() => {
    onFetchErrorRef.current = onFetchError;
  }, [onFetchError]);

  useEffect(() => {
    onStreamErrorRef.current = onStreamError;
  }, [onStreamError]);

  const resetPauseAndBuffer = useCallback(() => {
    setIsStreamPaused(false);
    setBufferedCount(0);
    isStreamPausedRef.current = false;
    bufferedLogsRef.current = [];
  }, []);

  const resetDroppedCount = useCallback(() => {
    droppedCountRef.current = 0;
    setDroppedCount(0);
  }, []);

  const recordDroppedLines = useCallback((count: number) => {
    if (count <= 0) return;
    droppedCountRef.current += count;
    setDroppedCount(droppedCountRef.current);
  }, []);

  const scheduleScrollToBottom = useCallback(
    (delay: number, behavior?: ScrollBehavior) => {
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }
      scrollTimeoutRef.current = setTimeout(() => {
        scrollTimeoutRef.current = null;
        scrollToBottomRef.current(behavior);
      }, delay);
    },
    []
  );

  const triggerRowAnimation = useCallback((start: number, end: number) => {
    if (animationTimeoutRef.current) {
      clearTimeout(animationTimeoutRef.current);
    }
    setAnimatedRange({ start, end });
    animationTimeoutRef.current = setTimeout(() => {
      setAnimatedRange(null);
    }, 260);
  }, []);

  // Flush accumulated entries into state: one setLogs (with the ring-buffer
  // cap applied) and one scheduled scroll per flush.
  const flushPendingLogs = useCallback(
    (scrollBehavior?: ScrollBehavior) => {
      const pending = pendingLogsRef.current;
      if (pending.length === 0) return;
      pendingLogsRef.current = [];

      const max = maxLogLinesRef.current;
      const total = logsLengthRef.current + pending.length;
      const dropped = Math.max(0, total - max);
      const nextLength = total - dropped;

      setLogs((prev) => {
        const next = prev.concat(pending);
        return next.length > max ? next.slice(next.length - max) : next;
      });
      logsLengthRef.current = nextLength;
      recordDroppedLines(dropped);
      triggerRowAnimation(Math.max(0, nextLength - pending.length), nextLength - 1);
      scheduleScrollToBottom(scrollBehavior === "smooth" ? 40 : 100, scrollBehavior);
    },
    [recordDroppedLines, scheduleScrollToBottom, triggerRowAnimation]
  );

  // While paused, keep the buffered backlog capped and sync its count on the
  // flush cadence instead of per line.
  const syncBufferedLogs = useCallback(() => {
    const buffered = bufferedLogsRef.current;
    const max = maxLogLinesRef.current;
    if (buffered.length > max) {
      recordDroppedLines(buffered.length - max);
      buffered.splice(0, buffered.length - max);
    }
    setBufferedCount(buffered.length);
  }, [recordDroppedLines]);

  const handleFlushTick = useCallback(() => {
    if (isStreamPausedRef.current) {
      syncBufferedLogs();
      return;
    }
    flushPendingLogs();
  }, [flushPendingLogs, syncBufferedLogs]);

  const stopFlushInterval = useCallback(() => {
    if (flushIntervalRef.current) {
      clearInterval(flushIntervalRef.current);
      flushIntervalRef.current = null;
    }
  }, []);

  useEffect(() => {
    return () => {
      if (animationTimeoutRef.current) {
        clearTimeout(animationTimeoutRef.current);
      }
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }
      if (flushIntervalRef.current) {
        clearInterval(flushIntervalRef.current);
      }
    };
  }, []);

  const fetchLogs = useCallback(async () => {
    if (!containerId || !host) return;

    setIsLoadingLogs(true);
    resetPauseAndBuffer();
    resetDroppedCount();
    onResetStateRef.current?.();
    try {
      const logEntries = await getLogsRef.current(containerId, host, { tail, search });
      const max = maxLogLinesRef.current;
      const capped =
        logEntries.length > max ? logEntries.slice(logEntries.length - max) : logEntries;
      setLogs(capped);
      logsLengthRef.current = capped.length;
      recordDroppedLines(logEntries.length - capped.length);
      scheduleScrollToBottom(100);
    } catch (error) {
      if (error instanceof Error) {
        onFetchErrorRef.current?.(error);
      }
      setLogs([]);
      logsLengthRef.current = 0;
    } finally {
      setIsLoadingLogs(false);
    }
  }, [
    containerId,
    host,
    recordDroppedLines,
    resetDroppedCount,
    resetPauseAndBuffer,
    scheduleScrollToBottom,
    tail,
    search,
  ]);

  const stopStreaming = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    resetPauseAndBuffer();
    setIsStreaming(false);
  }, [resetPauseAndBuffer]);

  const startStreaming = useCallback(async () => {
    if (!containerId || !host) return;

    setIsStreaming(true);
    setIsLoadingLogs(true);
    resetPauseAndBuffer();
    resetDroppedCount();
    onResetStateRef.current?.();
    setLogs([]);
    logsLengthRef.current = 0;
    pendingLogsRef.current = [];

    try {
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      const stream = streamLogsRef.current(
        containerId,
        host,
        { tail, search },
        abortController.signal
      );
      let hasReceivedFirstEntry = false;

      stopFlushInterval();
      flushIntervalRef.current = setInterval(handleFlushTick, STREAM_FLUSH_INTERVAL_MS);

      for await (const entry of stream) {
        if (abortController.signal.aborted) {
          break;
        }

        if (!hasReceivedFirstEntry) {
          hasReceivedFirstEntry = true;
          setIsLoadingLogs(false);
        }

        if (isStreamPausedRef.current) {
          bufferedLogsRef.current.push(entry);
          continue;
        }

        pendingLogsRef.current.push(entry);
      }
    } catch (error) {
      if (error instanceof Error) {
        const message = error.message.toLowerCase();
        const isAbort =
          error.name === "AbortError" || message.includes("aborted");
        if (!isAbort) {
          onStreamErrorRef.current?.(error);
        }
      }
    } finally {
      stopFlushInterval();
      flushPendingLogs();
      setIsLoadingLogs(false);
      setIsStreaming(false);
      abortControllerRef.current = null;
    }
  }, [
    containerId,
    host,
    flushPendingLogs,
    handleFlushTick,
    resetDroppedCount,
    resetPauseAndBuffer,
    stopFlushInterval,
    tail,
    search,
  ]);

  const toggleStreaming = useCallback(() => {
    if (isStreaming) {
      stopStreaming();
      return;
    }
    void startStreaming();
  }, [isStreaming, startStreaming, stopStreaming]);

  const togglePauseStreaming = useCallback(() => {
    if (!isStreaming) return;

    if (isStreamPaused) {
      isStreamPausedRef.current = false;
      setIsStreamPaused(false);
      setBufferedCount(0);
      if (bufferedLogsRef.current.length > 0) {
        pendingLogsRef.current = pendingLogsRef.current.concat(
          bufferedLogsRef.current
        );
        bufferedLogsRef.current = [];
        flushPendingLogs("smooth");
      }
      return;
    }

    // Land any entries that arrived before pausing so the view is current.
    flushPendingLogs();
    isStreamPausedRef.current = true;
    setIsStreamPaused(true);
  }, [flushPendingLogs, isStreamPaused, isStreaming]);

  const clearLogs = useCallback(() => {
    pendingLogsRef.current = [];
    setLogs([]);
    logsLengthRef.current = 0;
    resetDroppedCount();
  }, [resetDroppedCount]);

  return {
    bufferedCount,
    animatedRange,
    clearLogs,
    droppedCount,
    fetchLogs,
    isLoadingLogs,
    isStreamPaused,
    isStreaming,
    logs,
    maxLogLines,
    setLogs,
    startStreaming,
    stopStreaming,
    togglePauseStreaming,
    toggleStreaming,
  };
}
