import { useCallback, useState } from "react";
import { readStorage, writeStorage } from "@/lib/safe-storage";

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
	const stored = readStorage(STORAGE_KEY)?.split("\n");
	if (!stored) return DEFAULT_HIDDEN;
	return TOGGLEABLE_COLUMNS.map((column) => column.id).filter((id) =>
		stored.includes(id),
	);
}

export function useTableColumns() {
	const [hiddenColumns, setHiddenColumns] = useState<ReadonlySet<ColumnId>>(
		() => new Set(readHidden()),
	);

	const toggleColumn = useCallback((id: ColumnId) => {
		setHiddenColumns((current) => {
			const next = new Set(current);
			if (!next.delete(id)) next.add(id);
			writeStorage(STORAGE_KEY, [...next].join("\n"));
			return next;
		});
	}, []);

	return { hiddenColumns, toggleColumn };
}
