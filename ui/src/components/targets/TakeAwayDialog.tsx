"use client";

import { useState } from "react";

import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
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
function sessionSentence(name: string): string {
  return `If they are connected right now, they stay connected until they disconnect — ${name} cannot end a session. Their next connection is refused.`;
}

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
  const name = targetLabel(target);

  if (result) {
    return (
      <Modal open onClose={onClose} size="md" labelledBy="takeaway-result">
        <ModalHeader
          titleId="takeaway-result"
          title={
            result.status === "revoked"
              ? "Access revoked"
              : "Partly done — one step is still outstanding"
          }
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
          <Button onClick={onClose}>Close</Button>
        </ModalFooter>
      </Modal>
    );
  }

  return (
    <Modal open onClose={revoke.isPending ? () => {} : onClose} busy={revoke.isPending} size="md" labelledBy="takeaway-title">
      <ModalHeader
        titleId="takeaway-title"
        title={`Revoke ${subjectName}'s access on ${name}`}
        lede={`Revoke (end their access): ${subjectName} can no longer sign in to ${name}. They keep every role they hold — only what those roles give them on ${name} is put on hold.`}
      />

      <div className="grid gap-4 px-6">
        <div className="grid gap-1.5 text-[13.5px] text-muted">
          <p className="font-semibold text-ink">What happens</p>
          <ul className="list-disc pl-5">
            <li>Their storage password on {name} is replaced now.</li>
            <li>New connections to {name} are refused.</li>
            <li>Their roles stay as they are. Only what those roles give them on {name} is on hold, until somebody lifts it.</li>
          </ul>
          <p>To pause without replacing the password, use Put on hold instead.</p>
        </div>

        {/* Amber, and above the form rather than beside the button: it is what
            the reader needs to know BEFORE deciding, not a caveat attached to
            the decision. */}
        <p className="rounded-inner border border-warn-line bg-warn-soft px-4 py-3 text-[13.5px] text-warn-text">
          {sessionSentence(name)}
        </p>

        <div>
          <FieldLabel htmlFor="takeaway-reason">Why</FieldLabel>
          <Input
            id="takeaway-reason"
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
          <FieldHint>
            Shown to them on their own page, and wherever their access shows as on hold.
          </FieldHint>
        </div>

        <div>
          <FieldLabel htmlFor="takeaway-review-date">Look at this again by</FieldLabel>
          <Input
            id="takeaway-review-date"
            type="date"
            value={reviewDate}
            onChange={(e) => setReviewDate(e.target.value)}
          />
          <FieldHint>
            The hold stays until somebody lifts it. From this date Syndra reminds you to
            look at it again. Leave it empty and Syndra picks a date.
          </FieldHint>
        </div>

        <ConfirmByTyping
          expected={subjectName}
          noun="person's name"
          value={confirm.typed}
          onChange={confirm.setTyped}
          disabled={revoke.isPending}
        />

        {revoke.error && (
          <p className="text-[13.5px] text-danger-text">
            {revoke.error instanceof Error
              ? revoke.error.message
              : "That did not go through. Nothing was changed."}
          </p>
        )}
      </div>

      <ModalFooter
        note={`The storage password is replaced now. The hold is recorded now and reaches ${name} when someone sends it from Pending changes.`}
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
          {revoke.isPending ? "Revoking…" : `Revoke access for ${subjectName}`}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={revoke.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}
