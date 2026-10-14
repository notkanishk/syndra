"use client";

import { Fragment, useState } from "react";

import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { Select } from "@/components/ui/Select";
import { Relative } from "@/components/ui/Time";
import { ApiError } from "@/lib/api-client";
import { formatClock } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import {
  rehearseMappingCreate,
  useCreateMapping,
  useMappings,
  type MappingRehearsal,
} from "@/lib/queries/useMappings";
import { useProjects } from "@/lib/queries/useProjects";
import {
  useMappableValues,
  useTargetReach,
  useTargets,
  type MappableValues,
  type ReachPlace,
} from "@/lib/queries/useTargets";

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
  const name = targetLabel(target);
  const roster = useTargets();
  const row = (roster.data ?? []).find((t) => t.target === target);
  const project = row?.project ?? undefined;
  const projects = useProjects();
  const mappings = useMappings(target);
  const values = useMappableValues(target, FIELD);
  const reach = useTargetReach(target);
  const create = useCreateMapping();
  const [roleKey, setRoleKey] = useState("");
  const [value, setValue] = useState("");
  const [rehearsed, setRehearsed] = useState<MappingRehearsal | null>(null);

  // The system's one project's roles, less the ones already mapped.
  const mapped = new Set((mappings.data ?? []).filter((m) => m.field === FIELD).map((m) => m.role_key));
  const roles = (projects.data ?? [])
    .find((r) => r.project.id === project?.id)
    ?.project.roles.filter((r) => !mapped.has(r.key)) ?? [];

  const list = values.data;
  const neverRead = values.error instanceof ApiError && values.error.code === "VALUES_NEVER_READ";
  const noGroups = Boolean(list && list.values.length === 0);
  const fieldName = list ? `${list.label} on ${name}` : `Group on ${name}`;
  const ready = Boolean(project && roleKey && value && list && !noGroups);
  const notReadyReason = neverRead
    ? `Nothing to preview until ${name} lists its groups.`
    : noGroups
      ? `Nothing to preview until ${name} has a group of its own.`
      : "Nothing is sent until you apply the preview.";

  return (
    <RehearsalDialog
      title="Add a mapping"
      lede={`Everyone who holds the role gets an account on ${name} in this group. The preview says how many people that is before anything is sent.`}
      noun={["person", "people"]}
      ready={ready}
      notReadyReason={notReadyReason}
      definitionLabel="Save mapping"
      compose={
        <div className="grid gap-4">
          <div className="grid gap-1.5">
            <FieldLabel htmlFor="m-role">Role in {project?.name || "its project"}</FieldLabel>
            <Select id="m-role" value={roleKey} onChange={(e) => setRoleKey(e.target.value)}>
              <option value="">{roles.length ? "Choose a role" : "No role left to map"}</option>
              {roles.map((r) => (
                <option key={r.key} value={r.key}>
                  {r.label || r.key}
                </option>
              ))}
            </Select>
            <FieldHint>Roles already mapped are not offered.</FieldHint>
          </div>

          <div className="grid gap-1.5">
            <FieldLabel htmlFor="m-val">{fieldName}</FieldLabel>
            {neverRead || noGroups || !list ? (
              <>
                <Select id="m-val" disabled>
                  <option>
                    {neverRead ? `Waiting for ${name}'s list` : noGroups ? "No groups to offer" : `Reading ${name}'s groups…`}
                  </option>
                </Select>
                <FieldHint>{neverRead ? "Not read yet." : list ? <>Read from {name} <Relative iso={list.read_at} />.</> : " "}</FieldHint>
                {(neverRead || noGroups) && (
                  <div role="status" className="panel flex flex-col gap-1.5 px-4 py-3.5 text-[14px] leading-[1.55]">
                    <span className="font-semibold">
                      {neverRead ? `${name} hasn't listed its groups yet.` : `${name} has no groups of its own yet.`}
                    </span>
                    <span className="text-muted [text-wrap:pretty]">
                      {neverRead
                        ? `Syndra asks for the list when this opens, and ${name} hasn't answered${
                            row?.health?.started_at ? ` since Syndra restarted at ${formatClock(row.health.started_at)}` : " yet"
                          }. This usually clears within a minute and a half.`
                        : builtinSentence(name, list)}
                    </span>
                    <span className="mt-1 flex">
                      <Button variant="outline" size="sm" onClick={() => void values.refetch()}>
                        Read again
                      </Button>
                    </span>
                  </div>
                )}
              </>
            ) : (
              <>
                <Select id="m-val" value={value} onChange={(e) => setValue(e.target.value)}>
                  <option value="">Choose a group</option>
                  {list.values.map((v) => (
                    <option key={v} value={v}>
                      {list.labels?.[v] ?? v}
                    </option>
                  ))}
                </Select>
                <FieldHint>
                  {list.current ? (
                    <>
                      Groups made on {name}, read <Relative iso={list.read_at} />. Its built-in groups are never
                      offered.
                    </>
                  ) : (
                    `The list ${name} gave at ${formatClock(list.read_at)}.`
                  )}
                </FieldHint>
                {value && <OpensLine name={name} places={reach.data?.read ? reach.data.groups?.[value] : undefined} />}
                {!list.current && (
                  <div role="status" className="warn-note flex flex-col gap-1 px-3.5 py-3 text-[14px] leading-[1.55]">
                    <span className="font-semibold text-warn-text">This list is from {formatClock(list.read_at)}.</span>
                    <span>
                      Syndra can&apos;t read {name}&apos;s groups right now. You can still preview. A group deleted since{" "}
                      {formatClock(list.read_at)} fails when the mapping is applied, and the failure is reported.
                    </span>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      }
      consequence={
        rehearsed?.value_checked === false ? (
          <>
            <span className="font-semibold">
              {name} could not be reached, so <span className="type-mono">{value}</span> was not checked.
            </span>{" "}
            The mapping is allowed through so an outage does not look like your mistake. If{" "}
            <span className="type-mono">{value}</span> does not exist, the change fails when it reaches {name} and
            waits in Pending changes until somebody fixes it.
          </>
        ) : undefined
      }
      onRehearse={async (acknowledgeScope) => {
        try {
          const plan = await rehearseMappingCreate(
            { target, projectId: project?.id ?? "", roleKey, field: FIELD, value },
            acknowledgeScope,
          );
          setRehearsed(plan);
          return plan;
        } catch (error) {
          // The one refusal a picker leaves: the group went away on the
          // system after the list was read. Read it again, and say so.
          if (error instanceof ApiError && error.details?.value === "unresolvable") {
            const gone = value;
            setValue("");
            void values.refetch();
            throw new ApiError(400, {
              error: "VALIDATION_FAILED",
              message: `${gone} is no longer on ${name}. It was deleted on ${name} after the list was read, and the preview asked again. Pick another group, or create it on ${name}. The list above is fresh.`,
            });
          }
          throw error;
        }
      }}
      onApply={async (planId) => {
        const result = await create.mutateAsync({
          target,
          projectId: project?.id ?? "",
          roleKey,
          field: FIELD,
          value,
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
 * The field a mapping sets here. One today that a role picks a value for; the
 * home folder and its limit come with A-03.
 */
const FIELD = "group";

/** "It lists 34 groups, all of them built in, like builtin_users and …" */
function builtinSentence(name: string, list: MappableValues | undefined): string {
  const count = list?.builtin_count ?? 0;
  const examples = list?.builtin_examples ?? [];
  const like = examples.length ? `, like ${examples.join(" and ")}` : "";
  return `It lists ${count} group${count === 1 ? "" : "s"}, all of them built in${like}. Syndra never offers those: a role mapped to one would give everyone who holds it the NAS's administration or its default share access. Make a group on ${name}, under Credentials → Groups, then read again.`;
}

/**
 * What a group opens, under the picker, so the operator sees what a mapping
 * gives before the preview (A-02 §3).
 */
export function OpensLine({ name, places }: { name: string; places: ReachPlace[] | undefined }) {
  if (!places) return null;
  if (places.length === 0) {
    return <span className="text-[13.5px] text-warn-text">Opens nothing on {name} yet.</span>;
  }
  return (
    <span className="text-[13.5px] leading-[1.5] text-ink [text-wrap:pretty]">
      Opens{" "}
      {places.map((p, i) => (
        <Fragment key={p.path}>
          {i > 0 && (i === places.length - 1 ? ", and " : ", ")}
          <span className="type-mono text-[12.5px]">{placeName(p)}</span>,{" "}
          {p.access === "change" ? "can change files" : "read only"}
        </Fragment>
      ))}
      .
    </span>
  );
}

/** `main › waddle_share`. */
export function placeName(p: ReachPlace): string {
  return [p.share, ...(p.folder ? p.folder.split("/") : [])].join(" › ");
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
