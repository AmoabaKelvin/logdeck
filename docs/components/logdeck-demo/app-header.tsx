import { toast } from "sonner";
import {
  type DemoView,
  useDemoView,
} from "@/components/logdeck-demo/demo-view";
import { ThemeToggle } from "@/components/logdeck-demo/theme-toggle";
import { Button } from "@/components/logdeck-demo/ui/button";
import { SettingsIcon } from "@/components/logdeck-demo/ui/icons";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/logdeck-demo/ui/tooltip";

const NAV_ITEMS: { view: DemoView; label: string }[] = [
  { view: "containers", label: "Containers" },
  { view: "images", label: "Images" },
  { view: "volumes", label: "Volumes" },
  { view: "networks", label: "Networks" },
];

// text-sm on mobile so all four labels fit a 390px row; the vertical
// padding carries the touch target instead.
const navLinkClass =
  "rounded-md px-2.5 py-2 text-sm whitespace-nowrap text-muted-foreground hover:text-foreground sm:py-1.5";

export function AppHeader() {
  const { view, setView } = useDemoView();

  return (
    <header className="sticky top-0 z-40 border-b border-border/70 bg-background/85 backdrop-blur-sm">
      <div className="app-width flex flex-wrap items-center gap-x-6 gap-y-1 px-4 py-3 sm:h-14 sm:flex-nowrap sm:py-0 sm:pr-2 lg:px-8 lg:pr-4">
        <button
          type="button"
          onClick={() => setView("containers")}
          className="shrink-0 text-sm font-semibold tracking-tight text-foreground"
        >
          LogDeck
        </button>

        <div className="ml-auto flex shrink-0 items-center gap-0.5 sm:order-last">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="Settings"
                onClick={() => toast.info("Settings are not part of the demo")}
              >
                <SettingsIcon className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Settings</TooltipContent>
          </Tooltip>

          <ThemeToggle />
        </div>

        <nav className="order-last -mx-2.5 flex w-full min-w-0 items-center gap-0.5 overflow-x-auto px-2.5 sm:order-none sm:mx-0 sm:w-auto sm:px-0">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.view}
              type="button"
              onClick={() => setView(item.view)}
              className={`${navLinkClass} ${view === item.view ? "bg-muted text-foreground" : ""}`}
            >
              {item.label}
            </button>
          ))}
        </nav>
      </div>
    </header>
  );
}
