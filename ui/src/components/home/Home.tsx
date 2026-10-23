"use client";

import { useEffect, useMemo, useState } from "react";
import { HoldingNote, alreadyHeld } from "@/components/requests/Holding";

import Link from "next/link";

import { AccessSource } from "@/components/access/AccessSource";
import { Makerspace } from "@/components/home/Makerspace";
import { BulkDialog } from "@/components/people/BulkDialog";
import { Owner } from "@/components/system/ServiceAccountOwner";
import { ErrorState, RowSkeleton } from "@/components/states";
import { Button, ButtonLink } from "@/components/ui/Button";
import { KEPT, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Card, CardHeader, CardHeaderLink, CardRow } from "@/components/ui/Card";
import { Term } from "@/components/ui/Term";
import { RoleRef, UserAvatar, UserName } from "@/components/names";
import {
  useGovernanceSummary,
  type UnreconciledTarget,
  type UnconfirmedWriteSummary,
} from "@/lib/queries/useGovernance";
import { useRequestsAdmin, type AccessRequest } from "@/lib/queries/useRequests";
import { useRequestDecisions } from "@/lib/request-decisions";
import { useDrainPropagations } from "@/lib/queries/usePropagation";
import { useIndicatorsRead } from "@/lib/queries/useIndicators";
import { useMissedOnboarding, type MissedOnboardingReport } from "@/lib/queries/useOperations";
import { useTargets } from "@/lib/queries/useTargets";
import { serviceAccountHref, useServiceAccounts } from "@/lib/queries/useServiceAccounts";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { outcomeFromError, outcomeFromExtend, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useCreateGrant } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import type { ServiceAccount } from "@/lib/types";
import { useIsAdvanced } from "@/lib/ui-view";
import { targetLabel } from "@/lib/nav";
import { ArriveList } from "@/components/ui/ArriveList";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { NO_BUNDLE, peopleHref } from "@/lib/people-filters";
import { useIsPhone } from "@/lib/useViewport";
import { Unfold } from "@/components/ui/Unfold";
import { Crossfade } from "@/components/ui/Crossfade";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { ClockTime, Relative } from "@/components/ui/Time";
import { formatShortDate, formatWeekday, daysUntil, whyNotChecked } from "@/lib/format";

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
 * It is two columns (route-home layout 1c, chosen 2 Oct 2026): "Time-sensitive"
 * holds what changes on its own if you wait, "When you're ready" what only
 * waits. Each block type has one column for good, and a column with nothing in
 * it leaves (#27) without the other moving across into its place.
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
  const indicators = useIndicatorsRead();

  const pending = useMemo(() => requests.data ?? [], [requests.data]);
  // The queue's own decide and beat. They live here, not in the card, because
  // the card goes when the read empties and would take a decided row's outcome
  // with it: a decided row is held for the beat (Motion 5b), then fades (S5).
  const decisions = useRequestDecisions();
  const queue = useLeavingRows(pending, (entry) => entry.id, !requests.isLoading && !requests.error, {
    stay: decisions.staying,
  });
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
  // Newest joined first, a person Zitadel gave no date for last (A-07 §33). The
  // block's rows leave the way the queue's do (S5): after Give a bundle the next
  // read drops the person and the row fades where it was.
  const missed = useMemo(
    () =>
      [...(onboarding.data?.missed ?? [])].sort((a, b) => (b.joined ?? "").localeCompare(a.joined ?? "")),
    [onboarding.data?.missed],
  );
  const gaps = useLeavingRows(missed, (person) => person.user_id, !onboarding.isLoading && !onboarding.error);
  // Service accounts nobody answers for (A-07 §31). Not a read that can fail the
  // page: until the list answers there is nothing to look at.
  const accounts = useServiceAccounts();
  const unowned = useMemo(() => (accounts.data ?? []).filter((a) => a.owner_id === ""), [accounts.data]);
  // ponytail: "not used in 90 days", the other half of §31, is not built: Zitadel's
  // last use of a machine key isn't read yet. Add it to this filter, and to the
  // row's reason, when it is.

  // BLOCKS, not rows (#31, "the headline counts blocks; each badge counts its own
  // noun"): the headline said "98 things need you" for eighty-five rows in one
  // block. A block with something in it is one thing; each card's own badge says
  // how many rows it holds. Counted, and that is the whole point of it. An
  // unreadable target produces no drift findings, so a week of silence lands here
  // as blocks === 0 and the page says "Nothing here needs you" — the one sentence
  // that must never be said about a system nobody has been able to look at.
  const blocks = [
    pending.length > 0,
    expiring.length > 0,
    onboardingGaps > 0,
    unowned.length > 0,
    ...(advanced
      ? [
          (propagation?.count ?? 0) > 0,
          unconfirmedWrites.count > 0,
          (drift?.count ?? 0) > 0,
          unvouched.length > 0,
          findings > 0,
        ]
      : []),
  ].filter(Boolean).length;

  // What the line under the headline may call checked and clear. A block that
  // is on the page is not clear, and one that was never read is not claimed: the
  // placeholder zeros the rail holds before its first poll are not readings.
  const readIndicators = indicators.isPlaceholderData ? undefined : indicators.data;
  const clear: Array<[string, string]> = [
    queue.rows.length === 0 && ["Requests", "/requests"],
    expiring.length === 0 && ["Expiring access", "/review/expiring-access"],
    advanced && (drift?.count ?? 0) === 0 && ["Drift", "/governance/drift"],
    advanced && (propagation?.count ?? 0) === 0 && ["Pending changes", "/governance/pending"],
    advanced &&
      readIndicators?.unconfirmed_revocations === 0 && [
        "Unfinished revocations",
        "/governance/unconfirmed-revocations",
      ],
    advanced && readIndicators?.holds_due === 0 && ["Holds due", "/review/holds"],
  ].filter((entry): entry is [string, string] => Array.isArray(entry));

  const who = firstName(session);
  // The greeting and the weekday depend on the reader's clock and zone, which
  // the server doesn't share: rendered there, they were the server's hour in
  // UTC and hydration failed (React #418). They appear on mount, in a seat held
  // by an invisible stand-in so nothing below moves (same rule as Relative).
  const [now, setNow] = useState<Date | null>(null);
  useEffect(() => setNow(new Date()), []);
  const loading = summary.isLoading || requests.isLoading || onboarding.isLoading;
  const error = summary.error ?? requests.error ?? onboarding.error;
  // While the load is failing the count is unknown, and "nothing needs you" is
  // exactly the wrong thing to say to somebody whose queue might be full.
  const headline = error ? "Couldn't check." : workSentence(blocks, loading);

  return (
    <div className="flex flex-col gap-6">
      <div>
        {/* max-w in characters, not pixels: the greeting is a sentence, and a
            sentence that runs the full width of a wide window is read in
            sweeps rather than at a glance. It wraps rather than clipping, and
            `text-balance` keeps the two lines close in length instead of
            leaving one word stranded. */}
        <h1 className="type-greeting max-w-[22ch] text-balance sm:max-w-[34ch]">
          {now ? greeting(now) : <span className="invisible">Afternoon</span>}
          {who ? `, ${who}` : ""}. <span className="text-ink/40">{headline}</span>
        </h1>
        <div className="mt-2 text-[14.5px] text-faint">
          {now ? formatWeekday(now) : <span className="invisible">Wednesday 20 Sept</span>} · last checked <ClockTime at={summary.dataUpdatedAt} />
        </div>
        {/* #27: a clear block gives up its seat, so this says it was checked. Only
            where there is a queue to read: the one calm line below says the rest. */}
        {!error && !loading && (blocks > 0 || queue.rows.length > 0 || gaps.rows.length > 0) && clear.length > 0 && (
          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[13.5px] text-muted">
            <span aria-hidden className="h-[7px] w-[7px] rounded-pill bg-healthy" />
            <span>Checked, nothing waiting:</span>
            {clear.map(([name, href], index) => (
              <span key={href} className="inline-flex items-center gap-2">
                {index > 0 && <span aria-hidden>·</span>}
                <Link href={href} className="text-ink hover:text-accent-text hover:underline">
                  {name}
                </Link>
              </span>
            ))}
          </div>
        )}
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
      ) : blocks === 0 && queue.rows.length === 0 && gaps.rows.length === 0 ? (
        // The last decided row is still on its beat: the empty state waits for it.
        // One calm line, not a full-page card: the page continues below, and a
        // hero-sized empty state would read as the end of it.
        <div className="panel flex items-center gap-3 px-5 py-4">
          {/* Healthy, not accent. Violet is what you can act on, and the whole
              statement of this row is that there is nothing to act on. */}
          <span aria-hidden className="h-2.5 w-2.5 rounded-pill bg-healthy" />
          <span className="text-[14.5px]">
            <strong className="font-semibold">Nothing here needs you.</strong>{" "}
            <span className="text-muted">
              No open requests, and no access ending in the next 14 days. Checked <ClockTime />.
            </span>
          </span>
        </div>
      ) : (
        <>
          <div className="grid items-start gap-6 desktop:grid-cols-2">
            {/* Time-sensitive: access ends, stays wrong, or goes unseen if you wait.
                Drift leads, and a target Syndra cannot read stays beside the
                findings it qualifies, in the same column. */}
            {/* A block or a column that empties while you are here folds (5b): it fades, then its
                gap closes. It never just vanishes under the pointer. */}
            <Unfold
              open={
                expiring.length > 0 ||
                (advanced &&
                  ((drift?.count ?? 0) > 0 || unvouched.length > 0 || unconfirmedWrites.count > 0))
              }
              className="min-w-0 desktop:col-start-1"
            >
              <QueueColumn
                title="Time-sensitive"
                note="Access ends, stays wrong, or goes unseen if you wait."
              >
                <Block open={advanced && (drift?.count ?? 0) > 0}>
                  <UnexplainedAccess count={drift?.count ?? 0} />
                </Block>
                <Block open={expiring.length > 0}>
                  <ExpiringSoon grants={expiring} />
                </Block>
                <Block open={advanced && unvouched.length > 0}>
                  <UnvouchedTargets targets={unvouched} />
                </Block>
                <Block open={advanced && unconfirmedWrites.count > 0}>
                  <UnconfirmedWrites summary={unconfirmedWrites} />
                </Block>
              </QueueColumn>
            </Unfold>
            {/* When you're ready: nothing changes until you act. */}
            <Unfold
              open={
                queue.rows.length > 0 ||
                onboardingGaps > 0 ||
                gaps.rows.length > 0 ||
                unowned.length > 0 ||
                (advanced && (findings > 0 || (propagation?.count ?? 0) > 0))
              }
              className="min-w-0 desktop:col-start-2"
            >
              <QueueColumn title="When you're ready" note="Nothing changes until you act.">
                <Block open={queue.rows.length > 0}>
                  <OpenRequests rows={queue.rows} decisions={decisions} queue={queue} />
                </Block>
                <Block open={onboardingGaps > 0 || gaps.rows.length > 0}>
                  <OnboardingGaps report={onboarding.data!} gaps={gaps} />
                </Block>
                <Block open={unowned.length > 0}>
                  <ServiceAccountsToLookAt accounts={unowned} />
                </Block>
                <Block open={advanced && findings > 0}>
                  <MergeFindingsWaiting count={findings} />
                </Block>
                <Block open={advanced && (propagation?.count ?? 0) > 0}>
                  <PendingChanges
                    count={propagation?.count ?? 0}
                    reachable={propagation?.zitadel_reachable ?? true}
                  />
                </Block>
              </QueueColumn>
            </Unfold>
          </div>

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

/**
 * One of the queue's two columns. Its `Unfold` pins its side (`col-start`), so a
 * column that leaves never pulls the other across into its place.
 */
function QueueColumn({
  title,
  note,
  children,
}: {
  title: string;
  note: string;
  children: React.ReactNode;
}) {
  const id = `home-col-${title.replace(/\W+/g, "-").toLowerCase()}`;
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col">
      <div className="flex flex-col gap-1">
        <h2 id={id} className="type-card-title">
          {title}
        </h2>
        <span className="text-[13.5px] text-muted">{note}</span>
      </div>
      {children}
    </section>
  );
}

/**
 * One work block in a column. The 14px above it is inside what folds, so closing
 * the block takes its gap with it and nothing hops when it is gone.
 */
function Block({ open, children }: { open: boolean; children: React.ReactNode }) {
  return (
    <Unfold open={open}>
      <div className="pt-3.5">{children}</div>
    </Unfold>
  );
}

/**
 * A block's rows, at most five (three on a phone), then one violet line to the
 * rest (A-07 §32–34). The block's height has a ceiling, so an intake of 31 or
 * 300 draws the same card; fewer rows draw a shorter one, which is the queue
 * emptying, not structure moving. Every list block on this page goes through
 * here, so the cap and the line are said once. `items` arrive in the order to
 * show: newest or soonest first.
 *
 * Decided in JS rather than hidden in CSS because the count in the line has to
 * agree with the rows above it, and it differs by viewport.
 */
const HOME_ROWS = 5;
const HOME_ROWS_PHONE = 3;

function Capped<T>({
  items,
  href,
  children,
}: {
  items: readonly T[];
  /** The full list, already narrowed to what these rows are. */
  href: string;
  children: (item: T) => React.ReactNode;
}) {
  const cap = useIsPhone() ? HOME_ROWS_PHONE : HOME_ROWS;
  const more = items.length - cap;
  return (
    <>
      <ArriveList>{items.slice(0, cap).map((item) => children(item))}</ArriveList>
      {more > 0 && (
        <CardRow>
          <ArrowLink href={href}>{`and ${more} more`}</ArrowLink>
        </CardRow>
      )}
    </>
  );
}

/** Approve / Deny resolve in place, on the row. They never navigate. */
function OpenRequests({
  rows,
  decisions: { act, resolved, outcomes, cancelStep },
  queue,
}: {
  rows: AccessRequest[];
  decisions: ReturnType<typeof useRequestDecisions>;
  queue: ReturnType<typeof useLeavingRows<AccessRequest>>;
}) {
  // A decided row keeps its seat and its outcome through the beat, so the count
  // is what is still open.
  const open = rows.filter((entry) => !resolved.has(entry.id)).length;
  const newestFirst = [...rows].sort((a, b) => b.created_at.localeCompare(a.created_at));

  return (
    <Card {...queue.listProps}>
      {cancelStep}
      <CardHeader
        title="Open requests"
        count={open}
        action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>}
      />
      {/* `arrive`, the same stagger every list in the product gets from
          `ListStates`. These two lists compose their own rows and so had none:
          the dashboard was the one screen whose rows appeared all at once. */}
      <Capped items={newestFirst} href="/requests">
        {(entry) => (
          <LeavingRow key={entry.id} {...queue.props(entry.id)}>
            <CardRow>
              <UserAvatar id={entry.requester_id} size="list" />
              <div className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold">
                <UserName id={entry.requester_id} />
            </div>
            <div className="w-full sm:w-[250px] shrink-0 text-[14.5px] text-ink/80 [text-wrap:pretty]">
              <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
            </div>
            <div className="min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty]">
              {entry.justification ? `“${entry.justification}”` : "No reason given"}
            </div>
            <div className="w-[66px] shrink-0 text-[13px] text-faint">
              <Relative iso={entry.created_at} />
            </div>
            <div className="flex w-full flex-col gap-2 sm:w-[196px] sm:shrink-0">
              <Crossfade id={resolved.has(entry.id) ? "outcome" : "open"}>
              {resolved.has(entry.id) ? (
                outcomes[entry.id] && (
                  <ActionOutcome
                    placement="seat"
                    word={outcomes[entry.id].word}
                    outcome={outcomes[entry.id].outcome}
                  />
                )
              ) : (
                <>
                  <div className="flex gap-2">
                    <Button
                      variant="accent"
                      size="sm"
                      disabled={alreadyHeld(entry)}
                      reason={alreadyHeld(entry) ? "They already have this." : undefined}
                      onClick={() => act(entry, "approved")}
                    >
                      Approve
                    </Button>
                    <Button size="sm" onClick={() => act(entry, "rejected")}>
                      Decline
                    </Button>
                  </div>
                  {/* A failed write puts the buttons back and says why under them. */}
                  {outcomes[entry.id] && (
                    <ActionOutcome placement="seat" className="tint-in" outcome={outcomes[entry.id].outcome} />
                  )}
                </>
              )}
              </Crossfade>
            </div>

            {!resolved.has(entry.id) && <HoldingNote request={entry} />}
          </CardRow>
          </LeavingRow>
        )}
      </Capped>
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
  // No end date is the furthest end there is, so it goes last.
  const soonestFirst = [...grants].sort((a, b) =>
    (a.expires_at ?? "￿").localeCompare(b.expires_at ?? "￿"),
  );
  return (
    <Card>
      <CardHeader
        title="Direct access ending soon"
        count={grants.length}
        tone="warn"
        note="Extending is the only action — doing nothing lets it lapse."
      />
      <Capped items={soonestFirst} href="/review/expiring-access">
        {(grant) => <ExpiringRow key={grant.id} grant={grant} />}
      </Capped>
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
  // A row has no sheet of its own: if extending would cancel a change still
  // waiting for you to send, a small sheet holds the step (A-04 §14).
  const step = useCancelStep({ host: "Extend this access", goAhead: "extend", standalone: true });
  const remaining = daysUntil(grant.expires_at);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // Extending again would add another 90 days to the same grant.
  const [extended, setExtended] = useState(false);

  return (
    <CardRow>
      <UserAvatar id={grant.user_id} size="list" />
      <div className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold">
        <UserName id={grant.user_id} />
      </div>
      <div className="w-full sm:w-[250px] shrink-0 text-[14.5px] text-ink/80 [text-wrap:pretty]">
        <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="text-[14px] text-muted [text-wrap:pretty]">
          No action — expires {formatShortDate(grant.expires_at)}
        </span>
      </div>
      <div className="w-[66px] shrink-0 text-[13.5px] font-semibold text-warn-text">
        {remaining === null ? "—" : `${remaining} days`}
      </div>
      <Button
        size="sm"
        isPending={extend.isPending}
        disabled={extended}
        onClick={async () => {
          try {
            const made = await step.run(() =>
              extend.mutateAsync({
                project_id: grant.project_id,
                role_key: grant.role_key,
                reason: "Extended from Home",
                duration_days: 90,
                extend: true,
              }),
            );
            if (made === KEPT) return;
            setExtended(true);
            setOutcome(outcomeFromExtend(made, 90, "The row leaves this block on the next read."));
          } catch (error) {
            setOutcome(outcomeFromError(error));
          }
        }}
      >
        Extend
      </Button>

      {outcome && <ActionOutcome outcome={outcome} placement="inline" className="w-full" />}
      {step.element}
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
function OnboardingGaps({
  report,
  gaps,
}: {
  report: MissedOnboardingReport;
  /** `report.missed`, newest joined first, with the rows that just left still in place. */
  gaps: ReturnType<typeof useLeavingRows<MissedOnboardingReport["missed"][number]>>;
}) {
  // Give a bundle is People's own bulk flow, opened for the one person on the row.
  const [giving, setGiving] = useState<string | null>(null);
  // The count counts PEOPLE. It used to add one for "no default bundle is
  // configured", so a card headed "New people without a bundle · 1" could be
  // about nobody at all — the 1 was a settings fact wearing a people badge.
  // The setting still gets said, as its own row, where it reads as the cause
  // it is rather than as a person.
  return (
    <Card {...gaps.listProps}>
      <CardHeader title="New people without a bundle" count={report.missed.length} />
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
      {/* Newest joined first; the line opens People where the same people are
          sorted by Joined (A-07 §33). */}
      <Capped items={gaps.rows} href={peopleHref({ bundle: NO_BUNDLE })}>
        {(person) => (
          <LeavingRow key={person.user_id} {...gaps.props(person.user_id)}>
            <CardRow>
              <UserAvatar id={person.user_id} size="list" />
              <Link
                href={`/users/${person.user_id}`}
                className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold hover:underline"
              >
                <UserName id={person.user_id} />
              </Link>
              <div className="min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty]">
                Holds no bundle
              </div>
              {person.joined && (
                <div className="w-[66px] shrink-0 text-[13px] text-faint">
                  <Relative iso={person.joined} />
                </div>
              )}
              <Button size="sm" onClick={() => setGiving(person.user_id)}>
                Give a bundle
              </Button>
            </CardRow>
          </LeavingRow>
        )}
      </Capped>
      {giving && (
        <BulkDialog op="assign_bundle" userIds={[giving]} scope="on Home" onClose={() => setGiving(null)} />
      )}
    </Card>
  );
}

/**
 * Service accounts nobody answers for (A-07 §31). It isn't drift: nothing in
 * Zitadel is wrong, but nobody is answering for the access. The row names why in
 * its seat, a warn dot and a word, the same `Owner` the Service accounts list
 * draws, so Home and that page cannot say it differently.
 */
function ServiceAccountsToLookAt({ accounts }: { accounts: ServiceAccount[] }) {
  return (
    <Card>
      <CardHeader
        title="Service accounts to look at"
        count={accounts.length}
        tone="warn"
        action={<CardHeaderLink href="/system/service-accounts">Open Service accounts →</CardHeaderLink>}
      />
      <Capped items={accounts} href="/system/service-accounts">
        {(account) => (
          <CardRow key={account.id}>
            <Link
              href={serviceAccountHref(account.id)}
              className="min-w-0 flex-1 text-[15px] font-semibold [overflow-wrap:anywhere] hover:text-accent-text"
            >
              {account.name}
            </Link>
            <div className="w-[170px] shrink-0 text-[14px]">
              <Owner id={account.owner_id} />
            </div>
          </CardRow>
        )}
      </Capped>
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
          {count} {count === 1 ? "change is" : "changes are"} waiting for you to send to Zitadel
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
 * change — Zitadel said yes — and not one still waiting for you to send — Syndra
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
            — not a failed change, and not one still waiting for you to send. A later look at Zitadel
            just hasn&rsquo;t been able to see {count === 1 ? "it" : "them"} yet.
          </span>
        </div>
      </CardRow>
      <ArriveList>
        {rows.map((row, i) => (
          <CardRow key={row.id} first={i === 0}>
            <UserAvatar id={row.user_id} size="list" />
            <Link
              href={`/users/${row.user_id}`}
              className="w-full sm:w-[170px] shrink-0 truncate text-[15px] font-semibold hover:underline"
            >
              <UserName id={row.user_id} />
            </Link>
            <div className="w-full sm:w-[250px] shrink-0 text-[14.5px] text-ink/80 [text-wrap:pretty]">
              {row.role_keys && row.role_keys.length > 0 ? (
                row.role_keys.map((key) => (
                  <RoleRef key={key} projectId={row.project_id} roleKey={key} className="mr-2" />
                ))
              ) : (
                <span className="text-faint">—</span>
              )}
            </div>
            <div className="min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty]">
              {writeNoun(row.op_type)}, accepted <Relative iso={row.applied_at} />
            </div>
            <ButtonLink href={`/users/${row.user_id}`} size="sm">
              Look at their access
            </ButtonLink>
          </CardRow>
        ))}
      </ArriveList>
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
function UnvouchedTargets({ targets }: { targets: UnreconciledTarget[] }) {
  return (
    <Card>
      <CardHeader title="Systems Syndra could not check" count={targets.length} tone="danger" />
      <Capped items={targets} href="/system/targets">
        {(t) => (
          <CardRow key={t.target} className="flex-wrap">
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
        )}
      </Capped>
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

function greeting(now: Date): string {
  const hour = now.getHours();
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
  if (count === 0) return "Nothing needs you right now.";
  if (count === 1) return "One thing needs you.";
  return `${spell(count)} things need you.`;
}

const WORDS = ["Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"];

function spell(count: number): string {
  return WORDS[count] ?? String(count);
}
