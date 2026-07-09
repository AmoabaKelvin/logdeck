import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface RestartPolicy {
  name: string;
  maximumRetryCount: number;
}

export interface ContainerResources {
  memoryBytes: number;
  nanoCPUs: number;
  restartPolicy: RestartPolicy;
}

export async function getContainerResources(
  id: string,
  host: string
): Promise<ContainerResources> {
  const response = await authenticatedFetch(
    `${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/resources?host=${encodeURIComponent(host)}`
  );

  if (!response.ok) {
    throw new Error("Failed to fetch container resources");
  }

  return response.json();
}
