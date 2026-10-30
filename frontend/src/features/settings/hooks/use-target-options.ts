import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { getContainers } from "@/features/containers/api/get-containers";
import {
	formatContainerName,
	getComposeProject,
} from "@/features/containers/components/container-utils";

/** Suggestions for rule targeting: the container names, compose projects and hosts in view. */
export function useTargetOptions() {
	const containersQuery = useQuery({
		queryKey: ["containers"],
		queryFn: getContainers,
		staleTime: 30_000,
	});

	return useMemo(() => {
		const containers = new Set<string>();
		const projects = new Set<string>();
		const hosts = new Set<string>();
		for (const container of containersQuery.data?.containers ?? []) {
			const name = formatContainerName(container.names);
			if (name !== "—") containers.add(name);
			const project = getComposeProject(container.labels);
			if (project) projects.add(project);
			if (container.host) hosts.add(container.host);
		}
		for (const host of containersQuery.data?.hosts ?? []) {
			hosts.add(host.name);
		}
		return {
			containers: [...containers],
			projects: [...projects],
			hosts: [...hosts],
		};
	}, [containersQuery.data]);
}
