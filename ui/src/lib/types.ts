// API response types — mirrors Go models used by server-side fetchers in api.ts.
// Page-level components may define their own narrower types for client-side use.

export interface Bundle {
  id: string;
  name: string;
  description: string;
  roles: string[];
  created_at: string;
}

export interface ProjectRole {
  key: string;
  label: string;
  description: string;
}

export interface ProjectCatalog {
  id: string;
  name: string;
  kind: string;
  description: string;
  roles: ProjectRole[];
}

export interface ApplicationCatalog {
  id: string;
  name: string;
  project_id: string;
  description: string;
  consumer: string;
  claim_name: string;
  format_type: string;
}

export interface UserProfile {
  id: string;
  name: string;
  email: string;
  title: string;
  team: string;
  status: string;
  avatar: string;
  location: string;
}

export interface CatalogResponse {
  users: UserProfile[];
  projects: ProjectCatalog[];
  applications: ApplicationCatalog[];
}

export interface MappingRule {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  created_at: string;
}

export interface AuditLog {
  id: string;
  actor_id: string;
  target_id: string;
  action: string;
  resource_id: string;
  created_at: string;
}

export interface ProjectSummary {
  project: ProjectCatalog;
  member_count: number;
  bundle_count: number;
  rule_in_count: number;
  rule_out_count: number;
  active_role_keys: string[];
  sample_members: string[];
}

export interface ApplicationView {
  application: ApplicationCatalog;
  consumed_roles: string[];
  assigned_user_count: number;
}

export interface AccessRequest {
  id: string;
  requester_id: string;
  project_id: string;
  role_key: string;
  justification: string;
  duration_days?: number;
  status: string;
  reviewer_id?: string;
  review_note?: string;
  created_at: string;
  resolved_at?: string;
}

export interface EffectiveRole {
  project_id: string;
  project_name: string;
  role_key: string;
  is_source: boolean;
}

export interface ProjectAccessView {
  project_id: string;
  project_name: string;
  source_roles: EffectiveRole[];
  derived_roles: EffectiveRole[];
  effective_role_keys: string[];
}

export interface UserAccessView {
  user: UserProfile;
  bundles: Bundle[];
  projects: ProjectAccessView[];
  cleanup_hints: string[];
}
