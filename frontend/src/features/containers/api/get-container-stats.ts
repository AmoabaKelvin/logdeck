import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { ContainerStats } from "../types";

const CONTAINER_STATS_ENDPOINT = `${API_BASE_URL}/api/v1/containers/stats`;

export interface GetContainerStatsResponse {
  stats: ContainerStats[];
}

export async function getContainerStats(): Promise<GetContainerStatsResponse> {
  const response = await authenticatedFetch(CONTAINER_STATS_ENDPOINT);

  if (!response.ok) {
    throw new Error((await response.text()) || `Request failed with status ${response.status}`);
  }

  return response.json();
}
