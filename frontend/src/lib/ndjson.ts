/**
 * Iterates over an NDJSON response stream, yielding one parsed value per line.
 */
export async function* iterateNDJSONStream<T>(
	stream: ReadableStream<Uint8Array>,
	signal?: AbortSignal,
): AsyncGenerator<T, void, unknown> {
	const reader = stream.getReader();
	const decoder = new TextDecoder();
	let buffer = "";

	try {
		while (true) {
			if (signal?.aborted) {
				reader.cancel().catch(() => {});
				break;
			}

			const { done, value } = await reader.read();
			if (done) break;

			buffer += decoder.decode(value, { stream: true });

			const lines = buffer.split("\n");
			buffer = lines.pop() ?? "";

			for (const line of lines) {
				if (line.trim()) {
					try {
						yield JSON.parse(line) as T;
					} catch (error) {
						console.error("Failed to parse NDJSON line:", line, error);
					}
				}
			}
		}

		if (buffer.trim()) {
			try {
				yield JSON.parse(buffer) as T;
			} catch (error) {
				console.error("Failed to parse final NDJSON line:", buffer, error);
			}
		}
	} finally {
		reader.releaseLock();
	}
}
