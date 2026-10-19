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
/**
 * `verb` is the sentence with nothing named — used when the row carries no
 * target. `template` is the same sentence with a `{}` slot for the thing it
 * happened to, so a screen renders "Removed a role from the Community · Basic
 * bundle" rather than a verb and a name joined by a dash. What fills the slot
 * is decided by `targetKind`, which is why the slot never says what KIND of
 * thing it is: the resolver already knows.
 *
 * `passive` is the same event told from the TARGET's side, for a screen that
 * is already about that person — their own Activity tab, where naming them in
 * every row repeats the heading above it. "Gave somebody a bundle" was the
 * placeholder that made this necessary: on Gurasheesh's page, that somebody
 * was Gurasheesh. Only the actions a person can be the target of have one.
 */
export const AUDIT_ACTIONS: Record<
  string,
  { verb: string; template?: string; passive?: string; destructive?: boolean }
> = {
  "direct_grant.upserted": { verb: "Gave direct access", template: "Gave {} direct access", passive: "Given direct access" },
  "direct_grant.replaced": { verb: "Replaced somebody's direct access", template: "Replaced {}'s direct access", passive: "Their direct access was replaced" },
  "direct_grant.revoked": { verb: "Revoked direct access", template: "Revoked {}'s direct access", passive: "Their direct access was revoked", destructive: true },
  // The record itself was deleted by hand, as opposed to a revoke that a rule,
  // an expiry or a review set off.
  "direct_grant.removed": { verb: "Revoked direct access by hand", template: "Revoked {}'s direct access by hand", passive: "Their direct access was revoked by hand", destructive: true },
  "direct_grant.revoked_by_expiry": { verb: "Ended access on its end date", template: "Ended {}'s access on its end date", passive: "Their access ended on its end date", destructive: true },
  "bundle.created": { verb: "Created a bundle", template: "Created the {} bundle" },
  "bundle.assigned": { verb: "Gave somebody a bundle", template: "Gave {} a bundle", passive: "Given a bundle" },
  "bundle.unassigned": { verb: "Removed somebody from a bundle", template: "Removed {} from a bundle", passive: "Removed from a bundle", destructive: true },
  "bundle.role_added": { verb: "Added a role to a bundle", template: "Added a role to the {} bundle" },
  "bundle.role_removed": { verb: "Removed a role from a bundle", template: "Removed a role from the {} bundle", destructive: true },
  "bundle.welcome_set": { verb: "Set the default bundle for new members" },
  // Not "Renamed": the endpoint rewrites name and description, and the row records neither.
  // Claiming a rename for a description edit would be a specific falsehood in place of a vague
  // truth.
  "bundle.updated": { verb: "Changed a bundle's name or description" },
  // Same word the console puts on the button. An operator looking for what they just did should
  // find it under the verb they read.
  // `bundle.deleted` is the entry from before Retire kept its history (19 Oct 2026): those rows
  // are still in the log, and now say what happened to the bundle in the same words.
  "bundle.deleted": { verb: "Retired a bundle", destructive: true },
  "bundle.retired": { verb: "Retired a bundle", template: "Retired the {} bundle", destructive: true },
  "bundle.draft_discarded": { verb: "Discarded a bundle's unpublished changes", template: "Discarded the unpublished changes to the {} bundle" },
  "bundle.include_set": { verb: "Included a bundle in another", template: "Changed what the {} bundle includes" },
  "bundle.include_removed": { verb: "Took a bundle out of another", template: "Took a bundle out of the {} bundle" },
  "bundle.version_published": { verb: "Published a bundle version", template: "Published a version of the {} bundle" },
  "bundle.holder_moved": { verb: "Moved somebody to a different bundle version", template: "Moved {} to a different bundle version", passive: "Moved to a different bundle version" },
  welcome_bundle_assigned: { verb: "Gave a new member the default bundle", template: "Gave {} the default bundle, as a new member", passive: "Given the default bundle, as a new member" },
  "mapping_rule.created": { verb: "Created an automatic rule" },
  "mapping_rule.updated": { verb: "Changed an automatic rule" },
  // `mapping_rule.deleted` is the entry from before Retire kept its history: those rows stay.
  "mapping_rule.deleted": { verb: "Deleted an automatic rule", destructive: true },
  "mapping_rule.retired": { verb: "Retired an automatic rule", destructive: true },
  "role.created": { verb: "Created a role" },
  "access_request.created": { verb: "Asked for access" },
  // Roles that wait for you (N14 B): settings, not access, so no passive.
  "role.waits_marked": { verb: "Marked a role to wait for you" },
  "role.waits_unmarked": { verb: "Stopped a role waiting for you" },
  // Term dates (A-06 §7): one makerspace-wide setting.
  "terms.updated": { verb: "Changed the term dates" },
  "confirmation.switched_to_roles": { verb: "Switched to roles that wait" },
  "access_request.approved": { verb: "Approved a request", template: "Approved {}'s request", passive: "Their request was approved" },
  // Allowed and said (owner, 15 Oct): an operator approving their own request.
  "access_request.self_approved": { verb: "Approved their own request", passive: "They approved their own request" },
  "access_request.rejected": { verb: "Declined a request", template: "Declined {}'s request", passive: "Their request was declined" },
  // Not destructive, and deliberately not phrased as a refusal — the person who filed it took it
  // back. `requestOutcome` keeps the same distinction on the request screens.
  "access_request.withdrawn": { verb: "Withdrew their request" },
  // Closed by Syndra on 15 Oct (migration 54): a request is for a role someone
  // lacks, one per role.
  "access_request.closed_as_copy": {
    verb: "Closed a copy of a waiting request",
    passive: "A copy of their waiting request was closed",
  },
  // An action cancelled a change still waiting for you to send, after the operator
  // was shown it (owner, 15 Oct: never invisibly). resource_id is the outbox row.
  // Worded the operator's way (A-04 §14.8). The row carries who caused it, the person
  // and the outbox row, but not what the change was, who queued it or why: those are
  // on the superseded outbox row, which no read returns yet.
  "propagation.cancelled_unsent": {
    verb: "Cancelled a change that was waiting for you to send",
    template: "Cancelled a change that was waiting for you to send, for {}",
    passive: "A change that was waiting for you to send was cancelled",
  },
  "access_request.closed_as_given": {
    verb: "Closed a request for a role given another way",
    passive: "Their request was closed: the role was given another way",
  },
  "access_request.closed_as_held": {
    verb: "Closed a request for a role they already had",
    passive: "Their request was closed: they already had it",
  },
  // Not destructive: the access was already going to lapse. What was recorded is that somebody
  // looked, which is the opposite of something being taken away unnoticed.
  "grant_expiry.acknowledged": { verb: "Recorded that access that ends should be left to end" },
  "grant_expiry.acknowledgement_cleared": { verb: "Put access that ends back in the undecided list" },
  "claim_profile.updated": { verb: "Changed a project's token format" },
  "app_claim_override.updated": { verb: "Changed an app's token format" },
  "app_claim_override.deleted": { verb: "Removed an app's token override" },
};

/**
 * The two families the backend builds by concatenation, with the connected
 * system's name in the middle: a change to somebody's account on TrueNAS
 * recorded and waiting for you to send, and a hold that lifted itself on its date.
 * Neither can be a static key above, so they are matched by shape.
 */
const ACCOUNT_CHANGE = /^entitlement\.([a-z0-9_-]+)\.enqueued$/;
const HOLD_LAPSED = /^allowance\.([a-z0-9_-]+)\.lapsed$/;

export function describeAction(
  action: string,
): { verb: string; template?: string; passive?: string; destructive: boolean } {
  const known = AUDIT_ACTIONS[action];
  if (known) {
    return {
      verb: known.verb,
      template: known.template,
      passive: known.passive,
      destructive: Boolean(known.destructive),
    };
  }
  const account = action.match(ACCOUNT_CHANGE);
  if (account) {
    return {
      verb: `Recorded a change to their ${targetLabel(account[1])} account, waiting for makerspace staff to send it`,
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

/**
 * What kind of thing `entry.target_id` names, so the audit row asks the right
 * resolver for it.
 *
 * `target_id` is not always a person: `bundle.role_added`/`role_removed`/
 * `draft_discarded` record the bundle itself there (see db.CascadeAudit), and a raw bundle uuid
 * handed to the person resolver renders as "Unknown account <uuid>" — a
 * confident lie about an id that was never a person's in the first place.
 * `mapping_rule.*` writes no target today, but is matched here too so that
 * changes on the write side once again can't silently regress an id no
 * resolver here has ever been asked to explain.
 */
export function targetKind(action: string): "user" | "bundle" | "rule" {
  if (
    action === "bundle.role_added" ||
    action === "bundle.role_removed" ||
    action === "bundle.draft_discarded" ||
    action === "bundle.include_set" ||
    action === "bundle.include_removed"
  )
    return "bundle";
  if (action.startsWith("mapping_rule.")) return "rule";
  return "user";
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
  // Only a row whose target is a PERSON can have this person as its target.
  // `target_id` holds a bundle id on a bundle edit, and comparing a person's
  // id against one is a comparison that can only ever answer "no" — which
  // this function then reported as "somebody else did this TO them".
  const isTarget = targetKind(entry.action) === "user" && entry.target_id === userId;
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
