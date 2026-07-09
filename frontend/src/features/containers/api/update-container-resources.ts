import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { RestartPolicy } from "./get-container-resources";

export interface UpdateResourcesRequest {
  memoryBytes?: number;
  nanoCPUs?: number;
  restartPolicy?: RestartPolicy;
}

export async function updateContainerResources(
  id: string,
  host: string,
  request: UpdateResourcesRequest
): Promise<void> {
  const response = await authenticatedFetch(
    `${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/resources?host=${encodeURIComponent(host)}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(request),
    }
  );

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message.trim() || "Failed to update container resources");
  }
}
