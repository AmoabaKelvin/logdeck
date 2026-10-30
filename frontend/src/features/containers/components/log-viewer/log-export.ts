import type { LogSource } from "@/features/containers/api/get-aggregated-logs";
import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";

export function formatLogEntryLine(entry: LogEntry): string {
	const timestamp = entry.timestamp
		? new Date(entry.timestamp).toISOString()
		: "";
	const level = entry.level || "UNKNOWN";
	const message = entry.message || entry.raw || "";
	return `[${timestamp}] [${level}] ${message}`;
}

export function formatMissingSource(source: LogSource): string {
	const name = source.containerName || source.containerId;
	return `${name} on ${source.host}: ${source.error ?? "logs could not be read"}`;
}

// Sources whose logs could not be read travel with the export, so a file
// with gaps never passes for a complete one.
export function downloadLogs(
	entries: LogEntry[],
	containerName: string | undefined,
	format: "json" | "txt",
	missingSources: LogSource[] = [],
): void {
	const cleanContainerName = (containerName || "container")
		.replace(/^\//, "")
		.replace(/[/\\:*?"<>|]/g, "-");
	const timestamp = new Date()
		.toISOString()
		.replace(/:/g, "-")
		.replace(/\..+/, "");
	const filename = `${cleanContainerName}-logs-${timestamp}.${format}`;

	let content: string;
	let mimeType: string;
	const incomplete = missingSources.length > 0;
	if (format === "json") {
		// A complete export stays a plain array, as before.
		content = JSON.stringify(
			incomplete ? { incomplete, missingSources, logs: entries } : entries,
			null,
			2,
		);
		mimeType = "application/json";
	} else {
		const header = incomplete
			? [
					`# Incomplete: ${missingSources.length} source${missingSources.length === 1 ? "" : "s"} missing`,
					...missingSources.map((source) => `# ${formatMissingSource(source)}`),
				]
			: [];
		content = [...header, ...entries.map(formatLogEntryLine)].join("\n");
		mimeType = "text/plain";
	}

	const blob = new Blob([content], { type: mimeType });
	const url = URL.createObjectURL(blob);
	const a = document.createElement("a");
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	document.body.removeChild(a);
	URL.revokeObjectURL(url);
}
