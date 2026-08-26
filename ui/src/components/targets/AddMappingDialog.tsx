"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { Select } from "@/components/ui/Select";
import { targetLabel } from "@/lib/nav";
import {
  rehearseMappingCreate,
  useCreateMapping,
  type MappingRehearsal,
} from "@/lib/queries/useMappings";
import { useProjects } from "@/lib/queries/useProjects";

/**
 * Adding a mapping (design M1, M5).
 *
 * The last change on this screen that was not rehearsed, and the one most
 * easily mistaken for harmless: a new mapping looks like writing a row down,
 * and it is a grant. Entitlements are derived from mappings, so everybody
 * holding that role becomes entitled the moment it exists — and nothing else
 * would find them, because the periodic reconciler walks existing bindings and
 * a person never bound to this target is in no list it reads.
 *
 * So it goes through the same dialog as edit, delete and rollback: compose,
 * rehearse, meet the ceremony if the backend refuses for size, then apply the
 * plan that was reviewed.
 */
export function AddMappingDialog({ target, onClose }: { target: string; onClose: () => void }) {
  const projects = useProjects();
  const create = useCreateMapping();
  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");
  // `group` is what this add-on takes today. Prefilled rather than fixed: the
  // add-on's schema is the authority on which fields exist, and hard-coding the
  // list here would be a second definition of it — one that goes stale the day
  // an add-on declares a third. The backend refuses an undeclared field and
  // names the declared set in the refusal.
  const [field, setField] = useState("group");
  const [value, setValue] = useState("");
  const [rehearsed, setRehearsed] = useState<MappingRehearsal | null>(null);

  const rows = useMemo(() => projects.data ?? [], [projects.data]);
  const roles = useMemo(
    () => rows.find((entry) => entry.project.id === projectId)?.active_role_keys ?? [],
    [rows, projectId],
  );

  const ready = Boolean(projectId && roleKey && field.trim() && value.trim());

  return (
    <RehearsalDialog
      title={`Add a mapping to ${targetLabel(target)}`}
      lede={`Everybody holding the role gets an account on ${targetLabel(target)} with this value, at the next convergence. Nothing is written until the plan below is applied.`}
      noun={["person", "people"]}
      ready={ready}
      compose={
        <div className="grid gap-3">
          <label className="grid gap-1.5 text-[14px]">
            <span>Project</span>
            <Select
              value={projectId}
              onChange={(e) => {
                setProjectId(e.target.value);
                // The role belongs to the project. Keeping a stale one selected
                // would offer a pair that does not exist.
                setRoleKey("");
              }}
            >
              <option value="">Choose a project</option>
              {rows.map((entry) => (
                <option key={entry.project.id} value={entry.project.id}>
                  {entry.project.name}
                </option>
              ))}
            </Select>
          </label>

          <label className="grid gap-1.5 text-[14px]">
            <span>Role</span>
            <Select
              value={roleKey}
              disabled={!projectId}
              onChange={(e) => setRoleKey(e.target.value)}
            >
              <option value="">{projectId ? "Choose a role" : "Choose a project first"}</option>
              {roles.map((key) => (
                <option key={key} value={key}>
                  {key}
                </option>
              ))}
            </Select>
          </label>
          {projectId && roles.length === 0 && (
            // Not an empty dropdown. A project with no roles cannot confer
            // anything, and saying so is the answer to why the list is empty.
            <span className="-mt-1.5 text-[13px] text-faint">
              This project has no roles, so nothing in it can be granted — and there is
              nothing here for a mapping to hang off.
            </span>
          )}

          <div className="grid gap-1.5 text-[14px]">
            <label className="grid gap-1.5">
              <span>Field</span>
              <Input value={field} onChange={(e) => setField(e.target.value)} />
            </label>
            {/* Outside the label, deliberately. Inside it the hint becomes part
                of the control's accessible name, so a screen reader announces
                the whole paragraph where the field's name should be. */}
            <span className="text-[13px] text-faint">
              What the role sets on {targetLabel(target)}. Checked against the add-on&rsquo;s
              own schema — a field it does not declare is refused here, with the ones it does
              named.
            </span>
          </div>

          <div className="grid gap-1.5 text-[14px]">
            <label className="grid gap-1.5">
              <span>Value</span>
              <Input value={value} onChange={(e) => setValue(e.target.value)} />
            </label>
            <span className="text-[13px] text-faint">
              Checked against {targetLabel(target)} before anything is planned — a value it
              does not recognise is refused here rather than at apply.
            </span>
          </div>
        </div>
      }
      consequence={
        rehearsed?.value_checked === false ? (
          <>
            <span className="font-semibold">
              {targetLabel(target)} could not be asked whether{" "}
              <span className="type-mono">{value}</span> exists,
            </span>{" "}
            so the value was not checked and the mapping is allowed through — refusing it
            while a target is unreachable would make an outage look like your mistake. If the
            value turns out not to exist, the convergence that applies this mapping fails,
            and it arrives as a queued change that will not settle.
          </>
        ) : undefined
      }
      onRehearse={async (acknowledgeScope) => {
        const plan = await rehearseMappingCreate(
          { target, projectId, roleKey, field: field.trim(), value: value.trim() },
          acknowledgeScope,
        );
        setRehearsed(plan);
        return plan;
      }}
      onApply={async (planId) => {
        const result = await create.mutateAsync({
          target,
          projectId,
          roleKey,
          field: field.trim(),
          value: value.trim(),
          planId,
        });
        // Queued, never applied: the row is written here and the people it
        // reaches are moved by the drain.
        const base = rehearsed;
        return {
          op: "create_mapping",
          applied: true,
          outcomes: (base?.outcomes ?? []).map((outcome) => ({
            ...outcome,
            effect: "queued" as const,
          })),
          summary: {
            total: base?.summary.total ?? 0,
            apply: 0,
            no_change: base?.summary.no_change ?? 0,
            blocked: base?.summary.blocked ?? 0,
            failed: 0,
            succeeded: 0,
            queued: result.queued_convergences,
          },
        };
      }}
      onClose={onClose}
    />
  );
}

/**
 * The two fields Syndra will not map, named rather than left out (design M1).
 *
 * `enabled` and `smb_enabled` follow from whether somebody is entitled to the
 * add-on at all. Mapping them would let a role switch an account off while the
 * entitlement that created it still stood, so the API refuses them at three
 * separate layers.
 *
 * Said here because a field box that silently rejects two names reads as
 * unfinished software — and because an operator who tries one deserves the
 * reason rather than a validation error.
 */
export function RefusedFields({ target }: { target: string }) {
  return (
    <div className="grid gap-1.5 rounded-inner border border-line px-4 py-3">
      <p className="text-[13.5px] font-semibold">
        Two fields Syndra will not map, and it is not an omission
      </p>
      <p className="text-[13.5px] leading-[1.55] text-muted">
        <span className="type-mono">enabled</span> and{" "}
        <span className="type-mono">smb_enabled</span> follow from whether somebody is
        entitled to {targetLabel(target)} at all. Mapping them would let a role switch an
        account off while the entitlement that created it still stood, so the API refuses
        them.
      </p>
    </div>
  );
}

/** The one violet fill on this screen. */
export function AddMappingButton({ target, first }: { target: string; first: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="accent" size="sm" onClick={() => setOpen(true)}>
        {first ? "Add the first mapping" : "Add a mapping"}
      </Button>
      {open && <AddMappingDialog target={target} onClose={() => setOpen(false)} />}
    </>
  );
}
