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
						// SAFETY: callers name T as the Go struct their NDJSON endpoint
						// encodes once per line; the server bundles this frontend.
						yield JSON.parse(line) as T;
					} catch (error) {
						console.error("Failed to parse NDJSON line:", line, error);
					}
				}
			}
		}

		if (buffer.trim()) {
			try {
				// SAFETY: same per-line contract as above; this is the unterminated tail.
				yield JSON.parse(buffer) as T;
			} catch (error) {
				console.error("Failed to parse final NDJSON line:", buffer, error);
			}
		}
	} finally {
		reader.releaseLock();
	}
}
