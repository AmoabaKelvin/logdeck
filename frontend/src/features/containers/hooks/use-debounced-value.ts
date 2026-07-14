import { useEffect, useState } from "react";

// Trailing-edge debounce: returns the latest value once it has been stable for
// `delayMs`. Used to keep per-keystroke changes out of server query keys.
export function useDebouncedValue<T>(value: T, delayMs: number): T {
	const [debounced, setDebounced] = useState(value);

	useEffect(() => {
		const timer = setTimeout(() => setDebounced(value), delayMs);
		return () => clearTimeout(timer);
	}, [value, delayMs]);

	return debounced;
}
