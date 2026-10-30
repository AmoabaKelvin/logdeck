import { useMutation, useQueryClient } from "@tanstack/react-query";
import { formatDistanceToNowStrict } from "date-fns";
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { deployCoolifyEnv } from "../api/coolify-env";
import { checkImageUpdate, updateContainerImage } from "../api/image-update";
import type { ContainerInfo, ImageUpdate } from "../types";
import { PanelNote, PanelSection } from "./container-panel-ui";
import { getDeploymentOwner, getSystemdUnit } from "./container-utils";
import { EnvConfirmDialog } from "./env-confirm-dialog";

const STATUS = {
	available: {
		label: "Update available",
		className: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
	},
	current: {
		label: "Up to date",
		className: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
	},
	unknown: { label: "Unknown", className: "bg-muted text-muted-foreground" },
	unsupported: {
		label: "Not supported",
		className: "bg-muted text-muted-foreground",
	},
} satisfies Record<ImageUpdate["status"], { label: string; className: string }>;

/**
 * How to update a container whose owner would undo or fight a recreate from
 * LogDeck. Coolify is absent: LogDeck can ask it to deploy.
 */
function updateElsewhere(container: ContainerInfo): string | undefined {
	const labels = container.labels ?? {};
	const unit = getSystemdUnit(labels);
	if (unit) {
		return `systemd runs this container. Pull ${container.image} on the host, then run systemctl restart ${unit} (with --user for rootless Podman).`;
	}
	switch (getDeploymentOwner(container)?.name) {
		case "Dokploy":
			return "Dokploy deployed this container. Deploy it from the Environment panel, or in Dokploy, to pull the new image.";
		case "Swarm":
			// Swarm pins the digest it resolved; the service update resolves again.
			return `Swarm runs this container for a service. Update the service instead: docker service update --image ${container.image.split("@")[0]} ${labels["com.docker.swarm.service.name"]}`;
		case "Compose": {
			const dir = labels["com.docker.compose.project.working_dir"];
			return `Compose created this container, so update it with Compose: docker compose pull && docker compose up -d${dir ? ` in ${dir}` : ""}.`;
		}
	}
	return undefined;
}

interface ImageUpdateSectionProps {
	container: ContainerInfo;
	containerId: string;
	isReadOnly: boolean;
	onContainerRecreated: (newContainerId: string) => void;
}

/**
 * Whether the registry has a newer image for the container's tag, and the
 * way to move to it. The status comes with the container list, which the
 * server answers from its cache and refreshes in the background.
 */
export function ImageUpdateSection({
	container,
	containerId,
	isReadOnly,
	onContainerRecreated,
}: ImageUpdateSectionProps) {
	const queryClient = useQueryClient();
	const [confirm, setConfirm] = useState(false);
	const coolify = getDeploymentOwner(container)?.name === "Coolify";
	const elsewhere = updateElsewhere(container);

	const checkMutation = useMutation({
		mutationFn: () => checkImageUpdate(containerId, container.host),
		onSuccess: () =>
			queryClient.invalidateQueries({ queryKey: ["containers"] }),
		onError: (error: Error) =>
			toast.error("Image check failed", { description: error.message }),
	});

	const updateMutation = useMutation({
		mutationFn: () => updateContainerImage(containerId, container.host),
		onSuccess: (result) => {
			if (!result.updated || !result.new_container_id) {
				toast.success("Already up to date", { description: result.message });
				return;
			}
			checkMutation.reset();
			queryClient.invalidateQueries({ queryKey: ["containers"] });
			onContainerRecreated(result.new_container_id);
			toast.success("Image updated", {
				description: "Container recreated from the new image.",
			});
		},
		onError: (error: Error) =>
			toast.error("Image update failed", { description: error.message }),
	});

	const deployMutation = useMutation({
		mutationFn: () => deployCoolifyEnv(containerId, container.host),
		onSuccess: () =>
			toast.success("Coolify deployment requested", {
				description: "Check Coolify for progress.",
			}),
		onError: (error: Error) =>
			toast.error("Deployment request failed", { description: error.message }),
	});

	const update = checkMutation.data ?? container.image_update;
	const status = update && STATUS[update.status];
	const pending = updateMutation.isPending || deployMutation.isPending;

	return (
		<PanelSection
			title="Image updates"
			badge={
				status && (
					<Badge className={`border-transparent ${status.className}`}>
						{status.label}
					</Badge>
				)
			}
			action={
				<Button
					variant="ghost"
					size="sm"
					onClick={() => checkMutation.mutate()}
					disabled={checkMutation.isPending}
					className="-mr-2 h-8 text-muted-foreground hover:text-foreground"
				>
					{checkMutation.isPending && <Spinner className="size-4" />}
					Check now
				</Button>
			}
		>
			<PanelNote>
				{update
					? `${update.reason ? `${update.reason}. ` : ""}Checked ${formatDistanceToNowStrict(
							update.checked_at * 1000,
							{ addSuffix: true },
						)}.`
					: "Not checked yet. LogDeck checks images in the background."}
			</PanelNote>

			{update?.status !== "current" &&
				(elsewhere ? (
					<div className="mt-3">
						<PanelNote>{elsewhere}</PanelNote>
					</div>
				) : (
					<div className="mt-3 flex flex-wrap items-center gap-3">
						<Button
							variant="outline"
							onClick={() => setConfirm(true)}
							disabled={isReadOnly || pending}
						>
							{pending && <Spinner className="size-4" />}
							{coolify ? "Deploy in Coolify" : "Pull and recreate"}
						</Button>
						{isReadOnly && <PanelNote>LogDeck is in read-only mode.</PanelNote>}
					</div>
				))}

			<EnvConfirmDialog
				copy={
					!confirm
						? null
						: coolify
							? {
									title: "Deploy in Coolify?",
									description:
										"Coolify redeploys this resource from its saved settings. Check Coolify for progress.",
									confirmLabel: "Deploy",
								}
							: {
									title: "Pull and recreate?",
									description:
										"LogDeck pulls the container's image tag and, if a newer image arrives, recreates the container from it with the same settings. The container stops briefly.",
									confirmLabel: "Pull and recreate",
								}
				}
				onOpenChange={setConfirm}
				onConfirm={() =>
					coolify ? deployMutation.mutate() : updateMutation.mutate()
				}
			/>
		</PanelSection>
	);
}
