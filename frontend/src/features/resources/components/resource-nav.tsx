import { Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";

export function ResourceNav() {
	return (
		<nav className="flex items-center gap-1">
			<Button variant="ghost" size="sm" asChild>
				<Link to="/images">Images</Link>
			</Button>
			<Button variant="ghost" size="sm" asChild>
				<Link to="/volumes">Volumes</Link>
			</Button>
			<Button variant="ghost" size="sm" asChild>
				<Link to="/networks">Networks</Link>
			</Button>
		</nav>
	);
}
