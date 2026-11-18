import type { Standing } from "@/lib/queries/useTargets";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * Whether an account on a system can be used yet, and who is waited on, in
 * words (addon-contract §6). The add-on writes the note and the next step;
 * Syndra says who is waited on and never leaves the reader without a next
 * step: no `next` becomes a line from who is waited on, or "Nothing for you
 * to do." (owner, 6 Nov 2026).
 */
export interface StandingWords {
  /** "Ready to use", "Waiting on the member"... one short phrase. */
  state: string;
  tone: "healthy" | "warn" | "neutral";
  /** The add-on's own note, as given. */
  note?: string;
  /** Always present, and always a next step or "Nothing for you to do." */
  next: string;
}

export function standingWords(standing: Standing, system: string, audience: "member" | "operator"): StandingWords {
  const note = standing.note || undefined;
  if (standing.usable) {
    return { state: "Ready to use", tone: "healthy", note, next: standing.next || NOTHING_TO_DO };
  }
  const waiting = standing.waiting_on;
  const you = audience === "member";
  if (waiting === "member") {
    return {
      state: you ? "Waiting on you" : "Waiting on the member",
      tone: "warn",
      note,
      next: standing.next || (you ? "Do what it says above, then check back here." : "Ask them to do what it says above."),
    };
  }
  if (waiting === "operator") {
    return {
      state: "Waiting on makerspace staff",
      tone: "warn",
      note,
      next: standing.next || (you ? "Ask makerspace staff." : "It is yours to do: open the system's page."),
    };
  }
  if (waiting === "target") {
    return { state: `Waiting on ${system}`, tone: "neutral", note, next: standing.next || `${NOTHING_TO_DO} ${system} has to finish first.` };
  }
  if (waiting === "none") {
    // Nobody is waited on, and it is still not usable: it is switched off, and that does not end by
    // itself, so "yet" is not said. Whether Syndra asked for it is the add-on's to say, in its own
    // `note` and `next`, shown as written (TrueNAS says it for an account Syndra holds off, one
    // nobody is assigned to, and one whose intent it does not know). Where it says nothing more,
    // these words make no claim either way, and send the operator to Syndra's own comparison.
    // The add-on's "The account is switched off." only repeats the state, so it is not shown beside it.
    const repeats = !note || /^the account is switched off\.?$/i.test(note.trim());
    return {
      state: "Switched off",
      tone: "neutral",
      note: repeats ? undefined : note,
      next:
        standing.next ||
        (you
          ? "Ask makerspace staff if you did not expect this."
          : "If no one in Syndra asked for this, use Check for differences on the system's Mappings page."),
    };
  }
  return { state: "Not usable yet", tone: "neutral", note, next: standing.next || NOTHING_TO_DO };
}

/**
 * A table row's step: silence for normal (D-1 Q5, owner 18 Nov 2026; G17). A
 * healthy row with nothing to do says only its state; a step is written only
 * where there is one. This is the one exception to the next-step rule, and it
 * is for table rows only: every other row, and every outcome, notice and empty
 * state, still ends on a step or "Nothing for you to do." (`next-step.test.tsx`).
 */
export function rowStep(standing: Standing, system: string, audience: "member" | "operator"): string | undefined {
  const w = standingWords(standing, system, audience);
  return w.tone === "healthy" && w.next === NOTHING_TO_DO ? undefined : w.next;
}
