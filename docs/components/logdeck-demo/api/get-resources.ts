import { demoImages, demoNetworks, demoVolumes } from "@/lib/logdeck-demo/store";

import type { HostError, ImageInfo, NetworkInfo, VolumeInfo } from "../types";

export interface ResourceResponse<T> {
	items: T[];
	hostErrors: HostError[];
}

export async function getImages(): Promise<ResourceResponse<ImageInfo>> {
	return { items: structuredClone(demoImages), hostErrors: [] };
}

export async function getVolumes(): Promise<ResourceResponse<VolumeInfo>> {
	return { items: structuredClone(demoVolumes), hostErrors: [] };
}

export async function getNetworks(): Promise<ResourceResponse<NetworkInfo>> {
	return { items: structuredClone(demoNetworks), hostErrors: [] };
}
