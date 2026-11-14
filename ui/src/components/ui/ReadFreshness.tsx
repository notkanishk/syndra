"use client";

import { createContext, useContext, useEffect, useId, useRef, useState } from "react";

import { copyLine, cooldownLine, READ_AGAIN, READING, retryingLine, STOP_WAITING, unreachableLine, type Reader } from "@/components/results/copy";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Crossfade } from "@/lib/motion";
import { formatClock, formatRelative } from "@/lib/format";
import { outcomeFromError } from "@/lib/outcome";
import { cooldownOf, STALE, stamped, useCountdown, type Stale } from "@/lib/queries/fresh";

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
/** How long a press that landed keeps its bloom: flash's 900ms, and a beat. */
const LANDED_HOLD_MS = 1000;
/** How long a press is remembered once it settles, in case its read is still on its way. */
const PRESS_FORGET_MS = 250;

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
  /** Why it is a copy, when the add-on said (feature freshness). Decides the sentence and the control, never `classifyRead`. */
  stale?: Stale;
}

/**
 * The read state of a list Syndra holds itself (bundles and who has them), where
 * the server stamps nothing: the read is when this page last got an answer, and
 * it is not current if the last attempt to get one failed.
 */
export function readStateOfQuery(q: { dataUpdatedAt: number; isError: boolean }): ReadState {
  return { readAt: q.dataUpdatedAt ? new Date(q.dataUpdatedAt).toISOString() : null, current: !q.isError };
}

/** The read state of an add-on's answer that carries its own (`taken_at`, `current`, `stale`). */
export function observed(o: { taken_at: string; current: boolean; stale?: Stale; truncated?: boolean }, since?: number): ReadState {
  return { readAt: o.taken_at, current: o.current, stale: stamped(o.stale, since), truncated: o.truncated };
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
 * What colour the line and its dot are drawn in. A copy the add-on kept is the
 * normal state of a read, so it is neutral however old it is, and so is a
 * cooldown the reader did not cause (a page load inside someone else's, marked
 * `pressed: false`). A cooldown carried in the data is the reader's own press, so it is amber, and a system that cannot be reached are
 * amber; every other reading is drawn by its age.
 */
export function toneOf(reading: Freshness, why: string | undefined, pressed: boolean): { text: string; dot: string } {
  if (why === STALE.cached || (why === STALE.cooldown && !pressed)) return { text: "text-faint", dot: "bg-faint" };
  if (why === STALE.cooldown) return { text: "text-warn-text", dot: "bg-warn" };
  return {
    text: TONE[reading],
    dot: reading === "live" || reading === "ageing" ? "bg-healthy" : reading === "stale" ? "bg-warn" : "border border-warn",
  };
}

/**
 * Read again's pill: the line plus a few px (the `!`s beat the Button's 44px
 * floor), with the 44px target on touch moved to an unpainted `before` box.
 */
export const READ_AGAIN_PILL =
  "min-h-0! py-px! before:absolute before:inset-x-0 before:top-1/2 before:h-[var(--tap)] before:-translate-y-1/2 desktop:before:hidden";

export interface RefreshPress {
  /** A press is in flight: show the control busy, and the old age stays until the read lands. */
  pressing: boolean;
  /** The age of the read a press brought, for a second only: the bloom is for it alone. */
  landed: string | null;
  /** Records the age on screen, runs the read, and never throws. */
  press: (run: () => unknown) => Promise<void>;
}

/**
 * S17. Only a read the reader asked for blooms. Pressing records the age on
 * screen; the next read that differs from it is the answer to the press. A
 * background read finds nothing recorded and lands quietly, and a press whose
 * read never arrives is forgotten shortly after it settles, so it cannot claim
 * the next background read.
 */
export function useRefreshPress(readAt: string | null | undefined): RefreshPress {
  const [pressedAt, setPressedAt] = useState<string | null | undefined>(undefined);
  const [pressing, setPressing] = useState(false);
  const [landed, setLanded] = useState<string | null>(null);
  // The pending "forget this press" timer: one at a time (a press cancels the
  // one before it), never armed once the card has unmounted (a read still in
  // flight settles after), and cancelled on unmount, so it never sets state on
  // a card that is gone. `mounted` is set in the effect, not at creation, so
  // StrictMode's unmount and remount leaves it true.
  const forget = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      clearTimeout(forget.current);
    };
  }, []);
  // Decided in the render the new read arrives in, so the line that says so is drawn with it:
  // an effect would let one frame of the new age through before the crossfade began.
  if (pressedAt !== undefined && (readAt ?? null) !== pressedAt) {
    setPressedAt(undefined);
    setLanded(readAt ?? null);
  }
  useEffect(() => {
    if (landed === null) return;
    const t = setTimeout(() => setLanded(null), LANDED_HOLD_MS);
    return () => clearTimeout(t);
  }, [landed]);
  async function press(run: () => unknown) {
    clearTimeout(forget.current);
    setPressedAt(readAt ?? null);
    setPressing(true);
    try {
      await run();
    } catch {
      // The read failed: the button comes back, nothing blooms.
    } finally {
      if (mounted.current) {
        setPressing(false);
        forget.current = setTimeout(() => setPressedAt(undefined), PRESS_FORGET_MS);
      }
    }
  }
  return { pressing, landed, press };
}

/**
 * The "Read again" of the page a card sits on: the card's own GET with
 * `fresh=true` (`useReadAgain`). The renderers of an add-on's reads take it
 * from here, so a card names no press of its own.
 */
const ReadAgainContext = createContext<{ press: () => unknown; since?: number } | undefined>(undefined);
/** `since`: when the page's read last answered (its query's `dataUpdatedAt`), where a cooldown a reply carries counts from. */
export const ReadAgainProvider = ReadAgainContext.Provider;
export const useReadAgainPress = () => useContext(ReadAgainContext)?.press;
export const useReadAgainSince = () => useContext(ReadAgainContext)?.since;

/**
 * Why the last "Read again" failed, when it was not the cooldown (the line says that one): in the
 * outcome every refusal wears, under the line, so a press that did nothing is never silent.
 */
export function ReadAgainError({ error }: { error: unknown }) {
  if (!error || cooldownOf(error)) return null;
  return (
    <div className="px-5 pb-2">
      <ActionOutcome word="Not read" outcome={outcomeFromError(error)} />
    </div>
  );
}

/** Said by an operator unless the page says it is a member reading (the one place a reader is not known). */
const UNNAMED_OPERATOR: Reader = { audience: "operator", person: "" };

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
  refresh,
  reader = UNNAMED_OPERATOR,
  className = "",
}: {
  state: ReadState;
  subject?: string;
  /** Who is reading: it words the step beside a system that cannot be reached. */
  reader?: Reader;
  onRefresh?: () => unknown;
  refreshing?: boolean;
  /** When the press is somewhere else on the page (a header button), its `useRefreshPress`. */
  refresh?: RefreshPress;
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

  // S17: a refresh the reader pressed here, or one pressed on a button elsewhere on the page
  // and handed in as `refresh`.
  const own = useRefreshPress(state.readAt);
  const { pressing, landed, press } = refresh ?? own;

  const reading = freshness ?? classifyRead(state, new Date(state.readAt ?? 0).getTime());
  const age = state.readAt ? formatRelative(state.readAt) : null;

  // A copy the add-on kept, or a read pressed inside its cooldown: Read again is the way out, and
  // inside the cooldown it waits for the count. An unreachable one has no way out but time.
  const why = state.stale?.reason;
  const left = useCountdown(why === STALE.cooldown ? state.stale?.retry_after : undefined, state.stale?.since);
  const cooling = why === STALE.cooldown && left > 0;
  // Coloured by what the line says: the cooldown while it counts, then a copy is a copy (a copy of a
  // read that failed is still neutral) and anything else is drawn by its age.
  const said = cooling || why !== STALE.cooldown ? why : state.current === false ? STALE.cached : undefined;
  const tone = toneOf(reading, said, state.stale?.pressed !== false);
  const aCopy = why === STALE.cached || why === STALE.cooldown;
  const cannotReach = why === STALE.unreachable && Boolean(state.readAt);
  const lineId = useId();

  const blooming = landed !== null && landed === (state.readAt ?? null) && (reading === "live" || reading === "ageing");

  return (
    <div className={`flex flex-wrap items-center gap-x-2 gap-y-1 text-[12.5px] ${className}`}>
      {/* A dot for every reading, not only the healthy one. It is the thing
          the eye reaches before the sentence, and its absence on three of the
          four readings made "no dot" mean both "ageing" and "stale".
          `provisional` is a ring rather than a fill, because it is the one
          reading that is neither healthy nor broken: the read answered, and
          the target could not confirm it. */}
      {/* The dot and the sentence wrap as one, so a long sentence never leaves the dot alone on its line. */}
      <span className="flex min-w-0 items-start gap-2">
      <span
        aria-hidden
        data-freshness-dot
        className={`mt-[0.5em] size-1.5 flex-none rounded-pill motion-tint ${blooming ? "bloom " : ""}${tone.dot}`}
      />
      {/* The new line crossfades in over the old one when a read the reader asked for lands (S17). */}
      <Crossfade id={blooming ? `landed:${landed}` : "read"}>
        <span id={lineId} className={tone.text}>
          {cooling
            ? cooldownLine(subject, left)
            : cannotReach
              ? unreachableLine(subject, formatClock(state.readAt), reader)
              : aCopy && age
                ? copyLine(subject, age)
                : sentence(reading, subject, age, Boolean(onRefresh))}
        </span>
      </Crossfade>
      </span>
      {state.truncated && (
        // Its own clause, never folded into the age. A capped read is complete
        // about what it saw and silent about the rest, and an operator who
        // reads it as "that is everything" concludes an absence from a list
        // that was cut off.
        <span className="text-faint">· not the whole list, only the first part was read</span>
      )}
      {onRefresh && (reading === "stale" || aCopy) && (
        // It tints in where the line ends. Its slot is zero tall, so the button
        // overflows the line's own height instead of making the line taller: the
        // sentence beside it stays where it was (on desktop only: on touch the slot
        // is a thumb tall, because when the sentence wraps a zero-tall slot let the
        // button's edge cut the line above it). The pill drawn is the line and
        // a hair, never the 44px a thumb needs: on touch that comes from the
        // `before` box, a hit area with nothing painted, so no visible edge
        // reaches the cards below (A-08 §5).
        <span className="tint-in flex h-[var(--tap)] items-center desktop:h-0">
          <Button
            variant="outline"
            size="sm"
            className={READ_AGAIN_PILL}
            onClick={() => void press(onRefresh)}
            isPending={refreshing || pressing}
            disabled={cooling}
            // The line beside it is the reason it waits.
            aria-describedby={cooling ? lineId : undefined}
          >
            {refreshing || pressing ? READING : READ_AGAIN}
          </Button>
        </span>
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
      const wayOut = canRefresh ? "" : ". Reload the page to read it again";
      return age
        ? `${subject} was read ${age}, too old to act on${wayOut}`
        : `${subject} has not been read yet${wayOut}`;
    }
    case "provisional":
      // Never "unreachable" alone. The useful half is that what is on screen is
      // real and dated, not that the network is down.
      return age
        ? `Could not reach the connected system. This is the last state seen, ${age}`
        : "Could not reach the connected system, and there is no earlier copy to show";
  }
}

/**
 * A press going again by itself once its cooldown is over (useRetryOnce). Mount it with the card,
 * always: its `role="status"` region is there empty from the start and filled when a wait starts,
 * so the start is announced once (a region born with its text is often skipped; the visible count
 * ticks outside it). While waiting it draws the line amber, as a cooldown the reader caused is
 * (copy guide section 13), and **Stop waiting**, described by that line, which takes focus only
 * when the person is still on the button they pressed.
 */
export function RetryWait({ name, retry, onStop }: {
  name: string;
  retry: { waiting: boolean; seconds: number; left: number; takeFocus: boolean };
  onStop: () => void;
}) {
  const stopId = useId();
  const lineId = useId();
  const { waiting, takeFocus } = retry;
  useEffect(() => {
    if (waiting && takeFocus) document.getElementById(stopId)?.focus();
  }, [waiting, takeFocus, stopId]);
  return (
    <>
      <span role="status" className="sr-only">
        {waiting ? retryingLine(name, retry.seconds) : ""}
      </span>
      {waiting && (
        <span className="flex flex-wrap items-center gap-3">
          <span id={lineId} className="flex-1 text-[14px] text-warn-text">
            {retryingLine(name, retry.left)}
          </span>
          <Button id={stopId} variant="outline" size="sm" aria-describedby={lineId} onClick={onStop}>
            {STOP_WAITING}
          </Button>
        </span>
      )}
    </>
  );
}
