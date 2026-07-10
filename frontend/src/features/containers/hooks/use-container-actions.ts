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
 */
export function useContainerActions(refetch: () => Promise<unknown>) {
	const [pendingAction, setPendingAction] = useState<{
		id: string;
		type: ContainerActionType;
	} | null>(null);
	const [pendingComposeAction, setPendingComposeAction] = useState<{
		project: string;
		type: ComposeAction;
	} | null>(null);
	const [confirmAction, setConfirmAction] = useState<ConfirmableAction | null>(
		null,
	);

	const executeAction = async (
		actionType: ContainerActionType,
		container: ContainerInfo,
	) => {
		setPendingAction({ id: container.id, type: actionType });
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
			setPendingAction(null);
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
		setPendingComposeAction({ project: group.project, type: actionType });
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
			setPendingComposeAction(null);
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
		pendingAction?.id === confirmAction.container.id &&
		pendingAction?.type === confirmAction.type;

	return {
		pendingAction,
		pendingComposeAction,
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
