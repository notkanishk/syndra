import { DEFAULT_ZONE } from "@/lib/time";

/**
 * The makerspace's zone, from the backend (`time_zone` on /healthz: the one
 * unauthenticated read that exists, answered with or without the database).
 * The root layout awaits this before rendering, so the first paint is already
 * in the right zone. If the backend cannot be reached nothing renders anyway;
 * DEFAULT_ZONE (the backend's own fallback) is what a failed read degrades to.
 * Cached briefly: TZ changes only at a deploy.
 */
export async function fetchMakerspaceZone(): Promise<string> {
  try {
    const res = await fetch(`${process.env.BACKEND_URL || "http://backend:8080"}/healthz`, {
      next: { revalidate: 60 },
      signal: AbortSignal.timeout(3000),
    });
    const body = (await res.json()) as { time_zone?: unknown };
    return typeof body.time_zone === "string" && body.time_zone ? body.time_zone : DEFAULT_ZONE;
  } catch {
    return DEFAULT_ZONE;
  }
}
