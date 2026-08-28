"use client";

import { useMemo, useState } from "react";

import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { type ActionOutcome as Outcome } from "@/lib/outcome";
import { humanizeKey } from "@/lib/format";
import { useAddBundleRole, useBundleRoles } from "@/lib/queries/useBundles";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";

/**
 * Adding roles to a bundle, as the job it actually is.
 *
 * The control this replaces was a project select, then a role select, then a
 * button — one round trip per role, and no way to see what a bundle was
 * missing without walking the projects one at a time. Building a bundle is
 * normally "give this bundle the six things a new member needs", which is one
 * decision, not six.
 *
 * Three properties matter, and all three are why this is a dialog rather than
 * an inline row:
 *
 *   - **Search across every project**, because you know the role's name long
 *     before you remember which project it lives in.
 *   - **Roles already in the bundle are shown, ticked and disabled**, rather
 *     than absent. Absent reads as "doesn't exist" and sends someone to create
 *     a duplicate.
 *   - **A partial failure is resumable.** The API takes one role per call, so
 *     N roles is N writes; if the fourth fails the first three stay added and
 *     the rest stay selected, with the failure named. Clearing the selection on
 *     error would make the operator reconstruct what they had asked for.
 */
export function AddRolesToBundle({
  bundleId,
  name,
  holders,
  onClose,
}: {
  bundleId: string;
  name: string;
  holders: number;
  onClose: () => void;
}) {
  const catalog = useGlobalRoleCatalog();
  const existing = useBundleRoles(bundleId);
  const addRole = useAddBundleRole(bundleId);

  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  const [failure, setFailure] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const held = useMemo(
    () =>
      new Set(
        (existing.data ?? []).map(
          (role) => `${role.zitadel_project_id}:${role.zitadel_role_key}`,
        ),
      ),
    [existing.data],
  );

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

  const chosen = Array.from(selected);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  async function apply() {
    setApplying(true);
    setFailure(null);
    let added = 0;
    // Sequential, not concurrent. Each add is its own write against the
    // working copy, and six at once would interleave into a draft nobody
    // asked for if one of them failed halfway.
    for (const id of chosen) {
      const [projectId, roleKey] = splitId(id);
      try {
        await addRole.mutateAsync({ project_id: projectId, role_key: roleKey });
        added += 1;
        setSelected((prev) => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      } catch (error) {
        setFailure(
          error instanceof Error ? error.message : `${roleKey} couldn't be added to ${name}.`,
        );
        // `break`, and nothing else. This used to set a local flag because the
        // dialog closed itself on success and a failed apply must not be
        // closed over — the flag outlived the auto-close, and a variable that
        // is assigned and never read is a gate that guards nothing. What keeps
        // the failure visible now is that nothing closes this dialog but the
        // operator.
        break;
      }
    }
    setApplying(false);

    if (added > 0) {
      setOutcome({
        // `no_change` about access, deliberately: the working copy moved and
        // nobody's access did. Reporting this as `applied` is the misreading
        // the removal panel used to invite, in the opposite direction.
        kind: "no_change",
        message: `${added} ${added === 1 ? "role" : "roles"} added to the ${name} draft`,
        detail:
          holders > 0
            ? `Nobody has them yet. Publish a version to decide whether the ${holders} ${
                holders === 1 ? "person" : "people"
              } holding ${name} get them.`
            : "Publish a version to give them to people.",
      });
    }
  }

  return (
    <Modal open onClose={onClose} busy={applying} size="md" labelledBy="add-roles-title">
      <ModalHeader
        title={`Add roles to ${name}`}
        titleId="add-roles-title"
        lede={
          holders > 0
            ? `Roles you tick go into the draft. The ${holders} ${
                holders === 1 ? "person" : "people"
              } who hold ${name} keep exactly what they have until you publish a version and move them onto it.`
            : "Roles you tick go into the draft. Publish a version to give them to people."
        }
      />

      <div className="px-6">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search every project…"
          aria-label="Search roles"
        />
      </div>

      <div className="mt-3 max-h-[46vh] overflow-y-auto px-6">
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
                const already = held.has(id);
                return (
                  <label
                    key={id}
                    // The label is the target, so it carries the floor rather
                    // than the 16px glyph inside it. py-2.5 around 14.5px text
                    // lands a pixel or two under 44 — close enough to look
                    // right in a screenshot and not close enough to hit.
                    className={`row-divider flex min-h-[44px] items-center gap-3 py-2.5 text-[14.5px] ${
                      already ? "text-faint" : "cursor-pointer"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={already || selected.has(id)}
                      disabled={already || applying}
                      onChange={() => toggle(id)}
                      className="h-4 w-4 shrink-0 accent-[var(--accent)]"
                    />
                    <span className="min-w-0 flex-1 truncate">
                      {role.display_name || humanizeKey(role.role_key)}{" "}
                      <Mono className="font-normal text-faint">{role.role_key}</Mono>
                    </span>
                    {already && <span className="shrink-0 text-[13px]">already in {name}</span>}
                  </label>
                );
              })}
            </div>
          ))
        )}
      </div>

      {failure && (
        // Named, and the rest stay selected. A partial apply that reports only
        // "something went wrong" leaves the operator unable to tell which of
        // the six landed.
        <div className="danger-note mx-6 mt-3 px-4 py-3 text-[14px] leading-[1.5]">
          {failure} The roles still ticked were not added — press Add {chosen.length}{" "}
          {chosen.length === 1 ? "role" : "roles"} again to add the rest.
        </div>
      )}

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={chosen.length === 0 || applying}
          isPending={applying}
          onClick={apply}
        >
          {chosen.length === 0
            ? "Add roles"
            : `Add ${chosen.length} ${chosen.length === 1 ? "role" : "roles"}`}
        </Button>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}

/**
 * Project ids may not contain a colon; role keys are validated to letters,
 * numbers, dashes and underscores at creation. Splitting on the FIRST colon is
 * still the safe read of the two.
 */
function splitId(id: string): [string, string] {
  const at = id.indexOf(":");
  return [id.slice(0, at), id.slice(at + 1)];
}
