"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PlanReview, applyLabel, planNote } from "@/components/ui/PlanReview";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * Rehearse, then apply — the shape every bulk operation in the product wears.
 *
 * Callers supply what varies (the title, the compose step if there is one, the
 * two mutations) and this owns what must not: that the plan is always computed
 * before anything is written, that the plan on screen is the server's verbatim,
 * that applying re-sends the same request, and that the result is presented as
 * a diff against the plan the operator approved rather than a fresh document.
 *
 * A surface that wrote its own version of this would eventually diverge on the
 * one step that matters, which is the step that happens before the write.
 */

interface RehearsalDialogProps {
  title: string;
  /** Shown on the compose step, or above the plan when there is no compose step. */
  lede: string;
  /** Singular/plural for the apply button: ["person", "people"], ["item", "items"]. */
  noun: [string, string];
  /** Optional first step. Omit for operations whose target is already decided. */
  compose?: React.ReactNode;
  /** False while the compose step is incomplete. */
  ready?: boolean;
  /** Solid destructive confirm rather than accent. */
  destructive?: boolean;
  onRehearse: () => Promise<BulkPlan>;
  onApply: () => Promise<BulkPlan>;
  onClose: () => void;
}

export function RehearsalDialog({
  title,
  lede,
  noun,
  compose,
  ready = true,
  destructive = false,
  onRehearse,
  onApply,
  onClose,
}: RehearsalDialogProps) {
  // With no compose step there is nothing to fill in, so the dialog opens
  // straight into the rehearsal rather than making the operator press a button
  // whose only effect is to reveal the thing they came to see.
  const [step, setStep] = useState<"compose" | "review" | "result">(compose ? "compose" : "review");
  const [plan, setPlan] = useState<BulkPlan | null>(null);
  const [busy, setBusy] = useState(false);
  const [autoRan, setAutoRan] = useState(false);

  async function run(fn: () => Promise<BulkPlan>, next: "review" | "result") {
    setBusy(true);
    try {
      const result = await fn();
      setPlan(result);
      setStep(next);
      if (next === "result") {
        const { succeeded, failed } = result.summary;
        if (failed > 0) toast.error(`${succeeded} applied, ${failed} didn't go through.`);
        else toast.success(`${succeeded} ${succeeded === 1 ? noun[0] : noun[1]} updated.`);
      }
      return result;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "That didn't go through.");
      throw error;
    } finally {
      setBusy(false);
    }
  }

  // An operation with no compose step has nothing to fill in, so the rehearsal
  // fires on open. Guarded by a ref-like flag rather than a dependency list
  // because `onRehearse` is a fresh closure on every render and would otherwise
  // re-fire the rehearsal continuously.
  useEffect(() => {
    if (compose || autoRan) return;
    setAutoRan(true);
    void run(onRehearse, "review").catch(() => onClose());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [compose, autoRan]);

  return (
    <Modal open onClose={busy ? () => {} : onClose} busy={busy} size="lg" labelledBy="rehearsal-title">
      <ModalHeader
        titleId="rehearsal-title"
        title={title}
        lede={
          step === "compose"
            ? lede
            : step === "review"
              ? "Rehearsed against live state. Nothing has changed yet."
              : "Done. This is what happened."
        }
      />

      {step === "compose" ? (
        <div className="flex flex-col gap-4 px-6">{compose}</div>
      ) : (
        <PlanReview plan={plan} />
      )}

      <ModalFooter
        note={
          step === "review" && plan
            ? planNote(plan)
            : step === "compose"
              ? "The next step shows exactly what would change, row by row. It writes nothing."
              : undefined
        }
      >
        {step === "compose" && (
          <>
            <Button
              variant="accent"
              isPending={busy}
              disabled={!ready}
              onClick={() => void run(onRehearse, "review").catch(() => {})}
            >
              Rehearse
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </>
        )}

        {step === "review" && (
          <>
            <Button
              variant={destructive ? "dangerConfirm" : "accent"}
              isPending={busy}
              disabled={!plan || plan.summary.apply === 0}
              onClick={() => void run(onApply, "result").catch(() => {})}
            >
              {plan ? applyLabel(plan, noun) : "Rehearsing…"}
            </Button>
            {compose ? (
              <Button onClick={() => setStep("compose")}>Back</Button>
            ) : (
              <Button onClick={onClose}>Cancel</Button>
            )}
          </>
        )}

        {step === "result" && (
          <Button variant="accent" onClick={onClose}>
            Close
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
