import type { CollectionLabel, CollectionTone } from "./collection-health";

const TONE_CLASS = {
	ok: "text-emerald-700 dark:text-emerald-400",
	pending: "text-amber-700 dark:text-amber-400",
	bad: "font-medium text-rose-700 dark:text-rose-400",
	muted: "text-muted-foreground",
} satisfies Record<CollectionTone, string>;

/** One collection path's state: its label, then why, when there is a reason. */
export function CollectionStatus({
	status,
	since,
}: {
	status: CollectionLabel;
	since?: string;
}) {
	return (
		<span className="break-words">
			<span className={TONE_CLASS[status.tone]}>{status.label}</span>
			{status.tone === "bad" && since && (
				<span className="text-muted-foreground">
					{` since ${new Date(since).toLocaleString()}`}
				</span>
			)}
			{status.detail && (
				<span className="text-muted-foreground"> · {status.detail}</span>
			)}
		</span>
	);
}
