import { useCallback, useState } from "react";
import { readStorage, writeStorage } from "@/lib/safe-storage";

const STORAGE_KEY = "logdeck.collapsedGroups";

export function useCollapsedGroups() {
	const [collapsedGroups, setState] = useState<ReadonlySet<string>>(
		() => new Set(readStorage(STORAGE_KEY)?.split("\n").filter(Boolean)),
	);

	const setCollapsedGroups = useCallback((projects: Iterable<string>) => {
		const next = new Set(projects);
		writeStorage(STORAGE_KEY, [...next].join("\n"));
		setState(next);
	}, []);

	const toggleGroup = (project: string) => {
		const next = new Set(collapsedGroups);
		if (!next.delete(project)) next.add(project);
		setCollapsedGroups(next);
	};

	return { collapsedGroups, toggleGroup, setCollapsedGroups };
}
