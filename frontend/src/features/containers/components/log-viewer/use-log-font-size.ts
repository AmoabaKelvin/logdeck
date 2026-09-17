import { useCallback, useState } from "react";

// Log text sizes in px; 12 matches the rest of the app's mono text.
export const LOG_FONT_SIZES = [11, 12, 13, 14, 16, 18, 20];
export const DEFAULT_LOG_FONT_SIZE = 12;

const STORAGE_KEY = "logdeck.logFontSize";

// A reading preference, not part of a shareable view: it lives in localStorage
// so it follows the reader across containers, the sheet and the full page.
export function useLogFontSize() {
	const [fontSize, setFontSize] = useState(() => {
		const stored = Number(localStorage.getItem(STORAGE_KEY));
		return LOG_FONT_SIZES.includes(stored) ? stored : DEFAULT_LOG_FONT_SIZE;
	});

	const stepFontSize = useCallback((direction: 1 | -1) => {
		setFontSize((current) => {
			const next =
				LOG_FONT_SIZES[LOG_FONT_SIZES.indexOf(current) + direction] ?? current;
			localStorage.setItem(STORAGE_KEY, String(next));
			return next;
		});
	}, []);

	return { fontSize, stepFontSize };
}
