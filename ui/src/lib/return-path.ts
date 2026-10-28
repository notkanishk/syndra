/**
 * Where to send somebody after they sign in, when they were somewhere.
 *
 * A `next` parameter is attacker-controllable by construction: it arrives in a
 * URL anybody can compose and hand to a member. Unvalidated, it is an open
 * redirect — a link that starts on this deployment's real sign-in page and
 * finishes on somebody else's, which is the cheapest phishing there is.
 *
 * So this is a strict allowlist of shapes rather than a filter of bad ones. The
 * value is parsed the way a browser will parse it, against a base that is not
 * this deployment, and accepted only if it is still on that base:
 *
 *   - It must begin with a single `/`, and parsing it must not change the
 *     origin. That rejects absolute URLs (`https://elsewhere/`), protocol-
 *     relative `//elsewhere/path`, `/\elsewhere` (a backslash is a slash to the
 *     URL parser), and `/<tab>/elsewhere`, whose tab the parser strips and a
 *     hand-written prefix check reads as a harmless path.
 *   - What is returned is the parser's own `pathname + search + hash`, not the
 *     input, so what was validated is exactly what gets followed.
 *   - It must not be `/login`, or signing in returns to signing in.
 *
 * Anything else falls back to `/`, silently. A rejected return path is not
 * worth telling somebody about: either it was a typo in a link, or it was an
 * attempt, and neither is improved by a message.
 */
const BASE = "http://x.invalid";

export function safeReturnPath(next: string | null | undefined): string {
  if (!next || !next.startsWith("/")) return "/";
  let url: URL;
  try {
    url = new URL(next, BASE);
  } catch {
    return "/";
  }
  // `/..//elsewhere` normalises to a pathname of `//elsewhere`: same origin
  // here, protocol-relative once it becomes a Location.
  if (url.origin !== BASE || url.pathname.startsWith("//")) return "/";
  if (url.pathname === "/login") return "/";
  return url.pathname + url.search + url.hash;
}
