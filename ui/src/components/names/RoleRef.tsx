"use client";

import { Mono } from "@/components/ui/Badge";
import { ProjectName } from "@/components/names/ProjectName";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { roleName } from "@/lib/names";

interface RoleRefProps {
  projectId: string | null | undefined;
  roleKey: string | null | undefined;
  className?: string;
  /** Passed to the project half (G14); the role key never links. */
  link?: boolean;
}

/**
 * A role reference — always the PAIR, never the key on its own.
 *
 * The same key in two projects is two different roles. `admin` in Printing Lab
 * grants nothing in Metal Shop, and a row that renders only `admin` has told
 * the reader something that isn't true: that there is one such role.
 *
 * The shape is deliberate on both halves. The project resolves to its human
 * name because that is what staff recognise; the role stays as its raw key in
 * monospace because the key is what lands in the token and what an operator
 * matches against the identity provider. A table is scanned for identifiers,
 * so it shows one. Prose is read, so it doesn't — see `roleLabel()` in
 * `lib/format`, which is the same rule in sentence form.
 *
 * Do not use this inside a container already scoped to one project — a role
 * list under a project heading, the roles index with its Project column. The
 * pair is established there by structure, and repeating it reads as a stutter.
 */
export function RoleRef({ projectId, roleKey, className = "", link }: RoleRefProps) {
  if (!projectId || !roleKey) {
    return <span className={className}>—</span>;
  }

  return (
    <span className={className}>
      <ProjectName id={projectId} link={link} /> / <Mono>{roleKey}</Mono>
    </span>
  );
}

/**
 * A role in prose, as the planner says it: "Member in Laser Lab" (A-21 §Q3).
 * The role by its name, then its place; for a sentence, where RoleRef is for a row.
 */
export function RoleInPlace({ projectId, roleKey }: { projectId: string; roleKey: string }) {
  const role = roleName(useNameResolver().resolveRole(projectId, roleKey).value?.display_name, roleKey);
  return (
    <>
      {role} in <ProjectName id={projectId} />
    </>
  );
}
