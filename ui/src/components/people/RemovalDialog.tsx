"use client";

import { useState } from "react";

import {
  SourceChip,
  sourceQualifier,
  type RoleReason,
  type SourceKind,
} from "@/components/access/AccessSource";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useRemoveDirectGrant } from "@/lib/queries/useRoleMembers";
import { useRemoveBundle } from "@/lib/queries/useBundles";
import { humanizeKey } from "@/lib/format";
import {
  outcomeFromError,
  statesNothingChanged,
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

  if (!removal) return null;

  const subject = removal.userName ?? userName ?? "this person";
  const person = removal.userId ?? userId;
  const roleLabel = `${removal.projectName} / ${removal.roleKey}`;

  // With more than one source, the menu lists ONE entry per source, each named
  // after its own removal — never a single control that guesses.
  if (removal.sources.length > 1 && !chosen) {
    return (
      <Modal open onClose={onClose} size="sm">
        <ModalHeader
          title={`${subject} holds ${roleLabel} ${removal.sources.length} ways.`}
          lede="Each source is removed on its own terms. Pick the one you mean."
        />
        <div className="flex flex-col gap-2 px-6">
          {removal.sources.map((source) => (
            <button
              key={source.kind}
              type="button"
              onClick={() => setChosen(source.kind as SourceKind)}
              className="flex items-center gap-3 rounded-inner border border-line-strong px-4 py-3 text-left motion-tint hover:bg-[var(--hover)]"
            >
              <SourceChip kind={source.kind as SourceKind} />
              <span className="flex-1 text-[14.5px]">{actionName(source.kind as SourceKind)}</span>
              {sourceQualifier(source) && (
                <span className="text-[13px] text-faint">{sourceQualifier(source)}</span>
              )}
            </button>
          ))}
        </div>
        <ModalFooter>
          <Button onClick={onClose}>Cancel</Button>
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

/**
 * Whether the action ran. A FAILED attempt keeps its button — retiring the
 * control on any outcome at all would leave a refusal on screen with no way to
 * try again.
 */
function succeeded(outcome: Outcome | null): boolean {
  return outcome !== null && !statesNothingChanged(outcome.kind);
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const retained = others.length > 0;
  const roleLabel = `${removal.projectName} / ${removal.roleKey}`;

  // `grantsResolved === false` is the third state, and the reason this is not
  // a boolean: "no grant id" and "no answer yet" are different facts and the
  // operator needs to be told which one they are looking at.
  const grantListPending = removal.grantsResolved === false;
  const blocked = !removal.grantId || !userId;

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="direct" />}
        title={`Revoke direct access to ${roleLabel}?`}
        titleId="removal-title"
        lede={`Granted directly to ${subject}.`}
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
                This grant is still waiting to be sent, so {subject} never received it. Removing it withdraws the queued change; nothing reaches Zitadel.
              </span>
            </>
          ) : (
            <>
              <strong className="font-semibold text-danger-text">
                {subject} will lose this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                No bundle and no rule gives it to them. Their access ends once the change reaches Zitadel — revocations send on their own, every few minutes.
              </span>
            </>
          )}
        </div>
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
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
                : undefined
        }
      >
        {/* Same rule as the bundle dialog: an action that has run is not an
            action any more, and leaving it armed lets a second click revoke
            again. */}
        {!succeeded(outcome) && (
          <Button
            variant={source?.queued ? "accent" : "dangerConfirm"}
            disabled={blocked}
            isPending={remove.isPending}
            onClick={async () => {
              try {
                const result = await remove.mutateAsync({
                  userId: userId!,
                  grantId: removal.grantId!,
                });
                // The residual outcome, from the backend that computed it. A
                // role this person also holds through a bundle or a rule
                // survives the removal, and which ones those are is a closure
                // diff the server does — the UI must not hold a second opinion
                // about somebody's access.
                const retained = result?.retained_roles ?? [];
                setOutcome({
                  kind: "applied",
                  message: `Direct access to ${roleLabel} removed`,
                  detail: retained.length
                    ? `They still hold ${retained.join(", ")}, from a bundle or a rule.`
                    : "Nothing else was supplying it, so the access is gone.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {source?.queued ? "Withdraw the queued grant" : "Revoke access"}
          </Button>
        )}
        <Button variant={succeeded(outcome) ? "accent" : "outline"} onClick={onClose}>
          {succeeded(outcome) ? "Done" : "Cancel"}
        </Button>
      </ModalFooter>
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  return (
    <Modal open onClose={onClose} busy={removeBundle.isPending} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="bundle" />}
        title={`Remove the ${bundleName} bundle from ${subject}?`}
        titleId="removal-title"
        lede={
          source.queued
            ? `${bundleName} is still waiting to be sent, so nothing it carries has reached ${subject} yet.`
            : `Everything ${bundleName} carries goes with it, except what another source also gives them.`
        }
      />
      <div className="flex flex-col gap-2 px-6">
        {/* "They will lose" is a claim about an effect, and this bundle has not
            had one yet. The assignment was recorded and never sent, so removing
            it withdraws the queued delivery rather than queueing its opposite —
            which is also what the backend now does, instead of enqueueing a
            revocation for access nobody ever received. */}
        <div
          className={`text-[12.5px] font-semibold uppercase tracking-[0.1em] ${
            source.queued ? "text-label" : "text-danger-text"
          }`}
        >
          {source.queued ? "Nothing to take back" : "They will lose"}
        </div>
        <div
          className={`rounded-nav px-3.5 py-2.5 text-[14px] ${
            source.queued ? "bg-tint-1" : "bg-danger-soft"
          }`}
        >
          {removal.projectName} / <Mono>{removal.roleKey}</Mono>
          {source.queued ? (
            <span className="text-[13px] text-muted"> — recorded, never sent</span>
          ) : (
            others.length === 0 && (
              <span className="text-[13px] text-muted"> — no other source gives it</span>
            )
          )}
        </div>

        {others.length > 0 && (
          <>
            <div className="mt-2 type-label">They will keep</div>
            <div className="flex items-center gap-2.5 rounded-nav bg-accent-soft px-3.5 py-2.5 text-[14px]">
              {removal.projectName} / <Mono>{removal.roleKey}</Mono>
              <span className="text-[13px] text-muted">— still {describeOthers(others)}</span>
            </div>
          </>
        )}
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          source.queued
            ? "Every other role this bundle carries is withdrawn too, and none of them had been sent. Pending changes will be empty afterwards, not full of reversals."
            : "Every other role this bundle carries is removed too. Manage bundles shows the full list before you commit."
        }
      >
        {/* Gone once it has run. A destructive confirm that stays armed after
            it succeeded fires again on the next click — which is what a person
            does when the dialog does not close, and each press queued another
            cascade. What remains is the one action still true: leave. */}
        {!succeeded(outcome) && (
          <Button
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
                const result = await removeBundle.mutateAsync(source.bundle_id!);
                const waiting = result?.cascade?.enqueued ?? 0;
                setOutcome({
                  // `queued` paints an amber "Waiting to be sent" badge, and a
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
        <Button
          variant={succeeded(outcome) ? "accent" : "outline"}
          onClick={onClose}
        >
          {succeeded(outcome) ? "Done" : "Cancel"}
        </Button>
      </ModalFooter>
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
        lede={`${subject} holds ${removal.projectName} / ${removal.roleKey} because an automatic rule produced it.`}
      />
      <div className="flex flex-col gap-2.5 px-6">
        <div className="rounded-inner bg-tint-1 px-4 py-3.5 text-[14.5px] leading-[1.5]">
          <span className="text-muted">{input ?? "an input role"}</span>
          &nbsp;⇒&nbsp;
          <span className="font-semibold">
            {removal.projectName} / {humanizeKey(removal.roleKey)}
          </span>
          <div className="mt-1.5 text-[13.5px] text-faint">
            Editing the rule changes access for everyone it applies to.
          </div>
        </div>
        <p className="text-[14px] leading-[1.55] text-muted">
          To revoke this from {subject} alone, remove their{" "}
          <span className="text-ink">{input ?? "input role"}</span> — that is the input the rule
          reads.
        </p>
      </div>
      <ModalFooter>
        <ButtonLink href="/policies" variant="accentSoft">
          Open the rule →
        </ButtonLink>
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
      return "granted directly";
    })
    .join(" and ");
}
