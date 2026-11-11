"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";

import { describeFailure, sentence } from "@/lib/outcome";
import { useState } from "react";

import { ConfirmByTyping, useTypedConfirmation } from "@/components/ui/Acknowledge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import { useRevokeTargetAccess, type RevocationResult } from "@/lib/queries/useHolds";
import { useIsTouch } from "@/lib/useViewport";
import { endOfDayISO } from "@/lib/time";

/**
 * Revoking somebody's access on a target (6.17; design §27).
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
 *   - The session sentence is AMBER, not red. It is a broken assumption:
 *     *revoked* implies immediate and here it is not, rather than the danger
 *     itself. Red is the confirming button.
 *   - The label is "Revoke". The product says it on every other screen that
 *     ends access, and one word for one action beats a finer distinction only
 *     this file draws.
 */

/** Held here, not composed, because it is a statement about what the system does. */
function sessionSentence(name: string): string {
  return `If they are connected right now, they stay connected until they disconnect. ${name} cannot end a session. Their next connection is refused.`;
}

export function RevokeDialog({
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
  const nameOf = useSystemNames();
  const revoke = useRevokeTargetAccess(target, subjectId);
  const auto = useAutoRetry();
  const touch = useIsTouch();
  const [reason, setReason] = useState("");
  const [reviewDate, setReviewDate] = useState("");
  const confirm = useTypedConfirmation(subjectName);
  const [result, setResult] = useState<RevocationResult | null>(null);

  const armed = confirm.armed && reason.trim() !== "" && !revoke.isPending;
  const name = nameOf(target);

  return (
    <Modal open onClose={revoke.isPending ? () => {} : onClose} busy={revoke.isPending} size="md" labelledBy="revoke-title">
      <ModalHeader
        titleId="revoke-title"
        title={`Revoke ${subjectName}'s access on ${name}`}
        lede={result ? undefined : `Revoke ends their access: ${subjectName} can no longer sign in to ${name}.`}
      />

      {/* The sheet is its result once it has one: the lede, the list, the form and
          the typed name unmount with the body; the header keeps its title. */}
      {result ? (
        <div className="grid gap-3 px-6">
          <ActionOutcome
            word={result.status === "revoked" ? "Revoked" : "Partly done"}
            outcome={{
              kind: result.status === "revoked" ? "applied" : "queued",
              message: result.detail ?? "",
              detail: result.disclosure,
              ...(result.outstanding ? { next: `Still outstanding: ${result.outstanding}.` } : {}),
            }}
          />
        </div>
      ) : (
      <div className="grid gap-4 px-6">
        <div className="grid gap-1.5 text-[13.5px] text-muted">
          <p className="font-semibold text-ink">What happens</p>
          <ul className="list-disc pl-5">
            <li>Their storage password on {name} is replaced now.</li>
            <li>New connections to {name} are refused.</li>
            <li>{sessionSentence(name)}</li>
            <li>Their roles stay as they are. Only what those roles give them on {name} is on hold, until somebody lifts it.</li>
          </ul>
          <p>To pause without replacing the password, use Put on hold instead.</p>
        </div>

        <div>
          <FieldLabel htmlFor="revoke-reason">Why</FieldLabel>
          <Input
            id="revoke-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="What happened"
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
          <FieldLabel htmlFor="revoke-review-date">Look at this again by</FieldLabel>
          <Input
            id="revoke-review-date"
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
            {sentence(describeFailure(revoke.error))}
          </p>
        )}
      </div>
      )}

      {result ? (
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </ModalFooter>
      ) : (
      <ModalFooter
        cancel={{ label: "Keep it", onClick: onClose, disabled: revoke.isPending }}
        note={
          auto === null
            ? "It is listed in Pending changes until it is sent."
            : auto
            ? "Revocations send on their own, within a minute, not from Pending changes."
            : "This Syndra server does not send revocations on its own: it waits in Pending changes until someone presses Send."
        }
      >
        <Button
          variant="dangerConfirm"
          disabled={!armed}
          isPending={revoke.isPending}
          reason={
            !armed && !revoke.isPending
              ? !reason.trim()
                ? "Say why. The reason is recorded with the revoke."
                : !confirm.armed
                  ? `Type ${subjectName} to confirm.`
                  : undefined
              : undefined
          }
          onClick={() =>
            revoke.mutate(
              {
                reason,
                reviewDate: reviewDate ? endOfDayISO(reviewDate) : undefined,
              },
              { onSuccess: setResult },
            )
          }
        >
          {`Revoke access for ${subjectName}`}
        </Button>
      </ModalFooter>
      )}
    </Modal>
  );
}
