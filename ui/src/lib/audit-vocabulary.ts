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
  "direct_grant.upserted": { verb: "Granted direct access" },
  "direct_grant.replaced": { verb: "Replaced a direct grant" },
  "direct_grant.revoked": { verb: "Revoked direct access", destructive: true },
  "direct_grant.removed": { verb: "Removed direct access", destructive: true },
  "direct_grant.revoked_by_expiry": { verb: "Removed an expired grant", destructive: true },
  "bundle.created": { verb: "Created a bundle" },
  "bundle.assigned": { verb: "Assigned a bundle" },
  "bundle.unassigned": { verb: "Removed a bundle assignment", destructive: true },
  "bundle.role_added": { verb: "Added a role to a bundle" },
  "bundle.role_removed": { verb: "Removed a role from a bundle", destructive: true },
  "bundle.welcome_set": { verb: "Set the default bundle for new members" },
  welcome_bundle_assigned: { verb: "Assigned the default bundle to a new member" },
  "mapping_rule.created": { verb: "Created an automatic rule" },
  "mapping_rule.updated": { verb: "Changed an automatic rule" },
  "role.created": { verb: "Created a role" },
  "access_request.created": { verb: "Asked for access" },
  "access_request.approved": { verb: "Approved a request" },
  "access_request.rejected": { verb: "Declined a request" },
  "claim_profile.updated": { verb: "Changed a project's token format" },
  "app_claim_override.updated": { verb: "Changed an app's token format" },
  "app_claim_override.deleted": { verb: "Removed an app's token override" },
  "intent.emitted": { verb: "Queued a hardware provisioning intent" },
};

export function describeAction(action: string): { verb: string; destructive: boolean } {
  const known = AUDIT_ACTIONS[action];
  if (known) return { verb: known.verb, destructive: Boolean(known.destructive) };
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
 * The trace column links into Change history, which is the only place to see
 * what an entry actually did downstream. Only cascade-producing actions have
 * one; everything else shows an honest dash.
 */
export function isCascadeTrace(entry: AuditEntry): boolean {
  return (
    Boolean(entry.resource_id) &&
    /^(mapping_rule|bundle)\./.test(entry.action) &&
    entry.action !== "bundle.created"
  );
}

export function shortTrace(id: string): string {
  return `c_${id.replace(/-/g, "").slice(0, 4)}`;
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
