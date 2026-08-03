"use client";

import { useEffect, useMemo, useState } from "react";

import { FieldLabel, Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
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
  initial?: { projectId?: string; roleKey?: string };
  onClose: () => void;
}

export function BulkDialog({ op, userIds, grantIds, scope, initial, onClose }: BulkDialogProps) {
  const [projectId, setProjectId] = useState(initial?.projectId ?? "");
  const [roleKey, setRoleKey] = useState(initial?.roleKey ?? "");
  const [bundleId, setBundleId] = useState("");
  const [reason, setReason] = useState("");
  const [durationDays, setDurationDays] = useState(op === "extend" ? 90 : 0);

  const projects = useProjects();
  const bundles = useBundles();
  const rehearse = useRehearseBulk();
  const apply = useApplyBulk();

  const needsRole = op === "assign_role" || op === "remove_role";
  const needsBundle = op === "assign_bundle" || op === "remove_bundle";

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
    ...(op === "extend" && grantIds?.length ? { grant_ids: grantIds } : {}),
  };

  const ready =
    (!needsRole || Boolean(projectId && roleKey)) &&
    (!needsBundle || Boolean(bundleId)) &&
    (op !== "extend" || durationDays > 0) &&
    reason.trim().length > 0;

  const people = `${userIds.length} ${userIds.length === 1 ? "person" : "people"}`;

  return (
    <RehearsalDialog
      title={describeBulkOp(op)}
      lede={`${people} selected${scope ? ` ${scope}` : ""}. Nothing changes until you've seen what this would do.`}
      noun={["person", "people"]}
      ready={ready}
      destructive={op === "remove_role" || op === "remove_bundle"}
      onRehearse={() => rehearse.mutateAsync(input)}
      onApply={() => apply.mutateAsync(input)}
      onClose={onClose}
      compose={
        <>
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
        </>
      }
    />
  );
}
