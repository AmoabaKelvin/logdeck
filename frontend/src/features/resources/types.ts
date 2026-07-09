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
