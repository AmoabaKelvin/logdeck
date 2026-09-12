import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ChevronDownIcon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import type { ContainerInspect } from "../api/get-container-inspect";
import { getContainerResources } from "../api/get-container-resources";
import {
	type UpdateResourcesRequest,
	updateContainerResources,
} from "../api/update-container-resources";
import type { ContainerStats } from "../types";
import { PanelError, PanelLoading, PanelSection } from "./container-panel-ui";
import { formatBytes } from "./container-utils";
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

/**
 * How close the container is running to its ceiling. Without a ceiling there is
 * nothing to be close to, so the bar is left out rather than drawn against the
 * host's total — that reads as headroom the container has not been granted.
 */
function UsageBar({ percent }: { percent: number }) {
	const clamped = Math.min(100, Math.max(0, percent));
	const tone =
		clamped >= 90
			? "bg-rose-500"
			: clamped >= 75
				? "bg-amber-500"
				: "bg-foreground/60";

	return (
		// The reading is spelled out next to the bar, so the bar is decoration.
		<div
			className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
			aria-hidden="true"
		>
			<div
				className={`h-full rounded-full ${tone} w-(--usage)`}
				style={{ "--usage": `${clamped}%` } as React.CSSProperties}
			/>
		</div>
	);
}

function Reading({
	title,
	value,
	detail,
	percent,
	children,
}: {
	title: string;
	value: string;
	detail?: string;
	percent: number | null;
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
				{percent === null ? (
					<p className="text-base text-muted-foreground sm:text-sm">
						No limit set — the container can use the whole host.
					</p>
				) : (
					<UsageBar percent={percent} />
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
	// Live usage, so a limit is set next to what the container actually does.
	stats: ContainerStats | undefined;
	inspect: ContainerInspect | undefined;
}

/**
 * Resource limits shown against live usage. Setting a memory ceiling without
 * seeing what the container is using is guesswork, so the two live together.
 */
export function ContainerLimitsPanel({
	containerId,
	containerHost,
	isReadOnly,
	stats,
	inspect,
}: ContainerLimitsPanelProps) {
	const queryClient = useQueryClient();
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
		queryKey: ["container-resources", containerId, containerHost],
		queryFn: () => getContainerResources(containerId, containerHost),
		enabled: !!containerId && !!containerHost,
	});

	useEffect(() => {
		if (!resources) return;
		setMemory(formatMemoryBytes(resources.memoryBytes));
		setCpus(resources.nanoCPUs > 0 ? String(resources.nanoCPUs / 1e9) : "");
		setRestartPolicy(resources.restartPolicy.name || "no");
		setMaxRetries(
			resources.restartPolicy.maximumRetryCount > 0
				? String(resources.restartPolicy.maximumRetryCount)
				: "",
		);
	}, [resources]);

	const updateMutation = useMutation({
		mutationFn: (request: UpdateResourcesRequest) =>
			updateContainerResources(containerId, containerHost, request),
		onSuccess: () => {
			queryClient.invalidateQueries({
				queryKey: ["container-resources", containerId, containerHost],
			});
			queryClient.invalidateQueries({ queryKey: ["containers"] });
			toast.success("Limits updated", {
				description: "Applied live — the container did not restart.",
			});
		},
		onError: (mutationError: Error) => {
			toast.error("Failed to update limits", {
				description: mutationError.message,
			});
		},
	});

	if (isLoading && !resources) {
		return <PanelLoading label="Reading limits…" />;
	}
	if (error || !resources) {
		return <PanelError>Could not read this container's limits.</PanelError>;
	}

	const retryCount =
		restartPolicy === "on-failure" ? Number.parseInt(maxRetries, 10) || 0 : 0;
	const isDirty =
		formatMemoryBytes(resources.memoryBytes) !== memory ||
		(resources.nanoCPUs > 0 ? String(resources.nanoCPUs / 1e9) : "") !== cpus ||
		(resources.restartPolicy.name || "no") !== restartPolicy ||
		resources.restartPolicy.maximumRetryCount !== retryCount;

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

		updateMutation.mutate({
			memoryBytes,
			nanoCPUs: Math.round(cpuValue * 1e9),
			restartPolicy: { name: restartPolicy, maximumRetryCount: retryCount },
		});
	};

	const memoryLimited = resources.memoryBytes > 0;
	const cpuCores = resources.nanoCPUs > 0 ? resources.nanoCPUs / 1e9 : null;

	return (
		<div className="grid gap-x-10 gap-y-8 lg:grid-cols-3">
			<Reading
				title="Memory"
				value={stats ? formatBytes(stats.memory_used) : "—"}
				detail={memoryLimited ? `of ${formatBytes(resources.memoryBytes)}` : ""}
				percent={
					memoryLimited && stats
						? (stats.memory_used / resources.memoryBytes) * 100
						: null
				}
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
						disabled={isReadOnly}
						placeholder="unlimited"
						className="h-10 w-32 font-mono sm:h-9"
					/>
				</div>
			</Reading>

			<Reading
				title="CPU"
				value={stats ? `${stats.cpu_percent.toFixed(1)}%` : "—"}
				detail={
					cpuCores ? `of ${cpuCores} core${cpuCores === 1 ? "" : "s"}` : ""
				}
				percent={
					cpuCores && stats
						? (stats.cpu_percent / (cpuCores * 100)) * 100
						: null
				}
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
						disabled={isReadOnly}
						placeholder="unlimited"
						className="h-10 w-32 font-mono sm:h-9"
					/>
				</div>
			</Reading>

			<PanelSection title="Restart policy">
				<div className="flex flex-wrap items-center gap-2">
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button
								variant="outline"
								disabled={isReadOnly}
								className="h-10 text-base sm:h-9 sm:text-sm"
							>
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
								disabled={isReadOnly}
								placeholder="0"
								className="h-10 w-24 font-mono sm:h-9"
							/>
						</div>
					)}
				</div>
				<p className="mt-3 text-base text-muted-foreground sm:text-sm">
					{inspect && inspect.RestartCount > 0
						? `Restarted ${inspect.RestartCount} time${inspect.RestartCount === 1 ? "" : "s"} so far.`
						: "Has not restarted yet."}
				</p>
			</PanelSection>

			<div className="flex items-center gap-3 lg:col-span-3">
				<Button
					onClick={handleSave}
					disabled={isReadOnly || !isDirty || updateMutation.isPending}
					className="h-10 text-base sm:h-9 sm:text-sm"
				>
					{updateMutation.isPending && <Spinner className="size-4" />}
					Save limits
				</Button>
				<p className="text-base text-muted-foreground sm:text-sm">
					{isReadOnly
						? "LogDeck is in read-only mode."
						: "Empty means unlimited. Changes apply without a restart."}
				</p>
			</div>
		</div>
	);
}
