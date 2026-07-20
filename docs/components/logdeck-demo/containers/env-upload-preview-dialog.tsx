import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/logdeck-demo/ui/alert-dialog";

interface EnvUploadPreviewDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	parsedEnv: Record<string, string>;
	currentEnv: Record<string, string>;
	onConfirm: () => void;
	onCancel: () => void;
}

export function EnvUploadPreviewDialog({
	open,
	onOpenChange,
	parsedEnv,
	currentEnv,
	onConfirm,
	onCancel,
}: EnvUploadPreviewDialogProps) {
	const parsedKeys = Object.keys(parsedEnv);
	const newKeys = parsedKeys.filter((key) => !currentEnv[key]);
	const updatedKeys = parsedKeys.filter(
		(key) => currentEnv[key] && currentEnv[key] !== parsedEnv[key],
	);
	const unchangedCount = parsedKeys.filter(
		(key) => currentEnv[key] && currentEnv[key] === parsedEnv[key],
	).length;

	return (
		<AlertDialog open={open} onOpenChange={onOpenChange}>
			<AlertDialogContent className="max-w-3xl max-h-[80vh] overflow-hidden flex flex-col">
				<AlertDialogHeader className="shrink-0">
					<AlertDialogTitle>Preview .env File Import</AlertDialogTitle>
					<AlertDialogDescription>
						Review the variables that will be imported from the .env file.
					</AlertDialogDescription>
				</AlertDialogHeader>

				<div className="space-y-4 py-4 overflow-y-auto min-h-0">
					<div className="flex gap-4 text-sm">
						<div className="flex items-center gap-2">
							<span className="font-medium">Total:</span>
							<span className="px-2 py-0.5 rounded bg-muted">
								{parsedKeys.length}
							</span>
						</div>
						<div className="flex items-center gap-2">
							<span className="font-medium">New:</span>
							<span className="px-2 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
								{newKeys.length}
							</span>
						</div>
						<div className="flex items-center gap-2">
							<span className="font-medium">Updated:</span>
							<span className="px-2 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400">
								{updatedKeys.length}
							</span>
						</div>
					</div>

					{newKeys.length > 0 && (
						<div className="space-y-2">
							<h4 className="text-sm font-semibold text-green-700 dark:text-green-400">
								New Variables
							</h4>
							<div className="space-y-2 max-h-48 overflow-y-auto">
								{newKeys.map((key) => (
									<div
										key={key}
										className="p-2 rounded border border-green-200 dark:border-green-900/30 bg-green-50 dark:bg-green-900/10"
									>
										<div className="font-mono text-xs break-all overflow-hidden">
											<span className="font-semibold break-words">{key}</span>
											<span className="text-muted-foreground"> = </span>
											<span className="break-words">{parsedEnv[key]}</span>
										</div>
									</div>
								))}
							</div>
						</div>
					)}

					{updatedKeys.length > 0 && (
						<div className="space-y-2">
							<h4 className="text-sm font-semibold text-blue-700 dark:text-blue-400">
								Updated Variables
							</h4>
							<div className="space-y-3 max-h-48 overflow-y-auto">
								{updatedKeys.map((key) => (
									<div
										key={key}
										className="p-2 rounded border border-blue-200 dark:border-blue-900/30 bg-blue-50 dark:bg-blue-900/10"
									>
										<div className="font-mono text-xs space-y-1 overflow-hidden">
											<div className="font-semibold break-words">{key}</div>
											<div className="flex flex-col gap-1">
												<div className="flex items-start gap-2">
													<span className="text-muted-foreground shrink-0">
														Old:
													</span>
													<span className="text-red-600 dark:text-red-400 line-through break-all">
														{currentEnv[key]}
													</span>
												</div>
												<div className="flex items-start gap-2">
													<span className="text-muted-foreground shrink-0">
														New:
													</span>
													<span className="text-green-600 dark:text-green-400 break-all">
														{parsedEnv[key]}
													</span>
												</div>
											</div>
										</div>
									</div>
								))}
							</div>
						</div>
					)}

					{unchangedCount > 0 && (
						<div className="space-y-2">
							<h4 className="text-sm font-semibold text-muted-foreground">
								Unchanged Variables ({unchangedCount})
							</h4>
						</div>
					)}
				</div>

				<AlertDialogFooter className="shrink-0">
					<AlertDialogCancel onClick={onCancel}>Cancel</AlertDialogCancel>
					<AlertDialogAction onClick={onConfirm}>
						Import Variables
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
