/**
 * Wire types for POST /api/v1/lookup. Mirror the Go handler in
 * backend/internal/handlers/lookup.go. Each top-level map is always present
 * in the response (empty object when no entry resolved).
 */

export interface LookupRoleKey {
  project_id: string;
  role_key: string;
}

export interface LookupRequest {
  user_ids?: string[];
  project_ids?: string[];
  role_keys?: LookupRoleKey[];
  bundle_ids?: string[];
}

export interface ResolvedUser {
  display_name: string;
  email: string;
}

export interface ResolvedProject {
  name: string;
}

export interface ResolvedRole {
  display_name: string;
}

export interface ResolvedBundle {
  name: string;
}

export interface LookupResponse {
  users: Record<string, ResolvedUser>;
  projects: Record<string, ResolvedProject>;
  roles: Record<string, ResolvedRole>;
  bundles: Record<string, ResolvedBundle>;
}

/** Composite role key for cache lookups: "<project_id>:<role_key>". */
export function roleCompositeKey(projectId: string, roleKey: string): string {
  return `${projectId}:${roleKey}`;
}
