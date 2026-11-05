"use client";

import { useMemo, useRef, useState } from "react";

import { useWaitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { roleId, type PickerPlace } from "@/components/bundles/RolePickerOne";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useBundles } from "@/lib/queries/useBundles";
import { useProjects } from "@/lib/queries/useProjects";
import {
  describeBulkOp,
  useApplyBulk,
  useRehearseBulk,
  type BulkGrantInput,
  type BulkOp,
} from "@/lib/queries/useBulkGrants";
import dynamic from "next/dynamic";

// Opened on demand, so it stays out of the page's first load.
const RehearsalDialog = dynamic(() => import("@/components/ui/RehearsalDialog").then((m) => m.RehearsalDialog));

/**
 * Choosing what to do to a selection of people.
 *
 * Only the compose step lives here — what a grant/removal/bundle change needs
 * the operator to pick. The rehearse-then-apply machinery, the plan table and
 * the result view are RehearsalDialog's, shared with the drift queue and the
 * request queue so all three explain "what will change" in the same words.
 */

interface BulkDialogProps {
  op: BulkOp;
  userIds: string[];
  /** Sentence describing where the selection came from, shown in the lede. */
  scope: string;
  /**
   * `extend` only: the grants the operator ticked. Pass this from any screen whose rows ARE
   * grants — without it the backend extends everything those people hold that expires, which is
   * a wider change than the one that was selected.
   */
  grantIds?: string[];
  /** Pre-armed target, e.g. when arriving from a role page. */
  initial?: { projectId?: string; roleKey?: string; bundleId?: string };
  /** `applied` is true when the dialog closes on its result, so the caller can end Select. */
  onClose: (applied?: boolean) => void;
}

export function BulkDialog({ op, userIds, grantIds, scope, initial, onClose }: BulkDialogProps) {
  const [projectId, setProjectId] = useState(initial?.projectId ?? "");
  const [roleKey, setRoleKey] = useState(initial?.roleKey ?? "");
  const [bundleId, setBundleId] = useState(initial?.bundleId ?? "");
  const [reason, setReason] = useState("");
  const [durationDays, setDurationDays] = useState(op === "extend" ? 90 : 0);

  const projects = useProjects();
  const bundles = useBundles();
  const rehearse = useRehearseBulk();
  const apply = useApplyBulk();
  const waits = useWaitsBeforeLine("change");
  const applied = useRef(false);

  const needsRole = op === "assign_role" || op === "remove_role";
  const needsBundle = op === "assign_bundle" || op === "remove_bundle";

  // A role is chosen with its project, so a stale key from another project cannot be submitted.
  const places = useMemo<PickerPlace[]>(
    () =>
      (projects.data ?? []).map((row) => ({
        id: row.project.id,
        name: row.project.name,
        roles: (row.project.roles ?? []).map((role) => ({
          key: role.key,
          name: role.label || role.key,
          note: { kind: "none" as const },
        })),
      })),
    [projects.data],
  );

  const input: BulkGrantInput = {
    op,
    user_ids: userIds,
    ...(needsRole ? { project_id: projectId, role_key: roleKey } : {}),
    ...(needsBundle ? { bundle_id: bundleId } : {}),
    reason,
    ...(op === "assign_role" || op === "extend" ? { duration_days: durationDays } : {}),
    ...(op === "extend" && grantIds?.length ? { grant_ids: grantIds } : {}),
  };

  const ready =
    (!needsRole || Boolean(projectId && roleKey)) &&
    (!needsBundle || Boolean(bundleId)) &&
    (op !== "extend" || durationDays > 0) &&
    reason.trim().length > 0;

  // `ready` collapses five distinct gaps into one boolean, and RehearsalDialog
  // greys out its only button on `!ready` alone — this is what says which one,
  // in the order `ready` checks them, so the operator does not have to scan a
  // multi-field compose step to find what they left blank.
  const notReadyReason =
    needsRole && !(projectId && roleKey)
      ? "Choose a role."
      : needsBundle && !bundleId
        ? "Choose a bundle."
        : op === "extend" && durationDays <= 0
          ? "Enter how many days to extend by."
          : reason.trim().length > 0
            ? undefined
            : "Say why. This lands in Change history.";

  const people = `${userIds.length} ${userIds.length === 1 ? "person" : "people"}`;

  return (
    <RehearsalDialog
      title={describeBulkOp(op)}
      lede={`${people} selected${scope ? ` ${scope}` : ""}. Nothing changes until you've seen what this would do.`}
      noun={["person", "people"]}
      ready={ready}
      notReadyReason={notReadyReason}
      waits={waits}
      destructive={op === "remove_role" || op === "remove_bundle"}
      onRehearse={(acknowledgeScope) =>
        rehearse.mutateAsync({ ...input, acknowledge_scope: acknowledgeScope })
      }
      // The same body, plus the approval it is applying. The backend binds the
      // two: a body that is not the one the plan was computed for is refused,
      // so an edit made after the dialog cannot ride in under the approval.
      onApply={async (planId) => {
        const result = await apply.mutateAsync({ ...input, plan_id: planId });
        applied.current = true;
        return result;
      }}
      onClose={() => onClose(applied.current)}
      compose={
        <>
          {needsRole && (
            <div>
              <FieldLabel id="bulk-role-label">Role</FieldLabel>
              <RolePicker
                mode="one"
                audience="operator"
                places={places}
                value={projectId && roleKey ? roleId(projectId, roleKey) : null}
                onChange={(id) => {
                  const [project, key] = id ? splitRoleId(id) : ["", ""];
                  setProjectId(project);
                  setRoleKey(key);
                }}
                labelledBy="bulk-role-label"
              />
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
                {op === "extend" ? "Extend by (days)" : "Ends after (days)"}
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
                  0 grants access with no end date. It stays until somebody removes it.
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
              placeholder="Why. This lands in Change history for every person"
            />
          </div>
        </>
      }
    />
  );
}
