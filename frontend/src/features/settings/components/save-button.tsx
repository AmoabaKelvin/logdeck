import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";

interface SaveButtonProps {
	isPending: boolean;
	onClick: () => void;
}

export function SaveButton({ isPending, onClick }: SaveButtonProps) {
	return (
		<Button size="sm" disabled={isPending} onClick={onClick}>
			{isPending ? (
				<>
					<Spinner className="size-3" />
					Saving...
				</>
			) : (
				"Save changes"
			)}
		</Button>
	);
}
