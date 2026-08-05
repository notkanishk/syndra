/**
 * What the closed door says.
 *
 * The doorway has one failure picture — the arch shuts into a complete amber
 * line — and three things it can honestly say behind it. `/auth/callback` and
 * `/auth/zitadel` between them redirect with ten distinct `?error=` codes, and
 * flattening all of them to "Zitadel didn't answer" would tell a member the
 * provider was silent when it had in fact refused them by name.
 *
 * The code itself is never rendered. It arrives in a URL anyone can type, and
 * a visitor who cannot get in should learn that from the page, not from a
 * string the page was handed.
 */

export interface LoginFailure {
  head: string;
  sub: string;
}

/** The provider answered, and the answer was no. */
const REFUSED = new Set([
  "access_denied",
  "consent_required",
  "interaction_required",
  "login_required",
]);

/** Both sides are up; the round trip between them did not survive. */
const HANDSHAKE = new Set([
  "invalid_claims",
  "invalid_token",
  "missing_params",
  "no_access_token",
  "pkce_expired",
  "pkce_invalid",
  "pkce_missing",
  "state_mismatch",
]);

export function loginFailure(code: string | undefined): LoginFailure | null {
  if (!code) return null;

  if (REFUSED.has(code)) {
    return {
      head: "Zitadel didn't let you through.",
      sub: "Nothing was signed in. Check you used your makerspace account, or find a steward in the space.",
    };
  }

  if (HANDSHAKE.has(code)) {
    return {
      head: "The sign-in didn't complete.",
      sub: "Nothing was signed in. Try again — if it keeps happening, find a steward in the space.",
    };
  }

  // `misconfigured`, `token_exchange_failed`, and anything the provider
  // invents. All of them mean the same thing to the person standing here:
  // nobody answered the door.
  return {
    head: "Zitadel didn't answer.",
    sub: "Nothing was signed in. Try again in a minute, or find a steward in the space.",
  };
}
