import { useQuery } from "@tanstack/react-query";

import { getImages, getNetworks, getVolumes } from "../api/get-resources";

export function useImagesQuery() {
	return useQuery({
		queryKey: ["images"],
		queryFn: getImages,
		staleTime: 10_000,
	});
}

export function useVolumesQuery() {
	return useQuery({
		queryKey: ["volumes"],
		queryFn: getVolumes,
		staleTime: 10_000,
	});
}

export function useNetworksQuery() {
	return useQuery({
		queryKey: ["networks"],
		queryFn: getNetworks,
		staleTime: 10_000,
	});
}
