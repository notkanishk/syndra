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
  return { state: "Not usable yet", tone: "neutral", note, next: standing.next || NOTHING_TO_DO };
}
