import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

interface UpdateEnvResponse {
  message: string;
  new_container_id: string;
}

export async function updateContainerEnvVariables(
  id: string,
  env: Record<string, string>
): Promise<string> {
  const response = await authenticatedFetch(
    `${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ env }),
    }
  );

  if (!response.ok) {
    throw new Error("Failed to update container environment variables");
  }

  const data: UpdateEnvResponse = await response.json();
  return data.new_container_id;
}
