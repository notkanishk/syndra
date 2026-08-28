import { targetLabel } from "@/lib/nav";
import type { AuditEntry } from "@/lib/queries/useAudit";

/**
 * What an audit action means, in words.
 *
 * Shared rather than duplicated, because the audit log and a person's Activity
 * tab describe the same events: two vocabularies would let the same row read
 * "Revoked direct access" on one screen and "direct_grant.revoked" on the
 * other, and an operator comparing them would reasonably conclude one of the
 * screens is wrong.
 *
 * Keeping this map honest matters more than making it complete. An
 * unrecognised action falls through to its raw key, which is ugly but true; a
 * wrong sentence in an audit log is the one kind of bug nobody catches until it
 * matters.
 */
export const AUDIT_ACTIONS: Record<string, { verb: string; destructive?: boolean }> = {
  "direct_grant.upserted": { verb: "Gave direct access" },
  "direct_grant.replaced": { verb: "Replaced somebody's direct access" },
  "direct_grant.revoked": { verb: "Revoked direct access", destructive: true },
  // The record itself was deleted by hand, as opposed to a revoke that a rule,
  // an expiry or a review set off.
  "direct_grant.removed": { verb: "Revoked direct access by hand", destructive: true },
  "direct_grant.revoked_by_expiry": { verb: "Ended access on its expiry date", destructive: true },
  "bundle.created": { verb: "Created a bundle" },
  "bundle.assigned": { verb: "Gave somebody a bundle" },
  "bundle.unassigned": { verb: "Removed somebody from a bundle", destructive: true },
  "bundle.role_added": { verb: "Added a role to a bundle" },
  "bundle.role_removed": { verb: "Removed a role from a bundle", destructive: true },
  "bundle.welcome_set": { verb: "Set the default bundle for new members" },
  // Not "Renamed": the endpoint rewrites name and description, and the row records neither.
  // Claiming a rename for a description edit would be a specific falsehood in place of a vague
  // truth.
  "bundle.updated": { verb: "Changed a bundle's name or description" },
  // Same word the console puts on the button. An operator looking for what they just did should
  // find it under the verb they read.
  "bundle.deleted": { verb: "Retired a bundle", destructive: true },
  "bundle.version_published": { verb: "Published a bundle version" },
  "bundle.holder_moved": { verb: "Moved somebody to a different bundle version" },
  welcome_bundle_assigned: { verb: "Gave a new member the default bundle" },
  "mapping_rule.created": { verb: "Created an automatic rule" },
  "mapping_rule.updated": { verb: "Changed an automatic rule" },
  "mapping_rule.deleted": { verb: "Deleted an automatic rule", destructive: true },
  "role.created": { verb: "Created a role" },
  "access_request.created": { verb: "Asked for access" },
  "access_request.approved": { verb: "Approved a request" },
  "access_request.rejected": { verb: "Declined a request" },
  // Not destructive, and deliberately not phrased as a refusal — the person who filed it took it
  // back. `requestOutcome` keeps the same distinction on the request screens.
  "access_request.withdrawn": { verb: "Withdrew their request" },
  // Not destructive: the access was already going to lapse. What was recorded is that somebody
  // looked, which is the opposite of something being taken away unnoticed.
  "grant_expiry.acknowledged": { verb: "Recorded that expiring access should be left to lapse" },
  "grant_expiry.acknowledgement_cleared": { verb: "Put expiring access back in the undecided list" },
  "claim_profile.updated": { verb: "Changed a project's token format" },
  "app_claim_override.updated": { verb: "Changed an app's token format" },
  "app_claim_override.deleted": { verb: "Removed an app's token override" },
};

/**
 * The two families the backend builds by concatenation, with the connected
 * system's name in the middle: a change to somebody's account on TrueNAS
 * recorded and waiting to be sent, and a hold that lifted itself on its date.
 * Neither can be a static key above, so they are matched by shape.
 */
const ACCOUNT_CHANGE = /^entitlement\.([a-z0-9_-]+)\.enqueued$/;
const HOLD_LAPSED = /^allowance\.([a-z0-9_-]+)\.lapsed$/;

export function describeAction(action: string): { verb: string; destructive: boolean } {
  const known = AUDIT_ACTIONS[action];
  if (known) return { verb: known.verb, destructive: Boolean(known.destructive) };
  const account = action.match(ACCOUNT_CHANGE);
  if (account) {
    return {
      verb: `Recorded a change to their ${targetLabel(account[1])} account, waiting to be sent`,
      destructive: false,
    };
  }
  const lapsed = action.match(HOLD_LAPSED);
  if (lapsed) {
    return { verb: `A hold on ${targetLabel(lapsed[1])} lifted itself on its date`, destructive: false };
  }
  return { verb: action, destructive: /revoke|delete|remove/i.test(action) };
}

/**
 * Every line names a human or a NAMED machine. The backend writes system
 * actors as "system:onboarding" / "system:scheduler"; rendering the bare
 * string is right — it IS the machine's name — but the prefix is noise.
 */
export function machineName(id: string): string {
  if (!id || id === "-") return "system";
  return id.startsWith("system:") ? `${id.slice(7)} (automatic)` : id;
}

/**
 * What the Trace column can honestly say about one row.
 *
 * `cascade` — the backend recorded which cascade this event set off, so the id
 * is real and the link goes to that cascade and no other.
 *
 * `object` — rows written before cascade lineage existed (migration 000023).
 * They still name the bundle or rule the change was about, which is worth
 * showing; they cannot name its downstream effect, so they do not link. This
 * column previously rendered exactly this identifier with a `c_` prefix and a
 * link to an unfiltered change history — an id that was not what its prefix
 * claimed, pointing at a page that was not about it.
 *
 * `none` — nothing true to say. A dash.
 */
export type AuditTrace =
  | { kind: "cascade"; label: string; href: string }
  | { kind: "object"; label: string; title: string }
  | { kind: "none" };

const LEADING_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;

/**
 * "c_8841" from a uuid — short enough to compare across screens by eye, and the
 * one vocabulary every screen uses: c_ for one edit's set of changes, R_ an
 * automatic rule, b_ a bundle. Every page that shows a handle also carries a
 * one-line legend saying so.
 */
export function shortId(id: string | undefined, prefix: string): string {
  if (!id) return "—";
  return `${prefix}_${id.replace(/-/g, "").slice(0, 4)}`;
}

export function traceFor(entry: AuditEntry): AuditTrace {
  if (entry.cascade_id) {
    return {
      kind: "cascade",
      label: shortId(entry.cascade_id, "c"),
      href: `/operations/cascades?cascade=${encodeURIComponent(entry.cascade_id)}`,
    };
  }

  // Only when the resource IS an id. `bundle.role_added` records its resource as
  // `project/role`, and the first four characters of that are not an identifier of anything —
  // shortening it would produce a label that looks like a handle and refers to nothing.
  const id = entry.resource_id?.match(LEADING_UUID)?.[0];
  if (!id) return { kind: "none" };

  // The caption says what kind of thing the handle names. The full uuid used
  // to sit here, and meant nothing to the people reading it.
  if (entry.action.startsWith("mapping_rule.")) {
    return { kind: "object", label: shortId(id, "R"), title: "Rule" };
  }
  if (entry.action.startsWith("bundle.")) {
    return { kind: "object", label: shortId(id, "b"), title: "Bundle" };
  }
  return { kind: "none" };
}

/**
 * Was this entry something the person did, or something done to them? The
 * Activity tab shows both — a grant made *to* somebody is as much a part of
 * their history as one they made — but the two read differently and must not
 * be presented as if the person acted in both cases.
 */
export function actedOn(entry: AuditEntry, userId: string): "acted" | "affected" | "both" {
  const isActor = entry.actor_id === userId;
  const isTarget = entry.target_id === userId;
  if (isActor && isTarget) return "both";
  return isActor ? "acted" : "affected";
}

/** Groups entries by calendar day, newest first, preserving order within a day. */
export function groupByDay(entries: AuditEntry[]): Array<{ day: string; entries: AuditEntry[] }> {
  const groups: Array<{ day: string; entries: AuditEntry[] }> = [];
  for (const entry of entries) {
    const day = entry.created_at.slice(0, 10);
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.entries.push(entry);
    else groups.push({ day, entries: [entry] });
  }
  return groups;
}
