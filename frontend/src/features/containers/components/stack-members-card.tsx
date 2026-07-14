import { Link } from "@tanstack/react-router";
import { FileTextIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
	TooltipProvider,
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

interface StackMembersCardProps {
	members: ContainerInfo[];
	isReadOnly: boolean;
}

export function StackMembersCard({
	members,
	isReadOnly,
}: StackMembersCardProps) {
	const purgeHistory = useDeleteHistoryContainer();
	const [purgeTarget, setPurgeTarget] = useState<PurgeHistoryTarget | null>(
		null,
	);

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
		<Card>
			<CardHeader>
				<CardTitle className="text-base">
					Stack Members · {members.length}
				</CardTitle>
				{hasRemoved && (
					<p className="text-sm text-muted-foreground">
						Removed members are not part of the live aggregated stream below.
						Open their stored logs to read their history.
					</p>
				)}
			</CardHeader>
			<CardContent>
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Image</TableHead>
							<TableHead>State</TableHead>
							<TableHead className="text-right">Actions</TableHead>
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
									<TableCell className="text-xs text-muted-foreground">
										{formatImageName(member.image)}
									</TableCell>
									<TableCell>
										<div className="flex items-center gap-1.5">
											<Badge
												className={`${getStateBadgeClass(member.state)} border-0`}
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
										<TooltipProvider>
											<div className="flex items-center justify-end gap-1">
												<Tooltip>
													<TooltipTrigger asChild>
														<Button
															variant="outline"
															size="icon"
															className="h-8 w-8"
															asChild
														>
															<Link
																to="/containers/$containerId/logs"
																params={{
																	containerId:
																		getContainerUrlIdentifier(member),
																}}
																aria-label={
																	removed
																		? "View stored logs"
																		: "View container logs"
																}
															>
																<FileTextIcon className="size-4" />
															</Link>
														</Button>
													</TooltipTrigger>
													<TooltipContent>
														{removed ? "View stored logs" : "View logs"}
													</TooltipContent>
												</Tooltip>
												{removed && (
													<Tooltip>
														<TooltipTrigger asChild>
															<span className="inline-block">
																<Button
																	variant="outline"
																	size="icon"
																	className="h-8 w-8 text-destructive hover:bg-destructive hover:text-white"
																	onClick={() =>
																		setPurgeTarget({
																			name: getContainerUrlIdentifier(member),
																			host: member.host,
																			removed: true,
																		})
																	}
																	disabled={
																		isReadOnly || purgeHistory.isPending
																	}
																	aria-label="Delete stored logs"
																>
																	<Trash2Icon className="size-4" />
																</Button>
															</span>
														</TooltipTrigger>
														<TooltipContent>
															{isReadOnly
																? "Delete stored logs (Read-only mode)"
																: "Delete stored logs"}
														</TooltipContent>
													</Tooltip>
												)}
											</div>
										</TooltipProvider>
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			</CardContent>

			<PurgeHistoryDialog
				target={purgeTarget}
				isPending={purgeHistory.isPending}
				onConfirm={handleConfirmPurge}
				onOpenChange={(open) => {
					if (!open) setPurgeTarget(null);
				}}
			/>
		</Card>
	);
}
