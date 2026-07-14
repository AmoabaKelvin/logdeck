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

export interface PurgeHistoryTarget {
	name: string;
	host: string;
	/** Removed containers no longer exist, so the store is their last trace. */
	removed: boolean;
}

interface PurgeHistoryDialogProps {
	target: PurgeHistoryTarget | null;
	isPending: boolean;
	onConfirm: () => void;
	onOpenChange: (open: boolean) => void;
}

export function PurgeHistoryDialog({
	target,
	isPending,
	onConfirm,
	onOpenChange,
}: PurgeHistoryDialogProps) {
	return (
		<AlertDialog open={Boolean(target)} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete stored logs?</AlertDialogTitle>
					<AlertDialogDescription>
						This permanently deletes the stored log history for "{target?.name}"
						on host {target?.host}. This action cannot be undone.
						{target?.removed
							? " The container itself is already gone, so its logs cannot be recovered afterwards."
							: " The container keeps running and will store new logs from now on."}
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="flex items-center gap-2 bg-destructive text-white hover:bg-destructive/90"
						onClick={onConfirm}
						disabled={isPending}
					>
						{isPending && <Spinner className="size-4" />}
						Delete Logs
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
