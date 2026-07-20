import { subscribeContainerEvents } from "@/lib/logdeck-demo/store";

export interface ContainerEvent {
	host: string;
	containerId: string;
	containerName: string;
	action: string;
	timestamp: number;
}

// Yields events pushed by demo actions (start/stop/recreate/...) until the
// consumer aborts. Backed by the store's pub/sub instead of a network stream.
export async function* streamContainerEvents(
	signal?: AbortSignal,
	onOpen?: () => void,
): AsyncGenerator<ContainerEvent, void, unknown> {
	onOpen?.();

	const queue: ContainerEvent[] = [];
	let notify: (() => void) | null = null;

	const unsubscribe = subscribeContainerEvents((event) => {
		queue.push(event);
		notify?.();
	});

	const onAbort = () => notify?.();
	signal?.addEventListener("abort", onAbort);

	try {
		while (!signal?.aborted) {
			if (queue.length === 0) {
				await new Promise<void>((resolve) => {
					notify = resolve;
				});
				notify = null;
			}
			while (queue.length > 0 && !signal?.aborted) {
				const event = queue.shift();
				if (event) yield event;
			}
		}
	} finally {
		unsubscribe();
		signal?.removeEventListener("abort", onAbort);
	}
}
