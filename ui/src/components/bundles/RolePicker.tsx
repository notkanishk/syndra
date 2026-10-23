"use client";


import { projectLabel } from "@/components/names/Unresolved";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { useMemo, useState } from "react";

import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { FoldHeading } from "@/components/bundles/FoldHeading";
import { RolePickerOne, type RolePickerOneProps } from "@/components/bundles/RolePickerOne";
import { Unfold } from "@/components/ui/Unfold";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { Input } from "@/components/ui/Input";
import { formatList, humanizeKey } from "@/lib/format";
import { useBundleRolesByBundle, useBundles } from "@/lib/queries/useBundles";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";

/**
 * Choosing bundles in the same picker (B9, 3a): a "Bundles" fold first, and a
 * chosen bundle's roles ticked and locked in their projects. Only New bundle
 * and Add roles on a bundle page pass this; every other picker stays roles only.
 */
export interface BundleChoices {
  /** The bundle being edited: never listed, and if others include it, it can't include any. */
  selfId?: string;
  selected: ReadonlySet<string>;
  onToggle: (id: string) => void;
  /** Bundles already included, ticked and locked with `lockedNote`. */
  locked?: ReadonlySet<string>;
  lockedNote?: string;
}

interface BundleOption {
  id: string;
  name: string;
  description?: string;
  version?: number;
  roleIds: string[];
  blockedReason?: string;
}

interface PickerProps {
  /** Choosing many, the default: tick roles and bundles. `mode="one"` is choosing a single role. */
  mode?: "many";
  selected: ReadonlySet<string>;
  onToggle: (id: string) => void;
  /** Roles that cannot be ticked because they are already in, with the reason. */
  lockedIds?: ReadonlySet<string>;
  lockedNote?: string;
  disabled?: boolean;
  labelledBy?: string;
  /** The line saying what the Waits-for-you mark means, with the way to change it. Off on Roles that wait itself. */
  legend?: boolean;
  /** Offer bundles too. Off by default. */
  bundles?: BundleChoices;
}

const NO_BUNDLES: BundleOption[] = [];

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/**
 * Picking roles out of every project at once.
 *
 * Extracted from `AddRolesToBundle` when creating a bundle started needing the
 * same control. Three properties are why it is a search-and-tick list rather
 * than a project select followed by a role select, and all three are worth as
 * much to a bundle being created as to one being edited:
 *
 *   - **Search across every project**, because you know the role's name long
 *     before you remember which project it lives in.
 *   - **Roles already chosen are shown, ticked and disabled**, rather than
 *     absent. Absent reads as "doesn't exist" and sends someone to create a
 *     duplicate.
 *   - **The whole set is one decision.** Building a bundle is "give this the
 *     six things a new member needs", not six separate acts.
 *
 * It owns the search box and the selection, and nothing else. What ticking
 * means — a working-copy edit, or the contents of a bundle that does not exist
 * yet — belongs to the caller, and the two are not the same act.
 */
export function RolePicker(props: PickerProps | RolePickerOneProps) {
  if (props.mode === "one") return <RolePickerOne {...props} />;
  return props.bundles ? <WithBundles {...props} bundles={props.bundles} /> : <Picker {...props} />;
}

/** Reads the bundles, and says for each why it can't be chosen, if it can't. */
function WithBundles(props: PickerProps & { bundles: BundleChoices }) {
  const { selfId } = props.bundles;
  const list = useBundles();
  const all = useMemo(() => list.data ?? [], [list.data]);
  const others = useMemo(
    () => all.filter((b) => b.id !== selfId).sort((a, b) => a.name.localeCompare(b.name)),
    [all, selfId],
  );
  const roles = useBundleRolesByBundle(
    others.map((b) => b.id),
    { published: true },
  );
  const self = all.find((b) => b.id === selfId);
  const options: BundleOption[] = others.map((b) => {
    // What a bundle includes is read off the others' included_in.
    const inside = all.filter((x) => x.included_in?.includes(b.name)).map((x) => x.name);
    return {
      id: b.id,
      name: b.name,
      description: b.description,
      version: b.latest_version,
      roleIds: (roles.byId[b.id] ?? []).map((r) => `${r.zitadel_project_id}:${r.zitadel_role_key}`),
      blockedReason: self?.included_in?.length
        ? `${self.name} is inside ${formatList(self.included_in)}, so it can't include other bundles`
        : b.has_includes
          ? `Includes ${inside.length ? formatList(inside) : "other bundles"}, so it can't go inside another bundle`
          : undefined,
    };
  });
  return <Picker {...props} bundleOptions={options} />;
}

function Picker({
  selected,
  onToggle,
  lockedIds,
  lockedNote,
  disabled = false,
  labelledBy,
  legend = true,
  bundles,
  bundleOptions = NO_BUNDLES,
}: PickerProps & { bundleOptions?: BundleOption[] }) {
  const catalog = useGlobalRoleCatalog();
  const [query, setQuery] = useState("");
  // A project the operator opened or closed by hand keeps that, whatever the search does.
  const [folded, setFolded] = useState<Record<string, boolean>>({});

  const all = useMemo(() => catalog.data ?? [], [catalog.data]);
  const byId = useMemo(() => new Map(all.map((r) => [`${r.project_id}:${r.role_key}`, r])), [all]);
  const needle = query.trim().toLowerCase();

  const groups = useMemo(() => {
    const matches = (role: CatalogRole) =>
      !needle ||
      [role.role_key, role.display_name, role.description, role.group, role.project_name]
        .join(" ")
        .toLowerCase()
        .includes(needle);
    const byProject = new Map<string, { id: string; project: string; roles: CatalogRole[] }>();
    for (const role of all) {
      if (!matches(role)) continue;
      const g = byProject.get(role.project_id) ?? {
        id: role.project_id,
        project: projectLabel(role.project_name, role.project_id),
        roles: [],
      };
      g.roles.push(role);
      byProject.set(role.project_id, g);
    }
    return Array.from(byProject.values()).sort((a, b) => a.project.localeCompare(b.project));
  }, [all, needle]);

  // A chosen or included bundle's roles: ticked, locked, and saying where from.
  const via = useMemo(() => {
    const m = new Map<string, string>();
    for (const b of bundleOptions) {
      if (!bundles?.selected.has(b.id) && !bundles?.locked?.has(b.id)) continue;
      for (const id of b.roleIds) if (!m.has(id)) m.set(id, b.name);
    }
    return m;
  }, [bundleOptions, bundles?.selected, bundles?.locked]);
  const bundleMatches = bundleOptions.filter(
    (b) => !needle || `${b.name} ${b.description ?? ""}`.toLowerCase().includes(needle),
  );
  const bundlesPicked = bundleOptions.filter((b) => bundles?.selected.has(b.id) || bundles?.locked?.has(b.id)).length;
  const bundlesOpen = folded.__bundles ?? (Boolean(needle) || bundlesPicked > 0);
  const chosenBundles = bundleOptions.filter((b) => bundles?.selected.has(b.id) && !bundles?.locked?.has(b.id));

  const chosen = Array.from(selected).filter((id) => !lockedIds?.has(id));
  const label = (id: string) => {
    const r = byId.get(id);
    return r ? r.display_name || humanizeKey(r.role_key) : humanizeKey(splitRoleId(id)[1]);
  };

  return (
    <>
      <Input
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder={bundles ? "Search roles and bundles in every project" : "Search roles in every project"}
        aria-label="Search roles"
      />

      {chosen.length + chosenBundles.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-2" aria-label="Chosen">
          <span className="type-label mr-1">Chosen</span>
          {chosenBundles.map((b) => (
            <span
              key={b.id}
              className="chip-in inline-flex items-center gap-1 rounded-pill border border-accent-line bg-accent-soft py-1 pl-3 pr-1 text-[13px] font-semibold"
            >
              {b.name}
              <span className="font-normal text-muted"> · bundle · {plural(b.roleIds.length, "role", "roles")}</span>
              <Button
                variant="ghost"
                size="sm"
                disabled={disabled}
                onClick={() => bundles?.onToggle(b.id)}
                aria-label={`Remove ${b.name}`}
                className="-my-1.5 -mr-0.5 px-2.5"
              >
                <span aria-hidden>×</span>
              </Button>
            </span>
          ))}
          {chosen.map((id) => (
            <span
              key={id}
              className="chip-in inline-flex items-center gap-1 rounded-pill border border-line-strong bg-surface-2 py-1 pl-3 pr-1 text-[13px] font-semibold"
            >
              {label(id)}
              <Button
                variant="ghost"
                size="sm"
                disabled={disabled}
                onClick={() => onToggle(id)}
                aria-label={`Remove ${label(id)}`}
                className="-my-1.5 -mr-0.5 px-2.5"
              >
                <span aria-hidden>×</span>
              </Button>
            </span>
          ))}
        </div>
      )}

      {legend && all.some((r) => r.waits_for_send) && (
        <p className="mt-2.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-muted">
          <WaitsMark withLabel />
          <span>means a change waits in Pending changes until you send it.</span>
          <ArrowLink href="/changes/roles-that-wait">Set in Roles that wait</ArrowLink>
        </p>
      )}

      <div
        className="mt-3 max-h-[46vh] overflow-y-auto rounded-inner border border-line-strong"
        aria-labelledby={labelledBy}
      >
        {bundles && bundleMatches.length > 0 && (
          <div className="row-divider">
            <FoldHeading
              open={bundlesOpen}
              onClick={() => setFolded((f) => ({ ...f, __bundles: !bundlesOpen }))}
              title="Bundles"
              count={bundlesPicked ? `${bundlesPicked} included` : plural(bundleMatches.length, "bundle", "bundles")}
              picked={bundlesPicked > 0}
            />
            <Unfold open={bundlesOpen}>
              <div className="flex flex-col gap-1.5 px-2 pb-2">
                {bundleMatches.map((b) => (
                  <ChoiceRow
                    key={b.id}
                    selected={bundles.selected.has(b.id)}
                    locked={bundles.locked?.has(b.id) ?? false}
                    lockedNote={bundles.lockedNote}
                    blocked={Boolean(b.blockedReason)}
                    blockedReason={b.blockedReason}
                    disabled={disabled}
                    onToggle={() => bundles.onToggle(b.id)}
                    title={b.name}
                    meta={[b.version ? `v${b.version}` : null, plural(b.roleIds.length, "role", "roles")]
                      .filter(Boolean)
                      .join(" · ")}
                    description={b.description}
                  />
                ))}
              </div>
            </Unfold>
          </div>
        )}
        {groups.length === 0 && (!bundles || bundleMatches.length === 0) ? (
          <p className="px-4 py-6 text-[14px] text-muted">
            No role matches “{query}”. A role created directly in Zitadel may not be listed here
            yet — check there, or create it in Syndra.
          </p>
        ) : (
          groups.map((group) => {
            const ids = group.roles.map((r) => `${r.project_id}:${r.role_key}`);
            const picked = ids.filter((id) => selected.has(id) || lockedIds?.has(id) || via.has(id)).length;
            // Open while searching, or when something in it is chosen; the
            // operator's own open or close wins.
            const open = folded[group.id] ?? (Boolean(needle) || picked > 0);
            return (
              <div key={group.id} className="row-divider">
                <FoldHeading
                  open={open}
                  onClick={() => setFolded((f) => ({ ...f, [group.id]: !open }))}
                  title={group.project}
                  count={picked ? `${picked} of ${ids.length}` : plural(ids.length, "role", "roles")}
                  picked={picked > 0}
                />
                <Unfold open={open}>
                  <div className="flex flex-col gap-1.5 px-2 pb-2">
                    {group.roles.map((role) => {
                      const id = `${role.project_id}:${role.role_key}`;
                      const own = lockedIds?.has(id) ?? false;
                      const from = via.get(id);
                      return (
                        <ChoiceRow
                          key={id}
                          selected={selected.has(id)}
                          locked={own || Boolean(from)}
                          lockedNote={own ? lockedNote : `In ${from}`}
                          disabled={disabled}
                          onToggle={() => onToggle(id)}
                          title={
                            <>
                              {role.display_name || humanizeKey(role.role_key)}
                              {role.waits_for_send && <WaitsMark className="ml-2" />}
                            </>
                          }
                          meta={<Mono className="font-normal">{role.role_key}</Mono>}
                          description={role.description}
                        />
                      );
                    })}
                  </div>
                </Unfold>
              </div>
            );
          })
        )}
      </div>
    </>
  );
}

/**
 * Project ids may not contain a colon; role keys are validated to letters,
 * numbers, dashes and underscores at creation. Splitting on the FIRST colon is
 * still the safe read of the two.
 */
export function splitRoleId(id: string): [string, string] {
  const at = id.indexOf(":");
  return [id.slice(0, at), id.slice(at + 1)];
}
