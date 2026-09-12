import { useCallback, useEffect, useMemo, useRef, useState } from "react";

export interface EnvChangeCounts {
	added: number;
	changed: number;
	removed: number;
}

export interface EnvDraft {
	/** Every variable as it currently reads, including ones marked for removal. */
	effective: Record<string, string>;
	counts: EnvChangeCounts;
	isDirty: boolean;
	isRemoved: (key: string) => boolean;
	isChanged: (key: string) => boolean;
	/** Keys added moments ago, for the arrival highlight. */
	recentlyAdded: ReadonlySet<string>;
	setValue: (key: string, value: string) => void;
	remove: (key: string) => void;
	restore: (key: string) => void;
	/** Adds or overwrites, and un-removes anything the import brings back. */
	add: (entries: Record<string, string>) => void;
	discard: () => void;
	/** What to send: the original with the edits applied and removals dropped. */
	payload: () => Record<string, string>;
}

const HIGHLIGHT_MS = 1300;

/**
 * The pending change set for a container's environment, kept as edits against
 * the fetched original rather than a copy of it — so "what changed" is read
 * off the model instead of tracked alongside it.
 */
export function useEnvDraft(
	original: Record<string, string> | undefined,
): EnvDraft {
	const [edits, setEdits] = useState<Record<string, string>>({});
	const [removed, setRemoved] = useState<ReadonlySet<string>>(new Set());
	const [recentlyAdded, setRecentlyAdded] = useState<ReadonlySet<string>>(
		new Set(),
	);
	const highlightTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

	useEffect(() => () => clearTimeout(highlightTimer.current), []);

	const base = useMemo(() => original ?? {}, [original]);
	const effective = useMemo(() => ({ ...base, ...edits }), [base, edits]);

	const counts = useMemo(() => {
		const keys = Object.keys(edits);
		return {
			added: keys.filter((key) => !(key in base)).length,
			changed: keys.filter((key) => key in base && base[key] !== edits[key])
				.length,
			removed: removed.size,
		};
	}, [base, edits, removed]);

	const add = useCallback((entries: Record<string, string>) => {
		const keys = Object.keys(entries);
		setEdits((prev) => ({ ...prev, ...entries }));
		setRemoved((prev) => {
			const next = new Set(prev);
			for (const key of keys) next.delete(key);
			return next;
		});
		setRecentlyAdded(new Set(keys));
		clearTimeout(highlightTimer.current);
		highlightTimer.current = setTimeout(
			() => setRecentlyAdded(new Set()),
			HIGHLIGHT_MS,
		);
	}, []);

	const remove = useCallback(
		(key: string) => {
			// A key that only exists as a pending addition has nothing to remove
			// from the container — dropping the edit is the whole undo, and
			// counting it as a removal would report a change that isn't one.
			if (key in base) {
				setRemoved((prev) => new Set(prev).add(key));
				return;
			}
			setEdits((prev) => {
				const next = { ...prev };
				delete next[key];
				return next;
			});
		},
		[base],
	);

	const restore = useCallback((key: string) => {
		setRemoved((prev) => {
			const next = new Set(prev);
			next.delete(key);
			return next;
		});
	}, []);

	const setValue = useCallback((key: string, value: string) => {
		setEdits((prev) => ({ ...prev, [key]: value }));
	}, []);

	const discard = useCallback(() => {
		setEdits({});
		setRemoved(new Set());
		setRecentlyAdded(new Set());
		clearTimeout(highlightTimer.current);
	}, []);

	const payload = useCallback(() => {
		const next = { ...base, ...edits };
		for (const key of removed) delete next[key];
		return next;
	}, [base, edits, removed]);

	return {
		effective,
		counts,
		isDirty: counts.added + counts.changed + counts.removed > 0,
		isRemoved: useCallback((key: string) => removed.has(key), [removed]),
		isChanged: useCallback((key: string) => key in edits, [edits]),
		recentlyAdded,
		setValue,
		remove,
		restore,
		add,
		discard,
		payload,
	};
}
