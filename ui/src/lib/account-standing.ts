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
    // itself, so "yet" is not said. The contract makes this Syndra's own doing: an account the
    // `account` field holds off is `waiting_on: none` because off is what Syndra asked for
    // (addons/AUTHORING.md), and an account somebody switched off outside Syndra is a drift,
    // which Syndra shows where it triages them, never a standing. So it is one statement, and a
    // next step that stays in Syndra. The add-on's own "The account is switched off." only
    // repeats the state, so it is not shown beside it.
    const repeats = !note || /^the account is switched off\.?$/i.test(note.trim());
    return {
      state: "Switched off",
      tone: "neutral",
      note: repeats ? undefined : note,
      next:
        standing.next ||
        (you
          ? "It stays off while you are on hold or deactivated. Ask makerspace staff if you did not expect this."
          : `${NOTHING_TO_DO} Syndra keeps it off while the person is on hold, deactivated or has no role that reaches ${system}.`),
    };
  }
  return { state: "Not usable yet", tone: "neutral", note, next: standing.next || NOTHING_TO_DO };
}
