export type EventOutcome = "all" | "done" | "waiting" | "failed" | "dropped";

/**
 * Raw status → the bucket an operator asks in.
 *
 * The two tables do not share a status vocabulary — a webhook event is
 * `processed`, an onboarding trigger is `completed` — and neither word is what
 * somebody scanning this page has in mind. They want "did it work", "is it
 * still going", "did it break", "did we choose not to act".
 *
 * An unrecognised status returns itself, so it stays visible under All and is
 * never quietly counted as a success. A new backend status appearing as "done"
 * on a forensic log is exactly the kind of silence this page exists to prevent.
 */
export function outcomeOf(status: string): string {
  if (status === "failed") return "failed";
  // 'unconfigured' is an onboarding trigger that found no welcome bundle to
  // give — understood and not acted on, same bucket as a dropped webhook
  // event, and deliberately never "failed": nothing broke, so there is
  // nothing here for an operator to fix by retrying.
  if (status.startsWith("dropped") || status === "unconfigured") return "dropped";
  if (status === "pending" || status === "in_flight") return "waiting";
  if (status === "processed" || status === "completed" || status === "succeeded") return "done";
  return status;
}

/**
 * What a bucket is called on screen, and the dot that carries it. One table,
 * so the filter pills and the column on every row cannot name the same bucket
 * two ways. An unrecognised status gets a word of its own rather than a borrowed
 * one — the page must never say "Done" about something it does not understand.
 */
const OUTCOME_LABEL = {
  done: { word: "Done", tone: "healthy" },
  waiting: { word: "Waiting", tone: "neutral" },
  failed: { word: "Failed", tone: "danger" },
  dropped: { word: "Not acted on", tone: "neutral" },
} as const;

export function outcomeLabel(outcome: string): { word: string; tone: "healthy" | "neutral" | "danger" } {
  return (
    OUTCOME_LABEL[outcome as keyof typeof OUTCOME_LABEL] ?? { word: "Not recognised", tone: "neutral" }
  );
}
