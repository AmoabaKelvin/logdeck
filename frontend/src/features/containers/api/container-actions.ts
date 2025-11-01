import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const BASE_URL = `${API_BASE_URL}/api/v1/containers`;

type ContainerAction = "start" | "stop" | "restart" | "remove";

interface ActionResponse {
  message?: string;
}

async function performContainerAction(
  id: string,
  action: ContainerAction
): Promise<string> {
  const endpoint = `${BASE_URL}/${encodeURIComponent(id)}/${action}`;
  const response = await authenticatedFetch(endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Failed to ${action} container`);
  }

  const data = (await response.json()) as ActionResponse | undefined;

  if (data && typeof data.message === "string") {
    return data.message;
  }

  return "Action completed successfully";
}

export function startContainer(id: string) {
  return performContainerAction(id, "start");
}

export function stopContainer(id: string) {
  return performContainerAction(id, "stop");
}

export function restartContainer(id: string) {
  return performContainerAction(id, "restart");
}

export function removeContainer(id: string) {
  return performContainerAction(id, "remove");
}
