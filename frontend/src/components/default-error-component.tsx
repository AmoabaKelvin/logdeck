import type { ErrorComponentProps } from "@tanstack/react-router";
import { useRouter } from "@tanstack/react-router";
import { AlertTriangleIcon, RefreshCcwIcon, RotateCcwIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

// Router-level error boundary fallback (see defaultErrorComponent in
// src/main.tsx). Rendered in place of a route that threw during render or
// loading.
export function DefaultErrorComponent({ error, reset }: ErrorComponentProps) {
	const router = useRouter();

	const handleRetry = () => {
		reset();
		void router.invalidate();
	};

	return (
		<div className="min-h-screen bg-background flex items-center justify-center p-4">
			<Card className="w-full max-w-lg">
				<CardHeader>
					<CardTitle className="flex items-center gap-2 text-base">
						<AlertTriangleIcon className="size-5 text-destructive" />
						Something went wrong
					</CardTitle>
				</CardHeader>
				<CardContent className="space-y-4">
					<p className="text-sm text-muted-foreground">
						An unexpected error occurred while rendering this page.
					</p>
					<pre className="max-h-[200px] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs whitespace-pre-wrap break-all text-destructive">
						{error.message || String(error)}
					</pre>
					<div className="flex gap-2">
						<Button size="sm" onClick={handleRetry}>
							<RotateCcwIcon className="mr-2 size-4" />
							Try again
						</Button>
						<Button
							size="sm"
							variant="outline"
							onClick={() => window.location.reload()}
						>
							<RefreshCcwIcon className="mr-2 size-4" />
							Reload page
						</Button>
					</div>
				</CardContent>
			</Card>
		</div>
	);
}
