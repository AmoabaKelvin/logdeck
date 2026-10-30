import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { UserRole } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/users`;

export interface CreateUserPayload {
	username: string;
	password: string;
	role: UserRole;
}

export async function createUser(payload: CreateUserPayload): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to create user");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "User created";
}
