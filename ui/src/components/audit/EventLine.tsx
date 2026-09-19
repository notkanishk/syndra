"use client";

import { UserName, BundleName } from "@/components/names";
import { describeAction, targetKind, shortId } from "@/lib/audit-vocabulary";
import type { AuditEntry } from "@/lib/queries/useAudit";

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

/** Whether this row names something the sentence can point at. */
export function hasTarget(entry: AuditEntry): boolean {
  return Boolean(entry.target_id) && entry.target_id !== "-" && entry.target_id !== "system";
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
