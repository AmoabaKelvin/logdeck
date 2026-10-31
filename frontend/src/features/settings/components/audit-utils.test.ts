import { describe, expect, it } from "vitest";

import type { AuditEntry } from "../api/get-audit-log";
import {
	auditSince,
	describeActor,
	describeResult,
	describeTarget,
} from "./audit-utils";

const base: AuditEntry = {
	time: "2026-01-01T00:00:00Z",
	actor: "admin",
	action: "container.stop",
	method: "POST",
	route: "/api/v1/containers/{id}/stop",
	status: 200,
	durationMs: 12,
};

describe("describeActor", () => {
	it("names users, API tokens, and requests made with auth off", () => {
		expect(describeActor(base)).toBe("admin");
		expect(describeActor({ ...base, actor: "token:ci" })).toBe("API token ci");
		expect(describeActor({ ...base, actor: "" })).toBe("Anyone (auth off)");
	});
});

describe("describeTarget", () => {
	it("shortens full container IDs and adds the host", () => {
		const id = "a".repeat(64);
		expect(describeTarget({ ...base, target: id, host: "prod" })).toBe(
			`${"a".repeat(12)}@prod`,
		);
		expect(describeTarget({ ...base, target: "web" })).toBe("web");
		expect(describeTarget(base)).toBe("—");
	});
});

describe("describeResult", () => {
	it("tells successes, opened terminals, refusals and failures apart", () => {
		expect(describeResult(base)).toEqual({ ok: true, label: "Succeeded" });
		expect(describeResult({ ...base, status: 101 })).toEqual({
			ok: true,
			label: "Opened",
		});
		expect(describeResult({ ...base, status: 403 })).toEqual({
			ok: false,
			label: "Refused (403)",
		});
		expect(describeResult({ ...base, status: 500 })).toEqual({
			ok: false,
			label: "Failed (500)",
		});
	});
});

describe("auditSince", () => {
	it("turns a range into its start time, and any time into none", () => {
		const now = Date.parse("2026-01-02T00:00:00Z");
		expect(auditSince("24h", now)).toBe("2026-01-01T00:00:00.000Z");
		expect(auditSince("all", now)).toBeUndefined();
	});
});
