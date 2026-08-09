"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";

import { ApiError } from "@/lib/api-client";
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
 * that applying cites the approval that plan became, and that the result is
 * presented as a diff against the plan the operator approved rather than a
 * fresh document.
 *
 * A surface that wrote its own version of this would eventually diverge on the
 * one step that matters, which is the step that happens before the write.
 *
 * The plan id lives here for the same reason. Every surface must hold the id
 * the rehearsal returned and send exactly that — never one it composed, never
 * one left over from a previous rehearsal — and every surface must recover the
 * same way when the backend says the world moved. Three copies of that would
 * be three chances for one of them to re-plan silently and apply the new diff
 * as though the operator had read it.
 */

/**
 * The toast after an apply. It reports three populations, and the one that must
 * never be folded into the others is `queued`: those rows are recorded here and
 * have not reached Zitadel, so the access is still whatever it was. Announcing
 * "12 people updated" after a bulk removal that never left the outbox tells an
 * operator a door is locked while it is open.
 */
export function resultMessage(plan: BulkPlan, noun: [string, string]): string {
  const { succeeded, failed, queued } = plan.summary;
  const parts = [`${succeeded} applied`];
  if (queued > 0) parts.push(`${queued} recorded but not yet in Zitadel`);
  if (failed > 0) parts.push(`${failed} didn't go through`);
  if (parts.length === 1) {
    return `${succeeded} ${succeeded === 1 ? noun[0] : noun[1]} updated.`;
  }
  return `${parts.join(", ")}.`;
}

/** An error only when something actually failed; queued rows are a warning. */
export function resultTone(plan: BulkPlan): "success" | "warning" | "error" {
  if (plan.summary.failed > 0) return "error";
  if (plan.summary.queued > 0) return "warning";
  return "success";
}

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
  /**
   * Applies the approval the rehearsal issued. The id is passed in rather than
   * captured by the caller so there is exactly one place it can come from: the
   * plan currently on screen.
   */
  onApply: (planId: string) => Promise<BulkPlan>;
  onClose: () => void;
}

/**
 * Refusals that mean "the approval on screen can no longer be used". Each has a
 * different cause and they share one recovery: show the current plan and make
 * the operator approve it again.
 *
 * Read from the code rather than the message, because a message is prose and
 * this is a branch.
 */
const STALE_PLAN_CODES = new Set([
  "PLAN_STALE",
  "PLAN_EXPIRED",
  "PLAN_NOT_FOUND",
  "PLAN_ALREADY_APPLIED",
  "PLAN_REQUEST_MISMATCH",
]);

function stalePlanError(error: unknown): ApiError | null {
  return error instanceof ApiError && STALE_PLAN_CODES.has(error.code) ? error : null;
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
  /** Subjects the backend named as moved, so the re-plan says which rows to look at. */
  const [moved, setMoved] = useState<string[]>([]);

  async function run(fn: () => Promise<BulkPlan>, next: "review" | "result") {
    setBusy(true);
    try {
      const result = await fn();
      setPlan(result);
      setMoved([]);
      setStep(next);
      if (next === "result") toast[resultTone(result)](resultMessage(result, noun));
      return result;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "That didn't go through.");
      throw error;
    } finally {
      setBusy(false);
    }
  }

  /**
   * Apply, and re-plan rather than fail when the approval no longer holds.
   *
   * The operator stays on the review step looking at a CURRENT plan, with the
   * subjects that moved named. What must not happen is the re-plan being
   * applied on their behalf: they approved a diff, that diff is gone, and the
   * new one is a new decision. So this refreshes and stops.
   */
  async function applyPlan() {
    if (!plan?.plan_id) return;
    setBusy(true);
    try {
      const result = await onApply(plan.plan_id);
      setPlan(result);
      setMoved([]);
      setStep("result");
      toast[resultTone(result)](resultMessage(result, noun));
    } catch (error) {
      const stale = stalePlanError(error);
      if (!stale) {
        toast.error(error instanceof Error ? error.message : "That didn't go through.");
        return;
      }
      toast.warning(stale.message);
      try {
        const replanned = await onRehearse();
        setPlan(replanned);
        setMoved(Object.keys(stale.details ?? {}));
        setStep("review");
      } catch {
        toast.error("Couldn't re-plan against current state. Close and try again.");
      }
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
        <>
          {moved.length > 0 && (
            <div
              role="status"
              className="mx-6 mb-3 rounded-lg border border-warn-line bg-warn-soft px-4 py-3 text-[13.5px] text-warn-text"
            >
              <p className="font-medium">This changed while you were reading it.</p>
              <p className="mt-1">
                Nothing was applied. {moved.length === 1 ? "One row" : `${moved.length} rows`} moved
                since you approved: {moved.join(", ")}. Below is the plan against current state —
                review it again before applying.
              </p>
            </div>
          )}
          <PlanReview plan={plan} />
        </>
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
              disabled={!plan?.plan_id || plan.summary.apply === 0}
              onClick={() => void applyPlan()}
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
