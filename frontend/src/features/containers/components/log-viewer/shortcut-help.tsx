import { Fragment } from "react";

import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Kbd, KbdGroup } from "@/components/ui/kbd";

interface ShortcutEntry {
	// Alternative keys for the same action, e.g. ["j", "↓"].
	keys: string[];
	description: string;
}

// Every shortcut handled by the log viewer (see the keydown handler in
// log-viewer.tsx and shift-click range selection in handleLogClick).
export const LOG_VIEWER_SHORTCUTS: ShortcutEntry[] = [
	{ keys: ["/"], description: "Focus search" },
	{ keys: ["j", "↓"], description: "Select next line" },
	{ keys: ["k", "↑"], description: "Select previous line" },
	{ keys: ["Shift+J", "Shift+↓"], description: "Extend selection down" },
	{ keys: ["Shift+K", "Shift+↑"], description: "Extend selection up" },
	{ keys: ["n"], description: "Next search match" },
	{ keys: ["N"], description: "Previous search match" },
	{ keys: ["p"], description: "Next pinned line" },
	{ keys: ["P"], description: "Previous pinned line" },
	{ keys: ["Shift+Click"], description: "Select a range of lines" },
	{ keys: ["?"], description: "Toggle this help" },
];

interface ShortcutHelpDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function ShortcutHelpDialog({
	open,
	onOpenChange,
}: ShortcutHelpDialogProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-md">
				<DialogHeader>
					<DialogTitle>Keyboard shortcuts</DialogTitle>
					<DialogDescription>
						Available while viewing logs (not while typing in a field).
					</DialogDescription>
				</DialogHeader>
				<div className="grid grid-cols-[auto_1fr] items-center gap-x-6 gap-y-2">
					{LOG_VIEWER_SHORTCUTS.map((shortcut) => (
						<Fragment key={shortcut.description}>
							<KbdGroup>
								{shortcut.keys.map((key, keyIndex) => (
									<Fragment key={key}>
										{keyIndex > 0 && (
											<span className="text-xs text-muted-foreground">or</span>
										)}
										<Kbd>{key}</Kbd>
									</Fragment>
								))}
							</KbdGroup>
							<span className="text-sm text-muted-foreground">
								{shortcut.description}
							</span>
						</Fragment>
					))}
				</div>
			</DialogContent>
		</Dialog>
	);
}
