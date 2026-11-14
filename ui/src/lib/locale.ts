/**
 * The locale every date and number is shown in: the deployment's LOCALE (a
 * BCP 47 tag, RFC 5646, e.g. en-GB), read from the backend with the zone
 * (`locale` on /healthz, lib/zone.ts). Like the zone it is WRITE-ONCE from a
 * VALIDATED tag, so the server and the browser format identically (a date
 * rendered in two locales is a hydration mismatch).
 *
 * Until one is learned, and whenever the backend names none (LOCALE unset) or
 * one this runtime has no data for, the locale is the server runtime's own
 * default. The layout passes the EFFECTIVE locale down (effectiveLocale), so the
 * browser holds the server's value, not its own.
 */
let held: string | undefined;

/** The tag in canonical form if this runtime has locale data for it, else undefined. Checked once, on the server. */
export function validLocale(tag: unknown): string | undefined {
  if (typeof tag !== "string" || !tag) return undefined;
  try {
    const [canonical] = Intl.getCanonicalLocales(tag); // throws RangeError on a malformed tag
    return Intl.DateTimeFormat.supportedLocalesOf(canonical).length > 0 ? canonical : undefined;
  } catch {
    return undefined;
  }
}

/** What this runtime formats in when nothing was configured. */
const runtimeDefault = (): string => new Intl.NumberFormat().resolvedOptions().locale;

/** The locale to pass as the first argument of every toLocale* / Intl.* formatter. */
export const displayLocale = (): string => held ?? runtimeDefault();

/** Adopt the deployment's locale: the first valid tag is kept; an invalid, missing or different one changes nothing. */
export function setDisplayLocale(tag: string | null | undefined): void {
  if (held) return;
  held = validLocale(tag);
}

/**
 * Offer the tag just read (undefined when the read failed or named none) and
 * return the locale this process actually formats in: what the layout hands the
 * client. With nothing learned that is the runtime's default, which is a real
 * locale, so the client matches without a literal.
 */
export function effectiveLocale(read: string | undefined): string {
  setDisplayLocale(read);
  return displayLocale();
}

/** Test-only: forget the locale, as a fresh process. */
export function resetDisplayLocaleForTests(): void {
  held = undefined;
}
