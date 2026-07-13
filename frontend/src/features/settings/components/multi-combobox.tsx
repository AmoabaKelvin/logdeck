import { XIcon } from "lucide-react";
import { useId, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface MultiComboboxProps {
	id?: string;
	placeholder: string;
	options: string[];
	selected: string[];
	onChange: (values: string[]) => void;
}

/**
 * A multi-select combobox: type to filter live suggestions, or enter
 * arbitrary free text. Selected values render as removable token chips.
 *
 * While the list is open the root carries `data-combobox-open`, which the
 * parent dialog uses to keep Escape from dismissing the whole dialog.
 */
export function MultiCombobox({
	id,
	placeholder,
	options,
	selected,
	onChange,
}: MultiComboboxProps) {
	const listId = useId();
	const [query, setQuery] = useState("");
	const [open, setOpen] = useState(false);
	const [highlight, setHighlight] = useState(0);

	const trimmed = query.trim();

	const items = useMemo(() => {
		const lower = trimmed.toLowerCase();
		const matches = options
			.filter(
				(option) =>
					!selected.includes(option) && option.toLowerCase().includes(lower),
			)
			.map((value) => ({ value, isNew: false }));
		const exists =
			options.some((option) => option.toLowerCase() === lower) ||
			selected.some((value) => value.toLowerCase() === lower);
		if (trimmed && !exists) matches.push({ value: trimmed, isNew: true });
		return matches;
	}, [options, selected, trimmed]);

	const showList = open && items.length > 0;
	const activeIndex = Math.min(highlight, items.length - 1);

	function add(value: string) {
		const next = value.trim();
		if (!next) return;
		if (!selected.includes(next)) onChange([...selected, next]);
		setQuery("");
		setHighlight(0);
	}

	function remove(value: string) {
		onChange(selected.filter((v) => v !== value));
	}

	function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setOpen(true);
			setHighlight((i) => Math.min(i + 1, items.length - 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setHighlight((i) => Math.max(i - 1, 0));
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (showList && items[activeIndex]) {
				add(items[activeIndex].value);
			} else if (trimmed) {
				add(trimmed);
			}
		} else if (e.key === "Escape" && showList) {
			setOpen(false);
		}
	}

	return (
		<div className="space-y-1.5" data-combobox-open={showList || undefined}>
			{selected.length > 0 && (
				<div className="flex flex-wrap gap-1.5">
					{selected.map((value) => (
						<Badge
							key={value}
							variant="secondary"
							className="gap-1 overflow-visible pr-1"
						>
							{value}
							<button
								type="button"
								aria-label={`Remove ${value}`}
								onClick={() => remove(value)}
								className="relative flex size-4 items-center justify-center rounded-sm text-muted-foreground transition-colors after:absolute after:-inset-3 hover:text-foreground"
							>
								<XIcon className="size-3" />
							</button>
						</Badge>
					))}
				</div>
			)}
			<div className="relative">
				<Input
					id={id}
					role="combobox"
					aria-expanded={showList}
					aria-controls={listId}
					aria-activedescendant={
						showList ? `${listId}-${activeIndex}` : undefined
					}
					aria-autocomplete="list"
					autoComplete="off"
					value={query}
					onChange={(e) => {
						setQuery(e.target.value);
						setHighlight(0);
						setOpen(true);
					}}
					onFocus={() => setOpen(true)}
					onBlur={() => setOpen(false)}
					onKeyDown={handleKeyDown}
					placeholder={placeholder}
					className="h-8 text-xs"
				/>
				{showList && (
					<div
						role="listbox"
						id={listId}
						aria-label={placeholder}
						className="absolute top-full left-0 z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
					>
						{items.map((item, index) => (
							// biome-ignore lint/a11y/useKeyWithClickEvents: keyboard handled on the input via aria-activedescendant
							<div
								key={item.value}
								id={`${listId}-${index}`}
								role="option"
								tabIndex={-1}
								aria-selected={index === activeIndex}
								onMouseDown={(e) => e.preventDefault()}
								onClick={() => add(item.value)}
								onMouseEnter={() => setHighlight(index)}
								className={cn(
									"cursor-default rounded-sm px-2 py-1.5 text-sm",
									index === activeIndex && "bg-accent text-accent-foreground",
								)}
							>
								{item.isNew ? `Add "${item.value}"` : item.value}
							</div>
						))}
					</div>
				)}
			</div>
		</div>
	);
}
