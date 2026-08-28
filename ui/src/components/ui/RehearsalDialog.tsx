"use client";

import { useEffect, useState } from "react";

import { ApiError } from "@/lib/api-client";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import {
  outcomeFromError,
  type ActionOutcome as Outcome,
  type OutcomeKind,
} from "@/lib/outcome";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
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
 * What an apply reports. It states three populations, and the one that must
 * never be folded into the others is `queued`: those rows are recorded here and
 * have not reached Zitadel, so the access is still whatever it was. Announcing
 * "12 people updated" after a bulk removal that never left the outbox tells an
 * operator a door is locked while it is open.
 */
export function resultMessage(
  plan: BulkPlan,
  noun: [string, string],
  system: string = "Zitadel",
): string {
  const { succeeded, failed, queued } = plan.summary;
  const of = (n: number) => `${n} ${n === 1 ? noun[0] : noun[1]}`;
  const parts = [`Applied to ${of(succeeded)}.`];
  if (queued > 0) parts.push(`${of(queued)} ${queued === 1 ? "is" : "are"} waiting to be sent to ${system}.`);
  if (failed > 0) parts.push(`${of(failed)} failed.`);
  return parts.join(" ");
}

/**
 * What happens to the queued rows, without making the operator know the rule.
 *
 * Two rules drain this queue and they are not symmetric: a withdrawal leaves on
 * a background runner because a delayed revocation IS retained access, and
 * everything that confers access waits for a human to resume it. §7 states the
 * operator consequence plainly — somebody who has just approved something must
 * never have to know which rule applied — so the copy says what will happen
 * rather than naming the rule that decided it.
 */
export function queuedNote(plan: BulkPlan, system: string = "Zitadel"): string | undefined {
  if (plan.summary.queued === 0) return undefined;
  const revocation = plan.op === "remove_role" || plan.op === "remove_bundle";
  return revocation
    ? `Recorded in Syndra and waiting to be sent to ${system}. Syndra sends it on its own within a few minutes; until then the person still has the access.`
    : `Recorded in Syndra and waiting to be sent to ${system}. Nothing has changed there yet; send it from Pending changes.`;
}

/** An error only when something actually failed; queued rows are a warning. */
export function resultTone(plan: BulkPlan): "success" | "warning" | "error" {
  if (plan.summary.failed > 0) return "error";
  if (plan.summary.queued > 0) return "warning";
  return "success";
}

/**
 * The same judgement, in the vocabulary every surface reports in.
 *
 * `queued` outranks a clean apply on purpose: a plan where anything is
 * recorded-and-not-dispatched has not finished, and calling the whole thing
 * applied because most of it was is how "12 people updated" gets said about a
 * door that is still open.
 */
export function resultKind(plan: BulkPlan): OutcomeKind {
  if (plan.summary.failed > 0) return "failed";
  if (plan.summary.queued > 0) return "queued";
  return "applied";
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
  /**
   * The label for applying a change that reaches NOBODY, on a surface where
   * that is a real act rather than a no-op.
   *
   * Every plan in this dialog used to be assumed to move at least one person,
   * which is right for a bulk grant — a plan for nobody means nobody was
   * selected — and wrong for the mapping surfaces. A mapping on a role nobody
   * holds is a definition: it changes what the role WILL confer, the backend
   * writes it and issues no approval for it because there is nothing to
   * review, and the dialog then refused to let it be saved. Defining before
   * assigning is the ordinary order, and it was the one order this dialog
   * could not express.
   *
   * The relaxation is exact. It applies only when the cohort is empty; the
   * moment a plan reaches somebody, the approval is required again and this
   * prop does nothing. Absent, the dialog behaves as it always has.
   */
  definitionLabel?: string;
  /**
   * A consequence of this operation that the plan's own numbers cannot state.
   *
   * The plan counts what Syndra will change. Some operations also have an
   * effect Syndra does not perform and cannot undo — a mapping edit moves a
   * group, and the files the old group owns stay owned by it — and no count
   * implies that. Rendered under the plan, on the review step, because it is
   * part of what is being approved rather than a caveat about the form.
   */
  consequence?: React.ReactNode;
  /**
   * The system the change is sent to, by name, for the result copy. Zitadel
   * unless the surface says otherwise (pass `targetLabel(...)` on a connected
   * system's own screens).
   */
  system?: string;
  /**
   * Computes the plan. Takes the scope acknowledgement rather than reading it
   * from the caller, so a surface cannot acknowledge on the operator's behalf.
   */
  onRehearse: (acknowledgeScope: boolean) => Promise<BulkPlan>;
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
const STALE_PLAN_CODES = new Map<string, string>([
  ["PLAN_STALE", "Things changed after this preview was made."],
  ["PLAN_EXPIRED", "This preview is too old to use."],
  ["PLAN_NOT_FOUND", "This preview is no longer available."],
  [
    "PLAN_ALREADY_APPLIED",
    "This change was already applied once, so it was not applied again. The preview below shows the current state, and may show nothing left to do.",
  ],
  ["PLAN_REQUEST_MISMATCH", "This preview was for a different request."],
  // The sixth. Its absence meant a plan cited on the wrong surface fell through
  // to a bare error with no recovery, which is the one path §8 exists to close.
  ["PLAN_NOT_CITABLE_HERE", "This preview was made on a different screen and cannot be used here."],
  // A re-plan is issued to the CURRENT operator, so it resolves this refusal
  // rather than merely reporting it — the same recovery as every code above.
  ["PLAN_NOT_YOURS", "This preview was made by someone else, so it was not used."],
]);

/**
 * The headline above the per-code sentence.
 *
 * `PLAN_ALREADY_APPLIED` is the one code where "nothing was applied" is false:
 * the earlier apply landed, and only the second attempt did nothing. The bold
 * line is read first, so a headline that contradicts the sentence under it is
 * worse than no headline.
 */
function staleHeadline(code: string): string {
  return code === "PLAN_ALREADY_APPLIED"
    ? "Nothing was applied a second time. This is a fresh preview."
    : "Nothing was applied. This is a fresh preview.";
}

/** §5: every dialog that previews before applying opens with the same sentence. */
const PREVIEW_LEDE =
  "Syndra first shows exactly what would change, person by person. Nothing changes until you press Apply.";

function stalePlanError(error: unknown): ApiError | null {
  return error instanceof ApiError && STALE_PLAN_CODES.has(error.code) ? error : null;
}

/** What moved, in the names an operator recognises. */
function movedLabels(subjectIDs: string[], plan: BulkPlan | null): string[] {
  const byID = new Map((plan?.outcomes ?? []).map((o) => [o.user_id, o.name || o.email]));
  return subjectIDs.map((id) => byID.get(id) ?? id);
}

/**
 * The backend declining to approve a change of this size without being asked
 * twice. It carries the count it computed, which is the whole point: "too
 * large" leaves an operator guessing at what they are being warned about.
 */
function cohortRefusal(error: unknown): { affected: string; limit: string } | null {
  if (!(error instanceof ApiError) || error.code !== "COHORT_ACKNOWLEDGEMENT_REQUIRED") return null;
  return { affected: error.details?.affected ?? "?", limit: error.details?.limit ?? "?" };
}

export function RehearsalDialog({
  title,
  lede,
  noun,
  compose,
  ready = true,
  destructive = false,
  definitionLabel,
  consequence,
  system = "Zitadel",
  onRehearse,
  onApply,
  onClose,
}: RehearsalDialogProps) {
  // With no compose step there is nothing to fill in, so the dialog opens
  // straight into the rehearsal rather than making the operator press a button
  // whose only effect is to reveal the thing they came to see.
  const [step, setStep] = useState<"compose" | "scope" | "review" | "result">(
    compose ? "compose" : "review",
  );
  const [plan, setPlan] = useState<BulkPlan | null>(null);
  const [busy, setBusy] = useState(false);
  const [autoRan, setAutoRan] = useState(false);
  /**
   * The refusal that sent us back to a fresh plan: which one it was, and which
   * subjects the backend named as moved.
   *
   * Only PLAN_STALE carries subjects. Keying the banner on that list alone left
   * the other five refusals swapping the approved plan for a freshly computed
   * one with nothing on screen to say so — so the operator could
   * press Apply believing they had already read this diff, which is exactly the
   * gap the rehearsal exists to close, reintroduced in the recovery path.
   */
  const [stalePlan, setStalePlan] = useState<{ code: string; subjects: string[] } | null>(null);
  /** Set when the backend refuses to approve a change of this size unasked. */
  const [scope, setScope] = useState<{ affected: string; limit: string } | null>(null);
  const [scopeAcknowledged, setScopeAcknowledged] = useState(false);
  // Only when the plan genuinely reaches NOBODY, and the surface says that is
  // still an act. A plan that reaches somebody takes the ordinary path whatever
  // this dialog was told.
  //
  // "Reaches nobody" is three conditions, not one. `apply === 0` alone is the
  // tempting version and it is wrong: a plan carrying forty rows that all
  // resolve to `no_change` has an apply count of zero and is not a definition —
  // it reaches forty people and changes nothing for them. That would take the
  // definition label, submit with no citation, and meet a backend refusal the
  // label had just promised would not happen.
  //
  // So: the rows must have ARRIVED (an absent array is a payload that came
  // short, not an empty cohort), there must be none of them, and the plan must
  // say it counted nobody. Any two without the third is a plan this dialog does
  // not understand, and the safe reading of one of those is the ordinary path.
  const reachesNobody =
    Array.isArray(plan?.outcomes) && plan.outcomes.length === 0 && plan.summary.total === 0;
  const isDefinitionApply = Boolean(definitionLabel) && reachesNobody;
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  /**
   * Rehearse. A blast-radius refusal is not a failure — it is the backend
   * asking the operator to say the number out loud — so it stops on its own
   * step rather than closing the dialog.
   */
  async function rehearse(acknowledgeScope: boolean) {
    setBusy(true);
    try {
      const result = await onRehearse(acknowledgeScope);
      setPlan(result);
      setStalePlan(null);
      setScope(null);
      setStep("review");
      return result;
    } catch (error) {
      const oversized = cohortRefusal(error);
      if (oversized) {
        setScope(oversized);
        setScopeAcknowledged(false);
        setStep("scope");
        return null;
      }
      setOutcome(outcomeFromError(error));
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
    if (!plan) return;
    // A definition apply carries no approval, because the backend issued none:
    // there was nothing to review. Every other apply still cites one, and the
    // guard below is what keeps those two apart.
    if (!plan.plan_id && !isDefinitionApply) return;
    setBusy(true);
    try {
      const result = await onApply(plan.plan_id ?? "");
      setPlan(result);
      setStalePlan(null);
      setStep("result");
      // On the result step, which already existed — the notification was a
      // second surface reporting what this step is for, and the one that
      // removed itself after four seconds.
      setOutcome({
        kind: resultKind(result),
        message: resultMessage(result, noun, system),
        detail: queuedNote(result, system),
      });
    } catch (error) {
      const stale = stalePlanError(error);
      if (!stale) {
        setOutcome(outcomeFromError(error));
        return;
      }
      // Deliberately NOT an outcome block. The stale-plan banner below is
      // this refusal's report and a better one — it names the subjects that
      // moved — and two `role="alert"` regions saying the same thing is two
      // things a screen reader has to hear before reaching the plan.
      setOutcome(null);
      try {
        // Already acknowledged once if it needed to be: the cohort has not
        // grown, and making the operator confirm the size again to see what
        // moved buries the thing they actually need to read.
        const replanned = await onRehearse(true);
        setPlan(replanned);
        // Recorded for EVERY refusal in the set, not only the one that happens
        // to carry details. The banner is what tells the operator the plan
        // under their cursor is not the one they approved.
        setStalePlan({ code: stale.code, subjects: Object.keys(stale.details ?? {}) });
        setStep("review");
      } catch {
        setOutcome({
          kind: "failed",
          message: "Syndra couldn't redo the preview",
          detail: "Close and try again.",
        });
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
    void rehearse(false).catch(() => onClose());
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
            : step === "scope"
              ? "This change is larger than usual. Nothing has been checked or changed yet."
              : step === "review"
                ? PREVIEW_LEDE
                : "Finished. Here is what happened."
        }
      />

      {step === "compose" ? (
        <div className="flex flex-col gap-4 px-6">{compose}</div>
      ) : step === "scope" ? (
        <div className="grid gap-3 px-6">
          {/* The step did not exist a moment ago and does not appear below the
              limit. Said first, so an operator who crosses the threshold once
              cannot mistake it for a form they filled in wrong — and one who
              never crosses it never learns the step exists. */}
          <p role="status" className="text-[13.5px] leading-[1.55] text-muted">
            This change reaches{" "}
            <strong className="font-semibold text-ink">
              {scope?.affected} {noun[1]}
            </strong>
            . A change to more than {scope?.limit} {noun[1]} needs you to confirm the number
            first. Confirming shows the preview; nothing changes yet.
          </p>
          {/* Rung 2, and the product's own primitive for it. This step used to
              be a warn panel and a button carrying the count, which is a rung
              below what it claims: the ceremony is the tick, and a label is
              something a hand reaches past. Not rung 3 either — copying digits
              trains an operator not to look, and typing back a number the
              screen already shows proves nothing. Rung 3 stays reserved for
              taking access from a person somebody named. */}
          <AcknowledgeCount
            checked={scopeAcknowledged}
            onChange={setScopeAcknowledged}
            count={Number(scope?.affected ?? 0)}
            noun={noun[1]}
            verb="changes access for"
            disabled={busy}
          />
        </div>
      ) : (
        <>
          {stalePlan && (
            <div
              // alert, not status: this has to be read BEFORE the next click,
              // and a polite live region is announced whenever the screen
              // reader gets round to it.
              role="alert"
              className="mx-6 mb-3 rounded-lg border border-warn-line bg-warn-soft px-4 py-3 text-[13.5px] text-warn-text"
            >
              <p className="font-medium">{staleHeadline(stalePlan.code)}</p>
              <p className="mt-1">
                {STALE_PLAN_CODES.get(stalePlan.code)} The preview below is against the current
                state. Read it again before you apply.
              </p>
              {stalePlan.subjects.length > 0 && (
                <p className="mt-1">
                  {stalePlan.subjects.length === 1
                    ? "Changed"
                    : `${stalePlan.subjects.length} ${noun[1]} changed`}{" "}
                  since you last looked: {movedLabels(stalePlan.subjects, plan).join(", ")}.
                </p>
              )}
            </div>
          )}
          <PlanReview plan={plan} />
          {consequence && (
            <div className="px-6">
              <p className="text-[13.5px] leading-[1.55] text-warn-text">{consequence}</p>
            </div>
          )}
        </>
      )}

      {/* The plan's own result, on the step that exists for it. A refusal
          appears on whichever step the operator is standing on, because that
          is where they will look for the reason the button did nothing. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          step === "review" && plan
            ? planNote(plan, noun)
            : step === "compose"
              ? PREVIEW_LEDE
              : undefined
        }
      >
        {step === "compose" && (
          <>
            <Button
              variant="accent"
              isPending={busy}
              disabled={!ready}
              onClick={() => void rehearse(false).catch(() => {})}
            >
              Preview the change
            </Button>
            <Button disabled={busy} reason={busy ? "Wait for the preview to finish." : undefined} onClick={onClose}>
              Cancel
            </Button>
          </>
        )}

        {step === "scope" && (
          <>
            {/* Accent, not a solid red fill. This button computes a plan and
                writes nothing, and `dangerConfirm` is reserved for the button
                that performs a destruction. */}
            <Button
              variant="accent"
              isPending={busy}
              disabled={!scopeAcknowledged}
              onClick={() => void rehearse(true).catch(() => {})}
            >
              Preview the change for {scope?.affected} {noun[1]}
            </Button>
            <Button disabled={busy} onClick={compose ? () => setStep("compose") : onClose}>
              {compose ? "Back" : "Cancel"}
            </Button>
          </>
        )}

        {step === "review" && (
          <>
            <Button
              variant={destructive ? "dangerConfirm" : "accent"}
              isPending={busy}
              disabled={isDefinitionApply ? busy : !plan?.plan_id || plan.summary.apply === 0}
              onClick={() => void applyPlan()}
            >
              {!plan
                ? "Previewing…"
                : isDefinitionApply
                  ? definitionLabel
                  : applyLabel(plan, noun)}
            </Button>
            {/*
              Disabled while a write is out. Abandoning the dialog mid-apply
              does not abandon the write — it only takes away the report of
              what it did, which is the one thing the operator still needs.
            */}
            {compose ? (
              <Button disabled={busy} onClick={() => setStep("compose")}>
                Back
              </Button>
            ) : (
              <Button disabled={busy} reason={busy ? "Wait for the change to finish." : undefined} onClick={onClose}>
                Cancel
              </Button>
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
