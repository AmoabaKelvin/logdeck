import { useEffect, useRef, useState } from "react";

import {
	type ContainerEvent,
	streamContainerEvents,
} from "../api/get-container-events";

import { useDocumentVisible } from "./use-document-visible";

const INITIAL_RETRY_DELAY_MS = 1_000;
const MAX_RETRY_DELAY_MS = 30_000;

/**
 * Subscribes to the container lifecycle event stream while the page is
 * visible, reconnecting with capped exponential backoff. The subscription is
 * paused entirely while the document is hidden and resumed on visibility.
 */
export function useContainerEvents(onEvent: (event: ContainerEvent) => void) {
	const [isConnected, setIsConnected] = useState(false);
	const isVisible = useDocumentVisible();
	const onEventRef = useRef(onEvent);

	useEffect(() => {
		onEventRef.current = onEvent;
	}, [onEvent]);

	useEffect(() => {
		if (!isVisible) return;

		const abortController = new AbortController();
		let retryDelay = INITIAL_RETRY_DELAY_MS;
		let retryTimeout: ReturnType<typeof setTimeout> | null = null;

		const subscribe = async () => {
			try {
				const stream = streamContainerEvents(abortController.signal, () => {
					setIsConnected(true);
					retryDelay = INITIAL_RETRY_DELAY_MS;
				});
				for await (const event of stream) {
					onEventRef.current(event);
				}
			} catch {}

			setIsConnected(false);
			if (abortController.signal.aborted) return;

			retryTimeout = setTimeout(() => {
				void subscribe();
			}, retryDelay);
			retryDelay = Math.min(retryDelay * 2, MAX_RETRY_DELAY_MS);
		};

		void subscribe();

		return () => {
			abortController.abort();
			if (retryTimeout) {
				clearTimeout(retryTimeout);
			}
			setIsConnected(false);
		};
	}, [isVisible]);

	return { isConnected };
}
