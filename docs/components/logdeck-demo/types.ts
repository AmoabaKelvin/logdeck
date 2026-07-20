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

export interface ImageInfo {
	id: string;
	repo_tags: string[] | null;
	size: number;
	created: number;
	host: string;
}

export interface VolumeInfo {
	name: string;
	driver: string;
	mountpoint: string;
	created: string;
	labels?: Record<string, string>;
	host: string;
}

export interface NetworkInfo {
	id: string;
	name: string;
	driver: string;
	scope: string;
	subnets?: string[];
	host: string;
}
