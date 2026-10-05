import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface CoolifyEnvChange {
	uuid?: string;
	key: string;
	value?: string;
	expected_value?: string | null;
	is_preview: boolean;
	remove?: boolean;
}

export async function saveCoolifyEnv(
	id: string,
	host: string,
	changes: CoolifyEnvChange[],
) {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env?host=${encodeURIComponent(host)}`,
		{
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ changes }),
		},
	);
	if (
		!response.ok &&
		!response.headers.get("Content-Type")?.includes("application/json")
	) {
		throw new Error((await response.text()).trim() || "Coolify save failed");
	}
	const data = await readJson<{
		message: string;
		saved: boolean;
		completed: number;
	}>(response);
	if (!response.ok || !data.saved) throw new Error(data.message);
	return data;
}

export async function deployCoolifyEnv(id: string, host: string) {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env/deploy?host=${encodeURIComponent(host)}`,
		{ method: "POST" },
	);
	if (!response.ok) throw new Error((await response.text()).trim());
}
