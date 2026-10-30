import type {
	CollectionHealthReport,
	CollectionObservation,
	ContainerCollectionHealth,
} from "../api/get-collection-health";

export type CollectionTone = "ok" | "pending" | "bad" | "muted";

export interface CollectionLabel {
	label: string;
	tone: CollectionTone;
	detail?: string;
}

/**
 * The background tail. A tail that attached and has seen nothing is still
 * "Collecting": a quiet container is not a broken one.
 */
export function describeLive(
	o: CollectionObservation | undefined,
): CollectionLabel {
	switch (o?.state) {
		case undefined:
			return {
				label: "Not tailed in the background",
				tone: "muted",
				detail:
					"Only stored logs and log alert rules tail containers in the background.",
			};
		case "ok":
			return { label: "Collecting", tone: "ok" };
		case "pending":
			return { label: "Connecting", tone: "pending" };
		case "unsupported":
			return {
				label: "Logs can't be read",
				tone: "muted",
				detail: o?.error,
			};
		default:
			return { label: "Can't collect", tone: "bad", detail: o?.error };
	}
}

/** The log store's engine read, which fills in what the live tail missed. */
export function describeStored(
	o: CollectionObservation | undefined,
): CollectionLabel {
	switch (o?.state) {
		case undefined:
			return { label: "Not read yet", tone: "muted" };
		case "ok":
			return { label: "Up to date", tone: "ok" };
		case "pending":
			return { label: "Catching up", tone: "pending" };
		case "excluded":
			return {
				label: "Excluded from storage",
				tone: "muted",
				detail: o?.error,
			};
		default:
			return { label: "Can't read history", tone: "bad", detail: o?.error };
	}
}

export function findContainerHealth(
	report: CollectionHealthReport | undefined,
	host: string,
	id: string,
): ContainerCollectionHealth | undefined {
	return report?.hosts
		.find((h) => h.host === host)
		?.containers.find((c) => c.id === id);
}

/**
 * One line per failure, the same grouping the server alerts on: a host that
 * cannot be listed is one problem, however many containers it has.
 */
export function collectionProblems(
	report: CollectionHealthReport | undefined,
	{ includeHosts = true }: { includeHosts?: boolean } = {},
): string[] {
	if (!report) return [];
	const problems: string[] = [];
	for (const host of report.hosts) {
		if (host.state === "failing") {
			if (includeHosts)
				problems.push(`${host.host}: ${host.error ?? "unreachable"}`);
			continue;
		}
		const failing = host.containers.filter(
			(c) => c.live?.state === "failing" || c.stored?.state === "failing",
		);
		if (failing.length > 0) {
			const names = failing.map((c) => c.name || c.id.slice(0, 12));
			problems.push(
				`${host.host}: can't collect logs from ${names.join(", ")}`,
			);
		}
	}
	const storage = report.storage;
	if (storage?.state === "failing") {
		problems.push(`Log storage writes are failing: ${storage.error ?? ""}`);
	}
	if (storage?.lowSpace) {
		problems.push("Log storage is running out of disk space");
	}
	return problems;
}
