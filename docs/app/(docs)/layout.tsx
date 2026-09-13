import { SidebarNav } from "@/components/docs/sidebar-nav";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="mx-auto w-full max-w-7xl border-x border-dashed border-base-200 lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <aside className="border-r border-dashed border-base-200 max-lg:hidden">
        <div className="sticky top-16 max-h-[calc(100dvh-4rem)] overflow-y-auto px-4 py-10">
          <SidebarNav />
        </div>
      </aside>
      <div className="min-w-0 px-4 sm:px-8 lg:px-12">{children}</div>
    </div>
  );
}
