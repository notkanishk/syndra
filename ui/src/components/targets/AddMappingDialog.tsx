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
  const name = targetLabel(target);

  return (
    <RehearsalDialog
      title={`Add a mapping to ${name}`}
      lede={`Everybody holding the role gets an account on ${name} in this group the next time Syndra reconciles. Nothing changes until you preview the list below and apply it.`}
      noun={["person", "people"]}
      ready={ready}
      definitionLabel="Save mapping"
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
          <span className="-mt-1.5 text-[13px] text-faint">
            The machine or area the role belongs to — the Laser Cutter, the Studio.
          </span>

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
              This project has no roles yet, so there is nothing to map.
            </span>
          )}

          <div className="grid gap-1.5 text-[14px]">
            <label className="grid gap-1.5">
              <span>Account setting</span>
              <Input value={field} onChange={(e) => setField(e.target.value)} />
            </label>
            {/* Outside the label, deliberately. Inside it the hint becomes part
                of the control's accessible name, so a screen reader announces
                the whole paragraph where the field's name should be. */}
            <span className="text-[13px] text-faint">
              Which account setting this role controls on {name} — normally{" "}
              <span className="type-mono">group</span>. Anything {name} does not support is
              refused, and the supported ones are named.
            </span>
          </div>

          <div className="grid gap-1.5 text-[14px]">
            <label className="grid gap-1.5">
              <span>Value</span>
              <Input value={value} onChange={(e) => setValue(e.target.value)} />
            </label>
            <span className="text-[13px] text-faint">
              Checked against {name} before you go further — a group that does not exist
              there is refused now, not later.
            </span>
          </div>
        </div>
      }
      consequence={
        rehearsed?.value_checked === false ? (
          <>
            <span className="font-semibold">
              {name} could not be reached, so <span className="type-mono">{value}</span> was
              not checked.
            </span>{" "}
            The mapping is allowed through so an outage does not look like your mistake. If{" "}
            <span className="type-mono">{value}</span> does not exist, the change fails when it
            reaches {name} and waits in Pending changes until somebody fixes it.
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
      <p className="text-[13.5px] font-semibold">Two settings you cannot map, on purpose</p>
      <p className="text-[13.5px] leading-[1.55] text-muted">
        <span className="type-mono">enabled</span> and{" "}
        <span className="type-mono">smb_enabled</span> follow from whether the person has any
        access to {targetLabel(target)} at all. If a role could set them, one role could switch
        an account off while another still gave that person access — so Syndra refuses them.
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
