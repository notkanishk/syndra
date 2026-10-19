"use client";

import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import { bundlesQueryKeys, type BundleRoleRow } from "@/lib/queries/useBundles";

/**
 * Bundle versioning.
 *
 * Editing a bundle changes its WORKING COPY and reaches nobody. Publishing is
 * the separate, rehearsed step that can move the people already holding it —
 * which is why the publish hooks below speak the same `BulkPlan` every other
 * apply in the product speaks, and why `?apply=true` is a property of the
 * request rather than a field in the body.
 */

export interface BundleVersion {
  id: string;
  bundle_id: string;
  version: number;
  note: string;
  published_by: string;
  published_at: string;
  /** How many people are pinned to THIS version. */
  holder_count: number;
  roles?: BundleRoleRow[];
}

export interface BundleHolder {
  bundle_id: string;
  user_id: string;
  version_id: string;
  version: number;
  assigned_at: string;
  /** How they got it. Absent on rows recorded before this was tracked. */
  assigned_by?: string; // absent too when automatic (welcome)
  source?: "direct" | "request" | "moved" | "welcome";
  moved_from_version?: number;
}

/** The unpublished difference between the working copy and the latest version. */
export interface BundleDraft {
  bundle_id: string;
  latest_version: number;
  next_version: number;
  added: BundleRoleRow[];
  removed: BundleRoleRow[];
  holder_count: number;
  /** The bundles the working copy includes, each pinned to a version (B8). */
  includes: BundleInclude[];
  /** Includes the next version adds (from 0), drops (to 0) or moves. */
  include_changes: IncludeChange[];
}

export interface BundleInclude {
  child_bundle_id: string;
  child_name: string;
  child_version_id: string;
  child_version: number;
  child_latest_version: number;
}

export interface IncludeChange {
  child_bundle_id: string;
  child_name: string;
  from_version: number;
  to_version: number;
}

/** A bundle that includes this one (B8). */
export interface BundleIncluder {
  bundle_id: string;
  name: string;
  /** The version its latest published version includes; 0 if only its working copy does. */
  included_version: number;
  /** Set when its working copy includes a different version. */
  draft_version?: number;
  people: number;
}

export type IncluderAction = "update" | "leave";

/**
 * How many unpublished changes a draft carries.
 *
 * Every `?.` here is load-bearing, including the ones on the arrays. Optional chaining guards the
 * PARENT only: `draft?.added.length` short-circuits when `draft` is absent, and then throws on the
 * `.length` when `draft` is present but `added` is null — which is exactly what took the bundles
 * page down, with `?? 0` sitting right there looking like it handled the case.
 */
export function draftChangeCount(draft: BundleDraft | undefined): number {
  return (draft?.added?.length ?? 0) + (draft?.removed?.length ?? 0) + (draft?.include_changes?.length ?? 0);
}

const KEYS = {
  versions: (bundleId: string) => ["bundles", bundleId, "versions"] as const,
  holders: (bundleId: string) => ["bundles", bundleId, "holders"] as const,
  draft: (bundleId: string) => ["bundles", bundleId, "draft"] as const,
  includers: (bundleId: string) => ["bundles", bundleId, "includers"] as const,
};

async function fetchVersions(bundleId: string): Promise<BundleVersion[]> {
  const data = await request<unknown>(`/bundles/${bundleId}/versions`);
  return Array.isArray(data) ? (data as BundleVersion[]) : [];
}

export function useBundleVersions(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.versions(bundleId ?? ""),
    queryFn: () => fetchVersions(bundleId ?? ""),
    enabled: Boolean(bundleId),
  });
}

/**
 * Every version of several bundles at once, keyed by bundle: what an included
 * bundle's pinned version gives, and what its newer one would add. Shares the
 * cache with `useBundleVersions`.
 */
export function useVersionsByBundle(bundleIds: string[]): Record<string, BundleVersion[] | undefined> {
  const results = useQueries({
    queries: bundleIds.map((id) => ({ queryKey: KEYS.versions(id), queryFn: () => fetchVersions(id) })),
  });
  return Object.fromEntries(bundleIds.map((id, i) => [id, results[i].data]));
}

/** The bundles that include this one, with the version each is pinned to (B8). */
export function useBundleIncluders(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.includers(bundleId ?? ""),
    queryFn: async (): Promise<BundleIncluder[]> => {
      const data = await request<unknown>(`/bundles/${bundleId}/includers`);
      return Array.isArray(data) ? (data as BundleIncluder[]) : [];
    },
    enabled: Boolean(bundleId),
  });
}

export function useBundleHolders(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.holders(bundleId ?? ""),
    queryFn: async (): Promise<BundleHolder[]> => {
      const data = await request<unknown>(`/bundles/${bundleId}/holders`);
      return Array.isArray(data) ? (data as BundleHolder[]) : [];
    },
    enabled: Boolean(bundleId),
  });
}

export function useBundleDraft(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.draft(bundleId ?? ""),
    // Normalised here so `BundleDraft`'s non-nullable arrays are TRUE for everything downstream.
    // They were not: a Go nil slice marshals to `null`, five call sites called `.length` or `.map`
    // on it, and the bundles page threw on render for any bundle with nothing unpublished. The
    // backend no longer emits `null` either — but the type declared a guarantee this boundary was
    // not enforcing, and that is what made every consumer wrong at once.
    queryFn: async () => {
      const draft = await request<BundleDraft>(`/bundles/${bundleId}/draft`);
      return {
        ...draft,
        // A role both own and included comes back once per way in; it is one change.
        added: once(draft?.added ?? []),
        removed: once(draft?.removed ?? []),
        includes: draft?.includes ?? [],
        include_changes: draft?.include_changes ?? [],
      };
    },
    enabled: Boolean(bundleId),
  });
}

function once(rows: BundleRoleRow[]): BundleRoleRow[] {
  const seen = new Set<string>();
  return rows.filter((r) => {
    const id = `${r.zitadel_project_id}:${r.zitadel_role_key}`;
    return !seen.has(id) && Boolean(seen.add(id));
  });
}

interface PublishInput {
  note: string;
  /** Move everyone currently holding the bundle onto the new version. */
  migrate: boolean;
  /**
   * Cites the approval the rehearsal issued. Required by the backend whenever
   * the rehearsal reached anybody, and never composed by hand — it comes from
   * the plan currently on screen.
   */
  plan_id?: string;
  /**
   * The operator confirming a holder count above the configured limit. Sent on
   * the rehearsal only: it unlocks issuing the approval rather than changing
   * what the approval does.
   */
  acknowledge_scope?: boolean;
  /**
   * Each bundle that includes this one, answered (B8). Required for every one,
   * with no default: "update" puts the new version in its working copy only.
   */
  includers?: Array<{ bundle_id: string; action: IncluderAction }>;
}

/**
 * Rehearsing a publish returns the plan AND the draft, because the two answer
 * different halves of the same question: what the version will contain, and
 * what it will do to the fourteen people who hold the old one.
 *
 * The plan carries the `plan_id` the apply cites. It did not, once, and the
 * consequence was not subtle: the shared dialog disables Apply without one, so
 * every publish that reached anybody was a preview with no next step.
 */
export function useRehearsePublish(bundleId: string) {
  return useMutation({
    mutationFn: (input: PublishInput) =>
      request<{ plan: BulkPlan; draft: BundleDraft }>(`/bundles/${bundleId}/publish`, {
        method: "POST",
        body: input,
      }),
  });
}

export function useApplyPublish(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PublishInput) =>
      request<{ plan: BulkPlan; version: BundleVersion }>(
        `/bundles/${bundleId}/publish?apply=true`,
        { method: "POST", body: input },
      ),
    onSuccess: () => invalidateBundle(qc, bundleId),
  });
}

interface MoveInput {
  version_id: string;
  user_ids: string[];
  plan_id?: string;
  acknowledge_scope?: boolean;
}

export function useRehearseMoveHolders(bundleId: string) {
  return useMutation({
    mutationFn: (input: MoveInput) =>
      request<BulkPlan>(`/bundles/${bundleId}/holders/move`, { method: "POST", body: input }),
  });
}

export function useApplyMoveHolders(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: MoveInput) =>
      request<BulkPlan>(`/bundles/${bundleId}/holders/move?apply=true`, {
        method: "POST",
        body: input,
      }),
    onSuccess: () => invalidateBundle(qc, bundleId),
  });
}

/**
 * Everything a version change touches. The People list is included because a
 * person's row carries which version of each bundle they hold, and a stale row
 * there would show somebody on a version they were just moved off.
 */
function invalidateBundle(qc: ReturnType<typeof useQueryClient>, bundleId: string) {
  qc.invalidateQueries({ queryKey: KEYS.versions(bundleId) });
  qc.invalidateQueries({ queryKey: KEYS.holders(bundleId) });
  qc.invalidateQueries({ queryKey: KEYS.draft(bundleId) });
  qc.invalidateQueries({ queryKey: bundlesQueryKeys.list });
  qc.invalidateQueries({ queryKey: ["users"] });
  qc.invalidateQueries({ queryKey: ["propagations"] });
  // A version change is a bundle-wide closure diff, the same kind of event the
  // governance summary tracks everywhere else it happens.
  qc.invalidateQueries({ queryKey: ["governance"] });
}

export const bundleVersionKeys = KEYS;

/**
 * Discard changes (B6): the working copy goes back to the latest published
 * version. Nobody's access is touched, so only the bundle's own reads refresh.
 */
export function useDiscardBundleDraft(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => request<BundleDraft>(`/bundles/${encodeURIComponent(bundleId)}/draft`, { method: "DELETE" }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: KEYS.draft(bundleId) });
      qc.invalidateQueries({ queryKey: ["bundles", bundleId, "roles"] });
      qc.invalidateQueries({ queryKey: bundlesQueryKeys.list });
    },
  });
}
