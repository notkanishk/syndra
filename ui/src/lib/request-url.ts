/**
 * Resolving the browser-facing origin from a proxied request.
 *
 * The UI never sees the URL the browser asked for. Next's standalone server
 * builds `request.url` from the address it listens on, so inside the container
 * every request looks like `http://localhost:3000/...` no matter what the user
 * typed. Only `x-forwarded-host` / `x-forwarded-proto` know the real origin.
 *
 * This lived as three byte-identical copies of `buildRedirectUrl` plus two
 * hand-rolled variants that forgot the headers entirely, and each copy shared a
 * bug: they mutated the incoming URL with `url.host = forwardedHost`. The WHATWG
 * host setter only overwrites the port when the value it is given carries one,
 * so a bare `syndra.example.org` assigned onto `http://localhost:3000/...`
 * left the `:3000` in place. Behind a reverse proxy that sent users to
 * `https://syndra.example.org:3000/` on every redirect, and handed Zitadel
 * a redirect_uri it had no reason to accept.
 *
 * Everything that needs an absolute URL routes through here, and the origin is
 * always built from scratch rather than edited into shape.
 */

interface RequestLike {
  url: string;
  headers: Headers;
}

/** First value of a possibly comma-joined forwarded header. */
function firstHeaderValue(headers: Headers, name: string): string | null {
  const raw = headers.get(name);
  if (!raw) return null;
  const first = raw.split(",")[0]?.trim();
  return first || null;
}

/** Scheme and authority the browser used, e.g. `https://syndra.example.org`. */
export function resolveRequestOrigin(request: RequestLike): string {
  const requestUrl = new URL(request.url);
  const proto =
    firstHeaderValue(request.headers, "x-forwarded-proto") ||
    requestUrl.protocol.replace(/:$/, "");
  const host =
    firstHeaderValue(request.headers, "x-forwarded-host") ||
    firstHeaderValue(request.headers, "host") ||
    requestUrl.host;
  return `${proto}://${host}`;
}

/**
 * Absolute URL for `path` on the browser-facing origin.
 *
 * `search` is dropped unless supplied — a redirect target inherits nothing from
 * the request that triggered it, so an `?error=` or `?code=` cannot leak into
 * the next page.
 */
export function buildRedirectUrl(request: RequestLike, path: string, search = ""): URL {
  const url = new URL(path, resolveRequestOrigin(request));
  url.search = search;
  return url;
}

/**
 * Whether the browser's connection was HTTPS, for the `secure` cookie flag.
 *
 * TLS terminates at the proxy, so the request reaching this process is plain
 * HTTP and `requestUrl.protocol` says so. Trusting it marks production session
 * cookies non-Secure, letting them ride a downgraded request.
 */
export function isSecureRequest(request: RequestLike): boolean {
  const forwarded = firstHeaderValue(request.headers, "x-forwarded-proto");
  if (forwarded) return forwarded === "https";
  return new URL(request.url).protocol === "https:";
}
