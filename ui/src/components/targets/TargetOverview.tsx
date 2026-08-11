"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { blocksIrreversibleAction, ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { targetLabel } from "@/lib/nav";
import {
  useAdoptAccount,
  useSetLifecycle,
  useTargetHealth,
  useResolveLogFinding,
  useTargetInventory,
  useTargets,
  type AdoptionResult,
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
        meta={registered ? `Authenticated by ${registered.auth_mode}` : undefined}
      />

      <div className="grid gap-4">
        <Health target={target} health={health.data} isLoading={health.isLoading} />
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
function Health({
  target,
  health,
  isLoading,
}: {
  target: string;
  health: TargetHealth | undefined;
  isLoading: boolean;
}) {
  return (
    <Card>
      <CardHeader title="Health" />
      <div className="grid gap-3 px-5 pb-5">
        {isLoading && !health && <p className="text-[14px] text-faint">Reading…</p>}

        {health?.log_anchor?.violation_reason && <LogFinding anchor={health.log_anchor} />}

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
              {tooOldToAdopt ? (
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
