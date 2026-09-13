"use client";

import { useEffect, useState } from "react";

import { Box, Defs, Label, Wire } from "./diagram";

// The write path, animated: lines land in the hot table one batch at a time,
// and every full run of five (standing in for 1,000) seals into one block.
// step counts ticks; the layout below derives everything from it.

const rowsPerBlock = 5;
const maxBlocks = 3;
const lastStep = maxBlocks * (rowsPerBlock + 1);
const tickMs = 550;

const rowSlots = Array.from({ length: rowsPerBlock }, (_, i) => 58 + i * 17);
const blockSlots = Array.from({ length: maxBlocks }, (_, i) => 58 + i * 28);

export function FigPipeline() {
  const [step, setStep] = useState(0);
  const [run, setRun] = useState(0);

  // Every replay restarts the tick; run is the dependency on purpose.
  useEffect(() => {
    const id = setInterval(() => {
      setStep((s) => {
        if (s >= lastStep) {
          clearInterval(id);
          return s;
        }
        return s + 1;
      });
    }, tickMs);
    return () => clearInterval(id);
  }, [run]);

  const blocks = Math.min(maxBlocks, Math.floor(step / (rowsPerBlock + 1)));
  const inCycle = step - blocks * (rowsPerBlock + 1);
  const sealing = inCycle === rowsPerBlock && blocks < maxBlocks;
  const hot = blocks >= maxBlocks ? 0 : Math.min(inCycle, rowsPerBlock);
  const done = step >= lastStep;

  return (
    <figure className="not-prose my-8 rounded-xl bg-base-100 p-5 sm:p-8">
      <div className="overflow-x-auto">
        <div style={{ minWidth: 600 }}>
          <svg viewBox="0 0 760 244" className="h-auto w-full">
            <title>The write path from engine to sealed blocks</title>
            <Defs />
            <Label
              x={375}
              y={22}
              text="single writer"
              tone="accent"
              anchor="middle"
              strong
            />
            <Label
              x={375}
              y={36}
              text="batch · seal · retain"
              anchor="middle"
            />

            <Box
              x={0}
              y={60}
              w={120}
              h={64}
              title="ENGINE"
              sub="LIVE + BACKFILL"
            />
            <Wire d="M120 92 H158" />
            <Box x={160} y={60} w={110} h={64} title="QUEUE" sub="8,192 MSGS" />
            <Wire d="M270 92 H308" />
            <Box
              x={310}
              y={60}
              w={130}
              h={64}
              title="WRITER"
              sub="ONE GOROUTINE"
              tone="accent"
            />
            <Wire d="M440 92 H478" />

            <Box x={480} y={30} w={120} h={124} inset={false}>
              <Label
                x={540}
                y={48}
                text="log_lines"
                anchor="middle"
                tone="dark"
              />
              {rowSlots.map((y, i) => (
                <rect
                  key={y}
                  x={492}
                  y={y}
                  width={96}
                  height={11}
                  className={
                    i < hot
                      ? "fill-base-400 stroke-base-400"
                      : "fill-white stroke-base-300"
                  }
                  strokeDasharray={i < hot ? undefined : "2 2"}
                  strokeWidth="0.8"
                  style={{ transition: "fill 200ms" }}
                />
              ))}
              <Label x={540} y={150} text="hot · ≤ 1,000" anchor="middle" />
            </Box>

            <Wire d="M600 92 H638" tone={sealing ? "accent" : "gray"} />
            <Label
              x={619}
              y={84}
              text="seal"
              anchor="middle"
              tone={sealing ? "accent" : "gray"}
            />

            <Box x={640} y={30} w={120} h={124} inset={false}>
              <Label
                x={700}
                y={48}
                text="log_blocks"
                anchor="middle"
                tone="dark"
              />
              {blockSlots.map((y, i) => (
                <g key={y}>
                  <rect
                    x={652}
                    y={y}
                    width={96}
                    height={22}
                    className={
                      i < blocks
                        ? "fill-base-200 stroke-base-400"
                        : "fill-white stroke-base-300"
                    }
                    strokeDasharray={i < blocks ? undefined : "2 2"}
                    strokeWidth="0.8"
                    style={{ transition: "fill 200ms" }}
                  />
                  {i < blocks && (
                    <text
                      x={700}
                      y={73}
                      textAnchor="middle"
                      className="fill-base-600 font-mono text-[9px]"
                      style={{ letterSpacing: "0.1em" }}
                      transform={`translate(0 ${y - 58})`}
                    >
                      1,000 LINES
                    </text>
                  )}
                </g>
              ))}
              <Label x={700} y={150} text="1 row = 1,000" anchor="middle" />
            </Box>

            <Wire d="M700 154 V188 H560" />
            <Label
              x={550}
              y={192}
              text="retention evicts whole blocks, oldest first"
              anchor="end"
            />
            <Wire d="M540 154 V214 H700 V154" arrow={false} />
            <Label
              x={620}
              y={232}
              text="a query merges both, newest first"
              anchor="middle"
            />
          </svg>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <figcaption className="max-w-xl text-pretty text-sm text-base-500">
          One goroutine owns every write. Lines land in the hot table inside the
          batch transaction that makes them queryable, and between batches the
          writer folds each full run of 1,000 into a single compressed row.
        </figcaption>
        <button
          type="button"
          onClick={() => {
            setStep(0);
            setRun((r) => r + 1);
          }}
          className="shrink-0 border border-base-400 bg-white px-3 py-1.5 font-mono text-[11px] tracking-[0.1em] text-base-700 hover:border-accent-500 hover:text-accent-600"
        >
          {done ? "REPLAY ↻" : "PLAYING…"}
        </button>
      </div>
    </figure>
  );
}
