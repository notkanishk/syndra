"use client";

import { useMemo, useState } from "react";

import { Mono } from "@/components/ui/Badge";
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
}: {
  selected: ReadonlySet<string>;
  onToggle: (id: string) => void;
  lockedIds?: ReadonlySet<string>;
  lockedNote?: string;
  disabled?: boolean;
  labelledBy?: string;
}) {
  const catalog = useGlobalRoleCatalog();
  const [query, setQuery] = useState("");

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const matches = (role: CatalogRole) =>
      !needle ||
      [role.role_key, role.display_name, role.description, role.group, role.project_name]
        .join(" ")
        .toLowerCase()
        .includes(needle);

    const byProject = new Map<string, { project: string; roles: CatalogRole[] }>();
    for (const role of catalog.data ?? []) {
      if (!matches(role)) continue;
      const key = role.project_id;
      if (!byProject.has(key)) {
        byProject.set(key, { project: role.project_name || role.project_id, roles: [] });
      }
      byProject.get(key)!.roles.push(role);
    }
    return Array.from(byProject.values()).sort((a, b) => a.project.localeCompare(b.project));
  }, [catalog.data, query]);

  return (
    <>
      <Input
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Search every project…"
        aria-label="Search roles"
      />

      <div className="mt-3 max-h-[46vh] overflow-y-auto" aria-labelledby={labelledBy}>
        {groups.length === 0 ? (
          <p className="py-6 text-[14px] text-muted">
            No role matches “{query}”. A role created directly in Zitadel may not be listed here
            yet — check there, or create it in Syndra.
          </p>
        ) : (
          groups.map((group) => (
            <div key={group.project} className="mb-3">
              <div className="type-label sticky top-0 bg-surface-1 py-1.5">{group.project}</div>
              {group.roles.map((role) => {
                const id = `${role.project_id}:${role.role_key}`;
                const locked = lockedIds?.has(id) ?? false;
                return (
                  <label
                    key={id}
                    // The label is the target, so it carries the floor rather
                    // than the 16px glyph inside it. py-2.5 around 14.5px text
                    // lands a pixel or two under 44 — close enough to look
                    // right in a screenshot and not close enough to hit.
                    className={`row-divider flex min-h-[44px] items-center gap-3 py-2.5 text-[14.5px] ${
                      locked ? "text-faint" : "cursor-pointer"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={locked || selected.has(id)}
                      disabled={locked || disabled}
                      onChange={() => onToggle(id)}
                      className="h-4 w-4 shrink-0 accent-[var(--accent)]"
                    />
                    <span className="min-w-0 flex-1 truncate">
                      {role.display_name || humanizeKey(role.role_key)}{" "}
                      <Mono className="font-normal text-faint">{role.role_key}</Mono>
                    </span>
                    {locked && lockedNote && (
                      <span className="shrink-0 text-[13px]">{lockedNote}</span>
                    )}
                  </label>
                );
              })}
            </div>
          ))
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
