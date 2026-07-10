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

interface EnvUpdateConfirmDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	isCoolifyManaged: boolean;
	onConfirm: () => void;
}

export function EnvUpdateConfirmDialog({
	open,
	onOpenChange,
	isCoolifyManaged,
	onConfirm,
}: EnvUpdateConfirmDialogProps) {
	return (
		<AlertDialog open={open} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Update environment variables?</AlertDialogTitle>
					<AlertDialogDescription>
						Changing environment variables requires recreating the container.
						This will cause a brief downtime.
						{isCoolifyManaged
							? " Changes will also be synced to Coolify so they persist across redeployments. Are you sure you want to continue?"
							: " Are you sure you want to continue?"}
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction onClick={onConfirm}>
						Confirm & Update
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
