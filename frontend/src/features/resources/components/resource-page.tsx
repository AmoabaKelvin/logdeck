import { Link } from "@tanstack/react-router";
import { ArrowLeftIcon, RefreshCcwIcon } from "lucide-react";
import type { ReactNode } from "react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { HostError } from "@/features/containers/types";

export interface ResourceColumn<T> {
	header: string;
	cell: (item: T) => ReactNode;
}

interface ResourcePageProps<T> {
	title: string;
	items: T[];
	hostErrors: HostError[];
	isLoading: boolean;
	error: Error | null;
	isFetching: boolean;
	onRefresh: () => void;
	columns: ResourceColumn<T>[];
	rowKey: (item: T) => string;
	filterText: (item: T) => string;
}

export function ResourcePage<T>({
	title,
	items,
	hostErrors,
	isLoading,
	error,
	isFetching,
	onRefresh,
	columns,
	rowKey,
	filterText,
}: ResourcePageProps<T>) {
	const [filter, setFilter] = useState("");

	const filtered = useMemo(() => {
		const query = filter.trim().toLowerCase();
		if (!query) return items;
		return items.filter((item) =>
			filterText(item).toLowerCase().includes(query),
		);
	}, [items, filter, filterText]);

	return (
		<div className="w-full space-y-4">
			<div className="flex items-center justify-between gap-4">
				<div className="flex items-center gap-2">
					<Button variant="ghost" size="sm" className="h-9 shrink-0" asChild>
						<Link to="/" aria-label="Back to dashboard">
							<ArrowLeftIcon className="size-4" />
						</Link>
					</Button>
					<h1 className="text-2xl font-bold tracking-tight">{title}</h1>
				</div>
				<Button
					variant="ghost"
					size="sm"
					onClick={onRefresh}
					className="h-9 shrink-0"
				>
					<RefreshCcwIcon
						className={`size-4 ${isFetching ? "animate-spin" : ""}`}
					/>
				</Button>
			</div>

			{hostErrors.map((hostError) => (
				<p key={hostError.host} className="text-sm text-muted-foreground">
					Could not reach host "{hostError.host}": {hostError.message}
				</p>
			))}

			<Input
				placeholder={`Filter ${title.toLowerCase()}...`}
				value={filter}
				onChange={(event) => setFilter(event.target.value)}
				className="max-w-sm"
			/>

			{isLoading ? (
				<div className="flex items-center justify-center py-20">
					<Spinner className="size-6" />
				</div>
			) : error ? (
				<p className="text-sm text-destructive">
					Failed to load {title.toLowerCase()}: {error.message}
				</p>
			) : (
				<div className="rounded-md border">
					<Table>
						<TableHeader>
							<TableRow>
								{columns.map((column) => (
									<TableHead key={column.header}>{column.header}</TableHead>
								))}
							</TableRow>
						</TableHeader>
						<TableBody>
							{filtered.length === 0 ? (
								<TableRow>
									<TableCell
										colSpan={columns.length}
										className="py-8 text-center text-muted-foreground"
									>
										No {title.toLowerCase()} found.
									</TableCell>
								</TableRow>
							) : (
								filtered.map((item) => (
									<TableRow key={rowKey(item)}>
										{columns.map((column) => (
											<TableCell key={column.header}>
												{column.cell(item)}
											</TableCell>
										))}
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
			)}
		</div>
	);
}
