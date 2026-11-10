import type { RequestHolding } from "@/lib/queries/useRequests";

/**
 * A fixture of the access view's `holdings`, as the backend sends them
 * (services.holdingFrom, pinned there by TestExplainUserAccess_Holdings…): a
 * direct grant is had, until its end when it has one, unless its removal is on
 * its way; a role held, sent or waiting is had for good. Tests feed it so a
 * mocked access view says what the real one would; the UI never derives this.
 */
export function holdingsFor(
  grants: Array<{ role_key: string; expires_at?: string | null }>,
  standings: Record<string, string | null>,
): Record<string, RequestHolding> {
  const out: Record<string, RequestHolding> = {};
  const keys = new Set([...grants.map((g) => g.role_key), ...Object.keys(standings)]);
  for (const key of keys) {
    const s = standings[key] ?? null;
    const grant = grants.find((g) => g.role_key === key);
    const waiting = s === "waiting";
    if (s === "removing") out[key] = { kind: "none" };
    else if (grant?.expires_at) out[key] = { kind: "until", until: grant.expires_at, waiting };
    else if (grant || s === "held" || s === "sent" || waiting) out[key] = { kind: "for_good", waiting };
    else out[key] = { kind: "none" };
  }
  return out;
}
