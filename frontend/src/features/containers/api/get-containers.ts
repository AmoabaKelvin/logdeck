import type { ContainerInfo } from "../types";

const CONTAINERS_ENDPOINT = "http://localhost:8080/api/v1/containers";

export async function getContainers(): Promise<ContainerInfo[]> {
  const response = await fetch(CONTAINERS_ENDPOINT);

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  const data = (await response.json()) as unknown;

  if (!data || typeof data !== "object" || data === null) {
    throw new Error("Unexpected response format");
  }

  const containers = (data as { containers?: unknown }).containers;

  if (!Array.isArray(containers)) {
    throw new Error("Unexpected response format");
  }

  return containers as ContainerInfo[];
}
