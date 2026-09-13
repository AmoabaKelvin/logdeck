import type { Metadata } from "next";
import Link from "next/link";

import {
  FigBlockAnatomy,
  FigBloom,
  FigPipeline,
  FigQuery,
  FigResults,
  FigRetention,
  FigRowCost,
  FigTimestampTrap,
} from "@/components/blog/figures";
import { Toc } from "@/components/blog/toc";
import { ForceLight } from "@/components/landing/force-light";
import { Wrapper, h1Class, pill } from "@/components/landing/ui";

export const metadata: Metadata = {
  title: "Storing 10× more container logs in the same SQLite file",
  description:
    "How LogDeck's log store went from one row per line to sealed, columnar, zstd-compressed blocks without giving up instant queries, exact bytes, dedup, or stable cursors.",
  alternates: { canonical: "/blog/log-store" },
  openGraph: {
    type: "article",
    title: "Storing 10× more container logs in the same SQLite file",
    description:
      "One row per line cost more disk than the logs themselves. Sealing every 1,000 lines into a columnar zstd block fixed that. Most of the work went into the invariants around it.",
  },
};

function H2({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h2
      id={id}
      className="mt-14 scroll-m-24 font-display text-xl font-medium tracking-tight text-black md:text-2xl"
    >
      <a href={`#${id}`} className="hover:text-accent-500">
        {children}
      </a>
    </h2>
  );
}

function P({ children }: { children: React.ReactNode }) {
  return (
    <p className="mt-5 text-pretty text-base/7 text-base-700">{children}</p>
  );
}

function Code({ children }: { children: string }) {
  return (
    <div className="not-prose my-6 overflow-hidden rounded-xl border border-dashed border-base-200">
      <div className="flex items-center gap-1 bg-sand-100 px-4 py-3">
        <span className="size-2 rounded-full bg-[#ff421e]" />
        <span className="size-2 rounded-full bg-[#60beff]" />
        <span className="size-2 rounded-full bg-[#e3962d]" />
      </div>
      <pre className="overflow-x-auto p-4 font-mono text-xs/5 text-base-700 sm:text-[13px]/6">
        {children}
      </pre>
    </div>
  );
}

function Mono({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-sand-100 px-1 py-0.5 font-mono text-[0.85em] text-base-900">
      {children}
    </code>
  );
}

const sections = [
  { id: "problem", title: "A line cost 1.28 lines" },
  { id: "constraints", title: "What could not change" },
  { id: "design", title: "Two tables, one writer" },
  { id: "block", title: "Inside a block" },
  { id: "timestamps", title: "The one-in-ten timestamp" },
  { id: "dedup", title: "Dedup without an index" },
  { id: "paging", title: "Paging across two tables" },
  { id: "retention", title: "Retention got cheaper too" },
  { id: "results", title: "What it measures at" },
  { id: "inspect", title: "Look at your own store" },
  { id: "compare", title: "How this compares" },
  { id: "not-done", title: "What we did not do" },
];

const corpora = [
  { name: "Postgres 17, one real container", plain: "6.9×", block: "7.5×" },
  {
    name: "nginx, one real container, access and error logs from 6,000 scripted requests",
    plain: "13.1×",
    block: "12.5×",
  },
  {
    name: "31 containers on a dev laptop, about half of them synthetic demo apps",
    plain: "6.7×",
    block: "8.4×",
  },
  {
    name: "macOS install.log, real text with a synthetic Docker prefix",
    plain: "9.3×",
    block: "10.6×",
  },
  {
    name: "Synthetic app logs, JSON and key=value",
    plain: "14.7×",
    block: "22.7×",
  },
];

export default function LogStorePost() {
  return (
    <div className="bg-white font-display text-base-900 selection:bg-sand-100 selection:text-accent-500">
      <ForceLight />
      <article>
        <Wrapper className="pt-16 pb-4 sm:pt-24">
          <p className="font-mono text-xs text-base-500">
            Engineering · 13 September 2026 · 12 min read
          </p>
          <h1 className={`${h1Class} mt-4 max-w-[26ch] text-balance`}>
            Storing 10× more container logs in the same SQLite file
          </h1>
          <p className="mt-4 max-w-2xl text-pretty text-base/7 text-base-500 sm:text-lg/8">
            LogDeck keeps container logs after the container is gone. The first
            version stored them as one SQLite row per line, and when we measured
            real log corpora the database came out 28% larger than the text it
            held. Sealing every 1,000 lines into one compressed row holds about
            10× more history under the same cap on real Postgres output, 16× on
            nginx access logs, and 12× in the stress harness. The compression
            itself was the smaller part of the change. Most of the work went
            into keeping four promises the store had already made, about instant
            queries, exact bytes, dedup, and stable cursors, and that is most of
            what this post covers.
          </p>
        </Wrapper>

        <Wrapper className="border-t border-dashed border-base-200 pt-4 pb-24 lg:grid lg:grid-cols-[minmax(0,48rem)_1fr] lg:gap-16">
          <div className="min-w-0 max-w-3xl">
            <H2 id="problem">A line cost 1.28 lines</H2>
            <P>
              History mode is the reason the store exists. You open a container
              that was rebuilt an hour ago, and you can still scroll back
              through what the old one printed. To do that LogDeck tails every
              container on every host and writes each line into a SQLite
              database next to its config file, capped at 50 MB per container
              and 1 GB total by default.
            </P>
            <P>
              In the original layout every line was a row in{" "}
              <Mono>log_lines</Mono> with its timestamp, stream, level, and the
              raw text. Across five real log corpora, measured when the change
              was made, the database came out about 28% larger than the text it
              stored. A SQLite row carries a header, and the index the queries
              need, on (container, timestamp), carries one entry per row.
              Together those took more than a fifth of every cap.
            </P>
            <FigRowCost />
            <P>
              Log lines compress well. A thousand lines from one container share
              a format and a vocabulary, and each timestamp is mostly the same
              as the one before it, so the fix was to stop storing them one at a
              time.
            </P>

            <H2 id="constraints">What could not change</H2>
            <P>
              LogDeck is one Go binary on a small VPS. The SQLite driver is pure
              Go, because every release builds with <Mono>CGO_ENABLED=0</Mono>,
              so there is no extension to lean on and no separate process to
              spin up. Within that, the store had four promises that a
              compression scheme could easily break:
            </P>
            <ul className="mt-5 space-y-3 text-base/7 text-base-700">
              <li className="flex gap-3">
                <span className="mt-2.5 size-1.5 shrink-0 rounded-full bg-accent-500" />
                <span>
                  A line is queryable the moment it commits. If you switch from
                  Live to History, the line you just watched arrive is there.
                </span>
              </li>
              <li className="flex gap-3">
                <span className="mt-2.5 size-1.5 shrink-0 rounded-full bg-accent-500" />
                <span>
                  A stored line is byte-for-byte what the engine sent. History
                  is parsed by the same function as Live, so level detection and
                  multi-line grouping cannot drift between the two views.
                </span>
              </li>
              <li className="flex gap-3">
                <span className="mt-2.5 size-1.5 shrink-0 rounded-full bg-accent-500" />
                <span>
                  Re-reading a window never stores a line twice. When LogDeck
                  restarts, or a buffer overflows, it re-reads the engine from
                  its last watermark. That re-read overlaps what is already
                  stored on purpose, so every insert goes through dedup.
                </span>
              </li>
              <li className="flex gap-3">
                <span className="mt-2.5 size-1.5 shrink-0 rounded-full bg-accent-500" />
                <span>
                  Pages stay stable while the store moves underneath them. A
                  history page hands back a cursor. Ingestion, compression, and
                  retention keep running, and the next page must still resume
                  where the last one stopped.
                </span>
              </li>
            </ul>

            <H2 id="design">Two tables, one writer</H2>
            <P>
              The shape that satisfied all four is a hot table in front of a
              cold one. Lines still land in <Mono>log_lines</Mono>, in the same
              batch transaction as before, so the first promise holds with no
              extra work. Between batches the single writer goroutine looks at
              each container&apos;s hot count, and every time a container has
              1,000 unsealed lines it packs the oldest 1,000 into one row of{" "}
              <Mono>log_blocks</Mono> and deletes them from the hot table, in
              one transaction. After each batch a container holds fewer than a
              thousand unsealed lines, and everything older lives in blocks. A
              block is also full once it holds 8 MiB of text, so a container
              that prints very long lines cannot build one the decoder would
              refuse to read back. A seal that fails leaves its lines hot and is
              retried after the next batch.
            </P>
            <FigPipeline />
            <P>
              Sealing runs on the writer goroutine rather than a background one,
              and that rule came from a bug. Retention used to run on its own
              goroutine as a deferred transaction: select the oldest rows, then
              delete them. SQLite has one write lock, and a deferred transaction
              only asks for it at its first write, so under a firehose the
              janitor&apos;s delete lost the race to ingestion every time and
              died with <Mono>SQLITE_BUSY</Mono>. The caps held in tests, but in
              the stress run the file grew to 365 MB against a 5 MB cap. The fix
              routed eviction through the writer, which now sweeps between
              batches, and opened every transaction with the write lock up
              front. The same run then peaked at 13 MB. Sealing follows the same
              rule and takes its turn on that goroutine, between ingestion
              batches and retention sweeps.
            </P>
            <Code>{`CREATE TABLE log_blocks (
  container_ref INTEGER NOT NULL REFERENCES containers(id),
  ts_min_ns     INTEGER NOT NULL,
  ts_max_ns     INTEGER NOT NULL,
  seq_min       INTEGER NOT NULL,
  seq_max       INTEGER NOT NULL,
  lines         INTEGER NOT NULL,
  level_mask    INTEGER NOT NULL,  -- one bit per severity
  raw_bytes     INTEGER NOT NULL,  -- uncompressed size
  dedup_filter  BLOB NOT NULL,     -- bloom filter over line keys
  payload       BLOB NOT NULL      -- zstd(columnar layout)
);`}</Code>
            <P>
              Everything a query might want to know about a block without
              opening it lives in its own column: the time range, the sequence
              range, the line count, which severities appear, and the
              uncompressed size. The History summary, the cursor, and the
              retention math all read the time range, and none of them open a
              payload. The level mask is stored but not used yet. Level and
              search filters run in Go on grouped entries, because a stack
              trace&apos;s continuation lines classify as unknown level and can
              sit in the next block over, so skipping a block by its mask could
              cut the body off an error. Pruning blocks by level before the
              grouper runs is on the list, once that case is handled.
            </P>

            <H2 id="block">Inside a block</H2>
            <P>
              The fields in a block are grouped into runs rather than stored
              line by line. Compressors find repetition within a window, and a
              run of a thousand near-identical values compresses far better than
              the same values scattered between unrelated text. The whole layout
              is one loop per field:
            </P>
            <Code>{`// block.go, pack(), trimmed
buf = append(buf, blockFormatV1)
buf = binary.AppendUvarint(buf, uint64(len(lines)))
for _, l := range lines { buf = binary.AppendVarint(buf, l.tsNS-prevTS); prevTS = l.tsNS }
for _, l := range lines { buf = binary.AppendVarint(buf, l.seq-prevSeq); prevSeq = l.seq }
for _, l := range lines { buf = append(buf, byte(l.stream)) }
for _, l := range lines { buf = append(buf, byte(l.level)) }
buf = append(buf, verbatim...)
for _, b := range bodies { buf = binary.AppendUvarint(buf, uint64(len(b))) }
for _, b := range bodies { buf = append(buf, b...) }
payload := enc.EncodeAll(buf, nil)`}</Code>
            <P>
              The timestamp run is where most of the gain is. Docker and Podman
              prefix every line with an RFC 3339 timestamp at nanosecond
              precision, 30 bytes of mostly high-entropy digits. The store
              already holds the parsed nanosecond value, so the block stores the
              delta from the previous line as a varint. A gap under a
              millisecond is three bytes, a few seconds is five, against 30
              bytes of text either way, and a run of small varints is nearly
              free after zstd. The Postgres block below spans 201 hours of a
              mostly idle database, so its gaps are uneven: 664 of the 1,000
              deltas fit in two or three bytes, and the 23 that need seven are
              the hours of quiet between bursts. It still averages about three
              bytes per line.
            </P>
            <FigBlockAnatomy />
            <P>
              The baseline is plain zstd over the same thousand lines joined
              with newlines. The columnar layout beats it by 9% on Postgres, 25%
              on the mixed container set, and 1.5× on synthetic application
              logs, where the bodies repeat and the timestamps were most of the
              entropy. On nginx it barely does: access log lines repeat so much
              that plain zstd already reaches 13.1×, the layout gains 2% on top,
              and the dedup filter costs more than that, so the sealed block
              lands at 12.5×. Part of that is the capture: 6,000 requests in
              four seconds gave every line an almost identical Docker prefix,
              which is the best case for plain zstd. Re-timing the same lines
              with realistic gaps of 100 ms to 10 s between requests makes the
              prefixes cost plain zstd 7 to 9 bytes per line instead of 5, the
              layout&apos;s edge grows to 4 to 7%, and the sealed block ties
              plain zstd at about 11×. On nginx the ratio is zstd&apos;s; the
              layout is worth its filter and no more. We picked the block size
              by measuring too. 1,000 lines sits at the knee: 250 gives up about
              a fifth of the ratio, and 4,000 buys under a tenth more while
              quadrupling what a point read has to decompress.
            </P>
            <div className="not-prose my-8 overflow-x-auto rounded-xl bg-sand-100 p-4 sm:p-6">
              <table className="w-full min-w-[480px] text-sm">
                <thead>
                  <tr className="text-left text-base-500">
                    <th className="pb-3 font-medium">
                      corpus, 1,000-line blocks
                    </th>
                    <th className="pb-3 text-right font-medium">plain zstd</th>
                    <th className="pb-3 text-right font-medium">
                      sealed block, filter included
                    </th>
                  </tr>
                </thead>
                <tbody className="text-base-900">
                  {corpora.map((c) => (
                    <tr key={c.name} className="border-t border-sand-500/10">
                      <td className="py-2.5 pr-4">{c.name}</td>
                      <td className="py-2.5 text-right font-mono tabular-nums text-base-500">
                        {c.plain}
                      </td>
                      <td className="py-2.5 text-right font-mono tabular-nums">
                        {c.block}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <P>
              zstd runs at its default level with a 1 MiB window, which is
              enough to see an entire block. Both the encoder and decoder in the
              Go library size their internal state by GOMAXPROCS by default,
              which on a many-core host would reserve hundreds of megabytes for
              a store whose whole point is a small VPS. The store creates one
              encoder with concurrency 1, because only the writer seals, and one
              decoder bounded to the size of the database connection pool, since
              that already bounds concurrent readers.
            </P>

            <H2 id="timestamps">The one-in-ten timestamp</H2>
            <P>
              Lifting the timestamp out of the raw line nearly broke the second
              promise. On read, the store rebuilds the prefix from the
              nanosecond value and puts the body back, and the result has to be
              the exact bytes the engine sent. Go&apos;s{" "}
              <Mono>time.RFC3339Nano</Mono> layout looked like the obvious
              formatter and is wrong for about one line in ten: it trims
              trailing zeros from the fraction, so a timestamp ending in{" "}
              <Mono>.120000000Z</Mono> comes back as <Mono>.12Z</Mono>, and
              Docker itself never trims. Across three corpora, 9.5% to 10.9% of
              lines end their fraction with a zero, the rate a random last digit
              gives.
            </P>
            <FigTimestampTrap />
            <P>
              The fix is a fixed nine-digit layout, and packing checks it on
              every line anyway: it formats the value, compares it to the prefix
              on the original line, and only strips the prefix when the two
              match. A line that would not survive the round trip is stored
              whole, with a one-byte verbatim flag, so the worst an odd engine
              prefix can cost is a few bytes of compression on that line. The
              verbatim run is all zeros on every real Docker corpus so far, and
              the test suite feeds it lines it knows will fail.
            </P>

            <H2 id="dedup">Dedup without an index</H2>
            <P>
              The insert path rejects a line when an identical one, same
              timestamp, same stream, same bytes, is already stored for that
              container. In the hot table that is an index seek on the B-tree
              the insert is already touching. Once a line is inside a compressed
              blob there is nothing to seek, and decompressing a block to answer
              &quot;is this line in here?&quot; for every line of a backfill
              re-read would turn a cheap overlap into a slow one.
            </P>
            <P>
              So each block carries a bloom filter over its line keys, built at
              seal time and stored next to the payload. It is sized for a 2%
              false-positive rate, which works out to about 8 bits per line and
              six hash positions, roughly a kilobyte per block. A negative is
              proof the line is absent. A positive costs one decompression and
              an exact comparison, and the writer caches the last block it
              unpacked, because a re-read walks forward through time and
              consecutive lines usually land in the same block.
            </P>
            <FigBloom />
            <P>
              A backfill pays one small select per line, for the filters of the
              blocks whose time range covers it, and the payload is left out of
              that select on purpose. Live ingestion skips the filters entirely:
              the writer remembers the newest sealed timestamp per container,
              and a live line is newer than that by construction.
            </P>

            <H2 id="paging">Paging across two tables</H2>
            <P>
              A history page is a keyset cursor: the position of the oldest
              entry on the page, and the next page reads what is strictly older.
              The original cursor was (timestamp, rowid). Sealing destroys
              rowids, so a cursor taken while a line was hot would stop
              resolving once that line moved into a block. The fix was a
              store-wide sequence number: every line gets <Mono>seq</Mono> when
              it is inserted and keeps it when it is sealed, and the cursor is
              (timestamp, seq). Existing rows adopted their rowid at migration.
            </P>
            <FigQuery />
            <P>
              A query reads both tables newest-first and merges them, and
              decompresses surviving blocks in order of their newest line until
              the chunk is full. Because a backfill re-read can give an older
              timestamp a larger sequence, block ranges overlap and the scan
              cannot stop on count alone; it keeps going until the next
              block&apos;s newest possible position is older than the chunk
              boundary.
            </P>

            <H2 id="retention">Retention got cheaper too</H2>
            <P>
              The caps are enforced against <Mono>stored_bytes</Mono>, one
              counter per container generation, and it now counts what the
              generation occupies rather than its raw text. Sealing and eviction
              move exactly the same number in opposite directions, so repeated
              seal-and-evict cycles cannot drift the counter away from the disk.
            </P>
            <FigRetention />
            <P>
              Eviction is oldest-first per logical container, every generation
              of a (host, name) pair, so a container rebuilt ten times cannot
              hold ten times the cap. Deleting a block frees a thousand lines in
              one row, so a sweep under a firehose touches a handful of rows
              where it used to touch thousands. <Mono>logs.db</Mono> itself does
              not shrink: SQLite reuses freed pages rather than returning them,
              so the main file plateaus at its high-water mark, and only the
              write-ahead log is truncated, by a checkpoint the writer runs
              after a sweep that evicted something.
            </P>

            <H2 id="results">What it measures at</H2>
            <P>
              The store ships with a stress harness that drives the real
              ingestion path, batching, dedup, sealing, and retention, and
              reports what survived. The retention-churn scenario is the one to
              look at: tiny caps and a high rate, so the janitor is evicting the
              whole time. Both columns below are the same command, run before
              and after the change:
            </P>
            <Code>{`go run ./cmd/logstore-stress -scenario retention-churn -duration 60s`}</Code>
            <P>Every number in the table is what that command prints.</P>
            <FigResults />
            <div className="not-prose my-8 overflow-x-auto rounded-xl bg-sand-100 p-4 sm:p-6">
              <table className="w-full min-w-[420px] text-sm">
                <thead>
                  <tr className="text-left text-base-500">
                    <th className="pb-3 font-medium">retention-churn, 60 s</th>
                    <th className="pb-3 text-right font-medium">
                      row per line
                    </th>
                    <th className="pb-3 text-right font-medium">
                      sealed blocks
                    </th>
                  </tr>
                </thead>
                <tbody className="text-base-900">
                  {[
                    ["lines retained under a 5 MB cap", "34,952", "430,823"],
                    ["commit rate, lines/s", "19,998", "19,960"],
                    ["lines dropped", "0", "0"],
                    ["heap peak", "7.9 MB", "15.3 MB"],
                  ].map(([k, a, b]) => (
                    <tr key={k} className="border-t border-sand-500/10">
                      <td className="py-2.5 pr-4">{k}</td>
                      <td className="py-2.5 text-right font-mono tabular-nums text-base-500">
                        {a}
                      </td>
                      <td className="py-2.5 text-right font-mono tabular-nums">
                        {b}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <P>
              The throughput ceiling did not move. It comes from the single
              writer and SQLite&apos;s write lock, which compression does not
              change. The price is a heap peak that roughly doubles, from 7.9 MB
              to 15.3 MB: the zstd encoder and decoder state plus one unpacked
              block at a time. The synthetic corpus is also the least favourable
              case for the format, because its lines are short and the dedup
              filter is a large share of each block. The corpora in the table
              above sit between 7.5× and 22.7× against their raw text, and since
              the old layout cost 1.28× raw text, that is 9.6× more history per
              byte of cap on Postgres, 16× on nginx as captured and about 14×
              with realistic request timing, and up to 29× on synthetic
              application logs. Postgres and nginx are the two fully real Docker
              corpora, and the title uses the lower of the two.
            </P>
            <P>
              Search over the full history got faster as a side effect. When the
              change landed, a substring search over a 77,000-line store went
              from 122 ms to 43 ms. A search walks every line. Reading those
              lines now means far fewer SQLite pages, and the page reads cost
              more than the decompression that replaced them.
            </P>

            <H2 id="inspect">Look at your own store</H2>
            <P>
              The database sits next to the config file, which is{" "}
              <Mono>/data/logs.db</Mono> in the shipped compose file. LogDeck
              holds it open in WAL mode, so open it read-only. This groups the
              sealed blocks per container and reports the ratio:
            </P>
            <Code>{`sqlite3 -readonly /data/logs.db "
  SELECT c.name,
         count(*)                                         AS blocks,
         sum(b.lines)                                     AS lines,
         sum(b.raw_bytes)                                 AS raw_bytes,
         sum(length(b.payload) + length(b.dedup_filter))  AS stored_bytes,
         round(1.0 * sum(b.raw_bytes) /
               sum(length(b.payload) + length(b.dedup_filter)), 1) AS ratio
  FROM log_blocks b JOIN containers c ON c.id = b.container_ref
  GROUP BY c.name ORDER BY raw_bytes DESC;"

# name                     blocks  lines   raw_bytes  stored_bytes  ratio
# logdeck-edge-edge-proxy-1    20   20000    1600000        149351   10.7
# ...-payments-worker-1        10   10000     810000         73655   11.0
# logdeck-demo-worker          13   13000     689000         83882    8.2`}</Code>
            <P>
              The sample rows are from the demo stack on the laptop this post
              was written on. Anything still in <Mono>log_lines</Mono> is the
              hot tail, at most a thousand or so rows per container, and will
              seal once the container prints enough to fill a block.
            </P>

            <H2 id="compare">How this compares</H2>
            <P>
              None of this is new at scale. Loki keeps each log stream as a
              series of compressed chunks with a small label index in front of
              them, and answers a query by picking chunks by label and time and
              decompressing only those. Columnar formats like Parquet store each
              field as its own run and can delta-encode a timestamp column for
              the same reason this block does. What is different here is the
              scale and the host: one process, one SQLite file, no object store,
              no index service, on a machine that is also running the containers
              being logged. The block is a Loki chunk small enough to be a row.
            </P>
            <P>
              That difference sets the limits. LogDeck is a container tool, and
              this store exists so that History mode has something to show for a
              container that restarted an hour ago, on any host LogDeck watches.
              It keeps what fits under the caps and no more. Search is a full
              scan of decompressed blocks, so a search across a 1 GB store reads
              all of it, and there is no inverted index. It lives on the LogDeck
              host&apos;s disk, with no replication. The engine keeps writing
              its own log files exactly as before; nothing here changes where
              your logs go, it only keeps a bounded copy you can scroll back
              through after the container is gone.
            </P>

            <H2 id="not-done">What we did not do</H2>
            <P>
              There is no custom file format. SQLite page compression exists as
              extensions, but they need cgo, and a separate segment file would
              need its own crash safety and its own backup story. Blocks are
              rows in the same database, covered by the same WAL and the same
              backups, and an existing database upgrades in place: old rows seal
              as the writer gets to them, in the first batches after the
              upgrade. We also skipped an index over block contents; the summary
              columns plus the filter answer every question the store asks. A
              shared zstd dictionary is still missing, even though one trained
              per container would likely help the short-line corpora most. It is
              the obvious next step, and it can be added as a new block format
              byte without touching anything written so far.
            </P>
            <P>
              The proportions in the intro held up. The block codec is one file.
              The sequence number, the verbatim flag, the bloom filter, and the
              two-source eviction are the rest, and each exists because a
              promise the store had already made would otherwise have broken
              without anyone noticing. The store lives in{" "}
              <Mono>server/internal/logstore</Mono>. The{" "}
              <Link
                href="/docs/log-history"
                className="text-base-900 underline decoration-base-300 underline-offset-4 hover:text-accent-500"
              >
                log history docs
              </Link>{" "}
              cover the caps and what survives what.
            </P>

            <div className="mt-14 flex flex-wrap gap-2 border-t border-dashed border-base-200 pt-8">
              <a
                href="https://github.com/AmoabaKelvin/logdeck/tree/main/server/internal/logstore"
                target="_blank"
                rel="noopener noreferrer"
                className={pill.black}
              >
                Read the store on GitHub
              </a>
              <Link href="/demo" className={pill.muted}>
                Try the live demo
              </Link>
            </div>
          </div>
          <aside className="hidden lg:block lg:pt-14">
            <Toc items={sections} />
          </aside>
        </Wrapper>
      </article>
    </div>
  );
}
