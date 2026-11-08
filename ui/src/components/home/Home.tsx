"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ApproveButton, HoldingNote } from "@/components/requests/Holding";

import { AdvancedLink } from "@/components/ui/AdvancedLink";

import { AccessSource } from "@/components/access/AccessSource";
import { ClaimsLine } from "@/components/home/ClaimsLine";
import { Makerspace } from "@/components/home/Makerspace";
import { BulkDialog } from "@/components/people/BulkDialog";
import { Owner } from "@/components/system/ServiceAccountOwner";
import { EmptyState, ErrorState, RowSkeleton } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { BELOW_HEADER, usePageHead } from "@/components/ui/PageHeader";
import { Term } from "@/components/ui/Term";
import { RoleRef, UserAvatar, UserName } from "@/components/names";
import {
  useGovernanceSummary,
  type UnreconciledTarget,
  type UnconfirmedWriteSummary,
} from "@/lib/queries/useGovernance";
import { useRequestsAdmin, type AccessRequest } from "@/lib/queries/useRequests";
import { useRequestDecisions } from "@/lib/request-decisions";
import { isRefused, refusalCause } from "@/lib/wait-reason";
import { useDrainPropagations, useGaveUpPropagations, useMarkGaveUpSeen } from "@/lib/queries/usePropagation";
import { useAutoRetry, useIndicatorsRead } from "@/lib/queries/useIndicators";
import { useMissedOnboarding, type MissedOnboardingReport } from "@/lib/queries/useOperations";
import { useTargets } from "@/lib/queries/useTargets";
import { serviceAccountHref, useServiceAccounts } from "@/lib/queries/useServiceAccounts";
import { ActionForecast, ActionOutcome } from "@/components/ui/ActionOutcome";
import { outcomeFromDrain } from "@/lib/drain-outcome";
import { NOTHING_TO_DO, outcomeFromError, type ActionOutcome as Outcome, type AutoRetry } from "@/lib/outcome";
import { useExtend } from "@/lib/useExtend";
import type { SessionUser } from "@/lib/session";
import type { ServiceAccount } from "@/lib/types";
import { useIsAdvanced } from "@/lib/ui-view";
import { targetLabel } from "@/lib/nav";
import { ArriveList } from "@/lib/motion";
import { ARROW_LINK, ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot } from "@/components/ui/Badge";
import { NO_BUNDLE, peopleHref } from "@/lib/people-filters";
import { useIsPhone } from "@/lib/useViewport";
import { stillMotion, TINT_MS } from "@/lib/motion";
import { Unfold } from "@/lib/motion";
import { Crossfade } from "@/lib/motion";
import { LeavingRow, useLeavingRows } from "@/lib/motion";
import { ClockTime, Relative } from "@/components/ui/Time";
import { expiryLabel, formatShortDate, formatWeekday, whyNotChecked } from "@/lib/format";
import { ShellNotices } from "@/components/shell/ShellNotices";
import { firstName, greeting, isQuietHour, lastHere, WELCOME } from "@/lib/greeting";
import { useTerms } from "@/lib/queries/useTerms";
import { termDayLine } from "@/lib/term-dates";
import { compareNames, sortByName, sortNames } from "@/lib/sort";
import { useShownNames } from "@/components/names/shownNames";

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
  // Home is the most-visited first paint (S11): the greeting tints in, then the blocks arrive.
  const head = usePageHead();
  const advanced = useIsAdvanced();
  const summary = useGovernanceSummary();
  const requests = useRequestsAdmin("pending");
  // Every request, only to say whether any came in since they were last here (A-20 P18).
  const everyRequest = useRequestsAdmin("all");
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
  // Changes a connected system never answered, which a person must settle on
  // that system's page (A-09 §3). Per system; none is absent. The placeholder
  // zeros are not a reading, so they count as nothing.
  const unconfirmedBy = (indicators.isPlaceholderData ? undefined : indicators.data?.unconfirmed_operations) ?? {};
  const unconfirmed = Object.values(unconfirmedBy).reduce((sum, n) => sum + n, 0);
  // Changes Zitadel didn't take that Syndra sends again on its own, and the
  // ones it gave up on this week: said here so an automatic retry, and its
  // end, never go unnoticed (owner, 5 Nov 2026).
  const retrying = (indicators.isPlaceholderData ? 0 : indicators.data?.retrying) ?? 0;
  const gaveUp = (indicators.isPlaceholderData ? 0 : indicators.data?.gave_up) ?? 0;
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
  // Service accounts nobody answers for (A-07 §31). Until the list answers there
  // is nothing to look at, but a read that FAILED is a failed read: the headline
  // may not say "nothing needs you" over a list nobody could see.
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
  // Pending changes that wait for a send: the ones Syndra is retrying on its
  // own are not, and are said by the Retries block in both views.
  // With the background drain off, nothing is sent again on its own: those wait for Send too.
  const toSend = Math.max(0, (propagation?.count ?? 0) - (indicators.data?.auto_retry === false ? 0 : retrying));
  const blocks = [
    pending.length > 0,
    expiring.length > 0,
    onboardingGaps > 0,
    unowned.length > 0,
    retrying + gaveUp > 0,
    ...(advanced
      ? [
          toSend > 0,
          unconfirmedWrites.count > 0,
          (drift?.count ?? 0) > 0,
          unvouched.length > 0,
          findings > 0,
          unconfirmed > 0,
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
    advanced && (propagation?.count ?? 0) === 0 && gaveUp === 0 && ["Pending changes", "/governance/pending"],
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
  const terms = useTerms();
  const termDay = now && terms.data ? termDayLine(terms.data, now) : null;
  const loading = summary.isLoading || requests.isLoading || onboarding.isLoading;
  const error = summary.error ?? requests.error ?? onboarding.error ?? accounts.error;
  // While the load is failing the count is unknown, and "nothing needs you" is
  // exactly the wrong thing to say to somebody whose queue might be full.
  const calm = blocks === 0 && queue.rows.length === 0 && gaps.rows.length === 0;
  const swap = useCalmHandover(calm, {
    ready: !loading && !error,
    advanced,
    rows: queue.rows.length + gaps.rows.length,
  });
  // A swap held for the pointer (S5) keeps the grid as it last was, and the
  // headline with it: what leaves is what was there.
  const held = useRef<{ grid: React.ReactNode; blocks: number }>({ grid: null, blocks: 0 });
  // "Since Tuesday" (A-20 P18): what Syndra has a record of arriving after the
  // previous sign-in, never access. A block is new when a request or a joiner
  // in it arrived since; the sentence only ever lengthens in place.
  const here = now ? lastHere(session.previousSignIn, now) : null;
  const prevAt = session.previousSignIn ? new Date(session.previousSignIn).getTime() : NaN;
  const after = (iso?: string | null) => !!iso && new Date(iso).getTime() > prevAt;
  const newBlocks = Math.min(
    blocks,
    (pending.some((r) => after(r.created_at)) ? 1 : 0) + (missed.some((p) => after(p.joined)) ? 1 : 0),
  );
  const nothingCameIn =
    here !== null && !everyRequest.isLoading && !everyRequest.error && !(everyRequest.data ?? []).some((r) => after(r.created_at));
  const headline = error
    ? "Couldn't check."
    : workSentence(calm && !swap.showsCalm ? held.current.blocks : blocks, loading, here?.since, newBlocks);
  // The first visit's welcome takes the headline's seat on a calm day only
  // (A-20 P2): on a working day the work is the welcome. It is a sentence in
  // the seat that is already there, so the date line never moves.
  const welcome = session.firstVisit && !error && !loading && calm && swap.showsCalm;

  // One wrapper, so the Basic footnote's Unfold is not a direct child of the
  // arriving list (its settled rule would stop the unfold playing). It is also
  // what the pointer and focus are inside of, for a swap that waits (S5).
  const grid = (
    <div className="flex flex-col gap-6" {...swap.holdProps}>
      <div className="grid items-start gap-6 desktop:grid-cols-2">
        {/* Time-sensitive: access ends, stays wrong, or goes unseen if you wait.
            Drift leads, and a target Syndra cannot read stays beside the
            findings it qualifies, in the same column. */}
        {/* A block or a column that empties while you are here folds (5b): it fades, then its
            gap closes. It never just vanishes under the pointer. */}
        <Unfold
          open={
            expiring.length > 0 ||
            retrying + gaveUp > 0 ||
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
            {/* Both views (owner, 5 Nov 2026): a retry, and the end of one,
                are never only in Advanced. */}
            <Block open={retrying + gaveUp > 0}>
              <Retries
                retrying={retrying}
                gaveUp={gaveUp}
                retryingBy={readIndicators?.retrying_by}
                gaveUpBy={readIndicators?.gave_up_by}
                auto={readIndicators?.auto_retry ?? null}
              />
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
            (advanced && (findings > 0 || unconfirmed > 0 || toSend > 0))
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
            <Block open={advanced && unconfirmed > 0}>
              <NotConfirmed count={unconfirmed} bySystem={unconfirmedBy} />
            </Block>
            <Block open={advanced && toSend > 0}>
              <PendingChanges count={toSend} reachable={propagation?.zitadel_reachable ?? true} />
            </Block>
          </QueueColumn>
        </Unfold>
      </div>

      {/* Basic's own sentence folds when Advanced opens (A-08 §3), by the same
          Unfold as everything else the switch moves. */}
      <Unfold open={!advanced} gap={24}>
        {/* Two ways there, not a sentence about them (A-20 P10): each switches the view on the way. */}
        <p className="flex flex-wrap gap-x-6 gap-y-1">
          <ArrowLink href="/governance/pending">Pending changes</ArrowLink>
          <ArrowLink href="/governance/drift">Drift</ArrowLink>
        </p>
      </Unfold>
    </div>
  );
  if (!calm) held.current = { grid, blocks };

  return (
    <div className="flex flex-col gap-4">
      <div className={head.className} data-settled={head["data-settled"]} style={BELOW_HEADER}>
        {/* max-w in characters, not pixels: the greeting is a sentence, and a
            sentence that runs the full width of a wide window is read in
            sweeps rather than at a glance. It wraps rather than clipping, and
            `text-balance` keeps the two lines close in length instead of
            leaving one word stranded. */}
        <h1 className="type-greeting max-w-[22ch] text-balance tablet:max-w-[34ch]">
          {now ? greeting(now) : <span className="invisible">Afternoon</span>}
          {who ? `, ${who}` : ""}.{" "}
          <span className="text-faint">
            <Crossfade id={welcome ? WELCOME : headline} inline>
              {welcome ? WELCOME : headline}
            </Crossfade>
          </span>
        </h1>
        <div className="mt-2 text-[14.5px] text-faint">
          {now ? formatWeekday(now) : <span className="invisible">Wednesday 20 Sept</span>}
          {termDay && ` · ${termDay}`}
          {summary.data && (
            <>
              {" "}
              · last checked <ClockTime at={summary.dataUpdatedAt} />
            </>
          )}
        </div>
        {/* Claims delivery's one line: fixed here, ahead of anything that folds,
            so it holds its seat whether it says all is well or not (§4g). */}
        <ClaimsLine />
        {/* #27: a clear block gives up its seat, so this says it was checked. Only
            where there is a queue to read: the one calm line below says the rest. */}
        {/* It goes and comes with the grid, by the same Unfold, in step with the swap (A-12 §1).
            Mounted with the first read, so it is simply there when the page arrives. */}
        {!error && !loading && (
          <Unfold open={swap.gridOpen && clear.length > 0}>
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 pt-1 text-[13.5px] text-muted">
            <StatusDot tone="healthy" />
            <span>Checked, nothing waiting:</span>
            {clear.map(([name, href], index) => (
              <span key={href} className="inline-flex items-center gap-2">
                {index > 0 && <span aria-hidden>·</span>}
                <AdvancedLink href={href} className="inline-flex min-h-[var(--tap)] items-center text-ink link-hover desktop:min-h-0">
                  {name}
                </AdvancedLink>
              </span>
            ))}
          </div>
          </Unfold>
        )}
        {/* The shell's Notices, under the whole head (A-20 P3, G7). */}
        <ShellNotices />
      </div>

      {error ? (
        <ErrorState
          title="Couldn't load today's work."
          error={error}
          onRetry={() => {
            summary.refetch();
            requests.refetch();
            onboarding.refetch();
            accounts.refetch();
          }}
        />
      ) : (
        // One hand-over (S7): a skeleton only if the read is slow, then the blocks arrive.
        <ReadGate
          loading={loading}
          skeleton={
            <Card>
              <RowSkeleton rows={4} label="Loading today's work" />
            </Card>
          }
        >
        {/* The calm line and the grid hand over by Unfold, whatever flipped `calm`
            (A-12 §1, `useCalmHandover`): never in one frame, never a crossfade. */}
        {/* The last decided row is still on its beat: the empty state waits for it.
            One calm line, not a full-page card: the page continues below, and a
            hero-sized empty state would read as the end of it. */}
        <Unfold open={swap.calmOpen} gap={16}>
        <div className="panel">
          <EmptyState
            resolved
            title="Nothing here needs you."
            guidance={
              // Since they were last here (A-20 P18), in the guidance's own seat: no row is added.
              (nothingCameIn && here ? `Since ${here.since}, nothing came in. ` : "") +
              "No open requests, and no access ending in the next 14 days." +
              // Late at night, on a calm day only: permission to stop (A-18 idea 9).
              (now && isQuietHour(now) ? " It will keep until morning." : "")
            }
          />
        </div>
        </Unfold>
        <Unfold open={swap.gridOpen} gap={16}>
          {calm ? held.current.grid : grid}
        </Unfold>

        {/* The work zone above never moves. This is appended, never inserted. */}
        <Makerspace />
        </ReadGate>
      )}
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
/**
 * Which of the calm line and the grid is open (A-12 §1). One hands over to the
 * other: the leaving one fades and folds at once, the arriving one opens one
 * tint later, so the page makes one height change and no frame shows "Nothing
 * here needs you." beside a block that still needs you.
 *
 * What caused the swap decides only when it starts:
 *  - the view switch, or work arriving on a calm Home: at once;
 *  - the last row leaving (a decision after its beat, 5b; a row a read dropped,
 *    which has already waited for the pointer to leave its list, S5): at once;
 *  - anything else emptying Home (another operator, a background read): it
 *    waits while the pointer or focus is inside the blocks (`holdProps`), then
 *    plays once they leave (S5).
 * Under reduced motion both change at once. The first read is the page
 * arriving, not a swap: it is simply there.
 *
 * ponytail: a swap that turns back inside its tint never opened the other side,
 * so it is as if nothing happened; one that turns back later replays the
 * reopening Unfold rather than reversing it, because `Unfold` runs keyframes.
 * Reversing from where it is needs Unfold on transitions, as `leave-row` is.
 */
function useCalmHandover(calm: boolean, now: { ready: boolean; advanced: boolean; rows: number }) {
  const [target, setTarget] = useState(calm);
  const [opened, setOpened] = useState(calm);
  const [was, setWas] = useState(now);
  const [pointer, setPointer] = useState(false);
  const [focus, setFocus] = useState(false);

  if (was.ready !== now.ready || was.advanced !== now.advanced || was.rows !== now.rows) setWas(now);
  if (!now.ready || !was.ready) {
    if (target !== calm) setTarget(calm);
    if (opened !== calm) setOpened(calm);
  } else if (target !== calm) {
    const atOnce = !calm || was.advanced !== now.advanced || was.rows > 0 || !(pointer || focus);
    if (atOnce) {
      setTarget(calm);
      // The grid goes, and nothing is inside it any more.
      if (calm) {
        setPointer(false);
        setFocus(false);
      }
    }
  }
  if (opened !== target && stillMotion()) setOpened(target);

  // The arriving side waits one tint, for the leaving side's fade.
  useEffect(() => {
    if (opened === target) return;
    const timer = setTimeout(() => setOpened(target), TINT_MS);
    return () => clearTimeout(timer);
  }, [opened, target]);

  return {
    calmOpen: target && opened,
    gridOpen: !target && !opened,
    /** The side the page is going to, once any hold is over. */
    showsCalm: target,
    holdProps: {
      onPointerEnter: () => setPointer(true),
      // A focused button that is removed never says it blurred, so focus is read again here.
      onPointerLeave: (event: React.PointerEvent<HTMLElement>) => {
        setPointer(false);
        setFocus(event.currentTarget.contains(document.activeElement));
      },
      onFocus: () => setFocus(true),
      onBlur: (event: React.FocusEvent<HTMLElement>) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocus(false);
      },
    },
  };
}

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
  decisions: { act, resolved, outcomes, forecasts, cancelStep },
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
      <CardHeader title="Open requests" count={open} />
      {/* `arrive`, the same stagger every list in the product gets from
          `ListStates`. These two lists compose their own rows and so had none:
          the dashboard was the one screen whose rows appeared all at once. */}
      <Capped items={newestFirst} href="/requests">
        {(entry) => (
          <LeavingRow key={entry.id} {...queue.props(entry.id)}>
            <CardRow>
              <UserAvatar id={entry.requester_id} size="list" />
              <UserName className="w-full tablet:w-[170px] shrink-0 truncate text-[15px] font-semibold" id={entry.requester_id} />
            <div className="w-full tablet:w-[250px] shrink-0 text-[14.5px] [text-wrap:pretty]">
              <RoleRef projectId={entry.project_id} roleKey={entry.role_key} />
            </div>
            <div className="min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty]">
              {entry.justification ? `“${entry.justification}”` : "No reason given"}
            </div>
            <div className="w-[66px] shrink-0 text-[13px] text-faint">
              <Relative iso={entry.created_at} />
            </div>
            <div className="flex w-full flex-col gap-2 tablet:w-[196px] tablet:shrink-0">
              <Crossfade id={resolved.has(entry.id) ? (outcomes[entry.id] ? "outcome" : "forecast") : "open"}>
              {resolved.has(entry.id) ? (
                !outcomes[entry.id] ? (
                  forecasts[entry.id] && <ActionForecast word={forecasts[entry.id]} />
                ) : (
                  <ActionOutcome
                    placement="seat"
                    word={outcomes[entry.id].word ?? "Decided"}
                    outcome={outcomes[entry.id].outcome}
                  />
                )
              ) : (
                <>
                  <div className="flex gap-2">
                    <ApproveButton request={entry} onClick={() => act(entry, "approved")} />
                    <Button size="sm" onClick={() => act(entry, "rejected")}>
                      Decline
                    </Button>
                  </div>
                  {/* A failed write puts the buttons back and says why under them. */}
                  {outcomes[entry.id] && (
                    <ActionOutcome placement="seat" className="tint-in" word="Not done" outcome={outcomes[entry.id].outcome} />
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
        note="Extending is the only action. Doing nothing lets it lapse."
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
  // Extending moves the grant's end in Syndra, through the planner; nothing is
  // sent to Zitadel. A row has no sheet of its own: if extending would cancel a
  // removal still waiting for you to send, a small sheet holds the step (A-04 §14).
  const extend = useExtend(grant.user_id, grant.project_id, grant.role_key, "Extended from Home", "The row leaves this block on the next read.");
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // Extending again would add another 90 days to the same grant.
  const [extended, setExtended] = useState(false);

  return (
    <CardRow>
      <UserAvatar id={grant.user_id} size="list" />
      <UserName className="w-full tablet:w-[170px] shrink-0 truncate text-[15px] font-semibold" id={grant.user_id} />
      <div className="w-full tablet:w-[250px] shrink-0 text-[14.5px] [text-wrap:pretty]">
        <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="text-[14px] text-muted [text-wrap:pretty]">
          Ends {formatShortDate(grant.expires_at)}
        </span>
      </div>
      <div className="w-[66px] shrink-0 text-[13.5px] font-semibold text-warn-text">
        {expiryLabel(grant.expires_at, "short") ?? "—"}
      </div>
      <Button
        size="sm"
        isPending={extend.busy}
        disabled={extended}
        onClick={async () => {
          const made = await extend.press();
          if (!made) return;
          if (made.kind === "applied") setExtended(true);
          setOutcome(made);
        }}
      >
        Extend
      </Button>

      {outcome && (
        <ActionOutcome
          outcome={outcome}
          placement="inline"
          className="w-full"
          word={outcomeWord(outcome, "Extended", "Not extended")}
        />
      )}
      {extend.step.element}
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
            <span className="text-faint">. Anybody who joins gets nothing until one is set.</span>
          </div>
          <ArrowLink href="/bundles">Open bundles</ArrowLink>
        </CardRow>
      )}
      {/* Newest joined first; the line opens People where the same people are
          sorted by Joined (A-07 §33). */}
      <Capped items={gaps.rows} href={peopleHref({ bundle: NO_BUNDLE })}>
        {(person) => (
          <LeavingRow key={person.user_id} {...gaps.props(person.user_id)}>
            <CardRow>
              <UserAvatar id={person.user_id} size="list" />
              <UserName className="w-full tablet:w-[170px] shrink-0 truncate text-[15px] font-semibold" id={person.user_id} />
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
      <CardHeader title="Service accounts to look at" count={accounts.length} tone="warn" />
      <Capped items={accounts} href="/system/service-accounts">
        {(account) => (
          <CardRow key={account.id}>
            <AdvancedLink
              href={serviceAccountHref(account.id)}
              className="min-w-0 flex-1 text-[15px] font-semibold [overflow-wrap:anywhere] link-hover"
            >
              {account.name}
            </AdvancedLink>
            <div className="w-full tablet:w-[170px] shrink-0 text-[14px]">
              <Owner id={account.owner_id} />
            </div>
          </CardRow>
        )}
      </Capped>
    </Card>
  );
}

/**
 * Changes Syndra sends again on its own, and the ones it gave up on that
 * nobody has marked seen (owner, 5 Nov 2026: "the user should always be in the
 * know"). In both views, beside the rail's retry pill, each row ending on its
 * next step or on there being none.
 */
/**
 * The systems a count is for, by name and alphabetical ("TrueNAS and
 * Zitadel"): a TrueNAS change is never called Zitadel's (re-audit R11). An
 * older server sends no split; its retries were all counted as Zitadel's.
 */
export function systemsOf(by: Record<string, number> | undefined): string {
  const names = sortNames(Object.keys(by ?? {}).filter((t) => (by?.[t] ?? 0) > 0).map(targetLabel));
  if (names.length === 0) return "Zitadel";
  return names.length === 1 ? names[0] : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

export function Retries({
  retrying,
  gaveUp,
  retryingBy,
  gaveUpBy,
  auto = true,
}: {
  retrying: number;
  gaveUp: number;
  retryingBy?: Record<string, number>;
  gaveUpBy?: Record<string, number>;
  /** False when this server sends nothing again on its own (REVOCATION_DRAIN_ENABLED=false); null while unknown (N3). */
  auto?: AutoRetry;
}) {
  const seen = useMarkGaveUpSeen();
  const stopped = useGaveUpPropagations();
  // What each refusal was, once per cause with how many: the same words as
  // Pending changes' stopped list (lib/wait-reason.ts).
  const causes = new Map<string, number>();
  for (const r of stopped.data ?? []) {
    if (isRefused(r.wait_reason)) causes.set(r.wait_reason ?? "", (causes.get(r.wait_reason ?? "") ?? 0) + 1);
  }
  const [failure, setFailure] = useState<Outcome | null>(null);
  const one = (n: number) => (n === 1 ? "one change" : `${n} changes`);
  return (
    <Card>
      <CardHeader title={auto ? "Sent again on its own" : "Not taken when sent"} count={retrying + gaveUp} />
      {retrying > 0 && (
        <CardRow>
          <span className="inline-flex flex-1 items-center gap-2 text-[14px]">
            <StatusDot tone="warn" />
            <span className="text-warn-text">
              {systemsOf(retryingBy)} didn&apos;t take {one(retrying)}.{" "}
              {auto === null
                ? `Each change on Pending changes says whether Syndra sends it again on its own or it waits for you.`
                : auto
                ? `Syndra sends ${retrying === 1 ? "it" : "them"} again on its own and says here if it stops. ${NOTHING_TO_DO}`
                : `This Syndra server doesn't send changes again on its own, so ${retrying === 1 ? "it waits" : "they wait"} for you. Press Send on Pending changes to try again.`}
            </span>
          </span>
          <AdvancedLink href="/governance/pending" className={ARROW_LINK}>
            {auto ? "See when" : "Pending changes"} <span aria-hidden>→</span>
          </AdvancedLink>
        </CardRow>
      )}
      {gaveUp > 0 && (
        <CardRow>
          <span className="inline-flex flex-1 items-center gap-2 text-[14px]">
            <StatusDot tone="danger" />
            <span className="text-danger-text">
              Syndra stopped sending {one(gaveUp)}, so {gaveUp === 1 ? "it isn't" : "they aren't"} in{" "}
              {systemsOf(gaveUpBy)}. Pending changes lists {gaveUp === 1 ? "it" : "them"} and what to do: give a role
              again from the person&apos;s page; a revocation shows in Drift after Syndra&apos;s next check, to revoke
              there. Then mark this seen. It marks it for you only; each person marks their own.
              {[...causes].map(([reason, n]) => (
                <span key={reason} className="mt-1 block">
                  {n === 1 ? "One was" : `${n} were`} refused: {refusalCause(reason)}.
                </span>
              ))}
            </span>
          </span>
          <AdvancedLink href="/governance/pending?gave_up=1" className={ARROW_LINK}>
            See {gaveUp === 1 ? "it" : "them"} <span aria-hidden>→</span>
          </AdvancedLink>
          <Button
            variant="outline"
            size="sm"
            isPending={seen.isPending}
            onClick={async () => {
              setFailure(null);
              try {
                await seen.mutateAsync();
              } catch (error) {
                setFailure(outcomeFromError(error));
              }
            }}
          >
            Mark seen
          </Button>
        </CardRow>
      )}
      {failure && (
        <CardRow>
          <ActionOutcome outcome={failure} placement="inline" word="Not marked" />
        </CardRow>
      )}
    </Card>
  );
}

function PendingChanges({ count, reachable }: { count: number; reachable: boolean }) {
  const drain = useDrainPropagations();
  const auto = useAutoRetry();
  const [outcome, setOutcome] = useState<(Outcome & { word?: string }) | null>(null);

  return (
    <Card>
      <CardHeader title="Pending changes" count={count} />
      {count > 0 && (
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          {count} {count === 1 ? "change is" : "changes are"} waiting for you to send to Zitadel
          {/* Not "nothing has changed" — that flatly denies the count above it.
              The reassurance is about Zitadel, not the queue: it has not
              received any of this yet, so there is nothing to undo. */}
          <span className="text-faint">. None of it has reached Zitadel yet.</span>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={!reachable}
          isPending={drain.isPending}
          onClick={async () => {
            setOutcome(null);
            try {
              setOutcome(outcomeFromDrain(await drain.mutateAsync(), auto, count));
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Send {count === 1 ? "it" : "them"} now
        </Button>
      </CardRow>
      )}

      {/* The result sits in the block that ran it, at row weight rather than
          as a bordered box inside a bordered card. Today is a landing an
          operator scans, so an outcome here has to be legible at a glance and
          must not restyle the block around it. */}
      {outcome && (
        <CardRow>
          <ActionOutcome
            outcome={outcome}
            placement="inline"
            word={outcome.word ?? outcomeWord(outcome, "Sent", "Not sent")}
          />
        </CardRow>
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
  const names = useShownNames();
  const count = summary.count;
  const rows = summary.top ?? [];
  return (
    <Card>
      <CardHeader title="Sent, and not seen back yet" count={count} tone="warn" />
      <CardRow>
        <div className="flex-1 text-[14.5px]">
          Zitadel accepted {count === 1 ? "this change" : "these changes"} and hasn&rsquo;t shown{" "}
          {count === 1 ? "it" : "them"} back yet.
          <span className="text-faint">
            {" "}
            Each one clears when Zitadel shows it. If one stays, open the person to see what they hold.
          </span>
        </div>
      </CardRow>
      <Capped items={rows} href="/users?attention=unseen">
        {(row) => (
          <CardRow key={row.id}>
            <UserAvatar id={row.user_id} size="list" />
            <UserName className="w-full tablet:w-[170px] shrink-0 truncate text-[15px] font-semibold" id={row.user_id} />
            {row.role_keys && row.role_keys.length > 0 && (
              <div className="w-full tablet:w-[250px] shrink-0 text-[14.5px] [text-wrap:pretty]">
                {sortByName(row.role_keys, (key) => names.roleRef(row.project_id ?? "", key), (key) => key).map((key) => (
                  <RoleRef key={key} projectId={row.project_id} roleKey={key} className="mr-2" />
                ))}
              </div>
            )}
            <div className="min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty]">
              Sent <Relative iso={row.applied_at} /> {sentTo(row.op_type, row.role_keys?.length ?? 0)}
            </div>
          </CardRow>
        )}
      </Capped>
    </Card>
  );
}

/** The result's word: the act's own past tense, or "Not …" when it did not happen. */
function outcomeWord(outcome: Outcome, done: string, not: string): string {
  if (outcome.recorded) return "Refused by Zitadel";
  return outcome.kind === "failed" || outcome.kind === "refused" ? not : done;
}

/** What the write was sent to do, as a verb; never the raw op_type. */
function sentTo(opType: string, roles: number): string {
  const it = roles > 1 ? "them" : "it";
  if (opType === "revoke") return `to revoke ${it}`;
  if (opType === "replace") return `to change ${it}`;
  return `to give ${it}`;
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
      <Capped items={sortByName(targets, (t) => targetLabel(t.target), (t) => t.target)} href="/system/targets">
        {(t) => (
          <CardRow key={t.target} className="flex-wrap">
            <div className="flex-1 text-[14.5px]">
              <strong className="font-semibold">{targetLabel(t.target)}</strong> hasn&rsquo;t been
              read since <Relative iso={t.since} />.
              <span className="text-faint">
                {" "}
                Nothing found on it means nothing was looked at, not that it is clean.
              </span>
              {t.reason && (
                <div className="mt-1 text-[13px] text-faint">{whyNotChecked(t.reason)}</div>
              )}
            </div>
            <ArrowLink href={`/system/targets/${t.target}`}>{`Open ${targetLabel(t.target)}`}</ArrowLink>
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
          {count} {count === 1 ? "difference" : "differences"} Syndra found and can&rsquo;t
          resolve on its own
          <span className="text-faint">
            . Each one is somebody&rsquo;s access sitting disputed until a person decides.
          </span>
        </div>
        {rows.map((row) => (
          <ArrowLink key={row.target} href={`/system/targets/${row.target}`}>
            {`Open ${targetLabel(row.target)}`}
          </ArrowLink>
        ))}
      </CardRow>
    </Card>
  );
}

/**
 * Changes sent to a connected system that never answered (A-09 §3). Each may or
 * may not have happened, and they are kept with each system, under Waiting on
 * you, where one verb asks what happened. Never passwords: they settle
 * themselves (A-13 §5).
 */
function NotConfirmed({ count, bySystem }: { count: number; bySystem: Record<string, number> }) {
  const systems = Object.entries(bySystem)
    .filter(([, n]) => n > 0)
    .sort(([a], [b]) => compareNames(targetLabel(a), targetLabel(b)));
  return (
    <Card>
      <CardHeader
        title="Not confirmed"
        count={count}
        tone="warn"
        note="Changes sent to a connected system that never answered. They are kept with each system."
      />
      {systems.map(([target, n]) => (
        <CardRow key={target} className="flex-wrap">
          <span className="flex-1 text-[14.5px]">
            {targetLabel(target)} <span className="text-faint">· {n === 1 ? "1 change" : `${n} changes`}</span>
          </span>
          <ArrowLink href={`/system/targets/${target}#waiting`}>{`Open ${targetLabel(target)}`}</ArrowLink>
        </CardRow>
      ))}
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
        <ArrowLink href="/governance/drift">Review on Drift</ArrowLink>
      </CardRow>
    </Card>
  );
}


// "here" is load-bearing: the collapsed nav bar below counts PLACES elsewhere
// that want a look, and a things-count beside a places-count with no word
// telling them apart reads as one total that disagrees with itself.
function workSentence(count: number, loading: boolean, since?: string, fresh = 0): string {
  if (loading) return "Checking.";
  if (count === 0) return "Nothing needs you right now.";
  const base = count === 1 ? "One thing needs you" : `${spell(count)} things need you`;
  if (!since || fresh === 0) return `${base}.`;
  if (fresh >= count) return `${base}, ${count === 1 ? "new" : count === 2 ? "both new" : "all new"} since ${since}.`;
  return `${base}, ${spell(fresh).toLowerCase()} of them new since ${since}.`;
}

const WORDS = ["Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten"];

function spell(count: number): string {
  return WORDS[count] ?? String(count);
}
