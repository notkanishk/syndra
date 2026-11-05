"use client";

import { useMemo } from "react";

import { useNameResolver } from "@/lib/queries/useNameResolver";
import { roleName } from "@/lib/names";

/**
 * The names the name components show, as strings to sort by (RULES.md:
 * alphabetical by the shown name). A name not yet resolved falls back to its
 * id or key, so a list settles at most once, when the names land.
 */
export function useShownNames() {
  const resolver = useNameResolver();
  return useMemo(
    () => ({
      /** As <UserName/> reads. */
      user: (id: string, fallback = id) => resolver.resolveUser(id).value?.display_name || fallback,
      /** As <ProjectName/> reads. */
      project: (id: string) => resolver.resolveProject(id).value?.name || id,
      /** As <RoleRef/> reads: "Project / key". */
      roleRef: (projectId: string, roleKey: string) =>
        `${resolver.resolveProject(projectId).value?.name || projectId} / ${roleKey}`,
      /** A role's own name, the key made readable when unknown. */
      role: (projectId: string, roleKey: string) =>
        roleName(resolver.resolveRole(projectId, roleKey).value?.display_name, roleKey),
    }),
    [resolver],
  );
}
