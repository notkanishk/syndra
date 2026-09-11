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
  if (observed === undefined) {
    return { headline: String(given), note: "not checked yet", tone: "muted" };
  }

  const base = confirmed ?? 0;
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
  if (parts.length === 0 && observed > 0) {
    parts.push("confirmed");
  }

  return { headline: String(observed), note: parts.join(" · "), tone };
}
