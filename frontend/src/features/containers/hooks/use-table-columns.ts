import { useCallback, useState } from "react";

export const TOGGLEABLE_COLUMNS = [
	{ id: "host", label: "Host" },
	{ id: "status", label: "Status" },
	{ id: "uptime", label: "Uptime" },
	{ id: "created", label: "Created" },
	{ id: "ports", label: "Ports" },
	{ id: "usage", label: "Usage" },
] as const;

export type ColumnId = (typeof TOGGLEABLE_COLUMNS)[number]["id"];

const STORAGE_KEY = "logdeck.containerColumns.hidden";
const DEFAULT_HIDDEN: ColumnId[] = ["host"];

function readHidden(): ColumnId[] {
	try {
		const stored: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "");
		if (!Array.isArray(stored)) return DEFAULT_HIDDEN;
		return TOGGLEABLE_COLUMNS.map((column) => column.id).filter((id) =>
			stored.includes(id),
		);
	} catch {
		return DEFAULT_HIDDEN;
	}
}

export function useTableColumns() {
	const [hiddenColumns, setHiddenColumns] = useState<ReadonlySet<ColumnId>>(
		() => new Set(readHidden()),
	);

	const toggleColumn = useCallback((id: ColumnId) => {
		setHiddenColumns((current) => {
			const next = new Set(current);
			if (!next.delete(id)) next.add(id);
			localStorage.setItem(STORAGE_KEY, JSON.stringify([...next]));
			return next;
		});
	}, []);

	return { hiddenColumns, toggleColumn };
}
