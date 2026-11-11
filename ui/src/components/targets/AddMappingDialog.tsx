"use client";

import { roleName } from "@/lib/names";
import { useState } from "react";

import { HomeChoice, HomesMade } from "@/components/kinds/storage/HomeMapping";
import { OpensLine } from "@/components/kinds/storage/MappingReach";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Notice } from "@/components/ui/Notice";
import { Select } from "@/components/ui/Select";
import { Relative } from "@/components/ui/Time";
import { ApiError } from "@/lib/api-client";
import { fieldsByRole, mappingFields, membershipNoun } from "@/lib/fields";
import { schemaReason } from "@/components/states/SchemaState";
import { useFieldSchema, useFieldSchemaState, useSystemNames } from "@/lib/queries/useFieldSchema";
import { formatClock, formatList } from "@/lib/format";
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
} from "@/lib/queries/useTargets";
import dynamic from "next/dynamic";
import { sortByName } from "@/lib/sort";

// Opened on demand, so it stays out of the page's first load.
const RehearsalDialog = dynamic(() => import("@/components/ui/RehearsalDialog").then((m) => m.RehearsalDialog));

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
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const roster = useTargets();
  const row = (roster.data ?? []).find((t) => t.target === target);
  const project = row?.project ?? undefined;
  const projects = useProjects();
  const mappings = useMappings(target);
  const schema = useFieldSchema(target);
  const fields = mappingFields(schema);
  const noun = membershipNoun(schema);
  const FIELD = fields.membership ?? "";
  const values = useMappableValues(target, FIELD);
  const reach = useTargetReach(target);
  const settings = useTargetSettings(target);
  const create = useCreateMapping();
  const resolver = useNameResolver();

  const current = role ? rowsOf(mappings.data ?? [], role.projectId, role.roleKey, fields) : {};
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
    roleName(role && resolver.resolveRole(role.projectId, role.roleKey).value?.display_name, role?.roleKey ?? "");

  const homeSetting = settings.data?.settings?.find((x) => x.name === HOME_ROOT);
  const homeRoot = homeSetting?.value ? (homeSetting.labels?.[homeSetting.value] ?? homeSetting.value) : "";
  const choice: RoleChoice = {
    group: value,
    home,
    quota: home ? (limit === "none" ? "0" : gbToBytes(gb)) : null,
  };
  const ops = opsFor(current, choice, fields);

  const list = values.data;
  const neverRead = values.error instanceof ApiError && values.error.code === "VALUES_NEVER_READ";
  const noGroups = Boolean(list && list.values.length === 0);
  // A membership the system cannot list at all: Syndra can't check or manage a
  // value it can't see, so it is not the "only built-in groups" case.
  const unlistable = Boolean(list && !list.enumerable);
  const fieldName = list ? `${list.label} on ${name}` : `${noun[0].toUpperCase()}${noun.slice(1)} on ${name}`;
  const ready = Boolean(
    project && roleKey && value && list && !noGroups && ops.length > 0 && (choice.quota !== "" || !home),
  );
  const notReadyReason = neverRead
    ? `Nothing to preview until ${name} lists its ${noun}s.`
    : unlistable
      ? `Nothing to preview: ${name} doesn't list its ${noun}s.`
      : noGroups
      ? `Nothing to preview until ${name} has a ${noun} of its own.`
      : role && ops.length === 0
        ? "Nothing has changed yet."
        : "Nothing is sent until you apply the preview.";
  const who = { target, projectId: role?.projectId ?? project?.id ?? "", roleKey };
  const homes = homeCount(parts, fields);
  const checked = parts.every((p) => p.plan.value_checked !== false);
  const movesGroup = ops.some((o) => o.kind === "edit" && o.field === FIELD);

  return (
    <RehearsalDialog
      title={role ? `Change what ${roleLabel} gives` : "Add a mapping"}
      lede={
        role
          ? `Everyone who holds ${roleLabel} moves with the change. The preview says how many people that is before anything is sent.`
          : `Everyone who holds the role gets an account on ${name} through this ${noun}. The preview says how many people that is before anything is sent.`
      }
      noun={["person", "people"]}
      system={name}
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
                {sortByName(roles, (r) => r.label || r.key, (r) => r.key).map((r) => (
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
                    {neverRead ? `Waiting for ${name}'s list` : noGroups ? `No ${noun}s to offer` : `Reading ${name}'s ${noun}s…`}
                  </option>
                </Select>
                <FieldHint>{neverRead ? "Not read yet." : list ? <>Read from {name} <Relative iso={list.read_at} />.</> : " "}</FieldHint>
                {(neverRead || noGroups) && (
                  <Notice
                    tone="neutral"
                    title={
                      neverRead
                        ? `${name} hasn't listed its ${noun}s yet.`
                        : unlistable
                          ? `${name} doesn't list its ${noun}s.`
                          : `${name} has no ${noun}s of its own yet.`
                    }
                    action={unlistable ? undefined : { label: "Read again", onClick: () => void values.refetch() }}
                    next={
                      neverRead
                        ? "Press Read again in a minute."
                        : unlistable
                          ? `Ask whoever maintains the ${name} add-on to make them listable.`
                          : `Press Read again once ${name} has ${noun}s of its own.`
                    }
                  >
                    {neverRead
                      ? `Syndra asks for the list when this opens, and ${name} hasn't answered${
                          row?.health?.started_at ? ` since Syndra restarted at ${formatClock(row.health.started_at)}` : " yet"
                        }. This usually clears within a minute and a half.`
                      : unlistable
                        ? `Syndra can't check or manage a ${noun} it can't see, so it offers none.`
                        : builtinSentence(name, list, noun)}
                  </Notice>
                )}
              </>
            ) : (
              <>
                <Select id="m-val" value={value} onChange={(e) => setValue(e.target.value)}>
                  <option value="">Choose a {noun}</option>
                  {sortByName(list.values, (v) => list.labels?.[v] ?? v, (v) => v).map((v) => (
                    <option key={v} value={v}>
                      {list.labels?.[v] ?? v}
                    </option>
                  ))}
                </Select>
                <FieldHint>
                  {list.current ? (
                    <>
                      Made on {name}, read <Relative iso={list.read_at} />. Its built-in ones are never
                      offered.
                    </>
                  ) : (
                    `The list ${name} gave at ${formatClock(list.read_at)}.`
                  )}
                </FieldHint>
                {value && <OpensLine name={name} places={reach.data?.read ? reach.data.groups?.[value] : undefined} />}
                {!list.current && (
                  <Notice tone="warn" title={`This list is from ${formatClock(list.read_at)}.`} next={`Preview as usual. If you doubt a ${noun} still exists, check it on ${name} first.`}>
                    Syndra can&apos;t read {name}&apos;s {noun}s right now. You can still preview. A {noun} deleted since{" "}
                    {formatClock(list.read_at)} fails when the mapping is applied, and the failure is reported.
                  </Notice>
                )}
              </>
            )}
          </div>

          {settings.data?.read && (
            <HomeChoice
              target={target}
              name={name}
              homeRoot={homeRoot}
              hadHome={Boolean(current.home)}
              home={home}
              onHome={setHome}
              limit={limit}
              onLimit={setLimit}
              gb={gb}
              onGb={setGb}
            />
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
                What the {noun} <span className="type-mono">{current.group.value}</span> owns stays with it.
                People moved to the new {noun} lose access to that unless somebody changes its
                owner on {name}.{" "}
              </>
            )}
            {homes && homes.makes > 0 && <HomesMade homes={homes} homeRoot={homeRoot} />}
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
              message: `${gone} is no longer on ${name}. It was deleted on ${name} after the list was read, and the preview asked again. Pick another ${noun}, or create it on ${name}. The list above is fresh.`,
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

/** "It lists 34 groups, all of them built in, like builtin_users and …" */
function builtinSentence(name: string, list: MappableValues | undefined, noun: string): string {
  const count = list?.builtin_count ?? 0;
  const examples = list?.builtin_examples ?? [];
  const like = examples.length ? `, like ${examples.join(" and ")}` : "";
  return `It lists ${count} ${noun}${count === 1 ? "" : "s"}, all of them built in${like}. Syndra never offers those: a role mapped to one would give everyone who holds it ${name}'s administration or its default access. Make one on ${name}, then read again.`;
}

/**
 * The settings Syndra will not map, named rather than left out (design M1): the
 * account itself and each of its channels, found by role. They follow from
 * whether somebody is entitled to the add-on at all. Mapping them would let a
 * role switch an account off while the entitlement that created it still
 * stood, so the API refuses them at three separate layers.
 *
 * Said here because a field box that silently rejects them reads as unfinished
 * software, and because an operator who tries one deserves the reason rather
 * than a validation error. The words are the add-on's own labels.
 */
export function RefusedFields({ target }: { target: string }) {
  const nameOf = useSystemNames();
  const by = fieldsByRole(useFieldSchema(target));
  const names = [by.account.length ? "the account" : null, ...by.channel.map((c) => `its ${c.label ?? "other way in"}`)].filter(
    (n): n is string => n !== null,
  );
  if (names.length === 0) return null;
  const many = names.length > 1;
  return (
    <p className="px-1 text-[13.5px] leading-[1.55] text-muted">
      {many ? "Some settings cannot" : "One setting cannot"} be mapped, on purpose: {formatList(names)}.{" "}
      {many ? "They follow" : "It follows"} from whether the person has any access to {nameOf(target)} at all. If a role
      could set {many ? "them" : "it"}, one role could switch an account off while another still gave that person access.
    </p>
  );
}

/** Outline: the one violet fill on this screen is Publish, in the version band. */
export function AddMappingButton({ target, first }: { target: string; first: boolean }) {
  const nameOf = useSystemNames();
  const [open, setOpen] = useState(false);
  const state = useFieldSchemaState(target);
  const why = schemaReason(state, nameOf(target));
  const readable = !why && Boolean(mappingFields(state.schema).membership);
  return (
    <>
      <Button
        variant="outline"
        size="sm"
        disabled={!readable}
        reason={why}
        onClick={() => setOpen(true)}
      >
        {first ? "Add the first mapping" : "Add a mapping"}
      </Button>
      {open && <AddMappingDialog target={target} onClose={() => setOpen(false)} />}
    </>
  );
}
