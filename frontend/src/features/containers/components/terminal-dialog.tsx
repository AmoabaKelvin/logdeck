import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Terminal } from "./terminal";

interface TerminalDialogProps {
	containerId: string;
	containerName: string;
	host: string;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

/**
 * A shell wants height. Squeezed into a panel above the log list it was a
 * letterbox, so it gets its own window instead.
 */
export function TerminalDialog({
	containerId,
	containerName,
	host,
	open,
	onOpenChange,
}: TerminalDialogProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="flex h-[85dvh] max-h-[85dvh] w-full flex-col gap-4 sm:max-w-5xl">
				<DialogHeader className="shrink-0 text-left">
					<DialogTitle className="truncate">{containerName}</DialogTitle>
					<DialogDescription>
						An interactive shell inside the container. Closing this window ends
						the session.
					</DialogDescription>
				</DialogHeader>
				<div className="min-h-0 flex-1 overflow-hidden rounded-lg border border-border/70">
					{/* Remounting per open gives every session a fresh connection. */}
					{open && <Terminal containerId={containerId} host={host} />}
				</div>
			</DialogContent>
		</Dialog>
	);
}
