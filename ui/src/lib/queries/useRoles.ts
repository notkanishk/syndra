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
  /** The identity provider's own grouping — "Safety-gated", "Open bench". */
  group?: string;
  cloned_from_project?: string;
  cloned_from_role?: string;
  bundle_count: number;
  rule_count: number;
  /** Who Syndra decided holds this role — a Recorded fact, not a Zitadel one. */
  assigned_user_count: number;
  is_unused: boolean;
  source: string;
  /**
   * How many of assigned_user_count the observation store also shows holding
   * the role. Absent — not zero — when `observation.read_at` is missing: the
   * org has never been checked, and that must never render as a confirmed
   * zero. See ObservationBasis.
   */
  confirmed_user_count?: number;
  /**
   * One basis for the whole response — identical on every row. Optional so
   * fixtures that don't care about confirmation aren't forced to fabricate
   * one; a missing basis reads the same as "never observed".
   */
  observation?: ObservationBasis;
}

/**
 * What a Recorded count rests on: whether Zitadel has ever been asked, and
 * whether that read saw everything. Shaped to match `ReadState` in
 * `@/components/ui/ReadFreshness` so a surface can render it with that
 * component instead of inventing its own wording.
 */
export interface ObservationBasis {
  read_at?: string | null;
  current: boolean;
  truncated: boolean;
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
