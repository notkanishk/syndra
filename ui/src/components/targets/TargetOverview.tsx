"use client";

import { describeFailure, sentence } from "@/lib/outcome";
import { Radio } from "@/components/ui/Choice";
import { roleCount } from "@/lib/role-mapping-plan";
import { Fragment, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { MergeFindings } from "@/components/targets/MergeFindings";
import { Region } from "@/components/targets/Region";
import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { Chip, Mono, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { blocksIrreversibleAction, ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { CheckPips, targetReading } from "@/components/targets/TargetReading";
import { ProjectSet, ProjectStep, SetupStrip } from "@/components/targets/ProjectSetup";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { Term } from "@/components/ui/Term";
import { UserName } from "@/components/names";
import { formatBytes, personLine, versionLabel } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useMappings } from "@/lib/queries/useMappings";
import { useTargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";
import {
  useAdoptAccount,
  useMergeFindings,
  useReconcileTarget,
  useReleaseBinding,
  useSetLifecycle,
  useTargetHealth,
  useResolveBindingConflict,
  useResolveLogFinding,
  useTargetInventory,
  useTargets,
  type AdoptionResult,
  type BindingConflict,
  type LogAnchor,
  type TargetHealth,
  type BoundAccount,
  type TargetSummary,
} from "@/lib/queries/useTargets";
import { useUsers } from "@/lib/queries/useUsers";
import { Avatar } from "@/components/ui/Avatar";

/**
 * One add-on target's operator page (9.20, 1.18/1.19, 15.6; design §21).
 *
 * Four questions in this order, because each answer changes what the next one
 * means: is it healthy, whose accounts are on it, what can it do, and what
 * state did somebody put it in.
 */
export function TargetOverview({ target }: { target: string }) {
  const roster = useTargets();
  const health = useTargetHealth(target);
  const findings = useMergeFindings(target);
  const inventory = useTargetInventory(target);
  const registered = (roster.data ?? []).find((t) => t.target === target);

  const anchorFinding = health.data?.log_anchor?.violation_reason ? 1 : 0;
  const conflicts = health.data?.binding_conflicts ?? [];
  // Everything in region 1: differences reconciliation refused to resolve, a
  // change record edited after it was queued, and two records disagreeing about
  // an account. None of them is a status; each is a piece of work waiting on a
  // human, which is why they are one region and not three places.
  const waiting =
    (findings.data?.length ?? 0) + anchorFinding + conflicts.length;
  const waitingKnown = !findings.isLoading && !findings.error && !health.isLoading && !health.error;
  const name = targetLabel(target);
  // One Zitadel project per system (3a). Unknown until the roster answers;
  // null is "not chosen", and everything that gives access waits for it.
  const project = registered?.project;
  const gated = registered !== undefined && project === null;
  const [justSet, setJustSet] = useState<string | null>(null);

  return (
    <div className="grid gap-5">
      <PageHeader
        title={name}
        lede={
          <>
            Whether Syndra can reach {name} and what it says about itself, who has an account
            there, and anything waiting on you. Syndra{" "}
            <Term name="provision">provisions</Term> an account for every person whose roles
            reach {name}, and keeps it in step.
          </>
        }
        meta={
          health.data?.product ? (
            <Mono>
              {name} {versionLabel(name, health.data.product_version)}
            </Mono>
          ) : undefined
        }
      />

      {/* The project comes first: its step while unchosen, its result for the
          visit it was chosen in, then a thin strip that is the only link to the
          mappings. */}
      {gated && !justSet ? (
        <ProjectStep target={target} onSet={setJustSet} />
      ) : justSet ? (
        <ProjectSet target={target} project={justSet} />
      ) : project ? (
        <SetupStrip target={target} project={project} />
      ) : null}

      {/* One chip per region at the right end of the read line (board, RULES):
          a way to skip a region, never a way to hide one, so a finding cannot
          end up behind a tab nobody selected. Gone while the project is
          unchosen: there is nowhere to go yet. */}
      {!(gated && !justSet) && (
        <RegionChips
          health={health.data}
          name={name}
          waiting={waitingKnown ? waiting : null}
          accounts={inventory.data?.bound}
          runs={registered?.callable ? registered.operations.length : undefined}
        />
      )}

      <div className="grid gap-8">
        {/* Region 0 · whether changes can land.

            Health and maintenance are one question, and they used to sit six
            panels apart. A reachability reading has no meaning on its own: Offline
            while somebody is draining it for a credential rotation is a different
            fact from Offline at 04:00, and an operator who reads the second when
            the first is true walks to the wrong machine.

            Two machines, kept apart (board, A-01): what Syndra can reach, and what
            the system says about itself, each ending by saying which one to look
            at. A fault in either is one Notice above them. */}
        <Region
          id="land"
          title="Whether changes can land"
          lede={`Two machines, kept apart: what Syndra can reach, and what ${name} says about itself. Each reading ends by saying which one to look at.`}
        >
          {health.error && !health.data ? (
            // Syndra's own server failed, not the target: said so, and that
            // nothing was changed. No reading, count or age until it answers.
            <Notice
              tone="danger"
              blocking
              title={`This page for ${name} could not be read.`}
              action={{ label: "Try again", onClick: () => health.refetch() }}
            >
              Syndra&rsquo;s server did not answer when this page asked for it. This says nothing about{" "}
              {name} itself. Nothing was changed.
            </Notice>
          ) : (
            <>
              <ReachNotice
                target={target}
                health={health.data}
                // A deployment-side fault, carried here because that is where an
                // operator looks when a target stops working.
                transportError={
                  registered?.transport_status === "error" ? registered.transport_error : undefined
                }
              />

              {waitingKnown && anchorFinding + conflicts.length > 0 && (
                <p className="text-[13.5px] font-semibold text-danger-text">
                  {anchorFinding + conflicts.length === 1
                    ? "One item under Waiting on you needs a decision before this can be trusted."
                    : `${anchorFinding + conflicts.length} items under Waiting on you need a decision before this can be trusted.`}
                </p>
              )}

              <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))] items-start gap-4">
                <ReachCard
                  target={target}
                  health={health.data}
                  isLoading={health.isLoading}
                  transportError={
                    registered?.transport_status === "error" ? registered.transport_error : undefined
                  }
                />
                <SystemHealth target={target} stale={health.data?.reachable === false} />
              </div>
              <LifecycleControl target={target} health={health.data} isLoading={health.isLoading} />
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
        <div className="grid gap-8">
        {/* Region 1 · second on the page, and not first.

            Three findings on a target that has not answered for forty minutes
            are three findings nobody can act on, and the band is what says so.
            But it comes before people and before capabilities, because it is
            the only content here that is costing somebody access today. */}
        <Region
          id="waiting"
          title="Waiting on you"
          count={waitingKnown ? waiting : null}
          lede={`Things Syndra found and will not settle by itself: an account that differs from what Syndra expects, a change log on ${name} that was altered, or two records disagreeing about whose account it is. Each needs a decision from you.`}
        >
          {health.data?.log_anchor?.violation_reason && (
            <LogFinding anchor={health.data.log_anchor} />
          )}
          {conflicts.map((conflict) => (
            <BindingConflictFinding key={conflict.id} target={target} conflict={conflict} />
          ))}
          <MergeFindings target={target} />
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
          <MappingCensus target={target} project={project?.name} />
          <PeopleOnTarget target={target} />
          <Inventory target={target} />
          <DormantAccounts target={target} />
        </Region>

        {/* Region 3 · what an add-on can do and what actually runs against it
            are the same question asked twice. The manifest lists the operations
            and reconciliation is the thing that calls them on a schedule. */}
        <Region
          id="runs"
          title="What runs here"
          lede={`What Syndra is able to do on ${name}, and the scheduled check that keeps accounts in step.`}
        >
          {registered && <Capabilities registered={registered} />}
          <ReconcileControl target={target} />
        </Region>
        </div>
        </NotOpenYet>
      </div>
    </div>
  );
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
  const report = useTargetSystemHealth(target);
  const data = report.data;
  const name = targetLabel(target);

  // The card keeps its seat while it is asked, with nothing in it. Four calls to
  // the target take a moment, and a card that flashes "no alerts" before the
  // answer arrives has said something false.
  if (report.isLoading) {
    return (
      <Card>
        <CardHeader title={`What ${name} reports`} />
        <RowSkeleton rows={4} avatar={false} label={`Loading what ${name} reports`} />
      </Card>
    );
  }
  if (report.isError || !data) {
    return (
      <Card>
        <CardHeader title={`What ${name} reports`} />
        <Reading tone="neutral" label="Not read">
          Syndra has not read {name}&rsquo;s own report yet, so this is not a report that nothing is wrong.
        </Reading>
      </Card>
    );
  }

  const alerts = (data.alerts ?? []).filter((a) => !a.dismissed);
  const pools = data.pools ?? [];
  // Only the ones that matter to a target Syndra provisions accounts on. `cifs`
  // stopped is the single most likely explanation for "my drive vanished", and
  // it is invisible from every other read the add-on makes.
  const services = (data.services ?? []).filter((s) => s.service === "cifs" || s.service === "nfs");
  const degraded = data.degraded ?? [];
  const allRead = data.readable && degraded.length === 0;

  return (
    // Last known, dimmed under its Notice, while Syndra cannot reach the system
    // (RULES): still readable, never disabled.
    <Card className={stale ? "opacity-[0.62]" : ""}>
      <CardHeader title={`What ${name} reports`} note={allRead ? "All read" : "Partly read"} />
      {!data.readable && (
        <Reading tone="warn" label="Could not be asked">
          {name} did not answer its own health reads, so this is not a report that nothing
          is wrong.{data.detail ? ` ${data.detail}` : ""}
        </Reading>
      )}

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
          label={alertLabel(alert.level)}
        >
          {alert.text}
        </Reading>
      ))}

      {pools.map((pool) => (
        <Reading
          key={pool.name}
          tone={!pool.healthy ? "danger" : pool.warning ? "warn" : "healthy"}
          label={`Storage pool ${pool.name}`}
        >
          {pool.status}
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
            Nobody can open their shares while it is stopped
            {s.enable
              ? `, and it is set to start on boot — so it stopped by itself. Check ${name}.`
              : ", and it is not set to start on boot."}
          </Reading>
        ))}

      {data.readable &&
        degraded.length === 0 &&
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

/** The sharing services by what they do, not by their process name. */
function serviceName(service: string): string {
  if (service === "cifs") return "File sharing (SMB)";
  if (service === "nfs") return "NFS sharing";
  return service;
}

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
function ReconcileControl({ target }: { target: string }) {
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
  const result = run.data;
  const name = targetLabel(target);

  return (
    <Card>
      <CardHeader
        title="Check for differences"
        note={`Compares ${name} with what Syndra expects and records any fixes as changes waiting for you to send. Nothing is applied until somebody sends them from Pending changes.`}
      />
      <CardRow>
        <div className="flex-1 text-[14.5px] text-muted">
          {!result ? (
            "Syndra checks by itself every six hours."
          ) : result.halted ? (
            <span className="text-warn-text">
              The check stopped before it concluded anything
              {result.reason ? ` — ${result.reason}` : ""}. Nothing was learned about {name}; try
              again once it is reachable.
            </span>
          ) : (
            `${result.bound} accounts managed · ${result.queued} fixes waiting for you to send · ${result.stale?.length ?? 0} accounts missing from ${name}`
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          isPending={run.isPending}
          onClick={() => run.mutate()}
        >
          Check now
        </Button>
      </CardRow>

      {run.error && (
        <CardRow>
          <span className="text-[13.5px] text-danger-text">
            {sentence(describeFailure(run.error))}
          </span>
        </CardRow>
      )}

      {result && !result.current && (
        // A pass that concluded nothing must not read as a clean one.
        <CardRow>
          <span className="text-[13.5px] text-warn-text">
            Nothing was concluded: {result.reason || `${name} could not be read`}.
          </span>
        </CardRow>
      )}

      {(result?.stale?.length ?? 0) > 0 && (
        <>
          <CardRow>
            <span className="text-[14px] font-semibold text-warn-text">
              {result!.stale!.length === 1
                ? "One person is"
                : `${result!.stale!.length} people are`}{" "}
              recorded as owning an account that is no longer on {name}
            </span>
          </CardRow>
          {result!.stale!.map((b) => (
            <CardRow key={b.subject_id} className="flex-wrap">
              <Mono className="min-w-0 [overflow-wrap:anywhere]">{b.username}</Mono>
              <span className="flex-1" />
              {released.includes(b.subject_id) ? (
                <span className="text-[13px] text-faint">
                  Stopped tracking it. Nothing on {name} was changed.
                </span>
              ) : confirming === b.subject_id ? (
                <>
                  {/* What it does, in the sentence next to the button that does
                      it. The word "forget" is doing real work here: an operator
                      reading it as "delete" is the one misreading this row can
                      afford least, given the account it names is already gone. */}
                  <span className="text-[13px] text-muted">
                    Syndra stops managing <Mono>{b.username}</Mono>. Nothing is deleted, and
                    it can be tracked again by assigning the account to a person if it comes
                    back.
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    isPending={release.isPending}
                    onClick={() =>
                      release.mutate(b.subject_id, {
                        onSuccess: (res) => {
                          // Released, and nothing left over. Anything else keeps
                          // the row and its control, and says what the backend
                          // said — including the sentence naming the repair.
                          if (res.status === "released" && !res.warning) {
                            setReleased((prev) => [...prev, b.subject_id]);
                            setConfirming(null);
                            return;
                          }
                          setUnfinished((prev) => ({
                            ...prev,
                            [b.subject_id]:
                              res.warning ||
                              res.detail ||
                              `${name} did not confirm. Nothing was changed here.`,
                          }));
                        },
                      })
                    }
                  >
                    Stop tracking {b.username}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setConfirming(null)}>
                    Cancel
                  </Button>
                  {unfinished[b.subject_id] && (
                    <span className="w-full text-[13px] text-warn-text">
                      {unfinished[b.subject_id]}
                    </span>
                  )}
                </>
              ) : (
                <>
                  <span className="text-[13px] text-faint">
                    Not fixed by itself: recreating it would bring back an account somebody
                    deleted, so this is yours to decide.
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setConfirming(b.subject_id)}
                  >
                    Stop tracking this account
                  </Button>
                </>
              )}
            </CardRow>
          ))}
          {release.error && (
            <CardRow>
              <span className="text-[13.5px] text-danger-text">
                {sentence(describeFailure(release.error))}
              </span>
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
  transportError,
}: {
  target: string;
  health: TargetHealth | undefined;
  transportError?: string;
}) {
  const name = targetLabel(target);

  if (transportError || health?.status === "cannot_connect") {
    return (
      <Notice tone="danger" blocking title="Cannot connect.">
        Syndra cannot read the key it uses to talk to {name}, so it cannot make any request. This is
        a problem on <strong className="font-semibold">the Syndra server</strong>, not on {name}
        {transportError ? `: ${transportError}` : "."}
      </Notice>
    );
  }

  // The monitor's confirmed answer leads. One failed live read is a missed
  // check, not an outage; three in a row are offline (A-01).
  if (health?.status === "offline" || health?.status === "waiting_to_retry") {
    const offline = health.status === "offline";
    const reading = targetReading({ status: health.status, health: health.health, name });
    return (
      <Notice
        tone="warn"
        live={offline}
        title={offline ? `${name} isn't answering.` : `Syndra is waiting before it calls ${name} again.`}
      >
        {reading.sentence} {offline && reading.look}
        {!health.reachable && <LastKnown name={name} readAt={health.snapshot_taken_at} />}
      </Notice>
    );
  }

  return null;
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
}: {
  target: string;
  health: TargetHealth | undefined;
  isLoading: boolean;
  transportError?: string;
}) {
  const name = targetLabel(target);

  // Nothing read yet: no word, no count, no age.
  if (isLoading && !health) {
    return (
      <Card>
        <CardHeader title="Syndra's reach" />
        <RowSkeleton rows={3} avatar={false} label={`Loading whether changes can land`} />
      </Card>
    );
  }

  const cannot = Boolean(transportError) || health?.status === "cannot_connect";
  const status = health?.status;
  const reading = targetReading({
    status: cannot ? "cannot_connect" : status,
    health: health?.health,
    name,
    transportError,
  });
  const recent = health?.health?.recent ?? [];
  const lifecycle = health?.lifecycle ?? "active";
  const paused = lifecycle !== "active";
  // The add-on's own breaker toward the system, while Syndra's checks still
  // answer: the connector on the Syndra server is holding off, and says so.
  const holdingOff = health?.circuit_open;
  const fault = cannot || status === "offline" || status === "waiting_to_retry";

  // A fault's sentence, and where to look, are the Notice above; its row is
  // the word, the pips and the one faint line.
  let detail: React.ReactNode = null;
  if (!fault) {
    detail =
      health && status !== "checking"
        ? holdingOff
          ? `The add-on on the Syndra server is waiting before it calls ${name} again, after repeated failures, and tries again by itself. Changes for ${name} wait, and nothing is lost.`
          : paused
            ? `Syndra can reach ${name}, but it is ${stateLabel(lifecycle)} — somebody set that on purpose${
                health.lifecycle_note ? `: ${health.lifecycle_note}.` : "."
              } ${
                lifecycle === "draining"
                  ? "New changes are refused and the ones already sent are being allowed to finish."
                  : "Every change is refused at once. Reads keep working."
              }`
            : (reading.sentence ??
              (health.in_flight ? `Accepting changes. ${health.in_flight} in flight.` : "Accepting changes. Nothing is in flight."))
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
        label={
          <>
            {reading.word}
            {reading.pips && <CheckPips recent={recent} />}
            {faint && <span className="text-[13px] font-normal text-faint">{faint}</span>}
          </>
        }
      >
        {detail}
      </Reading>
      <ConnectionNotices target={target} health={health} />
      {/* A missed check is not an outage, but this read did fail: what is on
          screen is the last copy, and says so. */}
      {health && !health.reachable && !fault && (
        <div className="row-divider px-5 py-3">
          <LastKnown name={name} readAt={health.snapshot_taken_at} />
        </div>
      )}
    </Card>
  );
}

function LastKnown({ name, readAt }: { name: string; readAt?: string }) {
  return (
    <div className="mt-2">
      <ReadFreshness subject={`${name}'s last known state`} state={{ readAt, current: false }} />
    </div>
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
}: {
  target: string;
  health: TargetHealth | undefined;
}) {
  const name = targetLabel(target);
  if (!health?.reachable) return null;

  const notices: React.ReactNode[] = [];

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
      <Reading key="unaudited" tone="warn" label="SMB auditing is off">
        {health.unaudited_shares!.length === 1 ? "Share" : "Shares"}{" "}
        {health.unaudited_shares!.map((s, i, all) => (
          <Fragment key={s}>
            {i > 0 && (i === all.length - 1 ? " and " : ", ")}
            <Mono>{s}</Mono>
          </Fragment>
        ))}{" "}
        {health.unaudited_shares!.length === 1 ? "has" : "have"} auditing disabled, so a
        member&rsquo;s activity report comes back empty whether or not they used it. Enable it
        per share in {name}: Shares → SMB → Edit → Advanced.
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
 * The way to each region (board, RULES): one chip per section at the right end
 * of the read line, whose left says how current the reading is. A jump rather
 * than a filter: everything stays present, so a finding cannot end up behind a
 * chip nobody pressed. On a phone the chips wrap; there is no separate index.
 *
 * A count that has not been read is left off. Every region keeps its chip, at
 * zero too: structure never moves in response to data.
 */
function RegionChips({
  health,
  name,
  waiting,
  accounts,
  runs,
}: {
  health: TargetHealth | undefined;
  name: string;
  waiting: number | null;
  accounts?: number;
  runs?: number;
}) {
  const chips: Array<{ id: string; label: string; count?: number | null; dot?: boolean }> = [
    { id: "waiting", label: "Waiting on you", count: waiting, dot: !!waiting },
    { id: "accounts", label: "Accounts", count: accounts },
    { id: "runs", label: "What runs here", count: runs },
  ];
  const checkedAt = health?.health?.checked_at;

  return (
    <div className="-mt-2 flex flex-wrap items-center gap-x-3.5 gap-y-2 pl-5">
      <div className="min-w-[260px] flex-1">
        {checkedAt && <ReadFreshness subject={name} state={{ readAt: checkedAt, current: health?.reachable }} />}
      </div>
      <nav aria-label="Regions of this page" className="ml-auto flex flex-wrap gap-2">
        {chips.map((chip) => (
          <a key={chip.id} href={`#${chip.id}`} className="rounded-pill text-ink no-underline">
            <Chip className="gap-2">
              {chip.dot && <StatusDot tone="warn" />}
              {chip.label}
              {chip.count != null && <span className="text-faint">{chip.count}</span>}
              <span aria-hidden className="text-[12px] text-faint">
                ↓
              </span>
            </Chip>
          </a>
        ))}
      </nav>
    </div>
  );
}

/**
 * What the add-on can perform, read from its manifest.
 */
/** What each operation means, for the ids the connection is known to report. */
const OPERATION_LABEL: Record<string, string> = {
  "account.provision": "create an account for a person",
  "account.converge": "bring an account in line with the person's roles",
  "account.release": "stop managing an account",
  "account.adopt": "assign an existing account to a person",
  "account.purge": "delete accounts",
  "account.list": "read the list of accounts",
  "health.get": "read the system's own health report",
  "activity.get": "read what happened on the system for one person",
  "password.rotate": "replace a person's password on the system",
  "storage.status": "read space used and quota",
};

function Capabilities({ registered }: { registered: TargetSummary }) {
  const name = targetLabel(registered.target);
  return (
    <Card>
      <CardHeader
        title="What it can do"
        note={`As reported by the connection to ${name}`}
      />
      {!registered.callable ? (
        <div className="px-5 pb-5">
          <p className="text-[14px] text-muted">
            Configured, but {name} has not answered yet. Until it does, Syndra does not know
            what it can do here.
          </p>
        </div>
      ) : (
        registered.operations.map((op, i) => (
          <CardRow key={op.id} first={i === 0} className="flex-wrap">
            <span className="font-mono text-[13.5px]">{op.id}</span>
            {OPERATION_LABEL[op.id] && (
              <span className="text-[13.5px] text-muted">{OPERATION_LABEL[op.id]}</span>
            )}
            <span className="text-[13px] text-faint">{op.scope}</span>
            {/* Board §21 draws this beside `account.adopt` and `account.purge`,
                and the page had been dropping it — the manifest says which
                operations stop and ask, and this list is the only place an
                operator can learn that before pressing one. */}
            {op.confirm && <span className="text-[13px] text-faint">confirmation required</span>}
            {op.secret_params && op.secret_params.length > 0 && (
              // Named, never valued. There is nowhere in this payload for a
              // secret and nowhere on this page to render one.
              <span className="text-[12.5px] text-faint">
                never logged: {op.secret_params.join(", ")}
              </span>
            )}
            <span className="flex-1" />
            {!op.available && (
              // Shown disabled with its reason rather than omitted: omitted, an
              // operator wonders whether the feature exists.
              <span className="text-[13px] text-warn-text">
                unavailable — {op.unavailable_reason}
              </span>
            )}
          </CardRow>
        ))
      )}
    </Card>
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
  const [resolving, setResolving] = useState(false);
  const name = targetLabel(anchor.target);
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
        ? "Entries were added, but the log still ends where it did — the new ones do not follow on from anything Syndra has verified."
        : "The number of entries is the same, but their contents changed.";
  return (
    <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
      <p className="text-[13.5px] font-semibold text-danger-text">
        The change log on {name} has been altered
      </p>
      <p className="mt-1 text-[13.5px] text-muted">
        {what} Syndra last saw {anchor.records} {anchor.records === 1 ? "entry" : "entries"}{" "}
        <Relative iso={anchor.anchored_at} />; {name} reported {anchor.violation_records ?? 0}{" "}
        <Relative iso={anchor.violation_at} />.
      </p>
      <p className="mt-1 text-[13.5px] font-semibold text-danger-text">
        If you do not know why the log changed, do not resolve this — ask the person who runs
        the Syndra server.
      </p>
      <p className="mt-1 text-[13px] text-faint">
        This stays until somebody resolves it. Syndra&rsquo;s <Term name="logAnchor">log anchor</Term>{" "}
        is the only thing that can notice entries disappearing, and resolving replaces it with a
        new <Term name="baseline">baseline</Term>.
      </p>
      <details className="mt-1 text-[13px] text-faint">
        <summary className="cursor-pointer">Technical detail</summary>
        <p className="mt-1">
          Syndra&rsquo;s note: {anchor.records} {anchor.records === 1 ? "entry" : "entries"}{" "}
          ending <Mono>{anchor.head.slice(0, 12)}</Mono>, made{" "}
          <Relative iso={anchor.anchored_at} />. {name} reported{" "}
          {anchor.violation_records ?? 0}{" "}
          {anchor.violation_head ? (
            <>
              ending <Mono>{anchor.violation_head.slice(0, 12)}</Mono>{" "}
            </>
          ) : null}
          <Relative iso={anchor.violation_at} />.
        </p>
      </details>
      <div className="mt-3">
        <Button variant="outline" size="sm" onClick={() => setResolving(true)}>
          Resolve this finding
        </Button>
      </div>
      {resolving && (
        <ResolveFindingDialog target={anchor.target} anchor={anchor} onClose={() => setResolving(false)} />
      )}
    </div>
  );
}

/**
 * Two of Syndra's own records disagreeing about who owns an account.
 *
 * Rendered apart from a drain failure, which is where it used to land. "The
 * target refused this call" and "two of your records disagree about who owns an
 * account" read the same in a failure list and want completely different
 * actions — one is retried after fixing the target, and this one is never
 * retried at all.
 *
 * Both claimants are named and neither is called correct, because Syndra does
 * not know. That is the whole content of the finding.
 */
function BindingConflictFinding({
  target,
  conflict,
}: {
  target: string;
  conflict: BindingConflict;
}) {
  const [deciding, setDeciding] = useState(false);
  return (
    <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
      <p className="text-[13.5px] font-semibold text-danger-text">
        Two records disagree about who owns{" "}
        <Mono>{conflict.username}</Mono>
      </p>
      <p className="mt-1 text-[13.5px] text-muted">
        A change for{" "}
        <UserName id={conflict.converged_subject_id} fallback={conflict.converged_subject_id} /> was
        applied to this account, and Syndra records it as belonging to{" "}
        <UserName id={conflict.bound_subject_id} fallback={conflict.bound_subject_id} />. The change
        reached {targetLabel(target)} — what could not be recorded is whose account it is.
      </p>
      <p className="mt-1 text-[13px] text-faint">
        Noticed <Relative iso={conflict.detected_at} />. Nothing else will resolve this: any
        automatic fix for either person would act on whichever record it happens to read.
      </p>
      <div className="mt-3">
        <Button variant="outline" size="sm" onClick={() => setDeciding(true)}>
          Decide who owns it
        </Button>
      </div>
      {deciding && (
        <ResolveConflictDialog
          target={target}
          conflict={conflict}
          onClose={() => setDeciding(false)}
        />
      )}
    </div>
  );
}

/**
 * Choosing between the two claimants.
 *
 * Rung 3, because it takes an account away from somebody who is not in the
 * room. The two names are radio options rather than a free field: an operator
 * assigning it to a third person is not resolving this disagreement, and the
 * backend refuses that — offering a text box would let them try.
 */
function ResolveConflictDialog({
  target,
  conflict,
  onClose,
}: {
  target: string;
  conflict: BindingConflict;
  onClose: () => void;
}) {
  const resolve = useResolveBindingConflict(target);
  const [owner, setOwner] = useState("");
  const [note, setNote] = useState("");
  const confirmation = useTypedConfirmation(conflict.username);
  const ready = owner !== "" && note.trim() !== "" && confirmation.armed && !resolve.isPending;
  const name = targetLabel(target);

  return (
    <Modal
      open
      onClose={resolve.isPending ? () => {} : onClose}
      busy={resolve.isPending}
      size="md"
      labelledBy="resolve-conflict-title"
    >
      <ModalHeader
        titleId="resolve-conflict-title"
        title={`Who owns ${conflict.username}?`}
        lede="Syndra cannot tell. Both of these people are recorded as holding this account, in different places."
      />
      <div className="grid gap-4 px-6">
        <fieldset className="grid gap-2">
          <legend className="mb-1 text-[14px]">The account belongs to</legend>
          {[
            { id: conflict.bound_subject_id, why: "Syndra's ownership record says so." },
            { id: conflict.converged_subject_id, why: "Their account changes were applied to it." },
          ].map((claimant) => (
            <label
              key={claimant.id}
              className="flex cursor-pointer items-start gap-3 rounded-inner border border-line px-3.5 py-3 text-[14px] has-[:checked]:border-accent-line has-[:checked]:bg-accent-soft"
            >
              <Radio
                name="conflict-owner"
                className="mt-1"
                checked={owner === claimant.id}
                onChange={() => setOwner(claimant.id)}
              />
              <span>
                <span className="font-semibold">
                  <UserName id={claimant.id} fallback={claimant.id} />
                </span>
                <span className="block text-[13.5px] text-muted">{claimant.why}</span>
              </span>
            </label>
          ))}
        </fieldset>

        <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
          <p className="text-[13.5px] text-danger-text">
            The other person stops holding this account in Syndra, at once and without being
            told. Their data on {name} is untouched — this changes who Syndra says it belongs
            to, which is what every later action on it goes by. Account fixes for both people
            are recorded as changes waiting for you to send, because the change that caused this
            gave one person the other&rsquo;s settings.
          </p>
        </div>

        <div>
          <FieldLabel htmlFor="conflict-note">How you know</FieldLabel>
          <Input
            id="conflict-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Checked the home directory contents with them"
          />
          <FieldHint>
            The row somebody reads when the other person asks where their account went.
          </FieldHint>
        </div>

        <ConfirmByTyping
          expected={conflict.username}
          value={confirmation.typed}
          onChange={confirmation.setTyped}
          noun="account"
          disabled={resolve.isPending}
        />
        {resolve.error && (
          <p className="text-[13.5px] text-danger-text">
            {sentence(describeFailure(resolve.error))}
          </p>
        )}
      </div>
      <ModalFooter note="Fixes for both people wait in Pending changes. Until they are sent, the account keeps the settings the wrong change gave it.">
        <Button
          variant="dangerConfirm"
          disabled={!ready}
          isPending={resolve.isPending}
          reason={
            !ready && !resolve.isPending
              ? owner === ""
                ? "Choose who the account belongs to."
                : note.trim() === ""
                  ? "Say how you know — this becomes the record."
                  : !confirmation.armed
                    ? `Type ${conflict.username} to confirm.`
                    : undefined
              : undefined
          }
          onClick={() => resolve.mutate({ id: conflict.id, owner, note }, { onSuccess: onClose })}
        >
          Record the owner
        </Button>
        <Button onClick={onClose} disabled={resolve.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
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
  const resolve = useResolveLogFinding(target);
  const [note, setNote] = useState("");
  const confirmation = useTypedConfirmation(target);
  const ready = confirmation.armed && note.trim() !== "" && !resolve.isPending;

  return (
    <Modal open onClose={resolve.isPending ? () => {} : onClose} busy={resolve.isPending} size="md" labelledBy="resolve-finding-title">
      <ModalHeader
        titleId="resolve-finding-title"
        title="Resolve this finding"
        lede={`Syndra will accept the log as ${targetLabel(target)} reports it now, and compare against that from here on.`}
      />
      <div className="grid gap-4 px-6">
        <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
          <p className="text-[13.5px] text-danger-text">
            The records that went missing stay missing, and Syndra stops being able to tell
            you they did. Do this when you know why the log changed — a rebuilt Syndra server,
            a replaced volume — and not to clear a warning. If you do not know, ask the person
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
        {resolve.error && (
          <p className="text-[13.5px] text-danger-text">
            {sentence(describeFailure(resolve.error))}
          </p>
        )}
      </div>
      <ModalFooter note="Syndra takes the log as it is now and compares against that from here on.">
        {/* Not `dangerConfirm`: irreversible, but it takes nothing from
            anybody — the rule this file states itself further down, at the
            account-assignment button. */}
        <Button
          variant="accent"
          disabled={!ready}
          isPending={resolve.isPending}
          reason={
            !ready && !resolve.isPending
              ? note.trim() === ""
                ? "Say why the log changed — this is kept with the resolution."
                : !confirmation.armed
                  ? `Type ${target} to confirm.`
                  : undefined
              : undefined
          }
          onClick={() =>
            resolve.mutate(
              { head: anchor.violation_head ?? "", note },
              { onSuccess: onClose },
            )
          }
        >
          Accept this log and start over
        </Button>
        <Button onClick={onClose} disabled={resolve.isPending}>
          Cancel
        </Button>
      </ModalFooter>
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
  children,
}: {
  tone: StatusTone;
  label: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div className="row-divider flex gap-3 px-5 py-[13px]">
      <StatusDot tone={tone} className="mt-[7px]" />
      <span className="flex min-w-0 flex-col gap-[3px]">
        <span className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[14.5px] font-semibold">{label}</span>
        {children && (
          <span className="text-[13.5px] leading-[1.5] text-muted [text-wrap:pretty]">{children}</span>
        )}
      </span>
    </div>
  );
}

/**
 * How many roles reach this target, and how many people hold them.
 *
 * The census, not the mappings. It carries no reading about health and no count
 * of people bound — those are elsewhere on this page — and its control is
 * outline, because the one violet fill here belongs to Reconcile now.
 *
 * The two sentences are repeated verbatim at the top of the screen they lead
 * to. That is deliberate: an operator who clicked because of a sentence should
 * find that sentence at the top of what they clicked into, or the click feels
 * like it went somewhere else.
 */
function MappingCensus({ target, project }: { target: string; project?: string }) {
  const mappings = useMappings(target);
  const rows = mappings.data ?? [];
  // A role giving a group, a home and a limit is three rows and one role.
  const roles = roleCount(rows);

  return (
    <Card>
      <div className="flex flex-wrap items-start gap-4 px-5 py-4">
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <p className="text-[14.5px]">
            {mappings.isLoading ? (
              <span className="text-faint">Reading which roles are mapped to {targetLabel(target)}…</span>
            ) : rows.length === 0 ? (
              <>
                <strong className="font-semibold">No role is mapped to {targetLabel(target)}</strong>,
                so no role gives anything on it.
              </>
            ) : (
              <>
                <strong className="font-semibold">
                  {roles === 1 ? "One role" : `${roles} roles`}
                  {project ? ` in ${project}` : ""} {roles === 1 ? "reaches" : "reach"}{" "}
                  {targetLabel(target)} through mappings.
                </strong>{" "}
                {/* M6 states how many people hold those roles here. The number
                    is distinct people across every mapped role, and nothing
                    returns it: holders are read per mapping, so counting them
                    here means one request per row and then a union — and two
                    mappings on one role would otherwise be added together and
                    overstate it. Deferred rather than approximated; the rule is
                    that a screen never invents a number. */}
              </>
            )}
          </p>
          <p className="max-w-[78ch] text-[13.5px] leading-[1.55] text-muted">
            Editing one changes access for everybody holding that role. Mappings and their
            published versions have their own screen, linked at the top of this page.
          </p>
        </div>
      </div>
    </Card>
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
function Inventory({ target }: { target: string }) {
  const inventory = useTargetInventory(target);
  const adopt = useAdoptAccount(target);
  const [adopting, setAdopting] = useState<string | null>(null);
  // The outcome is kept WITH the account it happened to. It used to be one
  // unattributed result at the foot of the card, which on a list of eight
  // accounts said "Adopted." about none of them in particular.
  const [result, setResult] = useState<
    { username: string; subjectId: string; result: AdoptionResult } | null
  >(null);

  const read = {
    readAt: inventory.data?.read_at,
    current: inventory.data?.current,
    truncated: inventory.data?.truncated,
  };
  const tooOldToAdopt = !inventory.data || blocksIrreversibleAction(read);
  const name = targetLabel(target);

  return (
    <Card>
      <CardHeader
        title="Not created by Syndra"
        count={inventory.data?.unmanaged?.length}
        note={`Accounts on ${name} that Syndra did not make — root, service accounts, anything made by hand. Syndra leaves them alone unless you assign one to a person.`}
      />
      <div className="px-5 pb-2">
        <ReadFreshness
          subject="The account list"
          state={read}
          onRefresh={() => inventory.refetch()}
          refreshing={inventory.isFetching}
        />
      </div>
      <ListStates
        isLoading={inventory.isLoading}
        error={inventory.error}
        isEmpty={(inventory.data?.unmanaged ?? []).length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nothing outside Syndra"
            guidance={`Every account on ${name} was created by Syndra.`}
          />
        }
      >
        <>
          {(inventory.data?.unmanaged ?? []).map((account, i) => (
            <CardRow
              key={account.username}
              first={i === 0}
              // Under the row it is about, never at the foot of the list.
              expanded={result?.username === account.username}
              disclosure={
                result?.username === account.username ? (
                  <AdoptionOutcome
                    result={result.result}
                    subjectId={result.subjectId}
                    name={name}
                    onDismiss={() => setResult(null)}
                  />
                ) : null
              }
            >
              <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-[13.5px]">{account.username}</span>
              <span className="flex-1" />
              {account.self ? (
                // Syndra's own credential. Listed, because it IS on the target
                // and hiding it would leave an operator wondering where it
                // went — but never offered: adopting it hands Syndra's own
                // access to a member, and purging it deletes the credential
                // Syndra reaches this target with. The add-on refuses both; this
                // says so before anybody meets the refusal.
                <span className="text-[13px] text-faint">
                  Syndra&rsquo;s own account on {name} — cannot be assigned to anyone
                </span>
              ) : tooOldToAdopt ? (
                // The reason as text, never a tooltip. A disabled control whose
                // reason lives in a `title` is a control nobody can find out
                // about on a keyboard or a phone.
                <span className="text-[13px] text-faint">
                  Refresh the list above first — it is too old to act on
                </span>
              ) : (
                <Button variant="outline" size="sm" onClick={() => setAdopting(account.username)}>
                  Assign to a person
                </Button>
              )}
            </CardRow>
          ))}
          {adopting && (
            <AdoptPanel
              username={adopting}
              target={target}
              accounts={inventory.data?.accounts ?? []}
              pending={adopt.isPending}
              error={adopt.error}
              onCancel={() => setAdopting(null)}
              onAdopt={(subjectId) =>
                adopt.mutate(
                  { username: adopting, subjectId },
                  {
                    onSuccess: (res) => {
                      setResult({ username: adopting, subjectId, result: res });
                      setAdopting(null);
                    },
                  },
                )
              }
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
  onAdopt,
  onCancel,
}: {
  username: string;
  target: string;
  /** Who already has an account here: they can't be given a second. */
  accounts: BoundAccount[];
  pending: boolean;
  error: unknown;
  onAdopt: (subjectId: string) => void;
  onCancel: () => void;
}) {
  const name = targetLabel(target);
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
        lede={`${username} becomes the account of the person you choose. Everything it already holds on ${name} is theirs from that moment. There is no undo, and none that gives the data back.`}
      />
      <form
        id="adopt-form"
        className="flex flex-col gap-4 px-6"
        onSubmit={(e) => {
          e.preventDefault();
          if (chosen) onAdopt(chosen.id);
        }}
      >
        <div className="flex flex-col gap-2">
          <FieldLabel htmlFor="adopt-q">Person</FieldLabel>
          <Input
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
                const on = chosen?.id === user.id;
                const sub = personLine(user);
                return (
                  <button
                    key={user.id}
                    type="button"
                    role="radio"
                    aria-checked={on}
                    aria-disabled={Boolean(already)}
                    disabled={Boolean(already)}
                    onClick={() => setChosen({ id: user.id, name: user.name || user.email })}
                    className={`flex min-h-[52px] items-center gap-3 rounded-[12px] border px-3 py-2 text-left ${
                      on ? "border-accent-line bg-accent-soft" : "border-transparent hover-tint"
                    } ${already ? "cursor-default opacity-55" : ""}`}
                  >
                    <span
                      aria-hidden
                      className={`inline-flex h-6 w-6 flex-none items-center justify-center rounded-pill border-[1.5px] ${
                        on ? "border-accent" : "border-line-strong"
                      }`}
                    >
                      <span className={`h-2 w-2 rounded-pill ${on ? "bg-accent" : "bg-transparent"}`} />
                    </span>
                    <Avatar name={user.name} />
                    <span className="flex min-w-0 flex-1 flex-col gap-px [overflow-wrap:anywhere]">
                      <span className="text-[14.5px] font-semibold">{user.name || user.email}</span>
                      {sub && <span className="text-[12.5px] text-muted">{sub}</span>}
                      <span className="text-[12.5px] text-faint">{user.email}</span>
                    </span>
                    {already && (
                      <span className="text-right text-[12.5px] text-faint">
                        Already has {already} on {name}
                      </span>
                    )}
                  </button>
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
              <span className="w-[72px] flex-none text-[12.5px] font-semibold text-label">Changes</span>
              <span className="text-[14px] leading-[1.5]">
                {username} belongs to <strong className="font-semibold">{chosen.name}</strong>, for good.
              </span>
            </div>
            <div className="row-divider flex gap-3.5 px-4 py-3">
              <span className="w-[72px] flex-none text-[12.5px] font-semibold text-label">Stays</span>
              <span className="text-[14px] leading-[1.5] text-muted [text-wrap:pretty]">
                Its home folder, its shares and its group memberships, as they are. The next time Syndra brings
                accounts in line, it adds what {first}&apos;s roles give them on top.
              </span>
            </div>
          </div>
        )}

        <ConfirmByTyping expected={username} noun="account name" value={confirm.typed} onChange={confirm.setTyped} />
        {Boolean(error) && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(error))}</p>}
      </form>
      <ModalFooter>
        <Button type="button" onClick={onCancel}>
          Cancel
        </Button>
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
    </Modal>
  );
}

/**
 * What the adoption actually did — three outcomes, rendered as three.
 *
 * `unconfirmed` is the one that matters: the target did not answer, nothing was
 * recorded here, and the operator must look before trying again. Rendering it as
 * success is what this screen used to do.
 */
function AdoptionOutcome({
  result,
  subjectId,
  name,
  onDismiss,
}: {
  result: AdoptionResult;
  subjectId: string;
  name: string;
  onDismiss: () => void;
}) {
  const unconfirmed = result.status !== "adopted";
  return (
    <div
      role="status"
      className={`flex flex-wrap items-baseline gap-2 text-[13.5px] ${
        unconfirmed ? "text-warn-text" : "text-muted"
      }`}
    >
      <span>
        {/* The backend's own `detail` never names the person — it is written
            for the audit trail, not the row. Named here so the operator sees
            who they just handed the account to, not only that something
            happened. */}
        {!unconfirmed && (
          <>
            Assigned to <UserName id={subjectId} fallback={subjectId} />.{" "}
          </>
        )}
        {result.detail ?? (unconfirmed ? `${name} did not confirm it. Nothing was recorded.` : "Assigned.")}
      </span>
      {result.warning && <span className="text-warn-text">{result.warning}</span>}
      <span className="flex-1" />
      <Button variant="outline" size="sm" onClick={onDismiss}>
        Dismiss this result
      </Button>
    </div>
  );
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
  const set = useSetLifecycle(target);
  const [reason, setReason] = useState("");
  // Which action row is asking why. One at a time, so a reason typed for one
  // state is never sent for another.
  const [asking, setAsking] = useState<string | null>(null);
  const name = targetLabel(target);
  // The state is whatever health says it is. Before health has answered there is no state to
  // call Active, and no Drain or Read-only to offer from it: the card keeps its seat and says so.
  if (!health?.lifecycle) {
    return (
      <Card>
        <CardHeader title="Maintenance" />
        {isLoading ? (
          <RowSkeleton rows={2} label="Reading the maintenance state" />
        ) : (
          <div className="px-5 pb-3.5 text-[14px] text-muted">
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
      more: "Use it once the work on the key or on the NAS itself is done.",
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
      more: `This page stays current. Use it while you work on the NAS itself; members setting a password are told to try later.`,
    },
  ].filter((a) => a.id !== current);

  return (
    <Card>
      <CardHeader title="Maintenance" />
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-5 pb-3.5">
        <span className="inline-flex items-center gap-2 text-[15px] font-semibold">
          <StatusDot tone={current === "active" ? "healthy" : "accent"} />
          {current === "active" ? "Active" : stateLabel(current).replace(/^./, (c) => c.toUpperCase())}
        </span>
        <span className="text-[14px] text-muted">
          {current === "active"
            ? `${name} takes reads and changes. Two states you can put it in instead:`
            : `${
                current === "draining"
                  ? "Refusing new changes; finishing what's already sent."
                  : "Refusing every change. Reads keep working."
              }${health?.lifecycle_note ? ` Set on purpose: ${health.lifecycle_note}.` : ""}`}
        </span>
      </div>

      {ACTIONS.map((action) => {
        const open = asking === action.id;
        return (
          <div key={action.id} className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-2 px-5 py-3.5">
            <span className="w-[150px] flex-none">
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
            </span>
            <span className="min-w-0 flex-[1_1_320px] text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
              <span className="font-semibold text-ink">{action.head}</span> {action.more}
            </span>
            {open && (
              <div className="grid w-full gap-2 pt-1">
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
                  <Button
                    variant="accent"
                    size="sm"
                    disabled={!reason || set.isPending}
                    onClick={() =>
                      set.mutate(
                        { state: action.id, reason },
                        {
                          onSuccess: () => {
                            setReason("");
                            setAsking(null);
                          },
                        },
                      )
                    }
                  >
                    {action.verb}
                  </Button>
                </div>
              </div>
            )}
          </div>
        );
      })}

      <div className="row-divider px-5 py-3 text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
        {current === "active"
          ? "Neither takes anyone's access away. Each asks why, since the next person to read it is not you, and Make active undoes it."
          : "Neither takes anyone's access away. Each asks why, since the next person to read it is not you."}
      </div>

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
        <div className="row-divider px-5 py-3.5">
          <div className="grid gap-2 rounded-inner border border-warn-line bg-warn-soft px-4 py-3">
            <p className="text-[13.5px] font-semibold text-warn-text">
              That did not go through. {name} did not answer the request.
            </p>
            <p className="text-[13.5px] text-muted">
              <span className="font-semibold text-ink">
                The state is still {stateLabel(current)}
              </span>{" "}
              — the one above, unchanged. Nothing about the attempt is recorded as a state,
              and this message is its only trace.
            </p>
            <p className="text-[13px] text-faint">
              This does not affect the &ldquo;Waiting to retry&rdquo; reading above.
            </p>
            {reason && (
              <p className="text-[13px] text-muted">
                Your reason was kept. Trying again sends the same request with the same reason.
              </p>
            )}
          </div>
        </div>
      )}
    </Card>
  );
}
