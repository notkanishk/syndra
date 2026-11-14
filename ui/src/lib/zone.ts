import { validLocale } from "@/lib/locale";

/**
 * The deployment's zone, from the backend (`time_zone` on /healthz: the one
 * unauthenticated read that exists, answered with or without the database).
 * The root layout awaits this before rendering, so the first paint is already
 * in the right zone. Undefined when it cannot be read: the UI then shows UTC and
 * labels it (lib/time.ts), it does not guess. Cached briefly: TZ changes only
 * at a deploy.
 */
/** The name if Intl can read it, else undefined (UTC, labelled). Checked once, here, on the server. */
export function validZone(name: unknown): string | undefined {
  if (typeof name !== "string" || !name) return undefined;
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: name }); // locale: not displayed
    return name;
  } catch {
    return undefined;
  }
}

/** What /healthz says about the deployment's formatting: its zone and its display locale (lib/locale.ts), each undefined when unread or invalid. One fetch for both. */
export async function fetchMakerspaceFormat(): Promise<{ zone: string | undefined; locale: string | undefined }> {
  try {
    const res = await fetch(`${process.env.BACKEND_URL || "http://backend:8080"}/healthz`, {
      next: { revalidate: 60 },
      signal: AbortSignal.timeout(3000),
    });
    const body = (await res.json()) as { time_zone?: unknown; locale?: unknown };
    return { zone: validZone(body.time_zone), locale: validLocale(body.locale) };
  } catch {
    return { zone: undefined, locale: undefined };
  }
}
