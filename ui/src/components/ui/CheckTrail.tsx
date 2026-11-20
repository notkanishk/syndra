"use client";

import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Ticks } from "@/components/ui/TokenBlock";
import { BREATHE_MS, FLASH_MS, stillMotion } from "@/lib/motion";

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
 * `light` is a change's own (A-25 moment 5): a soft light that walks the
 * change home. It appears on the first step and walks down the steps already
 * seen, one a flash, to the first open one, where it waits and breathes; a step
 * that settles while it waits moves it on, the dot filling `--light-lead`
 * behind it. When no step is open it rests one breath on the last, and goes
 * out; it also goes out when `light` turns false (the trail stopped looking).
 * It walks over what is already true: the dots it passes are filled by the
 * steps, never by the light. Undefined, there is no light at all: a check's
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
  return (
    <ol className={`flex flex-col ${lit ? "relative" : ""}`} aria-label={label ?? "Checks"}>
      {/* Where its walk ends: the first open step, or the last step once the change is home. */}
      {lit && <TrailLight end={first >= 0 ? first : checks.length - 1} home={first < 0} on={Boolean(light)} />}
      {checks.map((check, i) => {
        const details = !compact || check.tone !== "healthy";
        return (
          <li key={i} aria-current={i === open ? "step" : undefined} className="grid grid-cols-[14px_1fr] gap-x-3">
            <span className="flex flex-col items-center">
              {/* Under a light, each dot fills a beat behind it: the light leads (A-25 §2). */}
              <StatusDot tone={check.tone} ring={check.ring} className={`mt-[8px] ${lit ? "light-lead" : ""}`} />
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
  );
}

/** Where the light rests on a step: its dot's centre, under the dot's `mt-[8px]` and half its `size-1.5`. */
const DOT_CENTRE = 11;
/** The pool (A-25 moment 5): at most 280 by 96. */
const POOL = { width: 280, height: 96 };
/** How far inside its room's sides the pool is spent. */
const MARGIN_KEPT = 4;

/**
 * The change's light: one pool on a step's dot, in its room's layer (the
 * result's card, `data-trail-room`, which is `isolate`), under every line in
 * the card. It is never wider or taller than the card's inner box allows
 * on the step it is on, so its closest-side falloff is spent inside the card's
 * edge (rule 1); in the card, 280 by 96 on every step.
 *
 * Its walk home runs once, from the first step to `end`, one step a flash on
 * tint's curve. After it, a step that settles moves it at once. Still (reduced
 * motion), there is no walk: it is on `end` from the first frame. When the
 * change is `home` it rests one breath on the last step and goes out; it goes
 * out at once when `on` turns false. Where it rested last it stays, to go out
 * there.
 */
function TrailLight({ end, home, on }: { end: number; home: boolean; on: boolean }) {
  const still = stillMotion();
  const [walk, setWalk] = useState(() => ({ step: still ? end : 0, done: still || end === 0 }));
  // The walk reads where it ends as it goes, so only its own steps drive it.
  const until = useRef(end);
  useEffect(() => {
    until.current = end;
  });
  useEffect(() => {
    if (walk.done) return;
    if (walk.step >= until.current) return setWalk({ step: walk.step, done: true });
    const next = setTimeout(() => setWalk({ step: walk.step + 1, done: false }), FLASH_MS);
    return () => clearTimeout(next);
  }, [walk]);
  const at = walk.done ? end : walk.step;

  // Home: one breath on the last step once it has arrived (its last move is a flash), then out.
  const [rested, setRested] = useState(false);
  useEffect(() => {
    if (!home || !walk.done || rested) return;
    const timer = setTimeout(() => setRested(true), still ? BREATHE_MS : FLASH_MS + BREATHE_MS);
    return () => clearTimeout(timer);
  }, [home, walk.done, rested, still]);

  // Measured from its own list (its parent), which is laid out before it is.
  const self = useRef<HTMLSpanElement>(null);
  const [pool, setPool] = useState<{ left: number; top: number; width: number; height: number } | null>(null);
  useLayoutEffect(() => {
    const el = self.current?.parentElement;
    if (!el) return;
    const measure = () => {
      const step = el.querySelectorAll<HTMLElement>(":scope > li")[at];
      if (!step) return;
      // The room's inner box, from the list's own top left.
      const room = el.closest<HTMLElement>("[data-trail-room]") ?? el;
      const a = el.getBoundingClientRect();
      const r = room.getBoundingClientRect();
      const top = r.top + room.clientTop - a.top;
      const left = r.left + room.clientLeft - a.left + MARGIN_KEPT;
      const bottom = top + room.clientHeight;
      const right = left - 2 * MARGIN_KEPT + room.clientWidth;
      const centre = step.offsetTop + DOT_CENTRE;
      const height = Math.max(0, Math.min(POOL.height, 2 * Math.min(centre - top, bottom - centre)));
      setPool({ left, top: centre - height / 2, width: Math.max(0, Math.min(POOL.width, right - left)), height });
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const watch = new ResizeObserver(measure);
    watch.observe(el);
    return () => watch.disconnect();
  }, [at]);

  // It appears where it is, on the first step, and travels only once it has left it: never sliding in.
  const glide = !still && at > 0;
  const going = on && !(home && rested);
  return (
    <span
      ref={self}
      aria-hidden
      data-trail-light
      data-step={at}
      data-going={going ? "" : undefined}
      // Its breath (the licensed loop, still happening); held where it is while the light goes out, so the
      // fade starts from the strength it had. Neither walks, travels nor breathes when still.
      data-breathe={still ? undefined : ""}
      className={`trail-light pointer-events-none absolute left-0 top-0 ${glide ? "trail-light-glide" : ""}`}
      style={pool ? { transform: `translate(${pool.left}px, ${pool.top}px)`, width: pool.width, height: pool.height } : { width: 0, height: 0 }}
    />
  );
}

const TONE_WORD: Record<StatusTone, string> = {
  healthy: "passes",
  neutral: "nothing to check",
  accent: "noted",
  warn: "needs a look",
  danger: "fails",
};
