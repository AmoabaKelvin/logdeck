import { useNetworksQuery } from "../hooks/use-resources";
import type { NetworkInfo } from "../types";
import type { ResourceColumn } from "./resource-page";
import { ResourcePage } from "./resource-page";

export function NetworksPage() {
	const { data, isLoading, error, isFetching, refetch } = useNetworksQuery();
	const items = data?.items ?? [];
	const hostErrors = data?.hostErrors ?? [];

	const showHost = new Set(items.map((item) => item.host)).size > 1;

	const columns: ResourceColumn<NetworkInfo>[] = [
		{ header: "Name", cell: (network) => network.name },
		{
			header: "ID",
			cell: (network) => (
				<span className="font-mono text-xs">{network.id}</span>
			),
		},
		{ header: "Driver", cell: (network) => network.driver },
		{ header: "Scope", cell: (network) => network.scope },
		{
			header: "Subnets",
			cell: (network) => network.subnets?.join(", ") ?? "—",
		},
		...(showHost
			? [{ header: "Host", cell: (network: NetworkInfo) => network.host }]
			: []),
	];

	return (
		<ResourcePage
			title="Networks"
			items={items}
			hostErrors={hostErrors}
			isLoading={isLoading}
			error={error}
			isFetching={isFetching}
			onRefresh={() => void refetch()}
			columns={columns}
			rowKey={(network) => `${network.host}-${network.id}`}
			filterText={(network) =>
				`${network.name} ${network.id} ${network.driver} ${network.scope} ${(network.subnets ?? []).join(" ")} ${network.host}`
			}
		/>
	);
}
