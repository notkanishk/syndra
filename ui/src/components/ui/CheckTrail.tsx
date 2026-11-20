"use client";

import { useLayoutEffect, useState, type ReactNode, type RefObject } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Ticks } from "@/components/ui/TokenBlock";
import { stillMotion, useGlideIndicator, type Box } from "@/lib/motion";

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
 * (`aria-current="step"`).
 *
 * `light` is a change's own (A-25 moment 5): a soft light resting on that
 * step. It moves only when the steps do, which is only on a read; each dot
 * fills `--light-lead` behind it; it goes out when no step is open (the change
 * is home) or when `light` turns false (the trail stopped looking). Undefined,
 * there is no light at all: a check's trail, a removal's, a route not yet
 * answered or not yet sent.
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
  // The step a route has reached: its current step, whether or not a light rests on it.
  const open = label !== undefined || lit ? checks.findIndex((check) => check.ring) : -1;
  const { ref, box } = useGlideIndicator<HTMLOListElement>(open);
  return (
    <ol ref={ref} className={`flex flex-col ${lit ? "relative isolate" : ""}`} aria-label={label ?? "Checks"}>
      {lit && <TrailLight trail={ref} box={box} going={Boolean(light) && open >= 0} />}
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
/** How far inside its room's side margin the pool is spent: 20px out of a dialog's 24 (A-25 §7). */
const MARGIN_KEPT = 4;

/**
 * The change's light: one pool on the open step's dot, the trail's own layer,
 * under everything in it. Its room is the trail's block and that block's own
 * margins (`data-trail-room`): it reaches into the side margin to 4px short of
 * its edge, and is never taller than the block and its top and bottom margins
 * allow, so its closest-side falloff is spent inside them (rule 1), whatever
 * sits around the block and however the panel has scrolled. Where it rested
 * last it stays, to go out there.
 */
function TrailLight({ trail, box, going }: { trail: RefObject<HTMLOListElement | null>; box: Box | null; going: boolean }) {
  const [pool, setPool] = useState<{ left: number; top: number; width: number; height: number } | null>(null);
  useLayoutEffect(() => {
    const list = trail.current;
    if (!list || !box) return;
    const margin = getComputedStyle(list.closest<HTMLElement>("[data-trail-room]") ?? list);
    const px = (value: string) => parseFloat(value) || 0;
    // From the list's top: up to its block's top margin, down through the list and its bottom margin.
    const above = -px(margin.marginTop);
    const below = list.offsetHeight + px(margin.marginBottom);
    const left = -Math.max(0, px(margin.marginLeft) - MARGIN_KEPT);
    const centre = box.top + DOT_CENTRE;
    const height = Math.max(0, Math.min(POOL.height, 2 * Math.min(centre - above, below - centre)));
    setPool({ left, top: centre - height / 2, width: Math.min(POOL.width, list.offsetWidth - left), height });
  }, [trail, box]);
  if (!pool) return null;
  const still = stillMotion();
  return (
    <span
      aria-hidden
      data-trail-light
      // Not one of the list's steps: the glide indicator's observer leaves it alone.
      data-indicator="light"
      data-going={going ? "" : undefined}
      // Its breath (the licensed loop, still happening); held where it is while the light goes out, so the
      // fade starts from the strength it had. Neither travels nor breathes when still.
      data-breathe={still ? undefined : ""}
      className={`trail-light pointer-events-none absolute left-0 top-0 ${still ? "" : "trail-light-glide"}`}
      style={{ transform: `translate(${pool.left}px, ${pool.top}px)`, width: pool.width, height: pool.height }}
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
