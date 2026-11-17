import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { Spinner } from "@/components/ui/spinner";

import {
	DokployRequestError,
	type DokployResource,
	type DokployRollout,
	getDokployRollout,
} from "../api/dokploy-env";
import { PanelError, PanelNote } from "./container-panel-ui";

const POLL_MS = 3000;
// Dokploy runs one deployment at a time, so a request can wait behind others
// before its build even starts.
const WATCH_MS = 30 * 60 * 1000;

const FINISHED: ReadonlySet<DokployRollout["phase"]> = new Set([
	"completed",
	"failed",
	"rolled_back",
	"paused",
]);

/**
 * A deployment request: the deploy response's baseline, and when this tab
 * sent it, which also tells two requests with the same baseline apart.
 */
export interface RequestedDeployment {
	afterDeployment: string;
	sentAt: number;
}

/** An answer that polling again cannot change, such as a changed mapping. */
function isPermanent(error: Error | null) {
	return error instanceof DokployRequestError && error.status < 500;
}

/**
 * Follows a requested deployment until Swarm has finished with it. When the
 * workload runs in a new container, switches to it, so what the panel
 * compares against is what now runs.
 */
export function useDokployRollout({
	host,
	resource,
	service,
	deployment,
	containerId,
	onContainerIdChange,
}: {
	host: string;
	resource: DokployResource;
	service: string;
	deployment: RequestedDeployment | undefined;
	containerId: string;
	onContainerIdChange?: (newContainerId: string) => void;
}) {
	const queryClient = useQueryClient();
	const sentAt = deployment?.sentAt;
	// Timed from when this tab sent the request, whatever the server's clock says.
	const [expiredFor, setExpiredFor] = useState<number>();
	useEffect(() => {
		if (!deployment) return;
		const timer = setTimeout(
			() => setExpiredFor(deployment.sentAt),
			deployment.sentAt + WATCH_MS - Date.now(),
		);
		return () => clearTimeout(timer);
	}, [deployment]);
	const expired = sentAt !== undefined && expiredFor === sentAt;
	const query = useQuery({
		queryKey: ["dokploy-rollout", host, resource.type, resource.id, sentAt],
		queryFn: () =>
			getDokployRollout(
				host,
				resource,
				deployment?.afterDeployment ?? "",
				service,
			),
		enabled: deployment !== undefined,
		retry: false,
		refetchOnWindowFocus: false,
		// Polling carries on through server and network errors: LogDeck itself
		// may be the workload being redeployed.
		refetchInterval: (current) => {
			const phase = current.state.data?.phase;
			const overdue =
				deployment !== undefined && Date.now() - deployment.sentAt > WATCH_MS;
			const ended =
				(phase && FINISHED.has(phase)) || isPermanent(current.state.error);
			return ended || overdue ? false : POLL_MS;
		},
	});

	const phase = query.data?.phase;
	const next = query.data?.container_id;
	// Each deployment announces its completion once.
	const announced = useRef<number>(undefined);
	useEffect(() => {
		if (phase !== "completed" || announced.current === sentAt) return;
		announced.current = sentAt;
		void queryClient.invalidateQueries({ queryKey: ["containers"] });
		toast.success("Deployed", {
			description: "The running container has the saved configuration.",
		});
		if (next && next !== containerId) onContainerIdChange?.(next);
	}, [phase, next, sentAt, containerId, onContainerIdChange, queryClient]);

	const permanent = isPermanent(query.error);
	const following =
		deployment !== undefined && !(phase && FINISHED.has(phase)) && !permanent;
	return {
		rollout: query.data,
		error: query.error,
		permanent,
		active: following && !expired,
		stoppedWatching: following && expired,
	};
}

function withMessage(text: string, message: string | undefined) {
	return message ? `${text}: ${message}.` : `${text}.`;
}

/** One line on where the deployment is, or how it ended. */
export function DokployRolloutStatus({
	rollout,
	error,
	permanent,
	stoppedWatching,
	isApplication,
}: {
	rollout: DokployRollout | undefined;
	error: Error | null;
	permanent: boolean;
	stoppedWatching: boolean;
	isApplication: boolean;
}) {
	if (permanent && error) {
		return (
			<PanelError>
				Stopped following the deployment: {error.message}. Check it in Dokploy.
			</PanelError>
		);
	}
	if (stoppedWatching) {
		return (
			<PanelNote>
				Stopped watching after 30 minutes. Check the deployment in Dokploy.
			</PanelNote>
		);
	}
	const phase = rollout?.phase;
	if (phase === "completed") {
		return (
			<PanelNote>
				Deployed. The running container has the saved configuration.
			</PanelNote>
		);
	}
	if (phase === "failed") {
		return (
			<PanelError>
				{withMessage("The Dokploy deployment failed", rollout?.message)} The
				running container still has the previous configuration.
			</PanelError>
		);
	}
	if (phase === "rolled_back") {
		return (
			<PanelError>
				{withMessage("Swarm rolled the update back", rollout?.message)} The
				running container still has the previous configuration. Check the new
				task's logs in Dokploy.
			</PanelError>
		);
	}
	if (phase === "paused") {
		return (
			<PanelError>
				{withMessage("Swarm paused the update", rollout?.message)} Check the
				service in Dokploy.
			</PanelError>
		);
	}
	const progress = {
		queued:
			"Waiting in Dokploy's deployment queue. Dokploy runs one deployment at a time.",
		building: isApplication
			? "Dokploy is building and deploying…"
			: "Dokploy is deploying…",
		updating:
			"Swarm is starting the new task and waiting for its health check…",
		rolling_back: withMessage(
			"Swarm is rolling the update back",
			rollout?.message,
		),
	} as const;
	return (
		<div className="space-y-1">
			<output className="flex items-center gap-2 text-base text-muted-foreground sm:text-sm">
				<Spinner className="size-4 shrink-0" />
				{phase ? progress[phase] : "Requesting deployment status…"}
			</output>
			{error && (
				<PanelNote>
					Could not read the deployment status ({error.message}). Retrying.
				</PanelNote>
			)}
		</div>
	);
}
