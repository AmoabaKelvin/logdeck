import { Spinner } from "@/components/ui/spinner";
import type {
	HostCollectionHealth,
	StorageCollectionHealth,
} from "@/features/containers/api/get-collection-health";
import {
	collectionProblems,
	describeLive,
	describeStored,
} from "@/features/containers/components/collection-health";
import { CollectionStatus } from "@/features/containers/components/collection-status";
import { formatBytes } from "@/features/containers/components/container-utils";
import { useCollectionHealth } from "@/features/containers/hooks/use-collection-health";

import {
	ErrorNote,
	Note,
	Outcome,
	SettingsSection,
	SettingsSubsection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
} from "./settings-ui";

function formatTime(value: string | undefined): string {
	return value ? new Date(value).toLocaleString() : "never";
}

function HostBlock({
	host,
	persistence,
}: {
	host: HostCollectionHealth;
	persistence: boolean;
}) {
	// Collecting containers are the normal case; listing them all would bury
	// the ones that need a look.
	const notable = host.containers.filter(
		(c) =>
			(c.live && c.live.state !== "ok") ||
			(persistence && c.stored && c.stored.state !== "ok"),
	);
	const collecting = host.containers.length - notable.length;

	return (
		<SettingsSubsection
			title={host.host}
			action={
				host.state === "pending" ? (
					<span className="text-sm text-muted-foreground">Not listed yet</span>
				) : (
					<Outcome ok={host.state === "ok"} title={host.error}>
						{host.state === "ok" ? "Reachable" : "Unreachable"}
					</Outcome>
				)
			}
		>
			<div className="space-y-3">
				{host.state === "failing" && (
					<ErrorNote>
						Can't list containers since {formatTime(host.since)}: {host.error}.
						Last listed {formatTime(host.lastOk)}.
					</ErrorNote>
				)}
				<Note>
					{collecting} {collecting === 1 ? "container" : "containers"}{" "}
					collecting normally
					{notable.length > 0 && `, ${notable.length} listed below`}.
				</Note>
				{notable.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Container</Th>
							<Th>Live</Th>
							{persistence && <Th>Stored</Th>}
						</THead>
						<TBody>
							{notable.map((c) => (
								<tr key={c.id}>
									<Td className="font-mono">{c.name || c.id.slice(0, 12)}</Td>
									<Td className="whitespace-normal">
										<CollectionStatus
											status={describeLive(c.live)}
											since={c.live?.since}
										/>
									</Td>
									{persistence && (
										<Td className="whitespace-normal">
											<CollectionStatus
												status={describeStored(c.stored)}
												since={c.stored?.since}
											/>
										</Td>
									)}
								</tr>
							))}
						</TBody>
					</SettingsTable>
				)}
			</div>
		</SettingsSubsection>
	);
}

function StorageBlock({ storage }: { storage: StorageCollectionHealth }) {
	return (
		<SettingsSubsection
			title="Log storage"
			action={
				<Outcome ok={storage.state !== "failing"} title={storage.error}>
					{storage.state === "failing" ? "Writes failing" : "Writing"}
				</Outcome>
			}
		>
			<div className="space-y-1">
				{storage.state === "failing" && (
					<ErrorNote>
						Writes failing since {formatTime(storage.since)}: {storage.error}
					</ErrorNote>
				)}
				<Note>
					{formatBytes(storage.freeBytes)} free on disk
					{storage.lowSpace &&
						", less than the store may still grow to. Free some space or lower the total cap"}
					.
				</Note>
				<Note>
					{storage.droppedLines.toLocaleString()} lines dropped since startup
					when the writer fell behind; each drop is re-read from the engine.
					{storage.recovering > 0
						? ` ${storage.recovering} ${storage.recovering === 1 ? "container is" : "containers are"} still being re-read, so their stored history has a gap for now.`
						: " No gaps are waiting to be re-read."}
				</Note>
				<Note>
					Write queue: {storage.backlog.toLocaleString()} of{" "}
					{storage.backlogCap.toLocaleString()}.
				</Note>
			</div>
		</SettingsSubsection>
	);
}

export function CollectionHealthSection() {
	const { data, isLoading, error } = useCollectionHealth();
	const problems = collectionProblems(data);

	return (
		<SettingsSection
			title="Collection health"
			description="Whether LogDeck can still observe every host and container. A container that is collecting but quiet is simply not logging; one that can't be collected is listed with the reason."
		>
			<div className="space-y-8">
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>
						Failed to load collection health: {error.message}
					</ErrorNote>
				)}
				{data && (
					<>
						{problems.length === 0 ? (
							<Note>Everything is being collected.</Note>
						) : (
							<ul className="space-y-1 text-base text-rose-700 sm:text-sm dark:text-rose-400">
								{problems.map((problem) => (
									<li key={problem}>{problem}</li>
								))}
							</ul>
						)}
						{data.hosts.map((host) => (
							<HostBlock
								key={host.host}
								host={host}
								persistence={data.storage !== undefined}
							/>
						))}
						{data.storage ? (
							<StorageBlock storage={data.storage} />
						) : (
							<SettingsSubsection title="Log storage">
								<Note>Log persistence is disabled, so nothing is stored.</Note>
							</SettingsSubsection>
						)}
					</>
				)}
				<SettingsSubsection title="Monitoring LogDeck itself">
					<Note>
						LogDeck can't send any alert while it is stopped, so watch it from
						outside. <code className="font-mono">/api/v1/healthz</code> answers
						200 whenever the server runs, for container liveness checks.{" "}
						<code className="font-mono">/api/v1/healthz?strict=1</code> answers
						503 while collection is degraded, for an external uptime monitor; no
						answer at all means LogDeck is down.
					</Note>
				</SettingsSubsection>
			</div>
		</SettingsSection>
	);
}
