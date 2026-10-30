import { useState } from "react";

import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { ContainersPagination } from "@/features/containers/components/containers-pagination";
import { useDebouncedValue } from "@/features/containers/hooks/use-debounced-value";

import { useAuditLog } from "../hooks/use-audit-log";
import {
	AUDIT_RANGE_LABELS,
	AUDIT_RANGES,
	type AuditRange,
	describeActor,
	describeResult,
	describeTarget,
} from "./audit-utils";
import {
	ErrorNote,
	Field,
	Note,
	Outcome,
	SettingsSection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
} from "./settings-ui";

const RESULT_FILTERS = ["any", "success", "failure"] as const;
type ResultFilter = (typeof RESULT_FILTERS)[number];

const RESULT_LABELS = {
	any: "Any result",
	success: "Succeeded",
	failure: "Failed or refused",
} satisfies Record<ResultFilter, string>;

export function AuditLogSection() {
	const [actor, setActor] = useState("");
	const [action, setAction] = useState("");
	const [target, setTarget] = useState("");
	const [result, setResult] = useState<ResultFilter>("any");
	const [range, setRange] = useState<AuditRange>("all");
	const [page, setPage] = useState(1);
	const [pageSize, setPageSize] = useState(20);
	const text = {
		actor: useDebouncedValue(actor.trim(), 250),
		action: useDebouncedValue(action.trim(), 250),
		target: useDebouncedValue(target.trim(), 250),
	};
	const { data, isLoading, isFetching, isPlaceholderData, error } = useAuditLog(
		{
			...text,
			result: result === "any" ? undefined : result,
			range,
			limit: pageSize,
			offset: (page - 1) * pageSize,
		},
	);

	const entries = data?.entries ?? [];
	const total = data?.total ?? 0;
	const totalPages = Math.max(1, Math.ceil(total / pageSize));
	const startIndex = (page - 1) * pageSize;
	// A refresh can shrink the results and leave the page past the end. Only
	// fresh results count: a cached or placeholder total may be out of date.
	if (data && !isFetching && !isPlaceholderData && page > totalPages) {
		setPage(totalPages);
	}
	const filtered =
		text.actor !== "" ||
		text.action !== "" ||
		text.target !== "" ||
		result !== "any" ||
		range !== "all";

	function textFilter(
		id: string,
		label: string,
		placeholder: string,
		value: string,
		set: (value: string) => void,
	) {
		return (
			<Field id={id} label={label}>
				<Input
					id={id}
					type="search"
					placeholder={placeholder}
					value={value}
					onChange={(e) => {
						set(e.target.value);
						setPage(1);
					}}
				/>
			</Field>
		);
	}

	return (
		<SettingsSection
			title="Audit log"
			description="Changes made through LogDeck, from the UI, the CLI or MCP: who made it, what, where, when, and whether it worked. Reads are not recorded, and neither are request bodies, so env values, passwords and tokens never appear here. The newest 10,000 changes are kept."
		>
			<div className="space-y-4">
				<div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
					{textFilter("audit-actor", "Who", "admin, token:ci", actor, setActor)}
					{textFilter(
						"audit-action",
						"Action",
						"container.stop",
						action,
						setAction,
					)}
					{textFilter("audit-target", "Target", "web, shop", target, setTarget)}
					<Field id="audit-result" label="Result">
						<Select
							value={result}
							onValueChange={(value) => {
								const next = RESULT_FILTERS.find((r) => r === value);
								if (next) setResult(next);
								setPage(1);
							}}
						>
							<SelectTrigger id="audit-result" className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{RESULT_FILTERS.map((key) => (
									<SelectItem key={key} value={key}>
										{RESULT_LABELS[key]}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</Field>
					<Field id="audit-range" label="Time">
						<Select
							value={range}
							onValueChange={(value) => {
								const next = AUDIT_RANGES.find((r) => r === value);
								if (next) setRange(next);
								setPage(1);
							}}
						>
							<SelectTrigger id="audit-range" className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{AUDIT_RANGES.map((key) => (
									<SelectItem key={key} value={key}>
										{AUDIT_RANGE_LABELS[key]}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</Field>
				</div>

				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load the audit log: {error.message}</ErrorNote>
				)}
				{!isLoading && !error && total === 0 && (
					<Note>
						{filtered ? "No changes match." : "No changes recorded yet."}
					</Note>
				)}

				{entries.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Time</Th>
							<Th>Who</Th>
							<Th>Action</Th>
							<Th>Target</Th>
							<Th>Result</Th>
						</THead>
						<TBody>
							{entries.map((entry, i) => {
								const outcome = describeResult(entry);
								return (
									<tr key={`${entry.time}-${startIndex + i}`}>
										<Td className="text-muted-foreground tabular-nums">
											{new Date(entry.time).toLocaleString()}
										</Td>
										<Td title={entry.role && `Role: ${entry.role}`}>
											{describeActor(entry)}
										</Td>
										<Td
											className="font-mono"
											title={`${entry.method} ${entry.route}`}
										>
											{entry.action}
										</Td>
										<Td
											className="font-mono text-muted-foreground"
											title={entry.target}
										>
											{describeTarget(entry)}
										</Td>
										<Td>
											<Outcome
												ok={outcome.ok}
												title={`HTTP ${entry.status} in ${entry.durationMs} ms`}
											>
												{outcome.label}
											</Outcome>
										</Td>
									</tr>
								);
							})}
						</TBody>
					</SettingsTable>
				)}

				{total > pageSize && (
					<ContainersPagination
						totalItems={total}
						startIndex={startIndex + 1}
						endIndex={startIndex + entries.length}
						page={page}
						totalPages={totalPages}
						pageSize={pageSize}
						onPageChange={setPage}
						onPageSizeChange={(size) => {
							setPageSize(size);
							setPage(1);
						}}
					/>
				)}
			</div>
		</SettingsSection>
	);
}
