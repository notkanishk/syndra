"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";

import {
  cooldownNext,
  describeFailure,
  failureMessage,
  isUnconfirmed,
  nextStep,
  isNothingSent,
  notDoneWord,
  outcomeFromError,
  sentence,
  UNCONFIRMED_WORD,
  unconfirmedOutcome,
} from "@/lib/outcome";
import { checkAgainNext, cooldownLine, FIRST_PART_READ, READING, retryingLine, whyLine, type Reader } from "@/components/results/copy";
import { ApiError } from "@/lib/api-client";
import { dataOf, notRead as isNotRead, notReadOf, ofRead, readOfQuery, REASON, type Why } from "@/lib/queries/read";
import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";

import { ArriveList, Crossfade, FOLD_MS, LeavingRow, TINT_MS, UnfoldOnArrival, Unfold, useExitPhase, useFlashOnChange, useLeavingRows } from "@/lib/motion";
import { useLongWait } from "@/lib/useLongWait";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { busyNext } from "@/lib/drain-outcome";

import { ADOPT_STAYS, SERVICE_STOPPED, STORAGE_OPERATION_LABEL, UNAUDITED_LABEL, UnauditedShares, serviceName } from "@/components/kinds/storage/readings";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { UnconfirmedChanges } from "@/components/targets/UnconfirmedChanges";
import { AllClear, useEmptied } from "@/components/targets/AllClear";
import { MergeFindings } from "@/components/targets/MergeFindings";
import { OVERVIEW_LEDE, OVERVIEW_TITLE } from "@/components/usage/copy";
import { StatsRetention } from "@/components/targets/StatsRetention";
import { TargetStats } from "@/components/usage/TargetStats";
import { focusHeading, Region } from "@/components/targets/Region";
import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { useUiView } from "@/lib/ui-view";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail } from "@/components/ui/TechnicalDetail";
import { Card, CardGrid, CardHeader, CardRow, type SeatWidth } from "@/components/ui/Card";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { ReadLine, SectionPill, type NavSection } from "@/components/ui/SectionNav";
import { StandingLine } from "@/components/ui/StandingLine";
import { rowStep } from "@/lib/account-standing";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { FieldHint, FieldLabel, Input, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { blocksIrreversibleAction, ReadAgainButton, ReadAgainError, ReadFreshness, RetryWait, stateOfRead } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { AdvancedOnly } from "@/components/ui/AdvancedOnly";
import { lifecycleWhy } from "@/lib/lifecycle-reason";
import { CheckPips, targetReading } from "@/components/targets/TargetReading";
import { ProjectSet, ProjectStep, SetupStrip } from "@/components/targets/ProjectSetup";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { Term } from "@/components/ui/Term";
import { formatBytes, formatClock, formatShortDate, formatWhen, personLine, versionLabel, whyNotChecked } from "@/lib/format";
import { useCountdown, useRetryOnce } from "@/lib/queries/fresh";
import { useTargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";
import {
  useAdoptAccount,
  useMergeFindings,
  useReconcileTarget,
  useReleaseBinding,
  useSetLifecycle,
  useTargetHealth,
  useTargetHealthAgain,
  useResolveLogFinding,
  useTargetInventory,
  useTargetInventoryAgain,
  useTargets,
  useUnconfirmedOperations,
  type AdoptionResult,
  type LogAnchor,
  type TargetHealth,
  type TargetHealthReply,
  type TargetReconcilePass,
  type BoundAccount,
  type TargetSummary,
} from "@/lib/queries/useTargets";
import { useUsers } from "@/lib/queries/useUsers";
import { Avatar } from "@/components/ui/Avatar";
import { sortByName } from "@/lib/sort";

/**
 * One add-on target's operator page (9.20, 1.18/1.19, 15.6; design §21).
 *
 * Four questions in this order, because each answer changes what the next one
 * means: is it healthy, whose accounts are on it, what can it do, and what
 * state did somebody put it in.
 */
export function TargetOverview({ target }: { target: string }) {
  const nameOf = useSystemNames();
  const roster = useTargets();
  const health = useTargetHealth(target);
  // A failed read is pressed again live (`?fresh=true`), never answered from the copy that failed.
  const healthAgain = useTargetHealthAgain(target);
  const findings = useMergeFindings(target);
  const inventory = useTargetInventory(target);
  const registered = (roster.data ?? []).find((t) => t.target === target);

  const anchorFinding = health.data?.log_anchor?.violation_reason ? 1 : 0;
  // Everything in region 1: differences reconciliation refused to resolve (a
  // binding conflict among them), and a change record edited after it was
  // queued. None of them is a status; each is a piece of work waiting on a
  // human, which is why they are one region and not two places.
  // And changes the system never answered, which a person must settle (A-09 §3).
  const unconfirmed = useUnconfirmedOperations(target);
  //
  // Counted from what is still drawn, not from the read (D-1 M3): a finding that has been decided
  // leaves by fading and folding, and the count changes after its row has gone. The anchor's own
  // Notice fades (tint) and then folds (fold), so the count follows the fade and the all-clear
  // waits for the fold.
  const [drawn, setDrawn] = useState<{ n: number; byReader: boolean } | null>(null);
  const onDrawn = useCallback(
    (n: number, byReader: boolean) =>
      setDrawn((was) => (was && was.n === n && was.byReader === byReader ? was : { n, byReader })),
    [],
  );
  const findingsDrawn = drawn?.n ?? findings.data?.length ?? 0;
  const anchorCounted = useExitPhase(anchorFinding > 0, TINT_MS).rendered ? 1 : 0;
  const anchorCleared = useExitPhase(anchorFinding > 0, TINT_MS + FOLD_MS).rendered ? 1 : 0;
  const waiting = findingsDrawn + anchorCounted + (unconfirmed.data?.length ?? 0);
  const clear = findingsDrawn + anchorCleared + (unconfirmed.data?.length ?? 0) === 0;
  const waitingKnown =
    !findings.isLoading && !findings.error && !health.isLoading && !health.error &&
    !unconfirmed.isLoading && !unconfirmed.error;
  const emptied = useEmptied(waitingKnown ? (clear ? 0 : waiting) : undefined);
  const name = nameOf(target);
  // The add-on's account of itself, when it gave one (design §13.3): its product, its pause.
  const said = health.data ? dataOf(health.data.addon) : undefined;
  // The add-on's own answer was not read (design §13.3): its cards recede and
  // say why, with no copy to draw, and the controls that need it live are off
  // (A-13). Syndra's own cards (reach, lifecycle, Waiting on you, what runs
  // here) stay raised. False while the health read is pending.
  const notRead = isNotRead(health.data?.addon);
  // One Zitadel project per system (3a). Unknown until the roster answers;
  // null is "not chosen", and everything that gives access waits for it.
  const project = registered?.project;
  const gated = registered !== undefined && project === null;
  const [justSet, setJustSet] = useState<string | null>(null);
  // A count that has not been read is left off. Every region keeps its chip, at
  // zero too: structure never moves in response to data.
  const sections: NavSection[] = [
    {
      id: "waiting",
      label: "Waiting on you",
      count: waitingKnown ? waiting : undefined,
      tone: waitingKnown && waiting ? "warn" : undefined,
    },
    // A count only from a read (design §13.6): a read with nothing to give leaves the chip without one.
    { id: "accounts", label: "Accounts", count: ofRead(readOfQuery(inventory), (d) => d.bound) },
    { id: "runs", label: "What runs here", count: registered?.callable ? registered.operations.length : undefined },
    { id: "stats", label: OVERVIEW_TITLE },
  ];

  const meta = [
    said?.product ? `${said.product} ${versionLabel(said.product, said.product_version)}`.trim() : null,
    gated && !justSet ? "no Zitadel project yet" : null,
  ];

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={name}
        lede={
          <>
            Whether Syndra can reach {name} and what it says about itself, who has an account
            there, and anything waiting on you. Syndra <Term name="provision">provisions</Term> an
            account for every person whose roles reach {name}, and keeps it in step.
          </>
        }
        meta={meta.some(Boolean) ? <MetaRow>{meta}</MetaRow> : undefined}
      />

      {/* The project comes first: its step while unchosen, its result for the
          visit it was chosen in. Then the settings card, which opens with the
          system's name, its one write place (addon-contract 3.10a; D-1 Q4), in
          every state: a name changes nobody's access, and a system nobody has
          set up still has one. Its other rows, the only link to the mappings
          among them, wait for the project. */}
      {gated && !justSet ? (
        <ProjectStep target={target} onSet={setJustSet} />
      ) : justSet ? (
        <ProjectSet target={target} project={justSet} />
      ) : null}
      {registered && (
        <SetupStrip target={target} registered={registered} project={justSet ? null : project ?? null} lastKnown={notRead} />
      )}

      {/* One chip per region at the right end of the read line (board, RULES):
          a way to skip a region, never a way to hide one, so a finding cannot
          end up behind a tab nobody selected. Gone while the project is
          unchosen: there is nowhere to go yet. */}
      {!(gated && !justSet) && (
        <ReadLine sections={sections} label="Regions of this page">
          <TargetRead health={health.data} name={name} />
        </ReadLine>
      )}

      <div data-arrive-through className="flex flex-col gap-(--region-gap)">
        <SectionPill sections={sections} label="Regions of this page" gap="var(--region-gap)" />
        {/* Region 0 · whether changes can land.

            Health and maintenance are one question, and they used to sit six
            panels apart. A reachability reading has no meaning on its own: Offline
            while somebody is draining it for a credential rotation is a different
            fact from Offline at 04:00, and an operator who reads the second when
            the first is true walks to the wrong machine.

            Two machines, kept apart (board, A-01): what Syndra can reach, and what
            the system says about itself, each ending by saying which one to look
            at. A fault in either is one Notice above them. */}
        <Region id="land" title="Whether changes can land">
          {health.error && !health.data ? (
            // Syndra's own server failed, not the target: said so, and that
            // nothing was changed. No reading, count or age until it answers.
            <Notice
              tone="danger"
              blocking
              title={`This page for ${name} could not be read.`}
              next="Press Try again. If it keeps failing, ask whoever runs the Syndra server."
            >
              Syndra&rsquo;s server did not answer when this page asked for it. This says nothing about{" "}
              {name} itself. Nothing was changed.
              <HealthTryAgain again={healthAgain} name={name} serverSaid />
            </Notice>
          ) : (
            <>
              <ReachNotice
                target={target}
                health={health.data}
                again={healthAgain}
                // A deployment-side fault, carried here because that is where an
                // operator looks when a target stops working.
                transportError={
                  registered?.transport_status === "error" ? registered.transport_error : undefined
                }
              />

              {/* Stays mounted through its own close: it fades, then its space folds (D-1 M3). */}
              <Notice
                open={waitingKnown && anchorFinding > 0}
                tone="danger"
                title="One item under Waiting on you needs a decision."
                next="Decide it under Waiting on you."
              >
                Until this is settled, what {name} reports cannot be trusted.
              </Notice>

              <CardGrid>
                <ReachCard
                  target={target}
                  health={health.data}
                  isLoading={health.isLoading}
                  transportError={
                    registered?.transport_status === "error" ? registered.transport_error : undefined
                  }
                  busySince={registered?.busy_since}
                />
                <SystemHealth target={target} stale={notRead} />
              </CardGrid>
              {/* Advanced only: lifecycle controls are for whoever looks after the add-on. */}
              <AdvancedOnly gap={16}>
                <LifecycleControl target={target} health={said} isLoading={health.isLoading} />
              </AdvancedOnly>
            </>
          )}
        </Region>

        {/* What reports on the system and its pause stays live above; what
            waits for the project is drawn whole, inert, under one line. */}
        <NotOpenYet
          open={!(gated && !justSet)}
          size="page"
          title={`Waits for ${name}'s Zitadel project`}
          why="Waiting on you, accounts and reconciliation open once it is set."
          step={{ label: "Choose it above ↑", href: "#project" }}
        >
        <div data-arrive-through className="grid gap-(--region-gap)">
        {/* Region 1 · second on the page, and not first.

            Three findings on a target that has not answered for forty minutes
            are three findings nobody can act on, and the band is what says so.
            But it comes before people and before capabilities, because it is
            the only content here that is costing somebody access today. */}
        <Region
          id="waiting"
          title="Waiting on you"
          count={waitingKnown ? waiting : null}
          lede={`Things Syndra found and will not settle by itself: an account that differs from what Syndra expects, a change log on ${name} that was altered, or a change ${name} never answered. Each needs a decision from you.`}
        >
          {/* First, because a person may be waiting on one. A loading or failed
              read shows no rows and no count (A-09 §4 States). */}
          {/* The seat does not come and go (A-20 P5): a card holds the place while
              the reads are out, and says so in words when nothing is waiting. */}
          {!waitingKnown && (unconfirmed.isLoading || findings.isLoading || health.isLoading) && (
            <Card>
              <RowSkeleton rows={1} avatar={false} label="Reading what is waiting on you" />
            </Card>
          )}
          {waitingKnown && clear && (
            // The card opens with what is in it: its border does not arrive ahead of its words.
            <AllClearCard emptied={emptied}>
              <Card>
                <AllClear
                  title="Nothing is waiting on you."
                  guidance={`A change ${name} never answered, an account two records disagree about, or a password that needs a new one appears here.`}
                  emptied={emptied}
                  inCard
                  byReader={drawn?.byReader ?? false}
                />
              </Card>
            </AllClearCard>
          )}
          <UnconfirmedChanges
            target={target}
            operations={unconfirmed.data ?? []}
            ready={!unconfirmed.isLoading && !unconfirmed.error}
          />
          {/* Leaves with its Notice: it fades, then its space folds, so the count follows a row that is drawn. */}
          <Unfold open={anchorFinding > 0} gap={16}>
            {health.data?.log_anchor?.violation_reason ? <LogFinding anchor={health.data.log_anchor} /> : null}
          </Unfold>
          <MergeFindings target={target} onDrawn={onDrawn} />
        </Region>

        {/* Region 2 · the second subject.

            Six panels on this page are about the target and five are about
            people and their access. They stay on one page under a seam rather
            than becoming a second screen: NOTHING BOUND means one thing on a
            target that answered a second ago and something else entirely on one
            that has not answered for forty minutes, and splitting them would
            take the roster away from the only sentence that explains it. */}
        <Region
          id="accounts"
          title={`Accounts on ${name}`}
          lede="Three separate lists: accounts Syndra manages, accounts it did not create, and accounts it created that no role needs any more. Each takes a different action."
        >
          <AccountsNotRead target={target} region="accounts" />
          <PeopleOnTarget target={target} stale={notRead} />
          <Inventory target={target} stale={notRead} />
          <DormantAccounts target={target} stale={notRead} />
        </Region>

        {/* Region 3 · what an add-on can do and what actually runs against it
            are the same question asked twice. The manifest lists the operations
            and reconciliation is the thing that calls them on a schedule. */}
        <Region
          id="runs"
          title="What runs here"
          lede={`What Syndra is able to do on ${name}, and the scheduled check that keeps accounts in step.`}
        >
          {/* Side by side where there is room, as the board draws them. */}
          <CardGrid>
            {registered && <Capabilities registered={registered} />}
            <ReconcileControl target={target} addonNotRead={notRead} />
          </CardGrid>
        </Region>

        {/* Region 4 · what has been used here. Every region keeps its seat:
            an add-on that reports no statistics says so in place. The retention
            control is Advanced's (A-23 R2), the region's last card, so nothing
            above it moves when it comes and goes. */}
        <Region id="stats" title={OVERVIEW_TITLE} lede={OVERVIEW_LEDE}>
          <TargetStats target={target} kind={registered?.kind} name={name} />
          <AdvancedOnly gap={16}>
            <StatsRetention target={target} />
          </AdvancedOnly>
        </Region>
        </div>
        </NotOpenYet>
      </div>
    </div>
  );
}

/** The all-clear's card, opened by Unfold when the list emptied in front of the reader. */
function AllClearCard({ emptied, children }: { emptied: boolean; children: React.ReactNode }) {
  return emptied ? <UnfoldOnArrival>{children}</UnfoldOnArrival> : <>{children}</>;
}

/**
 * The NAS's own account of itself: alerts, pools, services.
 *
 * `health.get` was declared in the manifest, implemented, dispatched and given
 * a policy entry the day the platform landed, and nothing ever called it — so
 * "What it can do" listed a capability with nothing behind it, and the four
 * questions an operator actually asks first when storage misbehaves went
 * unanswered by a system that could already answer them.
 *
 * Rendered as findings rather than as a dump. The reads return several
 * kilobytes each and the add-on keeps three fields from `system.info`; what
 * reaches here is what changes a decision.
 */
function SystemHealth({ target, stale }: { target: string; stale: boolean }) {
  const nameOf = useSystemNames();
  const report = useTargetSystemHealth(target);
  const read = readOfQuery(report);
  const name = nameOf(target);

  // The card keeps its seat while it is asked, with nothing in it. Four calls to
  // the target take a moment, and a card that flashes "no alerts" before the
  // answer arrives has said something false.
  if (read.state === "pending" && !report.isError) {
    return (
      <Card>
        <CardHeader title={`What ${name} reports`} />
        <RowSkeleton rows={4} avatar={false} label={`Loading what ${name} reports`} />
      </Card>
    );
  }
  if (read.state === "pending") {
    return (
      <Card>
        <CardHeader title={`What ${name} reports`} />
        <Reading tone="neutral" label="Not read">
          Syndra has not read {name}&rsquo;s own report yet, so this is not a report that nothing is wrong.
        </Reading>
      </Card>
    );
  }
  if (isNotRead(read)) {
    // Nothing to give (design §13.6): why, and what to check, never "nothing raised".
    return (
      <Card>
        <CardHeader title={`What ${name} reports`} />
        <ReadFreshness read={read} system={name} className="px-5 py-3" />
      </Card>
    );
  }
  const data = read.data;

  const alerts = (data.alerts ?? []).filter((a) => !a.dismissed);
  const pools = data.pools ?? [];
  // Only the ones that matter to a target Syndra provisions accounts on. `cifs`
  // stopped is the single most likely explanation for "my drive vanished", and
  // it is invisible from every other read the add-on makes.
  const services = (data.services ?? []).filter((s) => s.service === "cifs" || s.service === "nfs");
  const degraded = data.degraded ?? [];
  const allRead = degraded.length === 0;

  return (
    // Last known while Syndra cannot reach the system (A-13): the card recedes
    // and its dots ring, its words stay full strength, and it is its own read,
    // so its note carries the age — said once, in amber.
    <Card stale={stale}>
      <CardHeader
        title={`What ${name} reports`}
        note={
          stale ? (
            <span className="inline-flex items-center gap-1.5 text-fine text-warn-text">
              <StatusDot tone="warn" ring />
              Last known · {formatClock(new Date(report.dataUpdatedAt).toISOString())}
            </span>
          ) : allRead ? (
            "All read"
          ) : (
            "Partly read"
          )
        }
      />
      {/* Named sources. "alerts could not be read" and "there are no alerts"
          are the same empty list without this, and they are opposite facts. */}
      {degraded.length > 0 && (
        <Reading tone="warn" label="Partly read">
          {degraded.join(", ")} could not be read. Whatever those would have said is
          missing from this card, not absent from {name}.
        </Reading>
      )}

      {alerts.map((alert, i) => (
        <Reading
          key={`${alert.klass}-${i}`}
          tone={alert.level === "CRITICAL" || alert.level === "ERROR" ? "danger" : alert.level === "WARNING" ? "warn" : "accent"}
          label={ALERT_TITLE[alert.klass ?? ""] ?? `${alertLabel(alert.level)} from ${name}`}
        >
          {/* The device's text is quoted, not rewritten: Syndra cannot know what a disk or a pool
              is saying beyond its words (route-system-targets-id, "its words, not Syndra's"). */}
          {alert.at && <>{name} raised this on {formatShortDate(alert.at)} and has not cleared it. </>}
          These are its words, not Syndra&rsquo;s: &ldquo;{alert.text}&rdquo;{" "}
          <span className="text-ink">Look at {name}.</span>
        </Reading>
      ))}

      {sortByName(pools, (pool) => pool.name, (pool) => pool.name).map((pool) => (
        <Reading
          key={pool.name}
          tone={!pool.healthy ? "danger" : pool.warning ? "warn" : "healthy"}
          label={`Storage pool ${pool.name}`}
        >
          {/* ONLINE is the device's shout; the reader gets the word. */}
          {pool.status.charAt(0) + pool.status.slice(1).toLowerCase()}
          {pool.size_bytes > 0 && (
            <>
              {" · "}
              {formatBytes(pool.allocated_bytes)} of {formatBytes(pool.size_bytes)} used
            </>
          )}
        </Reading>
      ))}

      {services
        .filter((s) => s.state !== "RUNNING")
        .map((s) => (
          <Reading key={s.service} tone="danger" label={`${serviceName(s.service)} is not running`}>
            {SERVICE_STOPPED}
            {s.enable
              ? `, and it is set to start on boot, so it stopped by itself. Check ${name}.`
              : ", and it is not set to start on boot."}
          </Reading>
        ))}

      {degraded.length === 0 &&
        alerts.length === 0 &&
        pools.every((p) => p.healthy && !p.warning) &&
        services.every((s) => s.state === "RUNNING") && (
          <Reading tone="healthy" label="Nothing raised">
            No alerts, every pool healthy, and the sharing services are running.
          </Reading>
        )}

      {data.system?.hostname && (
        <Reading tone="neutral" label={data.system.hostname}>
          {data.system.version ?? ""}
        </Reading>
      )}
    </Card>
  );
}

/** The alert classes the connection is known to report, in plain words; any other is named by its level. */
const ALERT_TITLE: Record<string, string> = {
  SMARTUncorrectedErrors: "A disk reports errors it could not correct",
  RESTAPIUsage: "Something used an interface TrueNAS is retiring",
  HasUpdate: "An update is available",
};

function alertLabel(level: string): string {
  switch (level) {
    case "CRITICAL":
      return "Critical";
    case "ERROR":
      return "Error";
    case "WARNING":
      return "Warning";
    default:
      return "Notice";
  }
}

/**
 * Reconcile now, and what the pass found.
 *
 * The sweep runs every six hours and writes a log line nobody reads. An
 * operator asking "is this target in step?" had no way to ask it — the button
 * existed for Zitadel and for no target.
 *
 * The result is rendered rather than toasted, because the interesting half is
 * not "done": it is the STALE bindings, which the sweep deliberately refuses to
 * act on and which nothing else surfaces. A binding whose account is gone plans
 * as a create, and acting on it would recreate an account somebody deleted.
 */
function ReconcileControl({ target, addonNotRead }: { target: string; addonNotRead: boolean }) {
  const nameOf = useSystemNames();
  const run = useReconcileTarget(target);
  const release = useReleaseBinding(target);
  // Which row asked, and which rows have already let go. The reconcile result
  // is a mutation's answer rather than a query, so a released row would keep
  // listing itself until somebody pressed Reconcile again — and a row that
  // still says "points at nothing" after being acted on reads as a press that
  // did not work.
  const [confirming, setConfirming] = useState<string | null>(null);
  const [released, setReleased] = useState<string[]>([]);
  // A 2xx that is not a release. `request` resolves on any 2xx, and the backend
  // answers 202 for two states that are emphatically NOT done: the target did
  // not confirm, and the add-on let go while Syndra's own copy did not. The
  // second is a split binding, and both are repaired by pressing again — so the
  // press must stay on screen, which is what marking the row released removed.
  const [unfinished, setUnfinished] = useState<Record<string, string>>({});
  // The last settled result. A press makes a new mutation whose data is undefined until it answers;
  // the old words stay on screen until the new are read, and the pending gap is not a change (D-1 M4).
  const settled = useRef(run.data);
  if (run.data) settled.current = run.data;
  const result = settled.current;
  const name = nameOf(target);
  // A pass skipped inside the add-on's cooldown waits out its retry_after and runs once more by
  // itself (RFC 9110 §10.2.3), so one press ends on a live read; a second skip is shown as it is.
  const retry = useRetryOnce();
  const waiting = retry.waiting;
  // A pass with nothing to read is a 503 TARGET_NOT_READ (design §13.3): a read that is none, with
  // its why, never a result of zeros. It is newer than any result kept above while it stands.
  const notRead = notReadOf(run.error);
  // When the last skipped pass arrived: its count runs from there once nothing goes again by itself
  // (a second skip, or Stop waiting), and the card says it, never the pass's raw reason.
  const [skippedAt, setSkippedAt] = useState(0);
  const skipped = waiting ? undefined : notRead ? cooldownOf503(notRead.why) : result ? skipWait(result) : undefined;
  const left = useCountdown(skipped, skippedAt);
  // What the check concluded, as the sentence below is worded from it. A new result crossfades
  // its words and washes the row once; the same result read again is not news (D-1 M4).
  const resultKey = notRead
    ? `none:${notRead.why.reason}:${notRead.why.cause ?? ""}`
    : !result
      ? "none"
      : result.halted
        ? `halted:${result.reason ?? ""}:${result.detail ?? ""}:${result.read.state === "now" ? "" : (result.read.why.retryAfter ?? "")}`
        : `read:${result.read.state}:${result.bound}:${result.queued}:${result.stale?.length ?? 0}`;
  const changed = useFlashOnChange(resultKey);
  // A check past ten seconds says so, once, on tint (A-24 §6). Only that: its result is a mutation's
  // answer, gone if the page is left, so it never says the result will be here on return (§13).
  const long = useLongWait(run.isPending);
  function check() {
    const skip = (wait: number | undefined) => {
      if (!wait) return;
      setSkippedAt(Date.now());
      retry.refused(wait, check);
    };
    run.mutate(undefined, {
      onSuccess: (pass) => skip(skipWait(pass)),
      onError: (e) => {
        const none = notReadOf(e);
        skip(none ? cooldownOf503(none.why) : undefined);
      },
    });
  }

  return (
    <Card>
      <CardHeader
        title="Check for differences"
        caption={`Compares ${name} with what Syndra expects and records any fixes as changes waiting for you to send. Nothing is applied until somebody sends them from Pending changes.`}
      />
      <CardRow
        top
        seat="narrow"
        className={changed ? "flash" : undefined}
        action={
          /* It reads the system first, so it waits while that can't answer
              (A-13 §1.5). The card's one control, so its reason rides on it. */
          <Button
            variant="outline"
            size="sm"
            isPending={run.isPending}
            disabled={addonNotRead || waiting || left > 0}
            reason={
              waiting
                ? retryingLine(name, retry.left)
                : left > 0
                  ? cooldownLine(name, left)
                  : addonNotRead
                    ? `Check now waits too: it reads ${name} first.`
                    : undefined
            }
            onClick={() => retry.press(check)}
          >
            Check now
          </Button>
        }
      >
        <Crossfade id={long ? "long" : waiting ? "waiting" : skipped ? "skipped" : resultKey} className="flex-1 text-body text-muted">
          {long ? (
            `Still reading ${name}.`
          ) : waiting ? null : skipped ? (
            <span className="text-warn-text">
              {left > 0 ? `${cooldownLine(name, left)}. ` : ""}
              {checkAgainNext(left)}
            </span>
          ) : notRead ? (
            // Nothing to read: §13.1's sentence, which says which side is down and what to check.
            <span className="text-warn-text">
              The check stopped before it concluded anything. {whyLine(notRead.why, name, OPERATOR)}
            </span>
          ) : !result ? (
            "Syndra checks by itself every six hours."
          ) : result.halted && result.detail ? (
            // The add-on answered and refused the read (read_refused): its own
            // sentence says what failed and what to do, so "once it is
            // reachable" would be untrue.
            <span className="text-warn-text">
              The check stopped before it concluded anything. {result.detail}
            </span>
          ) : result.halted ? (
            // The reason in words (whyNotChecked), never its code, and a next
            // step true of it: a refused read is fixed, not waited out.
            <span className="text-warn-text">
              The check stopped before it concluded anything.{" "}
              {result.reason ? `${whyNotChecked(result.reason)} ` : ""}
              {result.reason === "read_refused"
                ? `Check ${name}'s add-on, then check again.`
                : `Nothing was learned about ${name}; try again once it is reachable.`}
            </span>
          ) : (
            reconcileLine(result.bound, result.queued, result.stale?.length, name)
          )}
        </Crossfade>
      </CardRow>
      <RetryWait name={name} retry={retry} onStop={retry.stop} />

      {run.error && !notRead && (
        <CardRow>
          <span className="text-small text-danger-text">
            {sentence(describeFailure(run.error))}
          </span>
        </CardRow>
      )}

      {result && !notRead && result.read.state !== "now" && !result.halted && !waiting && !skipped && (
        // A pass that concluded nothing must not read as a clean one.
        <CardRow>
          <span className="text-small text-warn-text">
            Nothing was concluded: {result.reason ? whyNotChecked(result.reason) : `${name} could not be read.`}
          </span>
        </CardRow>
      )}

      {(result?.stale?.length ?? 0) > 0 && (
        <>
          <CardRow>
            <span className="text-body font-semibold text-warn-text">
              {result!.stale!.length === 1
                ? "One person is"
                : `${result!.stale!.length} people are`}{" "}
              recorded as owning an account that is no longer on {name}
            </span>
          </CardRow>
          <ArriveList>
          {sortByName(result!.stale!, (b) => b.username, (b) => b.subject_id).map((b) => (
            <CardRow
              key={b.subject_id}
              seat="default"
              action={
                released.includes(b.subject_id) ? undefined : confirming === b.subject_id ? (
                  <>
                    <Button variant="ghost" size="sm" onClick={() => setConfirming(null)}>
                      Keep tracking it
                    </Button>
                    <Button
                      variant="accent"
                      size="sm"
                      isPending={release.isPending}
                      onClick={() =>
                        release.mutate({ subjectId: b.subject_id, version: b.version }, {
                          onSuccess: (res) => {
                            // Released, and nothing left over. Anything else keeps
                            // the row and its control, and says what the backend
                            // said — including the sentence naming the repair.
                            if (res.status === "released" && !res.warning) {
                              setReleased((prev) => [...prev, b.subject_id]);
                              setConfirming(null);
                              return;
                            }
                            // Sent, the answer lost (202): the one unconfirmed outcome every write says.
                            const unsure = unconfirmedOutcome();
                            setUnfinished((prev) => ({
                              ...prev,
                              [b.subject_id]: isUnconfirmed(res)
                                ? `${sentence(unsure.message)} ${unsure.next}`
                                : res.warning || res.detail || `${name} did not confirm. Check this list again before trying again.`,
                            }));
                          },
                        })
                      }
                    >
                      Stop tracking {b.username}
                    </Button>
                  </>
                ) : (
                  <Button variant="outline" size="sm" onClick={() => setConfirming(b.subject_id)}>
                    Stop tracking this account
                  </Button>
                )
              }
              detail={
                released.includes(b.subject_id) ? (
                  <>Stopped tracking it. Nothing on {name} was changed.</>
                ) : confirming === b.subject_id ? (
                  <>
                    {/* What it does, in the sentence under the button that does
                        it. The word "forget" is doing real work here: an operator
                        reading it as "delete" is the one misreading this row can
                        afford least, given the account it names is already gone. */}
                    Syndra stops managing <Mono>{b.username}</Mono>. Nothing is deleted, and it can
                    be tracked again by assigning the account to a person if it comes back.
                    {unfinished[b.subject_id] && (
                      <span className="mt-1 block text-warn-text">{unfinished[b.subject_id]}</span>
                    )}
                  </>
                ) : (
                  <>
                    Not fixed by itself: recreating it would bring back an account somebody deleted,
                    so this is yours to decide.
                  </>
                )
              }
            >
              <Mono className="min-w-0 [overflow-wrap:anywhere]">{b.username}</Mono>
            </CardRow>
          ))}
          </ArriveList>
          {release.error && (
            <CardRow>
              <ActionOutcome placement="inline" word={notDoneWord(release.error, "Not released")} outcome={outcomeFromError(release.error)} />
            </CardRow>
          )}
        </>
      )}
    </Card>
  );
}

/**
 * Every dysfunction is one Notice (RULES), above the two cards: Syndra cannot
 * make a request at all, the system has gone quiet, or Syndra stopped calling
 * it. The reading itself, and the check pips, are the first row of Syndra's
 * reach below.
 */
function ReachNotice({
  target,
  health,
  again,
  transportError,
}: {
  target: string;
  health: TargetHealthReply | undefined;
  again: HealthAgain;
  transportError?: string;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);

  if (transportError) {
    return (
      <Notice tone="danger" blocking title="Cannot connect." next="Ask whoever runs the Syndra server to fix the key. Syndra connects on its next check.">
        Syndra cannot read the key it uses to talk to {name}, so it cannot make any request. This is
        a problem on <strong className="font-semibold">the Syndra server</strong>, not on {name}: {transportError}
      </Notice>
    );
  }

  // The monitor's confirmed answer leads (design §13.2): the add-on down, or its app down, named
  // apart, each with what to check. One failed live read is not an outage.
  const status = health?.status;
  if (status === "addon_unreachable" || status === "app_unreachable") {
    const reading = targetReading({ status, health: health?.health, name });
    return (
      <Notice
        tone={reading.tone === "danger" ? "danger" : "warn"}
        live
        title={status === "addon_unreachable" ? `${name}'s add-on isn't answering.` : `${name} isn't answering.`}
        next="Nothing is lost: Syndra sends what waits once it answers."
      >
        {reading.sentence}
        <HealthTryAgain again={again} name={name} />
      </Notice>
    );
  }

  return null;
}

type HealthAgain = ReturnType<typeof useTargetHealthAgain>;

/** The page's reader: an operator. */
const OPERATOR: Reader = { audience: "operator", person: "" };

/**
 * "Try again" on the health page: a live read (`?fresh=true`). It says what it is doing: Reading
 * while it asks; a wait Syndra put on it (429) with its count; and a failure of Syndra's own
 * server, told apart from the add-on's, which the reach card says from the live read itself.
 * `serverSaid`: its Notice already says Syndra's server failed and what to do, so a failure here
 * says only what came back.
 */
function HealthTryAgain({ again, name, serverSaid = false }: { again: HealthAgain; name: string; serverSaid?: boolean }) {
  const { error, errorAt, pending } = again;
  const wait = error instanceof ApiError && error.status === 429 ? error.retryAfter : undefined;
  const left = useCountdown(wait, errorAt);
  const said = pending
    ? `Reading ${name} now.`
    : wait !== undefined
      ? `Syndra is being asked too often. ${cooldownNext(left)}`
      : error
        ? serverSaid
          ? sentence(failureMessage(error))
          : `${sentence(failureMessage(error))} This says nothing about ${name} itself. ${nextStep(false)}`
        : "";
  // A block of its own: a button is never inside the Notice's sentence (D-1 §4). Its words are
  // said by the Notice's own live region, so no second region speaks them again.
  return (
    <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
      <Button variant="outline" size="sm" keepFocus isPending={pending} disabled={left > 0} onClick={() => void again.press().catch(() => {})}>
        {pending ? READING : "Try again"}
      </Button>
      <span data-try-again-said className="text-small text-muted">
        {said}
      </span>
    </div>
  );
}

/**
 * What Syndra can reach: the reading first (dot, word, the check pips when they
 * say something, and one faint line), then what is wrong on the Syndra side.
 * Each ends by saying which machine to look at.
 */
function ReachCard({
  target,
  health,
  isLoading,
  transportError,
  busySince,
}: {
  target: string;
  health: TargetHealthReply | undefined;
  isLoading: boolean;
  transportError?: string;
  busySince?: string;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const { view, revealInAdvanced } = useUiView();
  const said = health ? dataOf(health.addon) : undefined;

  // Nothing read yet: no word, no count, no age.
  if (isLoading && !health) {
    return (
      <Card>
        <CardHeader title="Syndra's reach" />
        <RowSkeleton rows={3} avatar={false} label={`Loading whether changes can land`} />
      </Card>
    );
  }

  const cannot = Boolean(transportError);
  const status = health?.status;
  const reading = targetReading({ status, health: health?.health, name });
  const recent = health?.health?.recent ?? [];
  const lifecycle = said?.lifecycle ?? health?.health?.lifecycle ?? "active";
  const paused = lifecycle !== "active";
  // The add-on's own breaker toward the system, while Syndra's checks still
  // answer: the connector on the Syndra server is holding off, and says so.
  const holdingOff = said?.circuit_open;
  const fault = cannot || status === "addon_unreachable" || status === "app_unreachable";

  // A fault's sentence, and where to look, are the Notice above; its row is
  // the word, the pips and the one faint line.
  let detail: React.ReactNode = null;
  if (!fault) {
    detail =
      health && status !== "checking"
        ? holdingOff
          ? `The add-on on the Syndra server is waiting before it calls ${name} again, after repeated failures, and tries again by itself. Changes for ${name} wait, and nothing is lost.`
          : paused
            ? `Syndra can reach ${name}, but it is ${stateLabel(lifecycle)}. ${lifecycleWhy(said?.lifecycle_note)} ${
                lifecycle === "draining"
                  ? "New changes are refused and the ones already sent are being allowed to finish."
                  : "Every change is refused at once. Reads keep working."
              }`
            : isNotRead(health.addon)
              ? // The add-on's answer was not read (a missed check, a live "Try again" that came back
                // with none): never "accepting changes"; its why, in §13.1's words, says which side.
                [reading.sentence, whyLine(health.addon.why, name, OPERATOR, said?.product)].filter(Boolean).join(" ")
              : (reading.sentence ??
                (said?.in_flight ? `Accepting changes. ${said.in_flight} in flight.` : "Accepting changes. Nothing is in flight."))
        : reading.sentence;
  }
  const faint =
    reading.line ??
    (!fault && health?.health?.checked_at ? <>Answered <Relative iso={health.health.checked_at} /></> : null);

  return (
    <Card>
      <CardHeader title="Syndra's reach" />
      <Reading
        tone={reading.tone}
        seat="narrow"
        // The next step. Maintenance is Advanced only and structure does not move with data,
        // so in Basic this is how to reach it: it switches the view and glides there.
        action={
          !fault && health && status !== "checking" && !holdingOff && paused ? (
            <Button size="sm" onClick={() => revealInAdvanced("maintenance")}>
              {view === "advanced" ? "Open Maintenance" : "Open Maintenance in Advanced"}
            </Button>
          ) : undefined
        }
        label={
          <>
            {reading.word}
            {reading.pips && <CheckPips recent={recent} />}
            {faint && <span className="text-small font-normal text-faint">{faint}</span>}
          </>
        }
      >
        {detail}
      </Reading>
      <ConnectionNotices target={target} health={said} busySince={busySince} />
    </Card>
  );
}

/**
 * Syndra-side warnings that stand whether or not the target is reachable, and
 * that the one-sentence status card above has no room for. Renders nothing
 * when there is nothing to say — a healthy target gets no extra card, same as
 * /zitadel.
 */
function ConnectionNotices({
  target,
  health,
  busySince,
}: {
  target: string;
  health: TargetHealth | undefined;
  /** Set when the send stopped because the system has been busy for over half an hour (backend propagation busyHaltAfter). */
  busySince?: string;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const auto = useAutoRetry();
  const notices: React.ReactNode[] = [];

  // Busy is not down: the add-on answers, so it is said whatever the reach.
  if (busySince) {
    notices.push(
      <Reading key="busy" tone="warn" label={`${name} has been busy since ${formatWhen(busySince)}`}>
        No change was sent. {busyNext(auto, name)}
      </Reading>,
    );
  }
  if (!health?.reachable) return notices.length ? <>{notices}</> : null;

  if (health.in_flight !== undefined && health.in_flight > 0) {
    notices.push(
      <Reading key="in-flight" tone="warn" label="Still finishing">
        {health.in_flight} change{health.in_flight === 1 ? "" : "s"} sent before the pause{" "}
        {health.in_flight === 1 ? "has" : "have"} not completed. Wait for this to reach zero
        before changing the {name} API key.
      </Reading>,
    );
  }

  // A credential whose expiry nobody recorded. Not amber for its own sake:
  // the key CAN expire without Syndra knowing, and the day it does the
  // target simply stops answering — which reads as an outage and sends an
  // operator to the NAS. `none` is an operator's deliberate choice and says
  // nothing here.
  if (health.key_expiry === "unrecorded") {
    notices.push(
      <Reading key="key-expiry" tone="warn" label="API key expiry not recorded">
        Syndra does not know when the {name} API key (a password for a program rather than a
        person) expires. For whoever runs the Syndra server: if it has an expiry, set{" "}
        <Mono>TRUENAS_API_KEY_EXPIRES_AT</Mono> in the deployment&rsquo;s <Mono>.env</Mono> so
        Syndra warns before it fails; if it never expires, set it to <Mono>never</Mono>.
      </Reading>,
    );
  }

  // Auditing off means activity reports are empty, and an empty report is
  // indistinguishable from a member who did nothing. Said here so it is
  // learned before somebody depends on it.
  if (health.shares_readable && (health.unaudited_shares?.length ?? 0) > 0) {
    notices.push(
      <Reading key="unaudited" tone="warn" label={UNAUDITED_LABEL}>
        <UnauditedShares shares={health.unaudited_shares!} name={name} />
      </Reading>,
    );
  }

  if (health.version_tested === false) {
    notices.push(
      <Reading key="version" tone="warn" label={`Untested ${name} version`}>
        {health.version_note || `This version of ${name} has not been tested with Syndra.`} Reads
        keep working; changes are refused.
      </Reading>,
    );
  }

  if (notices.length === 0) return null;

  return <>{notices}</>;
}

/**
 * How current the reading is, said once for the page: the add-on's account of itself, dated by its
 * read (design §13.3). A read with nothing to give has no date to say: its why is said once, by the
 * fault Notice or the reach card, never again here.
 */
function TargetRead({ health, name }: { health: TargetHealthReply | undefined; name: string }) {
  return health && !isNotRead(health.addon) ? <ReadFreshness read={health.addon} subject={name} /> : null;
}

/**
 * What the add-on can perform, read from its manifest.
 */
/**
 * What each operation does, in the board's plain verbs, for the ids the connection is known to
 * report. The id stays beneath in mono: it is what an operator matches against the manifest.
 */
export const OPERATION_LABEL: Record<string, string> = {
  ...STORAGE_OPERATION_LABEL,
  "account.provision": "Creates an account",
  "account.converge": "Brings an account in line",
  "account.release": "Lets go of an account, keeping it",
  "account.adopt": "Assigns an existing account to a person",
  // Generic: this serves every system, and the manifest gives an operation no description, so it
  // claims nothing about what a system does with the account's files (TrueNAS's user.delete keeps
  // the home folder: middleware TS-25.10.7, plugins/account.py do_delete, but that is its fact).
  "account.purge": "Deletes an account",
  "account.list": "Reads the list of accounts",
  "health.get": "Reads the system's own health report",
  "credential.set": "Sets a member's password, never logged",
  "password.rotate": "Replaces a person's password, never logged",
  "password.set": "Lets a member set their own storage password, never logged",
  "holdings.list": "Reads what a member holds",
  "item.checkout": "Records that a member took an item",
  "storage.status": "Reads space used and quota",
};

export function Capabilities({ registered }: { registered: TargetSummary }) {
  const name = registered.name;
  return (
    <>
    {registered.last_error && (
      // The add-on's latest description of itself was refused, and the last good one still runs.
      // Said above the card it is about, with the reason as the backend gave it.
      <Notice
        tone="warn"
        title={`${name}'s latest description of itself was not accepted`}
        next={`Ask whoever looks after ${name}'s add-on to fix it. Syndra reads it again on its own.`}
      >
        Syndra is still using the last good one, so what this page lists has not changed. The reason:{" "}
        <span data-last-error className="type-mono text-small [overflow-wrap:anywhere]">{registered.last_error}</span>
      </Notice>
    )}
    <Card>
      <CardHeader
        title="What it can do"
        note={`As reported by the connection to ${name}`}
      />
      {!registered.callable ? (
        <div className="px-5 pb-5">
          <p className="text-body text-muted">
            Configured, but {name} has not answered yet. Until it does, Syndra does not know
            what it can do here.
          </p>
        </div>
      ) : (
        sortByName(
          registered.operations,
          (op) => OPERATION_LABEL[op.id] ?? op.id,
          (op) => op.id,
        ).map((op, i) => (
          <CardRow key={op.id} first={i === 0}>
            <span className="flex min-w-0 flex-[1_1_200px] flex-col gap-0.5">
              <span className="text-body">
                {OPERATION_LABEL[op.id] ?? `Something ${name} can do that Syndra has no name for`}
              </span>
            </span>
            {/* The manifest says which operations stop and ask, and this list is the only place an
                operator can learn that before pressing one. The secret parameters are named in the
                words of the operation ("never logged"), not listed, and the scope is the
                manifest's business, not the reader's. */}
            <StatusSeat tone={op.confirm ? "warn" : "neutral"} word={op.confirm ? "Stops and asks" : "Runs by itself"} />
            {!op.available && (
              // Shown disabled with its reason rather than omitted: omitted, an
              // operator wonders whether the feature exists.
              <span className="basis-full text-small text-warn-text">
                Not available: {op.unavailable_reason}
              </span>
            )}
          </CardRow>
        ))
      )}
      {registered.callable && registered.operations.length > 0 && (
        <AdvancedOnly>
          <div className="row-divider px-5 py-3">
            <TechnicalDetail placeKey={`capabilities:${registered.target}`}>
              {sortByName(registered.operations, (op) => op.id, (op) => op.id).map((op) => (
                <CopyableValue key={op.id} value={op.id} label={`Operation ${OPERATION_LABEL[op.id] ?? "id"}`} />
              ))}
            </TechnicalDetail>
          </div>
        </AdvancedOnly>
      )}
      <AdvancedOnly>
        <AlsoSays lines={registered.also_says} name={name} />
      </AdvancedOnly>
    </Card>
    </>
  );
}

/**
 * What the add-on says about itself that Syndra does not use, quoted exactly as
 * the backend carries it and never acted on. Always here, empty or not:
 * structure never moves in response to data.
 */
export function AlsoSays({ lines, name }: { lines: string[] | undefined; name: string }) {
  return (
    <div className="row-divider grid gap-1.5 px-5 py-3" data-also-says>
      <span className="type-label text-label">What the add-on also says</span>
      {lines && lines.length > 0 ? (
        <>
          <ul className="grid gap-1">
            {lines.map((line, i) => (
              <li key={i} className="type-mono text-fine leading-[1.5] text-muted [overflow-wrap:anywhere]">
                {line}
              </li>
            ))}
          </ul>
          <p className="text-small text-faint">Shown as {name} sent it. Syndra does not use any of it.</p>
        </>
      ) : (
        <p className="text-small text-faint">Nothing beyond what Syndra uses.</p>
      )}
    </div>
  );
}

/**
 * The mutation log no longer extending the one Syndra remembers.
 *
 * Not a health state, and rendered apart from them: the target can be perfectly
 * healthy and still be reporting a record that has been edited. This is the
 * strongest evidence the system produces, and until recently it reached an
 * operator as one line in a log file.
 */
function LogFinding({ anchor }: { anchor: LogAnchor }) {
  const nameOf = useSystemNames();
  const [resolving, setResolving] = useState(false);
  const name = nameOf(anchor.target);
  // Read from the two counts, not from the verdict.
  //
  // `head_rewritten` has TWO producers in ClassifyLogHead: the same number of
  // records hashing to something else, and records having GROWN while the head
  // stayed put (db/log_anchor.go:75-84). This said "the number of entries is
  // the same" for both, so the second case — new entries that do not chain onto
  // anything Syndra verified, which is the more alarming one — was reported as
  // its opposite. The counts are both on screen anyway; they cannot disagree
  // with themselves.
  const seen = anchor.records;
  const reported = anchor.violation_records ?? 0;
  const what =
    reported < seen
      ? "Entries that existed are gone."
      : reported > seen
        ? "Entries were added, but the log still ends where it did. The new ones do not follow on from anything Syndra has verified."
        : "The number of entries is the same, but their contents changed.";
  return (
    <Notice
      tone="danger"
      title={`The change log on ${name} has been altered`}
      action={{ label: "Resolve this finding", onClick: () => setResolving(true) }}
      next="Find out why the log changed. Resolve this finding only once you know."
    >
      <p>
        {what} Syndra last saw {anchor.records} {anchor.records === 1 ? "entry" : "entries"}{" "}
        <Relative iso={anchor.anchored_at} />; {name} reported {anchor.violation_records ?? 0}{" "}
        <Relative iso={anchor.violation_at} />.
      </p>
      <p className="mt-1 font-semibold text-danger-text">
        If you do not know why the log changed, do not resolve this. Ask the person who runs the
        Syndra server.
      </p>
      <p className="mt-1 text-small text-faint">
        This stays until somebody resolves it. Syndra&rsquo;s <Term name="logAnchor">log anchor</Term>{" "}
        is the only thing that can notice entries disappearing, and resolving replaces it with a
        new <Term name="baseline">baseline</Term>.
      </p>
      <TechnicalDetail className="mt-2" placeKey={`log:${anchor.target}`}>
        <p className="text-small text-faint">
          Syndra&rsquo;s note: {anchor.records} {anchor.records === 1 ? "entry" : "entries"}{" "}
          ending <Mono>{anchor.head.slice(0, 12)}</Mono>, made <Relative iso={anchor.anchored_at} />.{" "}
          {name} reported {anchor.violation_records ?? 0}{" "}
          {anchor.violation_head ? (
            <>
              ending <Mono>{anchor.violation_head.slice(0, 12)}</Mono>{" "}
            </>
          ) : null}
          <Relative iso={anchor.violation_at} />.
        </p>
      </TechnicalDetail>
      {resolving && (
        <ResolveFindingDialog target={anchor.target} anchor={anchor} onClose={() => setResolving(false)} />
      )}
    </Notice>
  );
}

/**
 * Clearing the finding, which means adopting the log that produced it.
 *
 * Rung 3 and the same gesture as every other irreversible action, because this
 * is the only one in the product that discards EVIDENCE: after it, the records
 * that went missing are still missing and Syndra can no longer tell anybody
 * they did. The copy leads with that rather than with what the button does.
 *
 * The head is cited, so an operator adopts what they read. A log that changed
 * again while this was open is refused by the backend and said out loud — that
 * second change is exactly the event the anchor exists to notice, and it must
 * not be swallowed by the action taken to clear the first.
 */
function ResolveFindingDialog({
  target,
  anchor,
  onClose,
}: {
  target: string;
  anchor: LogAnchor;
  onClose: () => void;
}) {
  const nameOf = useSystemNames();
  const resolve = useResolveLogFinding(target);
  const [note, setNote] = useState("");
  const confirmation = useTypedConfirmation(target);
  const ready = confirmation.armed && note.trim() !== "" && !resolve.isPending;
  const [done, setDone] = useState(false);

  return (
    <Modal open onClose={resolve.isPending ? () => {} : onClose} busy={resolve.isPending} size="md" labelledBy="resolve-finding-title">
      <ModalHeader
        titleId="resolve-finding-title"
        title="Resolve this finding"
        lede={done ? undefined : `Syndra will accept the log as ${nameOf(target)} reports it now, and compare against that from here on.`}
      />
      {done ? (
        <div className="grid gap-3 px-6">
          <ActionOutcome
            word="Resolved"
            outcome={{ kind: "applied", message: `Syndra takes the log as ${nameOf(target)} reports it now.` }}
          />
        </div>
      ) : (
      <div className="grid gap-4 px-6">
        <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
          <p className="text-small text-danger-text">
            The records that went missing stay missing, and Syndra stops being able to tell
            you they did. Do this when you know why the log changed, such as a rebuilt Syndra server
            or a replaced volume, and not to clear a warning. If you do not know, ask the person
            who runs the Syndra server first.
          </p>
        </div>
        <div>
          <FieldLabel htmlFor="finding-note">Why the log changed</FieldLabel>
          <Input
            id="finding-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="We replaced the add-on&rsquo;s volume on the 4th"
          />
          <FieldHint>
            Kept with the resolution. &ldquo;We replaced the volume&rdquo; and &ldquo;we do
            not know&rdquo; look the same to Syndra and are completely different facts.
          </FieldHint>
        </div>
        <ConfirmByTyping
          expected={target}
          value={confirmation.typed}
          onChange={confirmation.setTyped}
          noun="system name"
          disabled={resolve.isPending}
        />
        {resolve.error && <ActionOutcome word="Not resolved" outcome={outcomeFromError(resolve.error)} />}
      </div>
      )}
      {done ? (
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </ModalFooter>
      ) : (
      <ModalFooter
        cancel={{ onClick: onClose, disabled: resolve.isPending }}
        note="Syndra takes the log as it is now and compares against that from here on."
      >
        {/* dangerConfirm (G25): the records that went missing are gone for good. */}
        <Button
          variant="dangerConfirm"
          disabled={!ready}
          isPending={resolve.isPending}
          reason={
            !ready && !resolve.isPending
              ? note.trim() === ""
                ? "Say why the log changed. This is kept with the resolution."
                : !confirmation.armed
                  ? `Type ${target} to confirm.`
                  : undefined
              : undefined
          }
          onClick={() => {
            resolve.reset();
            resolve.mutate(
              { head: anchor.violation_head ?? "", note },
              { onSuccess: () => setDone(true) },
            );
          }}
        >
          Accept this log and start over
        </Button>
      </ModalFooter>
      )}
    </Modal>
  );
}

/**
 * One reading as a card row (board): a dot that carries the tone, a bold word,
 * and what it means, ending in where to look.
 */
function Reading({
  tone,
  label,
  seat,
  action,
  children,
}: {
  tone: StatusTone;
  label: React.ReactNode;
  /** The seat's width, when the card has one: the same on every row of it, whether or not this one has an action. */
  seat?: SeatWidth;
  /** A button, in the seat on the line that says the word. */
  action?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <CardRow
      seat={seat}
      action={action}
      detail={children && <span className="block pl-(--dot-indent)">{children}</span>}
    >
      {/* A share held to a minimum, so where it and the seat do not share a line (390) flex drops the
          seat under it, whatever the words say; past the minimum a long word (a hostname) wraps. With
          `min-w-0` it reported 0 to the wrap and its state word was cut to 65px of the 82 it needs
          (layout check, staging, 20 Nov 2026). */}
      <div className="flex min-w-[min(120px,100%)] flex-1 gap-2">
        <StatusDot tone={tone} className="mt-[7px]" />
        <span className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 text-body font-semibold [overflow-wrap:anywhere]">{label}</span>
      </div>
    </CardRow>
  );
}

/**
 * The Accounts region's one "could not be read": a read with nothing to give
 * (design §13.6), so the lists below have no count and no empty state, and this
 * line says why once, above them all, in §13.1's words, with Read again on it.
 * Inside the cooldown it says the one wait wording (`cooldownNext`).
 */
function AccountsNotRead({ target, region }: { target: string; region: string }) {
  const nameOf = useSystemNames();
  const read = readOfQuery(useTargetInventory(target));
  const again = useTargetInventoryAgain(target);
  const why = isNotRead(read) ? read.why : undefined;
  const cooldown = why?.reason === REASON.cooldown;
  const left = useCountdown(cooldown ? why?.retryAfter : undefined, why?.since);
  const lineId = useId();
  // Set by a press, consumed once, when the press settles: a read that lands takes this line, and its
  // focused button, away, so focus goes to the region's heading rather than to <body>. A press that
  // settles with nothing again (none, or a failure) only clears it, so a later read that lands by
  // itself never moves focus nobody asked to move.
  const pressed = useRef(false);
  const unread = isNotRead(read);
  useEffect(() => {
    if (!pressed.current || again.pending) return;
    pressed.current = false;
    if (!unread && document.activeElement === document.body) focusHeading(document.getElementById(`${region}-heading`));
  }, [read, unread, again.pending, region]);
  if (!why) return null;
  const name = nameOf(target);
  const line = cooldown
    ? cooldownLine(name, left)
    : `The account list could not be read. ${whyLine(why, name, OPERATOR)}`;
  return (
    <div className="grid gap-1">
      {/* The sentence and its button side by side, as ReadFreshness draws them (a button is never in prose). */}
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-small text-warn-text">
        <span id={lineId}>{line}</span>
        <ReadAgainButton
          onClick={() => {
            pressed.current = true;
            void again.press().catch(() => {});
          }}
          pending={again.pending}
          cooldown={left > 0 ? line : undefined}
          describedBy={lineId}
        />
      </div>
      <ReadAgainError error={again.error} className="" />
    </div>
  );
}

/**
 * The unmanaged inventory (1.18/1.19).
 *
 * Never drift, and never rendered as drift: a real NAS holds `root`, service
 * accounts and whatever an admin made by hand, and classifying those as untraced
 * access would bury the triage queue on the first sweep after deployment. Trust
 * in a triage queue is set the day it first fills.
 *
 * Adoption is blocked while the read is stale, and this is the deliberate half of
 * §31 A's split: adopting binds an identity irreversibly off a list that may have
 * moved, while applying a plan only joins a queue somebody can still inspect. The
 * two must not be unified.
 */
function Inventory({ target, stale }: { target: string; stale: boolean }) {
  const nameOf = useSystemNames();
  const inventory = useTargetInventory(target);
  const again = useTargetInventoryAgain(target);
  const adopt = useAdoptAccount(target);
  const [adopting, setAdopting] = useState<string | null>(null);
  // The outcome is kept WITH the account it happened to. It used to be one
  // unattributed result at the foot of the card, which on a list of eight
  // accounts said "Adopted." about none of them in particular.
  const [result, setResult] = useState<
    { username: string; person: string; result: AdoptionResult } | null
  >(null);

  // A read with nothing to give: no list, no count, no freshness (AccountsNotRead says so, once).
  const read = readOfQuery(inventory);
  // Assigning binds an identity for good: only off a read taken now, and young enough (§31 A). A kept
  // copy, or no read at all, keeps it off.
  const tooOldToAdopt = read.state !== "now" || blocksIrreversibleAction(stateOfRead(read)!);
  const name = nameOf(target);
  const unmanaged = ofRead(read, (d) => d.unmanaged) ?? [];
  // An account that is assigned fades where it was and its gap closes after (D-1 M5).
  // The account just assigned keeps its row until the outcome under it has been read.
  const stay = useMemo(() => new Set(result ? [result.username] : []), [result]);
  const leaving = useLeavingRows(
    sortByName(unmanaged, (a) => a.username, (a) => a.username),
    (a) => a.username,
    !inventory.isLoading && !inventory.error,
    { stay },
  );

  return (
    <Card stale={stale || read.state === "kept"} {...leaving.listProps}>
      <CardHeader
        title="Not created by Syndra"
        count={ofRead(read, () => leaving.rows.length)}
        caption={`Accounts on ${name} that Syndra did not make: root, service accounts, anything made by hand. Syndra leaves them alone unless you assign one to a person.`}
      />
      {/* A read with nothing to give has no freshness: the region's line above the cards says so, once. */}
      {!isNotRead(read) && (
        <div className="px-5 pb-2">
          <ReadFreshness
            subject="The account list"
            read={read}
            truncated={ofRead(read, (d) => d.truncated)}
            onRefresh={() => {
              setResult(null);
              return again.press();
            }}
            refreshing={again.pending}
          />
        </div>
      )}
      <ReadAgainError error={again.error} />
      <ListStates
        read={read}
        error={inventory.error}
        isEmpty={leaving.rows.length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nothing outside Syndra."
            guidance={`Every account on ${name} was created by Syndra.`}
          />
        }
      >
        <>
          {leaving.rows.map((account, i) => {
            const step = account.standing && rowStep(account.standing, name, "operator");
            return (
            <LeavingRow key={account.username} {...leaving.props(account.username)}>
            <CardRow
              first={i === 0}
              seat="default"
              action={
                account.self ? undefined : (
                  // Off while the read is too old, with its reason said once for
                  // the card below, never under every button (A-13 §1.5).
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={tooOldToAdopt}
                    onClick={() => setAdopting(account.username)}
                  >
                    Assign to a person
                  </Button>
                )
              }
              // Syndra's own credential. Listed, because it IS on the target
              // and hiding it would leave an operator wondering where it
              // went — but never offered: adopting it hands Syndra's own
              // access to a member, and purging it deletes the credential
              // Syndra reaches this target with. The add-on refuses both; this
              // says so before anybody meets the refusal.
              //
              // The standing's next step is a sentence, so it is the detail line, as on "Accounts Syndra
              // manages": the status column holds the state and its short note only (D-1 §5 #39). A
              // healthy row with nothing to do has no step, so no detail line (D-1 Q5).
              detail={
                (step || account.self) && (
                  <>
                    {step && (
                      <span className="block" data-standing-step>
                        {step}
                      </span>
                    )}
                    {account.self && (
                      <span className="block">{`Syndra\u2019s own account on ${name}, which cannot be assigned to anyone`}</span>
                    )}
                  </>
                )
              }
              // Under the row it is about, never at the foot of the list. Always passed (a fragment), so
              // the outcome arriving never changes the row's shape and remounts the control just pressed.
              expanded={result?.username === account.username}
              disclosure={
                <>
                  {result?.username === account.username && (
                    <AdoptionOutcome
                      result={result.result}
                      username={result.username}
                      person={result.person}
                    />
                  )}
                </>
              }
            >
              {/* Columns, as in "Accounts Syndra manages": the account flexes, the status has its seat
                  (`--seat-status`), so the standing starts at one x whatever the username's length. A
                  share held to a minimum, never a basis, as there: a 200px basis made the line 688px,
                  and at 1024 (663px) "Assign to a person" dropped under a three-letter name that fitted
                  (layout check, staging, 20 Nov 2026). */}
              <span className="min-w-[min(120px,100%)] flex-1 [overflow-wrap:anywhere] font-mono text-small">{account.username}</span>
              <span className="w-(--seat-status) flex-none" data-standing-seat>
                {account.standing && <StandingLine standing={account.standing} system={name} audience="operator" step={false} />}
              </span>
            </CardRow>
            </LeavingRow>
            );
          })}
          {tooOldToAdopt && unmanaged.some((a) => !a.self) && (
            <div className="row-divider px-5 py-3 text-small leading-[1.5] text-faint [text-wrap:pretty]">
              Assigning waits for a current read of {name}: it ties an account and its files to a
              person, and there is no undo.
            </div>
          )}
          {adopting && (
            <AdoptPanel
              username={adopting}
              target={target}
              accounts={ofRead(read, (d) => d.accounts) ?? []}
              pending={adopt.isPending}
              error={adopt.error}
              result={result?.username === adopting ? result : null}
              onCancel={() => setAdopting(null)}
              onAdopt={(subjectId, person) => {
                // One slot: the press clears the old result before it writes (G23).
                setResult(null);
                adopt.reset();
                adopt.mutate(
                  { username: adopting, subjectId, accountId: unmanaged.find((a) => a.username === adopting)?.account_id },
                  { onSuccess: (res) => setResult({ username: adopting, person, result: res }) },
                );
              }}
            />
          )}
        </>
      </ListStates>
    </Card>
  );
}

/**
 * Adopting one account. Rung 3, because there is no undo.
 *
 * The wrong choice hands a member somebody else's home directory, their shares
 * and their group memberships, and the next convergence makes that look
 * intended. Typing the account name is the same gesture as revoking access from a
 * named person, deliberately — the two are equally unrecoverable.
 */
function AdoptPanel({
  username,
  target,
  accounts,
  pending,
  error,
  result,
  onAdopt,
  onCancel,
}: {
  username: string;
  target: string;
  /** Who already has an account here: they can't be given a second. */
  accounts: BoundAccount[];
  pending: boolean;
  error: unknown;
  /** Once set, the sheet is its result: Close is all that is left. */
  result: { username: string; person: string; result: AdoptionResult } | null;
  onAdopt: (subjectId: string, person: string) => void;
  onCancel: () => void;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const [q, setQ] = useState("");
  const [chosen, setChosen] = useState<{ id: string; name: string } | null>(null);
  const people = useUsers(q.trim());
  const confirm = useTypedConfirmation(username);
  const has = new Map(accounts.map((a) => [a.subject_id, a.username]));
  const matches = q.trim() ? (people.data ?? []).slice(0, 8) : [];
  const first = chosen?.name.split(" ")[0];

  return (
    <Modal open onClose={onCancel} size="md" labelledBy="adopt-title" busy={pending}>
      <ModalHeader
        title={`Assign ${username} to a person`}
        titleId="adopt-title"
        lede={result ? undefined : `${username} becomes the account of the person you choose. Everything it already holds on ${name} is theirs from that moment. There is no undo, and none that gives the data back.`}
      />
      {result ? (
        <div className="grid gap-3 px-6">
          <ActionOutcome {...adoptionOutcome(result.result, result.username, result.person)} />
        </div>
      ) : (
      <form
        id="adopt-form"
        className="flex flex-col gap-4 px-6"
        onSubmit={(e) => {
          e.preventDefault();
          if (chosen) onAdopt(chosen.id, chosen.name);
        }}
      >
        <div className="flex flex-col gap-2">
          <FieldLabel htmlFor="adopt-q">Person</FieldLabel>
          <Input
            type="search"
            id="adopt-q"
            value={q}
            placeholder="Search by name or email"
            aria-label="Search people by name or email"
            onChange={(e) => {
              setQ(e.target.value);
              setChosen(null);
            }}
          />
          {matches.length > 0 && (
            <div role="radiogroup" aria-label={`People matching ${q.trim()}`} className="flex flex-col gap-1.5">
              {matches.map(({ user }) => {
                const already = has.get(user.id);
                const sub = personLine(user);
                return (
                  <ChoiceRow
                    key={user.id}
                    single
                    name="adopt-person"
                    selected={chosen?.id === user.id}
                    onToggle={() => setChosen({ id: user.id, name: user.name || user.email })}
                    blocked={Boolean(already)}
                    blockedReason={already ? `Already has ${already} on ${name}` : undefined}
                    title={
                      <span className="inline-flex items-center gap-2.5">
                        <Avatar name={user.name} />
                        {user.name || user.email}
                      </span>
                    }
                    description={
                      <>
                        {sub && <span className="block">{sub}</span>}
                        <span className="block">{user.email}</span>
                      </>
                    }
                  />
                );
              })}
            </div>
          )}
          {q.trim() && !people.isLoading && matches.length === 0 && (
            <FieldHint>Nobody matches {q.trim()}.</FieldHint>
          )}
        </div>

        {chosen && (
          <div className="flex flex-col rounded-inner border border-line-strong">
            <div className="flex gap-3.5 px-4 py-3">
              <span className="w-[72px] flex-none text-fine font-semibold text-label">Changes</span>
              <span className="text-body leading-[1.5]">
                {username} belongs to <strong className="font-semibold">{chosen.name}</strong>, for good.
              </span>
            </div>
            <div className="row-divider flex gap-3.5 px-4 py-3">
              <span className="w-[72px] flex-none text-fine font-semibold text-label">Stays</span>
              <span className="text-body leading-[1.5] text-muted [text-wrap:pretty]">
                {ADOPT_STAYS} The next time Syndra brings
                accounts in line, it adds what {first}&apos;s roles give them on top.
              </span>
            </div>
          </div>
        )}

        <ConfirmByTyping expected={username} noun="account name" value={confirm.typed} onChange={confirm.setTyped} />
        {Boolean(error) && <ActionOutcome word={notDoneWord(error, "Not assigned")} outcome={outcomeFromError(error)} />}
      </form>
      )}
      {result ? (
        <ModalFooter>
          <Button variant="outline" onClick={onCancel}>
            Close
          </Button>
        </ModalFooter>
      ) : (
      <ModalFooter cancel={{ onClick: onCancel }}>
        {/* Violet, not red: it gives an account to someone, and red is kept
            for taking access away. Rung 3 is the typed name, and the lede
            says there is no undo. */}
        <Button
          type="submit"
          form="adopt-form"
          variant="accent"
          isPending={pending}
          disabled={!chosen || !confirm.armed || pending}
          reason={
            !pending && !chosen
              ? "Choose the person first."
              : !pending && !confirm.armed
                ? `Type ${username} to confirm.`
                : undefined
          }
        >
          {chosen ? `Assign ${username} to ${chosen.name}` : `Assign ${username}`}
        </Button>
      </ModalFooter>
      )}
    </Modal>
  );
}

/**
 * What the adoption actually did. `unconfirmed` (202, design §13.4) is the one
 * that matters: sent, the answer lost, it may have been applied, and Syndra is
 * checking; it wears the one unconfirmed outcome every write wears. A send
 * where nothing went out is a 503, said where the error is.
 */
function adoptionOutcome(result: AdoptionResult, username: string, person: string) {
  if (isUnconfirmed(result)) return { word: UNCONFIRMED_WORD, outcome: unconfirmedOutcome() };
  // The backend's own `detail` never names the person (it is written for the
  // audit trail, not the row), so the sentence names them here.
  const said = [`${username} is now ${person}'s account.`, result.detail ?? "", result.warning ?? ""]
    .filter(Boolean)
    .map(sentence)
    .join(" ");
  return { word: "Assigned", outcome: { kind: "applied", message: said } as const };
}

/** The row's copy of the result. It leaves on the next read of the list (G26), never on a button. */
function AdoptionOutcome({
  result,
  username,
  person,
}: {
  result: AdoptionResult;
  username: string;
  person: string;
}) {
  return <ActionOutcome placement="inline" {...adoptionOutcome(result, username, person)} />;
}

/**
 * Stopping the add-on writing, without a redeploy (15.6).
 *
 * The explanation above the buttons is the whole of how an operator picks the
 * right one, so it carries the weight rather than the labels: `draining` and
 * `read_only` differ in exactly one way, and it is the one that matters during a
 * credential rotation. The reason is mandatory because the person who reads it is
 * not the person who set it.
 */
/** The lifecycle states in the words the buttons use, so a note and a button never disagree. */
function stateLabel(state: string): string {
  if (state === "draining") return "finishing up";
  if (state === "read_only") return "read-only";
  return state;
}

function LifecycleControl({
  target,
  health,
  isLoading,
}: {
  target: string;
  health: TargetHealth | undefined;
  isLoading: boolean;
}) {
  const nameOf = useSystemNames();
  const set = useSetLifecycle(target);
  // Sent, the answer lost (202): it may have been applied; the state shown is the last one read.
  const unsure = isUnconfirmed(set.data);
  const [reason, setReason] = useState("");
  // Which action row is asking why. One at a time, so a reason typed for one
  // state is never sent for another.
  const [asking, setAsking] = useState<string | null>(null);
  const name = nameOf(target);
  // The state is whatever health says it is. Before health has answered there is no state to
  // call Active, and no Drain or Read-only to offer from it: the card keeps its seat and says so.
  if (!health?.lifecycle) {
    return (
      <Card id="maintenance">
        <CardHeader title="Maintenance" />
        {isLoading ? (
          <RowSkeleton rows={2} label="Reading the maintenance state" />
        ) : (
          <div className="px-5 pb-3.5 text-body text-muted">
            Not read just now. {name}&rsquo;s maintenance state is unknown, so nothing is offered to change it.
          </div>
        )}
      </Card>
    );
  }
  const current = health.lifecycle;

  // The states you can put it in instead, in the order the board draws them:
  // Make active first when it is not, since that is the way back.
  const ACTIONS: Array<{ id: string; verb: string; head: string; more: string }> = [
    {
      id: "active",
      verb: "Make active",
      head: "Takes reads and changes again.",
      more: `Use it once the work on the key or on ${name} itself is done.`,
    },
    {
      id: "draining",
      verb: "Start draining",
      head: "Lets calls already sent finish, and refuses new ones.",
      more: `Use it before replacing ${name}'s key or restarting it: once nothing is in flight, nothing can be cut off halfway.`,
    },
    {
      id: "read_only",
      verb: "Set read-only",
      head: "Keeps answering reads and refuses every change.",
      more: `This page stays current. Use it while you work on ${name} itself; members setting a password are told to try later.`,
    },
  ].filter((a) => a.id !== current);

  return (
    <Card id="maintenance">
      <CardHeader title="Maintenance" />
      <CardRow first seat="narrow">
        <span className="inline-flex items-center gap-2 text-body font-semibold">
          <StatusDot tone={current === "active" ? "healthy" : "accent"} />
          {current === "active" ? "Active" : stateLabel(current).replace(/^./, (c) => c.toUpperCase())}
        </span>
        <span className="text-body text-muted">
          {current === "active"
            ? `${name} takes reads and changes. Two states you can put it in instead:`
            : `${
                current === "draining"
                  ? "Refusing new changes; finishing what's already sent."
                  : "Refusing every change. Reads keep working."
              } ${lifecycleWhy(health?.lifecycle_note)}`}
        </span>
      </CardRow>

      {ACTIONS.map((action) => {
        const open = asking === action.id;
        return (
          <CardRow
            key={action.id}
            top
            seat="narrow"
            // The reason form opens under the row (its panel), so the verb, now Cancel, stays where it was pressed. The
            // panel is always passed (a fragment), so opening it never changes the row's shape and remounts the button
            // that has focus; only the open row holds the form, so its field's id is never on the page twice.
            expanded={open}
            disclosure={
              <>
                {open && (
                  <form
                    className="grid w-full gap-2"
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (!reason || set.isPending) return;
                      set.reset();
                      set.mutate(
                        { state: action.id, reason },
                        {
                          onSuccess: () => {
                            setReason("");
                            setAsking(null);
                          },
                        },
                      );
                    }}
                  >
                    <FieldLabel htmlFor="lifecycle-reason">Reason</FieldLabel>
                    <Textarea id="lifecycle-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
                    {/* The reason blocks the button for one cause, said once: a
                        state change needs a reason before it will go. */}
                    <FieldHint>
                      {reason
                        ? "Recorded against the change, and shown to the next person who opens this page."
                        : "Say why first. A state change needs a reason before it will go."}
                    </FieldHint>
                    <div>
                      <Button type="submit" variant="accent" size="sm" disabled={!reason || set.isPending}>
                        {action.verb}
                      </Button>
                    </div>
                  </form>
                )}
              </>
            }
            action={
              <Button
                variant="outline"
                size="sm"
                aria-expanded={open}
                onClick={() => {
                  setAsking(open ? null : action.id);
                  setReason("");
                }}
              >
                {open ? "Cancel" : action.verb}
              </Button>
            }
          >
            <span className="min-w-0 flex-[1_1_320px] text-small leading-[1.55] text-muted [text-wrap:pretty]">
              <span className="font-semibold text-ink">{action.head}</span> {action.more}
            </span>
          </CardRow>
        );
      })}

      {/* A lifecycle change that did not land (design B2).

          The refusal and the state are two blocks, in that order, and the
          state still leads with its own dot and the word it always wore.
          The single most expensive misreading here is that the page looks
          like the change took, or like the state is now unknown. It is
          known: it is what it was.

          Amber and not red, because nothing is broken by this and nothing was
          lost — including the typed reason, which stays in the field. A
          mandatory-reason box that empties itself on a network failure
          teaches people to type "asdf" the second time. */}
      {set.error && (
        <CardRow>
          <ActionOutcome
            placement="inline"
            word={notDoneWord(set.error, "Not changed")}
            outcome={
              isNothingSent(set.error)
                ? // Nothing went out: §13.1's why, and its step, as the backend said them.
                  { ...outcomeFromError(set.error), detail: reason ? `The state is still ${stateLabel(current)}. Your reason was kept.` : undefined }
                : {
                    ...outcomeFromError(set.error),
                    message: `${name} did not answer the request. The state is still ${stateLabel(current)}, unchanged.`,
                    detail: reason ? "Your reason was kept. Trying again sends the same request with the same reason." : undefined,
                    next: `Try again. If it keeps failing, check that ${name} is running.`,
                  }
            }
          />
        </CardRow>
      )}
      {unsure && (
        <CardRow>
          <ActionOutcome placement="inline" word={UNCONFIRMED_WORD} outcome={unconfirmedOutcome()} />
        </CardRow>
      )}
    </Card>
  );
}

/** A pass skipped inside the add-on's cooldown, answered from its kept copy: the seconds to wait it out. */
function skipWait(pass: TargetReconcilePass): number | undefined {
  return pass.halted && pass.read.state === "kept" && pass.read.why.reason === REASON.cooldown ? pass.read.why.retryAfter : undefined;
}

/** A pass the cooldown refused with nothing kept (503 `cooldown`): the seconds to wait it out. */
function cooldownOf503(why: Why): number | undefined {
  return why.reason === REASON.cooldown ? why.retryAfter : undefined;
}

/** A write where nothing went out (503 NOTHING_SENT, design §13.4). */
/** The reconcile result in one line. Counts of people, singular and plural both right. */
export function reconcileLine(bound: number | undefined, queued: number, missing: number | undefined, name: string): string {
  const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
  // A count the pass did not take is never a 0. `stale` is absent only on a truncated read, which
  // cannot tell a missing account from one past the cut, so it says so in the strip's own words.
  return [
    bound === undefined ? null : `${people(bound)} with a managed account`,
    `${queued} ${queued === 1 ? "fix" : "fixes"} waiting for you to send`,
    missing === undefined
      ? `missing accounts not checked: ${FIRST_PART_READ}`
      : `${people(missing)} with an account missing from ${name}`,
  ]
    .filter(Boolean)
    .join(" · ");
}
