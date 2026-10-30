export interface DockerHost {
	name: string;
	host: string;
}

export interface ContainerInfo {
	id: string;
	names: string[];
	image: string;
	image_id: string;
	command: string;
	created: number;
	state: string;
	status: string;
	health?: string;
	labels?: Record<string, string>;
	host: string;
	ports?: ContainerPort[];
	/** Set by the server when a platform owns the container. */
	platform?: "dokploy";
	/** Absent until the server's first background check finishes. */
	image_update?: ImageUpdate;
}

/** The last check of a container's image against its registry. */
export interface ImageUpdate {
	/** "unknown" carries a reason; "unsupported" means the engine cannot ask. */
	status: "available" | "current" | "unknown" | "unsupported";
	reason?: string;
	/** Unix seconds. */
	checked_at: number;
}

export interface ContainerPort {
	publicPort: number;
	privatePort: number;
	type: string;
}

export interface HostError {
	host: string;
	message: string;
}

export interface ContainerStats {
	id: string;
	host: string;
	cpu_percent: number;
	memory_percent: number;
	memory_used: number;
	memory_limit: number;
}

export type ContainerStatsMap = Record<string, ContainerStats>;
