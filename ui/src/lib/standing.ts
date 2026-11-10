/**
 * Whether a person holds a role: the backend's one answer (services.standingOf),
 * rendered here and never worked out again. No component may decide holding from
 * observed_role_keys, reasons[].queued, in_zitadel or a second Zitadel read —
 * `standing-guard.test.ts` enforces it (one-path W3b, owner 10 Nov 2026).
 *
 * null is "not read completely and nothing queued settles it": absence can't be
 * concluded, and the view's `observation` says why.
 */
export type RoleStanding = "held" | "sent" | "waiting" | "removing" | "not_held";
export type Standing = RoleStanding | null | undefined;
export type StandingAudience = "operator" | "member";

/** Still held: Zitadel shows it, even if a removal is on its way. */
export const holds = (s: Standing): boolean => s === "held" || s === "removing";

/** Given, and not seen in Zitadel yet. */
export const onItsWay = (s: Standing): boolean => s === "sent" || s === "waiting";

/**
 * The words for a standing, or null where silence is the word (a member's held
 * role). A missing key reads as null: the backend couldn't say.
 */
export function standingWords(standing: Standing, audience: StandingAudience): string | null {
  const operator = audience === "operator";
  switch (standing) {
    case "held":
      return operator ? "In Zitadel" : null;
    case "sent":
      return operator ? "Sent, not seen back yet" : "On its way";
    case "waiting":
      return operator ? "Not sent to Zitadel yet" : "On its way";
    case "removing":
      return "Being removed";
    case "not_held":
      return operator ? "Not in Zitadel" : "Not there yet";
    default:
      return operator ? "Couldn’t check Zitadel" : "Couldn’t check";
  }
}
