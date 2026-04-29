"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Mirror of `models.CatalogRole` from the backend. Powers the clone-from
 * picker in the role-create modal and the role-by-project filter in the
 * Add-roles-to-bundle picker.
 */
export interface CatalogRole {
  project_id: string;
  project_name: string;
  role_key: string;
  display_name: string;
  description: string;
  bundle_count: number;
  rule_count: number;
  assigned_user_count: number;
  is_unused: boolean;
  source: string;
  display_label: string;
}

export interface CloneRef {
  project_id: string;
  role_key: string;
}

export interface CreateRoleInput {
  project_id: string;
  role_key: string;
  display_name?: string;
  description?: string;
  group?: string;
  clone_from?: CloneRef;
}

export interface RoleRecord {
  id: string;
  project_id: string;
  role_key: string;
  display_name: string;
  description: string;
}

const KEYS = {
  catalog: ["roles", "catalog"] as const,
  catalogByProject: (projectId: string) => ["roles", "catalog", projectId] as const,
};

/**
 * Hits `GET /api/v1/roles` for the consolidated role inventory across local
 * DB, the directory source (live Zitadel or demo fallback), and persisted
 * references (bundle_roles, mapping_rules, direct_role_grants). Backs the
 * clone-from picker and the Add-roles-to-bundle picker.
 */
export function useGlobalRoleCatalog() {
  return useQuery({
    queryKey: KEYS.catalog,
    queryFn: async (): Promise<CatalogRole[]> => {
      const data = await request<unknown>("/roles");
      return Array.isArray(data) ? (data as CatalogRole[]) : [];
    },
  });
}

/**
 * Server-side filtered variant for project-scoped surfaces. Uses the
 * `?project_id=` query param the backend honors so the JSON payload is
 * smaller and the picker doesn't have to filter client-side.
 */
export function useRolesByProject(projectId: string | null | undefined) {
  return useQuery({
    queryKey: projectId ? KEYS.catalogByProject(projectId) : ["roles", "catalog", "noop"],
    queryFn: async (): Promise<CatalogRole[]> => {
      if (!projectId) return [];
      const data = await request<unknown>(`/roles?project_id=${encodeURIComponent(projectId)}`);
      return Array.isArray(data) ? (data as CatalogRole[]) : [];
    },
    enabled: !!projectId,
  });
}

/**
 * Create a new role. Invalidates both the global catalog and the
 * by-project slice so the new role surfaces in every picker. Surfaces
 * 409 from the backend as `ApiError` (code: "CONFLICT") so the modal
 * can show a uniqueness warning inline.
 */
export function useCreateRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateRoleInput) => {
      return await request<RoleRecord>("/roles", { method: "POST", body: input });
    },
    onSuccess: (_data, input) => {
      qc.invalidateQueries({ queryKey: KEYS.catalog });
      qc.invalidateQueries({ queryKey: KEYS.catalogByProject(input.project_id) });
    },
  });
}

export const rolesQueryKeys = KEYS;
