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
    new Intl.DateTimeFormat("en-US", { timeZone: name });
    return name;
  } catch {
    return undefined;
  }
}

export async function fetchMakerspaceZone(): Promise<string | undefined> {
  try {
    const res = await fetch(`${process.env.BACKEND_URL || "http://backend:8080"}/healthz`, {
      next: { revalidate: 60 },
      signal: AbortSignal.timeout(1000),
    });
    const body = (await res.json()) as { time_zone?: unknown };
    return validZone(body.time_zone);
  } catch {
    return undefined;
  }
}
