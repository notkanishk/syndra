"use client";

import { useState } from "react";

import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { targetLabel } from "@/lib/nav";
import { useRevokeTargetAccess, type RevocationResult } from "@/lib/queries/useHolds";
import { useIsTouch } from "@/lib/useViewport";

/**
 * Taking somebody's access away on a target (6.17; design §27).
 *
 * This target has no way to end a session. There is no close, no disconnect and
 * no session list — so "revoke" cannot mean what an operator reasonably assumes
 * it means, and the backend composes the action out of the only two things that
 * CAN be done: a hold on the lifecycle field, and a credential rotation.
 *
 * **The sentence about sessions is fixed by the backend and shown verbatim.** A
 * UI implying the access is gone the moment the button is pressed is the exact
 * failure that endpoint's whole design exists to prevent.
 *
 * Three deliberate choices, all of them about not lying:
 *
 *   - It is dressed as a revocation, because it is one: rung 3, typed name, and
 *     the one solid red fill in the product. Muscle memory must not depend on
 *     which revocation an operator is doing.
 *   - The session sentence is AMBER, not red. It is a broken assumption —
 *     *revoked* implies immediate and here it is not — rather than the danger
 *     itself. Red is the confirming button.
 *   - The label is "Take away", not "Revoke". It says what happens to the
 *     person, and *revoke* already carries the meaning of undoing a grant
 *     Syndra itself made.
 */

/** Held here, not composed, because it is a statement about what the system does. */
const SESSION_SENTENCE =
  "Sessions already established end when they next reconnect — this target has no way to close one.";

export function TakeAwayDialog({
  target,
  subjectId,
  subjectName,
  onClose,
}: {
  target: string;
  subjectId: string;
  /** What the operator types to confirm. Their name, not their id. */
  subjectName: string;
  onClose: () => void;
}) {
  const revoke = useRevokeTargetAccess(target, subjectId);
  const touch = useIsTouch();
  const [reason, setReason] = useState("");
  const [reviewDate, setReviewDate] = useState("");
  const confirm = useTypedConfirmation(subjectName);
  const [result, setResult] = useState<RevocationResult | null>(null);

  const armed = confirm.armed && reason.trim() !== "" && !revoke.isPending;

  if (result) {
    return (
      <Modal open onClose={onClose} size="md" labelledBy="takeaway-result">
        <ModalHeader
          titleId="takeaway-result"
          title={result.status === "revoked" ? "Access taken away" : "Half of it went through"}
          lede={result.detail}
        />
        {result.disclosure && (
          <div className="px-6">
            <p className="rounded-inner border border-accent-line bg-accent-soft px-4 py-3 text-[13.5px] text-muted">
              {result.disclosure}
            </p>
          </div>
        )}
        {result.outstanding && (
          // Named so a surface can offer the one remaining action rather than
          // the whole composition again — and so an operator knows there IS one.
          <div className="px-6 pt-3">
            <p className="text-[13.5px] text-warn-text">
              Still outstanding: {result.outstanding}.
            </p>
          </div>
        )}
        <ModalFooter>
          <Button variant="accent" onClick={onClose}>
            Close
          </Button>
        </ModalFooter>
      </Modal>
    );
  }

  return (
    <Modal open onClose={revoke.isPending ? () => {} : onClose} busy={revoke.isPending} size="md" labelledBy="takeaway-title">
      <ModalHeader
        titleId="takeaway-title"
        title={`Take ${subjectName}'s access away on ${targetLabel(target)}`}
        lede={`New connections are refused and their credential is replaced. They keep every role they hold — this holds what the roles reach, on this target only.`}
      />

      <div className="grid gap-4 px-6">
        {/* Amber, and above the form rather than beside the button: it is what
            the operator needs to know BEFORE deciding, not a caveat attached to
            the decision. */}
        <p className="rounded-inner border border-warn-line bg-warn-soft px-4 py-3 text-[13.5px] text-warn-text">
          {SESSION_SENTENCE}
        </p>

        <label className="grid gap-1.5 text-[14px]">
          <span>Why</span>
          <Input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="What happened — this is what somebody reads six months from now"
            // Not on touch. Focusing on mount opens the keyboard the instant
            // the sheet rises, which covers the sentence saying what this does
            // to somebody's access — and that sentence is the most protected
            // element on the screen. On a desktop there is no keyboard to
            // raise and no reason to make the operator reach for the mouse.
            autoFocus={!touch}
          />
          <span className="text-[13px] text-faint">
            Shown to them on their own page, and on every surface where they still
            appear as held.
          </span>
        </label>

        <label className="grid gap-1.5 text-[14px]">
          <span>Look at this again by</span>
          <Input type="date" value={reviewDate} onChange={(e) => setReviewDate(e.target.value)} />
          <span className="text-[13px] text-faint">
            A hold does not lapse on its own — it stays until somebody lifts it. This is
            the date it starts asking. Left empty, one is set for you.
          </span>
        </label>

        <ConfirmByTyping
          expected={subjectName}
          noun="person's name"
          value={confirm.typed}
          onChange={confirm.setTyped}
          disabled={revoke.isPending}
        />

        {revoke.error && (
          <p className="text-[13.5px] text-danger-text">
            {revoke.error instanceof Error ? revoke.error.message : "That could not be applied."}
          </p>
        )}
      </div>

      <ModalFooter
        note="The hold is recorded here and reaches the target when the queue is resumed. The credential is replaced now."
      >
        <Button
          variant="dangerConfirm"
          disabled={!armed}
          onClick={() =>
            revoke.mutate(
              {
                reason,
                reviewDate: reviewDate ? new Date(reviewDate).toISOString() : undefined,
              },
              { onSuccess: setResult },
            )
          }
        >
          {/* What it does, not what it means. The queue is the honest verb. */}
          {revoke.isPending ? "Queueing…" : "Queue the revocation"}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={revoke.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}
