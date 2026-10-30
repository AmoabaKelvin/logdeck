import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ChevronDownIcon, ExternalLinkIcon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import type { ContainerInspect } from "../api/get-container-inspect";
import {
	getContainerResources,
	type ResourceLimits,
	type ResourcesOwner,
} from "../api/get-container-resources";
import {
	deployContainerResources,
	type UpdateResourcesRequest,
	updateContainerResources,
} from "../api/update-container-resources";
import type { ContainerStats } from "../types";
import {
	PanelError,
	PanelLoading,
	PanelNote,
	PanelSection,
} from "./container-panel-ui";
import { formatBytes } from "./container-utils";
import { PLAIN_COMPOSE, readDokployMapping } from "./dokploy-mapping";
import { type EnvConfirmCopy, EnvConfirmDialog } from "./env-confirm-dialog";
import { Meter } from "./meter";
import { formatMemoryBytes, parseMemoryInput } from "./parse-memory";

const RESTART_POLICIES = [
	{ value: "no", label: "Never" },
	{ value: "always", label: "Always" },
	{ value: "unless-stopped", label: "Unless stopped" },
	{ value: "on-failure", label: "On failure" },
] as const;

function policyLabel(value: string) {
	return RESTART_POLICIES.find((p) => p.value === value)?.label ?? value;
}

const PLATFORM_NAMES = {
	coolify: "Coolify",
	dokploy: "Dokploy",
	swarm: "its Swarm service",
	compose: "Compose",
} satisfies Record<ResourcesOwner["platform"], string>;

const DEPLOY_COPY = {
	coolify: {
		title: "Deploy through Coolify?",
		description:
			"Coolify redeploys the application with all of its saved settings, including these limits. Its container is replaced, so the application may be briefly unavailable.",
		confirmLabel: "Deploy",
	},
	dokploy: {
		title: "Deploy through Dokploy?",
		description:
			"Dokploy rebuilds and redeploys the application with all of its saved settings, including these limits. Its tasks are replaced, so the application may be briefly unavailable.",
		confirmLabel: "Deploy",
	},
} satisfies Record<string, EnvConfirmCopy>;

const LINK =
	"inline-flex shrink-0 items-center gap-1.5 rounded-sm text-base text-muted-foreground hover:text-foreground sm:text-sm";

/**
 * The Dokploy deployment confirmed in the Environment panel, remembered for
 * the Swarm service, stack or Compose project the way the server keys it.
 */
function confirmedDokployMapping(
	host: string,
	containerId: string,
	labels: Record<string, string> | undefined,
) {
	const key = [
		"com.docker.stack.namespace",
		"com.docker.swarm.service.name",
		"com.docker.compose.project",
	].find((label) => labels?.[label]);
	const mapping =
		(key && readDokployMapping(host, `${key}=${labels?.[key]}`)) ||
		readDokployMapping(host, containerId);
	return mapping === PLAIN_COMPOSE ? undefined : mapping;
}

function describeLimits(limits: ResourceLimits) {
	const memory = limits.memoryBytes
		? formatBytes(limits.memoryBytes)
		: "no memory limit";
	const cpus = limits.nanoCPUs
		? `${limits.nanoCPUs / 1e9} CPU${limits.nanoCPUs === 1e9 ? "" : "s"}`
		: "no CPU limit";
	return `${memory}, ${cpus}`;
}

/**
 * A reading against its ceiling. Without a ceiling there is nothing to be
 * close to, so no bar is drawn rather than one against the host's total —
 * that reads as headroom the container has not been granted.
 */
function Reading({
	title,
	value,
	detail,
	used,
	limit,
	children,
}: {
	title: string;
	value: string;
	detail?: string;
	used: number | null;
	limit: number | null;
	children: React.ReactNode;
}) {
	return (
		<div className="min-w-0">
			<div className="flex items-baseline justify-between gap-3">
				<h3 className="text-base font-medium sm:text-sm">{title}</h3>
				<p className="text-base tabular-nums sm:text-sm">
					{value}
					{detail && (
						<span className="ml-1.5 text-muted-foreground">{detail}</span>
					)}
				</p>
			</div>
			<div className="mt-2">
				{limit === null || used === null ? (
					<p className="text-base text-muted-foreground sm:text-sm">
						No limit set — the container can use the whole host.
					</p>
				) : (
					<Meter used={used} limit={limit} className="w-full" />
				)}
			</div>
			<div className="mt-3">{children}</div>
		</div>
	);
}

interface ContainerLimitsPanelProps {
	containerId: string;
	containerHost: string;
	isReadOnly: boolean;
	labels: Record<string, string> | undefined;
	// Live usage, so a limit is set next to what the container actually does.
	stats: ContainerStats | undefined;
	inspect: ContainerInspect | undefined;
}

/**
 * Resource limits shown against live usage. Setting a memory ceiling without
 * seeing what the container is using is guesswork, so the two live together.
 *
 * A platform recreates its containers from saved settings, so for those the
 * form edits the platform's saved limits and deploying applies them; the
 * readings keep showing what the container runs with now.
 */
export function ContainerLimitsPanel({
	containerId,
	containerHost,
	isReadOnly,
	labels,
	stats,
	inspect,
}: ContainerLimitsPanelProps) {
	const queryClient = useQueryClient();
	// Read once: the mapping is confirmed in the Environment panel, and only
	// one panel is open at a time.
	const [dokployMapping] = useState(() =>
		confirmedDokployMapping(containerHost, containerId, labels),
	);
	const [saved, setSaved] = useState(false);
	const [confirmDeploy, setConfirmDeploy] = useState(false);
	const [memory, setMemory] = useState("");
	const [cpus, setCpus] = useState("");
	const [restartPolicy, setRestartPolicy] = useState("no");
	const [maxRetries, setMaxRetries] = useState("");
	const memoryId = useId();
	const cpusId = useId();
	const maxRetriesId = useId();

	const {
		data: resources,
		isLoading,
		error,
	} = useQuery({
		queryKey: [
			"container-resources",
			containerId,
			containerHost,
			dokployMapping,
		],
		queryFn: () =>
			getContainerResources(containerId, containerHost, dokployMapping),
		enabled: !!containerId && !!containerHost,
	});

	const owner = resources?.owner;
	// Plain Compose updates live too; the Compose file has to keep the change.
	const live = !owner || owner.platform === "compose";
	const configured = owner?.persistent ? owner.configured : undefined;
	const editable = live || Boolean(configured);
	const platformName = owner ? PLATFORM_NAMES[owner.platform] : "";
	// What the form edits: the platform's saved limits, or the running ones.
	const limits = configured ?? resources;

	// Reset the form to the server's values whenever they (re)load.
	const [formResources, setFormResources] = useState<typeof resources>();
	if (resources && limits && resources !== formResources) {
		setFormResources(resources);
		setMemory(formatMemoryBytes(limits.memoryBytes));
		setCpus(limits.nanoCPUs > 0 ? String(limits.nanoCPUs / 1e9) : "");
		setRestartPolicy(resources.restartPolicy.name || "no");
		setMaxRetries(
			resources.restartPolicy.maximumRetryCount > 0
				? String(resources.restartPolicy.maximumRetryCount)
				: "",
		);
	}

	const refresh = () =>
		queryClient.invalidateQueries({
			queryKey: ["container-resources", containerId, containerHost],
		});

	const updateMutation = useMutation({
		mutationFn: (request: UpdateResourcesRequest) =>
			updateContainerResources(
				containerId,
				containerHost,
				request,
				dokployMapping,
			),
		onSuccess: (result) => {
			refresh();
			if (!result.applied) {
				setSaved(true);
				toast.success(`Limits saved in ${platformName}`, {
					description:
						"Deploy to apply them. The container keeps its current limits until then.",
				});
				return;
			}
			queryClient.invalidateQueries({ queryKey: ["containers"] });
			toast.success("Limits updated", {
				description: "Applied live — the container did not restart.",
			});
		},
		onError: (mutationError: Error) => {
			// A failed platform save may still have landed, so show what the
			// platform holds now.
			if (!live) {
				setSaved(false);
				refresh();
			}
			toast.error(live ? "Failed to update limits" : "Failed to save limits", {
				description: mutationError.message,
			});
		},
	});

	const deployMutation = useMutation({
		mutationFn: () =>
			deployContainerResources(containerId, containerHost, dokployMapping),
		onSuccess: () => {
			setSaved(false);
			toast.success(`${platformName} deployment requested`, {
				description: `Check ${platformName} for progress. The container ID may change.`,
			});
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (deployError: Error) =>
			toast.error("Deployment request failed", {
				description: deployError.message,
			}),
	});

	if (isLoading && !resources) {
		return <PanelLoading label="Reading limits…" />;
	}
	if (error || !resources || !limits) {
		return <PanelError>Could not read this container's limits.</PanelError>;
	}

	const retryCount =
		restartPolicy === "on-failure" ? Number.parseInt(maxRetries, 10) || 0 : 0;
	const isDirty =
		formatMemoryBytes(limits.memoryBytes) !== memory ||
		(limits.nanoCPUs > 0 ? String(limits.nanoCPUs / 1e9) : "") !== cpus ||
		(live &&
			((resources.restartPolicy.name || "no") !== restartPolicy ||
				resources.restartPolicy.maximumRetryCount !== retryCount));
	// Saved limits the container does not run with yet wait for a deployment.
	const differs =
		configured !== undefined &&
		(configured.memoryBytes !== resources.memoryBytes ||
			configured.nanoCPUs !== resources.nanoCPUs);
	const deployCopy =
		owner?.platform === "coolify" || owner?.platform === "dokploy"
			? DEPLOY_COPY[owner.platform]
			: null;
	const busy = updateMutation.isPending || deployMutation.isPending;

	const handleSave = () => {
		const memoryBytes = parseMemoryInput(memory);
		if (memoryBytes === null) {
			toast.error("Invalid memory limit", {
				description: 'Use a number with an optional unit, e.g. "512m" or "1g".',
			});
			return;
		}

		const cpuValue = cpus.trim() === "" ? 0 : Number.parseFloat(cpus);
		if (Number.isNaN(cpuValue) || cpuValue < 0) {
			toast.error("Invalid CPU limit", {
				description: "Use a positive number, e.g. 0.5 or 2.",
			});
			return;
		}

		const nanoCPUs = Math.round(cpuValue * 1e9);
		updateMutation.mutate(
			live
				? {
						memoryBytes,
						nanoCPUs,
						restartPolicy: {
							name: restartPolicy,
							maximumRetryCount: retryCount,
						},
					}
				: { memoryBytes, nanoCPUs },
		);
	};

	const memoryLimited = resources.memoryBytes > 0;
	const cpuCores = resources.nanoCPUs > 0 ? resources.nanoCPUs / 1e9 : null;

	return (
		<div className="grid gap-x-10 gap-y-8 lg:grid-cols-3">
			<Reading
				title="Memory"
				value={stats ? formatBytes(stats.memory_used) : "—"}
				detail={memoryLimited ? `of ${formatBytes(resources.memoryBytes)}` : ""}
				used={memoryLimited && stats ? stats.memory_used : null}
				limit={memoryLimited ? resources.memoryBytes : null}
			>
				<div className="flex items-center gap-2">
					<Label
						htmlFor={memoryId}
						className="text-base text-muted-foreground sm:text-sm"
					>
						Limit
					</Label>
					<Input
						id={memoryId}
						name="memory-limit"
						value={memory}
						onChange={(e) => setMemory(e.target.value)}
						disabled={isReadOnly || !editable}
						placeholder="unlimited"
						className="w-32 font-mono"
					/>
				</div>
			</Reading>

			<Reading
				title="CPU"
				value={stats ? `${stats.cpu_percent.toFixed(1)}%` : "—"}
				detail={
					cpuCores ? `of ${cpuCores} core${cpuCores === 1 ? "" : "s"}` : ""
				}
				used={cpuCores && stats ? stats.cpu_percent : null}
				limit={cpuCores ? cpuCores * 100 : null}
			>
				<div className="flex items-center gap-2">
					<Label
						htmlFor={cpusId}
						className="text-base text-muted-foreground sm:text-sm"
					>
						Limit
					</Label>
					<Input
						id={cpusId}
						name="cpu-limit"
						type="number"
						min="0"
						step="0.1"
						value={cpus}
						onChange={(e) => setCpus(e.target.value)}
						disabled={isReadOnly || !editable}
						placeholder="unlimited"
						className="w-32 font-mono"
					/>
				</div>
			</Reading>

			<PanelSection title="Restart policy">
				<div className="flex flex-wrap items-center gap-2">
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline" disabled={isReadOnly || !live}>
								{policyLabel(restartPolicy)}
								<ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="start">
							<DropdownMenuRadioGroup
								value={restartPolicy}
								onValueChange={setRestartPolicy}
							>
								{RESTART_POLICIES.map((policy) => (
									<DropdownMenuRadioItem
										key={policy.value}
										value={policy.value}
									>
										{policy.label}
									</DropdownMenuRadioItem>
								))}
							</DropdownMenuRadioGroup>
						</DropdownMenuContent>
					</DropdownMenu>

					{restartPolicy === "on-failure" && (
						<div className="flex items-center gap-2">
							<Label
								htmlFor={maxRetriesId}
								className="text-base text-muted-foreground sm:text-sm"
							>
								Max retries
							</Label>
							<Input
								id={maxRetriesId}
								name="max-retries"
								type="number"
								min="0"
								step="1"
								value={maxRetries}
								onChange={(e) => setMaxRetries(e.target.value)}
								disabled={isReadOnly || !live}
								placeholder="0"
								className="w-24 font-mono"
							/>
						</div>
					)}
				</div>
				<p className="mt-3 text-base text-muted-foreground sm:text-sm">
					{inspect && inspect.RestartCount > 0
						? `Restarted ${inspect.RestartCount} time${inspect.RestartCount === 1 ? "" : "s"} so far.`
						: "Has not restarted yet."}
					{!live && ` Set by ${platformName}, not here.`}
				</p>
			</PanelSection>

			{configured && differs && (
				<div className="lg:col-span-3">
					<PanelNote>
						Running with {describeLimits(resources)}. {platformName} has{" "}
						{describeLimits(configured)} saved, which its next deployment
						applies.
					</PanelNote>
				</div>
			)}

			<div className="flex flex-wrap items-center gap-3 lg:col-span-3">
				<Button
					onClick={handleSave}
					disabled={isReadOnly || !editable || !isDirty || busy}
				>
					{updateMutation.isPending && <Spinner className="size-4" />}
					{live ? "Save limits" : `Save in ${platformName}`}
				</Button>
				{deployCopy && configured && (saved || differs) && !isReadOnly && (
					<Button
						variant="outline"
						onClick={() => setConfirmDeploy(true)}
						disabled={busy}
					>
						{deployMutation.isPending && <Spinner className="size-4" />}
						Deploy through {platformName}
					</Button>
				)}
				<p className="text-base text-muted-foreground sm:text-sm">
					{isReadOnly
						? "LogDeck is in read-only mode."
						: !editable
							? owner?.reason
							: configured
								? `Empty means unlimited. Saved in ${platformName}; they apply when it deploys.`
								: owner?.platform === "compose"
									? "Applies live without a restart. Compose recreates this container from its file, so set the limits there too to keep them."
									: "Empty means unlimited. Changes apply without a restart."}
				</p>
				{!editable && owner?.url && (
					<a href={owner.url} target="_blank" rel="noreferrer" className={LINK}>
						Open {platformName}
						<ExternalLinkIcon className="size-4 shrink-0" />
					</a>
				)}
			</div>

			<EnvConfirmDialog
				copy={confirmDeploy ? deployCopy : null}
				onOpenChange={setConfirmDeploy}
				onConfirm={() => deployMutation.mutate()}
			/>
		</div>
	);
}
