import { useCallback, useEffect, useRef, useState } from "react";

import type { ContainerLogsOptions } from "@/lib/logdeck-demo/demo-api";

interface UseContainerLogStreamOptions<TLogEntry> {
  containerId?: string;
  host?: string;
  tail: number;
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
  const [animatedRange, setAnimatedRange] = useState<{
    start: number;
    end: number;
  } | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const isStreamPausedRef = useRef(false);
  const bufferedLogsRef = useRef<TLogEntry[]>([]);
  const logsLengthRef = useRef(0);
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

  const triggerRowAnimation = useCallback((start: number, end: number) => {
    if (animationTimeoutRef.current) {
      clearTimeout(animationTimeoutRef.current);
    }
    setAnimatedRange({ start, end });
    animationTimeoutRef.current = setTimeout(() => {
      setAnimatedRange(null);
    }, 260);
  }, []);

  useEffect(() => {
    return () => {
      if (animationTimeoutRef.current) {
        clearTimeout(animationTimeoutRef.current);
      }
    };
  }, []);

  const fetchLogs = useCallback(async () => {
    if (!containerId || !host) return;

    setIsLoadingLogs(true);
    resetPauseAndBuffer();
    onResetStateRef.current?.();
    try {
      const logEntries = await getLogsRef.current(containerId, host, { tail });
      setLogs(logEntries);
      logsLengthRef.current = logEntries.length;
      setTimeout(() => scrollToBottomRef.current(), 100);
    } catch (error) {
      if (error instanceof Error) {
        onFetchErrorRef.current?.(error);
      }
      setLogs([]);
      logsLengthRef.current = 0;
    } finally {
      setIsLoadingLogs(false);
    }
  }, [containerId, host, resetPauseAndBuffer, tail]);

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
    onResetStateRef.current?.();
    setLogs([]);
    logsLengthRef.current = 0;

    try {
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      const stream = streamLogsRef.current(
        containerId,
        host,
        { tail },
        abortController.signal
      );
      let hasReceivedFirstEntry = false;

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
          setBufferedCount((prev) => prev + 1);
          continue;
        }

        const nextIndex = logsLengthRef.current;
        setLogs((prev) => [...prev, entry]);
        logsLengthRef.current += 1;
        triggerRowAnimation(nextIndex, nextIndex);
        setTimeout(() => scrollToBottomRef.current(), 100);
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
      setIsLoadingLogs(false);
      setIsStreaming(false);
      abortControllerRef.current = null;
    }
  }, [containerId, host, resetPauseAndBuffer, tail, triggerRowAnimation]);

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
