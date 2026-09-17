import { useCallback, useState } from "react";

export const LOG_FONT_SIZES = [11, 12, 13, 14, 16, 18, 20];
export const DEFAULT_LOG_FONT_SIZE = 12;

const STORAGE_KEY = "logdeck.logFontSize";

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
