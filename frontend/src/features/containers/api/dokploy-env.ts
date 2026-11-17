import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface DokployResource {
	type: "application" | "compose";
	mapping_revision: string;
	id: string;
	name: string;
	appName: string;
	serverId: string | null;
	project: string;
	environment: string;
}

export interface DokployEnvironment {
	text: string | null;
	revision: string;
	createEnvFile: boolean;
}

export interface DokployEnvResponse {
	source: "dokploy";
	env: Record<string, string>;
	resources?: DokployResource[];
	suggested_resources?: string[];
	resource?: DokployResource;
	configuration?: DokployEnvironment;
	mapping_required?: boolean;
	plain_compose_allowed?: boolean;
	/** Dokploy could not be read, so ownership is unknown rather than absent. */
	inventory_error?: string;
	instance_url: string;
	workload?: string;
	/** The Swarm service, stack or Compose project a mapping is remembered for. */
	mapping_key?: string;
	/** The Compose or Swarm service the container runs for. */
	workload_service?: string;
}

/**
 * Where a requested deployment has got to. Dokploy reports "done" before Swarm
 * replaces the task and Swarm may roll back, so only "completed" means the
 * saved configuration is running.
 */
export interface DokployRollout {
	phase:
		| "queued"
		| "building"
		| "failed"
		| "updating"
		| "completed"
		| "rolling_back"
		| "rolled_back"
		| "paused";
	message?: string;
	/** The container running the workload once the rollout completed. */
	container_id?: string;
}

function endpoint(
	id: string,
	host: string,
	resource: DokployResource,
	deploy = false,
) {
	const query = new URLSearchParams({
		host,
		resource: resource.id,
		resource_type: resource.type,
		confirmed: "true",
		mapping_revision: resource.mapping_revision,
	});
	return `${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env${deploy ? "/deploy" : ""}?${query}`;
}

/** A failed Dokploy request, with the HTTP status LogDeck answered. */
export class DokployRequestError extends Error {
	constructor(
		message: string,
		readonly status: number,
	) {
		super(message);
	}
}

async function request<T>(
	url: string,
	method = "GET",
	body?: { text: string; revision: string },
): Promise<T> {
	const options: RequestInit = { method };
	if (body !== undefined) {
		options.headers = { "Content-Type": "application/json" };
		options.body = JSON.stringify(body);
	}
	const response = await authenticatedFetch(url, options);
	if (!response.ok) {
		const message = response.headers
			.get("Content-Type")
			?.includes("application/json")
			? (await readJson<{ message: string }>(response)).message
			: (await response.text()).trim();
		throw new DokployRequestError(
			message || "Dokploy request failed",
			response.status,
		);
	}
	return readJson<T>(response);
}

export function readDokployEnv(
	id: string,
	host: string,
	resource: DokployResource,
) {
	return request<DokployEnvResponse>(endpoint(id, host, resource));
}

export function saveDokployEnv(
	id: string,
	host: string,
	resource: DokployResource,
	text: string,
	revision: string,
) {
	return request<{
		saved: boolean;
		applied: false;
		configuration: DokployEnvironment;
	}>(endpoint(id, host, resource), "PUT", { text, revision });
}

export function deployDokployEnv(
	id: string,
	host: string,
	resource: DokployResource,
) {
	return request<{ applied: false; after_deployment: string }>(
		endpoint(id, host, resource, true),
		"POST",
	);
}

/**
 * Follows the deployments that started after afterDeployment, the deploy
 * response's baseline. It does not go through the container, which the
 * deployment replaces.
 */
export function getDokployRollout(
	host: string,
	resource: DokployResource,
	afterDeployment: string,
	service: string,
) {
	const query = new URLSearchParams({
		host,
		resource: resource.id,
		resource_type: resource.type,
		mapping_revision: resource.mapping_revision,
		after: afterDeployment,
		service,
	});
	return request<DokployRollout>(
		`${API_BASE_URL}/api/v1/dokploy/rollout?${query}`,
	);
}
