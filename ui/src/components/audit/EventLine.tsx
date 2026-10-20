"use client";

import { UserName, BundleName, ProjectName } from "@/components/names";
import { describeAction, targetKind, shortId } from "@/lib/audit-vocabulary";
import { formatRelative, humanizeKey } from "@/lib/format";
import type { AuditEntry, CancelledUnsentDetails } from "@/lib/queries/useAudit";
import { useNameResolver } from "@/lib/queries/useNameResolver";

/**
 * One audit event, as a sentence — THE one renderer, for every screen that
 * shows an event.
 *
 * There were two. The audit page knew that `target_id` is only sometimes a
 * person (a bundle edit stores a bundle id in the same column) and dispatched
 * on the action to pick a resolver; the home page's "Lately" panel did not,
 * and handed every id to the person resolver. So one screen read "Removed a
 * role from a bundle — Community · Basic" while the other read the same row as
 * "— Unknown account 9e93893d-…": the same event, in two places, disagreeing
 * about both what happened and to whom.
 *
 * The dispatch is the whole reason this file exists, so it lives with the
 * renderer rather than beside one of its callers. A screen that shows an event
 * imports this and gets the resolution for free; there is nowhere left to
 * forget it.
 */

/**
 * `entry.target_id` is only sometimes a person. A raw uuid must never be the
 * visible label, so a bundle or rule id renders through its own name (or a
 * "retired" fallback plus its short handle) rather than through the person
 * resolver, which would report it as an unknown account.
 */
export function TargetRef({ entry }: { entry: AuditEntry }) {
  const kind = targetKind(entry.action);
  if (kind === "bundle") {
    return (
      <BundleName
        id={entry.target_id}
        fallback={`a retired bundle (${shortId(entry.target_id, "b")})`}
      />
    );
  }
  if (kind === "rule") {
    return <>{`a rule (${shortId(entry.target_id, "R")})`}</>;
  }
  return <UserName id={entry.target_id} />;
}

/** "system" or "system:scheduler": work Syndra did on its own, with no operator to ask. */
function isSystemActor(id: string): boolean {
  return !id || id === "-" || id === "system" || id.startsWith("system:");
}

/** Whether this row names something the sentence can point at. */
export function hasTarget(entry: AuditEntry): boolean {
  return Boolean(entry.target_id) && entry.target_id !== "-" && entry.target_id !== "system";
}

const ACT: Record<string, string> = { grant: "gave it back", remove: "removed it", approve: "approved the request" };

/** A first name, never a pronoun: "someone" until the name resolves. */
function FirstName({ id }: { id: string }) {
  const name = useNameResolver().resolveUser(id).value?.display_name?.trim().split(/\s+/)[0];
  return <>{name || "someone"}</>;
}

/** "Network storage · Member": the place, then the role by its own name. */
export function PlaceRole({ projectId, roleKey }: { projectId: string; roleKey: string }) {
  const role = useNameResolver().resolveRole(projectId, roleKey).value?.display_name ?? humanizeKey(roleKey);
  return (
    <>
      <ProjectName id={projectId} /> · {role}
    </>
  );
}

/**
 * A cancellation that carries what it cancelled (A-04 §14.8): who cancelled, the change,
 * who queued it and when, the reason, and the act that caused it. Syndra's own names no
 * act and says nobody was asked. A cause through a bundle or rule is not named: the entry
 * carries the kind of path, not which bundle or rule.
 */
function CancelledChange({ entry, details }: { entry: AuditEntry; details: CancelledUnsentDetails }) {
  const { cancelled: c, cause } = details;
  const system = isSystemActor(entry.actor_id);
  const queuer = c.initiated_by && !isSystemActor(c.initiated_by) ? c.initiated_by : null;
  const give = c.kind === "give";
  const act = ACT[cause?.action];
  return (
    <span className="flex flex-col gap-1">
      <span>
        {/* The actor has its own seat beside every entry, as for every other verb; Syndra is named. */}
        {system ? "Syndra cancelled" : "Cancelled"} a change that was waiting for you to send
      </span>
      <span>
        {give ? "Giving " : "Removing "}
        <PlaceRole projectId={c.project_id} roleKey={c.role_key} />
        {give ? " to " : " from "}
        <UserName id={c.user_id || entry.target_id} />
      </span>
      <span className="text-faint">
        Queued by {queuer ? <UserName id={queuer} /> : "Syndra"} · {formatRelative(c.queued_at)}
      </span>
      {c.reason ? (
        <span>&ldquo;{c.reason}&rdquo;</span>
      ) : queuer ? (
        <span className="text-faint">
          <FirstName id={queuer} /> gave no reason.
        </span>
      ) : null}
      {system ? (
        <span className="text-faint">Nobody was asked.</span>
      ) : act ? (
        <span className="text-faint">
          cancelled when <FirstName id={entry.actor_id} /> {act}
        </span>
      ) : null}
    </span>
  );
}

/**
 * The event in words, with the thing it happened to named INSIDE the sentence
 * rather than appended after a dash.
 *
 * "Removed a role from a bundle — Community · Basic" makes a reader do the
 * joining themselves, and reads as two facts; "Removed a role from the
 * Community · Basic bundle" is the one fact it always was. `describeAction`
 * supplies the template with a slot; an action with no template, or a row with
 * no target, falls back to the bare verb rather than inventing a phrasing for
 * something this vocabulary does not recognise.
 */
export function EventSentence({ entry }: { entry: AuditEntry }) {
  const { verb, template, destructive } = describeAction(entry.action);
  const tone = destructive ? "font-semibold text-danger-text" : undefined;

  const d = entry.details as CancelledUnsentDetails | undefined;
  if (entry.action === "propagation.cancelled_unsent" && d?.cancelled) {
    return <CancelledChange entry={entry} details={d} />;
  }

  // Syndra's own cancellation (expiry, onboarding, a rule) names Syndra, and says
  // nobody was asked: an operator's was always shown first, this one has no one to show.
  if (entry.action === "propagation.cancelled_unsent" && isSystemActor(entry.actor_id)) {
    return (
      <span>
        Syndra {verb.charAt(0).toLowerCase()}
        {verb.slice(1)}
        {hasTarget(entry) ? (
          <>
            , for <TargetRef entry={entry} />
          </>
        ) : null}
        <span className="text-faint"> · Nobody was asked.</span>
      </span>
    );
  }

  if (!hasTarget(entry)) return <span className={tone}>{verb}</span>;

  if (!template) {
    // An action whose phrasing nobody has written a slot for. Naming the thing
    // after a dash is worse than the template and far better than dropping it.
    return (
      <span className={tone}>
        {verb} — <TargetRef entry={entry} />
      </span>
    );
  }

  const [before, after] = template.split("{}");
  return (
    <span className={tone}>
      {before}
      <TargetRef entry={entry} />
      {after}
    </span>
  );
}
