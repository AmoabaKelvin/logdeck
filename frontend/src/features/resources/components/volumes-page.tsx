import { formatDistanceToNow } from "date-fns";
import { useMemo } from "react";

import { useVolumesQuery } from "../hooks/use-resources";
import type { VolumeInfo } from "../types";
import type { ResourceColumn } from "./resource-page";
import { ResourcePage } from "./resource-page";

function formatCreated(created: string): string {
	const date = new Date(created);
	if (!created || Number.isNaN(date.getTime())) return "—";
	return formatDistanceToNow(date, { addSuffix: true });
}

export function VolumesPage() {
	const { data, isLoading, error, isFetching, refetch } = useVolumesQuery();
	const items = data?.items ?? [];
	const hostErrors = data?.hostErrors ?? [];

	const showHost = useMemo(
		() => new Set(items.map((item) => item.host)).size > 1,
		[items],
	);

	const columns: ResourceColumn<VolumeInfo>[] = [
		{ header: "Name", cell: (volume) => volume.name },
		{ header: "Driver", cell: (volume) => volume.driver },
		{
			header: "Mountpoint",
			cell: (volume) => (
				<span className="font-mono text-xs">{volume.mountpoint}</span>
			),
		},
		{ header: "Created", cell: (volume) => formatCreated(volume.created) },
		{
			header: "Labels",
			cell: (volume) => Object.keys(volume.labels ?? {}).length,
		},
		...(showHost
			? [{ header: "Host", cell: (volume: VolumeInfo) => volume.host }]
			: []),
	];

	return (
		<ResourcePage
			title="Volumes"
			items={items}
			hostErrors={hostErrors}
			isLoading={isLoading}
			error={error}
			isFetching={isFetching}
			onRefresh={() => void refetch()}
			columns={columns}
			rowKey={(volume) => `${volume.host}-${volume.name}`}
			filterText={(volume) =>
				`${volume.name} ${volume.driver} ${volume.mountpoint} ${volume.host}`
			}
		/>
	);
}
