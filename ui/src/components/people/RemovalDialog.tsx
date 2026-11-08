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
import { settleIndicators, useAutoRetry } from "@/lib/queries/useIndicators";
import { QueryClientContext } from "@tanstack/react-query";
import { StatusDot } from "@/components/ui/Badge";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { useWaitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { KEPT, PushedStep, useCancelStep, useForetell } from "@/components/ui/CancelQueuedStep";
import { Notice } from "@/components/ui/Notice";
import { RuleConsentFailed } from "@/components/requests/RuleConsent";
import { isStale, usePlanConsent } from "@/lib/usePlanConsent";
import { formatShortDate } from "@/lib/format";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useRemoveDirectGrant, type DirectGrantRemoval } from "@/lib/queries/useRoleMembers";
import { RETRYING, refusedOutcome, retireOutcome, retryingOutcome } from "@/lib/retry";
import { isRefused, waitReasonLine } from "@/lib/wait-reason";
import { useRemoveBundle } from "@/lib/queries/useBundles";
import {
  forecastFor,
  FORECAST,
  outcomeFromError,
  succeeded,
  DRAIN_UNKNOWN_NEXT,
  type ActionOutcome as Outcome,
  type AutoRetry,
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
const resultWord = (outcome: Outcome | null, act: "revoke" | "remove" | "cancel" = "remove") => {
  if (outcome?.recorded) return "Refused by Zitadel";
  const done = { revoke: "Revoked", remove: "Removed", cancel: "Cancelled" }[act];
  return succeeded(outcome) ? done : `Not ${done.toLowerCase()}`;
};

/**
 * What removing a direct grant did, from the server's answer (W7). "The access
 * is gone" only when nothing was queued or Zitadel took the revocation; a role
 * that waits, a send that stopped, and a send Zitadel didn't take each say so.
 */
export function removalOutcome(result: DirectGrantRemoval | undefined, roleLabel: string, names?: ShownNames, auto: AutoRetry = true): Outcome {
  const raw = result?.retained_roles ?? [];
  const retained = names ? retainedLabels(raw, names) : raw;
  const kept = retained.length ? ` They still hold ${retained.join(", ")}, from a bundle or a rule.` : "";
  const reason = result?.wait_reason;
  // Final verification N1: Zitadel refused the revocation, so the access is not gone.
  if (isRefused(reason) || result?.status === "failed") {
    const out = refusedOutcome("Revoked in Syndra", reason, { revocation: true });
    return { ...out, detail: `${out.detail}${kept}` };
  }
  if (reason === RETRYING) return retryingOutcome(`Revoked in Syndra`, result?.retry);
  if (reason === "role_waits") {
    return {
      kind: "queued",
      message: `Revoked in Syndra. ${roleLabel} always waits for you, so it leaves Zitadel when you send it.${kept}`,
      detail: waitReasonLine(reason, roleLabel),
    };
  }
  if (reason) {
    // Halted: a revocation the background drain sends on its own; with the
    // drain off it waits in Pending changes until someone presses Send.
    const why = waitReasonLine(reason, roleLabel, new Date().toISOString());
    if (auto === null) {
      // Not known yet whether it goes on its own (N3): no promise either way.
      return {
        kind: "queued",
        message: `Revoked in Syndra. ${why ? `${why}. ` : ""}The revocation is not in Zitadel yet.${kept}`,
        sendsItself: true,
        next: DRAIN_UNKNOWN_NEXT,
      };
    }
    if (!auto) {
      return {
        kind: "queued",
        message: `Revoked in Syndra. ${why ? `${why}. ` : ""}The revocation waits in Pending changes until someone presses Send.${kept}`,
      };
    }
    return {
      kind: "queued",
      message: `Revoked in Syndra. ${why ? `${why}. ` : ""}Revocations send on their own, so it leaves Zitadel within a minute.${kept}`,
      sendsItself: true,
    };
  }
  return {
    kind: "applied",
    message: `Direct access to ${roleLabel} revoked`,
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
  const auto = useAutoRetry();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [word, setWord] = useState<string | null>(null);
  const retained = others.length > 0;
  const roleLabel = useRoleLabel()(removal);
  const shown = useShownNames();
  const first = firstName(subject);

  // `grantsResolved === false` is the third state, and the reason this is not
  // a boolean: "no grant id" and "no answer yet" are different facts and the
  // operator needs to be told which one they are looking at.
  const grantListPending = removal.grantsResolved === false;
  const blocked = !removal.grantId || !userId;
  // What revoking it does, from the planner before the press (A-21 §3.2).
  const consent = usePlanConsent(
    removal.grantId && userId ? { op: "remove", userId, grantId: removal.grantId } : null,
    "revoking it",
  );
  const foretell = useForetell();
  const [stale, setStale] = useState(false);
  // The give never reached Zitadel: the dialog is the cancel, said plainly, and
  // nothing is sent. The plan says so; until it is read, the source does.
  const held = useHeldAtPress({
    cancelOnly: consent.plan ? consent.plan.delivery === "none" : Boolean(source?.queued),
    cancels: consent.cancels,
    rules: consent.rules,
  });
  const { cancelOnly, cancels, rules } = held.view;
  const waitingGive = cancels.find((c) => c.op_type !== "revoke");
  const actLabel = cancelOnly ? "Cancel giving it" : "Revoke this role";
  const done = succeeded(outcome);
  const roleIn = `${shown.role(removal.projectId, removal.roleKey)} in ${removal.projectName}`;
  // A revoke that also cancels someone's waiting change asks first: the press
  // pushes the step, filled from the plan. Keep it comes back here as it was.
  const step = useCancelStep({
    host: `Revoke direct access to ${roleLabel}`,
    goAhead: "revoke",
    tone: cancelOnly ? "accent" : "danger",
  });

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="removal-title">
      <PushedStep step={step}>
      <ModalHeader
        chip={<SourceChip kind="direct" />}
        // Titled as the act: cancelling a give that never went is not a revoke.
        title={
          cancelOnly
            ? `Cancel giving ${roleIn} to ${first}?`
            : `Revoke ${roleIn}?`
        }
        titleId="removal-title"
        lede={
          done
            ? undefined
            : cancelOnly && waitingGive
              ? `Given to ${first} on ${formatShortDate(waitingGive.queued_at)}${
                  waitingGive.initiated_by_name ? ` by ${waitingGive.initiated_by_name}` : ""
                }, and still waiting for you to send.`
              : cancelOnly
                ? `Still waiting for you to send.`
                : `Given by hand to ${subject}. In Zitadel now.`
        }
      />
      {stale && !outcome && (
        <Notice
          tone="warn"
          blocking
          className="mx-6 mb-3"
          title={`Nothing was changed. ${first}'s access moved while this was open.`}
          next={`Read the note under the form, then press ${actLabel} again.`}
        >
          {null}
        </Notice>
      )}
      {!done && <div className="px-6">
        {cancelOnly ? (
          // Never delivered, so there is nothing at the door to close: no box,
          // because nothing is wrong (RULES). The removal does not queue a
          // revocation; it cancels the give that was still waiting.
          <p className="flex items-start gap-2.5 text-[15px] leading-[1.5]">
            <StatusDot tone="neutral" className="mt-[8px]" />
            <span>
              <span className="text-muted">
                It&rsquo;s still waiting for you to send, so {first} never had it. Nothing is sent to Zitadel.
              </span>
            </span>
          </p>
        ) : (
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
                  : auto === null
                    ? "" // Not known yet whether it goes on its own (N3): nothing promised.
                    : auto
                    ? "Revocations send on their own, within a minute."
                    : "The revocation waits in Pending changes until someone presses Send."}
              </span>
            </>
          )}
          {cancels.length > 0 && (
            <span className="mt-1.5 block text-[14px] text-muted">{foretell("Revoking it", cancels)}</span>
          )}
          {rules.length > 0 && (
            <span className="mt-1.5 block text-[14px] text-muted">{rules.join(" ")}</span>
          )}
        </div>
        )}
      </div>}
      {/* The first 100ms. Cancelling a give that is still waiting; a role that
          waits holds its own removal; anything else is simply removed. */}
      {!outcome && remove.isPending && (
        <ActionForecast
          className="mx-6 mb-1"
          word={cancelOnly ? FORECAST.cancel : forecastFor("revoke", roleWaits(removal.projectId, removal.roleKey))}
        />
      )}
      {outcome && <ActionOutcome word={word ?? resultWord(outcome, cancelOnly ? "cancel" : "revoke")} outcome={outcome} person={userId} className="mx-6 mb-1" />}
      {outcome && succeeded(outcome) && step.after}

      <ModalFooter
        // Beside "Cancel the waiting give", a dismiss called Cancel is a second
        // cancel: it says what it keeps, as the cancel step does.
        cancel={done ? undefined : { onClick: onClose, label: cancelOnly ? "Keep it waiting" : "Keep it" }}
        note={
          step.note ??
          // Two causes, and they were reported as one. "This role was not given
          // directly" is true when the grant id is missing; it is a fabrication
          // when the real problem is that the caller passed no person, which is
          // what the person page did for the whole life of this dialog. A note
          // that blames the data for a wiring fault sends an operator to check
          // the wrong thing.
          (!userId
            ? "Refused · Syndra could not tell which person this is. Reload the page."
            : grantListPending
              ? "Waiting · Syndra is still reading this person's direct grants, so it cannot yet tell whether there is one to revoke."
              : blocked
                ? "Refused · this role was not given directly, so there is no direct access to revoke. Only direct access can be revoked here."
                : outcome
                  ? undefined
                  : consent.failed
                    ? <RuleConsentFailed consent={consent} className="flex flex-wrap items-center gap-x-3 gap-y-1" />
                    : waits)
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
            variant={cancelOnly ? "accent" : "dangerConfirm"}
            disabled={blocked || consent.hold !== undefined}
            reason={blocked ? undefined : consent.hold}
            isPending={remove.isPending}
            onClick={async () => {
              held.hold();
              try {
                const result = await step.confirm(cancelOnly ? [] : cancels, () =>
                  remove.mutateAsync({ userId: userId!, grantId: removal.grantId!, seen: consent.seen }),
                );
                if (result === KEPT) {
                  held.release();
                  return;
                }
                setStale(false);
                if (cancelOnly) {
                  // The give leaves the queue; the rail follows the answer.
                  if (client) settleIndicators(client, { pending_propagation: -Math.max(1, cancels.length) });
                  setWord("Cancelled");
                  setOutcome({
                    kind: "applied",
                    message: `${roleIn} won't be given to ${first}. It never reached Zitadel.`,
                    next: "Nothing for you to do.",
                  });
                  return;
                }
                // The residual outcome, from the backend that computed it. A
                // role this person also holds through a bundle or a rule
                // survives the removal, and which ones those are is a closure
                // diff the server does — the UI must not hold a second opinion
                // about somebody's access.
                setOutcome(removalOutcome(result, roleLabel, shown, auto));
              } catch (error) {
                held.release();
                // Their access moved since the plan was read: nothing was written.
                if (isStale(error)) {
                  setStale(true);
                  consent.retry();
                  return;
                }
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {actLabel}
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

const firstName = (name: string) => name.trim().split(/\s+/)[0] || name;

/**
 * What the dialog read before the press, held from the press on (browser
 * pass, 8 Nov). The act's own refetch lands before its answer, and
 * re-deriving from it put "will lose this role" above "Cancelled" and turned
 * the lede into another story. Let go when the act did not go (kept, stale,
 * an error), so the next press reads the world afresh.
 */
function useHeldAtPress<T>(live: T): { view: T; hold: () => void; release: () => void } {
  const [held, setHeld] = useState<{ value: T } | null>(null);
  return { view: held ? held.value : live, hold: () => setHeld({ value: live }), release: () => setHeld(null) };
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
  const auto = useAutoRetry();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [word, setWord] = useState<string | null>(null);
  const first = firstName(subject);
  // What removing it does, from the planner before the press (A-21 §3.2).
  const consent = usePlanConsent(
    userId && source.bundle_id ? { op: "remove_bundle", userId, bundleId: source.bundle_id } : null,
    "removing it",
  );
  const foretell = useForetell();
  const [stale, setStale] = useState(false);
  // Nothing it carries reached them: the dialog is the cancel, and nothing is sent.
  const held = useHeldAtPress({
    cancelOnly: consent.plan ? consent.plan.delivery === "none" : Boolean(source.queued),
    cancels: consent.cancels,
  });
  const { cancelOnly, cancels } = held.view;
  const actLabel = cancelOnly ? "Cancel giving it" : "Remove the bundle";
  const done = succeeded(outcome);
  const step = useCancelStep({
    host: `Remove the ${bundleName} bundle from ${subject}`,
    goAhead: "remove",
    tone: cancelOnly ? "accent" : "danger",
  });

  return (
    <Modal open onClose={onClose} busy={removeBundle.isPending} size="sm" labelledBy="removal-title">
      <PushedStep step={step}>
      <ModalHeader
        chip={<SourceChip kind="bundle" />}
        title={cancelOnly ? `Cancel giving the ${bundleName} bundle to ${first}?` : `Remove the ${bundleName} bundle from ${subject}?`}
        titleId="removal-title"
        lede={
          done
            ? undefined
            : cancelOnly
            ? `${bundleName} is still waiting for you to send, so nothing it carries has reached ${subject} yet.`
            : `Everything ${bundleName} carries goes with it, except what another source also gives them.`
        }
      />
      {stale && !outcome && (
        <Notice
          tone="warn"
          blocking
          className="mx-6 mb-3"
          title={`Nothing was changed. ${first}'s access moved while this was open.`}
          next={`Read the note under the form, then press ${actLabel} again.`}
        >
          {null}
        </Notice>
      )}
      {!done && <div className="flex flex-col gap-2 px-6">
        {/* "They will lose" is a claim about an effect, and this bundle has not
            had one yet. The assignment was recorded and never sent, so removing
            it cancels the queued delivery rather than queueing its opposite —
            which is also what the backend now does, instead of enqueueing a
            revocation for access nobody ever received. */}
        <div className="type-label">{cancelOnly ? "Nothing to take back" : "They will lose"}</div>
        <div className="flex flex-wrap items-center gap-2.5 text-[14px]">
          <StatusDot tone={cancelOnly ? "neutral" : "danger"} />
          <RoleRef projectId={removal.projectId} roleKey={removal.roleKey} />
          {cancelOnly ? (
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
      </div>}
      {!outcome && removeBundle.isPending && (
        <ActionForecast
          className="mx-6 mb-1"
          word={cancelOnly ? FORECAST.cancel : forecastFor("remove", roleWaits(removal.projectId, removal.roleKey))}
        />
      )}
      {outcome && <ActionOutcome word={word ?? resultWord(outcome, cancelOnly ? "cancel" : "remove")} outcome={outcome} person={userId} className="mx-6 mb-1" />}
      {outcome && succeeded(outcome) && step.after}

      <ModalFooter
        cancel={done ? undefined : { onClick: onClose, label: cancelOnly ? "Keep it waiting" : "Keep it" }}
        // A bundle whose grants are still waiting says nothing more about them
        // here: it is the cancel. One that also cancels someone else's waiting
        // change says so before the press, and the press pushes the step.
        note={
          step.note ??
          (consent.failed ? (
            <RuleConsentFailed consent={consent} className="flex flex-wrap items-center gap-x-3 gap-y-1" />
          ) : cancelOnly ? undefined : (
            <>
              {(!outcome && waits) ||
                "Every other role this bundle carries is removed too. Manage bundles shows the full list before you commit."}
              {!outcome && cancels.length > 0 && (
                <span className="block">{foretell("Removing it", cancels)}</span>
              )}
            </>
          ))
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
            // Red is a promise about consequence. Cancelling a delivery that
            // never went out takes nothing from anybody, and dressing it as
            // destruction contradicts the sentence directly above it.
            variant={cancelOnly ? "accent" : "dangerConfirm"}
            isPending={removeBundle.isPending}
            disabled={!userId || !source.bundle_id || consent.hold !== undefined}
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
                  : consent.hold
            }
            onClick={async () => {
              held.hold();
              try {
                const result = await step.confirm(cancelOnly ? [] : cancels, () =>
                  removeBundle.mutateAsync({ bundleId: source.bundle_id!, seen: consent.seen }),
                );
                if (result === KEPT) {
                  held.release();
                  return;
                }
                setStale(false);
                if (cancelOnly) {
                  setWord("Cancelled");
                  setOutcome({
                    kind: "applied",
                    message: `The ${bundleName} bundle won't be given to ${first}. It never reached Zitadel.`,
                    next: "Nothing for you to do.",
                  });
                  return;
                }
                const waiting = result?.cascade?.enqueued ?? 0;
                if (waiting > 0) {
                  // What the server's delivery did with the revocations, in
                  // the words a retired bundle uses (lib/retry.ts
                  // retireOutcome): sent, refused (N1), retrying, or waiting.
                  const out = retireOutcome(waiting, result?.cascade?.wait_reason, result?.cascade?.drain?.retry, "", auto);
                  setOutcome({ ...out, message: `${bundleName} removed from ${subject}. ${out.message}` });
                  return;
                }
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
                      ? "Every role it carries is still given another way, so nothing is sent to Zitadel."
                      : "The roles it supplied are revoked once you send Pending changes, except any that another source still gives.",
                });
              } catch (error) {
                held.release();
                // Their access moved since the plan was read: nothing was written.
                if (isStale(error)) {
                  setStale(true);
                  consent.retry();
                  return;
                }
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {actLabel}
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
