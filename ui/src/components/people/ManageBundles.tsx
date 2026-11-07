"use client";

import { useMemo, useState } from "react";

import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { Button } from "@/components/ui/Button";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { useWaitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { KEPT, PushedStep, useCancelStep, useForetell } from "@/components/ui/CancelQueuedStep";
import { Notice } from "@/components/ui/Notice";
import { RuleConsentFailed } from "@/components/requests/RuleConsent";
import { isStale, usePlanConsents, type ActPlan } from "@/lib/usePlanConsent";
import { DRAIN_UNKNOWN_NEXT, outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { refusedOutcome } from "@/lib/retry";
import { isRefused } from "@/lib/wait-reason";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { useBundleRolesByBundle, useBundles, useRemoveBundle } from "@/lib/queries/useBundles";
import { useAssignBundle } from "@/lib/queries/useUsers";

/**
 * E4 · Assign / unassign a bundle.
 *
 * The preview is the body of the panel, not a footnote: bundles expand to
 * roles and rules cascade further, so "what would this actually grant" has to
 * be answerable before the click, above the fold.
 *
 * Unassigning shows the same preview in reverse and distinguishes roles that
 * will actually be lost from roles retained through another source.
 *
 * It reads the PUBLISHED version of each bundle, and that is not a detail. An
 * assignment pins the latest published version and nothing else — the backend
 * says so at the read (`db.LatestVersionRoles`: "Not the working copy either.
 * Reading `bundle_roles` would project unpublished edits to somebody who is not
 * pinned to them") and a test pins the apply side to it. This panel asked for
 * the working copy, so it listed every unpublished edit as a role the person
 * was about to receive, and the apply then granted the published set. An
 * operator saw the roles they had just added promised here and called
 * unpublished on the bundle screen, which is two screens disagreeing about the
 * same fact.
 */
function ManageBundlesOpen({
  userId,
  userName,
  assigned,
  onClose,
}: {
  userId: string;
  userName: string;
  assigned: Array<{ id: string; name: string }>;
  open: boolean;
  onClose: () => void;
}) {
  const bundles = useBundles();
  const assign = useAssignBundle(userId);
  const remove = useRemoveBundle(userId);

  const assignedIds = useMemo(() => new Set(assigned.map((b) => b.id)), [assigned]);
  const [staged, setStaged] = useState<Set<string>>(new Set());
  const waits = useWaitsBeforeLine("change");
  const auto = useAutoRetry();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // One step for every staged change: its confirm names what goes ahead,
  // "Cancel all 3 and apply 2 changes" (A-21 §3.2).
  const onlyRemoving = staged.size > 0 && Array.from(staged).every((id) => assignedIds.has(id));
  const step = useCancelStep({
    host: `Manage bundles for ${userName}`,
    goAhead: `apply ${staged.size} ${staged.size === 1 ? "change" : "changes"}`,
    tone: onlyRemoving ? "danger" : "accent",
  });

  const allIds = (bundles.data ?? []).map((bundle) => bundle.id);
  const { byId: bundleRoles } = useBundleRolesByBundle(allIds, { published: true });

  const selected = useMemo(() => {
    const next = new Set(assignedIds);
    staged.forEach((id) => (next.has(id) ? next.delete(id) : next.add(id)));
    return next;
  }, [assignedIds, staged]);

  const changes = Array.from(staged);
  // What each staged change does for this person, from the planner before the
  // press (A-21 §3.2): the row's words, rules followed, whether it goes now or
  // waits, and the waiting changes it cancels. One source with the bulk plan.
  const consents = usePlanConsents(
    changes.map((id) => ({ op: assignedIds.has(id) ? "remove_bundle" : "assign_bundle", userId, bundleId: id })),
    "this",
  );
  const hold = consents.find((c) => c.hold)?.hold;
  const unread = consents.find((c) => c.failed);
  const refusedRow = consents.find((c) => c.plan && c.plan.effect !== "apply")?.plan;
  // Every staged change's cancels, gathered into one ask before the first write.
  const cancels = consents.flatMap((c) => c.cancels).filter((c, i, all) => all.findIndex((d) => d.id === c.id) === i);
  const foretell = useForetell();
  const [stale, setStale] = useState(false);
  const first = userName.trim().split(/\s+/)[0] || userName;

  async function apply() {
    // What is WAITING afterwards, summed from the backend rather than assumed
    // from the number of boxes ticked: a change that only cancels a waiting
    // one, or that reaches nobody's effective access, queues nothing.
    let waiting = 0;
    let revoking = 0;
    let sent = 0;
    let applied = 0;
    let refused: { reason?: string; removing: boolean } | undefined;
    let movedAt = -1;
    const write = async () => {
      for (const [i, id] of changes.entries()) {
        const removing = assignedIds.has(id);
        const seen = consents[i]?.seen;
        let result;
        try {
          result = await (removing ? remove.mutateAsync({ bundleId: id, seen }) : assign.mutateAsync({ bundleId: id, seen }));
        } catch (error) {
          // Their access moved since the plans were read: this one and the
          // rest are not written.
          if (isStale(error)) movedAt = i;
          throw error;
        }
        applied++;
        // Out of the staged set as soon as it is applied. Left in, a retry after
        // a later one fails would read it against the new state and send the
        // opposite (an assign becomes a remove).
        setStaged((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
        const cascade = result?.cascade;
        const enqueued = cascade?.enqueued ?? 0;
        if (isRefused(cascade?.wait_reason)) {
          // Final verification N1: terminal, never "waits" or "went".
          refused ??= { reason: cascade?.wait_reason, removing };
          continue;
        }
        // The planner decided delivery: no wait reason means every row went.
        if (!cascade?.wait_reason) {
          sent += enqueued;
          continue;
        }
        waiting += enqueued;
        // With the background drain off a revocation waits for Send like the
        // rest, and a role that waits holds one too.
        if (removing && auto === true && cascade.wait_reason !== "role_waits") revoking += enqueued;
      }
    };
    try {
      // One ask for every waiting change the staged changes cancel (A-21 §3.2).
      if ((await step.confirm(cancels, write)) === KEPT) return;
      setStale(false);
    } catch (error) {
      if (movedAt < 0) {
        setOutcome(outcomeFromError(error));
        return;
      }
      setStale(true);
      consents.forEach((c) => c.retry());
      if (applied === 0) return;
    }
    const recorded = applied === 1 ? "One bundle change recorded" : `${applied} bundle changes recorded`;
    if (refused) {
      setOutcome(refusedOutcome(recorded, refused.reason, { revocation: refused.removing }));
      return;
    }
    if (waiting === 0 && sent > 0) {
      setOutcome({ kind: "applied", message: recorded, detail: `Sent to Zitadel: ${sent === 1 ? "one change" : `${sent} changes`}.` });
      return;
    }
    setOutcome({
      // Amber "waiting" only where something waits. A change that queued
      // nothing is done, and painting it as pending sends an operator to an
      // empty Pending changes to look for it.
      kind: waiting > 0 ? "queued" : "applied",
      // Unknown drain (N3): no promise, and no Send link for a revocation that may go on its own.
      ...(waiting > 0 && auto === null ? { sendsItself: true, next: DRAIN_UNKNOWN_NEXT } : {}),
      message: recorded,
      // Revocations do not wait for a person — they send on their own within a
      // minute — so telling the operator to go and send them would be
      // false, and saying nothing about it would leave them wondering why
      // Pending changes emptied by itself.
      detail:
        waiting > 0
          ? `${sent > 0 ? `${sent === 1 ? "One change" : `${sent} changes`} went to Zitadel. ` : ""}${
              waiting === 1 ? "One change waits" : `${waiting} changes wait`
            } under Pending changes${
              auto === null
                ? ", listed there until each reaches Zitadel."
                : revoking === waiting
                  ? ". Revocations send on their own within a minute, or send them now."
                  : revoking > 0
                    ? `; the ${revoking === 1 ? "revocation sends" : `${revoking} revocations send`} on their own within a minute, the rest until you send them.`
                    : " until you send them."
            }`
          : "Nothing is waiting for you to send: this left no change for Zitadel or a connected system to carry out. Pending changes is unaffected.",
    });
  }

  const busy = assign.isPending || remove.isPending;
  const staging = changes.length;
  // Recorded, and nothing new ticked since. The modal has nothing left to do.
  const done = (outcome?.kind === "queued" || outcome?.kind === "applied") && staging === 0;

  return (
    <Modal open onClose={onClose} busy={busy} size="md" labelledBy="manage-bundles-title">
      <PushedStep step={step}>
      <ModalHeader title="Manage bundles" titleId="manage-bundles-title" />
      <div className="-mt-2 px-6 pb-1 text-[13px] text-faint">{userName}</div>

      <div className="flex flex-col gap-1.5 px-6">
        {(bundles.data ?? []).map((bundle) => {
          const isSelected = selected.has(bundle.id);
          const roleCount = (bundleRoles[bundle.id] ?? []).length;
          return (
            <ChoiceRow
              key={bundle.id}
              selected={isSelected}
              onToggle={() =>
                setStaged((prev) => {
                  const next = new Set(prev);
                  if (next.has(bundle.id)) next.delete(bundle.id);
                  else next.add(bundle.id);
                  return next;
                })
              }
              title={bundle.name}
              meta={`${roleCount} ${roleCount === 1 ? "role" : "roles"}${assignedIds.has(bundle.id) ? " · assigned" : ""}`}
              description={bundle.is_welcome ? "Given to new members" : undefined}
            />
          );
        })}
      </div>

      {stale && !outcome && (
        <Notice
          tone="warn"
          blocking
          className="mx-6 mt-4"
          title={`Nothing was changed. ${first}'s access moved while this was open.`}
          next={`Read the note under each bundle, then press Apply again.`}
        >
          {null}
        </Notice>
      )}

      {changes.length > 0 && (
        <div className="mx-6 mt-5 flex flex-col gap-3.5 border-t border-line pt-4">
          {changes.map((id, i) => {
            const bundle = (bundles.data ?? []).find((b) => b.id === id);
            if (!bundle) return null;
            const adding = !assignedIds.has(id);
            const plan = consents[i]?.plan;
            const empty = adding && (bundleRoles[id] ?? []).length === 0;
            // The planner's words for this change, rules followed (A-21 §3.2):
            // the same sentences a bulk plan's row says.
            const words = plan
              ? [plan.detail, ...(plan.effect === "apply" ? plan.said : [plan.consequence ?? ""])].filter(Boolean).join(" ")
              : consents[i]?.checking
                ? consents[i]?.hold
                : undefined;
            return (
              <div key={id} className="flex flex-col gap-1.5">
                <div className="type-label">{bundle.name}</div>
                {empty ? (
                  <span className="text-[14px] text-faint">
                    {bundle.latest_version
                      ? `v${bundle.latest_version} of this bundle carries no roles, so this gives nothing.`
                      : "This bundle carries no roles yet."}
                  </span>
                ) : (
                  words && <p className={`text-[14px] leading-[1.5] ${plan ? "" : "text-faint"}`}>{words}</p>
                )}
                {/*
                  Said here, on the screen that would otherwise be contradicted. A
                  bundle with unpublished edits grants its published version, so
                  what this says is right and the bundle screen's role list is a
                  different set.
                */}
                {adding && (bundle.unpublished_changes ?? 0) > 0 && (
                  <p className="text-[13px] leading-[1.5] text-muted">
                    {bundle.name} has {bundle.unpublished_changes} unpublished{" "}
                    {bundle.unpublished_changes === 1 ? "change" : "changes"}. Those are not part of this. An
                    assignment gives v{bundle.latest_version}, and publishing is what decides whether the people
                    holding it move.
                  </p>
                )}
              </div>
            );
          })}
          {unread && <RuleConsentFailed consent={unread} />}
        </div>
      )}

      {outcome && <ActionOutcome word={outcome.recorded ? "Refused by Zitadel" : "Recorded"} outcome={outcome} className="mx-6 mb-1" />}
      {outcome && step.after}

      {/* The footer follows the state of the work, because the work has three
          states and this used to describe only the middle one.

          "Queues for confirmation" sat permanently in the corner — true while
          something is staged, and a claim about a button that cannot be pressed
          both before anything is ticked and after the change has been recorded.
          Beside it the primary read "No changes": a status wearing a button's
          clothes, disabled, giving no reason, and still first in the tab order
          while the only live control was the secondary one next to it. */}
      <ModalFooter
        note={
          step.note ??
          ((!outcome && waits) ||
            (staging > 0 && !hold ? (
              // Whether this finishes now or waits, from the plans (A-21 §3.2),
              // and what it cancels, before the press.
              <>
                <span className="block">{bundlesBeforeLine(consents.map((c) => c.plan))}</span>
                {cancels.length > 0 && (
                  <span className="block">
                    {foretell("Applying", cancels)} Nothing is cancelled until you choose on the next step.
                  </span>
                )}
              </>
            ) : undefined))
        }
        cancel={done ? undefined : { onClick: onClose }}
      >
        {done ? (
          // The work is recorded and the outcome above says so. The next action
          // is to leave, so it stops being the quiet button in the corner.
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            variant="accent"
            disabled={staging === 0 || hold !== undefined || refusedRow !== undefined}
            isPending={busy}
            onClick={apply}
            // Never a bare disabled control. The label stays an ACTION at
            // every count: a button labelled with its own emptiness reads
            // as broken rather than as waiting for you.
            reason={
              staging === 0
                ? outcome
                  ? "Recorded. Tick another bundle to make a further change."
                  : "Tick a bundle above to add or remove it."
                : (hold ??
                  (refusedRow ? [refusedRow.detail, refusedRow.consequence].filter(Boolean).join(" ") : undefined))
            }
          >
            {staging === 0 ? "Apply changes" : `Apply ${staging} ${staging === 1 ? "change" : "changes"}`}
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

export const ManageBundles = mountedWhileOpen(ManageBundlesOpen);

/**
 * Whether the staged changes finish now or wait, from their plans (A-21 §3.2,
 * the bulk before-line's shapes): "Apply sends 2 changes to Zitadel now. 1
 * waits for you to send." A change that only cancels a waiting one sends
 * nothing, and says so.
 */
export function bundlesBeforeLine(plans: Array<ActPlan | undefined>): string {
  const going = plans.filter((p): p is ActPlan => p?.effect === "apply");
  const now = going.filter((p) => p.delivery === "straight_away").length;
  const wait = going.filter((p) => p.delivery === "waits_for_you").length;
  const cancelOnly = going.filter((p) => p.delivery === "none").length;
  if (going.length === 1 && (now || wait)) return now ? "This goes to Zitadel when you apply." : "This waits for you to send.";
  const parts: string[] = [];
  if (now) parts.push(`Apply sends ${now} ${now === 1 ? "change" : "changes"} to Zitadel now.`);
  if (wait) parts.push(wait === going.length ? "Every change waits for you to send." : `${wait} ${wait === 1 ? "waits" : "wait"} for you to send.`);
  if (cancelOnly) parts.push(`For ${cancelOnly}, this only cancels ${cancelOnly === 1 ? "a change that's waiting" : "changes that are waiting"}; nothing is sent.`);
  return parts.join(" ") || "Nothing here is sent to Zitadel.";
}
