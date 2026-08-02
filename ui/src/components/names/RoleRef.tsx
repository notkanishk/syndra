"use client";

import { Mono } from "@/components/ui/Badge";
import { ProjectName } from "@/components/names/ProjectName";

interface RoleRefProps {
  projectId: string | null | undefined;
  roleKey: string | null | undefined;
  className?: string;
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
export function RoleRef({ projectId, roleKey, className = "" }: RoleRefProps) {
  if (!projectId || !roleKey) {
    return <span className={className}>—</span>;
  }

  return (
    <span className={className}>
      <ProjectName id={projectId} /> / <Mono>{roleKey}</Mono>
    </span>
  );
}
