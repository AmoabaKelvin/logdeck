import { Link, useNavigate } from "@tanstack/react-router";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { LogOutIcon, SettingsIcon } from "@/components/ui/icons";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { useAuth } from "@/contexts/auth-context";

const NAV_ITEMS = [
	{ to: "/", label: "Containers", exact: true },
	{ to: "/images", label: "Images", exact: false },
	{ to: "/volumes", label: "Volumes", exact: false },
	{ to: "/networks", label: "Networks", exact: false },
] as const;

// text-sm on mobile so all four labels fit a 390px row; the vertical
// padding carries the touch target instead.
const navLinkClass =
	"rounded-md px-2.5 py-2 text-sm whitespace-nowrap text-muted-foreground hover:text-foreground sm:py-1.5";

export function AppHeader() {
	const { logout, user, isAuthEnabled } = useAuth();
	const navigate = useNavigate();

	const handleLogout = () => {
		logout();
		navigate({ to: "/login" });
	};

	return (
		<header className="sticky top-0 z-40 border-b border-border/70 bg-background/85 backdrop-blur-sm">
			<div className="app-width flex flex-wrap items-center gap-x-6 gap-y-1 px-4 py-3 sm:h-14 sm:flex-nowrap sm:py-0 sm:pr-2 lg:px-8 lg:pr-4">
				<Link
					to="/"
					className="shrink-0 text-sm font-semibold tracking-tight text-foreground"
				>
					LogDeck
				</Link>

				<div className="ml-auto flex shrink-0 items-center gap-0.5 sm:order-last">
					<Tooltip>
						<TooltipTrigger asChild>
							<Button variant="ghost" size="icon-sm" asChild>
								<Link to="/settings" aria-label="Settings">
									<SettingsIcon className="size-4" />
								</Link>
							</Button>
						</TooltipTrigger>
						<TooltipContent>Settings</TooltipContent>
					</Tooltip>

					<ThemeToggle />

					{isAuthEnabled && (
						<Tooltip>
							<TooltipTrigger asChild>
								<Button
									variant="ghost"
									size="icon-sm"
									onClick={handleLogout}
									aria-label="Log out"
								>
									<LogOutIcon className="size-4" />
								</Button>
							</TooltipTrigger>
							<TooltipContent>
								Log out{user?.username ? ` (${user.username})` : ""}
							</TooltipContent>
						</Tooltip>
					)}
				</div>

				<nav className="order-last -mx-2.5 flex w-full min-w-0 items-center gap-0.5 overflow-x-auto px-2.5 sm:order-none sm:mx-0 sm:w-auto sm:px-0">
					{NAV_ITEMS.map((item) => (
						<Link
							key={item.to}
							to={item.to}
							activeOptions={{ exact: item.exact }}
							activeProps={{ className: "bg-muted text-foreground" }}
							className={navLinkClass}
						>
							{item.label}
						</Link>
					))}
				</nav>
			</div>
		</header>
	);
}
