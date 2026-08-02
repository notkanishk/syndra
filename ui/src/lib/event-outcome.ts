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
  if (status.startsWith("dropped")) return "dropped";
  if (status === "pending" || status === "in_flight") return "waiting";
  if (status === "processed" || status === "completed" || status === "succeeded") return "done";
  return status;
}
