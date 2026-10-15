"use client";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { Fragment, useState } from "react";

import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { Segmented, Select } from "@/components/ui/Select";
import { Relative } from "@/components/ui/Time";
import { ApiError } from "@/lib/api-client";
import { formatClock, humanizeKey } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useCreateMapping, useMappings } from "@/lib/queries/useMappings";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import {
  applyParts,
  asQueued,
  bytesToGb,
  gbToBytes,
  homeCount,
  mergePlans,
  opsFor,
  rehearseOps,
  rowsOf,
  type RehearsedPart,
  type RoleChoice,
} from "@/lib/role-mapping-plan";
import { useProjects } from "@/lib/queries/useProjects";
import {
  HOME_ROOT,
  useMappableValues,
  useTargetReach,
  useTargetSettings,
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
  return <RoleMappingDialog target={target} onClose={onClose} />;
}

/**
 * What one role gives on a system (A-03 §2): its group, whether it gives a
 * home folder, and that folder's limit. The one write place for all three;
 * the mapping row shows them read-only, and Change opens this again.
 */
export function RoleMappingDialog({
  target,
  role,
  onClose,
}: {
  target: string;
  /** Changing a role already mapped; absent to add one. */
  role?: { projectId: string; roleKey: string };
  onClose: () => void;
}) {
  const name = targetLabel(target);
  const roster = useTargets();
  const row = (roster.data ?? []).find((t) => t.target === target);
  const project = row?.project ?? undefined;
  const projects = useProjects();
  const mappings = useMappings(target);
  const values = useMappableValues(target, FIELD);
  const reach = useTargetReach(target);
  const settings = useTargetSettings(target);
  const create = useCreateMapping();
  const resolver = useNameResolver();

  const current = role ? rowsOf(mappings.data ?? [], role.projectId, role.roleKey) : {};
  const [roleKey, setRoleKey] = useState(role?.roleKey ?? "");
  const [value, setValue] = useState(current.group?.value ?? "");
  const [home, setHome] = useState(Boolean(current.home));
  const [limit, setLimit] = useState<"size" | "none">(current.home && (!current.quota || current.quota.value === "0") ? "none" : "size");
  const [gb, setGb] = useState(bytesToGb(current.quota?.value));
  const [parts, setParts] = useState<RehearsedPart[]>([]);

  // The system's one project's roles, less the ones already mapped.
  const mapped = new Set((mappings.data ?? []).filter((m) => m.field === FIELD).map((m) => m.role_key));
  const roles =
    (projects.data ?? []).find((r) => r.project.id === project?.id)?.project.roles.filter((r) => !mapped.has(r.key)) ?? [];
  const roleLabel =
    (role && resolver.resolveRole(role.projectId, role.roleKey).value?.display_name) || humanizeKey(role?.roleKey ?? "");

  const homeSetting = settings.data?.settings?.find((x) => x.name === HOME_ROOT);
  const homeRoot = homeSetting?.value ? (homeSetting.labels?.[homeSetting.value] ?? homeSetting.value) : "";
  const choice: RoleChoice = {
    group: value,
    home,
    quota: home ? (limit === "none" ? "0" : gbToBytes(gb)) : null,
  };
  const ops = opsFor(current, choice);

  const list = values.data;
  const neverRead = values.error instanceof ApiError && values.error.code === "VALUES_NEVER_READ";
  const noGroups = Boolean(list && list.values.length === 0);
  const fieldName = list ? `${list.label} on ${name}` : `Group on ${name}`;
  const ready = Boolean(
    project && roleKey && value && list && !noGroups && ops.length > 0 && (choice.quota !== "" || !home),
  );
  const notReadyReason = neverRead
    ? `Nothing to preview until ${name} lists its groups.`
    : noGroups
      ? `Nothing to preview until ${name} has a group of its own.`
      : role && ops.length === 0
        ? "Nothing has changed yet."
        : "Nothing is sent until you apply the preview.";
  const who = { target, projectId: role?.projectId ?? project?.id ?? "", roleKey };
  const homes = homeCount(parts);
  const checked = parts.every((p) => p.plan.value_checked !== false);
  const movesGroup = ops.some((o) => o.kind === "edit" && o.field === FIELD);

  return (
    <RehearsalDialog
      title={role ? `Change what ${roleLabel} gives` : "Add a mapping"}
      lede={
        role
          ? `Everyone who holds ${roleLabel} moves with the change. The preview says how many people that is before anything is sent.`
          : `Everyone who holds the role gets an account on ${name} in this group. The preview says how many people that is before anything is sent.`
      }
      noun={["person", "people"]}
      ready={ready}
      notReadyReason={notReadyReason}
      definitionLabel="Save mapping"
      compose={
        <div className="grid gap-4">
          {role ? (
            <div className="grid gap-1.5">
              <span className="type-label text-label">Role in {project?.name || "its project"}</span>
              <span className="text-[15px] font-semibold">{roleLabel}</span>
            </div>
          ) : (
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
          )}

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

          {settings.data?.read && (
            <div className="grid gap-1.5">
              <span className="type-label text-label" id="m-home">
                Home folder
              </span>
              {!homeRoot && !current.home ? (
                // One control that can't act now: a sentence and the step.
                <span className="text-[14px] leading-[1.55] text-ink [text-wrap:pretty]">
                  No mapping can give one until you choose where home folders go on {name}. This one works without
                  it; give it a home folder later by changing it.{" "}
                  <ArrowLink href={`/system/targets/${target}`}>Choose where home folders go</ArrowLink>
                </span>
              ) : (
                <>
                  <span aria-labelledby="m-home">
                    <Segmented
                      label="Home folder"
                      value={home ? "gives" : "none"}
                      onChange={(v) => setHome(v === "gives")}
                      options={[
                        { value: "none", label: "No home folder" },
                        { value: "gives", label: "Gives a home folder" },
                      ]}
                    />
                  </span>
                  <FieldHint>
                    {home
                      ? `Each person who holds the role gets a folder of their own in ${homeRoot}, named after their account and private to them. Syndra never moves or deletes one, even when the role is revoked.`
                      : `Home folders go in ${homeRoot}, set on ${name}'s page for every role.`}
                  </FieldHint>
                  {home && (
                    <div className="mt-1 grid gap-1.5">
                      <span className="type-label text-label">Home folder limit</span>
                      <span className="flex flex-wrap items-center gap-3">
                        <Segmented
                          label="Home folder limit"
                          value={limit}
                          onChange={setLimit}
                          options={[
                            { value: "size", label: "Up to a size" },
                            { value: "none", label: "No limit" },
                          ]}
                        />
                        {limit === "size" && (
                          <span className="flex items-center gap-2">
                            <span className="w-[110px]">
                              <Input
                                aria-label="Limit in GB"
                                type="number"
                                min={1}
                                inputMode="decimal"
                                value={gb}
                                onChange={(e) => setGb(e.target.value)}
                              />
                            </span>
                            <span className="text-[14px] text-muted">GB</span>
                          </span>
                        )}
                      </span>
                      <FieldHint>
                        Someone with two roles that give a home folder gets the larger limit. One person can be given
                        their own, on {name}&apos;s page under Accounts.
                      </FieldHint>
                    </div>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      }
      consequence={
        homes || !checked || movesGroup ? (
          <>
            {movesGroup && current.group && (
              // The consequence no count implies: Syndra moves the group, not
              // what the old group owns, and it has no way to.
              <>
                Files owned by group <span className="type-mono">{current.group.value}</span> stay with that group.
                People moved to the new group lose access to those files unless somebody changes the files&rsquo;
                owner on {name}.{" "}
              </>
            )}
            {homes && homes.makes > 0 && (
              <>
                Makes {homes.makes} home folder{homes.makes === 1 ? "" : "s"} in{" "}
                <span className="type-mono">{homeRoot}</span>
                {homes.already > 0 ? `; ${homes.already} ${homes.already === 1 ? "person already has" : "people already have"} one there.` : "."}{" "}
              </>
            )}
            {!checked && (
              <>
                <span className="font-semibold">
                  {name} could not be reached, so <span className="type-mono">{value}</span> was not checked.
                </span>{" "}
                The mapping is allowed through so an outage does not look like your mistake. If{" "}
                <span className="type-mono">{value}</span> does not exist, the change fails when it reaches {name} and
                waits in Pending changes until somebody fixes it.
              </>
            )}
          </>
        ) : undefined
      }
      onRehearse={async (acknowledgeScope) => {
        try {
          const rehearsed = await rehearseOps(ops, who, acknowledgeScope);
          setParts(rehearsed);
          return mergePlans(rehearsed);
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
      onApply={async () => {
        const queued = await applyParts(parts, who, (input) => create.mutateAsync(input));
        return asQueued(mergePlans(parts), queued);
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
