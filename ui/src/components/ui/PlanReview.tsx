"use client";

import Link from "next/link";
import { useRef, useState } from "react";

import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import { OUTCOME_LABEL, PLAN_EFFECT_LABEL, PLAN_EFFECT_TONE, REVOKE_SENDS_ITSELF_WORD, type AutoRetry } from "@/lib/outcome";
import { goesItself, groupPlan, onlyCancels, refusedSeat, splitCancels, type PlanGroup } from "@/lib/plan-groups";
import { Badge, StatusDot } from "@/components/ui/Badge";
import { FoldHeading } from "@/components/ui/FoldHeading";
import { NextStep } from "@/components/ui/NextStep";
import { PAGE_FIRST, PAGE_MORE, PaginationRow } from "@/components/ui/PaginationRow";
import { RetryStatus } from "@/components/ui/RetryStatus";
import { UserName } from "@/components/names/UserName";
import { sortByName } from "@/lib/sort";


// The six words live in lib/outcome, shared with every other surface that
// reports what happened. They were declared here first, privately, which is
// how a plan and the row that ran it could have come to describe the same
// result in two vocabularies. `queued` keeps the warning tone rather than the
// accent one — recorded here, not yet at the target.

/**
 * The rehearsal, rendered.
 *
 * Every bulk operation in the product — granting a role to a cohort, resolving
 * a drift queue, deciding a batch of requests — computes the same plan shape on
 * the server and shows it through this one component. That is deliberate: an
 * operator should not have to learn what "will change" looks like separately on
 * each screen, and a preview drawn by different code from the write it previews
 * is a preview of nothing.
 *
 * Rows are never identified by id. A plan an operator cannot read is a plan
 * they will approve without reading.
 */
export function PlanReview({
  plan,
  noun = ["person", "people"],
  auto = true,
  system = "Zitadel",
}: {
  plan: BulkPlan | null;
  /** Whether this server sends changes again on its own: which fold a queued row sits in after Apply. */
  auto?: AutoRetry;
  /** The system the change is sent to, for the words (copy guide §3.10). */
  system?: string;
  /**
   * What this plan's rows ARE, in the caller's own vocabulary.
   *
   * `RehearsalDialog` is driven with `["account", "accounts"]` by entitlement
   * convergence, `["item", "items"]` by drift triage and `["request",
   * "requests"]` by the requests screen — and it never passed any of that down
   * here, so a short payload on the requests screen reported a missing list of
   * *people*. The default is the common case rather than a guess: most surfaces
   * really do act on people.
   */
  noun?: [string, string];
}) {
  if (!plan) return null;

  // `?? []`, as everywhere else that reads a list off a payload. A plan with no
  // outcomes array means the backend is wrong, and the right failure for that
  // is a plan showing no rows beside its summary — not an error boundary
  // blanking the screen an operator is standing on halfway through approving a
  // change. This crashed the page the first time a payload arrived short.
  //
  // The two empties are told apart. An ABSENT array is a payload that arrived
  // short; an EMPTY one is a change that reaches nobody, which on the mapping
  // surfaces is an ordinary act — a definition written before anybody holds the
  // role. Rendering the same sentence for both would tell an operator their
  // perfectly good plan was broken.
  const missing = plan.outcomes === undefined || plan.outcomes === null;
  const outcomes = plan.outcomes ?? [];

  // What the plan would cancel, once, under it: it goes stale if anyone queues
  // another change first, because the rows named exactly these.
  const cancels = outcomes.map((o) => splitCancels(o.consequence)).filter((c) => c !== null);
  const cancelled = cancels.reduce((n, c) => n + c.count, 0);
  // Grouped by how each change travels when every row that goes ahead says
  // so (A-21 §Q1); flat otherwise, as every plan was before.
  const groups = missing ? null : groupPlan(plan, noun, auto, system);

  return (
    <div className="px-6">
      <div className="max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong">
        {missing ? (
          <p className="px-4 py-3 text-[13.5px] text-faint">
            The list of {noun[1]} did not load. The totals under the buttons are all that came
            back.
          </p>
        ) : outcomes.length === 0 && !plan.excluded?.length ? (
          <p className="px-4 py-3 text-[13.5px] text-muted">{emptyNote(plan.op)}</p>
        ) : (
          <>
            {groups
              ? groups.map((group) => <PlanFold key={group.key} group={group} plan={plan} system={system} />)
              : sortByName(outcomes, (o) => o.name || o.email || "", (o) => o.user_id).map((outcome) => (
                  <PlanRow key={outcome.user_id} outcome={outcome} applied={plan.applied} word={queuedWord(outcome, plan.op, auto, system)} />
                ))}
            {/* #32: the requester, left out of anything that takes access away. */}
            {plan.excluded?.map((left) => (
              <div key={left.user_id} className="row-divider flex items-start gap-4 px-4 py-3">
                <span className="min-w-0 flex-1">
                  <span className="block text-[14.5px] font-semibold">
                    <UserName id={left.user_id} />
                  </span>
                  <span className="block text-[13px] text-muted">
                    Left out · this is you. Change your own access from{" "}
                    <Link
                      href={`/users/${encodeURIComponent(left.user_id)}`}
                      className="font-semibold text-accent-text link-hover"
                    >
                      your page
                    </Link>
                    .
                  </span>
                </span>
              </div>
            ))}
          </>
        )}
      </div>
      {cancelled > 0 && (
        <p role="note" className="mt-3 text-[13px] leading-[1.5] text-warn-text">
          Cancels {cancelled === 1 ? "a change that is" : `${cancelled} changes that are`} waiting for you to
          send: {cancels.map((c) => c.parts).join("; ")}. If anyone queues another first, this plan goes stale
          and nothing is cancelled unseen.
        </p>
      )}
    </div>
  );
}

/**
 * A queued row's word once the plan has run, by who sends it (goesItself, the
 * reading the folds and the result sentence use): on its way when Syndra sends
 * it, waiting for you when you do, not yet in the system while that is unknown.
 */
function queuedWord(outcome: BulkOutcome, op: BulkPlan["op"], auto: AutoRetry, system: string): string {
  const goes = goesItself(outcome, op, auto);
  if (goes === null) return `Not in ${system} yet`;
  return goes ? REVOKE_SENDS_ITSELF_WORD : OUTCOME_LABEL.queued;
}

function PlanRow({ outcome, applied, word }: { outcome: BulkOutcome; applied: boolean; word: string }) {
  // Before the apply, a queued row forecasts ("Will wait for you to send");
  // once the plan has run it is true, and says who sends it (queuedWord).
  const forecast = outcome.effect === "queued" && !applied;
  const label =
    outcome.effect === "queued" && applied
      ? word
      : (PLAN_EFFECT_LABEL[outcome.effect] ?? PLAN_EFFECT_LABEL.no_change);
  const tone = PLAN_EFFECT_TONE[outcome.effect] ?? PLAN_EFFECT_TONE.no_change;

  return (
    <div className="row-divider flex items-start gap-4 px-4 py-3">
      <span className="min-w-0 flex-1">
        <span className="block text-[14.5px] font-semibold">
          {/* A plan names people when its backend surface could; otherwise the
              resolver does, from the id every outcome carries. Never a dead
              "Name not available": that read as a broken group or person. */}
          {outcome.name || outcome.email || <UserName id={outcome.user_id} />}
        </span>
        <span className="block text-[13px] text-muted">
          {outcome.detail}
          {outcome.consequence ? (
            <>
              {" "}
              <span className="text-faint">
                {(() => {
                  const said = splitCancels(outcome.consequence);
                  return said ? [said.rest, said.line].filter(Boolean).join(" ") : outcome.consequence;
                })()}
              </span>
            </>
          ) : null}
        </span>
      </span>
      <Badge tone={tone} className="shrink-0 gap-1.5">
        {/* A change that will wait wears the amber dot and says so in the future
            tense. No link: nothing is queued yet, so there is nothing to send. */}
        {forecast && <StatusDot tone="warn" />}
        {label}
      </Badge>
    </div>
  );
}

/**
 * One fold of a grouped plan (A-21 §2.2, §2.6): its heading, sticky at the
 * top of the box so the glance survives scrolling, then its people, ten at a
 * time. Open by default; the heading says which, the row says what.
 */
function PlanFold({ group, plan, system }: { group: PlanGroup; plan: BulkPlan; system: string }) {
  const [open, setOpen] = useState(true);
  const [limit, setLimit] = useState(PAGE_FIRST);
  const anchor = useRef<HTMLDivElement>(null);
  const shown = group.rows.slice(0, limit);
  return (
    <div role="group" aria-label={`${group.heading}, ${group.count}`}>
      <div ref={anchor} className="row-divider sticky top-0 bg-surface-0">
        <FoldHeading
          open={open}
          onClick={() => setOpen(!open)}
          title={group.heading}
          count={group.count}
          dot={group.dot}
          bloom={group.bloom}
          note={group.note}
          href={group.link ? "/governance/pending" : undefined}
        />
      </div>
      {open && (
        <>
          {shown.map((outcome) => (
            <FoldRow key={outcome.user_id} outcome={outcome} group={group.key} op={plan.op} system={system} />
          ))}
          <PaginationRow
            shown={shown.length}
            total={group.rows.length}
            onMore={() => setLimit((n) => n + PAGE_MORE)}
            onAll={() => setLimit(group.rows.length)}
            onBack={() => setLimit(PAGE_FIRST)}
            anchor={anchor}
          />
        </>
      )}
    </div>
  );
}

/**
 * A person in a fold: the name, then the planner's words exactly as written,
 * before Apply and after (the settle keeps them). No badge except under "Left
 * as they are", where each row keeps its own word. After Apply, the row's seat
 * says its state: the link to send it, its retry, or the refusal and its step.
 */
function FoldRow({ outcome, group, op, system }: { outcome: BulkOutcome; group: PlanGroup["key"]; op: BulkPlan["op"]; system: string }) {
  const said = splitCancels(outcome.consequence);
  // A row that only cancels says so in its own one sentence (A-21 §2.4); the
  // cancels line under the box still names what it cancels.
  const words = said ? (onlyCancels(outcome) ? said.rest : [said.rest, said.line].filter(Boolean).join(" ")) : outcome.consequence;
  const applied = outcome.effect !== "apply" && outcome.effect !== "no_change" && outcome.effect !== "blocked";
  return (
    <div className="row-divider px-4 py-3">
      <div className="flex flex-wrap items-start gap-x-4 gap-y-1">
        <span className="min-w-0 flex-1 text-[14.5px] font-semibold">
          {outcome.name || outcome.email || <UserName id={outcome.user_id} />}
        </span>
        {group === "left" && (
          <Badge tone={PLAN_EFFECT_TONE[outcome.effect] ?? PLAN_EFFECT_TONE.no_change} className="ml-auto shrink-0">
            {PLAN_EFFECT_LABEL[outcome.effect] ?? PLAN_EFFECT_LABEL.no_change}
          </Badge>
        )}
      </div>
      <span className="block text-[13px] text-muted">
        {outcome.detail}
        {words ? <> {words}</> : null}
      </span>
      {applied && group === "wait" && <NextStep person={outcome.person || outcome.user_id} className="mt-1.5" />}
      {applied && group === "unanswered" && outcome.retry && <RetryStatus retry={outcome.retry} className="mt-1.5" />}
      {group === "refused" && <p className="mt-1.5 text-[13px] leading-[1.5] text-danger-text">{refusedSeat(outcome, op, system)}</p>}
    </div>
  );
}

/**
 * What the empty-cohort sentence is ABOUT, from the plan's own op.
 *
 * The sentence said "this role" to everybody, which was true while only the
 * mapping surfaces could reach it. Publishing a bundle version nothing holds
 * yet reaches it too, and told the operator about a role on a screen that has
 * no role on it. Read from `op` rather than taken as a prop: the plan already
 * says what it is, and a prop would be a second place for a caller to get it
 * wrong.
 */
/**
 * The whole empty-cohort sentence, from the plan's own op.
 *
 * It said "this role" to everybody, which was right while only the mapping
 * surfaces reached it and wrong the moment bundle publishing did — a bundle
 * screen has no role on it.
 *
 * The mapping surfaces genuinely do say "role", and that is worth stating
 * because the obvious correction gets it backwards: a mapping is not something
 * anybody HOLDS. It hangs off a (project, role) pair and describes what that
 * role reaches, so the population who feel it are the people holding the role.
 * "Nobody holds this mapping yet" is the wrong sentence, and a test caught it.
 *
 * A rollback is the one that needs its own words. It restores a whole target's
 * SET of mappings across several roles, so no singular noun fits the template
 * at all.
 *
 * Exhaustive over the union rather than defaulted, because the first cut WAS
 * defaulted and rollback fell through it. A switch makes the next op added to
 * the union fail the typecheck instead of quietly inheriting somebody else's
 * sentence.
 */
function emptyNote(op: BulkPlan["op"]): string {
  switch (op) {
    case "publish_bundle_version":
    case "move_bundle_holders":
    case "assign_bundle":
    case "remove_bundle":
      return "Nobody holds this bundle yet, so nobody's access changes today. This only changes what the bundle gives to people who hold it later.";
    case "rollback_mappings":
      return "None of the roles in this version reach anybody yet, so nobody's access changes today. This only changes what they will reach for people who hold them later.";
    case "assign_role":
    case "remove_role":
    case "extend":
    case "create_mapping":
    case "edit_mapping":
    case "delete_mapping":
      return "Nobody holds this role yet, so nobody's access changes today. This only changes what the role gives to people who hold it later.";
  }
}
