import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	ChevronDownIcon,
	FileTextIcon,
	Trash2Icon,
} from "@/components/ui/icons";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { useDeleteHistoryContainer } from "../hooks/use-delete-history-container";
import type { ContainerInfo } from "../types";
import {
	formatBytes,
	formatContainerName,
	formatImageName,
	getContainerUrlIdentifier,
	getStateBadgeClass,
	isRemovedContainer,
	toTitleCase,
} from "./container-utils";
import type { PurgeHistoryTarget } from "./purge-history-dialog";
import { PurgeHistoryDialog } from "./purge-history-dialog";

interface StackMembersPanelProps {
	members: ContainerInfo[];
	isReadOnly: boolean;
}

/**
 * The stack's members, behind a disclosure so the aggregated log stream keeps
 * the page — the same shape the container detail page uses.
 */
export function StackMembersPanel({
	members,
	isReadOnly,
}: StackMembersPanelProps) {
	const purgeHistory = useDeleteHistoryContainer();
	const [purgeTarget, setPurgeTarget] = useState<PurgeHistoryTarget | null>(
		null,
	);
	const [isOpen, setIsOpen] = useState(false);

	if (members.length === 0) return null;

	const hasRemoved = members.some(isRemovedContainer);

	function handleConfirmPurge() {
		if (!purgeTarget) return;
		purgeHistory.mutate(
			{ name: purgeTarget.name, host: purgeTarget.host },
			{ onSettled: () => setPurgeTarget(null) },
		);
	}

	return (
		<div>
			<button
				type="button"
				aria-expanded={isOpen}
				aria-controls="stack-members-panel"
				onClick={() => setIsOpen((open) => !open)}
				className={`-ml-2.5 inline-flex items-center gap-1.5 rounded-md px-2.5 py-2 text-sm whitespace-nowrap sm:py-1.5 ${
					isOpen
						? "bg-muted text-foreground"
						: "text-muted-foreground hover:text-foreground"
				}`}
			>
				Containers
				<span className="tabular-nums">{members.length}</span>
				{isOpen && <ChevronDownIcon className="size-4 shrink-0 rotate-180" />}
			</button>

			{isOpen && (
				<div
					id="stack-members-panel"
					className="mt-3 rounded-xl border border-border/70 p-4 sm:p-5"
				>
					{hasRemoved && (
						<p className="mb-3 text-base text-muted-foreground sm:text-sm">
							Removed members are not part of the live aggregated stream below.
							Open their stored logs to read their history.
						</p>
					)}
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="whitespace-nowrap">Name</TableHead>
								<TableHead className="whitespace-nowrap">Image</TableHead>
								<TableHead className="whitespace-nowrap">State</TableHead>
								<TableHead className="sr-only text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{members.map((member) => {
								const removed = isRemovedContainer(member);

								return (
									<TableRow key={`${member.host}/${member.id}`}>
										<TableCell
											className={`font-medium ${removed ? "text-muted-foreground" : ""}`}
										>
											{formatContainerName(member.names)}
										</TableCell>
										<TableCell className="text-muted-foreground">
											{formatImageName(member.image)}
										</TableCell>
										<TableCell>
											<div className="flex items-center gap-1.5">
												<Badge
													className={`border-transparent ${getStateBadgeClass(member.state)}`}
												>
													{toTitleCase(member.state)}
												</Badge>
												{removed && member.storedBytes > 0 && (
													<span className="font-mono text-xs text-muted-foreground">
														{formatBytes(member.storedBytes)} stored
													</span>
												)}
											</div>
										</TableCell>
										<TableCell className="text-right">
											<div className="flex items-center justify-end gap-0.5">
												<Tooltip>
													<TooltipTrigger asChild>
														<Button variant="ghost" size="icon-sm" asChild>
															<Link
																to="/containers/$containerId/logs"
																params={{
																	containerId:
																		getContainerUrlIdentifier(member),
																}}
																aria-label={
																	removed
																		? `Stored logs for ${formatContainerName(member.names)}`
																		: `Logs for ${formatContainerName(member.names)}`
																}
															>
																<FileTextIcon className="size-4" />
															</Link>
														</Button>
													</TooltipTrigger>
													<TooltipContent>
														{removed ? "Stored logs" : "Logs"}
													</TooltipContent>
												</Tooltip>
												{removed && (
													<Tooltip>
														<TooltipTrigger asChild>
															<Button
																variant="ghost"
																size="icon-sm"
																onClick={() =>
																	setPurgeTarget({
																		name: getContainerUrlIdentifier(member),
																		host: member.host,
																		removed: true,
																	})
																}
																disabled={isReadOnly || purgeHistory.isPending}
																aria-label={`Delete stored logs for ${formatContainerName(member.names)}`}
																className="text-muted-foreground hover:text-destructive"
															>
																<Trash2Icon className="size-4" />
															</Button>
														</TooltipTrigger>
														<TooltipContent>
															{isReadOnly
																? "Delete stored logs (read-only mode)"
																: "Delete stored logs"}
														</TooltipContent>
													</Tooltip>
												)}
											</div>
										</TableCell>
									</TableRow>
								);
							})}
						</TableBody>
					</Table>
				</div>
			)}

			<PurgeHistoryDialog
				target={purgeTarget}
				isPending={purgeHistory.isPending}
				onConfirm={handleConfirmPurge}
				onOpenChange={(open) => {
					if (!open) setPurgeTarget(null);
				}}
			/>
		</div>
	);
}
