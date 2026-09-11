"use client";

import { useState } from "react";

import Link from "next/link";

import { AccessSource } from "@/components/access/AccessSource";
import { Makerspace } from "@/components/home/Makerspace";
import { ErrorState, RowSkeleton } from "@/components/states";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader, CardHeaderLink, CardRow } from "@/components/ui/Card";
import { Term } from "@/components/ui/Term";
import { RoleRef, UserAvatar, UserName } from "@/components/names";
import {
  useGovernanceSummary,
  type UnreconciledTarget,
  type UnconfirmedWriteSummary,
} from "@/lib/queries/useGovernance";
import {
  useDecideRequest,
  useRequestsAdmin,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { useDrainPropagations } from "@/lib/queries/usePropagation";
import { useMissedOnboarding, type MissedOnboardingReport } from "@/lib/queries/useOperations";
import { useTargets } from "@/lib/queries/useTargets";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useCreateGrant } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import { useIsAdvanced } from "@/lib/ui-view";
import { targetLabel } from "@/lib/nav";
import { ClockTime, Relative } from "@/components/ui/Time";
import { formatShortDate, formatWeekday, daysUntil } from "@/lib/format";

/**
 * Home — the operator landing. Two zones, in a fixed order.
 *
 * It was called Today, in the rail and here, and the name was retired in both
 * places at once so that "Today" now means exactly one thing in this codebase:
 * the day. The page outgrew it. The queue on top IS today's work — a 14-day
 * expiry horizon, the weekday, "last checked" — but the makerspace zone below
 * is not day-scoped at all, so a name promising a day described half the
 * screen. Home is the one position-word in that rail which is also a true
 * name, because this page's identity is that it is where you land.
 *
 * Where "today" still appears below it is the day and nothing else: today's
 * work, what expires today. It is never the name of this page.
 *
 * **Work, on top.** Open requests, expiring access, and — in Advanced — pending
 * changes and unexplained access. Every block here is something you can finish.
 * Nothing is ever inserted above this zone, and nothing below it can push it
 * down: the queue is the first thing under the cursor or the page has failed.
 *
 * **The makerspace, below.** Always present, including on a day when the queue
 * is empty.
 *
 * That second zone is a deliberate revision of this page's original contract,
 * which read "actionable work only — no counts you cannot act on, no charts".
 * That rule was right about the top of the page and wrong about the rest of it:
 * it assumed a non-empty queue, and most days the queue is empty. An operator
 * landing on "Nothing here needs you." and nothing else learns nothing about the
 * space they run, so they go hunting through the nav — which is the navigation
 * this page exists to prevent. The half of the rule worth keeping is kept, and
 * enforced below: **every number is a link into the thing it counts.** Still no
 * charts, still nothing you can only look at.
 *
 * Basic shows two work blocks. Advanced appends two more, because Pending
 * changes and Unexplained access belong to the machine, and the machine lives
 * in Advanced.
 */
export function Home({ session }: { session: SessionUser }) {
  const advanced = useIsAdvanced();
  const summary = useGovernanceSummary();
  const requests = useRequestsAdmin("pending");
  const onboarding = useMissedOnboarding();

  const pending = requests.data ?? [];
  const expiring = summary.data?.expiring_grants ?? [];
  const propagation = summary.data?.pending_propagation;
  const drift = summary.data?.drift;
  const unvouched = summary.data?.unreconciled_targets ?? [];
  // Differences a reconciliation refused to resolve. Counted for the same
  // reason the unreadable targets are: each one is a person's decision that has
  // not been made, and a page that omits them says "nothing needs you" while
  // somebody's access sits disputed.
  const findings = summary.data?.merge_findings ?? 0;
  // Writes Zitadel accepted that no later read has been able to see, old
  // enough that the gap is a finding rather than Zitadel's read projection
  // catching up (see UnconfirmedWrites — the threshold lives on the backend
  // so this page never invents its own answer to "how long is too long").
  const unconfirmedWrites = summary.data?.unconfirmed_writes ?? { count: 0, top: [] };
  // Two different gaps that both mean "somebody joined and got nothing": no
  // default is set at all (1, a fact about right now, not a person), and
  // named people the reconciler finds holding no bundle when a default DOES
  // exist. See OnboardingGaps — this must never be inferred from the trigger
  // log, which only knows what a webhook happened to report.
  const onboardingGaps =
    (onboarding.data && !onboarding.data.welcome_bundle_configured ? 1 : 0) +
    (onboarding.data?.missed.length ?? 0);

  // Counted, and that is the whole point of it. An unreadable target produces
  // no drift findings, so a week of silence lands here as blocks === 0 and the
  // page says "Nothing here needs you" — the one sentence that must never be said
  // about a system nobody has been able to look at.
  const blocks = advanced
    ? pending.length +
      expiring.length +
      onboardingGaps +
      (propagation?.count ?? 0) +
      unconfirmedWrites.count +
      (drift?.count ?? 0) +
      unvouched.length +
      findings
    : pending.length + expiring.length + onboardingGaps;

  const who = firstName(session);
  const loading = summary.isLoading || requests.isLoading || onboarding.isLoading;
  const error = summary.error ?? requests.error ?? onboarding.error;
  // While the load is failing the count is unknown, and "nothing needs you" is
  // exactly the wrong thing to say to somebody whose queue might be full.
  const headline = error ? "Couldn't check." : workSentence(blocks, loading);

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="type-greeting">
          {greeting()}
          {who ? `, ${who}` : ""}. <span className="text-ink/40">{headline}</span>
        </h1>
        <div className="mt-2 text-[14.5px] text-faint">
          {formatWeekday()} · last checked <ClockTime at={summary.dataUpdatedAt} />
        </div>
      </div>

      {error ? (
        <ErrorState
          title="Couldn't load today's work."
          error={error}
          onRetry={() => {
            summary.refetch();
            requests.refetch();
            onboarding.refetch();
          }}
        />
      ) : loading ? (
        <Card>
          <RowSkeleton rows={4} label="Loading today's work" />
        </Card>
      ) : blocks === 0 ? (
        // One calm line, not a full-page card: the page continues below, and a
        // hero-sized empty state would read as the end of it.
        <div className="panel flex items-center gap-3 px-5 py-4">
          {/* Healthy, not accent. Violet is what you can act on, and the whole
              statement of this row is that there is nothing to act on. */}
          <span aria-hidden className="h-2.5 w-2.5 rounded-pill bg-healthy" />
          <span className="text-[14.5px]">
            <strong className="font-semibold">Nothing here needs you.</strong>{" "}
            <span className="text-muted">
              No open requests, and no access expiring in the next 14 days. Checked <ClockTime />.
            </span>
          </span>
        </div>
      ) : (
        <>
          {pending.length > 0 && <OpenRequests requests={pending} />}
          {expiring.length > 0 && <ExpiringSoon grants={expiring} />}
          {onboardingGaps > 0 && <OnboardingGaps report={onboarding.data!} />}

          {advanced && (propagation?.count ?? 0) > 0 && (
            <PendingChanges
              count={propagation!.count}
              reachable={propagation!.zitadel_reachable}
            />
          )}
          {advanced && unconfirmedWrites.count > 0 && (
            <UnconfirmedWrites summary={unconfirmedWrites} />
          )}
          {/* Above unexplained access, because it qualifies it: a target Syndra
              cannot read contributes no findings, so the count below is a
              statement about the targets it COULD read. */}
          {advanced && unvouched.length > 0 && <UnvouchedTargets targets={unvouched} />}
          {advanced && (drift?.count ?? 0) > 0 && <UnexplainedAccess count={drift!.count} />}
          {advanced && findings > 0 && <MergeFindingsWaiting count={findings} />}

          {!advanced && (
            <p className="max-w-[820px] text-[14px] leading-[1.55] text-faint">
              In Basic these blocks are your whole queue. Pending changes and unexplained access
              aren&rsquo;t hidden from you — they belong to the machine, and the machine lives in
              Advanced.
            </p>
          )}
        </>
      )}

      {/* The work zone above never moves. This is appended, never inserted. */}
      {!error && !loading && <Makerspace />}
    </div>
  );
}

/** Approve / Deny resolve in place, on the row. They never navigate. */
function OpenRequests({ requests }: { requests: AccessRequest[] }) {
  const decide = useDecideRequest();
  const [resolved, setResolved] = useState<Set<string>>(new Set());
  const [outcomes, setOutcomes] = useState<Record<string, Outcome | null>>({});

  const visible = requests.filter((entry) => !resolved.has(entry.id));
  if (visible.length === 0) return null;

  async function act(id: string, status: "approved" | "rejected", who: string) {
    setResolved((prev) => new Set(prev).add(id));
    try {
      await decide.mutateAsync({ id, status });
      setOutcomes((prev) => ({
        ...prev,
        [id]: {
          kind: "applied",
          message: status === "approved" ? `Approved for ${who}` : `Declined for ${who}`,
        },
      }));
    } catch (error) {
      // Put the row back: a row that vanished on a failed write would read as
      // a decision that was recorded.
      setResolved((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
      setOutcomes((prev) => ({ ...prev, [id]: outcomeFromError(error) }));
    }
  }

  return (
    <Card>
      <CardHeader
        title="Open requests"
        count={visible.length}
        action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>}
      />
      {/* `arrive`, the same stagger every list in the product gets from
          `ListStates`. These two lists compose their own rows and so had none:
          the dashboard was the one screen whose rows appeared all at once. */}
      <div className="contents arrive-list">
        {visible.map((entry) => (
          <CardRow key={entry.id}>
            <UserAvatar id={entry.requester_id} size="list" />
            <div className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold">
              <UserName id={entry.requester_id} />
          </div>
          <div className="w-full sm:w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
            <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
          </div>
          <div className="min-w-0 flex-1 truncate text-[14px] text-muted">
            {entry.justification ? `“${entry.justification}”` : "No reason given"}
          </div>
          <div className="w-[66px] shrink-0 text-[13px] text-faint">
            <Relative iso={entry.created_at} />
          </div>
          <div className="flex shrink-0 gap-2">
            <Button
              variant="accent"
              size="sm"
              onClick={() => act(entry.id, "approved", entry.requester_id)}
            >
              Approve
            </Button>
            <Button size="sm" onClick={() => act(entry.id, "rejected", entry.requester_id)}>
              Decline
            </Button>
          </div>

          {outcomes[entry.id] && (
            <ActionOutcome outcome={outcomes[entry.id]!} placement="inline" className="w-full" />
          )}
        </CardRow>
      ))}
      </div>
    </Card>
  );
}

/**
 * Extending is the only action. Doing nothing is not an action and is not
 * drawn as one — the row states the outcome of inaction in words.
 */
function ExpiringSoon({
  grants,
}: {
  grants: Array<{ id: string; user_id: string; project_id: string; role_key: string; expires_at?: string | null }>;
}) {
  return (
    <Card>
      <CardHeader
        title="Direct access expiring soon"
        count={grants.length}
        tone="warn"
        note="Extending is the only action — doing nothing lets it lapse."
      />
      {grants.map((grant) => (
        <ExpiringRow key={grant.id} grant={grant} />
      ))}
    </Card>
  );
}

function ExpiringRow({
  grant,
}: {
  grant: { id: string; user_id: string; project_id: string; role_key: string; expires_at?: string | null };
}) {
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role), so this renews in place rather than duplicating.
  const extend = useCreateGrant(grant.user_id);
  const remaining = daysUntil(grant.expires_at);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  return (
    <CardRow>
      <UserAvatar id={grant.user_id} size="list" />
      <div className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold">
        <UserName id={grant.user_id} />
      </div>
      <div className="w-full sm:w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
        <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="truncate text-[14px] text-muted">
          No action — expires {formatShortDate(grant.expires_at)}
        </span>
      </div>
      <div className="w-[66px] shrink-0 text-[13.5px] font-semibold text-warn-text">
        {remaining === null ? "—" : `${remaining} days`}
      </div>
      <Button
        size="sm"
        isPending={extend.isPending}
        onClick={async () => {
          try {
            await extend.mutateAsync({
              project_id: grant.project_id,
              role_key: grant.role_key,
              reason: "Extended from Home",
              duration_days: 90,
            });
            setOutcome({
              kind: "applied",
              message: "Extended by 90 days",
              detail: "The row leaves this block on the next read.",
            });
          } catch (error) {
            setOutcome(outcomeFromError(error));
          }
        }}
      >
        Extend
      </Button>

      {outcome && <ActionOutcome outcome={outcome} placement="inline" className="w-full" />}
    </CardRow>
  );
}

/**
 * Two different gaps, both meaning "somebody joined and got nothing" — never
 * read from the onboarding trigger log, which only knows what a webhook
 * happened to report (see `services.FindMissedOnboarding`).
 *
 * `!welcomeBundleConfigured` is a fact about right now, not a person: nobody
 * is named because nobody CAN be checked against a default that does not
 * exist, and it may be deliberate — this states the fact and links to where
 * it is decided, rather than guessing at intent. `missed` is the opposite:
 * named people Zitadel confirms are active, checked against who the default
 * <Term name="bundle">bundle</Term> was actually given, this moment.
 */
function OnboardingGaps({ report }: { report: MissedOnboardingReport }) {
  const count = (report.welcome_bundle_configured ? 0 : 1) + report.missed.length;
  return (
    <Card>
      <CardHeader title="New people without a bundle" count={count} />
      {!report.welcome_bundle_configured && (
        <CardRow>
          <div className="flex-1 text-[14.5px]">
            No default <Term name="bundle">bundle</Term> is set for new members
            <span className="text-faint"> — anybody who joins gets nothing until one is set</span>
          </div>
          <ButtonLink href="/bundles" size="sm">
            Open bundles
          </ButtonLink>
        </CardRow>
      )}
      {report.missed.map((person) => (
        <CardRow key={person.user_id}>
          <UserAvatar id={person.user_id} size="list" />
          <Link
            href={`/users/${person.user_id}`}
            className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold hover:underline"
          >
            <UserName id={person.user_id} />
          </Link>
          <div className="min-w-0 flex-1 truncate text-[14px] text-muted">
            Holds no bundle
          </div>
        </CardRow>
      ))}
    </Card>
  );
}

function PendingChanges({ count, reachable }: { count: number; reachable: boolean }) {
  const drain = useDrainPropagations();
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  return (
    <Card>
      <CardHeader title="Pending changes" count={count} />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          {count} {count === 1 ? "change" : "changes"} waiting to be sent to Zitadel
          {/* Not "nothing has changed" — that flatly denies the count above it.
              The reassurance is about Zitadel, not the queue: it has not
              received any of this yet, so there is nothing to undo. */}
          <span className="text-faint"> — none of it has reached Zitadel yet</span>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={!reachable}
          isPending={drain.isPending}
          onClick={async () => {
            setOutcome(null);
            try {
              setOutcome(outcomeFromDrain(await drain.mutateAsync()));
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Send {count === 1 ? "it" : "them"} now
        </Button>
      </CardRow>

      {/* The result sits in the block that ran it, at row weight rather than
          as a bordered box inside a bordered card. Today is a landing an
          operator scans, so an outcome here has to be legible at a glance and
          must not restyle the block around it. */}
      {outcome && (
        <CardRow>
          <ActionOutcome outcome={outcome} placement="inline" />
        </CardRow>
      )}
      {!reachable && (
        // A disabled action states its reason in the row, not on hover: hover
        // doesn't exist on touch and doesn't survive a screenshot.
        <div className="border-t border-dashed border-warn-line bg-warn-soft px-[18px] py-3 text-[13.5px] text-warn-text">
          Disabled — Zitadel is not answering. Changes wait here, in order, and nothing is lost.
        </div>
      )}
    </Card>
  );
}

/**
 * Writes Zitadel accepted that no read has since observed, old enough that
 * the gap is a finding rather than the read projection catching up
 * (`one-truth-many-checks`: accepted is not confirmed).
 *
 * Neither of the two states this page already has a block for: not a failed
 * change — Zitadel said yes — and not one still waiting to be sent — Syndra
 * already sent it. A 2xx is an acknowledgement of receipt, never evidence
 * Zitadel can show the change back, and the age is the entire content of the
 * finding: the backend only lists a row here once it is old enough that the
 * gap stopped being Zitadel's own read catching up with itself.
 *
 * The next move is the person's own page rather than a resend — there is
 * nothing here to resend, it already went through — so the row points at
 * where an operator can look at what that person currently holds.
 */
function UnconfirmedWrites({ summary }: { summary: UnconfirmedWriteSummary }) {
  const count = summary.count;
  const rows = summary.top ?? [];
  return (
    <Card>
      <CardHeader title="Sent, and not seen back yet" count={count} tone="warn" />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          Syndra sent {count === 1 ? "this change" : "these changes"} and Zitadel accepted{" "}
          {count === 1 ? "it" : "them"}
          <span className="text-faint">
            {" "}
            — not a failed change, and not one still waiting to be sent. A later look at Zitadel
            just hasn&rsquo;t been able to see {count === 1 ? "it" : "them"} yet.
          </span>
        </div>
      </CardRow>
      <div className="contents arrive-list">
        {rows.map((row, i) => (
          <CardRow key={row.id} first={i === 0}>
            <UserAvatar id={row.user_id} size="list" />
            <Link
              href={`/users/${row.user_id}`}
              className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold hover:underline"
            >
              <UserName id={row.user_id} />
            </Link>
            <div className="w-full sm:w-[250px] shrink-0 truncate text-[14.5px] text-ink/80">
              {row.role_keys && row.role_keys.length > 0 ? (
                row.role_keys.map((key) => (
                  <RoleRef key={key} projectId={row.project_id} roleKey={key} className="mr-2" />
                ))
              ) : (
                <span className="text-faint">—</span>
              )}
            </div>
            <div className="min-w-0 flex-1 truncate text-[14px] text-muted">
              {writeNoun(row.op_type)}, accepted <Relative iso={row.applied_at} />
            </div>
            <ButtonLink href={`/users/${row.user_id}`} size="sm">
              Look at their access
            </ButtonLink>
          </CardRow>
        ))}
      </div>
    </Card>
  );
}

/** The write, in the word a reader uses for it — never the raw op_type. */
function writeNoun(opType: string): string {
  if (opType === "revoke") return "A withdrawal";
  if (opType === "replace") return "A change";
  return "A grant";
}

/**
 * Targets Syndra has not been able to read for itself.
 *
 * Danger rather than warn, and above the drift count rather than beside it. The
 * failure mode this exists for is not that something is broken — it is that
 * nothing looks broken: a sweep that cannot reach a target finds no drift on
 * it, so a week of blindness and a week of good behaviour render identically
 * everywhere else in the product.
 *
 * The age is the number the operator acts on ("since when", not "for how many
 * ticks"), and the reason travels with it because "unreachable" and "answered
 * and refused the read" send them to different machines.
 */
/**
 * Why the last pass concluded nothing, in words.
 *
 * The reason is a closed vocabulary the backend writes to a row
 * (`db/target_reconciliation.go`), and this card printed the value itself —
 * `target_unreachable`, in the smallest grey type on the dashboard, under a
 * sentence written for somebody who has never read the schema. The whole point
 * of the entry is that the operator's next move differs by reason: there is
 * nothing to reach for, or there is something to fix.
 *
 * An unrecognised value falls through unchanged rather than being swallowed. A
 * reason nobody has worded yet is still more use than no reason at all, and it
 * shows up here rather than disappearing quietly.
 */
function whyNotChecked(reason: string): string {
  switch (reason) {
    case "target_unreachable":
      return "It did not answer when Syndra last tried.";
    case "read_refused":
      return "It answered and would not serve the read — something to fix rather than wait out.";
    case "read_stale":
      return "It served an older copy of itself, which cannot be compared against what Syndra expects.";
    case "read_truncated":
      return "The read hit its safety limit, so what is missing cannot be told from what is absent.";
    case "findings_unrecorded":
      return "The pass ran and its findings could not be written down.";
    default:
      return reason;
  }
}

function UnvouchedTargets({ targets }: { targets: UnreconciledTarget[] }) {
  return (
    <Card>
      <CardHeader title="Systems Syndra could not check" count={targets.length} tone="danger" />
      <div className="contents arrive-list">
        {targets.map((t, i) => (
          <CardRow key={t.target} first={i === 0} className="flex-wrap">
            <div className="flex-1 text-[14.5px]">
              <strong className="font-semibold">{targetLabel(t.target)}</strong> hasn&rsquo;t been
              read since <Relative iso={t.since} />.
              <span className="text-faint">
                {" "}
                Nothing found on it means nothing was looked at — not that it is clean.
              </span>
              {t.reason && (
                <div className="mt-1 text-[13px] text-faint">{whyNotChecked(t.reason)}</div>
              )}
          </div>
          <ButtonLink href={`/system/targets/${t.target}`} size="sm">
            Open {targetLabel(t.target)}
          </ButtonLink>
        </CardRow>
      ))}
      </div>
    </Card>
  );
}

/**
 * Differences the reconciliation found and is not entitled to resolve.
 *
 * This count was already inside the headline's arithmetic and had no block of
 * its own, so an operator on Advanced could read "6 things need you", scan the
 * page, and find five. A number that names work with no way to reach it is
 * worse than a number that is missing: it sends somebody looking for a screen
 * that is not there, and then leaves them assuming they misread it.
 *
 * Findings live per target rather than in one queue, so this links to each
 * registered target rather than inventing an index that does not exist. In
 * this deployment that is one row.
 */
function MergeFindingsWaiting({ count }: { count: number }) {
  const targets = useTargets();
  const rows = targets.data ?? [];

  return (
    <Card>
      <CardHeader title="Waiting on a decision" count={count} tone="warn" />
      <CardRow className="flex-wrap">
        <div className="flex-1 text-[14.5px]">
          {count} {count === 1 ? "difference" : "differences"} the reconciliation found and
          can&rsquo;t resolve on its own
          <span className="text-faint">
            {" "}
            — each one is somebody&rsquo;s access sitting disputed until a person decides.
          </span>
        </div>
        {rows.map((row) => (
          <ButtonLink key={row.target} href={`/system/targets/${row.target}`} size="sm">
            {`Open ${targetLabel(row.target)}`}
          </ButtonLink>
        ))}
      </CardRow>
    </Card>
  );
}

function UnexplainedAccess({ count }: { count: number }) {
  return (
    <Card>
      <CardHeader title="Drift" count={count} tone="danger" />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          {count} {count === 1 ? "grant" : "grants"} Syndra can&rsquo;t explain
        </div>
        <ButtonLink href="/governance/drift" size="sm">
          Review them
        </ButtonLink>
      </CardRow>
    </Card>
  );
}

function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Morning";
  if (hour < 17) return "Afternoon";
  return "Evening";
}

/**
 * The greeting addresses a person, so it must never address an id. When the
 * session has no name at all the greeting simply drops the address rather than
 * greeting a Zitadel subject — "Morning." is fine; "Morning, 318...447." is not.
 */
function firstName(session: SessionUser): string {
  const source = session.name.trim() || (session.email.split("@")[0] ?? "");
  return source.trim().split(/\s+/)[0] ?? "";
}

// "here" is load-bearing: the collapsed nav bar below counts PLACES elsewhere
// that want a look, and a things-count beside a places-count with no word
// telling them apart reads as one total that disagrees with itself.
function workSentence(count: number, loading: boolean): string {
  if (loading) return "Checking.";
  if (count === 0) return "Nothing here needs you.";
  if (count === 1) return "One thing here needs you.";
  return `${spell(count)} things here need you.`;
}

const WORDS = ["Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"];

function spell(count: number): string {
  return WORDS[count] ?? String(count);
}
