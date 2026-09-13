import { Box, Defs, Fig, Label, T, Wire } from "./diagram";

export { FigPipeline } from "./pipeline";

// Every number drawn here was measured against the real codec; the post
// names the corpora.

// 1. What a line costs when it is one SQLite row.
export function FigRowCost() {
  const w = 520;
  return (
    <Fig caption="Measured across five real log corpora: a one-row-per-line table costs about 28% more disk than the log text it holds. Each row carries a header and an entry in the (container, timestamp) index.">
      <svg viewBox="0 0 760 176" className="h-auto w-full">
        <title>What one line costs as a SQLite row</title>
        <Defs />
        <Label x={0} y={14} text="one log line, as written" />
        <Box x={0} y={24} w={w} h={44} title="RAW TEXT" inset={false} />
        <Label x={w + 14} y={50} text="1.00×" tone="dark" strong />

        <Label x={0} y={112} text="the same line as a row in log_lines" />
        <Box x={0} y={122} w={w} h={44} title="RAW TEXT" inset={false} />
        <Box
          x={w}
          y={122}
          w={146}
          h={44}
          title="+28%"
          sub="HEADER + INDEX"
          tone="accent"
        />
        <Label x={w + 146 + 14} y={148} text="1.28×" tone="dark" strong />
      </svg>
    </Fig>
  );
}

// 2. Inside a block: field runs instead of interleaved rows, drawn to scale.
export function FigBlockAnatomy() {
  const rowCells = [
    { name: "TS", w: 110 },
    { name: "SEQ", w: 60 },
    { name: "ST", w: 40 },
    { name: "LV", w: 40 },
    { name: "RAW", w: 486 },
  ];
  const runCells = [
    { name: "FMT", w: 38 },
    { name: "N", w: 34 },
    { name: "TS Δ", w: 126, accent: true },
    { name: "SEQ Δ", w: 86 },
    { name: "STREAM", w: 76 },
    { name: "LEVEL", w: 70 },
    { name: "VERBATIM", w: 86 },
    { name: "LEN", w: 56 },
    { name: "BODIES", w: 156 },
  ];
  const total = 125642;
  const scale = 760 / total;
  const segs = [
    { name: "TS DELTAS", bytes: 3197, cls: "fill-accent-500" },
    { name: "OTHER RUNS", bytes: 5453, cls: "fill-base-400" },
    { name: "MESSAGE BODIES", bytes: 116989, cls: "fill-base-200" },
  ];
  const rowY = [24, 58, 92];
  let cx = 0;
  let sx = 0;
  let lx = 0;
  return (
    <Fig caption="One real block: the first 1,000 lines a postgres:17 container printed. Grouping the fields puts 1,000 timestamps next to each other, and storing them as deltas turns 31,000 bytes of timestamp text into 3,197 bytes. zstd then takes the 126 KB layout to 18.7 KB.">
      <svg viewBox="0 0 760 372" className="h-auto w-full">
        <title>Byte layout of one sealed block</title>
        <Defs />
        <Label x={0} y={14} text="as rows, fields interleaved" />
        {rowY.map((y) => {
          let rx = 0;
          return (
            <g key={y}>
              {rowCells.map((c) => {
                const el = (
                  <Box
                    key={c.name}
                    x={rx}
                    y={y}
                    w={c.w - 4}
                    h={28}
                    title={c.name}
                    inset={false}
                  />
                );
                rx += c.w + 2;
                return el;
              })}
            </g>
          );
        })}
        <Wire d="M380 128 V150" />

        <Label x={0} y={172} text="as a block, one run per field" />
        {runCells.map((c) => {
          const el = (
            <Box
              key={c.name}
              x={cx}
              y={182}
              w={c.w}
              h={32}
              title={c.name}
              inset={false}
              tone={c.accent ? "accent" : "gray"}
            />
          );
          cx += c.w + 4;
          return el;
        })}

        <Label x={0} y={246} text="bytes, to scale · 125,642 B before zstd" />
        {segs.map((s) => {
          const w = s.bytes * scale;
          const el = (
            <rect
              key={s.name}
              x={sx}
              y={256}
              width={w}
              height={26}
              className={`${s.cls} stroke-base-400`}
              strokeWidth="1"
            />
          );
          sx += w;
          return el;
        })}
        {segs.map((s) => {
          const el = (
            <g key={s.name}>
              <rect
                x={lx}
                y={296}
                width={10}
                height={10}
                className={`${s.cls} stroke-base-400`}
                strokeWidth="1"
              />
              <Label
                x={lx + 16}
                y={305}
                text={`${s.name} ${s.bytes.toLocaleString()} B`}
              />
            </g>
          );
          lx += 200;
          return el;
        })}

        <Label x={0} y={338} text="after zstd" />
        <rect
          x={0}
          y={346}
          width={18650 * scale}
          height={20}
          className="fill-base-900"
        />
        <Label
          x={18650 * scale + 12}
          y={360}
          text="18,650 B payload + 1,022 B filter, from 147,989 B of lines"
          tone="dark"
        />
      </svg>
    </Fig>
  );
}

// 3. The timestamp prefix round trip and the trailing-zero trap.
export function FigTimestampTrap() {
  return (
    <Fig caption="The engine prefix is lifted out and rebuilt on read. Go's RFC3339Nano trims trailing zeros from the fraction, which changes the bytes of about one line in ten. A fixed nine-digit layout reproduces them, and packing still checks each line against its original prefix.">
      <svg viewBox="0 0 760 292" className="h-auto w-full">
        <title>Rebuilding the timestamp prefix</title>
        <Defs />
        <Box x={0} y={0} w={760} h={44} inset={false}>
          <text
            x={380}
            y={27}
            textAnchor="middle"
            className={`${T.title} fill-base-900`}
          >
            <tspan className="fill-accent-600">
              2026-09-13T10:04:07.120000000Z
            </tspan>
            <tspan> GET /health 200 in 3ms</tspan>
          </text>
        </Box>
        <Wire d="M180 44 V76" />
        <Wire d="M580 44 V76" />
        <Box
          x={0}
          y={78}
          w={360}
          h={48}
          title="TS_NS"
          sub="1789293847120000000"
        />
        <Box
          x={400}
          y={78}
          w={360}
          h={48}
          title="BODY"
          sub='"GET /health 200 in 3ms"'
        />

        <Wire d="M180 126 V148 H580 V182" />
        <Wire d="M180 148 V182" />
        <Label
          x={380}
          y={166}
          text="rebuild on read"
          tone="accent"
          anchor="middle"
          strong
        />
        <Label
          x={380}
          y={178}
          text="format ts_ns, put the body back"
          anchor="middle"
        />

        <Box
          x={0}
          y={184}
          w={360}
          h={68}
          title="TIME.RFC3339NANO"
          sub={["2026-09-13T10:04:07.12Z", "7 BYTES SHORT ✗"]}
        />
        <Box
          x={400}
          y={184}
          w={360}
          h={68}
          title="FIXED 9-DIGIT LAYOUT"
          sub={["2026-09-13T10:04:07.120000000Z", "IDENTICAL ✓"]}
          tone="accent"
        />
        <Label
          x={0}
          y={282}
          text="pack compares the rebuilt prefix with the original bytes; a line that would not round-trip is stored whole"
          upper={false}
        />
      </svg>
    </Fig>
  );
}

// 4. The per-block bloom filter that replaces the index for dedup.
export function FigBloom() {
  const set = new Set([2, 5, 9, 10, 14, 17, 21, 23, 26, 29, 31]);
  const bits = Array.from({ length: 32 }, (_, i) => i);
  return (
    <Fig caption="Every block carries a bloom filter over its line keys: about 8 bits per line, six hash positions, a 2% false-positive rate. A zero at any position proves the line is not in the block. Only a full hit costs a decompression and an exact comparison.">
      <svg viewBox="0 0 760 262" className="h-auto w-full">
        <title>The per-block bloom filter</title>
        <Defs />
        <Label x={0} y={14} text="six hash positions" tone="accent" strong />
        <Label x={0} y={28} text="≈ 8 bits per line, 1,022 bytes per block" />

        <Box
          x={0}
          y={96}
          w={160}
          h={56}
          title="LINE KEY"
          sub="FNV64(TS, STREAM, RAW)"
        />
        <Wire d="M160 124 H198" />
        <Box x={200} y={72} w={270} h={104} inset={false}>
          <Label x={335} y={94} text="dedup filter" anchor="middle" />
          {bits.map((i) => (
            <rect
              key={i}
              x={212 + i * 7.7}
              y={110}
              width={6}
              height={22}
              className={
                set.has(i) ? "fill-base-900" : "fill-white stroke-base-400"
              }
              strokeWidth="0.8"
            />
          ))}
          <Label x={335} y={158} text="8,142 bits" anchor="middle" />
        </Box>

        <Wire d="M470 124 H505 V62 H528" tone="accent" />
        <Wire d="M470 124 H505 V186 H528" />
        <Box
          x={530}
          y={30}
          w={230}
          h={64}
          title="ANY ZERO"
          sub={["SKIP THE BLOCK", "NO I/O"]}
          tone="accent"
        />
        <Box
          x={530}
          y={154}
          w={230}
          h={64}
          title="ALL SIX SET"
          sub={["UNPACK + COMPARE", "≈ 2% FALSE POSITIVES"]}
        />
        <Label
          x={0}
          y={252}
          text="live lines are newer than anything sealed and never consult a filter; only a backfill re-read does"
          upper={false}
        />
      </svg>
    </Fig>
  );
}

// 5. Paging newest-first across the hot table and sealed blocks.
export function FigQuery() {
  const cursorX = 470;
  const hot = Array.from({ length: 14 }, (_, i) => 600 + i * 11);
  const blocks = [
    { x: 0, title: "TS_MAX < SINCE", sub: "SKIPPED", tone: "ghost" as const },
    { x: 144, title: "UNPACK", sub: "IN WINDOW", tone: "gray" as const },
    { x: 288, title: "UNPACK", sub: "IN WINDOW", tone: "gray" as const },
    { x: 432, title: "UNPACK", sub: "SPANS CURSOR", tone: "gray" as const },
    {
      x: 576,
      title: "TS_MIN > CURSOR",
      sub: "SKIPPED",
      tone: "ghost" as const,
    },
  ];
  return (
    <Fig caption="A page walks backwards from a cursor of (timestamp, sequence). A block's stored bounds say whether it can hold anything the page needs: blocks entirely newer than the cursor and blocks older than the page's window are never decompressed. The hot table and the surviving blocks are merged newest-first.">
      <svg viewBox="0 0 760 262" className="h-auto w-full">
        <title>Paging across hot lines and sealed blocks</title>
        <Defs />
        <Label x={0} y={14} text="older" />
        <Label x={760} y={14} text="newer" anchor="end" />
        <line x1="0" y1="20" x2="760" y2="20" className="stroke-base-300" />

        <Label x={0} y={46} text="log_lines · hot" tone="dark" />
        {hot.map((x) => (
          <rect
            key={x}
            x={x}
            y={54}
            width={8}
            height={22}
            className="fill-white stroke-base-300"
            strokeDasharray="2 2"
          />
        ))}
        <Label
          x={754}
          y={92}
          text="all newer than the cursor · skipped"
          anchor="end"
        />

        <Label x={0} y={126} text="log_blocks · sealed" tone="dark" />
        {blocks.map((b) => (
          <Box
            key={b.x}
            x={b.x}
            y={134}
            w={136}
            h={56}
            title={b.title}
            sub={b.sub}
            tone={b.tone}
          />
        ))}

        <Wire d={`M${cursorX} 30 V214`} tone="accent" arrow={false} />
        <Label
          x={cursorX + 8}
          y={226}
          text="cursor (ts_ns, seq)"
          tone="accent"
          strong
        />
        <Label
          x={0}
          y={252}
          text="seq survives sealing: a cursor taken while a line was hot still resolves once it is in a block"
          upper={false}
        />
      </svg>
    </Fig>
  );
}

// 6. Retention drops whole blocks.
export function FigRetention() {
  const blockX = Array.from({ length: 7 }, (_, i) => i * 84);
  const tail = Array.from({ length: 14 }, (_, i) => 600 + i * 11);
  return (
    <Fig caption="Retention compares the oldest hot line with the oldest sealed block and evicts from whichever is older. Freeing a megabyte of history usually means deleting a handful of block rows rather than thousands of line rows, and a checkpoint right after keeps the file near its cap.">
      <svg viewBox="0 0 760 166" className="h-auto w-full">
        <title>Retention evicting whole blocks</title>
        <Defs />
        <Label x={0} y={14} text="oldest" />
        <Label x={760} y={14} text="newest" anchor="end" />
        {blockX.map((x, i) => (
          <Box
            key={x}
            x={x}
            y={24}
            w={76}
            h={52}
            title={i < 2 ? "EVICTED" : "1,000"}
            sub={i < 2 ? undefined : "LINES"}
            tone={i < 2 ? "outline" : "gray"}
          />
        ))}
        {tail.map((x) => (
          <rect
            key={x}
            x={x}
            y={30}
            width={8}
            height={40}
            className="fill-white stroke-base-300"
            strokeDasharray="2 2"
          />
        ))}
        <Label x={754} y={92} text="hot tail · uncompressed" anchor="end" />
        <Wire d="M164 18 V86" tone="accent" arrow={false} />
        <Label x={170} y={98} text="stored_bytes cap" tone="accent" strong />
        <Label
          x={0}
          y={130}
          text="the cap counts what a container occupies: payload + filter per block, plus the raw bytes of hot lines"
          upper={false}
        />
        <Label
          x={0}
          y={146}
          text="sealing subtracts the raw bytes it removes and adds the payload it writes; eviction subtracts that"
          upper={false}
        />
      </svg>
    </Fig>
  );
}

// 7. Lines retained under the same cap, before and after.
export function FigResults() {
  const before = 34952;
  const after = 430823;
  const scale = 560 / after;
  return (
    <Fig caption="Stress harness, retention-churn scenario: eight containers, 1 MB per container, 5 MB total, 1.2 million lines offered in 60 seconds. Same machine, same corpus, before and after sealing. Zero lines dropped in either run.">
      <svg viewBox="0 0 760 150" className="h-auto w-full">
        <title>Lines retained under the same cap, before and after</title>
        <Defs />
        <Label x={0} y={14} text="row per line" />
        <Box x={0} y={22} w={before * scale} h={36} inset={false} />
        <Label
          x={before * scale + 12}
          y={45}
          text={`${before.toLocaleString()} lines under 5 MB`}
          tone="dark"
          strong
        />
        <Label x={0} y={92} text="sealed blocks" />
        <Box
          x={0}
          y={100}
          w={after * scale}
          h={36}
          title="12.3× MORE HISTORY IN THE SAME BYTES"
          tone="accent"
        />
        <Label
          x={after * scale + 12}
          y={123}
          text={after.toLocaleString()}
          tone="dark"
          strong
        />
      </svg>
    </Fig>
  );
}
