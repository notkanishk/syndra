/**
 * Where to send somebody after they sign in, when they were somewhere.
 *
 * A `next` parameter is attacker-controllable by construction: it arrives in a
 * URL anybody can compose and hand to a member. Unvalidated, it is an open
 * redirect — a link that starts on this deployment's real sign-in page and
 * finishes on somebody else's, which is the cheapest phishing there is.
 *
 * So this is a strict allowlist of shapes rather than a filter of bad ones:
 *
 *   - It must begin with a single `/`. That rejects absolute URLs
 *     (`https://elsewhere/`), and it rejects `//elsewhere/path`, which a
 *     browser reads as protocol-relative and follows off-site.
 *   - It must not begin with `/\`, which some browsers normalise to `//`.
 *   - It must not be `/login`, or signing in returns to signing in.
 *
 * Anything else falls back to `/`, silently. A rejected return path is not
 * worth telling somebody about: either it was a typo in a link, or it was an
 * attempt, and neither is improved by a message.
 */
export function safeReturnPath(next: string | null | undefined): string {
  if (!next) return "/";
  if (!next.startsWith("/")) return "/";
  if (next.startsWith("//") || next.startsWith("/\\")) return "/";
  // Only the path is honoured; a `next` carrying its own origin is already
  // rejected above, and this keeps a stray colon or backslash from being
  // re-interpreted downstream.
  const [pathOnly] = next.split("#");
  if (pathOnly === "/login" || pathOnly.startsWith("/login?")) return "/";
  return pathOnly;
}
