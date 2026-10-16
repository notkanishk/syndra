"use client";

import { useRef, useState } from "react";

import { LANES, type Highlight, type Model, type Tone } from "@/components/insights/model";
import { NodeGlyph, TONE_COLOUR, nodeSurface } from "@/components/insights/tone";
import { AnchoredPanel } from "@/components/ui/Anchored";
import { PILL } from "@/components/ui/Button";

export type Mark = "drift" | "send";

const CAP = 12;
const ROW = 46;
const TOP = 56;
const NODE_H = 38;
const W = 1000;
const LANE_W = 19.2;
const STEP = (98.4 - LANE_W) / (LANES.length - 1);
const left = (lane: number) => 0.8 + lane * STEP;

const KEYS: Array<[string, string]> = [
  ["/", "Ask, or find anything"],
  ["← →", "Along a line, following"],
  ["↑ ↓", "Up and down a lane"],
  ["P", "Pin what you follow"],
  ["?", "Show or hide these"],
  ["Esc", "Back one question"],
];

export const MARK_WORDS: Record<Mark, { tone: Tone; words: string }> = {
  drift: { tone: "danger", words: "Held in Zitadel, not given by Syndra" },
  send: { tone: "warn", words: "Waiting for you to send" },
};

/**
 * The map: four lanes drawn in HTML, with SVG only for the threads between
 * them. A node is a row in its lane; focusing one lights the threads to what
 * gives it and what it gives, and everything else steps back to 0.35.
 *
 * Nothing moves in response to data: lanes keep their A-to-Z order. A lane
 * longer than twelve shows "N more" in place, and opens by itself only when
 * the answer reaches a row that is hidden.
 */
export function LaneMap({
  model,
  highlight,
  focusId,
  count,
  unused,
  marks,
  keysOpen,
  onKeys,
  onPick,
}: {
  model: Model;
  highlight: Highlight | null;
  focusId: string | null;
  /** People holding a role, as Zitadel shows it; absent when never read. */
  count: (id: string) => number | null;
  unused: Set<string>;
  marks: Map<string, Mark[]>;
  keysOpen: boolean;
  onKeys: () => void;
  onPick: (id: string | null) => void;
}) {
  const [opened, setOpened] = useState<Set<string>>(new Set());
  const keysButton = useRef<HTMLButtonElement | null>(null);

  const lanes = LANES.map((lane, l) => {
    const all = model.lanes[lane.kind];
    const forced = all.slice(CAP).some((n) => highlight?.nodes.has(n.id) || n.id === focusId);
    const open = all.length <= CAP || opened.has(lane.kind) || forced;
    return { ...lane, l, shown: open ? all : all.slice(0, CAP), more: open ? 0 : all.length - CAP, total: all.length };
  });
  const rows = Math.max(4, ...lanes.map((x) => x.shown.length + (x.more ? 1 : 0)));
  const H = TOP + rows * ROW + 18 + 40;

  const pos = new Map<string, { l: number; y: number }>();
  const moreTop = new Map<string, number>();
  for (const lane of lanes) {
    const slots = lane.shown.length + (lane.more ? 1 : 0);
    const off = ((rows - slots) * ROW) / 2;
    lane.shown.forEach((n, i) => pos.set(n.id, { l: lane.l, y: TOP + off + i * ROW + NODE_H / 2 }));
    moreTop.set(lane.kind, TOP + off + lane.shown.length * ROW);
  }

  const xr = (l: number) => (left(l) + LANE_W) * 10;
  const xl = (l: number) => left(l) * 10;
  const draw = (a: { l: number; y: number }, b: { l: number; y: number }, rule: boolean) => {
    if (rule || a.l === b.l) {
      const x = xr(a.l);
      return `M ${x} ${a.y} C ${x + 36} ${a.y}, ${x + 36} ${b.y}, ${x} ${b.y}`;
    }
    const x1 = xr(a.l);
    const x2 = xl(b.l);
    const c = (x2 - x1) * 0.55;
    return `M ${x1} ${a.y} C ${x1 + c} ${a.y}, ${x2 - c} ${b.y}, ${x2} ${b.y}`;
  };
  const drawn = model.threads.flatMap((t) => {
    const a = pos.get(t.from);
    const b = pos.get(t.to);
    return a && b ? [{ t, d: draw(a, b, t.rule) }] : [];
  });
  const lit = drawn.filter(({ t }) => highlight?.threads.has(t.id));
  const litKey = lit.map(({ t }) => t.id).join("|");
  const marked = (mark: Mark) => Array.from(marks.values()).some((m) => m.includes(mark));

  return (
    <div className="overflow-x-auto rounded-card">
      <div
        role="group"
        aria-label="Access map"
        onClick={(e) => e.target === e.currentTarget && onPick(null)}
        className="relative min-w-[1040px] overflow-hidden rounded-card border border-line bg-surface-0"
        style={{
          height: H,
          backgroundImage: "radial-gradient(color-mix(in oklch, var(--line-strong) 70%, transparent) 1px, transparent 1.3px)",
          backgroundSize: "22px 22px",
          backgroundPosition: "11px 11px",
        }}
      >
        {lanes.map((lane) => (
          <div
            key={lane.kind}
            className="pointer-events-none absolute flex items-center gap-2"
            style={{ top: 16, left: `${left(lane.l)}%`, width: `${LANE_W}%` }}
          >
            <span className="type-label">{lane.title}</span>
            <span className="text-[12.5px] text-faint">{lane.total}</span>
            {lane.l < LANES.length - 1 && (
              <span aria-hidden className="ml-auto text-[12px] text-faint">
                →
              </span>
            )}
          </div>
        ))}

        <svg
          aria-hidden
          viewBox={`0 0 ${W} ${H}`}
          preserveAspectRatio="none"
          className="pointer-events-none absolute inset-0 h-full w-full"
        >
          {drawn.map(({ t, d }) => (
            <path
              key={t.id}
              d={d}
              fill="none"
              stroke="var(--line-strong)"
              strokeWidth={1}
              strokeLinecap="round"
              strokeDasharray={t.rule ? "4 5" : undefined}
              vectorEffect="non-scaling-stroke"
              opacity={highlight ? 0.1 : 0.55}
              className="motion-tint"
            />
          ))}
          <g key={litKey} className="tint-in">
            {lit.map(({ t, d }) => (
              <path
                key={t.id}
                d={d}
                fill="none"
                stroke={TONE_COLOUR[highlight!.threads.get(t.id)!]}
                strokeWidth={1.8}
                strokeLinecap="round"
                strokeDasharray={t.rule ? "4 5" : undefined}
                vectorEffect="non-scaling-stroke"
                opacity={0.95}
              />
            ))}
          </g>
        </svg>

        {lanes.map((lane) => (
          <div key={lane.kind} className="arrive-list">
            {lane.shown.map((node) => {
              const p = pos.get(node.id)!;
              const tone = highlight?.nodes.get(node.id);
              const people = node.kind === "role" ? count(node.id) : null;
              const name = model.nameOf(node.id);
              return (
                <div
                  key={node.id}
                  className="absolute"
                  style={{ left: `${left(lane.l)}%`, width: `${LANE_W}%`, top: p.y - NODE_H / 2 }}
                >
                  <button
                    type="button"
                    data-node={node.id}
                    aria-pressed={node.id === focusId}
                    aria-label={people != null ? `${name}, ${people} ${people === 1 ? "person" : "people"}` : name}
                    onClick={() => onPick(node.id === focusId ? null : node.id)}
                    className="motion-tint flex min-h-[44px] desktop:min-h-[38px] w-full items-center gap-2 rounded-nav border px-3 py-1.5 text-left"
                    style={nodeSurface(tone, node.id === focusId, unused.has(node.id), Boolean(highlight) && !tone)}
                  >
                    <NodeGlyph kind={node.kind} />
                    <span className="min-w-0 flex-1 text-[13px] font-semibold leading-tight">{name}</span>
                    {(marks.get(node.id) ?? []).map((mark) => (
                      <span
                        key={mark}
                        title={MARK_WORDS[mark].words}
                        className="size-[7px] flex-none rounded-pill"
                        style={{ background: TONE_COLOUR[MARK_WORDS[mark].tone] }}
                      />
                    ))}
                    {people != null && <span className="text-[12.5px] tabular-nums text-faint">{people}</span>}
                  </button>
                </div>
              );
            })}
            {lane.more > 0 && (
              <div
                className="absolute"
                style={{ left: `${left(lane.l)}%`, width: `${LANE_W}%`, top: moreTop.get(lane.kind) }}
              >
                <button
                  type="button"
                  onClick={() => setOpened((s) => new Set(s).add(lane.kind))}
                  className="motion-tint flex min-h-[44px] desktop:min-h-[38px] w-full items-center rounded-nav border border-dashed border-line-strong px-3 text-left text-[13px] text-muted hover:text-ink"
                >
                  {lane.more} more {lane.noun}
                </button>
              </div>
            )}
          </div>
        ))}

        <div className="absolute inset-x-5 bottom-3 flex flex-wrap items-center gap-x-[18px] gap-y-2">
          <span className="inline-flex items-center gap-2 text-[12.5px] text-muted">
            <span aria-hidden className="h-0.5 w-[22px] flex-none bg-line-strong" />
            Given
          </span>
          <span className="inline-flex items-center gap-2 text-[12.5px] text-muted">
            <span aria-hidden className="w-[22px] flex-none border-t-2 border-dashed border-line-strong" />
            Automatic
          </span>
          {(["drift", "send"] as const)
            .filter(marked)
            .map((mark) => (
              <span key={mark} className="inline-flex items-center gap-2 text-[12.5px] text-muted">
                <span aria-hidden className="size-[7px] rounded-pill" style={{ background: TONE_COLOUR[MARK_WORDS[mark].tone] }} />
                {MARK_WORDS[mark].words}
              </span>
            ))}
          <button
            ref={keysButton}
            type="button"
            onClick={onKeys}
            aria-expanded={keysOpen}
            className={`motion-tint ml-auto inline-flex items-center gap-2 rounded-pill border border-line bg-surface-1 text-muted hover:text-ink ${PILL.sm}`}
          >
            Keys
            <span className="type-mono text-ink">?</span>
          </button>
        </div>

        <AnchoredPanel
          anchor={keysButton}
          open={keysOpen}
          width={290}
          align="end"
          expectHeight={240}
          className="settle-in rounded-block border border-line-strong bg-surface-2 p-4 shadow-popover"
        >
          <div role="dialog" aria-label="Keys" className="flex flex-col gap-[7px]">
            <span className="type-label pb-0.5">Keys</span>
            {KEYS.map(([key, text]) => (
              <div key={key} className="flex items-baseline gap-3 text-[13px]">
                <span className="type-mono w-[54px] flex-none text-ink">{key}</span>
                <span className="text-muted">{text}</span>
              </div>
            ))}
          </div>
        </AnchoredPanel>
      </div>
    </div>
  );
}
