import { useId } from "react";

import { dayGap, dayNoValue, dayValue, sparkLabel } from "@/components/usage/copy";
import type { StatsUnit } from "@/lib/queries/useStats";
import { dayWords, formatValue, type Plot } from "@/lib/usage";

/**
 * One series over the last 30 makerspace days (dataviz: change over time is a
 * line; a headline number carries the total).
 *
 * The baseline is zero, so a line never exaggerates. A day nobody could
 * observe is a hatched band and the line BREAKS there: no point is drawn, so a
 * gap can never read as a zero. A counter's unobserved-nothing day is a real
 * zero and is drawn as one; a gauge's day with no new sample is joined over.
 * Colours are tokens (accent for the series, `--chart-hatch` for the gap), so
 * both themes are the stylesheet's.
 */
const W = 132;
const H = 32;
const PAD_X = 3;
const PAD_Y = 4;

export function Spark({
  plot,
  unit,
  what,
}: {
  plot: Plot;
  unit: StatsUnit;
  /** What the line is, for the screen reader; the numbers are beside it. */
  what: string;
}) {
  const id = useId();
  const n = plot.cells.length;
  const step = (W - 2 * PAD_X) / (n - 1);
  const x = (i: number) => PAD_X + i * step;
  const max = Math.max(1, ...plot.cells.map((c) => c.value ?? 0));
  const y = (v: number) => H - PAD_Y - (v / max) * (H - 2 * PAD_Y);

  // Runs of cells with a value, broken only by a gap.
  const runs: Array<Array<[number, number]>> = [[]];
  plot.cells.forEach((c, i) => {
    if (c.gap) {
      if (runs[runs.length - 1].length) runs.push([]);
    } else if (c.value !== undefined) runs[runs.length - 1].push([x(i), y(c.value)]);
  });
  const last = [...plot.cells.keys()].reverse().find((i) => plot.cells[i].value !== undefined);

  return (
    <svg role="img" aria-label={sparkLabel(what, plot.gapRuns.length > 0)} viewBox={`0 0 ${W} ${H}`} width={W} height={H} className="shrink-0 overflow-visible">
      <defs>
        <pattern id={`${id}-gap`} width="5" height="5" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <line x1="0" y1="0" x2="0" y2="5" stroke="var(--chart-hatch)" strokeWidth="1.5" />
        </pattern>
      </defs>
      <line x1="0" x2={W} y1={H - PAD_Y} y2={H - PAD_Y} stroke="var(--line-strong)" strokeWidth="1" />
      {plot.gapRuns.map(([a, b]) => (
        <rect key={a} data-gap x={Math.max(0, x(a) - step / 2)} width={Math.min(W, x(b) + step / 2) - Math.max(0, x(a) - step / 2)} y="0" height={H - PAD_Y} fill={`url(#${id}-gap)`} />
      ))}
      {runs
        .filter((run) => run.length > 1)
        .map((run) => (
          <polyline key={run[0][0]} data-line points={run.map(([px, py]) => `${px},${py}`).join(" ")} fill="none" stroke="var(--accent)" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" />
        ))}
      {last !== undefined && <circle cx={x(last)} cy={y(plot.cells[last].value!)} r="2.5" fill="var(--accent)" stroke="var(--surface-1)" strokeWidth="1.5" />}
      {/* A hit target a whole day wide, so the day is readable without aiming at a 2px line. */}
      {plot.cells.map((c, i) => (
        <rect key={c.day} x={x(i) - step / 2} width={step} y="0" height={H} fill="transparent">
          <title>{c.gap ? dayGap(dayWords(c.day)) : c.value === undefined ? dayNoValue(dayWords(c.day)) : dayValue(dayWords(c.day), formatValue(c.value, unit))}</title>
        </rect>
      ))}
    </svg>
  );
}
