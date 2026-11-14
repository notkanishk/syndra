import { CAP, capText } from "@/lib/usage";

/**
 * Why an add-on is not simply active, in a sentence (health `lifecycle_note`).
 *
 * The add-on sends a reason with its lifecycle state: a code of its own for what it did itself
 * (`startup`, `shutting down`, an unrecognised configured state), or the free text an operator
 * typed when they changed it. This is the one place that turns either into words, so no card
 * shows a raw code. A reason it does not know is an operator's, and is quoted as given.
 */
const OWN_REASON: Record<string, string> = {
  startup: "It was started this way by its configuration.",
  "shutting down": "It is shutting down.",
  "configured lifecycle state was not recognised":
    "Its configured state was not recognised, so it started read-only to be safe.",
};

export function lifecycleWhy(note: string | undefined): string {
  // An operator's words, cut and stripped of control characters like every add-on text (capText).
  const text = note ? capText(note, CAP.reason) : "";
  if (!text) return "Somebody set that on purpose.";
  return OWN_REASON[text] ?? `Somebody set that on purpose: “${text}”.`;
}
