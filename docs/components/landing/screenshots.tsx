import Image from "next/image";

export function Screenshots() {
  return (
    <section className="border-y bg-muted/30 py-20 sm:py-24">
      <div className="container">
        <div className="max-w-2xl">
          <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Built around the log view
          </h2>
          <p className="mt-3 text-muted-foreground">
            Streaming logs with search, level filters, time ranges, and
            one-click download — for a single container or a whole Compose
            stack merged by timestamp.
          </p>
        </div>

        <div className="mt-10 overflow-hidden rounded-xl border bg-background shadow-sm">
          <Image
            src="/logs-view.png"
            alt="LogDeck log viewer with search, level filters, and streaming logs"
            width={5128}
            height={2892}
            className="h-auto w-full"
          />
        </div>
      </div>
    </section>
  );
}
