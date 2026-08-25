"use client";

import { Fragment, useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { MergeFindings } from "@/components/targets/MergeFindings";
import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { blocksIrreversibleAction, ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { formatBytes } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useTargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";
import {
  useAdoptAccount,
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
  const registered = (roster.data ?? []).find((t) => t.target === target);

  return (
    <>
      <PageHeader
        title={targetLabel(target)}
        meta={registered ? authLabel(registered.auth_mode) : undefined}
      />

      <div className="grid gap-4">
        <Health
          target={target}
          health={health.data}
          isLoading={health.isLoading}
          // A deployment-side fault, carried into the health card because that
          // is where an operator looks when a target stops working — and this
          // one explains the reading below it rather than sitting beside it.
          transportError={
            registered?.transport_status === "error" ? registered.transport_error : undefined
          }
        />
        {/* What the TARGET says about itself, directly under what the ADD-ON
            says about the target. Same question, one layer further down: the
            card above answers "is Syndra able to talk to it", this one answers
            "and is the machine itself all right". A failing disk shows up here
            and nowhere else in Syndra. */}
        <SystemHealth target={target} />
        {/* What roles reach here, before whose accounts are on it: the mappings
            are the reason any of those accounts exist, and reading the
            inventory first invites the question this panel answers. */}
        <MappingManagement target={target} />
        {/* Whose accounts are on it — the managed half first, because it is the
            half an operator acts on, and the unmanaged inventory below reads as
            "and what else is here". */}
        <PeopleOnTarget target={target} />
        {/* Accounts Syndra created and no longer has a reason for, between the
            people it manages and the accounts it never made: it is the third
            answer to "whose accounts are on it", and the only one with an
            action that removes data. */}
        <DormantAccounts target={target} />
        <Inventory target={target} />

        {registered && (
          <Card>
            <CardHeader
              title="What it can do"
              note="Read from the add-on's manifest, never from a list here"
            />
            {!registered.callable ? (
              <div className="px-5 pb-5">
                <p className="text-[14px] text-muted">
                  Registered, and it has not published a capability manifest yet.
                  Registration is a deployment fact; what it can do is a runtime
                  one, and nothing is offered until it answers.
                </p>
              </div>
            ) : (
              registered.operations.map((op, i) => (
                <CardRow key={op.id} first={i === 0} className="flex-wrap">
                  <span className="font-mono text-[13.5px]">{op.id}</span>
                  <span className="text-[13px] text-faint">{op.scope}</span>
                  {op.secret_params && op.secret_params.length > 0 && (
                    // Named, never valued. There is nowhere in this payload for
                    // a secret and nowhere on this page to render one.
                    <span className="text-[12.5px] text-faint">
                      never logged: {op.secret_params.join(", ")}
                    </span>
                  )}
                  <span className="flex-1" />
                  {!op.available && (
                    // Shown disabled with its reason rather than omitted:
                    // omitted, an operator wonders whether the feature exists.
                    <span className="text-[13px] text-warn-text">
                      unavailable — {op.unavailable_reason}
                    </span>
                  )}
                </CardRow>
              ))
            )}
          </Card>
        )}

        {/* Above the reconcile control on purpose: pressing [Reconcile now]
            with disputed values outstanding does not resolve them, and an
            operator who reads the button first will assume it did. */}
        <MergeFindings target={target} />

        <ReconcileControl target={target} />

        <LifecycleControl target={target} health={health.data} />
      </div>
    </>
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
        title="What the target reports"
        note={
          data.system?.hostname
            ? `${data.system.hostname}${data.system.version ? ` · ${data.system.version}` : ""}`
            : "Read from the target itself, not from Syndra's record"
        }
      />
      <div className="flex flex-col gap-2.5 px-5 pb-5">
        {!data.readable && (
          <Reading tone="warn" label="Could not be asked">
            The target did not answer its own health reads, so this is not a report that
            nothing is wrong.{data.detail ? ` ${data.detail}` : ""}
          </Reading>
        )}

        {/* Named sources. "alerts could not be read" and "there are no alerts"
            are the same empty list without this, and they are opposite facts. */}
        {degraded.length > 0 && (
          <Reading tone="warn" label="Partly read">
            {degraded.join(", ")} could not be read. Whatever those would have said is
            missing from this card rather than absent from the target.
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
            label={pool.name}
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
            <Reading key={s.service} tone="danger" label={`${s.service} is not running`}>
              Accounts Syndra provisions here reach nothing while it is stopped
              {s.enable ? ", and it is set to start on boot — so it stopped on its own." : ", and it is not set to start on boot."}
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

  return (
    <Card>
      <CardHeader
        title="Reconciliation"
        note="Reads the target and queues what is already owed. Queueing is not applying."
      />
      <CardRow>
        <div className="flex-1 text-[14.5px] text-muted">
          {result
            ? `${result.bound} managed · ${result.queued} queued · ${result.stale?.length ?? 0} pointing at nothing`
            : "The scheduled sweep runs every six hours."}
        </div>
        <Button
          variant="outline"
          size="sm"
          isPending={run.isPending}
          onClick={() => run.mutate()}
        >
          Reconcile now
        </Button>
      </CardRow>

      {run.error && (
        <CardRow>
          <span className="text-[13.5px] text-danger-text">
            {run.error instanceof Error ? run.error.message : "The pass did not complete."}
          </span>
        </CardRow>
      )}

      {result && !result.current && (
        // A pass that concluded nothing must not read as a clean one.
        <CardRow>
          <span className="text-[13.5px] text-warn-text">
            Nothing was concluded: {result.reason || "the target could not be read for itself"}.
          </span>
        </CardRow>
      )}

      {(result?.stale?.length ?? 0) > 0 && (
        <>
          <CardRow>
            <span className="text-[14px] font-semibold text-warn-text">
              {result!.stale!.length} binding{result!.stale!.length === 1 ? "" : "s"} point at an
              account that is no longer on the target
            </span>
          </CardRow>
          {result!.stale!.map((b) => (
            <CardRow key={b.subject_id} className="flex-wrap">
              <Mono>{b.username}</Mono>
              {b.uid ? <span className="text-[13px] text-faint">uid {b.uid}</span> : null}
              <span className="flex-1" />
              {released.includes(b.subject_id) ? (
                <span className="text-[13px] text-faint">
                  Released. Nothing on the target was changed.
                </span>
              ) : confirming === b.subject_id ? (
                <>
                  {/* What it does, in the sentence next to the button that does
                      it. The word "forget" is doing real work here: an operator
                      reading it as "delete" is the one misreading this row can
                      afford least, given the account it names is already gone. */}
                  <span className="text-[13px] text-muted">
                    Syndra stops managing <span className="font-mono">{b.username}</span>.
                    Nothing is deleted, and the binding can be made again by adopting the
                    account if it comes back.
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
                              "The release was not confirmed. Nothing was changed here.",
                          }));
                        },
                      })
                    }
                  >
                    Forget it
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
                    Not converged. Re-provisioning would recreate a deleted account, so this
                    is yours to decide.
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setConfirming(b.subject_id)}
                  >
                    Forget this binding
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
                  : "The binding was not released."}
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
function authLabel(mode: string): string {
  if (mode === "derived") return "Authenticated by a key derived from this deployment's secret";
  if (mode === "none") return "NOT AUTHENTICATED — no transport secret configured";
  return `Authenticated by ${mode}`;
}

function Health({
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
  return (
    <Card>
      <CardHeader title="Health" />
      <div className="grid gap-3 px-5 pb-5">
        {isLoading && !health && <p className="text-[14px] text-faint">Reading…</p>}

        {health?.log_anchor?.violation_reason && <LogFinding anchor={health.log_anchor} />}

        {/* Above the reachability reading, deliberately. A target being down is
            temporary and this is not: it is two of Syndra's own records
            disagreeing, and it stands whether or not the add-on is answering. */}
        {(health?.binding_conflicts ?? []).map((conflict) => (
          <BindingConflictFinding key={conflict.id} target={target} conflict={conflict} />
        ))}

        {/* Above reachability, because it EXPLAINS it. A target whose transport
            secret cannot be read will also not answer, and an operator who
            reads "not answering" first goes to the NAS — which is the wrong
            machine, and the one that takes longest to rule out. */}
        {transportError && (
          <Reading tone="danger" label="Transport secret unreadable">
            Syndra cannot read this target&apos;s transport secret, so no call to it can be
            authenticated. This is a fault on <strong className="font-semibold">this</strong>{" "}
            host, not on the target: {transportError}
          </Reading>
        )}

        {health && !health.reachable && (
          <Reading tone="danger" label="Not answering">
            {health.detail || `${targetLabel(target)} did not answer.`} The add-on is the
            thing to look at.
          </Reading>
        )}

        {health?.circuit_open && (
          <Reading tone="danger" label="Backed off">
            Syndra is refusing its own calls after repeated failures.{" "}
            <strong className="font-semibold">This is not the target being down</strong> — it
            clears on its own, and the machine to look at is this one.
          </Reading>
        )}

        {health?.reachable && health.lifecycle && health.lifecycle !== "active" && (
          // Accent, never amber. Somebody chose this, and the same choice is
          // accent on the withdrawn-access queue for the same reason.
          <Reading tone="accent" label={health.lifecycle === "draining" ? "Draining" : "Read-only"}>
            Set deliberately{health.lifecycle_note ? `: ${health.lifecycle_note}` : ""}.{" "}
            {health.lifecycle === "draining"
              ? "New changes are refused and the ones already sent are being allowed to finish."
              : "Every change is refused immediately. Reads keep working."}
          </Reading>
        )}

        {health?.in_flight !== undefined && health.in_flight > 0 && (
          <Reading tone="warn" label="Still settling">
            {health.in_flight} call{health.in_flight === 1 ? "" : "s"} issued before the drain{" "}
            {health.in_flight === 1 ? "has" : "have"} not come back. This is what to wait for
            before pulling a credential out from under one.
          </Reading>
        )}

        {/* A credential whose expiry nobody recorded. Not amber for its own
            sake: the key CAN expire without Syndra knowing, and the day it does
            the target simply stops answering — which reads as an outage and
            sends an operator to the NAS. `none` is an operator's deliberate
            choice and says nothing here. */}
        {health?.reachable && health.key_expiry === "unrecorded" && (
          <Reading tone="warn" label="Key expiry not recorded">
            Syndra does not know when this target&rsquo;s API key expires. If it has an
            expiry, set <Mono>TRUENAS_API_KEY_EXPIRES_AT</Mono> so this warns before it
            fails; if it has none, set it to <Mono>never</Mono> to say so.
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
            Enable it per share on the target: Shares → SMB → Edit → Advanced.
          </Reading>
        )}

        {health?.reachable && health.version_tested === false && (
          <Reading tone="warn" label="Untested release">
            {health.version_note || "This release has not been tested against."} Reads keep
            working; changes are refused.
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
            <Line label="Credential expires">
              <Relative iso={health.key_expires_at} />
            </Line>
          )}
          {health?.log_head && (
            <Line label="Change record">
              {health.log_records ?? 0} records ·{" "}
              <span className="font-mono text-[12.5px] text-faint">
                {health.log_head.slice(0, 12)}
              </span>
            </Line>
          )}
        </dl>

        {health?.snapshot_taken_at && (
          <ReadFreshness
            subject="During an outage this target's state"
            state={{ readAt: health.snapshot_taken_at, current: health.reachable }}
          />
        )}
      </div>
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
  const what =
    anchor.violation_reason === "records_decreased"
      ? "Records that existed are gone."
      : "The same number of records now hash to something else.";
  return (
    <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
      <p className="text-[13.5px] font-semibold text-danger-text">
        This target&rsquo;s change record has been edited
      </p>
      <p className="mt-1 text-[13.5px] text-muted">
        {what} Syndra last saw {anchor.records} record{anchor.records === 1 ? "" : "s"} ending{" "}
        <span className="font-mono text-[12.5px]">{anchor.head.slice(0, 12)}</span>, anchored{" "}
        <Relative iso={anchor.anchored_at} />; the target reported {anchor.violation_records ?? 0}{" "}
        <Relative iso={anchor.violation_at} />.
      </p>
      <p className="mt-1 text-[13px] text-faint">
        The anchor has not moved and will not, so this stays until somebody resolves it. A
        chain verifies its own contents and cannot notice its own truncation — this is the
        only thing that can.
      </p>
      <div className="mt-3">
        <Button variant="ghost" size="sm" onClick={() => setResolving(true)}>
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
        <span className="font-mono text-[13px]">{conflict.username}</span>
      </p>
      <p className="mt-1 text-[13.5px] text-muted">
        A change for{" "}
        <UserName id={conflict.converged_subject_id} fallback={conflict.converged_subject_id} /> was
        applied to this account, and Syndra records it as belonging to{" "}
        <UserName id={conflict.bound_subject_id} fallback={conflict.bound_subject_id} />. The change
        landed on the target — what could not be recorded is whose account it is.
      </p>
      <p className="mt-1 text-[13px] text-faint">
        Noticed <Relative iso={conflict.detected_at} />. Nothing else will resolve this: a
        convergence for either person acts on whichever record it reads.
      </p>
      <div className="mt-3">
        <Button variant="ghost" size="sm" onClick={() => setDeciding(true)}>
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
            { id: conflict.bound_subject_id, why: "Syndra's own binding says so." },
            { id: conflict.converged_subject_id, why: "Their change was applied to it." },
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
            The other person stops holding this account in Syndra, immediately and without
            being told. Their data on the target is untouched — this changes who Syndra says
            it belongs to, which is what every later revocation, sweep and convergence acts
            on. A convergence is queued for both of them, because the change that caused
            this overwrote one person&rsquo;s entitlements with the other&rsquo;s.
          </p>
        </div>

        <label className="grid gap-1.5 text-[14px]">
          <span>How you know</span>
          <Input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Checked the home directory contents with them"
          />
          <span className="text-[13px] text-faint">
            The row somebody reads when the other person asks where their account went.
          </span>
        </label>

        <ConfirmByTyping
          expected={conflict.username}
          value={confirmation.typed}
          onChange={confirmation.setTyped}
          noun="account"
          disabled={resolve.isPending}
        />
        {resolve.error && (
          <p className="text-[13.5px] text-danger-text">
            {resolve.error instanceof Error ? resolve.error.message : "That could not be applied."}
          </p>
        )}
      </div>
      <ModalFooter note="A convergence is queued for both people. The account keeps what the change that caused this wrote to it until that drains.">
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
        lede={`Syndra will adopt the log ${targetLabel(target)} is reporting now as the new baseline.`}
      />
      <div className="grid gap-4 px-6">
        <div className="rounded-inner border border-danger-line bg-danger-soft px-4 py-3">
          <p className="text-[13.5px] text-danger-text">
            The records that went missing stay missing, and Syndra stops being able to tell
            you they did. Do this when you know why the log changed — a rebuilt add-on, a
            replaced volume — and not to clear a warning.
          </p>
        </div>
        <label className="grid gap-1.5 text-[14px]">
          <span>Why the log changed</span>
          <Input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="We replaced the add-on&rsquo;s volume on the 4th"
          />
          <span className="text-[13px] text-faint">
            Kept with the resolution. &ldquo;We replaced the volume&rdquo; and &ldquo;we do
            not know&rdquo; leave the anchor in the same state and are completely different
            facts.
          </span>
        </label>
        <ConfirmByTyping
          expected={target}
          value={confirmation.typed}
          onChange={confirmation.setTyped}
          noun="target"
          disabled={resolve.isPending}
        />
        {resolve.error && (
          <p className="text-[13.5px] text-danger-text">
            {resolve.error instanceof Error ? resolve.error.message : "That could not be applied."}
          </p>
        )}
      </div>
      <ModalFooter note="The anchor moves to the reported head and starts comparing again from there.">
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
          {resolve.isPending ? "Resolving…" : "Adopt this log as the baseline"}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={resolve.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

const READING_TONE = {
  healthy: { dot: "bg-healthy", label: "text-ink" },
  accent: { dot: "bg-accent", label: "text-accent-text" },
  warn: { dot: "bg-warn", label: "text-warn-text" },
  danger: { dot: "bg-danger", label: "text-danger-text" },
} as const;

/** One health reading: a dot that carries the tone, a label, and the sentence. */
function Reading({
  tone,
  label,
  children,
}: {
  tone: keyof typeof READING_TONE;
  label: string;
  children: React.ReactNode;
}) {
  const style = READING_TONE[tone];
  return (
    <div className="flex items-baseline gap-2.5 text-[14px]">
      <span aria-hidden className={`mt-1.5 size-1.5 shrink-0 rounded-pill ${style.dot}`} />
      <span>
        <span className={`font-semibold ${style.label}`}>{label}.</span>{" "}
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
  const [result, setResult] = useState<AdoptionResult | null>(null);

  const read = {
    readAt: inventory.data?.read_at,
    current: inventory.data?.current,
    truncated: inventory.data?.truncated,
  };
  const tooOldToAdopt = !inventory.data || blocksIrreversibleAction(read);

  return (
    <Card>
      <CardHeader
        title="Accounts Syndra did not create"
        count={inventory.data?.unmanaged?.length}
        note="Reported, never triaged. These are not drift."
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
            title="Nothing unmanaged"
            guidance="Every account on this target belongs to somebody Syndra provisioned."
          />
        }
      >
        <>
          {(inventory.data?.unmanaged ?? []).map((account, i) => (
            <CardRow key={account.username} first={i === 0}>
              <span className="font-mono text-[13.5px]">{account.username}</span>
              {account.uid !== undefined && (
                <span className="text-[13px] text-faint">uid {account.uid}</span>
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
                  Syndra&rsquo;s own account on this target — not adoptable
                </span>
              ) : tooOldToAdopt ? (
                // The reason as text, never a tooltip. A disabled control whose
                // reason lives in a `title` is a control nobody can find out
                // about on a keyboard or a phone.
                <span className="text-[13px] text-faint">
                  Adoption needs a current read of this list
                </span>
              ) : (
                <Button variant="ghost" size="sm" onClick={() => setAdopting(account.username)}>
                  Adopt
                </Button>
              )}
            </CardRow>
          ))}
        </>
      </ListStates>

      {adopting && (
        <AdoptPanel
          username={adopting}
          pending={adopt.isPending}
          error={adopt.error}
          onCancel={() => setAdopting(null)}
          onAdopt={(subjectId) =>
            adopt.mutate(
              { username: adopting, subjectId },
              {
                onSuccess: (res) => {
                  setResult(res);
                  setAdopting(null);
                },
              },
            )
          }
        />
      )}

      {result && <AdoptionOutcome result={result} onDismiss={() => setResult(null)} />}
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
  pending,
  error,
  onAdopt,
  onCancel,
}: {
  username: string;
  pending: boolean;
  error: unknown;
  onAdopt: (subjectId: string) => void;
  onCancel: () => void;
}) {
  const [subjectId, setSubjectId] = useState("");
  const confirm = useTypedConfirmation(username);

  return (
    <form
      className="row-divider grid gap-3 px-5 py-4 text-[14px]"
      onSubmit={(e) => {
        e.preventDefault();
        onAdopt(subjectId);
      }}
    >
      <p className="text-muted">
        Adopting <span className="font-mono text-ink">{username}</span> hands its home
        directory, its shares and its group memberships to that person.{" "}
        <strong className="font-semibold text-ink">There is no undo.</strong>
      </p>
      <p className="text-[13.5px] text-faint">
        Nothing on the account changes now; the next convergence applies their entitlements
        to it.
      </p>
      <Input
        aria-label="Person to adopt it for"
        placeholder="Subject id"
        value={subjectId}
        onChange={(e) => setSubjectId(e.target.value)}
      />
      <ConfirmByTyping
        expected={username}
        noun="account name"
        value={confirm.typed}
        onChange={confirm.setTyped}
      />
      <div className="flex gap-2">
        <Button
          type="submit"
          variant="dangerConfirm"
          disabled={!subjectId || !confirm.armed || pending}
        >
          {pending ? "Adopting…" : `Adopt ${username} for this person`}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {Boolean(error) && (
        <p className="text-[13.5px] text-danger-text">
          {error instanceof Error ? error.message : "That could not be applied."}
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
function AdoptionOutcome({ result, onDismiss }: { result: AdoptionResult; onDismiss: () => void }) {
  const unconfirmed = result.status !== "adopted";
  return (
    <div
      role="status"
      className={`row-divider flex flex-wrap items-baseline gap-2 px-5 py-3.5 text-[13.5px] ${
        unconfirmed ? "text-warn-text" : "text-muted"
      }`}
    >
      <span>{result.detail ?? (unconfirmed ? "The target did not confirm it." : "Adopted.")}</span>
      {result.warning && <span className="text-warn-text">{result.warning}</span>}
      <span className="flex-1" />
      <Button variant="ghost" size="sm" onClick={onDismiss}>
        Dismiss
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
function LifecycleControl({ target, health }: { target: string; health: TargetHealth | undefined }) {
  const set = useSetLifecycle(target);
  const [reason, setReason] = useState("");
  const current = health?.lifecycle ?? "active";

  const STATES: Array<{ id: string; label: string; blurb: string }> = [
    { id: "active", label: "Active", blurb: "Accept changes normally." },
    {
      id: "draining",
      label: "Draining",
      blurb:
        "Refuse new changes, let the ones already sent finish. This is what makes a credential rotation safe.",
    },
    { id: "read_only", label: "Read-only", blurb: "Refuse every change immediately." },
  ];

  return (
    <Card>
      <CardHeader title="Maintenance" note={`Currently ${current.replace("_", " ")}`} />
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
        <Input
          aria-label="Reason"
          placeholder="Why — this is what the next operator reads"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
        <div className="flex flex-wrap gap-2">
          {STATES.map((state) => (
            <Button
              key={state.id}
              variant={state.id === current ? "ghost" : "outline"}
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
        {set.error && (
          <p className="text-[13.5px] text-danger-text">
            {set.error instanceof Error ? set.error.message : "That could not be applied."}
          </p>
        )}
      </div>
    </Card>
  );
}
