"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** One role that always waits for the operator (N14 B). */
export interface RoleWait {
  project_id: string;
  role_key: string;
  marked_by: string;
  marked_at: string;
}

export interface RolesThatWait {
  roles: RoleWait[];
  /** Only roles decide whether a change waits; every deployment has switched over (migration 000078). */
  switched_over: boolean;
  switch?: { set_by: string; set_at: string };
}

export const rolesThatWaitKey = ["roles-that-wait"] as const;
const catalogKey = ["roles", "catalog"] as const;

export function useRolesThatWait() {
  return useQuery({
    queryKey: rolesThatWaitKey,
    queryFn: () => request<RolesThatWait>("/roles-that-wait"),
  });
}

const path = (projectId: string, roleKey: string) =>
  `/roles-that-wait/${encodeURIComponent(projectId)}/${encodeURIComponent(roleKey)}`;

/** Marks or unmarks roles, one call each; the catalogue's marks follow. */
export function useSetRolesWait() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ ids, waits }: { ids: string[]; waits: boolean }) => {
      for (const id of ids) {
        const at = id.indexOf(":");
        await request(path(id.slice(0, at), id.slice(at + 1)), { method: waits ? "PUT" : "DELETE" });
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: rolesThatWaitKey });
      qc.invalidateQueries({ queryKey: catalogKey });
    },
  });
}
