import type { LogEntry } from "@/components/logdeck-demo/api/get-container-logs-parsed";

export function formatLogEntryLine(entry: LogEntry): string {
	const timestamp = entry.timestamp
		? new Date(entry.timestamp).toISOString()
		: "";
	const level = entry.level || "UNKNOWN";
	const message = entry.message || entry.raw || "";
	return `[${timestamp}] [${level}] ${message}`;
}

export function downloadLogs(
	entries: LogEntry[],
	containerName: string | undefined,
	format: "json" | "txt",
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
	if (format === "json") {
		content = JSON.stringify(entries, null, 2);
		mimeType = "application/json";
	} else {
		content = entries.map(formatLogEntryLine).join("\n");
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
