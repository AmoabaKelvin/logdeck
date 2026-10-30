import { PanelNote } from "./container-panel-ui";

export interface EnvChanges {
	added: string[];
	changed: string[];
	removed: string[];
}

/** Keys added, changed and removed between two parsed environments. */
export function diffEnv(
	before: Record<string, string>,
	after: Record<string, string>,
): EnvChanges {
	const keys = Object.keys(after);
	return {
		added: keys.filter((key) => !(key in before)),
		changed: keys.filter((key) => key in before && before[key] !== after[key]),
		removed: Object.keys(before).filter((key) => !(key in after)),
	};
}

/** "Adds A. Changes B, C. Removes D." for a save confirmation. */
export function describeEnvChanges({ added, changed, removed }: EnvChanges) {
	return [
		added.length > 0 && `Adds ${added.join(", ")}.`,
		changed.length > 0 && `Changes ${changed.join(", ")}.`,
		removed.length > 0 && `Removes ${removed.join(", ")}.`,
	]
		.filter(Boolean)
		.join(" ");
}

/**
 * Saved values the running container does not have yet, and running
 * variables the saved configuration does not mention. A null saved value is
 * one LogDeck cannot compare: hidden, or a reference the platform resolves.
 */
export function compareWithRunning(
	saved: Record<string, string | null>,
	running: string[],
) {
	const live = new Map(
		running.map((entry) => {
			const index = entry.indexOf("=");
			return index === -1
				? [entry, ""]
				: [entry.slice(0, index), entry.slice(index + 1)];
		}),
	);
	const entries = Object.entries(saved);
	return {
		pending: entries
			.filter(([key, value]) => value !== null && live.get(key) !== value)
			.map(([key]) => key),
		unknown: entries.filter(([, value]) => value === null).length,
		runningOnly: [...live.keys()].filter((key) => !(key in saved)).sort(),
	};
}

export type RunningComparison = ReturnType<typeof compareWithRunning>;

/**
 * How the saved configuration differs from the running container, so a save
 * is not mistaken for a deployment. Only for applications: Compose and service
 * variables reach a container only where the Compose file references them.
 */
export function RunningEnvSummary({
	comparison: { pending, unknown, runningOnly },
	platform,
	isGenerated,
}: {
	comparison: RunningComparison;
	platform: string;
	isGenerated?: (key: string) => boolean;
}) {
	const generated = isGenerated ? runningOnly.filter(isGenerated) : [];
	const other = runningOnly.filter((key) => !generated.includes(key));

	return (
		<div className="space-y-1">
			{pending.length > 0 ? (
				<p className="text-base text-amber-700 sm:text-sm dark:text-amber-400">
					Not deployed yet: the running container has a different value or none
					for {pending.join(", ")}. Deploy to apply the saved configuration.
				</p>
			) : (
				<PanelNote>
					The running container has every saved value LogDeck can read.
				</PanelNote>
			)}
			{unknown > 0 && (
				<PanelNote>
					{unknown} hidden or referenced value{unknown === 1 ? "" : "s"} cannot
					be compared.
				</PanelNote>
			)}
			{generated.length > 0 && (
				<PanelNote>
					Set by {platform} at deploy time: {generated.join(", ")}.
				</PanelNote>
			)}
			{other.length > 0 && (
				<details className="text-base text-muted-foreground sm:text-sm">
					<summary className="cursor-pointer">
						{other.length} running variable
						{other.length === 1 ? " is" : "s are"} not in the saved
						configuration
					</summary>
					<p className="mt-1">
						They come from the image or {platform}, or were removed and are
						waiting for a deployment:{" "}
						<span className="break-all font-mono">{other.join(", ")}</span>
					</p>
				</details>
			)}
		</div>
	);
}
