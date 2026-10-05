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

export interface EnvConfirmCopy {
	title: string;
	description: string;
	confirmLabel: string;
}

/** Confirms one step of a platform-managed environment edit. */
export function EnvConfirmDialog({
	copy,
	onConfirm,
	onOpenChange,
}: {
	// Goes null while the dialog animates closed; fall back to blanks.
	copy: EnvConfirmCopy | null;
	onConfirm: () => void;
	onOpenChange: (open: boolean) => void;
}) {
	return (
		<AlertDialog open={Boolean(copy)} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>{copy?.title ?? ""}</AlertDialogTitle>
					<AlertDialogDescription>
						{copy?.description ?? ""}
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction onClick={onConfirm}>
						{copy?.confirmLabel ?? "Confirm"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
