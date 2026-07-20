import { Button } from "@/components/logdeck-demo/ui/button";

import { useDemoView } from "../demo-view";

export function ResourceNav() {
	const { setView } = useDemoView();

	return (
		<nav className="flex items-center gap-1">
			<Button variant="ghost" size="sm" onClick={() => setView("images")}>
				Images
			</Button>
			<Button variant="ghost" size="sm" onClick={() => setView("volumes")}>
				Volumes
			</Button>
			<Button variant="ghost" size="sm" onClick={() => setView("networks")}>
				Networks
			</Button>
		</nav>
	);
}
