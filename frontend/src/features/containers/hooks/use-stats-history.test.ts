import { describe, expect, it } from "vitest";
import type { ContainerStats } from "../types";

import type { StatsHistoryMap } from "./use-stats-history";
import { appendSamples, MAX_SAMPLES } from "./use-stats-history";

function makeStat(id: string, cpu: number, memory: number): ContainerStats {
	return {
		id,
		host: "local",
		cpu_percent: cpu,
		memory_percent: memory,
		memory_used: 0,
		memory_limit: 0,
	};
}

describe("appendSamples", () => {
	it("appends a sample per container", () => {
		const history = appendSamples({}, [makeStat("a", 10, 20)]);
		expect(history.a).toEqual([{ cpu: 10, memoryPercent: 20 }]);

		const next = appendSamples(history, [makeStat("a", 30, 40)]);
		expect(next.a).toEqual([
			{ cpu: 10, memoryPercent: 20 },
			{ cpu: 30, memoryPercent: 40 },
		]);
	});

	it("caps buffers at MAX_SAMPLES, dropping the oldest", () => {
		let history: StatsHistoryMap = {};
		for (let i = 0; i < MAX_SAMPLES + 5; i++) {
			history = appendSamples(history, [makeStat("a", i, i)]);
		}
		const samples = history.a;
		expect(samples).toHaveLength(MAX_SAMPLES);
		expect(samples[0].cpu).toBe(5);
		expect(samples[samples.length - 1].cpu).toBe(MAX_SAMPLES + 4);
	});

	it("evicts containers no longer present", () => {
		const history = appendSamples({}, [
			makeStat("a", 1, 1),
			makeStat("b", 2, 2),
		]);
		const next = appendSamples(history, [makeStat("b", 3, 3)]);
		expect(next.a).toBeUndefined();
		expect(next.b).toHaveLength(2);
	});
});
