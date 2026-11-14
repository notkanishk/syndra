/**
 * The locale every date, number and ordering is shown in: the deployment's
 * LOCALE (a BCP 47 tag, RFC 5646, e.g. en-GB), read from the backend with the
 * zone (`locale` on /healthz, lib/zone.ts). It mirrors the zone (lib/time.ts):
 *
 *  - the CONFIGURED tag is write-once: the first one the backend sent is kept,
 *    a failed or empty read never replaces or pins anything, and a different
 *    one later is warned about once and ignored;
 *  - the backend validated it at start and the server canonicalised it again
 *    (validLocale, in the one fetch); the browser trusts what the layout hands
 *    it and does not judge it a second time, so the two cannot diverge;
 *  - until one is configured and read, `displayLocale()` is the FALLBACK the
 *    layout passed down: the server runtime's own default, validated on the
 *    server. It is never learned: a failed first read does not make it the
 *    deployment's locale, and a later good read still takes effect. If even
 *    that is not a valid tag it is undefined, and Intl then uses its own
 *    default (server and browser alike); that is the one deterministic case
 *    left, and it needs a broken runtime.
 */
let held: string | undefined;
let fallback: string | undefined;
let warned = false;

/** The tag in canonical form, or undefined if it is not a well-formed one. Runs on the server only. */
export function validLocale(tag: unknown): string | undefined {
  if (typeof tag !== "string" || !tag) return undefined;
  try {
    return Intl.getCanonicalLocales(tag)[0]; // RangeError on a malformed tag
  } catch {
    return undefined;
  }
}

/** The server runtime's own default, validated; what a deployment with no LOCALE formats in. */
export const runtimeLocale = (): string | undefined => validLocale(new Intl.NumberFormat().resolvedOptions().locale);

/** The locale to pass as the first argument of every toLocale* / Intl.* formatter and localeCompare. */
export const displayLocale = (): string | undefined => held ?? fallback;

/** Adopt the deployment's configured locale (write-once; empty or undefined changes nothing). */
export function setDisplayLocale(tag: string | null | undefined): void {
  if (!tag) return;
  if (held) {
    if (tag !== held && !warned) {
      warned = true;
      console.warn(`[locale] backend now names ${tag}; keeping ${held} until the UI restarts`);
    }
    return;
  }
  held = tag;
}

/** The fallback the layout passes down, so the browser formats as the server does. */
export function setFallbackLocale(tag: string | undefined): void {
  fallback = tag;
}

/**
 * Offer the tag just read (undefined when the read failed or named none) and
 * return the CONFIGURED locale this process holds, undefined when none: what
 * the layout hands the client beside the fallback. Same shape as effectiveZone.
 */
export function effectiveLocale(read: string | undefined): string | undefined {
  setDisplayLocale(read);
  return held;
}

/** An amount with `digits` decimals in the display locale ("1.5" or "1,5"). */
export const formatDecimal = (n: number, digits = 1): string =>
  new Intl.NumberFormat(displayLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(n);

/** Test-only: forget everything, as a fresh process. */
export function resetDisplayLocaleForTests(): void {
  held = undefined;
  fallback = undefined;
  warned = false;
}
