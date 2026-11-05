"use client";
import { personOr } from "@/lib/names";

import { useContext, useState } from "react";

import {
  SourceChip,
  sourceQualifier,
  type RoleReason,
  type SourceKind,
} from "@/components/access/AccessSource";
import { RoleRef } from "@/components/names";
import { useShownNames } from "@/components/names/shownNames";
import { sortNames } from "@/lib/sort";
import { ActionForecast, ActionOutcome } from "@/components/ui/ActionOutcome";
import { useRoleWaits } from "@/lib/queries/useRoleWaits";
import { settleIndicators } from "@/lib/queries/useIndicators";
import { QueryClientContext } from "@tanstack/react-query";
import { StatusDot } from "@/components/ui/Badge";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { useWaitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { KEPT, PushedStep, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useRemoveDirectGrant, type DirectGrantRemoval } from "@/lib/queries/useRoleMembers";
import { RETRYING, retryingOutcome } from "@/lib/retry";
import { waitReasonLine } from "@/lib/wait-reason";
import { useRemoveBundle } from "@/lib/queries/useBundles";
import {
  forecastFor,
  FORECAST,
  outcomeFromError,
  succeeded,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";

/**
 * Source-specific removal.
 *
 * There is never a generic "Revoke role". A person can hold one role through
 * several sources at once, so a generic action is ambiguous at best and
 * destructive at worst. The action is named after the thing being removed, and
 * the confirmation states the residual outcome — what they are left holding.
 *
 * That sentence is not garnish. It is the difference between a safe click and
 * an outage on the laser cutter.
 */

export interface Removal {
  projectId: string;
  projectName: string;
  roleKey: string;
  sources: RoleReason[];
  /** The direct grant id, when one of the sources is direct. */
  grantId?: string;
  /**
   * Whether the caller's grant list RESOLVED, as distinct from whether it
   * contained this role.
   *
   * Without it, an absent `grantId` has two causes that want opposite
   * sentences: the role genuinely is not a direct grant, or the query that
   * would have said so has not answered yet (or failed). The person page reads
   * grants from a second query with no loading or error gate, so the second
   * case is reachable on every load and permanent if that fetch fails — and the
   * dialog told the operator "this role was not given directly", which is the
   * screen inventing a fact about their data to explain its own missing read.
   *
   * Absent means resolved, so the caller that has its grant ids inline (the
   * project role page) needs to say nothing.
   */
  grantsResolved?: boolean;
  /** Whose access this is. Defaults to the person whose page we're on. */
  userId?: string;
  userName?: string;
}

/** "Project / Role name" for a title, which is text; the body shows the pair as RoleRef. */
function useRoleLabel() {
  const names = useShownNames();
  return (removal: Removal) => `${removal.projectName} / ${names.role(removal.projectId, removal.roleKey)}`;
}

type ShownNames = Pick<ReturnType<typeof useShownNames>, "project" | "role">;

/** "Role in Project" for each "project/role" pair the server names, alphabetical. */
export function retainedLabels(pairs: string[], names: ShownNames): string[] {
  return sortNames(
    pairs.map((pair) => {
      const at = pair.indexOf("/");
      return at < 0 ? pair : `${names.role(pair.slice(0, at), pair.slice(at + 1))} in ${names.project(pair.slice(0, at))}`;
    }),
  );
}

/** The act's own word on the result, never "Applied" (A-17 G4). */
const resultWord = (outcome: Outcome | null) => (succeeded(outcome) ? "Removed" : "Not removed");

/**
 * What removing a direct grant did, from the server's answer (W7). "The access
 * is gone" only when nothing was queued or Zitadel took the revocation; a role
 * that waits, a send that stopped, and a send Zitadel didn't take each say so.
 */
export function removalOutcome(result: DirectGrantRemoval | undefined, roleLabel: string, names?: ShownNames): Outcome {
  const raw = result?.retained_roles ?? [];
  const retained = names ? retainedLabels(raw, names) : raw;
  const kept = retained.length ? ` They still hold ${retained.join(", ")}, from a bundle or a rule.` : "";
  const reason = result?.wait_reason;
  if (reason === RETRYING) return retryingOutcome(`Removed in Syndra`, result?.retry);
  if (reason === "role_waits") {
    return {
      kind: "queued",
      message: `Removed in Syndra. ${roleLabel} always waits for you, so it leaves Zitadel when you send it.${kept}`,
      detail: waitReasonLine(reason, roleLabel),
    };
  }
  if (reason) {
    // Halted: a revocation the background drain sends on its own.
    const why = waitReasonLine(reason, roleLabel, new Date().toISOString());
    return {
      kind: "queued",
      message: `Removed in Syndra. ${why ? `${why}. ` : ""}Revocations send on their own, so it leaves Zitadel when Zitadel answers.${kept}`,
      sendsItself: true,
    };
  }
  return {
    kind: "applied",
    message: `Direct access to ${roleLabel} removed`,
    detail: retained.length
      ? `They still hold ${retained.join(", ")}, from a bundle or a rule.`
      : "Nothing else was supplying it, so the access is gone.",
  };
}

export function RemovalDialog({
  removal,
  onClose,
  userId,
  userName,
}: {
  removal: Removal | null;
  onClose: () => void;
  /**
   * Who this is about. REQUIRED, and deliberately `string | undefined` rather
   * than optional: a caller must state the person even when it has none,
   * because the alternative is what happened — `PersonAccess` simply never
   * passed it, so every removal button on the person page was disabled and the
   * dialog titled itself "…from this person" for the whole life of the feature.
   *
   * An optional prop is silent when omitted. This one is load-bearing, so the
   * typecheck asks for it and the next call site cannot forget it the same way.
   */
  userId: string | undefined;
  userName: string | undefined;
}) {
  const [chosen, setChosen] = useState<SourceKind | null>(null);
  const label = useRoleLabel();

  if (!removal) return null;

  const subject = personOr(removal.userName ?? userName);
  const person = removal.userId ?? userId;
  const roleLabel = label(removal);

  // With more than one source, the menu lists ONE entry per source, each named
  // after its own removal — never a single control that guesses.
  if (removal.sources.length > 1 && !chosen) {
    return (
      <Modal open onClose={onClose} size="sm">
        <ModalHeader
          title={`${subject} holds ${roleLabel} ${removal.sources.length} ways.`}
          lede="Each source is removed on its own terms. Pick the one you mean."
        />
        <div className="flex flex-col gap-1.5 px-6">
          {removal.sources.map((source) => (
            <ChoiceRow
              key={source.kind}
              single
              name="removal-source"
              selected={false}
              onToggle={() => setChosen(source.kind as SourceKind)}
              title={actionName(source.kind as SourceKind)}
              description={sourceQualifier(source) ?? undefined}
              aside={<SourceChip kind={source.kind as SourceKind} />}
            />
          ))}
        </div>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
        </ModalFooter>
      </Modal>
    );
  }

  const kind = (chosen ?? (removal.sources[0]?.kind as SourceKind)) ?? "direct";
  const source = removal.sources.find((entry) => entry.kind === kind) ?? removal.sources[0];
  const others = removal.sources.filter((entry) => entry.kind !== kind);

  const close = () => {
    setChosen(null);
    onClose();
  };

  if (kind === "mapping") {
    return <AutomaticDialog removal={removal} source={source} subject={subject} onClose={close} />;
  }
  if (kind === "bundle") {
    return (
      <BundleDialog
        removal={removal}
        source={source}
        subject={subject}
        userId={person}
        others={others}
        onClose={close}
      />
    );
  }
  return (
    <DirectDialog
      removal={removal}
      source={source}
      subject={subject}
      userId={person}
      others={others}
      onClose={close}
    />
  );
}

function actionName(kind: SourceKind): string {
  if (kind === "direct") return "Revoke direct access";
  if (kind === "bundle") return "Remove bundle assignment";
  return "Open the rule";
}

/**
 * Direct removal. The residual outcome in bold, plus the real-world
 * consequence — the sentence an operator needs before they click.
 */
function DirectDialog({
  removal,
  source,
  subject,
  userId,
  others,
  onClose,
}: {
  removal: Removal;
  /** The source being removed — carries whether its grant has been sent. */
  source: RoleReason | undefined;
  subject: string;
  userId?: string;
  others: RoleReason[];
  onClose: () => void;
}) {
  const remove = useRemoveDirectGrant();
  const roleWaits = useRoleWaits();
  const client = useContext(QueryClientContext);
  const waits = useWaitsBeforeLine("remove");
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const retained = others.length > 0;
  const roleLabel = useRoleLabel()(removal);
  const shown = useShownNames();

  // `grantsResolved === false` is the third state, and the reason this is not
  // a boolean: "no grant id" and "no answer yet" are different facts and the
  // operator needs to be told which one they are looking at.
  const grantListPending = removal.grantsResolved === false;
  const blocked = !removal.grantId || !userId;
  // Taking back a grant that is still waiting cancels it: the sheet turns to the
  // step first (A-04 §14), and Keep it comes back to this dialog as it was.
  const step = useCancelStep({
    host: `Revoke direct access to ${roleLabel}`,
    goAhead: "revoke",
    tone: source?.queued ? "accent" : "danger",
  });

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="removal-title">
      <PushedStep step={step}>
      <ModalHeader
        chip={<SourceChip kind="direct" />}
        title={`Revoke direct access to ${roleLabel}?`}
        titleId="removal-title"
        lede={`Given directly to ${subject}.`}
      />
      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[15px] leading-[1.5]">
          {retained ? (
            <>
              <strong className="font-semibold text-danger-text">
                {subject} will still hold this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                It also comes from {describeOthers(others)}, so nothing changes at the door today.
              </span>
            </>
          ) : source?.queued ? (
            // Never delivered, so there is nothing at the door to close. Saying
            // "they will lose this role" here would describe an effect this
            // grant never had — and the removal does not queue a revocation,
            // it withdraws the delivery that was still waiting.
            <>
              <strong className="font-semibold text-ink">
                Nothing to take back.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                This grant is still waiting for you to send, so {subject} never received it. Removing it withdraws the queued change; nothing reaches Zitadel.
              </span>
            </>
          ) : (
            <>
              <strong className="font-semibold text-danger-text">
                {subject} will lose this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                No bundle and no rule gives it to them. Their access ends once the change reaches Zitadel.{" "}
                {roleWaits(removal.projectId, removal.roleKey)
                  ? `${roleLabel} always waits for you, so the change waits in Pending changes until you send it.`
                  : "Revocations send on their own, within a minute."}
              </span>
            </>
          )}
        </div>
      </div>
      {/* The first 100ms. Withdrawing a grant that is still waiting cancels it; a role
          that waits holds its own removal; anything else is simply removed. */}
      {!outcome && remove.isPending && (
        <ActionForecast
          className="mx-6 mb-1"
          word={source?.queued ? FORECAST.cancel : forecastFor("remove", roleWaits(removal.projectId, removal.roleKey))}
        />
      )}
      {outcome && <ActionOutcome word={resultWord(outcome)} outcome={outcome} person={userId} className="mx-6 mb-1" />}
      {outcome && succeeded(outcome) && step.after}

      <ModalFooter
        cancel={succeeded(outcome) ? undefined : { onClick: onClose }}
        note={
          step.note ??
          // Two causes, and they were reported as one. "This role was not given
          // directly" is true when the grant id is missing; it is a fabrication
          // when the real problem is that the caller passed no person, which is
          // what the person page did for the whole life of this dialog. A note
          // that blames the data for a wiring fault sends an operator to check
          // the wrong thing.
          !userId
            ? "Refused · Syndra could not tell which person this is. Reload the page."
            : grantListPending
              ? "Waiting · Syndra is still reading this person's direct grants, so it cannot yet tell whether there is one to revoke."
              : blocked
                ? "Refused · this role was not given directly, so there is no direct access to revoke. Only direct access can be revoked here."
                : outcome
                  ? undefined
                  : waits
        }
      >
        {/* The act never takes the dialog's initial focus (`data-skip-initial-focus`,
            Modal.tsx): Enter, pressed on reflex to close a dialog that just
            opened, once revoked access. Cancel takes it, wherever it sits.
            And an action that has run is not an action any more: leaving it
            armed lets a second click revoke again, so what remains is Close. */}
        {succeeded(outcome) ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            data-skip-initial-focus
            variant={source?.queued ? "accent" : "dangerConfirm"}
            disabled={blocked}
            isPending={remove.isPending}
            onClick={async () => {
              try {
                const result = await step.run(() =>
                  remove.mutateAsync({
                    userId: userId!,
                    grantId: removal.grantId!,
                  }),
                );
                if (result === KEPT) return;
                // A withdrawn grant leaves the queue; the rail follows the answer.
                if (source?.queued && client) settleIndicators(client, { pending_propagation: -1 });
                // The residual outcome, from the backend that computed it. A
                // role this person also holds through a bundle or a rule
                // survives the removal, and which ones those are is a closure
                // diff the server does — the UI must not hold a second opinion
                // about somebody's access.
                setOutcome(removalOutcome(result, roleLabel, shown));
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {source?.queued ? "Withdraw the queued grant" : "Revoke access"}
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

/**
 * Bundle removal. Two explicit lists — what they lose, and what they keep and
 * why it survives. The "why" is what stops the operator second-guessing.
 */
function BundleDialog({
  removal,
  source,
  subject,
  userId,
  others,
  onClose,
}: {
  removal: Removal;
  source: RoleReason;
  subject: string;
  userId: string | undefined;
  others: RoleReason[];
  onClose: () => void;
}) {
  const bundleName = source.bundle_name ?? "this bundle";
  const removeBundle = useRemoveBundle(userId ?? "");
  const roleWaits = useRoleWaits();
  const waits = useWaitsBeforeLine("remove");
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const step = useCancelStep({
    host: `Remove the ${bundleName} bundle from ${subject}`,
    goAhead: "remove",
    tone: source.queued ? "accent" : "danger",
  });

  return (
    <Modal open onClose={onClose} busy={removeBundle.isPending} size="sm" labelledBy="removal-title">
      <PushedStep step={step}>
      <ModalHeader
        chip={<SourceChip kind="bundle" />}
        title={`Remove the ${bundleName} bundle from ${subject}?`}
        titleId="removal-title"
        lede={
          source.queued
            ? `${bundleName} is still waiting for you to send, so nothing it carries has reached ${subject} yet.`
            : `Everything ${bundleName} carries goes with it, except what another source also gives them.`
        }
      />
      <div className="flex flex-col gap-2 px-6">
        {/* "They will lose" is a claim about an effect, and this bundle has not
            had one yet. The assignment was recorded and never sent, so removing
            it withdraws the queued delivery rather than queueing its opposite —
            which is also what the backend now does, instead of enqueueing a
            revocation for access nobody ever received. */}
        <div className="type-label">{source.queued ? "Nothing to take back" : "They will lose"}</div>
        <div className="flex flex-wrap items-center gap-2.5 text-[14px]">
          <StatusDot tone={source.queued ? "neutral" : "danger"} />
          <RoleRef projectId={removal.projectId} roleKey={removal.roleKey} />
          {source.queued ? (
            <span className="text-[13px] text-muted">· recorded, never sent</span>
          ) : (
            others.length === 0 && <span className="text-[13px] text-muted">· no other source gives it</span>
          )}
        </div>

        {others.length > 0 && (
          <>
            <div className="mt-2 type-label">They will keep</div>
            <div className="flex flex-wrap items-center gap-2.5 text-[14px]">
              <StatusDot tone="healthy" />
              <RoleRef projectId={removal.projectId} roleKey={removal.roleKey} />
              <span className="text-[13px] text-muted">· still {describeOthers(others)}</span>
            </div>
          </>
        )}
      </div>
      {!outcome && removeBundle.isPending && (
        <ActionForecast
          className="mx-6 mb-1"
          word={source.queued ? FORECAST.cancel : forecastFor("remove", roleWaits(removal.projectId, removal.roleKey))}
        />
      )}
      {outcome && <ActionOutcome word={resultWord(outcome)} outcome={outcome} person={userId} className="mx-6 mb-1" />}
      {outcome && succeeded(outcome) && step.after}

      <ModalFooter
        cancel={succeeded(outcome) ? undefined : { onClick: onClose }}
        // A bundle whose grants are still waiting says nothing about them here:
        // the step is the one place that is said, because only it carries who
        // queued each change and why (A-04 §13.4).
        note={
          step.note ??
          (source.queued
            ? undefined
            : ((!outcome && waits) ||
              "Every other role this bundle carries is removed too. Manage bundles shows the full list before you commit."))
        }
      >
        {/* Gone once it has run. A destructive confirm that stays armed after
            it succeeded fires again on the next click, which is what a person
            does when the dialog does not close, and each press queued another
            cascade. What remains is the one action still true: leave. As in
            the direct dialog, the act never takes the initial focus. */}
        {succeeded(outcome) ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            data-skip-initial-focus
            // Red is a promise about consequence. Withdrawing a delivery that
            // never went out takes nothing from anybody, and dressing it as
            // destruction contradicts the sentence directly above it.
            variant={source.queued ? "accent" : "dangerConfirm"}
            isPending={removeBundle.isPending}
            disabled={!userId || !source.bundle_id}
          // It was disabled in silence, which is how a dead removal flow went
          // unnoticed: the cause was upstream (the person page passed no
          // userId) and the button said nothing at all. Now it names which
          // half is missing, so the next time this happens it is a bug report
          // rather than a shrug.
            reason={
              !userId
                ? "Syndra could not tell which person this is, so it will not remove a bundle from them. Reload the page."
                : !source.bundle_id
                  ? "Syndra could not tell which bundle gives this role. Manage bundles can remove it by name."
                  : undefined
            }
            onClick={async () => {
              try {
                const result = await step.run(() => removeBundle.mutateAsync(source.bundle_id!));
                if (result === KEPT) return;
                const waiting = result?.cascade?.enqueued ?? 0;
                setOutcome({
                  // `queued` paints an amber "Waiting for you to send" badge, and a
                  // withdrawal leaves nothing waiting — the badge sat directly
                  // above "Nothing is waiting under Pending changes" and
                  // contradicted it. The record DID change, so this is not
                  // `no_change` either.
                  kind: waiting === 0 ? "applied" : "queued",
                  message: `${bundleName} removed from ${subject}`,
                  // What actually happened, rather than one sentence for both
                  // outcomes. The dialog above had just said nothing would be
                  // taken back, and this then said the roles are revoked when
                  // you send: two opposite accounts of one click, a centimetre
                  // apart.
                  detail:
                    waiting === 0
                      ? "The grants it queued were withdrawn. Nothing is waiting under Pending changes, because nothing had been sent."
                      : "The roles it supplied are revoked once you send Pending changes, except any that another source still gives.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {source.queued ? "Withdraw the queued change" : "Remove bundle"}
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

/**
 * Automatic — no removal offered, because nothing here is the operator's to
 * remove. Two real ways forward instead, and no destructive colour anywhere:
 * nothing is being destroyed.
 */
function AutomaticDialog({
  removal,
  source,
  subject,
  onClose,
}: {
  removal: Removal;
  source: RoleReason;
  subject: string;
  onClose: () => void;
}) {
  const input = sourceQualifier(source);

  return (
    <Modal open onClose={onClose} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="mapping" />}
        title="This one isn't yours to remove."
        titleId="removal-title"
        lede={
          <>
            {subject} holds <RoleRef projectId={removal.projectId} roleKey={removal.roleKey} /> because an
            automatic rule produced it.
          </>
        }
      />
      <div className="flex flex-col gap-2.5 px-6">
        <div className="text-[14.5px] leading-[1.5]">
          <span className="text-muted">{input ?? "an input role"}</span>
          <span className="text-faint"> gives </span>
          <span className="font-semibold">
            <RoleRef projectId={removal.projectId} roleKey={removal.roleKey} />
          </span>
          <div className="mt-1.5 text-[13.5px] text-faint">
            Editing the rule changes access for everyone it applies to.
          </div>
        </div>
        <p className="text-[14px] leading-[1.55] text-muted">
          To revoke this from {subject} alone, remove their{" "}
          <span className="text-ink">{input ?? "input role"}</span>. That is the input the rule
          reads.
        </p>
        <ArrowLink href="/policies" className="self-start">
          Open the rule
        </ArrowLink>
      </div>
      <ModalFooter>
        <Button onClick={onClose}>Close</Button>
      </ModalFooter>
    </Modal>
  );
}

/** "via Lab Tech and automatic from 3D Lab / operator" — the residual sources. */
function describeOthers(others: RoleReason[]): string {
  if (others.length === 0) return "no other source";
  return others
    .map((source) => {
      const qualifier = sourceQualifier(source);
      if (source.kind === "bundle") return `via ${qualifier ?? "a bundle"}`;
      if (source.kind === "mapping") return `automatic${qualifier ? ` from ${qualifier}` : ""}`;
      return "given directly";
    })
    .join(" and ");
}
