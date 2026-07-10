import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Spinner } from "@/components/ui/spinner";

import type { ConfirmableAction } from "../hooks/use-container-actions";
import { formatContainerName } from "./container-utils";

const CONFIRM_COPY: Record<
	ConfirmableAction["type"],
	{ title: string; description: string; confirmLabel: string }
> = {
	stop: {
		title: "Stop container?",
		description: "Stopping a container will terminate its running processes.",
		confirmLabel: "Stop Container",
	},
	remove: {
		title: "Remove container?",
		description:
			"Removing a container will permanently delete it and its resources. This action cannot be undone.",
		confirmLabel: "Remove Container",
	},
};

interface ConfirmActionDialogProps {
	action: ConfirmableAction | null;
	isPending: boolean;
	onConfirm: () => void;
	onOpenChange: (open: boolean) => void;
}

export function ConfirmActionDialog({
	action,
	isPending,
	onConfirm,
	onOpenChange,
}: ConfirmActionDialogProps) {
	// action goes null while the dialog animates closed; fall back to blanks.
	const copy = action ? CONFIRM_COPY[action.type] : null;

	return (
		<AlertDialog open={Boolean(action)} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>{copy?.title ?? ""}</AlertDialogTitle>
					<AlertDialogDescription>
						{copy?.description ?? ""}
					</AlertDialogDescription>
				</AlertDialogHeader>
				{action && (
					<div className="space-y-2">
						<div className="text-sm font-medium text-muted-foreground">
							Container Details
						</div>
						<div className="rounded-md border bg-muted/30 p-3 space-y-2">
							<div className="flex items-start justify-between gap-4">
								<span className="text-xs text-muted-foreground">Name</span>
								<span className="text-sm font-medium text-right">
									{formatContainerName(action.container.names)}
								</span>
							</div>
							<div className="flex items-start justify-between gap-4">
								<span className="text-xs text-muted-foreground">Image</span>
								<span className="text-sm font-mono text-right break-all">
									{action.container.image}
								</span>
							</div>
							<div className="flex items-start justify-between gap-4">
								<span className="text-xs text-muted-foreground">ID</span>
								<span className="text-sm font-mono text-right break-all">
									{action.container.id.slice(0, 12)}
								</span>
							</div>
						</div>
					</div>
				)}
				<AlertDialogFooter>
					<AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className={`flex items-center gap-2 ${
							action?.type === "remove"
								? "bg-destructive text-white hover:bg-destructive/90"
								: ""
						}`}
						onClick={onConfirm}
						disabled={isPending}
					>
						{isPending && <Spinner className="size-4" />}
						{copy?.confirmLabel ?? "Confirm"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
