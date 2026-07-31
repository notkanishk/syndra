"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * The token shape — what an application actually receives.
 *
 * A project has one default profile; an application may override it. Because
 * Zitadel's Actions v2 function payload carries no application identifier, a
 * token issued for a project carries the default AND every override key on
 * that project; each application reads its own. Keys are therefore validated
 * unique, and the UI says so rather than letting an operator discover it by
 * decoding a production token.
 */

export type ClaimFormat = "array" | "csv" | "space_delimited";

export interface ClaimProfile {
  project_id: string;
  application_id?: string;
  application_name?: string;
  claim_name: string;
  format_type: ClaimFormat;
  attribute_claims?: Record<string, string>;
  static_claims?: Record<string, unknown>;
}

export interface ClaimKeyOwner {
  key: string;
  owner_label: string;
  application_id?: string;
  kind: "roles" | "attribute" | "static";
  source?: string;
}

export interface ClaimConflict {
  claim_key: string;
  owner: string;
  other: string;
}

export interface ClaimShape {
  project_id: string;
  project_name: string;
  default: ClaimProfile;
  overrides: ClaimProfile[];
  applications: Array<{ id: string; name: string; project_id: string; consumer: string }>;
  emitted_keys: ClaimKeyOwner[];
  conflicts: ClaimConflict[] | null;
}

export interface ClaimVocabulary {
  attributes: string[];
  formats: ClaimFormat[];
}

export type ClaimProfileInput = Pick<
  ClaimProfile,
  "claim_name" | "format_type" | "attribute_claims" | "static_claims"
>;

export function useClaimShape(projectId: string | null | undefined) {
  return useQuery({
    queryKey: ["claim-shape", projectId],
    queryFn: () => request<ClaimShape>(`/projects/${encodeURIComponent(projectId!)}/claim-shape`),
    enabled: Boolean(projectId),
  });
}

/** The attribute sources and formats the shaper accepts, straight from it. */
export function useClaimVocabulary() {
  return useQuery({
    queryKey: ["claim-attributes"],
    queryFn: () => request<ClaimVocabulary>("/claim-attributes"),
    staleTime: Infinity,
  });
}

function invalidateShape(qc: ReturnType<typeof useQueryClient>, projectId: string) {
  qc.invalidateQueries({ queryKey: ["claim-shape", projectId] });
  // The preview is computed from the same profiles, so it is stale the moment
  // the shape changes. Re-running it is the whole point of the screen.
  qc.invalidateQueries({ queryKey: ["applications"] });
}

export function useSaveProjectClaimProfile(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ClaimProfileInput) =>
      request<ClaimShape>(`/projects/${encodeURIComponent(projectId)}/claim-profile`, {
        method: "PUT",
        body,
      }),
    onSuccess: () => invalidateShape(qc, projectId),
  });
}

export function useSaveAppClaimOverride(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ applicationId, ...body }: ClaimProfileInput & { applicationId: string }) =>
      request<ClaimShape>(`/applications/${encodeURIComponent(applicationId)}/claim-profile`, {
        method: "PUT",
        body,
      }),
    onSuccess: () => invalidateShape(qc, projectId),
  });
}

export function useDeleteAppClaimOverride(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (applicationId: string) =>
      request(`/applications/${encodeURIComponent(applicationId)}/claim-profile`, {
        method: "DELETE",
      }),
    onSuccess: () => invalidateShape(qc, projectId),
  });
}
