/**
 * "Holding is observed; explanation is recorded."
 *
 * Every surface that shows a headcount for a role, project or app has to say
 * two different things: what Zitadel currently shows (observed — the truth),
 * and how much of that Syndra's own record explains (confirmed vs. given).
 * This is the one place that turns those three numbers into a sentence, so no
 * surface invents its own wording for "we haven't checked" versus "we checked
 * and it doesn't match".
 */

/** The headline of a count Zitadel has not been read for. Not a number. */
export const NOT_CHECKED = "—";

/**
 * The sort key of a people column: what Zitadel shows, never the recorded
 * number. Not checked sorts below zero, so it never ranks as a headcount.
 */
export function heldSortKey(observed: number | null | undefined): number {
  return observed ?? -1;
}

/** " person" or " people" after a headline; nothing after a count not checked. */
export function peopleSuffix(headline: string): string {
  if (headline === NOT_CHECKED) return "";
  return headline === "1" ? " person" : " people";
}

export type HoldersTone = "ok" | "warn" | "danger" | "muted";

export interface HoldersLine {
  headline: string;
  note: string;
  tone: HoldersTone;
}

/** Shared tone → text colour class, so every surface renders the same tone the same way. */
export const holdersToneClass: Record<HoldersTone, string> = {
  ok: "text-faint",
  warn: "text-warn-text",
  danger: "text-danger-text",
  muted: "text-faint",
};

/**
 * `given` is what Syndra decided to grant. `confirmed` is how much of that the
 * observation store also shows held — `undefined`, not zero, when nothing has
 * ever been confirmed. `observed` is what Zitadel shows right now — also
 * `undefined`, not zero, when the org has never been read.
 */
export function holdersLine(
  given: number,
  confirmed: number | undefined,
  observed: number | undefined,
): HoldersLine {
  // Never the recorded number in the headline: "12" over "not checked yet" is
  // still read as twelve people holding it. The recorded number is the note.
  if (observed === undefined) {
    return {
      headline: NOT_CHECKED,
      note: given > 0 ? `not checked yet · ${given} given by Syndra` : "not checked yet",
      tone: "muted",
    };
  }

  // The overlap is unknown — this caller does not compute it. It is NOT zero:
  // reading it as zero calls every holder both unexplained and undelivered at
  // once, about the same people. State what Zitadel shows and claim nothing
  // about how much of it the record explains.
  if (confirmed === undefined) {
    return { headline: String(observed), note: "", tone: "ok" };
  }

  const base = confirmed;
  const unexplained = observed - base;
  const missing = given - base;

  const parts: string[] = [];
  let tone: HoldersTone = "ok";
  if (unexplained > 0) {
    parts.push(`${unexplained} unexplained`);
    tone = "danger";
  }
  if (missing > 0) {
    parts.push(`${missing} not in Zitadel yet`);
    if (tone !== "danger") tone = "warn";
  }
  // All as expected says nothing (G17): only the exception is written.

  return { headline: String(observed), note: parts.join(" · "), tone };
}
