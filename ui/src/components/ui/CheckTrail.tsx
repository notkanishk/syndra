"use client";

import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Ticks } from "@/components/ui/TokenBlock";
import { ARRIVE_MS, BREATHE_MS, FLASH_MS, LIGHT_LEAD_MS, SETTLE_MS, stillMotion } from "@/lib/motion";

export interface TrailCheck {
  tone: StatusTone;
  /** The check's sentence; backticks are mono. */
  sentence: string;
  /** Faint lines under it: why, or what to do. */
  faint?: string[];
  /** The way on, a violet link ending in →. */
  link?: { label: string; href: string };
  /** Anything drawn under the check (check 5's token). */
  children?: ReactNode;
  /** A ring, not a dot: the check reads Zitadel's last answer, not a fresh one. */
  ring?: boolean;
}

/**
 * The six-check trail (A-11, A-14 §1, S3): one dot and one sentence a check, in
 * the order a token is made, each step joined to the next by a quiet line.
 * `compact` is debug mode's form: details only where a check didn't pass.
 *
 * `label` makes it a route rather than a set of checks (A-24 §5, a change's
 * trail): the list is named by it, and each step is read as "Step n:" with no
 * pass or fail word, because its own sentence already says how far it got. A
 * route's current step is the one it has reached, its first open (`ring`) one
 * past a settled one (`aria-current="step"`): a route drawn before its answer
 * has seen nothing, so it has reached nothing.
 *
 * `light` is a change's own (A-25 moment 5, "the lamp carried home"): a soft
 * lamp centred on the dots, carried down the steps already seen in one motion
 * (each move a flash, the next setting out a beat later, so it slows at each
 * dot and never stops), the hairline lit where it is, and each read dot's own
 * glow opening once as the lamp reaches it. It ends on the first open step,
 * where it waits and breathes, or, when no step is open, goes out a beat after
 * the last. A step that settles while it waits sends it on, the dot filling
 * `--light-lead` behind it. It walks over what is already true: a dot is filled
 * by its step, never by the lamp. It goes out when `light` turns false (the
 * trail stopped looking). Undefined, there is no light at all: a check's
 * trail, a removal's, a route not yet answered or not yet sent.
 */
export function CheckTrail({
  checks,
  compact = false,
  label,
  light,
}: {
  checks: TrailCheck[];
  compact?: boolean;
  label?: string;
  light?: boolean;
}) {
  const lit = light !== undefined;
  // The step a route has reached: its current step, whether or not a light rests on it. Reached only past a
  // step that was seen: a route with nothing settled yet (drawn before its answer) has reached nothing.
  const first = label !== undefined || lit ? checks.findIndex((check) => check.ring) : -1;
  const open = first > 0 ? first : -1;
  // Where its walk ends: the first open step, or the last step once the change is home.
  const walk = useTrailWalk(lit, Boolean(light), first >= 0 ? first : checks.length - 1, first < 0, checks.map((check) => !check.ring));
  return (
    <Lit lit={lit} lamp={<TrailLamp walk={walk} steps={checks.length} />}>
    <ol className="flex flex-col" aria-label={label ?? "Checks"}>
      {checks.map((check, i) => {
        const details = !compact || check.tone !== "healthy";
        return (
          <li key={i} aria-current={i === open ? "step" : undefined} className="grid grid-cols-[14px_1fr] gap-x-3">
            <span className="flex flex-col items-center">
              {/* Under a light, a dot that fills does so a beat behind it (the light leads, A-25 §2), and a read
                  dot answers the lamp once, with its own glow. It stands in front of the lit line. */}
              <StatusDot
                tone={check.tone}
                ring={check.ring}
                glow={lit && walk.answered.includes(i)}
                className={`mt-[8px] ${lit ? "light-lead relative" : ""}`}
              />
              {i < checks.length - 1 && <span aria-hidden className="mt-1.5 w-px flex-1 bg-line" />}
            </span>
            <div className={`flex min-w-0 flex-col gap-1 ${i < checks.length - 1 ? "pb-3.5" : ""}`}>
              <span className="text-body leading-[1.5] text-ink [text-wrap:pretty]">
                <span className="sr-only">{label ? `Step ${i + 1}: ` : `Check ${i + 1}, ${TONE_WORD[check.tone]}: `}</span>
                <Ticks text={check.sentence} />
              </span>
              {details &&
                check.faint?.map((line) => (
                  <span key={line} className="text-small leading-[1.5] text-faint [text-wrap:pretty]">
                    <Ticks text={line} />
                  </span>
                ))}
              {details && check.link && (
                <a href={check.link.href} target="_blank" rel="noreferrer" className="text-small font-semibold text-accent-text link-hover">
                  {check.link.label} →
                </a>
              )}
              {check.children && <div className="mt-1.5">{check.children}</div>}
            </div>
          </li>
        );
      })}
    </ol>
    </Lit>
  );
}

/**
 * A lit trail's own box: the lamp and its line first (so the dots, later, stand in front of the line), then
 * the list, which sits at the box's top left. A trail with no light is its list alone, as it always was.
 */
function Lit({ lit, lamp, children }: { lit: boolean; lamp: ReactNode; children: ReactNode }) {
  if (!lit) return children;
  return (
    <div className="relative" data-trail>
      {lamp}
      {children}
    </div>
  );
}

/** A step's dot: its centre, under the dot's `mt-[8px]` and half its `size-1.5`, in its 14px column. */
const DOT_CENTRE = 11;
const DOT_X = 7;
/** The lamp's reach from the dot (A-25 moment 5): over the sentence, and up and down; on the left, the room. */
const REACH = { across: 240, up: 56 };
/** How far inside its room's edges the lamp is spent. */
const MARGIN_KEPT = 4;
/** A beat: one move's flash less a settle, so the next move sets out before the last has landed (no new token). */
export const BEAT_MS = FLASH_MS - SETTLE_MS;

interface TrailWalk {
  lit: boolean;
  still: boolean;
  /** How many moves have set out: the lamp is carried by their sum. */
  moves: number;
  /** The read steps whose dot has answered the lamp. */
  answered: number[];
  going: boolean;
  breathing: boolean;
}

/**
 * The lamp's walk, in tokens. It swells in on the first step; at `ARRIVE_MS` and
 * every `BEAT_MS` after, it sets out for the next step (until `end`), and the
 * read step it is on answers `LIGHT_LEAD_MS` behind it. On `end`: if the change
 * is `home` it goes out a beat later; if not, it breathes a flash later, until a
 * read sends it on. It runs once; still (reduced motion) there is no walk, no
 * answer and no breath, and home it goes one breath later.
 */
function useTrailWalk(lit: boolean, on: boolean, end: number, home: boolean, read: boolean[]): TrailWalk {
  const still = stillMotion();
  // The beats. `reached` is the dot the lamp has come to; each time it comes to one short of the end it sets
  // out for the next, and the next beat is a beat after that move set out. The first beat is after the swell.
  // A read that moves the end on later sets the lamp out at once and walks on one beat at a time, never in a jump.
  const [reached, setReached] = useState(-1);
  const moves = still ? end : reached < 0 ? 0 : Math.min(reached + 1, end);
  const setOut = useRef(0);
  const before = useRef(moves);
  useEffect(() => {
    if (moves > before.current) setOut.current = Date.now();
    before.current = moves;
  }, [moves]);
  useEffect(() => {
    if (!lit || still || reached >= end) return;
    const wait = reached < 0 ? ARRIVE_MS : Math.max(0, setOut.current + BEAT_MS - Date.now());
    const next = setTimeout(() => setReached((r) => r + 1), wait);
    return () => clearTimeout(next);
  }, [lit, still, reached, end]);
  const walked = still || reached >= end;

  // Each read step answers once, a beat behind the lamp reaching it.
  const upTo = still ? end : reached;
  const [answered, setAnswered] = useState<number[]>([]);
  const dueKey = read
    .map((r, i) => (r && i <= upTo && !answered.includes(i) ? i : -1))
    .filter((i) => i >= 0)
    .join(",");
  useEffect(() => {
    if (!lit || still || !dueKey) return;
    const steps = dueKey.split(",").map(Number);
    const timer = setTimeout(() => setAnswered((a) => [...a, ...steps.filter((i) => !a.includes(i))]), LIGHT_LEAD_MS);
    return () => clearTimeout(timer);
  }, [lit, still, dueKey]);

  // Home: out a beat after the last step (one breath when still).
  const [rested, setRested] = useState(false);
  useEffect(() => {
    if (!lit || !home || !walked || rested) return;
    const timer = setTimeout(() => setRested(true), still ? BREATHE_MS : BEAT_MS);
    return () => clearTimeout(timer);
  }, [lit, home, walked, rested, still]);

  // Waiting on a step not yet read: it breathes once it has landed there, and only then.
  const [landed, setLanded] = useState<number | null>(null);
  useEffect(() => {
    if (!lit || still || !walked || home) return;
    const timer = setTimeout(() => setLanded(end), FLASH_MS);
    return () => clearTimeout(timer);
  }, [lit, still, walked, home, end]);

  const going = on && !(home && rested);
  return { lit, still, moves, answered, going, breathing: going && !home && landed === end };
}

/**
 * The lamp and the lit line (A-25 moment 5), in the list, at the first dot. Each
 * is carried by nested moves, one `transform` each of its own step's height on
 * flash and tint's even curve, so the two keep time and stay on the compositor.
 * The lamp's shape is measured once against its room (the result's card,
 * `data-trail-room`), at the walk's tightest steps, and again only if the card
 * or the list changes size, so it is never resized step by step and is spent
 * inside the card's inner box (rule 1).
 */
function TrailLamp({ walk, steps }: { walk: TrailWalk; steps: number }) {
  const self = useRef<HTMLSpanElement>(null);
  const pool = useRef<HTMLSpanElement>(null);
  const [geo, setGeo] = useState<{ y: number; strides: number[]; length: number; left: number; right: number; up: number; down: number } | null>(
    null,
  );
  useLayoutEffect(() => {
    // The trail's box (the lamp's parent), whose list sits at its top left.
    const list = self.current?.parentElement;
    if (!list) return;
    const room = list.closest<HTMLElement>("[data-trail-room]") ?? list;
    const measure = () => {
      const dots = [...list.querySelectorAll<HTMLElement>(":scope > ol > li")].map((li) => li.offsetTop + DOT_CENTRE);
      if (dots.length === 0) return;
      const a = list.getBoundingClientRect();
      const r = room.getBoundingClientRect();
      const top = r.top + room.clientTop - a.top;
      const left = r.left + room.clientLeft - a.left;
      const reach = (n: number) => Math.max(0, n - MARGIN_KEPT);
      const next = {
        y: dots[0],
        strides: dots.slice(1).map((y, j) => y - dots[j]),
        length: dots[dots.length - 1] - dots[0],
        left: Math.min(REACH.across, reach(DOT_X - left)),
        right: Math.min(REACH.across, reach(left + room.clientWidth - DOT_X)),
        up: Math.min(REACH.up, reach(dots[0] - top)),
        down: Math.min(REACH.up, reach(top + room.clientHeight - dots[dots.length - 1])),
      };
      setGeo((was) => (was && JSON.stringify(was) === JSON.stringify(next) ? was : next));
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const watch = new ResizeObserver(measure);
    watch.observe(list);
    if (room !== list) watch.observe(room);
    return () => watch.disconnect();
  }, [steps]);

  // The breath, only while it waits: from full, and when it ends it ends from where it is, easing to full
  // over the pool's own settle, never with a jump (the Web Animations API, so its strength can be read).
  useEffect(() => {
    const el = pool.current;
    if (!walk.breathing || !el || typeof el.animate !== "function") return;
    const easing = getComputedStyle(el).getPropertyValue("--ease-breathe").trim() || "ease-in-out";
    const breath = el.animate({ opacity: [1, 0.5] }, { duration: BREATHE_MS, direction: "alternate", iterations: Infinity, easing });
    return () => {
      const now = getComputedStyle(el).opacity;
      breath.cancel();
      el.style.opacity = now;
      void el.offsetWidth;
      el.style.opacity = "";
    };
  }, [walk.breathing]);

  const vars = {
    "--trail-x": `${DOT_X}px`,
    "--trail-y": `${geo?.y ?? DOT_CENTRE}px`,
    "--trail-length": `${geo?.length ?? 0}px`,
    "--reach-left": `${geo?.left ?? 0}px`,
    "--reach-right": `${geo?.right ?? 0}px`,
    "--reach-up": `${geo?.up ?? 0}px`,
    "--reach-down": `${geo?.down ?? 0}px`,
  } as CSSProperties;
  // The moves, nested: the outermost is the first. Each that has set out carries all inside it by its step.
  const carried = (inside: ReactNode) =>
    (geo?.strides ?? []).reduceRight<ReactNode>(
      (within, stride, j) => (
        <span data-move={j} className="trail-move" style={j < walk.moves ? { transform: `translateY(${stride}px)` } : undefined}>
          {within}
        </span>
      ),
      inside,
    );
  const state = {
    "data-going": walk.going ? "" : undefined,
    // Carried: the moves glide. Still, they are where they are from the first frame.
    "data-carried": walk.still ? undefined : "",
  };
  return (
    <>
      <span
        ref={self}
        aria-hidden
        data-trail-light
        data-moves={walk.moves}
        data-breathing={walk.breathing ? "" : undefined}
        {...state}
        className="trail-light pointer-events-none"
        style={vars}
      >
        {carried(
          <span ref={pool} className="trail-pool">
            <span data-quarter="tl" />
            <span data-quarter="tr" />
            <span data-quarter="bl" />
            <span data-quarter="br" />
          </span>,
        )}
      </span>
      <span aria-hidden data-trail-path {...state} className="trail-path" style={vars}>
        {carried(<span className="trail-thread" />)}
      </span>
    </>
  );
}

const TONE_WORD: Record<StatusTone, string> = {
  healthy: "passes",
  neutral: "nothing to check",
  accent: "noted",
  warn: "needs a look",
  danger: "fails",
};
