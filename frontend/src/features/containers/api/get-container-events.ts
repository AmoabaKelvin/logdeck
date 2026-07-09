import { authenticatedFetch } from "@/lib/api-client";
import { iterateNDJSONStream } from "@/lib/ndjson";
import { API_BASE_URL } from "@/types/api";

const EVENTS_ENDPOINT = `${API_BASE_URL}/api/v1/events`;

export interface ContainerEvent {
  host: string;
  containerId: string;
  containerName: string;
  action: string;
  timestamp: number;
}

export async function* streamContainerEvents(
  signal?: AbortSignal,
  onOpen?: () => void
): AsyncGenerator<ContainerEvent, void, unknown> {
  const response = await authenticatedFetch(EVENTS_ENDPOINT, {
    headers: {
      Accept: "application/x-ndjson",
    },
    signal,
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "Failed to subscribe to container events");
  }

  if (!response.body) {
    throw new Error("Streaming is not supported in this environment.");
  }

  onOpen?.();

  yield* iterateNDJSONStream<ContainerEvent>(response.body, signal);
}
