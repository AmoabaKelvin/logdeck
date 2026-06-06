import { useCallback, useEffect, useRef, useState } from "react";

import type { ContainerLogsOptions } from "@/features/containers/api/get-container-logs-parsed";

interface UseContainerLogStreamOptions<TLogEntry> {
  containerId?: string;
  host?: string;
  tail: number;
  search?: string;
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
  streamEvents?: (containerId: string, host: string, signal: AbortSignal) => AsyncGenerator<{ action: string }, void, unknown>;
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
  getLogs,
  streamLogs,
  streamEvents,
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
  const [animatedRange, setAnimatedRange] = useState<{
    start: number;
    end: number;
  } | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const reconnectAbortRef = useRef<AbortController | null>(null);
  const isStreamPausedRef = useRef(false);
  const bufferedLogsRef = useRef<TLogEntry[]>([]);
  const logsLengthRef = useRef(0);
  const animationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const getLogsRef = useRef(getLogs);
  const streamLogsRef = useRef(streamLogs);
  const streamEventsRef = useRef(streamEvents);
  const scrollToBottomRef = useRef(scrollToBottom);
  const onResetStateRef = useRef(onResetState);
  const onFetchErrorRef = useRef(onFetchError);
  const onStreamErrorRef = useRef(onStreamError);

  useEffect(() => { isStreamPausedRef.current = isStreamPaused; }, [isStreamPaused]);
  useEffect(() => { getLogsRef.current = getLogs; }, [getLogs]);
  useEffect(() => { streamLogsRef.current = streamLogs; }, [streamLogs]);
  useEffect(() => { streamEventsRef.current = streamEvents; }, [streamEvents]);
  useEffect(() => { scrollToBottomRef.current = scrollToBottom; }, [scrollToBottom]);
  useEffect(() => { onResetStateRef.current = onResetState; }, [onResetState]);
  useEffect(() => { onFetchErrorRef.current = onFetchError; }, [onFetchError]);
  useEffect(() => { onStreamErrorRef.current = onStreamError; }, [onStreamError]);

  useEffect(() => {
    return () => {
      if (animationTimeoutRef.current) clearTimeout(animationTimeoutRef.current);
    };
  }, []);

  const resetPauseAndBuffer = useCallback(() => {
    setIsStreamPaused(false);
    setBufferedCount(0);
    isStreamPausedRef.current = false;
    bufferedLogsRef.current = [];
  }, []);

  const triggerRowAnimation = useCallback((start: number, end: number) => {
    if (animationTimeoutRef.current) clearTimeout(animationTimeoutRef.current);
    setAnimatedRange({ start, end });
    animationTimeoutRef.current = setTimeout(() => setAnimatedRange(null), 260);
  }, []);

  const fetchLogs = useCallback(async () => {
    if (!containerId || !host) return;

    setIsLoadingLogs(true);
    resetPauseAndBuffer();
    onResetStateRef.current?.();
    try {
      const logEntries = await getLogsRef.current(containerId, host, { tail, search });
      setLogs(logEntries);
      logsLengthRef.current = logEntries.length;
      setTimeout(() => scrollToBottomRef.current(), 100);
    } catch (error) {
      if (error instanceof Error) onFetchErrorRef.current?.(error);
      setLogs([]);
      logsLengthRef.current = 0;
    } finally {
      setIsLoadingLogs(false);
    }
  }, [containerId, host, resetPauseAndBuffer, tail, search]);

  const stopStreaming = useCallback(() => {
    if (reconnectAbortRef.current) {
      reconnectAbortRef.current.abort();
      reconnectAbortRef.current = null;
    }
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
    onResetStateRef.current?.();
    setLogs([]);
    logsLengthRef.current = 0;

    const reconnectAbort = new AbortController();
    reconnectAbortRef.current = reconnectAbort;

    while (!reconnectAbort.signal.aborted) {
      try {
        const abortController = new AbortController();
        abortControllerRef.current = abortController;

        const onReconnectAbort = () => abortController.abort();
        reconnectAbort.signal.addEventListener("abort", onReconnectAbort, { once: true });

        const stream = streamLogsRef.current(
          containerId,
          host,
          { tail, search },
          abortController.signal
        );
        let hasReceivedFirstEntry = false;

        for await (const entry of stream) {
          if (abortController.signal.aborted) break;

          if (!hasReceivedFirstEntry) {
            hasReceivedFirstEntry = true;
            setIsLoadingLogs(false);
          }

          if (isStreamPausedRef.current) {
            bufferedLogsRef.current.push(entry);
            setBufferedCount((prev) => prev + 1);
            continue;
          }

          const nextIndex = logsLengthRef.current;
          setLogs((prev) => [...prev, entry]);
          logsLengthRef.current += 1;
          triggerRowAnimation(nextIndex, nextIndex);
          setTimeout(() => scrollToBottomRef.current(), 100);
        }

        reconnectAbort.signal.removeEventListener("abort", onReconnectAbort);
      } catch (error) {
        if (error instanceof Error) {
          const isAbort = error.name === "AbortError" || error.message.toLowerCase().includes("aborted");
          if (!isAbort) onStreamErrorRef.current?.(error);
        }
      }

      if (reconnectAbort.signal.aborted) break;

      const streamEventsFn = streamEventsRef.current;
      if (!streamEventsFn) break;

      let reconnected = false;
      const eventsAbort = new AbortController();
      const onStop = () => eventsAbort.abort();
      reconnectAbort.signal.addEventListener("abort", onStop, { once: true });

      while (!reconnectAbort.signal.aborted && !reconnected) {
        try {
          for await (const event of streamEventsFn(containerId, host, eventsAbort.signal)) {
            if (event.action === "start") {
              reconnected = true;
              break;
            }
          }
        } catch {
          // ignore — engine still coming up
        }
        if (!reconnected && !reconnectAbort.signal.aborted) {
          await new Promise((resolve) => setTimeout(resolve, 2000));
        }
      }

      reconnectAbort.signal.removeEventListener("abort", onStop);

      if (!reconnected || reconnectAbort.signal.aborted) break;

      setIsLoadingLogs(true);
      setLogs([]);
      logsLengthRef.current = 0;
      resetPauseAndBuffer();
      onResetStateRef.current?.();
    }

    setIsLoadingLogs(false);
    setIsStreaming(false);
    abortControllerRef.current = null;
    reconnectAbortRef.current = null;
  }, [containerId, host, resetPauseAndBuffer, tail, search, triggerRowAnimation]);

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
      const buffered = bufferedLogsRef.current;
      if (buffered.length > 0) {
        const startIndex = logsLengthRef.current;
        const endIndex = startIndex + buffered.length - 1;
        setLogs((prev) => [...prev, ...buffered]);
        logsLengthRef.current += buffered.length;
        triggerRowAnimation(startIndex, endIndex);
        setTimeout(() => scrollToBottomRef.current("smooth"), 40);
      }
      bufferedLogsRef.current = [];
      setBufferedCount(0);
      isStreamPausedRef.current = false;
      setIsStreamPaused(false);
      return;
    }

    isStreamPausedRef.current = true;
    setIsStreamPaused(true);
  }, [isStreamPaused, isStreaming, triggerRowAnimation]);

  const clearLogs = useCallback(() => {
    setLogs([]);
    logsLengthRef.current = 0;
  }, []);

  return {
    bufferedCount,
    animatedRange,
    clearLogs,
    fetchLogs,
    isLoadingLogs,
    isStreamPaused,
    isStreaming,
    logs,
    setLogs,
    startStreaming,
    stopStreaming,
    togglePauseStreaming,
    toggleStreaming,
  };
}
