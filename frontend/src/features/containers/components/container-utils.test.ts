import { describe, expect, it } from "vitest";

import type { StoredContainer } from "../api/get-history";
import type { ContainerInfo } from "../types";
import {
	countContainerStates,
	getComposeProject,
	selectStackMembers,
	selectVisibleContainers,
	sortStoredContainersBySize,
	synthesizeRemovedContainers,
} from "./container-utils";

function live(overrides: Partial<ContainerInfo> = {}): ContainerInfo {
	return {
		id: "abc123",
		names: ["/api"],
		image: "nginx:latest",
		image_id: "sha256:1",
		command: "nginx",
		created: 1_700_000_000,
		state: "running",
		status: "Up 2 hours",
		host: "local",
		...overrides,
	};
}

function stored(overrides: Partial<StoredContainer> = {}): StoredContainer {
	return {
		host: "local",
		name: "worker",
		storedBytes: 1_200_000,
		oldestTs: "2026-07-01T00:00:00Z",
		newestTs: "2026-07-02T00:00:00Z",
		removed: true,
		excluded: false,
		...overrides,
	};
}

describe("synthesizeRemovedContainers", () => {
	it("turns removed entries that are not live into dashboard rows", () => {
		const [row] = synthesizeRemovedContainers(
			[stored({ image: "worker:1.2" })],
			[live()],
		);

		expect(row).toMatchObject({
			id: "worker",
			names: ["/worker"],
			image: "worker:1.2",
			state: "removed",
			host: "local",
			storedBytes: 1_200_000,
			created: Math.floor(Date.parse("2026-07-01T00:00:00Z") / 1000),
		});
	});

	it("skips entries whose (host, name) is still live", () => {
		const rows = synthesizeRemovedContainers(
			[stored({ name: "api" })],
			[live({ names: ["/api"], host: "local" })],
		);

		expect(rows).toEqual([]);
	});

	it("keeps an entry when the same name is live on a different host", () => {
		const rows = synthesizeRemovedContainers(
			[stored({ name: "api", host: "remote" })],
			[live({ names: ["/api"], host: "local" })],
		);

		expect(rows).toHaveLength(1);
		expect(rows[0].host).toBe("remote");
	});

	it("skips entries that are not removed or are excluded", () => {
		const rows = synthesizeRemovedContainers(
			[
				stored({ name: "still-running", removed: false }),
				stored({ name: "ignored", excluded: true }),
			],
			[],
		);

		expect(rows).toEqual([]);
	});

	it("applies the compose project under both docker and podman labels", () => {
		const [row] = synthesizeRemovedContainers(
			[stored({ composeProject: "demostack" })],
			[],
		);

		expect(row.labels).toEqual({
			"com.docker.compose.project": "demostack",
			"io.podman.compose.project": "demostack",
		});
		expect(getComposeProject(row.labels)).toBe("demostack");
	});
});

describe("selectVisibleContainers", () => {
	const liveContainers = [live()];
	const removedContainers = synthesizeRemovedContainers([stored()], []);

	it("hides removed containers from the default view", () => {
		expect(
			selectVisibleContainers(liveContainers, removedContainers, "all"),
		).toEqual(liveContainers);
	});

	it("hides removed containers from other state filters", () => {
		expect(
			selectVisibleContainers(liveContainers, removedContainers, "running"),
		).toEqual(liveContainers);
	});

	it("shows only removed containers when filtering by removed", () => {
		expect(
			selectVisibleContainers(liveContainers, removedContainers, "removed"),
		).toEqual(removedContainers);
	});
});

describe("countContainerStates", () => {
	it("counts live states and removed containers separately", () => {
		const counts = countContainerStates(
			[
				live({ id: "1", state: "running" }),
				live({ id: "2", state: "exited" }),
				live({ id: "3", state: "created" }),
			],
			synthesizeRemovedContainers(
				[stored({ name: "a" }), stored({ name: "b" })],
				[],
			),
			() => true,
		);

		expect(counts).toEqual({
			running: 1,
			exited: 1,
			paused: 0,
			restarting: 0,
			dead: 0,
			removed: 2,
			other: 1,
		});
	});

	it("respects the shared filter predicate", () => {
		const counts = countContainerStates(
			[live({ host: "remote" })],
			synthesizeRemovedContainers([stored({ host: "local" })], []),
			(container) => container.host === "local",
		);

		expect(counts.running).toBe(0);
		expect(counts.removed).toBe(1);
	});
});

describe("selectStackMembers", () => {
	it("includes removed containers that belong to the project", () => {
		const members = selectStackMembers(
			[
				live({
					id: "1",
					names: ["/web"],
					labels: { "com.docker.compose.project": "shop" },
				}),
				live({
					id: "2",
					names: ["/other"],
					labels: { "com.docker.compose.project": "blog" },
				}),
			],
			synthesizeRemovedContainers(
				[
					stored({ name: "worker", composeProject: "shop" }),
					stored({ name: "cache", composeProject: "blog" }),
				],
				[],
			),
			"shop",
		);

		expect(members.map((member) => member.names[0])).toEqual([
			"/web",
			"/worker",
		]);
	});

	it("matches removed members labelled by podman-compose", () => {
		const [removed] = synthesizeRemovedContainers(
			[stored({ name: "worker", composeProject: "shop" })],
			[],
		);
		removed.labels = { "io.podman.compose.project": "shop" };

		expect(selectStackMembers([], [removed], "shop")).toHaveLength(1);
	});

	it("returns nothing for a project with no members", () => {
		expect(selectStackMembers([live()], [], "shop")).toEqual([]);
	});
});

describe("sortStoredContainersBySize", () => {
	it("orders stored containers by size descending", () => {
		const sorted = sortStoredContainersBySize([
			stored({ name: "small", storedBytes: 10 }),
			stored({ name: "large", storedBytes: 900 }),
			stored({ name: "medium", storedBytes: 100 }),
		]);

		expect(sorted.map((entry) => entry.name)).toEqual([
			"large",
			"medium",
			"small",
		]);
	});

	it("breaks size ties by name and leaves the input untouched", () => {
		const input = [
			stored({ name: "b", storedBytes: 5 }),
			stored({ name: "a", storedBytes: 5 }),
		];
		const sorted = sortStoredContainersBySize(input);

		expect(sorted.map((entry) => entry.name)).toEqual(["a", "b"]);
		expect(input.map((entry) => entry.name)).toEqual(["b", "a"]);
	});
});
