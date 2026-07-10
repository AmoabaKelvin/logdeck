import { useState } from "react";
import { toast } from "sonner";

import type { ComposeAction } from "../api/compose-actions";
import { performComposeAction } from "../api/compose-actions";
import {
	removeContainer,
	restartContainer,
	startContainer,
	stopContainer,
} from "../api/container-actions";
import type {
	ContainerActionType,
	GroupedContainers,
} from "../components/container-utils";
import type { ContainerInfo } from "../types";

export interface ConfirmableAction {
	type: Extract<ContainerActionType, "stop" | "remove">;
	container: ContainerInfo;
}

const containerActionFns: Record<
	ContainerActionType,
	(id: string, host: string) => Promise<string>
> = {
	start: startContainer,
	stop: stopContainer,
	restart: restartContainer,
	remove: removeContainer,
};

/**
 * Container and compose lifecycle actions with pending state and a
 * confirmation step for destructive actions (stop/remove).
 *
 * In-flight actions are tracked per key (container id or compose project)
 * so concurrent actions on different rows don't clobber each other's
 * pending state: an action settling on row B must not clear row A's
 * spinner while A is still running.
 */
export function useContainerActions(refetch: () => Promise<unknown>) {
	const [pendingActions, setPendingActions] = useState<
		ReadonlyMap<string, ContainerActionType>
	>(new Map());
	const [pendingComposeActions, setPendingComposeActions] = useState<
		ReadonlyMap<string, ComposeAction>
	>(new Map());
	const [confirmAction, setConfirmAction] = useState<ConfirmableAction | null>(
		null,
	);

	const beginPending = (id: string, type: ContainerActionType) => {
		setPendingActions((prev) => new Map(prev).set(id, type));
	};

	const endPending = (id: string) => {
		setPendingActions((prev) => {
			const next = new Map(prev);
			next.delete(id);
			return next;
		});
	};

	const beginComposePending = (project: string, type: ComposeAction) => {
		setPendingComposeActions((prev) => new Map(prev).set(project, type));
	};

	const endComposePending = (project: string) => {
		setPendingComposeActions((prev) => {
			const next = new Map(prev);
			next.delete(project);
			return next;
		});
	};

	const executeAction = async (
		actionType: ContainerActionType,
		container: ContainerInfo,
	) => {
		beginPending(container.id, actionType);
		try {
			const message = await containerActionFns[actionType](
				container.id,
				container.host,
			);
			if (message) {
				toast.success(message);
			}
			await refetch();
		} catch (error) {
			if (error instanceof Error) {
				toast.error(error.message);
			} else {
				toast.error("Unexpected error while performing container action.");
			}
		} finally {
			endPending(container.id);
		}
	};

	const executeComposeAction = async (
		actionType: ComposeAction,
		group: GroupedContainers,
	) => {
		// A UI group can theoretically span hosts; act once per distinct host.
		const groupHosts = Array.from(
			new Set(group.items.map((container) => container.host)),
		);
		beginComposePending(group.project, actionType);
		try {
			const results = await Promise.all(
				groupHosts.map((host) =>
					performComposeAction(group.project, actionType, host),
				),
			);
			const succeeded = results.reduce(
				(sum, result) => sum + result.succeeded,
				0,
			);
			const verb =
				actionType === "start"
					? "Started"
					: actionType === "stop"
						? "Stopped"
						: "Restarted";
			toast.success(
				`${verb} ${succeeded} container${succeeded === 1 ? "" : "s"} in ${group.project}`,
			);
			await refetch();
		} catch (error) {
			if (error instanceof Error) {
				toast.error(error.message);
			} else {
				toast.error("Unexpected error while performing compose action.");
			}
		} finally {
			endComposePending(group.project);
		}
	};

	const startContainerAction = (container: ContainerInfo) => {
		void executeAction("start", container);
	};

	const restartContainerAction = (container: ContainerInfo) => {
		void executeAction("restart", container);
	};

	const stopContainerAction = (container: ContainerInfo) => {
		setConfirmAction({ type: "stop", container });
	};

	const deleteContainerAction = (container: ContainerInfo) => {
		setConfirmAction({ type: "remove", container });
	};

	const composeAction = (action: ComposeAction, group: GroupedContainers) => {
		void executeComposeAction(action, group);
	};

	const confirmPendingAction = async () => {
		if (!confirmAction) return;
		const { type, container } = confirmAction;
		await executeAction(type, container);
		setConfirmAction(null);
	};

	const handleConfirmDialogOpenChange = (open: boolean) => {
		if (!open) {
			setConfirmAction(null);
		}
	};

	const isConfirmActionPending =
		!!confirmAction &&
		pendingActions.get(confirmAction.container.id) === confirmAction.type;

	return {
		pendingActions,
		pendingComposeActions,
		confirmAction,
		isConfirmActionPending,
		startContainerAction,
		stopContainerAction,
		restartContainerAction,
		deleteContainerAction,
		composeAction,
		confirmPendingAction,
		handleConfirmDialogOpenChange,
	};
}
