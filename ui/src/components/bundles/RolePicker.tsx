"use client";


import { ArrowLink } from "@/components/ui/ArrowLink";
import { useMemo, useState } from "react";

import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { Unfold } from "@/components/ui/Unfold";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { Input } from "@/components/ui/Input";
import { humanizeKey } from "@/lib/format";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";

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
export function RolePicker({
  selected,
  onToggle,
  /** Roles that cannot be ticked because they are already in, with the reason. */
  lockedIds,
  lockedNote,
  disabled = false,
  labelledBy,
  legend = true,
}: {
  selected: ReadonlySet<string>;
  onToggle: (id: string) => void;
  lockedIds?: ReadonlySet<string>;
  lockedNote?: string;
  disabled?: boolean;
  labelledBy?: string;
  /** The line saying what the Waits-for-you mark means, with the way to change it. Off on Roles that wait itself. */
  legend?: boolean;
}) {
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
        project: role.project_name || role.project_id,
        roles: [],
      };
      g.roles.push(role);
      byProject.set(role.project_id, g);
    }
    return Array.from(byProject.values()).sort((a, b) => a.project.localeCompare(b.project));
  }, [all, needle]);

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
        placeholder="Search roles in every project"
        aria-label="Search roles"
      />

      {chosen.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-2" aria-label="Chosen">
          <span className="type-label mr-1">Chosen</span>
          {chosen.map((id) => (
            <span
              key={id}
              className="tint-in inline-flex items-center gap-1 rounded-pill border border-line-strong bg-surface-2 py-1 pl-3 pr-1 text-[13px] font-semibold"
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
        {groups.length === 0 ? (
          <p className="px-4 py-6 text-[14px] text-muted">
            No role matches “{query}”. A role created directly in Zitadel may not be listed here
            yet — check there, or create it in Syndra.
          </p>
        ) : (
          groups.map((group) => {
            const ids = group.roles.map((r) => `${r.project_id}:${r.role_key}`);
            const picked = ids.filter((id) => selected.has(id) || lockedIds?.has(id)).length;
            // Open while searching, or when something in it is chosen; the
            // operator's own open or close wins.
            const open = folded[group.id] ?? (Boolean(needle) || picked > 0);
            return (
              <div key={group.id} className="row-divider">
                <button
                  type="button"
                  aria-expanded={open}
                  onClick={() => setFolded((f) => ({ ...f, [group.id]: !open }))}
                  className="flex min-h-[44px] w-full items-center gap-3 px-3 py-2 text-left motion-tint hover:bg-[var(--hover)]"
                >
                  <span
                    aria-hidden
                    className={`flex size-7 flex-none items-center justify-center rounded-pill border border-line-strong text-[13px] text-muted motion-press ${
                      open ? "rotate-90" : ""
                    }`}
                  >
                    ›
                  </span>
                  <span className="min-w-0 flex-1 text-[14.5px] font-semibold">{group.project}</span>
                  <span className={`text-[13px] ${picked ? "font-semibold text-accent-text" : "text-faint"}`}>
                    {picked ? `${picked} of ${ids.length}` : `${ids.length} ${ids.length === 1 ? "role" : "roles"}`}
                  </span>
                </button>
                <Unfold open={open}>
                  <div className="flex flex-col gap-1.5 px-2 pb-2">
                    {group.roles.map((role) => {
                      const id = `${role.project_id}:${role.role_key}`;
                      return (
                        <ChoiceRow
                          key={id}
                          selected={selected.has(id)}
                          locked={lockedIds?.has(id) ?? false}
                          lockedNote={lockedNote}
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
