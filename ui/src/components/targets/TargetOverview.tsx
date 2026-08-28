"use client";

import { Fragment, useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { MergeFindings } from "@/components/targets/MergeFindings";
import { Region } from "@/components/targets/Region";
import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { CountChip, Mono, STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { blocksIrreversibleAction, ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { Term } from "@/components/ui/Term";
import { UserName } from "@/components/names";
import { formatBytes } from "@/lib/format";
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
  type TargetSummary,
} from "@/lib/queries/useTargets";

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
  const registered = (roster.data ?? []).find((t) => t.target === target);

  const anchorFinding = health.data?.log_anchor?.violation_reason ? 1 : 0;
  const conflicts = health.data?.binding_conflicts ?? [];
  // Everything in region 1: differences reconciliation refused to resolve, a
  // change record edited after it was queued, and two records disagreeing about
  // an account. None of them is a status; each is a piece of work waiting on a
  // human, which is why they are one region and not three places.
  const waiting =
    (findings.data?.length ?? 0) + anchorFinding + conflicts.length;
  const waitingKnown = !findings.isLoading && !health.isLoading;
  const name = targetLabel(target);

  return (
    <div className="grid gap-5">
      <PageHeader
        title={name}
        lede={`Whether Syndra can reach ${name} and what it says about itself, who has an account there, and anything waiting on you. Syndra provisions an account on ${name} for every person whose role is mapped to a group there, reconciles it every six hours, and asks you when it finds something it should not decide alone.`}
        meta={registered ? authLabel(registered.auth_mode, name) : undefined}
      />

      <TargetLede target={target} health={health.data} waiting={waiting} known={waitingKnown} />

      {/* The touch form of the four regions: a way to skip one rather than a
          way to hide three. Everything below it is present and scrollable, so a
          finding cannot end up behind a tab nobody selected. */}
      <RegionIndex waiting={waitingKnown ? waiting : null} />

      <div className="grid gap-8">
        {/* Region 0 · the band.

            Health and maintenance are one question, and they used to sit six
            panels apart. A reachability reading has no meaning on its own: NOT
            ANSWERING while somebody is draining it for a credential rotation is
            a different fact from NOT ANSWERING at 04:00, and an operator who
            reads the second when the first is true walks to the wrong machine.

            Two cards side by side and never merged into one word: the left is
            Syndra's ability to reach the target, the right is the target's own
            account of itself. Keeping them apart is what makes "look at Syndra"
            and "look at the NAS" possible to say at all. */}
        <Region
          id="answering"
          title="Is it answering"
          lede={`Whether Syndra can reach ${name}, what ${name} says about itself, and whether somebody has paused changes to it.`}
        >
          <div className="grid gap-4 desktop:grid-cols-2">
            <Health
              target={target}
              health={health.data}
              isLoading={health.isLoading}
              // A deployment-side fault, carried into this card because that is
              // where an operator looks when a target stops working — and it
              // explains the reading below it rather than sitting beside it.
              transportError={
                registered?.transport_status === "error" ? registered.transport_error : undefined
              }
              needsAPerson={waitingKnown ? anchorFinding + conflicts.length : 0}
            />
            <SystemHealth target={target} />
          </div>
          <LifecycleControl target={target} health={health.data} />
        </Region>

        {/* Region 1 · second on the page, and not first.

            Three findings on a target that has not answered for forty minutes
            are three findings nobody can act on, and the band is what says so.
            But it comes before people and before capabilities, because it is
            the only content here that is costing somebody access today. */}
        <Region
          id="waiting"
          title="Waiting on a person"
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
          id="people"
          title="People and their access here"
          lede="Three separate lists: accounts Syndra manages, accounts it did not create, and accounts it created that no role needs any more. Each takes a different action."
        >
          <MappingCensus target={target} />
          <PeopleOnTarget target={target} />
          <Inventory target={target} />
          <DormantAccounts target={target} />
        </Region>

        {/* Region 3 · what an add-on can do and what actually runs against it
            are the same question asked twice. The manifest lists the operations
            and reconciliation is the thing that calls them on a schedule. */}
        <Region
          id="runs"
          title="What Syndra can do here"
          lede={`What Syndra is able to do on ${name}, and the scheduled check that keeps accounts in step.`}
        >
          {registered && <Capabilities registered={registered} />}
          <ReconcileControl target={target} />
        </Region>
      </div>
    </div>
  );
}

/**
 * Health — five readings a single "status" chip would flatten, plus the one
 * finding that is not about health at all.
 *
 * Each reading sends an operator to a different machine, so each is rendered as
 * itself. The tones are load-bearing and follow §21: a maintenance state
 * somebody chose is ACCENT, because amber would read as a fault and send them
 * looking for one; the backend backing off is danger and says so in words,
 * because an operator who reads `circuit_open` as "the target is down" looks at
 * the wrong machine entirely.
 */
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
function SystemHealth({ target }: { target: string }) {
  const report = useTargetSystemHealth(target);
  const data = report.data;
  const name = targetLabel(target);

  // Nothing at all while it is still being asked. Four calls to the target take
  // a moment, and a card that flashes "no alerts" before the answer arrives has
  // said something false.
  if (report.isLoading || report.isError || !data) return null;

  const alerts = (data.alerts ?? []).filter((a) => !a.dismissed);
  const pools = data.pools ?? [];
  // Only the ones that matter to a target Syndra provisions accounts on. `cifs`
  // stopped is the single most likely explanation for "my drive vanished", and
  // it is invisible from every other read the add-on makes.
  const services = (data.services ?? []).filter((s) => s.service === "cifs" || s.service === "nfs");
  const degraded = data.degraded ?? [];

  return (
    <Card>
      <CardHeader
        title={`What ${name} reports about itself`}
        note={
          data.system?.hostname
            ? `${data.system.hostname}${data.system.version ? ` · ${data.system.version}` : ""}`
            : `Read from ${name} just now, not from Syndra's records`
        }
      />
      <div className="flex flex-col gap-2.5 px-5 pb-5">
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
      </div>
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
        note={`Compares ${name} with what Syndra expects and records any fixes as changes waiting to be sent. Nothing is applied until somebody sends them from Pending changes.`}
      />
      <CardRow>
        <div className="flex-1 text-[14.5px] text-muted">
          {result
            ? `${result.bound} accounts managed · ${result.queued} fixes waiting to be sent · ${result.stale?.length ?? 0} accounts missing from ${name}`
            : "Syndra checks by itself every six hours."}
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
            {run.error instanceof Error
              ? run.error.message
              : "The check did not complete. Nothing was changed."}
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
              <Mono>{b.username}</Mono>
              {b.uid ? <span className="text-[13px] text-faint">id {b.uid}</span> : null}
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
                  <Button variant="ghost" size="sm" onClick={() => setConfirming(null)}>
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
                {release.error instanceof Error
                  ? release.error.message
                  : "Could not stop tracking it. Nothing was changed."}
              </span>
            </CardRow>
          )}
        </>
      )}
    </Card>
  );
}

/**
 * What the roster's `auth_mode` means in words.
 *
 * `derived` is the accurate token and an unhelpful thing to render: it names the
 * mechanism, and the operator's question is whether the channel is authenticated
 * at all. `none` never reaches this page — a target with no secret does not
 * register — but it is spelled out rather than left to a fallback, because the
 * one thing this line must never do is read as reassurance when it is not.
 */
function authLabel(mode: string, name: string): string {
  if (mode === "derived") return `Connection to ${name} signed with this deployment's key`;
  if (mode === "none") return `Connection to ${name} NOT secured — no key configured`;
  return `Connection to ${name} secured by ${mode}`;
}

function Health({
  target,
  health,
  isLoading,
  transportError,
  needsAPerson,
}: {
  target: string;
  health: TargetHealth | undefined;
  isLoading: boolean;
  transportError?: string;
  /** How many Syndra-side findings are waiting in the region below. */
  needsAPerson: number;
}) {
  const name = targetLabel(target);
  return (
    <Card>
      <CardHeader title="Can Syndra reach it" />
      <div className="grid gap-3 px-5 pb-5">
        {isLoading && !health && <p className="text-[14px] text-faint">Reading…</p>}

        {/* The two Syndra-side findings used to render HERE, above the
            reachability reading. The priority was right and the placement was
            not: neither is a fact about whether Syndra can reach the machine,
            both stand whether or not it answers, and both wait on a person —
            which is the definition of the region below. A reader who met them
            in a list of readings skimmed them in the same rhythm as
            `in flight: 0`.

            What stays is one red line saying something below needs a person
            before this card can be trusted. */}
        {needsAPerson > 0 && (
          <p className="text-[13.5px] font-semibold text-danger-text">
            {needsAPerson === 1
              ? "One item under Waiting on a person needs a decision before this card can be trusted."
              : `${needsAPerson} items under Waiting on a person need a decision before this card can be trusted.`}
          </p>
        )}

        {/* Above reachability, because it EXPLAINS it. A target whose transport
            secret cannot be read will also not answer, and an operator who
            reads "not answering" first goes to the NAS — which is the wrong
            machine, and the one that takes longest to rule out. */}
        {transportError && (
          <Reading tone="danger" label="Connection key missing">
            Syndra cannot read the key it uses to talk to {name}, so it cannot make any
            request. This is a problem on{" "}
            <strong className="font-semibold">the Syndra server</strong>, not on {name}:{" "}
            {transportError}
          </Reading>
        )}

        {health && !health.reachable && (
          <Reading tone="danger" label="Not answering">
            {health.detail || `${name} did not answer.`} The program that connects Syndra to{" "}
            {name}, on the Syndra server, is the thing to look at.
          </Reading>
        )}

        {health?.circuit_open && (
          <Reading tone="danger" label="Paused after failures">
            Syndra has stopped trying to reach {name} for a while after repeated failures, and
            will try again by itself.{" "}
            <strong className="font-semibold">This does not mean {name} is down</strong> — look
            at the Syndra server.
          </Reading>
        )}

        {health?.reachable && health.lifecycle && health.lifecycle !== "active" && (
          // Accent, never amber. Somebody chose this, and the same choice is
          // accent on the withdrawn-access queue for the same reason.
          <Reading tone="accent" label={health.lifecycle === "draining" ? "Finishing up" : "Read-only"}>
            Set on purpose{health.lifecycle_note ? `: ${health.lifecycle_note}` : ""}.{" "}
            {health.lifecycle === "draining"
              ? "New changes are refused and the ones already sent are being allowed to finish."
              : "Every change is refused at once. Reads keep working."}
          </Reading>
        )}

        {health?.in_flight !== undefined && health.in_flight > 0 && (
          <Reading tone="warn" label="Still finishing">
            {health.in_flight} change{health.in_flight === 1 ? "" : "s"} sent before the pause{" "}
            {health.in_flight === 1 ? "has" : "have"} not completed. Wait for this to reach
            zero before changing the {name} API key.
          </Reading>
        )}

        {/* A credential whose expiry nobody recorded. Not amber for its own
            sake: the key CAN expire without Syndra knowing, and the day it does
            the target simply stops answering — which reads as an outage and
            sends an operator to the NAS. `none` is an operator's deliberate
            choice and says nothing here. */}
        {health?.reachable && health.key_expiry === "unrecorded" && (
          <Reading tone="warn" label="API key expiry not recorded">
            Syndra does not know when the {name} API key (a password for a program rather
            than a person) expires. For whoever runs the Syndra server: if it has an expiry,
            set <Mono>TRUENAS_API_KEY_EXPIRES_AT</Mono> in the deployment&rsquo;s{" "}
            <Mono>.env</Mono> so Syndra warns before it fails; if it never expires, set it to{" "}
            <Mono>never</Mono>.
          </Reading>
        )}

        {/* Auditing off means activity reports are empty, and an empty report
            is indistinguishable from a member who did nothing. Said here so it
            is learned before somebody depends on it. */}
        {health?.reachable && health.shares_readable && (health.unaudited_shares?.length ?? 0) > 0 && (
          <Reading tone="warn" label="SMB auditing is off">
            {health.unaudited_shares!.length === 1 ? "Share" : "Shares"}{" "}
            {health.unaudited_shares!.map((s, i, all) => (
              <Fragment key={s}>
                {/* Separators between, never before the first and never after
                    the last: two names run together into one that names no
                    share at all. */}
                {i > 0 && (i === all.length - 1 ? " and " : ", ")}
                <Mono>{s}</Mono>
              </Fragment>
            ))}{" "}
            {health.unaudited_shares!.length === 1 ? "has" : "have"} auditing disabled, so a
            member&rsquo;s activity report comes back empty whether or not they used it.
            Enable it per share in {name}: Shares → SMB → Edit → Advanced.
          </Reading>
        )}

        {health?.reachable && health.version_tested === false && (
          <Reading tone="warn" label={`Untested ${name} version`}>
            {health.version_note || `This version of ${name} has not been tested with Syndra.`}{" "}
            Reads keep working; changes are refused.
          </Reading>
        )}

        {health?.reachable &&
          (health.lifecycle ?? "active") === "active" &&
          !health.circuit_open &&
          health.version_tested !== false && (
            <Reading tone="healthy" label="Serving">
              {health.product} {health.product_version} · answering, tested, and accepting
              changes.
            </Reading>
          )}

        <dl className="grid gap-2 pt-1 text-[13.5px]">
          {health?.last_read_at && (
            <Line label="Last answered">
              <Relative iso={health.last_read_at} />
            </Line>
          )}
          {health?.key_expires_at && (
            <Line label="API key expires">
              <Relative iso={health.key_expires_at} />
            </Line>
          )}
          {health?.log_head && (
            <Line label="Change log">
              {health.log_records ?? 0} entries ·{" "}
              <span className="font-mono text-[12.5px] text-faint">
                {health.log_head.slice(0, 12)}
              </span>
            </Line>
          )}
        </dl>

        {health?.snapshot_taken_at && (
          <ReadFreshness
            subject={`${name}'s last known state`}
            state={{ readAt: health.snapshot_taken_at, current: health.reachable }}
          />
        )}
      </div>
    </Card>
  );
}

/**
 * The page's one-line answer, under its title.
 *
 * The page difference between a quiet target and one with somebody's access
 * disputed is carried HERE and in each region's lede — by copy in a fixed
 * place, never by a panel appearing. That is what lets the structure stay
 * still while the page still reads differently.
 */
function TargetLede({
  target,
  health,
  waiting,
  known,
}: {
  target: string;
  health?: TargetHealth;
  waiting: number;
  known: boolean;
}) {
  const name = targetLabel(target);
  const reach = !health
    ? `Reading ${name}.`
    : !health.reachable
      ? `${name} is not answering.`
      : health.lifecycle === "draining"
        ? `${name} is finishing up (refusing new changes while the ones already sent finish) — somebody set that on purpose.`
        : health.lifecycle && health.lifecycle !== "active"
          ? `${name} is read-only (refusing every change) — somebody set that on purpose.`
          : `${name}${health.product_version ? ` ${health.product_version}` : ""}, answering and accepting changes.`;

  return (
    <p className="max-w-[86ch] text-[15px] leading-[1.6] text-muted">
      {reach}{" "}
      {!known ? null : waiting === 0 ? (
        <span className="text-ink">Nothing is waiting on a person.</span>
      ) : (
        <span className="font-semibold text-danger-text">
          {waiting === 1 ? "One thing is" : `${waiting} things are`} waiting on a person.
        </span>
      )}
    </p>
  );
}

/**
 * The touch form of the four regions (design T5).
 *
 * The mobile board made this page four horizontally scrolling tabs. What a
 * phone actually needs is a way to SKIP a region, not a way to hide three — a
 * finding behind an unselected tab is a finding nobody is looking at, and the
 * only fix for that is a badge on the tab, which is data driving structure by
 * another route.
 *
 * So: five rows, always the same five, hollow zeros included, each a jump
 * rather than a filter. Everything below stays present and scrollable.
 */
function RegionIndex({ waiting }: { waiting: number | null }) {
  const regions: Array<{ id: string; label: string; count?: number | null }> = [
    { id: "answering", label: "Is it answering" },
    { id: "waiting", label: "Waiting on a person", count: waiting },
    { id: "people", label: "People and their access here" },
    { id: "runs", label: "What Syndra can do here" },
  ];

  return (
    <nav aria-label="Sections of this page" className="desktop:hidden">
      <Card>
        {regions.map((region, i) => (
          <a
            key={region.id}
            href={`#${region.id}`}
            className={`flex min-h-[48px] items-center gap-3 px-5 py-2.5 text-[14.5px] motion-tint hover:bg-[var(--hover)] ${
              i === 0 ? "" : "row-divider"
            }`}
          >
            <span className="flex-1">{region.label}</span>
            {region.count !== undefined && <CountChip n={region.count} />}
          </a>
        ))}
      </Card>
    </nav>
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
  const what =
    anchor.violation_reason === "records_decreased"
      ? "Entries that existed are gone."
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
        This stays until somebody resolves it. Syndra keeps its own note of how long the log
        was, and that note is the only thing that can notice entries disappearing.
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
              <input
                type="radio"
                name="conflict-owner"
                className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
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
            are recorded as changes waiting to be sent, because the change that caused this
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
            {resolve.error instanceof Error
              ? resolve.error.message
              : "That did not go through. Nothing was changed."}
          </p>
        )}
      </div>
      <ModalFooter note="Fixes for both people wait in Pending changes. Until they are sent, the account keeps the settings the wrong change gave it.">
        <Button
          variant="dangerConfirm"
          disabled={!ready}
          onClick={() => resolve.mutate({ id: conflict.id, owner, note }, { onSuccess: onClose })}
        >
          {resolve.isPending ? "Recording…" : "Record the owner"}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={resolve.isPending}>
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
            {resolve.error instanceof Error
              ? resolve.error.message
              : "That did not go through. Nothing was changed."}
          </p>
        )}
      </div>
      <ModalFooter note="Syndra takes the log as it is now and compares against that from here on.">
        <Button
          variant="dangerConfirm"
          disabled={!ready}
          onClick={() =>
            resolve.mutate(
              { head: anchor.violation_head ?? "", note },
              { onSuccess: onClose },
            )
          }
        >
          {resolve.isPending ? "Resolving…" : "Accept this log and start over"}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={resolve.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/** One health reading: a dot that carries the tone, a label, and the sentence. */
function Reading({
  tone,
  label,
  children,
}: {
  tone: StatusTone;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-baseline gap-2.5 text-[14px]">
      <StatusDot tone={tone} className="mt-1.5" />
      <span>
        <span className={`font-semibold ${STATUS_TONE[tone].label}`}>{label}.</span>{" "}
        <span className="text-muted">{children}</span>
      </span>
    </div>
  );
}

function Line({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <dt className="w-40 shrink-0 text-faint">{label}</dt>
      <dd className="text-muted">{children}</dd>
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
function MappingCensus({ target }: { target: string }) {
  const mappings = useMappings(target);
  const rows = mappings.data ?? [];

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
                  {rows.length === 1 ? "One role is mapped to" : `${rows.length} roles are mapped to`}{" "}
                  {targetLabel(target)}.
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
            published versions have their own screen.
          </p>
        </div>
        <ButtonLink href={`/system/targets/${target}/mappings`} size="sm" className="shrink-0">
          {rows.length === 0 ? "Add the first mapping" : "Open mappings"}
        </ButtonLink>
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
  const [result, setResult] = useState<{ username: string; result: AdoptionResult } | null>(null);

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
              expanded={adopting === account.username || result?.username === account.username}
              disclosure={
                adopting === account.username ? (
                  <AdoptPanel
                    username={account.username}
                    target={target}
                    pending={adopt.isPending}
                    error={adopt.error}
                    onCancel={() => setAdopting(null)}
                    onAdopt={(subjectId) =>
                      adopt.mutate(
                        { username: account.username, subjectId },
                        {
                          onSuccess: (res) => {
                            setResult({ username: account.username, result: res });
                            setAdopting(null);
                          },
                        },
                      )
                    }
                  />
                ) : result?.username === account.username ? (
                  <AdoptionOutcome result={result.result} name={name} onDismiss={() => setResult(null)} />
                ) : null
              }
            >
              <span className="font-mono text-[13.5px]">{account.username}</span>
              {account.uid !== undefined && (
                <span className="text-[13px] text-faint">id {account.uid}</span>
              )}
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
  pending,
  error,
  onAdopt,
  onCancel,
}: {
  username: string;
  target: string;
  pending: boolean;
  error: unknown;
  onAdopt: (subjectId: string) => void;
  onCancel: () => void;
}) {
  const [subjectId, setSubjectId] = useState("");
  const confirm = useTypedConfirmation(username);
  const fieldId = `adopt-subject-${username}`;

  return (
    <form
      className="grid gap-3.5 text-[14px]"
      onSubmit={(e) => {
        e.preventDefault();
        onAdopt(subjectId);
      }}
    >
      {/* Two sentences that used to read as a contradiction — "hands over
          everything" directly above "nothing changes" — because the thing that
          moves and the thing that stays were never named apart. What moves is
          who the account BELONGS to. What stays is everything in it.

          "that person" was also a pronoun with nothing in front of it: the
          field naming the person came afterwards, so the first thing an
          operator read referred to something they had not been asked for yet. */}
      <p className="text-muted">
        <Mono className="text-ink">{username}</Mono> becomes the account of the person named
        below. Everything it already holds on {targetLabel(target)} — its home directory, its
        shares, its group memberships — is theirs from that moment.{" "}
        <strong className="font-semibold text-ink">There is no undo</strong>, and none that
        gives the data back.
      </p>
      <p className="text-[13.5px] text-faint">
        Syndra changes nothing in the account now. The next time it brings accounts in line,
        it adds whatever the person&rsquo;s roles give them, on top of what is already there.
      </p>
      <div>
        <FieldLabel htmlFor={fieldId}>Person</FieldLabel>
        <Input id={fieldId} value={subjectId} onChange={(e) => setSubjectId(e.target.value)} />
        {/* The hint is outside the label on purpose: inside it, every control's
            accessible name becomes its title plus a paragraph. */}
        <FieldHint>
          Their Syndra ID — the long code at the end of the address when you open their page
          under People.
        </FieldHint>
      </div>
      <ConfirmByTyping
        expected={username}
        noun="account name"
        value={confirm.typed}
        onChange={confirm.setTyped}
      />
      <div className="flex gap-2">
        {/* Not `dangerConfirm`. A solid red fill is this product's word for a
            click that TAKES ACCESS AWAY, and this one gives an account to
            somebody. Dressing a grant as a destruction spends the red on the
            wrong act, and the next real one reads as routine. What makes this
            irreversible is carried where it belongs: rung 3 above, and the
            sentence that says there is no undo. */}
        <Button
          type="submit"
          variant="accent"
          isPending={pending}
          disabled={!subjectId || !confirm.armed || pending}
        >
          {pending ? "Assigning…" : `Assign ${username} to this person`}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {Boolean(error) && (
        <p className="text-[13.5px] text-danger-text">
          {error instanceof Error
            ? error.message
            : "That did not go through. Nothing was changed."}
        </p>
      )}
    </form>
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
  name,
  onDismiss,
}: {
  result: AdoptionResult;
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
        {result.detail ??
          (unconfirmed ? `${name} did not confirm it. Nothing was recorded.` : "Assigned.")}
      </span>
      {result.warning && <span className="text-warn-text">{result.warning}</span>}
      <span className="flex-1" />
      <Button variant="ghost" size="sm" onClick={onDismiss}>
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

function LifecycleControl({ target, health }: { target: string; health: TargetHealth | undefined }) {
  const set = useSetLifecycle(target);
  const [reason, setReason] = useState("");
  const current = health?.lifecycle ?? "active";
  const name = targetLabel(target);

  const STATES: Array<{ id: string; label: string; blurb: string }> = [
    { id: "active", label: "Active", blurb: "Accept changes normally." },
    {
      id: "draining",
      label: "Finishing up",
      blurb: `Refuse new changes, let the ones already sent finish. This is the safe state for changing the ${name} API key.`,
    },
    { id: "read_only", label: "Read-only", blurb: "Refuse every change at once." },
  ];

  return (
    <Card>
      <CardHeader title="Maintenance" note={`Currently ${stateLabel(current)}`} />
      <div className="grid gap-3 px-5 pb-5">
        <dl className="grid gap-1.5 text-[13.5px]">
          {STATES.map((state) => (
            <div key={state.id} className="flex gap-3">
              <dt
                className={`w-24 shrink-0 font-semibold ${
                  state.id === current ? "text-accent-text" : "text-faint"
                }`}
              >
                {state.label}
              </dt>
              <dd className="text-muted">{state.blurb}</dd>
            </div>
          ))}
        </dl>
        <p className="text-[13px] text-faint">Reads keep working in all three.</p>
        <Input aria-label="Reason" value={reason} onChange={(e) => setReason(e.target.value)} />
        <p className="-mt-1.5 text-[13px] text-faint">
          Why — this is what the next person to open this page reads.
        </p>
        <div className="flex flex-wrap gap-2">
          {STATES.map((state) => (
            <Button
              key={state.id}
              // All three outline, including the one already in force. A
              // borderless control between two bordered ones reads as a
              // rendering fault, not as emphasis — and the label ("Already
              // active") plus the disabled state already say which is current.
              variant="outline"
              size="sm"
              disabled={!reason || set.isPending || state.id === current}
              onClick={() =>
                set.mutate({ state: state.id, reason }, { onSuccess: () => setReason("") })
              }
            >
              {state.id === current ? `Already ${state.label.toLowerCase()}` : state.label}
            </Button>
          ))}
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
              This does not affect the &ldquo;Paused after failures&rdquo; reading above.
            </p>
            {reason && (
              <p className="text-[13px] text-muted">
                Your reason was kept. Trying again sends the same request with the same reason.
              </p>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}
