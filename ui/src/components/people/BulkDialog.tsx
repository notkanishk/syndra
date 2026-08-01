"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/Button";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { useBundles } from "@/lib/queries/useBundles";
import { useProjects } from "@/lib/queries/useProjects";
import {
  describeBulkOp,
  useApplyBulk,
  useRehearseBulk,
  type BulkEffect,
  type BulkGrantInput,
  type BulkOp,
  type BulkOutcome,
  type BulkPlan,
} from "@/lib/queries/useBulkGrants";

/**
 * Bulk changes are rehearsed, then applied.
 *
 * Two steps, always, in that order. The first computes what would happen to
 * every selected person and shows it; the second executes it. There is no path
 * from the selection straight to a write — not for the small selections, not
 * for the additive operations. A bulk change is the one operation whose blast
 * radius an operator genuinely cannot hold in their head from a count, and the
 * rehearsal is the only thing standing between "remove role from 40 people" and
 * finding out afterwards which of them lost access they needed.
 *
 * The dialog does not re-derive anything: the plan on screen is verbatim what
 * the server computed, and applying re-sends the same request. A preview drawn
 * by different logic than the write it previews is a preview of nothing.
 */

type Step = "compose" | "review" | "result";

interface BulkDialogProps {
  op: BulkOp;
  userIds: string[];
  /** Sentence describing where the selection came from, shown in the lede. */
  scope: string;
  /** Pre-armed target, e.g. when arriving from a role page. */
  initial?: { projectId?: string; roleKey?: string };
  onClose: () => void;
}

export function BulkDialog({ op, userIds, scope, initial, onClose }: BulkDialogProps) {
  const [step, setStep] = useState<Step>("compose");
  const [projectId, setProjectId] = useState(initial?.projectId ?? "");
  const [roleKey, setRoleKey] = useState(initial?.roleKey ?? "");
  const [bundleId, setBundleId] = useState("");
  const [reason, setReason] = useState("");
  const [durationDays, setDurationDays] = useState(op === "extend" ? 90 : 0);
  const [plan, setPlan] = useState<BulkPlan | null>(null);

  const projects = useProjects();
  const bundles = useBundles();
  const rehearse = useRehearseBulk();
  const apply = useApplyBulk();

  const needsRole = op === "assign_role" || op === "remove_role";
  const needsBundle = op === "assign_bundle" || op === "remove_bundle";
  const busy = rehearse.isPending || apply.isPending;

  const roles = useMemo(() => {
    const project = projects.data?.find((row) => row.project.id === projectId);
    return project?.project.roles ?? [];
  }, [projects.data, projectId]);

  // A role key from a previous project is not a role in this one. Clearing it
  // on change stops a stale key being submitted against the wrong project.
  useEffect(() => {
    if (roleKey && !roles.some((role) => role.key === roleKey)) setRoleKey("");
  }, [roles, roleKey]);

  const input: BulkGrantInput = {
    op,
    user_ids: userIds,
    ...(needsRole ? { project_id: projectId, role_key: roleKey } : {}),
    ...(needsBundle ? { bundle_id: bundleId } : {}),
    reason,
    ...(op === "assign_role" || op === "extend" ? { duration_days: durationDays } : {}),
  };

  const ready =
    (!needsRole || (projectId && roleKey)) &&
    (!needsBundle || bundleId) &&
    (op !== "extend" || durationDays > 0) &&
    reason.trim().length > 0;

  async function runRehearsal() {
    try {
      const result = await rehearse.mutateAsync(input);
      setPlan(result);
      setStep("review");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Couldn't work out what this would do.");
    }
  }

  async function runApply() {
    try {
      const result = await apply.mutateAsync(input);
      setPlan(result);
      setStep("result");
      const { succeeded, failed } = result.summary;
      if (failed > 0) {
        toast.error(`${succeeded} applied, ${failed} didn't go through.`);
      } else {
        toast.success(`${succeeded} ${succeeded === 1 ? "person" : "people"} updated.`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The change didn't go through.");
    }
  }

  const people = `${userIds.length} ${userIds.length === 1 ? "person" : "people"}`;

  return (
    <Modal open onClose={busy ? () => {} : onClose} busy={busy} size="lg" labelledBy="bulk-title">
      <ModalHeader
        titleId="bulk-title"
        title={describeBulkOp(op)}
        lede={
          step === "compose"
            ? `${people} selected${scope ? ` ${scope}` : ""}. Nothing changes until you've seen what this would do.`
            : step === "review"
              ? "Rehearsed against the live directory. Nothing has changed yet."
              : "Done. This is what happened."
        }
      />

      {step === "compose" ? (
        <div className="flex flex-col gap-4 px-6">
          {needsRole && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <FieldLabel htmlFor="bulk-project">Project</FieldLabel>
                <Select
                  id="bulk-project"
                  value={projectId}
                  onChange={(event) => setProjectId(event.target.value)}
                >
                  <option value="">Choose a project…</option>
                  {(projects.data ?? []).map((row) => (
                    <option key={row.project.id} value={row.project.id}>
                      {row.project.name}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <FieldLabel htmlFor="bulk-role">Role</FieldLabel>
                <Select
                  id="bulk-role"
                  emphasis
                  value={roleKey}
                  disabled={!projectId}
                  onChange={(event) => setRoleKey(event.target.value)}
                >
                  <option value="">{projectId ? "Choose a role…" : "Pick a project first"}</option>
                  {roles.map((role) => (
                    <option key={role.key} value={role.key}>
                      {role.label || role.key}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
          )}

          {needsBundle && (
            <div>
              <FieldLabel htmlFor="bulk-bundle">Bundle</FieldLabel>
              <Select
                id="bulk-bundle"
                emphasis
                value={bundleId}
                onChange={(event) => setBundleId(event.target.value)}
              >
                <option value="">Choose a bundle…</option>
                {(bundles.data ?? []).map((bundle) => (
                  <option key={bundle.id} value={bundle.id}>
                    {bundle.name}
                  </option>
                ))}
              </Select>
            </div>
          )}

          {(op === "assign_role" || op === "extend") && (
            <div>
              <FieldLabel htmlFor="bulk-duration">
                {op === "extend" ? "Extend by (days)" : "Expires after (days)"}
              </FieldLabel>
              <Input
                id="bulk-duration"
                type="number"
                min={op === "extend" ? 1 : 0}
                value={durationDays}
                onChange={(event) => setDurationDays(Number(event.target.value))}
              />
              {op === "assign_role" && durationDays === 0 && (
                <p className="mt-1.5 text-[12.5px] text-faint">
                  0 grants access with no expiry — it stays until somebody removes it.
                </p>
              )}
            </div>
          )}

          <div>
            <FieldLabel htmlFor="bulk-reason">Reason</FieldLabel>
            <Input
              id="bulk-reason"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder="Why — this lands in the audit log for every person"
            />
          </div>
        </div>
      ) : (
        <PlanTable plan={plan} />
      )}

      <ModalFooter
        note={
          step === "review" && plan
            ? planNote(plan)
            : step === "compose"
              ? "The next step shows exactly what would change, person by person. It writes nothing."
              : undefined
        }
      >
        {step === "compose" && (
          <>
            <Button variant="accent" isPending={rehearse.isPending} disabled={!ready} onClick={runRehearsal}>
              Rehearse
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </>
        )}
        {step === "review" && plan && (
          <>
            <Button
              // A solid destructive fill, per the button contract: this is the
              // confirming button inside a dialog, and it takes access away.
              variant={isDestructive(plan.op) ? "dangerConfirm" : "accent"}
              isPending={apply.isPending}
              disabled={plan.summary.apply === 0}
              onClick={runApply}
            >
              {plan.summary.apply === 0
                ? "Nothing to apply"
                : `Apply to ${plan.summary.apply} ${plan.summary.apply === 1 ? "person" : "people"}`}
            </Button>
            <Button onClick={() => setStep("compose")}>Back</Button>
          </>
        )}
        {step === "result" && <Button variant="accent" onClick={onClose}>Close</Button>}
      </ModalFooter>
    </Modal>
  );
}

function isDestructive(op: BulkOp): boolean {
  return op === "remove_role" || op === "remove_bundle";
}

/**
 * The note under the confirm button says what the button does NOT cover — the
 * rows that are already done and the rows that were refused. Both are counts an
 * operator would otherwise discover only by reading the whole table.
 */
function planNote(plan: BulkPlan): string {
  const parts: string[] = [];
  if (plan.summary.no_change > 0) parts.push(`${plan.summary.no_change} already in that state`);
  if (plan.summary.blocked > 0) parts.push(`${plan.summary.blocked} refused`);
  if (parts.length === 0) return "Every selected person will change.";
  return `${parts.join(" · ")} — untouched by this.`;
}

const EFFECT_STYLE: Record<BulkEffect, { label: string; className: string }> = {
  apply: { label: "Will change", className: "bg-accent-soft text-accent-text" },
  applied: { label: "Applied", className: "bg-accent-soft text-accent-text" },
  no_change: { label: "No change", className: "bg-tint-2 text-muted" },
  blocked: { label: "Refused", className: "bg-warn-soft text-warn-text" },
  failed: { label: "Failed", className: "bg-danger-soft text-danger-text" },
};

function PlanTable({ plan }: { plan: BulkPlan | null }) {
  if (!plan) return null;

  return (
    <div className="px-6">
      <div className="max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong">
        {plan.outcomes.map((outcome) => (
          <PlanRow key={outcome.user_id} outcome={outcome} />
        ))}
      </div>
    </div>
  );
}

function PlanRow({ outcome }: { outcome: BulkOutcome }) {
  const style = EFFECT_STYLE[outcome.effect] ?? EFFECT_STYLE.no_change;

  return (
    <div className="row-divider flex items-start gap-4 px-4 py-3">
      <span className="min-w-0 flex-1">
        {/* A plan that identifies people by id is a plan nobody can check. */}
        <span className="block truncate text-[14.5px] font-semibold">
          {outcome.name || outcome.email || "Unknown account"}
        </span>
        <span className="block text-[13px] text-muted">
          {outcome.detail}
          {outcome.consequence ? (
            <>
              {" "}
              <span className="text-faint">{outcome.consequence}</span>
            </>
          ) : null}
        </span>
      </span>
      <span
        className={`shrink-0 rounded-pill px-2.5 py-1 text-[12px] font-semibold ${style.className}`}
      >
        {style.label}
      </span>
    </div>
  );
}
