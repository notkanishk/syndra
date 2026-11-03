"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * What each app is told (A-14, ADR-003). A shape belongs to one app; an app
 * with none gets its project's default, else the built-in `roles` list.
 *
 * Every shaped value on screen comes from the backend's explain step, which
 * runs `internal/claims` — the one shaper. Nothing here shapes a token.
 * Contract: `openspec/changes/claims-shaping/design.md`.
 */

export type ClaimKind = "roles" | "attribute" | "static" | "if_role" | "bundles" | "role_ends";
export type ClaimFormat = "array" | "csv" | "space_delimited";
export type Lands = "both" | "access_token" | "userinfo";
export type ShapeSource = "app" | "project_default" | "builtin";

export interface Claim {
  path: string[];
  kind: ClaimKind;
  source?: string;
  value?: unknown;
  roles?: string[];
  format?: ClaimFormat;
}

export interface RoleMap {
  key: string;
  as?: string;
  included: boolean;
}

export interface Profile {
  claims: Claim[];
  roles?: RoleMap[];
  include_new: boolean;
  lands: Lands;
}

export type ChangeKind = "add" | "remove" | "rename" | "retype" | "move" | "lands" | "include_new";

export interface Change {
  kind: ChangeKind;
  role?: string;
  path?: string[];
  new_path?: string[];
  from?: string;
  to?: string;
}

export interface ClaimTemplate {
  id: string;
  version: number;
  name: string;
  description?: string;
  profile: Profile;
  reads?: "access_token" | "id_token" | "userinfo";
  breaking_rule?: "rename_or_remove" | "admins_users";
}

export interface ProjectRole {
  key: string;
  label: string;
  group?: string;
  description: string;
}

export interface ClaimShapeVersion {
  application_id: string;
  version: number;
  project_id: string;
  profile: Profile;
  template_id: string;
  template_version: number;
  copied_from_application_id?: string;
  copied_from_version?: number;
  restored_from?: number;
  note: string;
  saved_by: string;
  saved_at: string;
}

export interface AppClaimShape {
  application: { id: string; name: string; project_id: string; consumer: string };
  client_id: string;
  source: ShapeSource;
  profile: Profile;
  version: ClaimShapeVersion | null;
  project_default: Profile;
  project_default_source: ShapeSource;
  project_roles: ProjectRole[];
  template: { id: string; version: number; name: string; changes: Change[]; updated: boolean } | null;
  suggested_template?: string;
}

export type CheckState = "pass" | "none" | "warn" | "fail" | "stale";
export interface Check {
  state: CheckState;
  facts: Record<string, unknown>;
}

/** One person's six checks; `token` keeps Syndra's key order. */
export interface Explanation {
  user_id: string;
  checks: Check[];
  token: Record<string, unknown> | null;
  would_receive?: Record<string, unknown>;
  read_at: string;
}

export interface Reach {
  by_role: Record<string, number>;
  people: number;
}

export interface Preview {
  people: Array<{ user_id: string; now: Explanation; draft?: Explanation }>;
  changes: Change[];
  breaking: Change[];
  reach?: Reach | null;
  left_out: number[];
}

export interface Draft {
  profile: Profile;
  template_id: string;
  template_version: number;
  copied_from?: { application_id: string; version: number } | null;
  not_copied: Array<{ role_key: string; name: string }>;
  missing_roles: Array<{ row: number; role_key: string; name: string }>;
}

export interface SaveRequest {
  profile: Profile;
  template_id?: string;
  template_version?: number;
  copied_from?: { application_id: string; version: number };
  note?: string;
  acknowledge_breaking?: boolean;
}

export interface SaveResult {
  version: ClaimShapeVersion;
  changes: Change[];
  breaking: Change[];
}

export interface HistoryEntry extends ClaimShapeVersion {
  changes: Change[];
}

export interface DebugSession {
  application_id: string;
  user_id: string;
  started_by: string;
  started_at: string;
  ends_at: string;
  ended_at?: string;
  ended_by?: string;
}

export interface SignIn {
  seq: number;
  at: string;
  function: string;
  client_id: string;
  user_id: string;
  grants: unknown;
  request: unknown;
  response: unknown;
  duration_ms: number;
  checks: Check[] | { error: string } | null;
}

const app = (id: string) => `/applications/${encodeURIComponent(id)}`;

export const claimKeys = {
  shape: (appId: string) => ["claim-shape", "app", appId] as const,
  preview: (appId: string) => ["claim-shape", "app", appId, "preview"] as const,
  versions: (appId: string) => ["claim-shape", "app", appId, "versions"] as const,
  debug: (appId: string) => ["claim-debug", appId] as const,
  project: (projectId: string) => ["claim-shape", "project", projectId] as const,
};

export function useClaimTemplates() {
  return useQuery({
    queryKey: ["claim-templates"],
    queryFn: () => request<{ templates: ClaimTemplate[] }>("/claim-templates"),
    select: (data) => data.templates,
    staleTime: Infinity,
  });
}

export function useClaimVocabulary() {
  return useQuery({
    queryKey: ["claim-attributes"],
    queryFn: () => request<{ attributes: string[]; formats: ClaimFormat[] }>("/claim-attributes"),
    staleTime: Infinity,
  });
}

export function useAppClaimShape(appId: string | undefined) {
  return useQuery({
    queryKey: claimKeys.shape(appId ?? ""),
    queryFn: () => request<AppClaimShape>(`${app(appId!)}/claim-shape`),
    enabled: Boolean(appId),
  });
}

/**
 * The explain step for the preview person, now and (with a draft) after it.
 * Keyed by the draft itself, so every edit re-runs it and the last answer
 * stays on screen until the next lands.
 */
export function useClaimPreview(
  appId: string,
  userIds: string[],
  draft: Profile | null,
  templateId?: string,
) {
  return useQuery({
    queryKey: [...claimKeys.preview(appId), userIds, draft, templateId ?? ""],
    queryFn: () =>
      request<Preview>(`${app(appId)}/claim-shape/preview`, {
        method: "POST",
        body: { user_ids: userIds, ...(draft ? { draft } : {}), ...(templateId ? { template_id: templateId } : {}) },
      }),
    enabled: Boolean(appId) && userIds.length > 0,
    placeholderData: keepPreviousData,
  });
}

/** A draft from a template or another app. Nothing is written. */
export function useDraftFrom(appId: string) {
  return useMutation({
    mutationFn: (from: { template_id: string } | { application_id: string }) =>
      request<Draft>(`${app(appId)}/claim-shape/draft-from`, { method: "POST", body: from }),
  });
}

function useInvalidateApp(appId: string) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: ["claim-shape", "app", appId] });
    // The index's Claims column reads the same shape.
    qc.invalidateQueries({ queryKey: ["applications"] });
  };
}

export function useSaveAppClaimShape(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (body: SaveRequest) =>
      request<SaveResult>(`${app(appId)}/claim-shape`, { method: "PUT", body }),
    onSuccess: invalidate,
  });
}

export function useClaimShapeVersions(appId: string) {
  return useQuery({
    queryKey: claimKeys.versions(appId),
    queryFn: () => request<{ versions: HistoryEntry[] }>(`${app(appId)}/claim-shape/versions`),
    select: (data) => data.versions,
  });
}

export function useRestoreClaimShape(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: ({ version, acknowledge }: { version: number; acknowledge: boolean }) =>
      request<SaveResult>(`${app(appId)}/claim-shape/versions/${version}/restore`, {
        method: "POST",
        body: { acknowledge_breaking: acknowledge },
      }),
    onSuccess: invalidate,
  });
}

// ---------------------------------------------------------------------------
// Debug mode: one account at a time, 15 minutes, enforced by the server.

export function useAppDebug(appId: string) {
  return useQuery({
    queryKey: claimKeys.debug(appId),
    queryFn: () => request<{ on: boolean; session: DebugSession | null }>(`${app(appId)}/debug`),
  });
}

export function useStartDebug(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId?: string) =>
      request<{ ends_at: string; switched: boolean; session: DebugSession }>(`${app(appId)}/debug`, {
        method: "PUT",
        body: userId ? { user_id: userId } : {},
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: claimKeys.debug(appId) }),
  });
}

export function useStopDebug(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => request<{ on: boolean; session: DebugSession | null }>(`${app(appId)}/debug`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: claimKeys.debug(appId) }),
  });
}

/** The live list, polled every 3s while it runs; newest first. */
export function useDebugSignIns(appId: string, on: boolean) {
  return useQuery({
    queryKey: [...claimKeys.debug(appId), "sign-ins"],
    queryFn: () =>
      request<{ on: boolean; session: DebugSession | null; sign_ins: SignIn[] }>(`${app(appId)}/debug/sign-ins`),
    refetchInterval: on ? 3000 : false,
  });
}

// ---------------------------------------------------------------------------
// The project default: edited on the project's page, its one write place.

export interface ProjectDefaultForm {
  claim_name: string;
  format_type: ClaimFormat;
  attribute_claims: Record<string, string>;
  static_claims: Record<string, unknown>;
}

export interface ProjectClaimShape {
  project_id: string;
  project_name: string;
  configured: boolean;
  default: ProjectDefaultForm;
  profile: Profile;
  applications: Array<{ application_id: string; name: string; source: ShapeSource; version?: number; template_id?: string }>;
}

export function useProjectClaimShape(projectId: string | undefined) {
  return useQuery({
    queryKey: claimKeys.project(projectId ?? ""),
    queryFn: () => request<ProjectClaimShape>(`/projects/${encodeURIComponent(projectId!)}/claim-shape`),
    enabled: Boolean(projectId),
  });
}

export function useSaveProjectClaimProfile(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ProjectDefaultForm) =>
      request<ProjectClaimShape>(`/projects/${encodeURIComponent(projectId)}/claim-profile`, { method: "PUT", body }),
    // Every app on the default reads it.
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["claim-shape"] });
      qc.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}
