"use client";

import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { targetLabel } from "@/lib/nav";
import { NOTHING_CHANGED } from "@/lib/outcome";
import { useCreateHold } from "@/lib/queries/useHolds";

/**
 * Putting a hold on something a role grants (9.22; design §25).
 *
 * A hold takes away access a role still confers, without touching the role. It
 * is the same object the member reads as *paused* on their own page, which is
 * why the reason field is labelled with who reads it: an operator writing "per
 * the safety review" is writing it to a person, not to a log.
 *
 * **Framed as "what happens if nobody comes back", not as "expiry / review
 * date".** The operator is choosing what happens if they never return, and
 * that is the actual decision — the dates are how it gets recorded. Two bounded
 * forms and no third: the backend refuses a hold with neither, so the UI does
 * not offer one.
 */

type Ending = "lifts" | "stays";

export function HoldDialog({
  subjectId,
  subjectName,
  target,
  field,
  value,
  label,
  onClose,
}: {
  subjectId: string;
  subjectName: string;
  target: string;
  /** The field being held — `group`, `share`, `enabled`. */
  field: string;
  /**
   * The value the RESOLVER reads, sent verbatim.
   *
   * For a normal field it is the thing being held — a share name, a group. For
   * a lifecycle field it is the state being refused, which is always "true":
   * the resolver reads the allowance as "deny `enabled = true`", and anything
   * else here is refused by the backend as a malformed term.
   */
  value: string;
  /**
   * What a PERSON calls it. Separate from the value on purpose: "true" is the
   * right thing to send and the wrong thing to put in a sentence, and a dialog
   * that used one string for both would either lie to the operator or be
   * rejected by the resolver.
   */
  label?: string;
  onClose: () => void;
}) {
  const spoken = label ?? value;
  const create = useCreateHold();
  const [ending, setEnding] = useState<Ending>("stays");
  const [date, setDate] = useState("");
  const [reason, setReason] = useState("");

  const ready = reason.trim() !== "" && date !== "" && !create.isPending;

  return (
    <Modal
      open
      onClose={create.isPending ? () => {} : onClose}
      busy={create.isPending}
      size="md"
      labelledBy="hold-title"
    >
      <ModalHeader
        titleId="hold-title"
        title={`Hold ${spoken} for ${subjectName}`}
        lede={`They keep every role they have. This blocks ${spoken} on ${targetLabel(target)}, and nothing else.`}
      />

      <div className="grid gap-4 px-6">
        <fieldset className="grid gap-2">
          <legend className="mb-1 text-[14px]">What happens if nobody comes back to this</legend>

          <label className="flex cursor-pointer items-start gap-3 rounded-inner border border-line px-3.5 py-3 text-[14px] has-[:checked]:border-accent-line has-[:checked]:bg-accent-soft">
            <input
              type="radio"
              name="hold-ending"
              className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
              checked={ending === "stays"}
              onChange={() => setEnding("stays")}
            />
            <span>
              <span className="font-semibold">It stays until somebody decides.</span>
              <span className="block text-[13.5px] text-muted">
                Doing nothing keeps the access blocked. On the date below it appears under
                Review › Holds due as a reminder, and stays blocked until somebody lifts it.
              </span>
            </span>
          </label>

          <label className="flex cursor-pointer items-start gap-3 rounded-inner border border-line px-3.5 py-3 text-[14px] has-[:checked]:border-accent-line has-[:checked]:bg-accent-soft">
            <input
              type="radio"
              name="hold-ending"
              className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
              checked={ending === "lifts"}
              onChange={() => setEnding("lifts")}
            />
            <span>
              <span className="font-semibold">It lifts itself on a date.</span>
              <span className="block text-[13.5px] text-muted">
                Doing nothing gives the access back. Use this when the reason has an end
                you already know — a hold to the end of term.
              </span>
            </span>
          </label>
        </fieldset>

        <div>
          <FieldLabel htmlFor="hold-until">
            {ending === "stays" ? "Remind us on" : "Lifts on"}
          </FieldLabel>
          <Input
            id="hold-until"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
          <FieldHint>
            {/* Both forms are bounded, and this is why: a hold with no bound is an
                open-ended carve-out nobody is prompted to revisit. */}
            A hold with no date is one nobody comes back to, so a date is required either
            way.
          </FieldHint>
        </div>

        <div>
          <FieldLabel htmlFor="hold-reason">Why — shown to {subjectName}</FieldLabel>
          <Input
            id="hold-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Until the safety refresher on 3 October"
          />
          <FieldHint>
            {subjectName} sees this on their own page instead of having to ask makerspace
            staff why the access is missing.
          </FieldHint>
        </div>

        {create.error && (
          <p className="text-[13.5px] text-danger-text">
            {create.error instanceof Error
              ? create.error.message
              : "The hold could not be placed."}{" "}
            {NOTHING_CHANGED}
          </p>
        )}
      </div>

      <ModalFooter note={`Holding ${spoken} does not remove the role that gives it.`}>
        <Button
          variant="accent"
          disabled={!ready}
          onClick={() =>
            create.mutate(
              {
                subjectId,
                target,
                field,
                value,
                reason,
                expiresAt: ending === "lifts" ? new Date(date).toISOString() : undefined,
                reviewDate: ending === "stays" ? new Date(date).toISOString() : undefined,
              },
              { onSuccess: onClose },
            )
          }
        >
          {create.isPending ? "Holding…" : `Hold ${spoken}`}
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={create.isPending}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}
