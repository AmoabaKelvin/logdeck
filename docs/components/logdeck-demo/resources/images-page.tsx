import { formatDistanceToNow } from "date-fns";

import { formatBytes } from "@/components/logdeck-demo/containers/container-utils";

import { useImagesQuery } from "@/components/logdeck-demo/hooks/use-resources";
import type { ImageInfo } from "@/components/logdeck-demo/types";
import type { ResourceColumn } from "./resource-page";
import { ResourcePage } from "./resource-page";

export function ImagesPage() {
	const { data, isLoading, error, isFetching, refetch } = useImagesQuery();
	const items = data?.items ?? [];
	const hostErrors = data?.hostErrors ?? [];

	const showHost = new Set(items.map((item) => item.host)).size > 1;

	const columns: ResourceColumn<ImageInfo>[] = [
		{
			header: "Tags",
			cell: (image) =>
				image.repo_tags?.length ? (
					image.repo_tags.join(", ")
				) : (
					<span className="text-muted-foreground">&lt;none&gt; (dangling)</span>
				),
		},
		{
			header: "ID",
			cell: (image) => <span className="font-mono text-xs">{image.id}</span>,
		},
		{ header: "Size", cell: (image) => formatBytes(image.size) },
		{
			header: "Created",
			cell: (image) =>
				formatDistanceToNow(new Date(image.created * 1000), {
					addSuffix: true,
				}),
		},
		...(showHost
			? [{ header: "Host", cell: (image: ImageInfo) => image.host }]
			: []),
	];

	return (
		<ResourcePage
			title="Images"
			items={items}
			hostErrors={hostErrors}
			isLoading={isLoading}
			error={error}
			isFetching={isFetching}
			onRefresh={() => void refetch()}
			columns={columns}
			rowKey={(image) => `${image.host}-${image.id}`}
			filterText={(image) =>
				`${(image.repo_tags ?? []).join(" ")} ${image.id} ${image.host}`
			}
		/>
	);
}
