import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { ContainerInfo } from "../types";

const CONTAINERS_ENDPOINT = `${API_BASE_URL}/api/v1/containers`;

export interface GetContainersResponse {
  containers: ContainerInfo[];
  readOnly: boolean;
}

export async function getContainers(): Promise<GetContainersResponse> {
  const response = await authenticatedFetch(CONTAINERS_ENDPOINT);

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  const data = (await response.json()) as unknown;

  if (!data || typeof data !== "object" || data === null) {
    throw new Error("Unexpected response format");
  }

  const containers = (data as { containers?: unknown }).containers;
  const readOnly = (data as { readOnly?: boolean }).readOnly ?? false;

  if (!Array.isArray(containers)) {
    throw new Error("Unexpected response format");
  }

  return {
    containers: containers as ContainerInfo[],
    readOnly,
  };
}
