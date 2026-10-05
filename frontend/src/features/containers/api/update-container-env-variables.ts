import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

interface UpdateEnvResponse {
	message: string;
	new_container_id: string;
}

export interface UpdateEnvResult {
	newContainerId: string;
}

export async function updateContainerEnvVariables(
	id: string,
	host: string,
	env: Record<string, string>,
	plainCompose = false,
): Promise<UpdateEnvResult> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env?host=${encodeURIComponent(host)}${plainCompose ? "&platform=docker&confirmed=true" : ""}`,
		{
			method: "PUT",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify({ env }),
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(
			message.trim() || "Failed to update container environment variables",
		);
	}

	const data = await readJson<UpdateEnvResponse>(response);
	return {
		newContainerId: data.new_container_id,
	};
}
