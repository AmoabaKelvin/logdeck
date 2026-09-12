import { ExternalLinkIcon } from "@/components/ui/icons";
import type { ContainerInspect } from "../api/get-container-inspect";
import {
	PanelError,
	PanelLoading,
	PanelNote,
	PanelSection,
} from "./container-panel-ui";
import { resolvePublishedHost } from "./container-utils";

interface PortBinding {
	hostPort: string;
	containerPort: string;
	protocol: string;
}

/**
 * Docker lists one binding per address family, so a single published port shows
 * up twice (0.0.0.0 and ::). Collapse to one row per host port.
 */
function collectBindings(inspect: ContainerInspect): PortBinding[] {
	const seen = new Map<string, PortBinding>();

	for (const [spec, bindings] of Object.entries(
		inspect.NetworkSettings.Ports ?? {},
	)) {
		if (!bindings) continue;
		const [containerPort, protocol = "tcp"] = spec.split("/");
		for (const binding of bindings) {
			const key = `${binding.HostPort}/${protocol}`;
			if (seen.has(key)) continue;
			seen.set(key, { hostPort: binding.HostPort, containerPort, protocol });
		}
	}

	return Array.from(seen.values()).sort(
		(a, b) => Number(a.hostPort) - Number(b.hostPort),
	);
}

function PortRow({
	binding,
	publishedHost,
}: {
	binding: PortBinding;
	publishedHost: string | null;
}) {
	const target =
		publishedHost && binding.protocol === "tcp"
			? `http://${publishedHost}:${binding.hostPort}`
			: null;

	return (
		<li className="flex items-center justify-between gap-4 py-2 first:pt-0 last:pb-0">
			<span className="font-mono text-base tabular-nums sm:text-sm">
				{binding.hostPort}
				<span className="px-1.5 text-muted-foreground">→</span>
				{binding.containerPort}
				<span className="text-muted-foreground">/{binding.protocol}</span>
			</span>
			{target && (
				<a
					href={target}
					target="_blank"
					rel="noreferrer"
					title={target}
					className="inline-flex shrink-0 items-center gap-1.5 rounded-sm text-base text-muted-foreground hover:text-foreground sm:text-sm"
				>
					Open
					<ExternalLinkIcon className="size-4 shrink-0" />
				</a>
			)}
		</li>
	);
}

interface ContainerNetworkPanelProps {
	inspect: ContainerInspect | undefined;
	isLoading: boolean;
	isError: boolean;
	// The Docker host's address, used to work out where published ports live.
	hostAddress: string | undefined;
}

/** Everything the container is attached to: networks, ports, and storage. */
export function ContainerNetworkPanel({
	inspect,
	isLoading,
	isError,
	hostAddress,
}: ContainerNetworkPanelProps) {
	if (isLoading && !inspect) {
		return <PanelLoading label="Inspecting container…" />;
	}
	if (isError || !inspect) {
		return <PanelError>Could not inspect this container.</PanelError>;
	}

	const networks = Object.entries(inspect.NetworkSettings.Networks ?? {});
	const bindings = collectBindings(inspect);
	const mounts = inspect.Mounts ?? [];
	const publishedHost = resolvePublishedHost(
		hostAddress,
		window.location.hostname,
	);

	return (
		<div className="grid gap-x-10 gap-y-8 lg:grid-cols-3">
			<PanelSection title="Published ports">
				{bindings.length === 0 ? (
					<PanelNote>No ports published to the host.</PanelNote>
				) : (
					<ul className="divide-y divide-border/60">
						{bindings.map((binding) => (
							<PortRow
								key={`${binding.hostPort}/${binding.protocol}`}
								binding={binding}
								publishedHost={publishedHost}
							/>
						))}
					</ul>
				)}
			</PanelSection>

			<PanelSection title="Networks">
				{networks.length === 0 ? (
					<PanelNote>Not attached to any network.</PanelNote>
				) : (
					<ul className="divide-y divide-border/60 text-base sm:text-sm">
						{networks.map(([name, network]) => (
							<li key={name} className="py-2 first:pt-0 last:pb-0">
								<p className="truncate font-medium" title={name}>
									{name}
								</p>
								<p className="font-mono text-muted-foreground">
									{network.IPAddress || "no address"}
									{network.Gateway && ` · via ${network.Gateway}`}
								</p>
								{network.Aliases && network.Aliases.length > 0 && (
									<p
										className="truncate text-muted-foreground"
										title={network.Aliases.join(", ")}
									>
										aka {network.Aliases.join(", ")}
									</p>
								)}
							</li>
						))}
					</ul>
				)}
			</PanelSection>

			<PanelSection title="Mounts">
				{mounts.length === 0 ? (
					<PanelNote>No volumes or binds.</PanelNote>
				) : (
					<ul className="divide-y divide-border/60 text-base sm:text-sm">
						{mounts.map((mount) => (
							<li key={mount.Destination} className="py-2 first:pt-0 last:pb-0">
								<p className="truncate font-mono" title={mount.Destination}>
									{mount.Destination}
								</p>
								<p
									className="truncate text-muted-foreground"
									title={mount.Source}
								>
									{mount.Type === "volume"
										? (mount.Name ?? mount.Source)
										: mount.Source}
									<span className="px-1.5">·</span>
									{mount.Type}
									<span className="px-1.5">·</span>
									{mount.RW ? "read-write" : "read-only"}
								</p>
							</li>
						))}
					</ul>
				)}
			</PanelSection>
		</div>
	);
}
