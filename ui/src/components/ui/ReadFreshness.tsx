"use client";

import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/Button";
import { formatRelative } from "@/lib/format";

/**
 * How old the state on this screen is (design §31 A).
 *
 * Anything read from a target carries exactly one of these, and it always
 * carries an age. "Recently" is not something an operator can act on, and the
 * whole reason this component exists is that the answer was previously given
 * three different ways on three screens that sit two clicks apart: one said
 * "Served from a copy taken 4 minutes ago", one said nothing at all, and one
 * said "Reading…" for as long as the request took and then said nothing either.
 *
 * Five readings, and `truncated` is not one of them — it is orthogonal and can
 * ride along with any of the other four, because "I saw 200 of more than 200"
 * is a statement about completeness and the rest are statements about age.
 */

/** How old a read may be before it is too old to bind an identity on. */
export const STALE_AFTER_MS = 10 * 60_000;
/** Below this, the read is the present tense. */
const LIVE_UNTIL_MS = 60_000;

export type Freshness = "live" | "ageing" | "stale" | "provisional";

export interface ReadState {
  /** When the target was read. Absent means it never was. */
  readAt?: string | null;
  /**
   * Whether this came from the target just now, or from the add-on's mirror.
   * A mirror read is `provisional` at any age: the number is how old the copy
   * is, not how long ago we last succeeded.
   */
  current?: boolean;
  /** The read hit the add-on's cap. Says nothing about age. */
  truncated?: boolean;
}

/**
 * The read state of a list Syndra holds itself (bundles and who has them), where
 * the server stamps nothing: the read is when this page last got an answer, and
 * it is not current if the last attempt to get one failed.
 */
export function readStateOfQuery(q: { dataUpdatedAt: number; isError: boolean }): ReadState {
  return { readAt: q.dataUpdatedAt ? new Date(q.dataUpdatedAt).toISOString() : null, current: !q.isError };
}

/**
 * Which of the four a read is, given when it happened.
 *
 * `provisional` outranks age deliberately. A four-second-old copy of state we
 * could not reach is still a copy, and rendering it as `live` would be the one
 * mistake this whole vocabulary exists to prevent.
 */
export function classifyRead(state: ReadState, now: number = Date.now()): Freshness {
  if (state.current === false) return "provisional";
  if (!state.readAt) return "stale";
  const age = now - new Date(state.readAt).getTime();
  if (Number.isNaN(age)) return "stale";
  if (age >= STALE_AFTER_MS) return "stale";
  if (age < LIVE_UNTIL_MS) return "live";
  return "ageing";
}

/**
 * Whether an action that BINDS or DESTROYS may run against this read.
 *
 * Deliberately not "is the read fresh". §31 A draws the line at what the action
 * does, not at how old the data is: adopting an account off a list that has
 * moved hands somebody else's home directory to a member, while applying a plan
 * only joins a queue an operator can still inspect. So adoption is blocked at
 * eleven minutes and a fourteen-minute-old plan still applies, and the two must
 * not be unified — which is easiest to guarantee when only one of them ever
 * calls this.
 */
export function blocksIrreversibleAction(state: ReadState, now: number = Date.now()): boolean {
  return classifyRead(state, now) !== "live" && classifyRead(state, now) !== "ageing";
}

const TONE: Record<Freshness, string> = {
  live: "text-faint",
  ageing: "text-faint",
  stale: "text-warn-text",
  provisional: "text-warn-text",
};

/**
 * The strip itself.
 *
 * `subject` names what was read, so the sentence works on a page with more than
 * one of these ("The account list", "This plan"). `onRefresh` is offered only
 * where re-reading is the operator's next move — a provisional read has nothing
 * to refresh to, since the target is the thing that is not answering.
 */
export function ReadFreshness({
  state,
  subject = "This",
  onRefresh,
  refreshing = false,
  className = "",
}: {
  state: ReadState;
  subject?: string;
  onRefresh?: () => unknown;
  refreshing?: boolean;
  className?: string;
}) {
  // The age is a function of now, so it is computed on the client only — the
  // server and the browser do not share a clock, and a freshness label that
  // flickers on hydration is worse than one that appears a frame late.
  const [freshness, setFreshness] = useState<Freshness | null>(null);
  useEffect(() => {
    const tick = () => setFreshness(classifyRead(state));
    tick();
    // Re-classified on a timer, because this is the one label in the product
    // whose meaning changes while nobody touches the page: a read that was
    // fine when the page loaded is what an operator adopts from ten minutes
    // later.
    const id = setInterval(tick, 30_000);
    return () => clearInterval(id);
  }, [state.readAt, state.current]); // eslint-disable-line react-hooks/exhaustive-deps

  // S17. Only a read the reader asked for blooms. Pressing records the age on
  // screen; the next read that differs from it is the answer to the press. A
  // background read finds nothing recorded and lands quietly, and a press whose
  // read never arrives is forgotten shortly after it settles, so it cannot
  // claim the next background read.
  const pressedAt = useRef<string | null | undefined>(undefined);
  const [pressing, setPressing] = useState(false);
  const [landed, setLanded] = useState<string | null>(null);
  useEffect(() => {
    if (pressedAt.current === undefined || (state.readAt ?? null) === pressedAt.current) return;
    pressedAt.current = undefined;
    setLanded(state.readAt ?? null);
    const t = setTimeout(() => setLanded(null), 1000);
    return () => clearTimeout(t);
  }, [state.readAt]);
  function press() {
    pressedAt.current = state.readAt ?? null;
    setPressing(true);
    let result: unknown;
    try {
      result = onRefresh?.();
    } catch {
      // The read failed to start: the button comes back, nothing blooms.
    }
    Promise.resolve(result)
      .catch(() => {})
      .finally(() => {
        setPressing(false);
        setTimeout(() => {
          pressedAt.current = undefined;
        }, 250);
      });
  }

  const reading = freshness ?? classifyRead(state, new Date(state.readAt ?? 0).getTime());
  const age = state.readAt ? formatRelative(state.readAt) : null;

  const blooming = landed !== null && landed === state.readAt && (reading === "live" || reading === "ageing");

  return (
    <div className={`flex flex-wrap items-center gap-x-2 gap-y-1 text-[12.5px] ${className}`}>
      {/* A dot for every reading, not only the healthy one. It is the thing
          the eye reaches before the sentence, and its absence on three of the
          four readings made "no dot" mean both "ageing" and "stale".
          `provisional` is a ring rather than a fill, because it is the one
          reading that is neither healthy nor broken: the read answered, and
          the target could not confirm it. */}
      <span
        aria-hidden
        data-freshness-dot
        className={`size-1.5 flex-none rounded-pill ${blooming ? "bloom " : ""}${
          reading === "live" || reading === "ageing"
            ? "bg-healthy"
            : reading === "stale"
              ? "bg-warn"
              : "border border-warn"
        }`}
      />
      <span key={blooming ? landed : "read"} className={`${TONE[reading]}${blooming ? " tint-in" : ""}`}>
        {sentence(reading, subject, age, Boolean(onRefresh))}
      </span>
      {state.truncated && (
        // Its own clause, never folded into the age. A capped read is complete
        // about what it saw and silent about the rest, and an operator who
        // reads it as "that is everything" concludes an absence from a list
        // that was cut off.
        <span className="text-faint">· not the whole list — only the first part was read</span>
      )}
      {onRefresh && reading === "stale" && (
        <Button variant="outline" size="sm" onClick={press} isPending={refreshing || pressing}>
          {refreshing || pressing ? "Reading" : "Read again"}
        </Button>
      )}
    </div>
  );
}

function sentence(
  reading: Freshness,
  subject: string,
  age: string | null,
  canRefresh: boolean,
): string {
  switch (reading) {
    case "live":
      return `${subject} was read just now`;
    case "ageing":
      return `${subject} was read ${age}`;
    case "stale": {
      // Always the way out. The button is the way out when there is one;
      // otherwise the sentence has to say it, or the reader is left with a
      // warning and no move.
      const wayOut = canRefresh ? "" : " — reload the page to read it again";
      return age
        ? `${subject} was read ${age} — too old to act on${wayOut}`
        : `${subject} has not been read yet${wayOut}`;
    }
    case "provisional":
      // Never "unreachable" alone. The useful half is that what is on screen is
      // real and dated, not that the network is down.
      return age
        ? `Could not reach the connected system — this is the last state seen, ${age}`
        : "Could not reach the connected system, and there is no earlier copy to show";
  }
}
